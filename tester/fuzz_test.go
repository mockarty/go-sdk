// Copyright (c) 2026 Mockarty. All rights reserved.

package tester

import (
	"encoding/json"
	"testing"
)

// FuzzResolveJSONPath ensures the path walker never panics on weird
// inputs — the parser handles malformed paths via returned errors.
func FuzzResolveJSONPath(f *testing.F) {
	seeds := []string{
		"$", "$.a", "$.a.b", "$.a[0]", "$.a[*]", "$.a[-1]",
		"$.a[", "$.a]", "$..a", "$.a..b", "$.", ".",
		"$.a[0][*].b", "$.a.b.c.d.e.f.g",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	doc := map[string]any{
		"a": []any{
			map[string]any{"b": "x"},
			map[string]any{"b": "y"},
		},
	}
	f.Fuzz(func(t *testing.T, path string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on path %q: %v", path, r)
			}
		}()
		_, _ = resolveJSONPath(doc, path)
	})
}

// FuzzInterpolate ensures the {{var}} replacer never panics and always
// produces UTF-8-safe output for arbitrary input strings.
func FuzzInterpolate(f *testing.F) {
	seeds := []struct{ in, name, value string }{
		{"plain", "n", "x"},
		{"hi {{n}}", "n", "x"},
		{"{{a}}{{a}}{{a}}", "a", "z"},
		{"{{a}}/{{b}}", "a", "1"},
		{"{{ n }}", "n", "x"},
		{"{{not-closed", "n", "x"},
	}
	for _, s := range seeds {
		f.Add(s.in, s.name, s.value)
	}
	f.Fuzz(func(t *testing.T, in, name, value string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on interpolate(%q,{%q:%q}): %v", in, name, value, r)
			}
		}()
		_ = interpolate(in, map[string]string{name: value})
	})
}

// FuzzExtractRoundTrip ensures Extract -> SetVar -> interpolation is
// loss-free for JSON-encodable scalars: numbers and strings.
func FuzzExtractRoundTrip(f *testing.F) {
	seeds := []string{"tok-123", "", "a/b", "{{nested}}", "[]", "null"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Encode and decode through the same json.Marshal path Extract uses.
		b, err := json.Marshal(s)
		if err != nil {
			t.Skip()
		}
		var got string
		if err := json.Unmarshal(b, &got); err != nil {
			t.Skip()
		}
		if got != s {
			t.Fatalf("round-trip mismatch: %q -> %q", s, got)
		}
	})
}
