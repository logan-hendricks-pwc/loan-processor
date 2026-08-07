package loan

// Applicant holds the identifying and financial details of the person
// requesting a loan.
//
// SSN and DateOfBirth use the self-redacting types from sensitive.go
// (ADR-0002). Email carries no redacting type of its own, but is kept
// unexported: encoding/json only ever sees exported fields, so this keeps
// it out of any json.Marshal of an Applicant without needing one. Use
// NewApplicant to construct a value from outside this package, and Email()
// to read it back.
type Applicant struct {
	FullName          string
	DateOfBirth       DateOfBirth
	SSN               SSN
	email             string
	AnnualIncomeCents int64
	MonthlyDebtCents  int64
	CreditScore       int
}

// NewApplicant constructs an Applicant.
func NewApplicant(fullName string, dateOfBirth DateOfBirth, ssn SSN, email string, annualIncomeCents, monthlyDebtCents int64, creditScore int) Applicant {
	return Applicant{
		FullName:          fullName,
		DateOfBirth:       dateOfBirth,
		SSN:               ssn,
		email:             email,
		AnnualIncomeCents: annualIncomeCents,
		MonthlyDebtCents:  monthlyDebtCents,
		CreditScore:       creditScore,
	}
}

// Email returns the applicant's email address.
func (a Applicant) Email() string { return a.email }

// Application is one applicant's request for a loan of a given amount and
// term. It has no JSON tags: this is the domain shape, not the wire shape.
// See internal/server's request DTO for the wire shape and the conversion
// between the two.
type Application struct {
	Applicant            Applicant
	RequestedAmountCents int64
	TermMonths           int
	Purpose              string
}
