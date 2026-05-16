// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

// CoverageHint nudges the engine toward unseen behaviour.  The runtime
// uses these as a priority signal — they DO NOT bound what the engine
// explores.
//
// Two complementary axes are supported:
//
//   - ResponseStatusCodes — status codes the user has already exercised
//     in normal tests, so the engine should reward mutations that elicit
//     other codes.
//   - JSONPaths — JSONPath expressions the user has already asserted
//     against in normal tests, so the engine should reward mutations
//     that change OTHER paths' shapes.
type CoverageHint struct {
	ResponseStatusCodes []int    `json:"responseStatusCodes,omitempty"`
	JSONPaths           []string `json:"jsonPaths,omitempty"`
}

// WithCoverageHint replaces the per-target coverage hint.
func WithCoverageHint(h CoverageHint) Option {
	return func(t *Target) {
		t.coverage = CoverageHint{
			ResponseStatusCodes: append([]int(nil), h.ResponseStatusCodes...),
			JSONPaths:           append([]string(nil), h.JSONPaths...),
		}
	}
}

// WithExpectedStatus appends a single status code to the coverage hint.
//
// Sugar over WithCoverageHint for the common case of incrementally
// listing the codes the API normally returns:
//
//	fuzz.NewTarget("t",
//	    fuzz.WithExpectedStatus(200),
//	    fuzz.WithExpectedStatus(201),
//	    fuzz.WithExpectedStatus(404),
//	)
func WithExpectedStatus(code int) Option {
	return func(t *Target) {
		t.coverage.ResponseStatusCodes = append(t.coverage.ResponseStatusCodes, code)
	}
}

// WithAssertedJSONPath appends one JSONPath expression to the coverage
// hint.
func WithAssertedJSONPath(path string) Option {
	return func(t *Target) {
		t.coverage.JSONPaths = append(t.coverage.JSONPaths, path)
	}
}
