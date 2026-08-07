# 3. Monetary amounts as int64 cents

- Status: accepted
- Date: 2026-08-07
- Supersedes: —

## Context

This service is a loan processor: principal, balances, payments, income, and debt are its subject matter. The first endpoint introduces monetary fields on the public API, which fixes a convention that later endpoints — loan creation, balance queries, payment posting — will either follow or contradict.

Floating-point money accumulates representation error under repeated arithmetic, and amortization and payment allocation are exactly repeated arithmetic. A currency amount also has to survive JSON, where a bare number gives a client no indication of its unit and where large values risk precision loss in consumers that parse into a double.

## Decision

Every monetary value is an `int64` count of cents, and every field name carries the unit as a suffix: `annual_income_cents`, `requested_amount_cents`, `monthly_debt_cents`.

This applies to the wire contract, the domain types, and any future storage schema. No monetary value is ever a float. Amounts are not formatted for display by this service; rendering dollars is the client's concern.

## Consequences

**Positive.** Arithmetic is exact and associative — no accumulated drift over an amortization schedule. The unit is unambiguous at every call site and to every API client without consulting documentation, so the "was that dollars or cents?" defect class is closed. `int64` cents reaches roughly $92 quadrillion, which is not a practical bound. JSON integers within that range are exactly representable by any conformant parser.

**Negative.** Division does not distribute cleanly: splitting a balance into payments leaves remainder cents that must be allocated by an explicit, documented rule rather than left to rounding. Callers must convert at the boundary, and a client that sends dollars gets a silent factor-of-100 error that validation cannot detect — the field-name suffix is the only defense.

**Neutral.** The design assumes a single currency. Multi-currency support would need a paired currency code, which this convention does not preclude but does not provide.
