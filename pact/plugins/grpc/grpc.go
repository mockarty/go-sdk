// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package grpc is the built-in Pact V4 plugin for the gRPC transport.
//
// # Scope
//
// The plugin operates at the WIRE-FORMAT level: it validates a gRPC
// length-prefixed frame (1-byte compressed flag + 4-byte big-endian
// payload length + payload bytes) and delegates the payload itself to
// the [protobuf] plugin. Embedding a real gRPC server inside the SDK is
// explicitly out of scope (see feedback_sdk_thin_layer.md) — that is
// Phase 2 territory if a customer actually needs HTTP/2 framing.
//
// The plugin's `expected` shape mirrors protobuf, with an extra field
// for service/method context that round-trips into pact.json:
//
//	{
//	  "service": "Greeter",
//	  "method":  "SayHello",
//	  "request": { ... protobuf expected ... },
//	  "response":{ ... protobuf expected ... }
//	}
//
// Authors can omit `request`/`response` to assert "any payload as long
// as it is well-formed gRPC". This is useful for transport-level
// integration tests that already verify the message body in a separate
// protobuf interaction.
package grpc

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/mockarty/mockarty-go/pact/plugins"
	"github.com/mockarty/mockarty-go/pact/plugins/protobuf"
)

// Name is the manifest key written into pact.json.
const Name = "grpc"

// Version is the semver string recorded next to the plugin name.
const Version = "1.0.0"

// Plugin is the SPI implementation.
type Plugin struct {
	inner *protobuf.Plugin
}

// New constructs the plugin instance. It owns a private Protobuf
// instance for payload-body matching; we don't reach into the global
// registry so test-scope unregistration of `protobuf` cannot
// accidentally break gRPC matching.
func New() *Plugin {
	return &Plugin{inner: protobuf.New()}
}

// Name implements plugins.Plugin.
func (Plugin) Name() string { return Name }

// Version implements plugins.Plugin.
func (Plugin) Version() string { return Version }

// SupportedContentTypes implements plugins.Plugin.
//
// gRPC over HTTP/2 conventionally uses `application/grpc` (and
// `application/grpc+proto`); HTTP/1.1 gRPC-Web uses `application/grpc-web`.
// We accept all three so the same plugin handles both transports.
func (Plugin) SupportedContentTypes() []string {
	return []string{
		"application/grpc",
		"application/grpc+proto",
		"application/grpc-web",
		"application/grpc-web+proto",
	}
}

// MatchRequest validates a gRPC frame.
func (p Plugin) MatchRequest(ctx context.Context, expected map[string]any, actual []byte, contentType string) *plugins.MatchError {
	if len(actual) == 0 {
		if len(expected) == 0 {
			return nil
		}
		return &plugins.MatchError{Reason: "empty gRPC frame"}
	}
	payload, err := unframe(actual)
	if err != nil {
		return &plugins.MatchError{Reason: "malformed gRPC frame", Cause: err}
	}
	// Service/method assertions are informational — they round-trip
	// into the pact.json for a downstream verifier but we cannot
	// enforce them here without HTTP/2 :path header access. The mock
	// server stashes them on the interaction headers instead, so a
	// caller wiring real HTTP/2 traffic will see the mismatch through
	// the header layer.
	req, _ := expected["request"].(map[string]any)
	if len(req) == 0 {
		return nil
	}
	if me := p.inner.MatchRequest(ctx, req, payload, "application/x-protobuf"); me != nil {
		// Re-root the path so the user sees `$.grpc.body` rather than
		// the protobuf plugin's `$.body`. This keeps the trail meaningful
		// when both plugins co-validate a payload.
		me.Path = rerootPath(me.Path)
		return me
	}
	return nil
}

// GenerateResponse builds a gRPC-framed response from the declared
// payload. If the declaration omits `response.bytes` we return nil to
// let the mock server fall back to its declared response body.
func (p Plugin) GenerateResponse(ctx context.Context, expected map[string]any, contentType string) ([]byte, error) {
	resp, _ := expected["response"].(map[string]any)
	if len(resp) == 0 {
		return nil, nil
	}
	body, err := p.inner.GenerateResponse(ctx, resp, "application/x-protobuf")
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, nil
	}
	return frame(body), nil
}

// frame wraps raw payload bytes in a single gRPC length-prefixed frame.
// We always emit uncompressed frames (`compressed` flag = 0).
func frame(payload []byte) []byte {
	out := make([]byte, 5+len(payload))
	out[0] = 0 // compressed flag
	binary.BigEndian.PutUint32(out[1:5], uint32(len(payload)))
	copy(out[5:], payload)
	return out
}

// unframe peels off the 5-byte gRPC prefix and returns the payload. It
// fails fast on malformed input rather than returning a partial payload.
func unframe(b []byte) ([]byte, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("frame shorter than 5-byte header: %d bytes", len(b))
	}
	compressed := b[0]
	if compressed != 0 && compressed != 1 {
		return nil, fmt.Errorf("invalid compressed flag %d", compressed)
	}
	length := binary.BigEndian.Uint32(b[1:5])
	body := b[5:]
	if uint32(len(body)) < length {
		return nil, fmt.Errorf("declared length %d exceeds remaining bytes %d", length, len(body))
	}
	return body[:length], nil
}

// rerootPath rewrites a protobuf-rooted path under a `$.grpc.body.*`
// breadcrumb so the user can tell which transport surfaced the
// mismatch.
func rerootPath(in string) string {
	const protobufRoot = "$.body"
	const protobufFields = "$.fields"
	if len(in) == 0 {
		return "$.grpc.body"
	}
	if len(in) >= len(protobufRoot) && in[:len(protobufRoot)] == protobufRoot {
		return "$.grpc.body" + in[len(protobufRoot):]
	}
	if len(in) >= len(protobufFields) && in[:len(protobufFields)] == protobufFields {
		return "$.grpc.body.fields" + in[len(protobufFields):]
	}
	return "$.grpc." + in
}

func init() {
	_ = plugins.Register(New())
}
