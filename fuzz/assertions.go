// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"fmt"
	"time"
)

// Assertion is a pass/fail criterion the runtime evaluates after each
// fuzzed request.  Built-in assertions live in this file; the JSON shape
// is a small tagged union the engine dispatches on by Kind.
//
// Assertion is intentionally a struct, not an interface — it must
// round-trip through encoding/json.  Variants are produced by the
// helper constructors (AssertStatus, AssertNoCrash, ...).
type Assertion struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern,omitempty"`
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	MaxMS   int64  `json:"maxMs,omitempty"`
}

// Assertion kinds.  Adding one is a coordinated server-side change — the
// engine has to grow a matching evaluator.
const (
	AssertKindStatus       = "status"
	AssertKindNoCrash      = "no_crash"
	AssertKindRespTimeMax  = "response_time_max"
	AssertKindNoErrorRegex = "no_error_regex"
	AssertKindBodyContains = "body_contains"
)

// AssertStatus says responses must fall inside [min, max] inclusive.
// Passing min=200, max=299 expresses "2xx only — anything else is a
// finding".
func AssertStatus(minStatus, maxStatus int) Assertion {
	return Assertion{Kind: AssertKindStatus, Min: minStatus, Max: maxStatus}
}

// AssertStatusClass is a sugar for AssertStatus.  classDigit=2 → 200..299,
// classDigit=4 → 400..499, etc.  Out-of-range digits clamp to 2xx.
func AssertStatusClass(classDigit int) Assertion {
	if classDigit < 1 || classDigit > 5 {
		classDigit = 2
	}
	base := classDigit * 100
	return AssertStatus(base, base+99)
}

// AssertNoCrash says no request must trigger a TCP reset, TLS abort,
// or HTTP 5xx (the runtime treats those as a "crash" finding).
func AssertNoCrash() Assertion { return Assertion{Kind: AssertKindNoCrash} }

// AssertResponseTimeUnder says no request may exceed d.  Sub-millisecond
// durations are rounded up to 1ms (the runtime's resolution).
func AssertResponseTimeUnder(d time.Duration) Assertion {
	ms := d.Milliseconds()
	if ms < 1 && d > 0 {
		ms = 1
	}
	return Assertion{Kind: AssertKindRespTimeMax, MaxMS: ms}
}

// AssertNoErrorInBody says the response body must NOT match the given
// regex (e.g. `(?i)(exception|stack trace|panic|undefined)`).
func AssertNoErrorInBody(pattern string) Assertion {
	return Assertion{Kind: AssertKindNoErrorRegex, Pattern: pattern}
}

// AssertBodyContains says the response body MUST match the regex (a
// presence assertion for happy-path probes).
func AssertBodyContains(pattern string) Assertion {
	return Assertion{Kind: AssertKindBodyContains, Pattern: pattern}
}

// WithAssertion appends an assertion to the target. Multiple WithAssertion
// calls accumulate; they are AND-ed at runtime.
func WithAssertion(a Assertion) Option {
	return func(t *Target) { t.assertions = append(t.assertions, a) }
}

// validateAssertion is the local sanity-check the transpiler runs before
// emitting the JSON.  It catches the obvious shape errors (e.g.
// AssertStatus with min > max) so an invalid target fails fast in Go
// instead of opaquely on the server.
func validateAssertion(a Assertion) error {
	switch a.Kind {
	case AssertKindStatus:
		if a.Min < 0 || a.Max < 0 || a.Min > a.Max {
			return fmt.Errorf("AssertStatus min=%d max=%d: min must be ≤ max and non-negative", a.Min, a.Max)
		}
	case AssertKindRespTimeMax:
		if a.MaxMS < 0 {
			return fmt.Errorf("AssertResponseTimeUnder: duration must be non-negative")
		}
	case AssertKindNoErrorRegex, AssertKindBodyContains:
		if a.Pattern == "" {
			return fmt.Errorf("%s: pattern must not be empty", a.Kind)
		}
	case AssertKindNoCrash:
		// nothing to validate
	case "":
		return fmt.Errorf("assertion: empty Kind")
	default:
		return fmt.Errorf("assertion: unknown Kind %q", a.Kind)
	}
	return nil
}
