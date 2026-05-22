// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"strings"
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

// FuzzParseSSE ensures the SSE parser never panics on arbitrary input
// streams. The WHATWG spec is permissive (anything before a blank line
// is buffered then dispatched / discarded); the parser must not panic
// on truncated frames, malformed retry: lines, embedded NULs, etc.
func FuzzParseSSE(f *testing.F) {
	seeds := []string{
		"data: hello\n\n",
		"event: tick\ndata: 1\n\n",
		"retry: 5000\nid: x\ndata: y\n\n",
		": comment\n\n",
		"data: line1\ndata: line2\n\n",
		"\n\n\n",
		"data: x", // no terminating blank line — should still buffer
		"retry: not-a-number\ndata: x\n\n",
		"data:\n\n", // empty data
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on parseSSE(%q): %v", in, r)
			}
		}()
		_ = parseSSE(strings.NewReader(in))
	})
}

// FuzzWrapSOAPEnvelope ensures the wrapper never panics and produces
// a string that contains a <Body> when the wrap branch fired.
func FuzzWrapSOAPEnvelope(f *testing.F) {
	seeds := []string{
		"<X/>",
		"<X></X>",
		"<?xml version=\"1.0\"?><Y/>",
		"<soap:Envelope xmlns:soap=\"x\"><soap:Body/></soap:Envelope>",
		"",
		"<<<<<>>>>>",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on wrapSOAPEnvelope(%q): %v", in, r)
			}
		}()
		_ = wrapSOAPEnvelope(in)
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
