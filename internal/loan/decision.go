package loan

import "time"

// Outcome is the result of underwriting an application.
type Outcome string

const (
	Approved Outcome = "APPROVED"
	Denied   Outcome = "DENIED"
)

// Reason codes emitted by the built-in rules engine. These are the stable,
// machine-readable part of the contract: if the underlying underwriting
// policy changes, the values these represent may change, but the codes
// themselves should not.
const (
	ReasonMeetsAllCriteria         = "MEETS_ALL_CRITERIA"
	ReasonCreditScoreBelowMinimum  = "CREDIT_SCORE_BELOW_MINIMUM"
	ReasonDebtToIncomeTooHigh      = "DEBT_TO_INCOME_TOO_HIGH"
	ReasonAmountExceedsIncomeLimit = "AMOUNT_EXCEEDS_INCOME_LIMIT"
	ReasonInsufficientIncome       = "INSUFFICIENT_INCOME"
	ReasonApplicantUnderAge        = "APPLICANT_UNDER_AGE"
	ReasonAmountExceedsProgramMax  = "AMOUNT_EXCEEDS_PROGRAM_MAXIMUM"
)

// Decision is the outcome of evaluating an Application. Reference is left
// empty by a DecisionEngine; the HTTP layer generates it (see
// internal/server's newReference) and attaches it after evaluation.
type Decision struct {
	Outcome     Outcome
	ReasonCodes []string
	Reference   string
	EvaluatedAt time.Time
}
