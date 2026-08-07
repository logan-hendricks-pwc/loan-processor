package loan

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSSNNeverRendersDigits(t *testing.T) {
	const digits = "123456789"
	s := SSN(digits)

	for _, rendered := range []string{
		fmt.Sprintf("%v", s),
		fmt.Sprintf("%s", s),
		fmt.Sprintf("%+v", s),
	} {
		if strings.Contains(rendered, digits) {
			t.Errorf("SSN leaked through fmt rendering: got %q", rendered)
		}
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(data), digits) {
		t.Errorf("SSN leaked through json.Marshal: got %s", data)
	}

	if got := s.Reveal(); got != digits {
		t.Errorf("Reveal() = %q, want %q", got, digits)
	}
}

func TestDateOfBirthNeverRendersValue(t *testing.T) {
	birth, err := time.Parse("2006-01-02", "1990-05-15")
	if err != nil {
		t.Fatalf("parse fixture date: %v", err)
	}
	d := DateOfBirth(birth)

	for _, rendered := range []string{
		fmt.Sprintf("%v", d),
		fmt.Sprintf("%s", d),
		fmt.Sprintf("%+v", d),
	} {
		if strings.Contains(rendered, "1990") {
			t.Errorf("DateOfBirth leaked through fmt rendering: got %q", rendered)
		}
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(data), "1990") {
		t.Errorf("DateOfBirth leaked through json.Marshal: got %s", data)
	}

	if got := d.Reveal(); !got.Equal(birth) {
		t.Errorf("Reveal() = %v, want %v", got, birth)
	}
}
