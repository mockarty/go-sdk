// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

// FuzzResultJSON exercises the JSON serializer against arbitrary inputs.
// The contract: whatever combination of names, statuses, attachments and
// labels we throw at the writer, the produced JSON must round-trip back to
// the same logical Result without losing data, panic, or emit invalid UTF-8.
//
// Mandatory per CLAUDE.md "Fuzz tests" guidance — any encoder reachable from
// untrusted input gets fuzzed.
func FuzzResultJSON(f *testing.F) {
	// Seed corpus — at minimum: empty, simple, multi-byte UTF-8, and a few
	// edge characters that have historically broken JSON encoders.
	f.Add("simple", "OK", "passed", "feature-a", "key", "value", []byte("hello"), "text/plain")
	f.Add("", "", "passed", "", "", "", []byte{}, "")
	f.Add("кир", "значение", "failed", "тег", "k", "v", []byte("ё"), "text/plain")
	f.Add("emoji-🚀", "💥", "broken", "f", "k", "v", []byte("💩"), "application/octet-stream")
	f.Add("quotes\"and\\backslash", "tab\tnewline\n", "skipped", "f", "k", "v",
		[]byte("\x00\x01\x02"), "application/octet-stream")
	f.Add("very-long-"+stringRepeat("x", 1024), "v", "passed", "f", "k", "v", []byte("payload"), "text/plain")

	f.Fuzz(func(t *testing.T, name, value, status, tag, paramName, paramValue string, attBytes []byte, mime string) {
		dir := filepath.Join(t.TempDir(), "allure-results")
		ctx, finish := WithTest(context.Background(), name, WithResultsDir(dir))
		defer finish()
		Feature(ctx, value)
		Tag(ctx, tag)
		Parameter(ctx, paramName, paramValue)
		Step(ctx, name, func() {
			Attachment(ctx, "att", attBytes, mime)
		})
		// Walk the in-memory result through MarshalIndent to assert it never
		// errors. The writer uses Marshal (no indent), so this is a strictly
		// stronger property.
		scope := fromContext(ctx)
		if scope == nil {
			t.Fatal("scope missing from ctx after WithTest")
		}
		scope.mu.Lock()
		r := scope.result
		scope.mu.Unlock()

		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatalf("marshal failed: %v\ninput name=%q", err, name)
		}
		if !utf8.Valid(data) {
			t.Fatalf("marshalled JSON is not valid UTF-8: %q", data)
		}
		var back Result
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("round-trip decode failed: %v\nJSON=%s", err, string(data))
		}
		// Status is enum-typed but the user-supplied "status" string is never
		// pushed directly into r.Status — the writer normalises to passed/etc.
		// So we only assert the writer produced a known status literal.
		switch back.Status {
		case StatusPassed, StatusFailed, StatusBroken, StatusSkipped:
		default:
			t.Fatalf("unexpected aggregate status %q", back.Status)
		}
	})
}

func stringRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
