## Why

The loan-processor service exposes no loan functionality today — only `/healthz`. Before applicants can create, update, or pay down loans, the service needs a front door that takes a loan application and returns an underwriting decision. This change adds that entry point: a single endpoint that accepts an applicant's details and requested amount and responds with an approval or denial plus the reasons behind it.

## What Changes

- Add `POST /api/v1/loan-applications` — accepts applicant identity, financial details, and a requested loan amount; returns an `APPROVED` or `DENIED` decision with machine-readable reason codes.
- Introduce a `DecisionEngine` interface so underwriting logic is swappable. Ship one implementation, `RulesEngine`, that decides from documented deterministic thresholds (credit score bands, debt-to-income ratio, requested amount vs. income, applicant eligibility). A future bureau or third-party underwriting integration replaces the implementation without touching the handler.
- Add request validation with field-level error responses for malformed or out-of-range input.
- Add PII-safe logging: the audit line records the decision, reason codes, and a correlation id — never the SSN, date of birth, or raw applicant identity.
- Add a domain package holding the application, decision, and reason-code types.

**Non-goals** (explicitly deferred to later changes):

- **No persistence.** The endpoint is stateless: it decides and responds. Nothing about the applicant is written to disk or to a database. This is deliberate — the service has no datastore, and adding one would drag encryption-at-rest, retention, and redaction policy into this change.
- **No authentication.** See Risks below.
- No loan booking, servicing, amortization, or payment handling. "Approved" here means an underwriting outcome, not a funded loan.

## Capabilities

### New Capabilities

- `loan-application-decision`: Submitting a loan application and receiving an underwriting decision — the request contract, validation rules, decision outcomes and reason codes, the pluggable decision engine boundary, and the handling requirements for the sensitive data the request carries.

### Modified Capabilities

None. This is the first capability spec in the project; `openspec/specs/` is currently empty.

## Impact

**Code**

- `internal/server/server.go` — register the new route on the existing `/api/v1` subrouter.
- `internal/server/handlers.go` — new handler; may split loan handlers into their own file.
- `internal/server/server.go` (`Server` struct) — gains a `DecisionEngine` dependency; `New()` gains a way to supply it.
- New `internal/loan/` package — domain types, reason codes, and the `DecisionEngine` interface with its rules implementation.
- `cmd/server/main.go` — wire the concrete engine into `server.New()`.

**APIs**

- New public endpoint `POST /api/v1/loan-applications`. Additive; no existing behavior changes.

**Dependencies**

- None added. Standard library plus the existing `gorilla/mux`.

**Risks**

- ⚠️ **The endpoint accepts unauthenticated PII.** The service has no authentication layer, so anyone who can reach it can submit SSNs and income data and consume underwriting capacity. Accepted for this change to keep it scoped; it must not reach a public environment in this state. Tracked as a follow-up change, `add-api-authentication`, alongside rate limiting and TLS termination requirements.
- ⚠️ **Decision logic is a placeholder, not a compliance-reviewed underwriting policy.** The thresholds in `RulesEngine` are engineering defaults chosen to make the contract testable. Real lending decisions carry fair-lending and adverse-action-notice obligations (ECOA/Reg B) that this change does not address. The reason codes are designed so an adverse-action notice can be generated later, but generating one is out of scope.
