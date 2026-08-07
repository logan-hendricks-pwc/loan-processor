# 1. Decision logic behind a DecisionEngine interface

- Status: accepted
- Date: 2026-08-07
- Supersedes: —

## Context

The first underwriting endpoint needs an approve/deny answer. The logic that produces that answer is provisional: the thresholds shipped now are engineering defaults, and the eventual source of truth is expected to be a credit bureau or a third-party underwriting vendor reached over the network. That replacement is a near-certainty, not a hypothetical.

If the HTTP handler computes the decision inline, swapping in a network-backed underwriter later means rewriting the handler, re-deriving its tests, and re-establishing that the request and response contracts did not drift in the process. The endpoint's public contract would be entangled with a policy implementation that is known to be temporary.

## Decision

Underwriting is reached only through a single-method interface owned by the domain package:

```go
// internal/loan
type DecisionEngine interface {
    Evaluate(ctx context.Context, app Application) (Decision, error)
}
```

The HTTP layer depends on the interface and never on a concrete engine. Implementations are chosen in `main` and injected through `server.New`. Input validation stays outside the engine, so an implementation may assume it receives a well-formed application. `context.Context` is part of the signature from the start, so a network-backed implementation inherits cancellation and deadlines without a contract change.

The rules-based implementation shipped alongside this ADR is one implementation, not a privileged one.

## Consequences

**Positive.** Replacing underwriting is a new type plus one line in `main`; the endpoint, its request and response shapes, and its handler tests are untouched. Handler behavior — validation, error mapping, redaction, timeouts — is testable against a substitute engine with no real policy involved, and policy is testable as a pure function with no HTTP involved. Failure and timeout handling live at the boundary, so every implementation gets the same externally visible behavior on error.

**Negative.** One layer of indirection for what is currently arithmetic; reading the decision path means following an interface to its implementation. The `Evaluate` signature is now a commitment — an implementation needing input the interface does not carry (a document upload, a multi-step referral) forces a signature change across all implementations.

**Neutral.** The interface deliberately stays at one method. Additional operations (scoring, explanation, pricing) should be justified by a real second caller, not anticipated.
