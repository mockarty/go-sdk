// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package plugins is the SDK-side Service Provider Interface (SPI) for
// Pact V4 transport / content-type plugins.
//
// The Pact V4 specification allows third-party plugins to extend the
// contract format with new transports (gRPC, async messaging) and new
// content types (protobuf, avro, csv, …). Verifiers and brokers see
// these plugins as opaque manifests in `metadata.plugins[]`; the
// SDK-side runtime lets a Go test author register the plugin code and
// have it actually validate live payloads inside the in-process mock
// server.
//
// The SPI is intentionally narrow — the SDK is a thin client (see
// feedback_sdk_thin_layer.md): a plugin validates and matches; it does
// NOT embed a transport server. gRPC, for example, is matched at the
// payload level by [github.com/mockarty/mockarty-go/pact/plugins/grpc],
// not by spinning up a `google.golang.org/grpc` server.
//
// # Architecture
//
//	+------------------+         +------------------+
//	| pact.Consumer    | --DSL-->| pact.MockServer  |
//	|  WithPlugin(p)   |         |  serve()         |
//	+------------------+         +--------+---------+
//	                                       |
//	                                       v
//	                              +------------------+
//	                              | plugins.Registry |
//	                              | lookup by name   |
//	                              +--------+---------+
//	                                       |
//	                                       v
//	                              +------------------+
//	                              | Plugin.Match()   |
//	                              | per request body |
//	                              +------------------+
//
// # Concurrency
//
// Registry operations are safe for concurrent use. Plugins themselves
// MUST be implemented as immutable values once registered — the mock
// server calls Match on many goroutines in parallel.
//
// # Compatibility
//
// The pact_ffi Rust runtime is NOT a hard dependency. A grep across the
// SDK confirms zero `pact-ffi`/`libpact_ffi` references; the Go-side
// plugin runtime is independent and self-contained.
package plugins

import (
	"context"
	"errors"
)

// Plugin is the SPI every Pact V4 plugin implements. Plugins are
// registered once at program init and looked up by name at mock-server
// dispatch time.
//
// Implementations MUST be immutable after registration. Plugins that
// need per-test state should obtain it from MatchRequest's ctx (e.g.
// via context.Value) so the plugin object itself stays goroutine-safe.
type Plugin interface {
	// Name returns the manifest name written into `metadata.plugins[].name`.
	// Convention: lowercase, hyphen-separated, e.g. "protobuf", "grpc".
	Name() string

	// Version returns a semver-shaped version string. Recorded in
	// `metadata.plugins[].version` for broker auditing.
	Version() string

	// SupportedContentTypes lists the MIME content types this plugin
	// handles. The mock server consults this list to route incoming
	// requests by their Content-Type header.
	//
	// A plugin MAY return wildcards ("application/*") but the registry
	// matches by exact MIME first, then by `type/*`, then by `*/*`.
	SupportedContentTypes() []string

	// MatchRequest validates the wire-side request body against the
	// declared expectations carried in `expected`. The implementation
	// returns nil on match, or a structured error explaining the
	// mismatch (path, expected, actual) so the mock server can include
	// it in the 404 debug envelope.
	//
	// `expected` is the per-interaction configuration the consumer
	// supplied via Consumer.WithPlugin; the implementation interprets
	// it freely (typically a `map[string]any` parsed from JSON).
	MatchRequest(ctx context.Context, expected map[string]any, actual []byte, contentType string) *MatchError

	// GenerateResponse builds the response body the mock server returns
	// when the interaction matches. May return nil to signal "use the
	// declared response body unchanged"; this lets a plugin opt out of
	// dynamic response generation and rely on the pact author's
	// declared example.
	GenerateResponse(ctx context.Context, expected map[string]any, contentType string) ([]byte, error)
}

// MatchError is the structured failure surfaced by a plugin's
// MatchRequest. Field order keeps 16-byte string headers grouped to
// minimise struct padding.
type MatchError struct {
	// Cause is the underlying error (parse failure, type mismatch,
	// etc.). nil is treated as "no underlying error, just a logical
	// mismatch described by Reason".
	Cause error
	// Path is the JSON-pointer-style breadcrumb to the mismatching
	// field (`$.request.user.id`). Empty if the plugin matches at the
	// root payload level.
	Path string
	// Reason is the human-readable explanation surfaced in the mock
	// server's 404 debug envelope. Required.
	Reason string
}

// Error makes MatchError compatible with the error interface so plugin
// authors can return `&MatchError{...}` directly.
func (m *MatchError) Error() string {
	if m == nil {
		return ""
	}
	if m.Path == "" {
		if m.Cause != nil {
			return m.Reason + ": " + m.Cause.Error()
		}
		return m.Reason
	}
	if m.Cause != nil {
		return m.Path + ": " + m.Reason + ": " + m.Cause.Error()
	}
	return m.Path + ": " + m.Reason
}

// Unwrap exposes the underlying Cause for errors.Is/As traversal.
func (m *MatchError) Unwrap() error {
	if m == nil {
		return nil
	}
	return m.Cause
}

// ErrPluginNotFound is returned by Registry.Get when the requested name
// is not registered.
var ErrPluginNotFound = errors.New("pact: plugin not found")
