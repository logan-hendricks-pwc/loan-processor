## Context

`loan-processor` is a Go 1.26 HTTP service with one route (`/healthz`), a `gorilla/mux` router, a logging middleware, and an empty `/api/v1` subrouter left as a marker for future work. `Server` is a struct holding only a router; `routes()` is the single registration point. There is no domain package, no datastore, no configuration layer beyond `ADDR`, and no tests.

This change adds the first real endpoint. Two facts shape the design more than anything else:

1. **The request carries direct identifiers** — SSN, date of birth, name, email — alongside financial detail. The service has no encryption, no key management, and no access control. Every design choice below is made to minimize how long that data exists and how many places it can escape to.
2. **The underwriting logic is provisional.** The thresholds chosen here are engineering defaults, not a reviewed lending policy. The real decision source will eventually be a credit bureau or an underwriting vendor. The design must make that swap cheap.

There are no ADRs in this repository yet (`adr/` does not exist), so no prior architectural commitments constrain this design.

## Goals / Non-Goals

**Goals:**

- One endpoint, `POST /api/v1/loan-applications`, that validates an application and returns a decision with reason codes.
- A `DecisionEngine` seam such that replacing rules-based underwriting with a network-backed underwriter touches one constructor call and one new type — not the handler, not the request contract, not the response contract.
- Sensitive fields are unable to reach logs or a persistent store, enforced by type design rather than by developer discipline.
- Every scenario in the spec is reachable by a `net/http/httptest` test with no external services running.

**Non-Goals:**

- Persistence of any kind. No database, no cache, no file, no outbound call carrying applicant data.
- Authentication, authorization, rate limiting, or TLS termination — see Risks.
- Interest modeling, amortization schedules, loan servicing, or funding. `APPROVED` is an underwriting outcome, not a booked loan.
- Adverse-action notice generation. Reason codes are structured so a notice generator can be built later, but the generator is not in this change.
- Configuration of the rules thresholds at runtime. They are compile-time constants for now.

## Decisions

### D1 — `DecisionEngine` is a single-method interface owned by the consumer

```go
// internal/loan
type DecisionEngine interface {
    Evaluate(ctx context.Context, app Application) (Decision, error)
}
```

Defined in `internal/loan` next to the domain types, with `RulesEngine` as the only implementation in this change. `Server` holds the interface, not the concrete type.

*Why:* A one-method interface is the cheapest possible seam. A bureau-backed engine is then a type that satisfies the same signature; `ctx` is already threaded so a network implementation gets cancellation and deadlines for free without a contract change.

*Alternative — no interface, call `RulesEngine` directly:* Less indirection today, but every test of the handler would have to construct inputs that steer the real rules, and swapping in a bureau later would mean editing the handler. The spec explicitly requires an engine-agnostic handler, so the interface is load-bearing, not speculative.

*Alternative — a multi-method interface (`Evaluate`, `Explain`, `Score`):* Speculative generality. There is one caller and one call.

### D2 — Sensitive fields are distinct types that redact themselves

```go
type SSN string
func (SSN) String() string           { return "[REDACTED]" }
func (SSN) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }
func (s SSN) Reveal() string         { return string(s) }
```

Same treatment for `DateOfBirth`. The plain value is reachable only through an explicit `Reveal()` call.

*Why:* The spec forbids these values from appearing in logs, errors, or responses. Relying on reviewers to never pass an applicant to `log.Printf` or `json.Marshal` fails eventually — one `%+v` on a struct is all it takes. Making the zero-effort path safe and the unsafe path explicit inverts that: leaking now requires typing `Reveal()`, which is greppable and reviewable.

*Cost:* Unmarshaling needs a matching `UnmarshalJSON` that accepts the real value while `MarshalJSON` emits the placeholder — an asymmetry that must be commented, or a future reader will "fix" it.

*Alternative — a redacting log wrapper:* Only covers the logger. `json.Marshal` on an error path, a `fmt.Errorf("%v", app)`, and a future template render all bypass it.

### D3 — Validation runs as a separate pass before the engine, and collects all errors

A `validate(req) []FieldError` over the decoded request, returning every violation rather than short-circuiting on the first.

*Why:* The spec requires multi-field error reporting in one round trip, and requires that no engine call happens for invalid input. Keeping validation outside the engine means the engine's contract is "this application is already well-formed" — a bureau implementation doesn't reimplement range checks.

*Alternative — a struct-tag validation library:* Adds a dependency for roughly 10 fields, and the DTI and age rules don't express well as tags anyway. Hand-written validation is ~60 lines and has no dependency cost.

### D4 — Decode into a DTO, then convert to the domain type

`internal/server` decodes into an unexported request struct with JSON tags; `internal/loan.Application` is constructed from it after validation. The wire shape and the domain shape are allowed to drift.

*Why:* The domain type can then use the redacting types from D2 and semantically meaningful types (`time.Time` for DOB, not `string`), while the DTO stays a faithful mirror of the JSON contract. It also keeps `internal/loan` free of any HTTP or JSON-tag concern, so it stays unit-testable in isolation.

### D5 — Money is `int64` cents; the wire contract says so in the field names

All monetary fields are `int64` named `*_cents`.

*Why:* Floating-point money is a defect waiting to be filed. Naming the unit in the field makes the contract unambiguous to clients and removes a whole class of "was that dollars?" bugs. `int64` cents covers ~$92 quadrillion — not a constraint.

### D6 — `application_reference` is a random UUID, generated per request

Generated fresh for each evaluation from `crypto/rand`; not derived from any applicant field.

*Why:* It gives support and log correlation a handle without any tie to identity. The spec requires that it not be reversibly derived from the SSN — hashing the SSN would create a stable pseudo-identifier that is re-identifiable given a candidate SSN list, which is the opposite of the goal. Because it is random, two identical submissions get different references; the spec names this explicitly so it isn't mistaken for an idempotency key.

*Implementation note:* Go 1.26 has `crypto/rand` and the standard library can format a v4 UUID by hand in a dozen lines; no dependency needed.

### D7 — Engine calls are wrapped in a timeout derived from the request context

`ctx, cancel := context.WithTimeout(r.Context(), evaluationTimeout)` with `evaluationTimeout` at 5 seconds, comfortably inside the server's existing 10-second `WriteTimeout`.

*Why:* `RulesEngine` is pure arithmetic and will never come close, so this is dead weight today — but it is the wiring a bureau-backed engine needs, and adding it now means the swap doesn't have to touch the handler (the point of D1). Deriving from `r.Context()` means client disconnects abandon the evaluation too.

### D8 — Body size is capped with `http.MaxBytesReader`

Wrap `r.Body` at 64 KiB before decoding, and use `decoder.DisallowUnknownFields()`.

*Why:* The endpoint is unauthenticated (see Risks), so an unbounded decode is a trivial memory-exhaustion vector. `MaxBytesReader` fails during the read rather than after buffering. `DisallowUnknownFields` catches client typos — a misspelled `credit_scoree` would otherwise silently decode as a zero credit score and produce a confident denial.

### D9 — A recovery middleware is added alongside the existing logger

A `recoverPanic` middleware that logs the panic and stack, then writes a generic `500`.

*Why:* The spec requires that a panic in the handler not spill the request body. Without recovery, `net/http`'s default logs the panic and closes the connection; with a request struct on the stack, a panic message can carry field values. Recovery also keeps one bad request from taking a goroutine down mid-response. Ordering matters: recovery must wrap the logger so a panic still produces an access log line.

### D10 — Dependency injection through `New`, wired in `main`

`server.New(engine loan.DecisionEngine) *Server`, called from `cmd/server/main.go` with `loan.NewRulesEngine()`. This is a **BREAKING** change to `server.New`'s signature, with exactly one caller in the repo.

*Alternative — a functional-options constructor:* Better when there are many optional dependencies. There is one, and it is required. Options would be ceremony now; the migration to options later is mechanical if a third dependency shows up.

### D11 — Handlers for loans live in a new file

`internal/server/loan_handlers.go`, leaving `handlers.go` for `handleHealth` and the shared `respondJSON`/`respondError` helpers. `respondError` gains a sibling `respondFieldErrors` for the `400` shape.

*Why:* Follows the existing convention of one concern per file (`server.go` / `handlers.go` / `middleware.go`) rather than growing a catch-all.

## Risks / Trade-offs

- **The endpoint accepts PII with no authentication** → Not mitigated in this change; explicitly accepted and bounded. The controls that partially reduce blast radius are here (body cap, no persistence, redacting types, evaluation timeout), but anyone who can reach the port can submit SSNs. Mitigation is deployment-level until `add-api-authentication` lands: do not expose this service publicly, bind behind an authenticated gateway, and treat the follow-up change as a release blocker for any non-development environment. This must be stated in the PR description, not only here.

- **Thresholds are engineering defaults, not lending policy** → Constants live in one block in `internal/loan` with a comment stating they are provisional and unreviewed. Reason codes are stable, machine-readable strings so that when real policy arrives, the values change but the contract does not.

- **Fair-lending exposure if this is mistaken for production underwriting** → The rules use no protected characteristic, and the only age input is a hard under-18 eligibility gate. Even so, real ECOA/Reg B obligations (adverse-action notices, model documentation) are unaddressed. Flagged in the proposal; must be resolved before real applicants are decisioned.

- **Redacting types make debugging harder** → A developer chasing a bad decision cannot see the input in a log. Accepted deliberately — that is the feature. The mitigation is that `RulesEngine` is a pure function, so a failing case is reproducible from a unit test without ever logging real data.

- **`MarshalJSON` and `UnmarshalJSON` are deliberately asymmetric** → A future contributor may see the mismatch as a bug and "fix" it, silently un-redacting logs. Mitigated with a comment at the type and a test asserting that marshaling an `Application` produces no SSN digits.

- **No persistence means no audit trail of applications** → If a regulator or a support case needs "what did this applicant submit," the answer is that the service does not know. This is the correct posture for a service with no encryption at rest, but it is a real limitation to revisit when a datastore with proper controls exists.

- **Breaking `server.New`'s signature** → One caller, compile-time failure if missed. No runtime risk.

## Migration Plan

Additive; nothing to migrate. Deployment is a normal rolling replace — the only externally visible change is a new route, and `server.New`'s signature change is caught at compile time. Rollback is a redeploy of the previous binary; with no persistence and no schema, there is no state to unwind.

**Release gate:** this must not be deployed to any environment reachable from outside the trust boundary until `add-api-authentication` is in place.

## Open Questions

- **Should the response distinguish "denied" from "referred for manual review"?** Real underwriting usually has three outcomes. This change ships two, which keeps the contract small — but adding a third value to the `decision` enum later is a breaking change for clients that switch exhaustively on it. Worth deciding before the first external consumer integrates.
- **Should reason codes be returned for approvals at all?** Currently `MEETS_ALL_CRITERIA` is a single filler value so the array is never empty. An alternative is an empty array on approval, which is arguably more honest but forces clients to handle both shapes.
- **What is the intended deployment boundary?** The authentication decision depends on whether this sits behind an API gateway that already authenticates, or is directly exposed. This changes how urgent `add-api-authentication` is and what form it should take.

No in-force ADRs need revisiting — there are none.
