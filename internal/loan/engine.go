package loan

import "context"

// DecisionEngine evaluates a loan application and returns a decision. The
// HTTP layer depends only on this interface (ADR-0001), never on a concrete
// engine, so a bureau-backed or third-party underwriter can replace the
// built-in rules without touching the endpoint, its request contract, or
// its response contract.
type DecisionEngine interface {
	Evaluate(ctx context.Context, app Application) (Decision, error)
}
