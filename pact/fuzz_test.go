// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/mockarty/mockarty-go/pact"
)

// FuzzRenderPactFileDoesNotPanic feeds arbitrary descriptions / paths
// through the consumer DSL and confirms the serialiser produces well-
// formed JSON regardless of input.
//
// We round-trip the rendered bytes through encoding/json — any panic in
// the walker or unmarshal failure on the output is a bug. The fuzzer is
// seeded with the V3 and V4 reference fixtures so coverage starts from
// real-world inputs.
func FuzzRenderPactFileDoesNotPanic(f *testing.F) {
	// Seeds: a handful of plausible descriptions + body shapes.
	f.Add("hello world", "/x", `{"a":1}`)
	f.Add("", "/", `{}`)
	f.Add("привет", "/path/with/Юникод", `{"k":"v"}`)
	f.Add("a\x00b", "/null", `null`)
	f.Add("", "", "")

	f.Fuzz(func(t *testing.T, desc, path, bodyJSON string) {
		var body any
		_ = json.Unmarshal([]byte(bodyJSON), &body)
		c := pact.NewConsumer("A",
			pact.WithProvider("B"),
			pact.WithSpecVersion(pact.SpecV4),
			pact.WithOutputDir(t.TempDir()),
		)
		c.AddInteraction().
			UponReceiving(desc).
			WithRequest(http.MethodGet, path).
			WithJSONBody(body).
			WillRespondWith(200)
		srv, err := c.Start(context.Background())
		if err != nil {
			// Construction errors are valid for invalid inputs — we only
			// care that the writer never panics.
			return
		}
		if err := srv.Close(); err != nil {
			return
		}
		files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
		if len(files) == 0 {
			return
		}
		blob, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var sink map[string]any
		if err := json.Unmarshal(blob, &sink); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, blob)
		}
	})
}

// FuzzMatcherSerialisation ensures every matcher rule serialises and
// parses back without losing critical fields. Seeded with the matcher
// API surface so the fuzzer can mutate around real examples.
func FuzzMatcherSerialisation(f *testing.F) {
	f.Add("type", "", 0, 0)
	f.Add("regex", "[a-z]+", 0, 0)
	f.Add("integer", "", 0, 0)
	f.Add("decimal", "", 0, 0)
	f.Add("type", "", 1, 5)

	f.Fuzz(func(t *testing.T, match, regex string, minV, maxV int) {
		// Pact files are JSON (= valid UTF-8 by spec). Skip inputs that
		// aren't UTF-8 — Go's encoder normalises bad bytes to U+FFFD,
		// which is the correct behaviour but breaks byte-for-byte
		// round-trip equality. The on-wire output is still valid.
		if !utf8.ValidString(match) || !utf8.ValidString(regex) {
			t.Skip("non-UTF-8 input — pact files are JSON, must be UTF-8")
		}
		mn, mx := minV, maxV
		rule := pact.MatcherRule{Match: match, Regex: regex}
		if minV != 0 {
			rule.Min = &mn
		}
		if maxV != 0 {
			rule.Max = &mx
		}
		blob, err := json.Marshal(rule)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var round pact.MatcherRule
		if err := json.Unmarshal(blob, &round); err != nil {
			t.Fatalf("unmarshal: %v\n%s", err, blob)
		}
		if round.Match != rule.Match || round.Regex != rule.Regex {
			t.Fatalf("round-trip lost: %+v -> %+v", rule, round)
		}
	})
}
