// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

// Matcher is a placeholder embedded in request/response bodies. The
// writer walks the body during serialisation, extracts every Matcher
// into the appropriate `matchingRules` entry, and replaces the
// placeholder with the matcher's Example (the value used at
// mock-server-time to satisfy the interaction).
//
// Matchers are immutable value types — the DSL returns a fresh Matcher
// per call so a single matcher constant can be reused across
// interactions without aliasing.
type Matcher struct {
	// Example is the concrete value the mock server uses for this
	// position. Verifiers also fall back to Example when no provider-
	// supplied actual is available.
	Example any
	// Rule is the on-the-wire representation in the `matchers` array.
	// Type is the V4 `type` tag, redundant with Rule.Match for V3 but
	// kept separate because the V4 schema spells the same concept
	// differently in different categories ("type" vs "regex" etc.).
	Type string
	Rule MatcherRule
	// Children describe sub-matchers for compound matchers like
	// EachLike — the writer recurses into Children when it builds the
	// matching-rules paths.
	Children []ChildMatcher
}

// ChildMatcher is one sub-matcher inside a compound (EachLike, etc.).
//
// PathSegment is the JSON-path step relative to the parent — e.g.
// `[*]` for "each element", `.field` for "one named child". The writer
// concatenates segments to produce full matching-rule paths.
type ChildMatcher struct {
	PathSegment string
	Matcher     Matcher
}

// Like matches by JSON type. The actual value at this position must
// have the same JSON type (number/string/bool/object/array) as
// example.
//
// V3 emits {"match":"type"}; V4 emits {"type":"type"} — both are
// understood by every modern verifier.
func Like(example any) Matcher {
	return Matcher{
		Example: example,
		Type:    "type",
		Rule:    MatcherRule{Match: "type"},
	}
}

// Term matches values against a regular expression. Example must
// itself satisfy regex — the DSL does not enforce this client-side,
// but server-side verifiers do.
//
// Both V3 and V4 spell this `{"match":"regex","regex":"..."}`.
func Term(example string, regex string) Matcher {
	return Matcher{
		Example: example,
		Type:    "regex",
		Rule:    MatcherRule{Match: "regex", Regex: regex},
	}
}

// Regex is an alias of [Term] kept for parity with pact-js / pact-jvm
// where both names are conventional.
func Regex(example string, regex string) Matcher { return Term(example, regex) }

// Integer matches any integer of any width.
func Integer(example int) Matcher {
	return Matcher{
		Example: example,
		Type:    "integer",
		Rule:    MatcherRule{Match: "integer"},
	}
}

// Decimal matches any floating-point number.
func Decimal(example float64) Matcher {
	return Matcher{
		Example: example,
		Type:    "decimal",
		Rule:    MatcherRule{Match: "decimal"},
	}
}

// Boolean matches any boolean value.
func Boolean(example bool) Matcher {
	return Matcher{
		Example: example,
		Type:    "boolean",
		Rule:    MatcherRule{Match: "boolean"},
	}
}

// EachLike matches an array whose every element has the same JSON
// shape as example. Use min==0 for "may be empty"; the writer rejects
// negative values.
//
// example is wrapped in a single-element slice for the on-wire body
// preview, satisfying verifiers that need at least one concrete
// instance.
func EachLike(example any, min int) Matcher {
	if min < 0 {
		min = 0
	}
	m := min
	return Matcher{
		Example: []any{example},
		Type:    "type",
		Rule:    MatcherRule{Match: "type", Min: &m},
		Children: []ChildMatcher{{
			PathSegment: "[*]",
			Matcher:     Like(example),
		}},
	}
}

// MinType matches a typed array of at least min elements (V4-leaning,
// but V3 accepts it as well). Equivalent to EachLike with min set.
func MinType(example any, min int) Matcher {
	if min < 0 {
		min = 0
	}
	m := min
	return Matcher{
		Example: []any{example},
		Type:    "type",
		Rule:    MatcherRule{Match: "type", Min: &m},
	}
}

// MaxType caps an array at max elements.
func MaxType(example any, max int) Matcher {
	if max < 0 {
		max = 0
	}
	mx := max
	return Matcher{
		Example: []any{example},
		Type:    "type",
		Rule:    MatcherRule{Match: "type", Max: &mx},
	}
}

// MinMaxType bounds an array by both min and max.
func MinMaxType(example any, min, max int) Matcher {
	if min < 0 {
		min = 0
	}
	if max < min {
		max = min
	}
	mn, mx := min, max
	return Matcher{
		Example: []any{example},
		Type:    "type",
		Rule:    MatcherRule{Match: "type", Min: &mn, Max: &mx},
	}
}

// MatchType is the V4-spelled alias for [Like] — same wire shape, kept
// distinct so DSL reads idiomatically when the user is explicitly
// targeting V4.
func MatchType(example any) Matcher {
	m := Like(example)
	return m
}

// EachKeyLike matches an object whose every key obeys the matcher's
// type (the value pattern is independent and is matched by EachValue
// when the user wants that constraint).
//
// V4-only construct; on V3 it degrades to a permissive `type` match
// to preserve round-trip.
func EachKeyLike(example any) Matcher {
	return Matcher{
		Example: example,
		Type:    "values",
		Rule:    MatcherRule{Match: "values"},
	}
}

// EachKey is the V4 typed key-shape matcher. Keys must satisfy the
// example matcher (e.g. all keys must be UUIDs).
func EachKey(keyMatcher Matcher) Matcher {
	return Matcher{
		Example: keyMatcher.Example,
		Type:    "each-key",
		Rule:    MatcherRule{Match: "each-key"},
		Children: []ChildMatcher{{
			PathSegment: "[*]",
			Matcher:     keyMatcher,
		}},
	}
}

// EachValue is the V4 typed value-shape matcher. Values must satisfy
// the supplied matcher; useful in combination with [EachKey] for fully
// typed maps.
func EachValue(valueMatcher Matcher) Matcher {
	return Matcher{
		Example: valueMatcher.Example,
		Type:    "each-value",
		Rule:    MatcherRule{Match: "each-value"},
		Children: []ChildMatcher{{
			PathSegment: "[*]",
			Matcher:     valueMatcher,
		}},
	}
}

// ArrayContains is a V4 matcher: succeeds when the actual array
// contains at least one element matching each of variants. variants
// must be non-empty — an empty variants list is treated as "match any
// non-empty array".
func ArrayContains(variants ...Matcher) Matcher {
	children := make([]ChildMatcher, 0, len(variants))
	examples := make([]any, 0, len(variants))
	for i, v := range variants {
		children = append(children, ChildMatcher{
			PathSegment: indexSegment(i),
			Matcher:     v,
		})
		examples = append(examples, v.Example)
	}
	return Matcher{
		Example:  examples,
		Type:     "arrayContains",
		Rule:     MatcherRule{Match: "arrayContains"},
		Children: children,
	}
}

// Equality forces exact-equality comparison (overrides any inherited
// type-based matching). V4-only; V3 verifiers fall back to default
// equality so the output is still valid.
func Equality(example any) Matcher {
	return Matcher{
		Example: example,
		Type:    "equality",
		Rule:    MatcherRule{Match: "equality"},
	}
}

// indexSegment renders an array index as a JSON-path segment.
//
// Numbers above 9 still serialise correctly because matching paths
// only ever index arrays by literal position.
func indexSegment(i int) string {
	// Use a small-int fast path to avoid pulling fmt for the common case.
	switch i {
	case 0:
		return "[0]"
	case 1:
		return "[1]"
	case 2:
		return "[2]"
	case 3:
		return "[3]"
	case 4:
		return "[4]"
	}
	return "[" + itoa(i) + "]"
}

// itoa is a tiny decimal printer; equivalent to strconv.Itoa for non-
// negative values but avoids the import for a single-use helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	idx := len(buf)
	for n > 0 {
		idx--
		buf[idx] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		idx--
		buf[idx] = '-'
	}
	return string(buf[idx:])
}
