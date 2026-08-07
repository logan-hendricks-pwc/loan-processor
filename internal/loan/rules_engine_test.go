package loan

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"
)

// qualifyingApplication mirrors the spec's "qualifying application" scenario:
// credit score 720, annual income $90,000, monthly debt $500, requesting
// $20,000 over 60 months from an adult applicant. Tests copy and perturb it.
func qualifyingApplication() Application {
	return Application{
		Applicant: NewApplicant(
			"Qualifying Applicant",
			DateOfBirth(adultBirthDate(30)),
			SSN("123456789"),
			"applicant@example.com",
			9_000_000, // $90,000
			50_000,    // $500
			720,
		),
		RequestedAmountCents: 2_000_000, // $20,000
		TermMonths:           60,
	}
}

// adultBirthDate returns a date of birth that makes the applicant exactly
// ageYears old today, computed relative to time.Now() so the test doesn't
// rot as the calendar advances.
func adultBirthDate(ageYears int) time.Time {
	return time.Now().AddDate(-ageYears, 0, 0)
}

func evaluate(t *testing.T, app Application) Decision {
	t.Helper()
	engine := NewRulesEngine()
	decision, err := engine.Evaluate(context.Background(), app)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return decision
}

func TestRulesEngineScenarios(t *testing.T) {
	t.Run("qualifying application is approved", func(t *testing.T) {
		decision := evaluate(t, qualifyingApplication())
		if decision.Outcome != Approved {
			t.Fatalf("Outcome = %s, want %s (codes: %v)", decision.Outcome, Approved, decision.ReasonCodes)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonMeetsAllCriteria) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonMeetsAllCriteria)
		}
	})

	t.Run("credit score below the floor is denied", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.CreditScore = 580
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s", decision.Outcome, Denied)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonCreditScoreBelowMinimum) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonCreditScoreBelowMinimum)
		}
	})

	t.Run("debt to income above the ceiling is denied", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.AnnualIncomeCents = 3_600_000 // $36,000
		app.Applicant.MonthlyDebtCents = 120_000     // $1,200
		app.RequestedAmountCents = 1_200_000          // $12,000
		app.TermMonths = 24
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s", decision.Outcome, Denied)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonDebtToIncomeTooHigh) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonDebtToIncomeTooHigh)
		}
	})

	t.Run("all failing conditions are reported", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.CreditScore = 500
		app.Applicant.AnnualIncomeCents = 0
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s", decision.Outcome, Denied)
		}
		for _, want := range []string{ReasonCreditScoreBelowMinimum, ReasonInsufficientIncome} {
			if !slices.Contains(decision.ReasonCodes, want) {
				t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, want)
			}
		}
	})

	t.Run("credit score boundary is inclusive at 600", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.CreditScore = 600
		decision := evaluate(t, app)
		if decision.Outcome != Approved {
			t.Fatalf("Outcome at 600 = %s, want %s (codes: %v)", decision.Outcome, Approved, decision.ReasonCodes)
		}
	})

	t.Run("credit score boundary denies at 599", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.CreditScore = 599
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome at 599 = %s, want %s", decision.Outcome, Denied)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonCreditScoreBelowMinimum) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonCreditScoreBelowMinimum)
		}
	})

	t.Run("identical applications yield identical decisions", func(t *testing.T) {
		app := qualifyingApplication()
		first := evaluate(t, app)
		second := evaluate(t, app)
		if first.Outcome != second.Outcome {
			t.Errorf("Outcome differs across identical evaluations: %s vs %s", first.Outcome, second.Outcome)
		}
		if !reflect.DeepEqual(first.ReasonCodes, second.ReasonCodes) {
			t.Errorf("ReasonCodes differ across identical evaluations: %v vs %v", first.ReasonCodes, second.ReasonCodes)
		}
	})

	t.Run("zero income is denied", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.AnnualIncomeCents = 0
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s", decision.Outcome, Denied)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonInsufficientIncome) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonInsufficientIncome)
		}
	})

	t.Run("under 18 applicant is denied", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.DateOfBirth = DateOfBirth(adultBirthDate(17))
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s", decision.Outcome, Denied)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonApplicantUnderAge) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonApplicantUnderAge)
		}
	})

	t.Run("amount exceeding the program maximum is denied", func(t *testing.T) {
		app := qualifyingApplication()
		app.Applicant.AnnualIncomeCents = 100_000_000 // $1,000,000, so income limits don't also trip
		app.Applicant.MonthlyDebtCents = 0
		app.RequestedAmountCents = 15_000_000 // $150,000
		app.TermMonths = 84
		decision := evaluate(t, app)
		if decision.Outcome != Denied {
			t.Fatalf("Outcome = %s, want %s (codes: %v)", decision.Outcome, Denied, decision.ReasonCodes)
		}
		if !slices.Contains(decision.ReasonCodes, ReasonAmountExceedsProgramMax) {
			t.Errorf("ReasonCodes = %v, want it to contain %s", decision.ReasonCodes, ReasonAmountExceedsProgramMax)
		}
	})
}
