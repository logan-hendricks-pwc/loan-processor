# ADR Review Manifest

## ADR Review Completed

- Date: 2026-08-07
- Reviewer: logan.hendricks@pwc.com
- Change: add-loan-application-decision

## In-Force ADR Context Reviewed

- None: no existing repository-level ADRs were present. The `adr/` folder did not exist before this change, so no prior decisions constrained the design and no supersession graph applied.

## Repository-Level ADRs Created

- `adr/0001-decision-logic-behind-a-decisionengine-interface.md` — Underwriting is reachable only through a single-method `DecisionEngine` interface; the HTTP layer never depends on a concrete engine, so a bureau-backed implementation can replace the built-in rules without touching the endpoint contract.
- `adr/0002-self-redacting-types-for-sensitive-fields.md` — SSN, date of birth, and comparable direct identifiers use named types whose `String` and `MarshalJSON` emit a redaction placeholder; the real value is reachable only via an explicit `Reveal()`.
- `adr/0003-monetary-amounts-as-int64-cents.md` — All monetary values are `int64` cents, never floats, with the unit named in every wire field (`*_cents`).

## Notes

Three of the eleven decisions in `design.md` cleared the durable bar — each establishes a boundary or convention that later changes (loan creation, payments, bureau integration) must follow or explicitly supersede.

The remaining design decisions were judged tactical and deliberately left out of `adr/`: validation as a separate pass (D3), DTO-to-domain conversion (D4), random UUID references (D6), the engine timeout (D7), body-size capping and `DisallowUnknownFields` (D8), panic-recovery middleware (D9), constructor injection over functional options (D10), and handler file layout (D11). These are implementation choices recorded in `design.md`; reversing any of them would not ripple beyond this change.

Two decisions were considered and rejected as ADR material because they are explicitly temporary rather than durable: the stateless no-persistence posture and the absence of authentication. Both are expected to change, are recorded as risks in `proposal.md` and `design.md`, and the authentication gap is tracked as the follow-up change `add-api-authentication`. Should a persistence model be adopted later, that decision will warrant its own ADR.

Sequence numbering starts at 0001 and is monotonic across the repository.
