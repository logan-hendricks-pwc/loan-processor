## ADDED Requirements

### Requirement: Submit a loan application for a decision

The system SHALL expose `POST /api/v1/loan-applications`, accepting a JSON body describing one applicant and one requested loan, and SHALL respond synchronously with an underwriting decision.

The request body SHALL contain:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `applicant.full_name` | string | yes | 1–200 characters |
| `applicant.date_of_birth` | string | yes | `YYYY-MM-DD` |
| `applicant.ssn` | string | yes | 9 digits, with or without dashes |
| `applicant.email` | string | yes | must contain `@` |
| `applicant.annual_income_cents` | integer | yes | ≥ 0 |
| `applicant.monthly_debt_cents` | integer | yes | ≥ 0 |
| `applicant.credit_score` | integer | yes | 300–850 inclusive |
| `requested_amount_cents` | integer | yes | > 0 |
| `term_months` | integer | yes | 12–84 inclusive |
| `purpose` | string | no | free text, ≤ 500 characters |

The response body SHALL contain `decision` (`APPROVED` or `DENIED`), `reason_codes` (array of strings), `application_reference` (an opaque identifier for this request), and `evaluated_at` (RFC 3339 timestamp).

#### Scenario: Well-formed application returns a decision

- **WHEN** a client sends a valid application body
- **THEN** the service responds `200 OK` with `Content-Type: application/json`
- **AND** the body contains `decision` equal to `APPROVED` or `DENIED`
- **AND** the body contains a non-empty `reason_codes` array
- **AND** the body contains an `application_reference` and an `evaluated_at` timestamp

#### Scenario: Response carries no applicant identity

- **WHEN** the service returns a decision for any application
- **THEN** the response body contains no SSN, date of birth, name, or email
- **AND** `application_reference` is not derived from the SSN in any reversible way

### Requirement: Reject malformed or out-of-range requests

The system SHALL validate the request before invoking any decision logic and SHALL reject invalid input with `400 Bad Request` and a body listing every offending field, so a client can correct all problems in one round trip.

A rejection body SHALL have the shape `{"error": "<summary>", "fields": [{"field": "<path>", "message": "<reason>"}]}`.

#### Scenario: Missing required field

- **WHEN** a client omits `requested_amount_cents`
- **THEN** the service responds `400 Bad Request`
- **AND** `fields` includes an entry for `requested_amount_cents`
- **AND** no decision engine call is made

#### Scenario: Multiple invalid fields reported together

- **WHEN** a client sends `credit_score` of `900` and `term_months` of `6`
- **THEN** the service responds `400 Bad Request`
- **AND** `fields` includes entries for both `credit_score` and `term_months`

#### Scenario: Malformed JSON

- **WHEN** the request body is not valid JSON
- **THEN** the service responds `400 Bad Request` with an `error` describing the parse failure
- **AND** the response does not echo the raw request body

#### Scenario: Unsupported method on the collection

- **WHEN** a client sends `GET /api/v1/loan-applications`
- **THEN** the service responds `405 Method Not Allowed`

#### Scenario: Oversized body is refused

- **WHEN** a client sends a body larger than 64 KiB
- **THEN** the service responds `413 Payload Too Large`
- **AND** the service does not buffer the entire body into memory

### Requirement: Decision logic is behind a swappable engine

The system SHALL define a `DecisionEngine` abstraction that takes a validated application and returns a decision with reason codes. The HTTP handler SHALL depend only on that abstraction, so an implementation backed by a credit bureau or third-party underwriter can replace the built-in rules without changing the endpoint, its request contract, or its response contract.

#### Scenario: Handler is engine-agnostic

- **WHEN** the service is constructed with a substitute engine that always returns `DENIED`
- **THEN** every valid application receives `DENIED`
- **AND** no change to the handler or the request/response contract is required

#### Scenario: Engine failure does not leak internals

- **WHEN** the configured engine returns an error
- **THEN** the service responds `503 Service Unavailable`
- **AND** the response body contains a generic message with no stack trace, engine name, or internal detail

#### Scenario: Engine evaluation is bounded

- **WHEN** an engine does not return within the configured evaluation timeout
- **THEN** the service abandons the evaluation and responds `503 Service Unavailable`
- **AND** the timeout is recorded in the audit log

### Requirement: Built-in rules engine produces deterministic decisions

The system SHALL ship a rules-based `DecisionEngine` whose outcome is a pure function of the submitted application — the same input SHALL always produce the same decision and the same reason codes.

The rules engine SHALL deny an application when any of the following holds, and SHALL emit the corresponding reason code:

| Condition | Reason code |
| --- | --- |
| `credit_score` < 600 | `CREDIT_SCORE_BELOW_MINIMUM` |
| Debt-to-income ratio ≥ 0.43, where DTI = (`monthly_debt_cents` + estimated monthly payment) ÷ (`annual_income_cents` ÷ 12) | `DEBT_TO_INCOME_TOO_HIGH` |
| `requested_amount_cents` > 50% of `annual_income_cents` | `AMOUNT_EXCEEDS_INCOME_LIMIT` |
| `annual_income_cents` = 0 | `INSUFFICIENT_INCOME` |
| Applicant is under 18 years old at `evaluated_at` | `APPLICANT_UNDER_AGE` |
| `requested_amount_cents` > 10,000,000 (i.e. $100,000) | `AMOUNT_EXCEEDS_PROGRAM_MAXIMUM` |

Estimated monthly payment SHALL be `requested_amount_cents ÷ term_months` (principal only; the rules engine does not model interest).

#### Scenario: Qualifying application is approved

- **WHEN** an applicant has a credit score of 720, annual income of $90,000, monthly debt of $500, and requests $20,000 over 60 months
- **THEN** the decision is `APPROVED`
- **AND** `reason_codes` contains `MEETS_ALL_CRITERIA`

#### Scenario: Credit score below the floor

- **WHEN** an applicant has a credit score of 580 and otherwise qualifies
- **THEN** the decision is `DENIED`
- **AND** `reason_codes` contains `CREDIT_SCORE_BELOW_MINIMUM`

#### Scenario: Debt-to-income above the ceiling

- **WHEN** an applicant has annual income of $36,000, monthly debt of $1,200, and requests $12,000 over 24 months
- **THEN** the decision is `DENIED`
- **AND** `reason_codes` contains `DEBT_TO_INCOME_TOO_HIGH`

#### Scenario: All failing conditions are reported

- **WHEN** an applicant has a credit score of 500 and zero annual income
- **THEN** the decision is `DENIED`
- **AND** `reason_codes` contains both `CREDIT_SCORE_BELOW_MINIMUM` and `INSUFFICIENT_INCOME`

#### Scenario: Boundary values are inclusive as specified

- **WHEN** an applicant has a credit score of exactly 600 and otherwise qualifies
- **THEN** the decision is `APPROVED`
- **AND** an otherwise identical applicant with a credit score of 599 is `DENIED`

#### Scenario: Same input yields the same decision

- **WHEN** the identical application is submitted twice
- **THEN** both responses carry the same `decision` and the same `reason_codes`
- **AND** the two `application_reference` values differ, because each submission is a distinct evaluation

### Requirement: Sensitive applicant data is never persisted

The system SHALL evaluate the application in memory and discard it when the response is written. The system SHALL NOT write applicant identity or financial detail to a database, a file, a cache, or any outbound service.

#### Scenario: No datastore write occurs

- **WHEN** an application is evaluated
- **THEN** no record of the applicant is written to any persistent store
- **AND** a subsequent request cannot retrieve the submitted application from this service

### Requirement: Sensitive applicant data is kept out of logs and errors

The system SHALL keep SSN, date of birth, full name, and email out of every log line, error message, and response body. The audit record for an evaluation SHALL contain only the `application_reference`, the decision, the reason codes, the evaluation duration, and the request's correlation id.

#### Scenario: Audit line omits identity

- **WHEN** an application is evaluated and the audit line is written
- **THEN** the line contains the `application_reference`, decision, and reason codes
- **AND** the line contains no SSN, date of birth, name, or email

#### Scenario: Validation error message omits the rejected value

- **WHEN** validation rejects `applicant.ssn` because it is the wrong length
- **THEN** the `message` for that field describes the rule that failed
- **AND** the `message` does not include the submitted SSN value

#### Scenario: Panic recovery does not spill the request

- **WHEN** the handler panics while processing an application
- **THEN** the service responds `500 Internal Server Error` with a generic body
- **AND** the recovery log line contains no field from the request body

#### Scenario: Domain type does not stringify sensitive fields

- **WHEN** an applicant value is formatted with a default string or JSON representation for logging
- **THEN** the SSN and date of birth are rendered as a redaction placeholder rather than their values
