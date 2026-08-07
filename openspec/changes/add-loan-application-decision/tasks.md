## 1. Domain package foundation

- [x] 1.1 Create `internal/loan/` package.
- [x] 1.2 Add `SSN` and `DateOfBirth` types in `internal/loan/sensitive.go`, each with `String()` and `MarshalJSON()` returning `"[REDACTED]"`, an `UnmarshalJSON` that accepts the real value, and a `Reveal()` accessor. Comment the marshal/unmarshal asymmetry at the type — it reads as a bug otherwise (ADR-0002).
- [x] 1.3 Add `internal/loan/sensitive_test.go` asserting that `fmt.Sprintf("%v"/"%s"/"%+v")` and `json.Marshal` on a populated `SSN` and `DateOfBirth` never emit the underlying digits, and that `Reveal()` does.
- [x] 1.4 Define `Applicant` and `Application` in `internal/loan/application.go`, using the redacting types for SSN and date of birth and `int64` cents for every monetary field (ADR-0003). No JSON tags — this type is not the wire shape.
- [x] 1.5 Define `Decision` (an `Outcome` of `APPROVED`/`DENIED`, `ReasonCodes []string`, `Reference string`, `EvaluatedAt time.Time`) and the reason-code constants from the spec in `internal/loan/decision.go`.
- [x] 1.6 Add a test asserting `json.Marshal` of a fully populated `Application` contains no SSN digits, no date-of-birth string, and no email — the regression guard for ADR-0002.

## 2. Decision engine

- [x] 2.1 Declare `DecisionEngine` in `internal/loan/engine.go` as `Evaluate(ctx context.Context, app Application) (Decision, error)` (ADR-0001).
- [x] 2.2 Implement `RulesEngine` with the six denial rules and their reason codes from the spec. Put every threshold in one `const` block with a comment stating they are provisional engineering defaults, not reviewed lending policy.
- [x] 2.3 Make `RulesEngine` accumulate all failing conditions rather than returning on the first — the spec requires every triggered reason code in the response.
- [x] 2.4 Emit `MEETS_ALL_CRITERIA` when nothing fails, so `reason_codes` is never empty.
- [x] 2.5 Compute age from `DateOfBirth` against the evaluation timestamp, not `time.Now()` inside the rule, so the engine stays a pure function of its inputs.
- [x] 2.6 Table-driven tests in `internal/loan/engine_test.go` covering each spec scenario: the qualifying approval, credit-score denial, DTI denial, the multi-code case, the 599/600 boundary pair, and determinism (same input twice → same decision and codes).
- [x] 2.7 Add cases for the remaining rules not named in a spec scenario: zero income, under-18, and the $100,000 program maximum.

## 3. HTTP request and response contract

- [x] 3.1 Create `internal/server/loan_handlers.go`. Define an unexported request DTO mirroring the JSON contract exactly, with JSON tags and plain types (ADR-0002's redacting types belong to the domain, not the wire).
- [x] 3.2 Define the response DTO: `decision`, `reason_codes`, `application_reference`, `evaluated_at` (RFC 3339). It must carry no applicant identity.
- [x] 3.3 Add `FieldError` and the `400` body shape `{"error": ..., "fields": [{"field": ..., "message": ...}]}`, plus a `respondFieldErrors` helper alongside the existing `respondJSON`/`respondError` in `handlers.go`.
- [x] 3.4 Write `validate(req) []FieldError` covering every field rule in the spec table. Collect all violations — do not short-circuit. Error messages must state the rule that failed and must never echo the submitted value.
- [x] 3.5 Add a `newReference()` helper generating a random v4 UUID from `crypto/rand`, not derived from any applicant field.
- [x] 3.6 Write the DTO→`loan.Application` conversion, parsing the date of birth and normalizing the SSN (strip dashes) into the redacting types.

## 4. Handler wiring

- [x] 4.1 Add a `DecisionEngine` field to `Server` and change `New` to `New(engine loan.DecisionEngine) *Server`. **BREAKING** — one caller.
- [x] 4.2 Update `cmd/server/main.go` to construct `loan.NewRulesEngine()` and pass it to `server.New`.
- [x] 4.3 Implement `handleCreateLoanApplication`: wrap the body in `http.MaxBytesReader` at 64 KiB, decode with `DisallowUnknownFields()`, validate, convert, evaluate, respond `200`.
- [x] 4.4 Map failures to status codes: malformed JSON and validation failures → `400`; body over the cap → `413`; engine error or timeout → `503` with a generic body carrying no internal detail.
- [x] 4.5 Derive an evaluation context with `context.WithTimeout(r.Context(), evaluationTimeout)` at 5 seconds, inside the server's existing 10s `WriteTimeout`.
- [x] 4.6 Register `POST /api/v1/loan-applications` on the existing `api` subrouter in `routes()` and drop the `_ = api` placeholder. Verify `gorilla/mux` returns `405` for `GET` on the same path; add an explicit method-not-allowed handler if it does not.

## 5. Logging and panic safety

- [x] 5.1 Add a `recoverPanic` middleware to `internal/server/middleware.go` that logs the panic and stack, then writes a generic `500`.
- [x] 5.2 Register `recoverPanic` so it wraps `logging` — a panic must still produce an access log line.
- [x] 5.3 Emit the audit line after each evaluation with only the reference, decision, reason codes, duration, and correlation id. Never the applicant.
- [x] 5.4 Log engine timeouts explicitly, as the spec requires them recorded.

## 6. Endpoint tests

- [ ] 6.1 Add `internal/server/loan_handlers_test.go` using `httptest`, with a stub `DecisionEngine` so handler behavior is tested independently of policy (ADR-0001).
- [ ] 6.2 Happy path: valid body → `200`, JSON content type, `decision` present, non-empty `reason_codes`, reference and timestamp present.
- [ ] 6.3 Assert the success response body contains no SSN, date of birth, name, or email.
- [ ] 6.4 Validation cases: missing `requested_amount_cents` → `400` naming that field; `credit_score` 900 with `term_months` 6 → `400` naming both; malformed JSON → `400` that does not echo the body.
- [ ] 6.5 Assert no engine call occurs on a validation failure — record invocations on the stub.
- [ ] 6.6 `GET /api/v1/loan-applications` → `405`.
- [ ] 6.7 Body over 64 KiB → `413`.
- [ ] 6.8 Stub engine returning an error → `503` with no stack trace or engine name in the body; stub blocking past the timeout → `503`.
- [ ] 6.9 Stub engine always returning `DENIED` → every valid application is denied, proving the handler is engine-agnostic.
- [ ] 6.10 Stub engine that panics → `500` with a generic body; assert the captured log output contains no request field values.
- [ ] 6.11 Capture log output during a successful evaluation and assert it contains the reference and decision but no SSN, date of birth, name, or email.
- [ ] 6.12 Assert two identical submissions return the same decision and reason codes but different `application_reference` values.

## 7. Verification

- [ ] 7.1 `make vet` clean.
- [ ] 7.2 `make test` — all tests pass.
- [ ] 7.3 `make build` succeeds.
- [ ] 7.4 Run `make run` and exercise the endpoint with `curl` for an approval, a denial, and a validation failure; confirm the server log lines carry no PII.
- [ ] 7.5 Walk the spec scenario list and confirm each has a corresponding test.
- [ ] 7.6 In the PR description, state that the endpoint is unauthenticated and must not reach an externally reachable environment until `add-api-authentication` lands.
