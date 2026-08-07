package loan

import (
	"context"
	"math"
	"time"
)

// Thresholds below are provisional engineering defaults chosen to make the
// contract testable — they are not a reviewed lending policy. Real
// underwriting thresholds require compliance and fair-lending review before
// this engine (or its thresholds) are used to decision real applicants (see
// proposal.md Risks).
const (
	minCreditScore          = 600
	maxDebtToIncomeRatio    = 0.43
	maxAmountToIncomeRatio  = 0.50
	minApplicantAgeYears    = 18
	maxRequestedAmountCents = 100_000 * 100 // $100,000
)

// RulesEngine is the built-in DecisionEngine. It is a pure function of its
// input: the same Application always produces the same decision and reason
// codes, because every rule is evaluated against the timestamp captured
// once at the top of Evaluate rather than against time.Now() inside the
// rule itself.
type RulesEngine struct{}

// NewRulesEngine constructs a RulesEngine.
func NewRulesEngine() *RulesEngine {
	return &RulesEngine{}
}

// Evaluate implements DecisionEngine.
func (e *RulesEngine) Evaluate(ctx context.Context, app Application) (Decision, error) {
	evaluatedAt := time.Now().UTC()

	var codes []string
	if app.Applicant.CreditScore < minCreditScore {
		codes = append(codes, ReasonCreditScoreBelowMinimum)
	}
	if debtToIncomeRatio(app) >= maxDebtToIncomeRatio {
		codes = append(codes, ReasonDebtToIncomeTooHigh)
	}
	if exceedsIncomeLimit(app) {
		codes = append(codes, ReasonAmountExceedsIncomeLimit)
	}
	if app.Applicant.AnnualIncomeCents == 0 {
		codes = append(codes, ReasonInsufficientIncome)
	}
	if isUnderAge(app.Applicant.DateOfBirth, evaluatedAt, minApplicantAgeYears) {
		codes = append(codes, ReasonApplicantUnderAge)
	}
	if app.RequestedAmountCents > maxRequestedAmountCents {
		codes = append(codes, ReasonAmountExceedsProgramMax)
	}

	outcome := Approved
	if len(codes) > 0 {
		outcome = Denied
	} else {
		codes = append(codes, ReasonMeetsAllCriteria)
	}

	return Decision{
		Outcome:     outcome,
		ReasonCodes: codes,
		EvaluatedAt: evaluatedAt,
	}, nil
}

// estimatedMonthlyPaymentCents is principal-only: the rules engine does not
// model interest.
func estimatedMonthlyPaymentCents(app Application) int64 {
	if app.TermMonths == 0 {
		return 0
	}
	return app.RequestedAmountCents / int64(app.TermMonths)
}

func debtToIncomeRatio(app Application) float64 {
	monthlyIncomeCents := float64(app.Applicant.AnnualIncomeCents) / 12
	if monthlyIncomeCents == 0 {
		// Zero income already triggers INSUFFICIENT_INCOME on its own;
		// treat the ratio as unbounded rather than dividing by zero.
		return math.Inf(1)
	}
	monthlyDebtCents := float64(app.Applicant.MonthlyDebtCents) + float64(estimatedMonthlyPaymentCents(app))
	return monthlyDebtCents / monthlyIncomeCents
}

func exceedsIncomeLimit(app Application) bool {
	return float64(app.RequestedAmountCents) > maxAmountToIncomeRatio*float64(app.Applicant.AnnualIncomeCents)
}

// isUnderAge reports whether dob is younger than minYears as of "at".
func isUnderAge(dob DateOfBirth, at time.Time, minYears int) bool {
	birth := dob.Reveal()
	age := at.Year() - birth.Year()
	if birth.AddDate(age, 0, 0).After(at) {
		age--
	}
	return age < minYears
}
