// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"encoding/json"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

func TestLikeBuildsTypeMatcher(t *testing.T) {
	t.Parallel()
	m := pact.Like(42)
	if m.Type != "type" || m.Rule.Match != "type" {
		t.Fatalf("Like: %+v", m)
	}
	if m.Example != 42 {
		t.Fatalf("example = %v", m.Example)
	}
}

func TestTermAndRegexAlias(t *testing.T) {
	t.Parallel()
	a := pact.Term("ab", "[ab]+")
	b := pact.Regex("ab", "[ab]+")
	if a.Rule.Match != "regex" || b.Rule.Match != "regex" {
		t.Fatalf("Term/Regex must produce regex matcher: %+v / %+v", a, b)
	}
	if a.Rule.Regex != "[ab]+" || b.Rule.Regex != "[ab]+" {
		t.Fatalf("regex pattern lost")
	}
}

func TestIntegerDecimalBoolean(t *testing.T) {
	t.Parallel()
	if pact.Integer(1).Rule.Match != "integer" {
		t.Fatalf("Integer")
	}
	if pact.Decimal(1.5).Rule.Match != "decimal" {
		t.Fatalf("Decimal")
	}
	if pact.Boolean(true).Rule.Match != "boolean" {
		t.Fatalf("Boolean")
	}
}

func TestEachLikeAndBounds(t *testing.T) {
	t.Parallel()
	m := pact.EachLike(map[string]any{"id": 1}, 2)
	if m.Rule.Match != "type" {
		t.Fatalf("EachLike match = %q", m.Rule.Match)
	}
	if m.Rule.Min == nil || *m.Rule.Min != 2 {
		t.Fatalf("min lost: %+v", m.Rule.Min)
	}
	// Negative min clamps to 0.
	m2 := pact.EachLike("x", -3)
	if *m2.Rule.Min != 0 {
		t.Fatalf("negative min not clamped: %d", *m2.Rule.Min)
	}
}

func TestMinMaxType(t *testing.T) {
	t.Parallel()
	m := pact.MinMaxType("x", 1, 5)
	if *m.Rule.Min != 1 || *m.Rule.Max != 5 {
		t.Fatalf("bounds: %+v / %+v", m.Rule.Min, m.Rule.Max)
	}
	// max<min coerces upward to keep the contract self-consistent.
	bad := pact.MinMaxType("x", 5, 1)
	if *bad.Rule.Min != 5 || *bad.Rule.Max != 5 {
		t.Fatalf("inverted bounds not coerced: %+v / %+v", bad.Rule.Min, bad.Rule.Max)
	}
}

func TestEachKeyValue(t *testing.T) {
	t.Parallel()
	k := pact.EachKey(pact.Regex("k", "[a-z]+"))
	v := pact.EachValue(pact.Like("v"))
	if k.Rule.Match != "each-key" {
		t.Fatalf("each-key: %+v", k.Rule)
	}
	if v.Rule.Match != "each-value" {
		t.Fatalf("each-value: %+v", v.Rule)
	}
}

func TestArrayContainsEmptyOk(t *testing.T) {
	t.Parallel()
	m := pact.ArrayContains()
	if m.Rule.Match != "arrayContains" {
		t.Fatalf("array contains rule = %+v", m.Rule)
	}
	if _, ok := m.Example.([]any); !ok {
		t.Fatalf("example must be []any; got %T", m.Example)
	}
}

func TestEqualityMatcher(t *testing.T) {
	t.Parallel()
	m := pact.Equality("x")
	if m.Rule.Match != "equality" {
		t.Fatalf("equality: %+v", m.Rule)
	}
}

func TestMatchTypeAliasOfLike(t *testing.T) {
	t.Parallel()
	a := pact.MatchType(7)
	b := pact.Like(7)
	if a.Rule != b.Rule {
		t.Fatalf("MatchType must alias Like; got %+v vs %+v", a, b)
	}
}

func TestMatchersJSONRoundTrip(t *testing.T) {
	t.Parallel()
	// Every matcher's Rule must marshal+unmarshal cleanly via stdlib JSON,
	// confirming the omitempty tags are wired up correctly.
	cases := []pact.MatcherRule{
		pact.Like(1).Rule,
		pact.Term("x", ".+").Rule,
		pact.Integer(1).Rule,
		pact.EachLike(0, 1).Rule,
		pact.MinMaxType(0, 1, 2).Rule,
		pact.Equality("y").Rule,
	}
	for i, r := range cases {
		blob, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}
		var round pact.MatcherRule
		if err := json.Unmarshal(blob, &round); err != nil {
			t.Fatalf("case %d unmarshal: %v", i, err)
		}
		if round.Match != r.Match {
			t.Fatalf("case %d match lost: %q -> %q", i, r.Match, round.Match)
		}
	}
}
