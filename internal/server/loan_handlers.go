package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/logan-hendricks-pwc/loan-processor/internal/loan"
)

const (
	// maxRequestBodyBytes bounds decoding so an unauthenticated caller
	// (see proposal.md Risks) can't exhaust memory with an oversized body.
	maxRequestBodyBytes = 64 * 1024

	dateOfBirthLayout = "2006-01-02"
)

// evaluationTimeout bounds an engine call, comfortably inside the server's
// 10s WriteTimeout (see cmd/server/main.go). It is a var, not a const, so
// tests can shrink it rather than sleeping for the full 5 seconds.
var evaluationTimeout = 5 * time.Second

// createLoanApplicationRequest mirrors the JSON request contract exactly.
// It uses plain wire types; the redacting domain types from ADR-0002 belong
// to internal/loan, not to this DTO.
type createLoanApplicationRequest struct {
	Applicant            applicantRequest `json:"applicant"`
	RequestedAmountCents int64            `json:"requested_amount_cents"`
	TermMonths           int              `json:"term_months"`
	Purpose              string           `json:"purpose"`
}

type applicantRequest struct {
	FullName          string `json:"full_name"`
	DateOfBirth       string `json:"date_of_birth"`
	SSN               string `json:"ssn"`
	Email             string `json:"email"`
	AnnualIncomeCents int64  `json:"annual_income_cents"`
	MonthlyDebtCents  int64  `json:"monthly_debt_cents"`
	CreditScore       int    `json:"credit_score"`
}

// loanApplicationResponse carries no applicant identity — only the
// decision, why it was reached, and how to refer to this evaluation.
type loanApplicationResponse struct {
	Decision             string   `json:"decision"`
	ReasonCodes          []string `json:"reason_codes"`
	ApplicationReference string   `json:"application_reference"`
	EvaluatedAt          string   `json:"evaluated_at"`
}

// validate checks every field rule from the spec and collects every
// violation rather than stopping at the first, so a client can fix all of
// them in one round trip. Messages state the rule that failed and never
// echo the submitted value.
func validate(req createLoanApplicationRequest) []FieldError {
	var errs []FieldError

	if n := len(req.Applicant.FullName); n < 1 || n > 200 {
		errs = append(errs, FieldError{"applicant.full_name", "must be 1-200 characters"})
	}

	if _, err := time.Parse(dateOfBirthLayout, req.Applicant.DateOfBirth); err != nil {
		errs = append(errs, FieldError{"applicant.date_of_birth", "must be a date in YYYY-MM-DD format"})
	}

	if !isValidSSN(req.Applicant.SSN) {
		errs = append(errs, FieldError{"applicant.ssn", "must be 9 digits, with or without dashes"})
	}

	if !strings.Contains(req.Applicant.Email, "@") {
		errs = append(errs, FieldError{"applicant.email", "must contain @"})
	}

	if req.Applicant.AnnualIncomeCents < 0 {
		errs = append(errs, FieldError{"applicant.annual_income_cents", "must be >= 0"})
	}

	if req.Applicant.MonthlyDebtCents < 0 {
		errs = append(errs, FieldError{"applicant.monthly_debt_cents", "must be >= 0"})
	}

	if req.Applicant.CreditScore < 300 || req.Applicant.CreditScore > 850 {
		errs = append(errs, FieldError{"applicant.credit_score", "must be between 300 and 850 inclusive"})
	}

	if req.RequestedAmountCents <= 0 {
		errs = append(errs, FieldError{"requested_amount_cents", "must be > 0"})
	}

	if req.TermMonths < 12 || req.TermMonths > 84 {
		errs = append(errs, FieldError{"term_months", "must be between 12 and 84 inclusive"})
	}

	if len(req.Purpose) > 500 {
		errs = append(errs, FieldError{"purpose", "must be at most 500 characters"})
	}

	return errs
}

// isValidSSN reports whether raw is 9 digits, with or without dashes.
func isValidSSN(raw string) bool {
	digits := strings.ReplaceAll(raw, "-", "")
	if len(digits) != 9 {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// toApplication converts a validated request into the domain type,
// normalizing the SSN and parsing the date of birth into the redacting
// types. Callers must run validate first — the date parse below is not
// re-checked here.
func (req createLoanApplicationRequest) toApplication() loan.Application {
	dob, _ := time.Parse(dateOfBirthLayout, req.Applicant.DateOfBirth)
	ssn := strings.ReplaceAll(req.Applicant.SSN, "-", "")

	applicant := loan.NewApplicant(
		req.Applicant.FullName,
		loan.DateOfBirth(dob),
		loan.SSN(ssn),
		req.Applicant.Email,
		req.Applicant.AnnualIncomeCents,
		req.Applicant.MonthlyDebtCents,
		req.Applicant.CreditScore,
	)

	return loan.Application{
		Applicant:            applicant,
		RequestedAmountCents: req.RequestedAmountCents,
		TermMonths:           req.TermMonths,
		Purpose:              req.Purpose,
	}
}

// newUUID returns a random version 4 UUID from crypto/rand. It backs both
// newReference and the correlation id fallback below; neither is derived
// from any applicant or request field.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand only fails if the OS RNG is unavailable, which
		// leaves nothing safe to fall back to (a predictable reference
		// would defeat the point of using one).
		panic("newUUID: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newReference generates the opaque application_reference returned to the
// client. It is fresh per evaluation and not derived from any applicant
// field, so two identical submissions get different references.
func newReference() string {
	return newUUID()
}

// correlationID returns the caller-supplied correlation id if present, or
// generates one so every audit line still has a handle for log
// correlation.
func correlationID(r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	return newUUID()
}

// isMaxBytesError reports whether err came from an http.MaxBytesReader
// rejecting an oversized body.
func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func (s *Server) handleCreateLoanApplication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		corrID := correlationID(r)

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

		var req createLoanApplicationRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			if isMaxBytesError(err) {
				respondError(w, http.StatusRequestEntityTooLarge, "request body exceeds the maximum allowed size")
				return
			}
			respondError(w, http.StatusBadRequest, "request body is not valid JSON")
			return
		}

		if fieldErrs := validate(req); len(fieldErrs) > 0 {
			respondFieldErrors(w, "validation failed", fieldErrs)
			return
		}

		app := req.toApplication()

		ctx, cancel := context.WithTimeout(r.Context(), evaluationTimeout)
		defer cancel()

		start := time.Now()
		decision, err := s.engine.Evaluate(ctx, app)
		duration := time.Since(start)

		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Printf("audit: engine timeout correlation_id=%s duration=%s", corrID, duration)
			}
			respondError(w, http.StatusServiceUnavailable, "unable to evaluate the application at this time")
			return
		}

		decision.Reference = newReference()

		log.Printf("audit: reference=%s decision=%s reason_codes=%s duration=%s correlation_id=%s",
			decision.Reference, decision.Outcome, strings.Join(decision.ReasonCodes, ","), duration, corrID)

		respondJSON(w, http.StatusOK, loanApplicationResponse{
			Decision:             string(decision.Outcome),
			ReasonCodes:          decision.ReasonCodes,
			ApplicationReference: decision.Reference,
			EvaluatedAt:          decision.EvaluatedAt.Format(time.RFC3339),
		})
	}
}
