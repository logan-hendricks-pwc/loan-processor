package loan

import (
	"encoding/json"
	"time"
)

// redacted is what every sensitive field renders as under any default
// string or JSON encoding.
const redacted = "[REDACTED]"

// SSN holds an applicant's Social Security number. It redacts itself under
// every default rendering — String(), fmt's %v/%s/%+v verbs (which dispatch
// to String() for any Stringer), and json.Marshal all emit the placeholder
// instead of the digits. The real value is reachable only through Reveal
// (ADR-0002).
//
// MarshalJSON and UnmarshalJSON are deliberately asymmetric: UnmarshalJSON
// accepts the real digits, so a request body can populate this type, but
// MarshalJSON always emits the placeholder, so the value can never round
// -trip back out through an encode. This asymmetry is intentional, not a
// bug — do not "fix" it into a symmetric pair, or SSNs will start appearing
// in any JSON encoding of a value that embeds one, including log lines and
// error bodies.
type SSN string

func (SSN) String() string { return redacted }

func (SSN) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

func (s *SSN) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = SSN(v)
	return nil
}

// Reveal returns the underlying SSN digits. Calling it is the only way to
// see the real value; it is deliberately explicit, greppable, and worth a
// second look in review.
func (s SSN) Reveal() string { return string(s) }

// DateOfBirth holds an applicant's date of birth. It is redacted the same
// way as SSN, and for the same reason and via the same asymmetric
// Marshal/Unmarshal pair — see the comment on SSN.
type DateOfBirth time.Time

func (DateOfBirth) String() string { return redacted }

func (DateOfBirth) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }

func (d *DateOfBirth) UnmarshalJSON(data []byte) error {
	var v time.Time
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*d = DateOfBirth(v)
	return nil
}

// Reveal returns the underlying date of birth.
func (d DateOfBirth) Reveal() time.Time { return time.Time(d) }
