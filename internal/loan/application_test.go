package loan

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestApplicationMarshalRedactsSensitiveFields is the regression guard for
// ADR-0002: whatever gets added to Application or Applicant next, marshaling
// a fully populated value must never surface the SSN, the date of birth, or
// the email.
func TestApplicationMarshalRedactsSensitiveFields(t *testing.T) {
	const (
		ssnDigits = "123456789"
		dobString = "1990-05-15"
		email     = "jane.public@example.com"
	)

	dob, err := time.Parse("2006-01-02", dobString)
	if err != nil {
		t.Fatalf("parse fixture date: %v", err)
	}

	app := Application{
		Applicant: NewApplicant(
			"Jane Q. Public",
			DateOfBirth(dob),
			SSN(ssnDigits),
			email,
			9_000_000,
			50_000,
			720,
		),
		RequestedAmountCents: 2_000_000,
		TermMonths:           60,
		Purpose:              "debt consolidation",
	}

	data, err := json.Marshal(app)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	out := string(data)

	for _, forbidden := range []string{ssnDigits, dobString, email} {
		if strings.Contains(out, forbidden) {
			t.Errorf("marshaled Application leaked %q: %s", forbidden, out)
		}
	}
}
