# 2. Self-redacting types for sensitive fields

- Status: accepted
- Date: 2026-08-07
- Supersedes: —

## Context

The loan application endpoint accepts direct identifiers — Social Security number, date of birth, full name, email — alongside financial detail. The service has no encryption at rest, no key management, and no access control. Requirements state these values must never appear in a log line, an error message, or a response body.

The default path in Go works against that. `log.Printf("%+v", app)`, `fmt.Errorf("bad application: %v", app)`, and `json.Marshal(app)` on an error path all render every field, and each is the shortest thing to type. Enforcement by code review means the requirement holds only until the first reviewer is in a hurry — and the failure is silent, discovered when an SSN turns up in a log aggregator.

## Decision

Fields carrying direct identifiers are given distinct named types that redact themselves under every default rendering:

```go
type SSN string

func (SSN) String() string                { return "[REDACTED]" }
func (SSN) MarshalJSON() ([]byte, error)  { return []byte(`"[REDACTED]"`), nil }
func (s SSN) Reveal() string              { return string(s) }
```

The same treatment applies to date of birth and to any field of comparable sensitivity added later. `MarshalJSON` and `UnmarshalJSON` are deliberately asymmetric: unmarshaling accepts the real value, marshaling emits the placeholder. This asymmetry is intentional and must be commented at the type, because it reads as a bug.

The underlying value is reachable only through an explicit `Reveal()`. A test asserts that marshaling a populated application produces no SSN digits.

## Consequences

**Positive.** The zero-effort path is the safe one; leaking a value now requires typing `Reveal()`, which is greppable and stands out in review. Protection covers every rendering path at once — logger, JSON encoder, `fmt` verbs, future template rendering — rather than the one path a redacting log wrapper would cover. Compliance review has a single place to look for what counts as sensitive.

**Negative.** Debugging is genuinely harder: a developer chasing an unexpected decision cannot see the input in a log. This is the intended trade, and it is workable only because the rules engine is a pure function whose failing cases reproduce in a unit test from synthetic data. Every sensitive field also needs a conversion at the boundary, and a contributor who does not know why the marshal asymmetry exists may "fix" it and silently un-redact logs — hence the comment and the test.

**Neutral.** The pattern only helps for fields someone remembered to type. Adding a sensitive field as a plain `string` bypasses it entirely, so new fields still require the judgment call.
