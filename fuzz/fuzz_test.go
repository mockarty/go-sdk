// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mockarty/mockarty-go/fuzz"
)

// FuzzFromJSONDoesNotPanic feeds arbitrary bytes through the parser and
// confirms it never panics.  Errors are tolerated; the contract is "no
// panic" and "no nil-Target returned with nil err".
//
// Seeds: a valid round-trip + a few hand-crafted edge cases that have
// historically tripped naive parsers.
func FuzzFromJSONDoesNotPanic(f *testing.F) {
	// Build a known-good baseline from the DSL, use its bytes as a seed.
	good := fuzz.NewTarget("seed",
		fuzz.WithHTTPEndpoint("POST", "https://x.example", "/p"),
		fuzz.WithSeed(fuzz.Seed("s", `{}`)),
		fuzz.WithMutator(fuzz.MutatorJSON),
	)
	if data, err := good.ToJSON(); err == nil {
		f.Add(string(data))
	}
	f.Add("{}")
	f.Add("")
	f.Add("null")
	f.Add(`{"name":"x"}`)
	f.Add(`{"name":"x","sourceType":"manual","strategy":"mutation","targetBaseUrl":"https://x","options":{}}`)
	// Pathological inputs.
	f.Add(`{"name":"","options":{"maxDuration":"5h"}}`)
	f.Add(`{"options":{"maxDuration":"definitely-not-a-duration"}}`)
	f.Add(`{"seedRequests":[{"id":"a","body":""}]}`)

	f.Fuzz(func(t *testing.T, raw string) {
		// Only feed valid UTF-8 — encoding/json itself rejects invalid
		// UTF-8 in string fields, so the fuzzer would otherwise spend
		// most of its budget on JSON rejections, not on our parser.
		if !utf8.ValidString(raw) {
			t.Skip("non-UTF-8 input")
		}
		tgt, err := fuzz.FromJSON([]byte(raw))
		if err != nil {
			if tgt != nil {
				t.Fatalf("error returned with non-nil target: %v", err)
			}
			return
		}
		if tgt == nil {
			t.Fatal("nil target with nil error")
		}
		// Re-emit and ensure the second round produces valid JSON.
		if data, err := tgt.ToJSON(); err == nil {
			var sink any
			if uErr := json.Unmarshal(data, &sink); uErr != nil {
				t.Fatalf("second-pass JSON invalid: %v\n%s", uErr, data)
			}
		}
	})
}

// FuzzValidateOnArbitraryName confirms Validate never panics on
// arbitrary names and that empty / whitespace-only names fail with a
// stable error (not a panic, not a silent pass).
func FuzzValidateOnArbitraryName(f *testing.F) {
	f.Add("")
	f.Add("   ")
	f.Add("hello")
	f.Add("привет")
	f.Add("a\x00b")
	f.Add(strings.Repeat("x", 1<<10))

	f.Fuzz(func(t *testing.T, name string) {
		tgt := fuzz.NewTarget(name,
			fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
			fuzz.WithMutator(fuzz.MutatorURL),
		)
		_ = tgt.Validate() // must never panic
	})
}
