// Copyright (c) 2026 Mockarty.  All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"fmt"
	"os"

	"github.com/google/uuid"
)

// SeedRequest is one record in the corpus the fuzz engine warps. It
// projects onto the server-side fuzzing.FuzzSeedRequest type.
//
// Fields are tagged with json to keep the on-wire shape stable when the
// transpiler serialises a slice of SeedRequest into the FuzzingConfig
// SeedRequests blob.
type SeedRequest struct {
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	PathParams  map[string]string `json:"pathParams,omitempty"`
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Method      string            `json:"method,omitempty"`
	URL         string            `json:"url,omitempty"`
	Path        string            `json:"path,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
}

// Seed creates a SeedRequest from a human-readable label and a body
// payload (string).  The ID is generated lazily by the transpiler if
// left empty.
func Seed(name, body string) SeedRequest {
	return SeedRequest{
		ID:   uuid.NewString(),
		Name: name,
		Body: body,
	}
}

// SeedBytes is the byte-slice variant of Seed for binary payloads.
// The body is stored as the raw string view of the bytes (the runtime
// re-decodes from the stable JSON-escaped form on the other side).
func SeedBytes(name string, body []byte) SeedRequest {
	return SeedRequest{
		ID:   uuid.NewString(),
		Name: name,
		Body: string(body),
	}
}

// SeedFile reads a payload from disk and returns a SeedRequest. The
// file path becomes the seed's Name; the file contents become Body.
//
// Returns an error if the file cannot be read.  A typical use is to
// keep large reference payloads alongside the test binary:
//
//	s, err := fuzz.SeedFile("testdata/login.json")
//	if err != nil { t.Fatal(err) }
//	target := fuzz.NewTarget("t", fuzz.WithSeedCorpus(s))
func SeedFile(path string) (SeedRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SeedRequest{}, fmt.Errorf("fuzz: read seed file %q: %w", path, err)
	}
	return SeedRequest{
		ID:   uuid.NewString(),
		Name: path,
		Body: string(data),
	}, nil
}

// SeedHTTP is the protocol-tailored variant of Seed that fills out the
// HTTP method / path triple commonly required when the target endpoint
// itself is supposed to vary across seeds (e.g. parameter sweeping a
// REST surface).
func SeedHTTP(name, method, path, body string) SeedRequest {
	return SeedRequest{
		ID:     uuid.NewString(),
		Name:   name,
		Method: method,
		Path:   path,
		Body:   body,
	}
}

// WithSeedCorpus sets (or replaces) the seed corpus for the target.
// Calling the option twice replaces the previous corpus; use the
// variadic form to add many at once.
func WithSeedCorpus(seeds ...SeedRequest) Option {
	cp := append([]SeedRequest(nil), seeds...)
	return func(t *Target) { t.seeds = cp }
}

// WithSeed appends a single seed to the corpus (additive — unlike
// WithSeedCorpus which replaces).
func WithSeed(s SeedRequest) Option {
	return func(t *Target) { t.seeds = append(t.seeds, s) }
}
