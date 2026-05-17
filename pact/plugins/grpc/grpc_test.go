// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package grpc_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"testing"

	"github.com/mockarty/mockarty-go/pact/plugins/grpc"
)

// gframe wraps payload into a gRPC length-prefixed frame.
func gframe(payload []byte) []byte {
	out := make([]byte, 5+len(payload))
	out[0] = 0
	binary.BigEndian.PutUint32(out[1:5], uint32(len(payload)))
	copy(out[5:], payload)
	return out
}

// protoVarint produces a wire-type-0 protobuf field for testing.
// The protobuf tag layout is `(field_number << 3) | wire_type`; for varints
// wire_type = 0, so the OR-with-zero collapses to a pure left shift.
func protoVarint(field int, value uint64) []byte {
	out := []byte{}
	tag := uint64(field) << 3 // wire type 0 (varint) — OR omitted because it's zero
	for tag >= 0x80 {
		out = append(out, byte(tag)|0x80)
		tag >>= 7
	}
	out = append(out, byte(tag))
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	out = append(out, byte(value))
	return out
}

func TestGRPCEmptyExpectationMatchesValidFrame(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	payload := protoVarint(1, 42)
	frame := gframe(payload)
	if err := p.MatchRequest(context.Background(), map[string]any{}, frame, "application/grpc"); err != nil {
		t.Fatalf("valid frame with empty expectation must match: %v", err)
	}
}

func TestGRPCRejectsTruncatedFrame(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	if err := p.MatchRequest(context.Background(), map[string]any{}, []byte{0, 0, 0}, "application/grpc"); err == nil {
		t.Fatalf("truncated frame must fail")
	}
}

func TestGRPCDelegatesToProtobuf(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	payload := protoVarint(1, 99)
	frame := gframe(payload)
	cfg := map[string]any{
		"service": "Greeter",
		"method":  "SayHello",
		"request": map[string]any{
			"bytes": base64.StdEncoding.EncodeToString(payload),
		},
	}
	if err := p.MatchRequest(context.Background(), cfg, frame, "application/grpc"); err != nil {
		t.Fatalf("inner protobuf match should succeed: %v", err)
	}
	// Tamper the payload — inner match must surface a `$.grpc.*` path.
	tampered := append([]byte(nil), frame...)
	tampered[len(tampered)-1] ^= 0xFF
	if err := p.MatchRequest(context.Background(), cfg, tampered, "application/grpc"); err == nil {
		t.Fatalf("tampered payload must fail")
	}
}

func TestGRPCGenerateResponseFraming(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	payload := protoVarint(1, 7)
	cfg := map[string]any{"response": map[string]any{"bytes": base64.StdEncoding.EncodeToString(payload)}}
	out, err := p.GenerateResponse(context.Background(), cfg, "application/grpc")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(out) != 5+len(payload) {
		t.Fatalf("response not framed: len=%d", len(out))
	}
	// Empty config -> nil
	out, err = p.GenerateResponse(context.Background(), map[string]any{}, "application/grpc")
	if err != nil || out != nil {
		t.Fatalf("empty cfg should yield nil/nil; got %v err=%v", out, err)
	}
}

func TestGRPCSupportedContentTypes(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	cts := p.SupportedContentTypes()
	want := map[string]bool{"application/grpc": false, "application/grpc-web": false}
	for _, ct := range cts {
		if _, ok := want[ct]; ok {
			want[ct] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Fatalf("missing content type %q", k)
		}
	}
}

func TestGRPCMetadata(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	if p.Name() == "" || p.Version() == "" {
		t.Fatalf("metadata empty")
	}
}

func TestGRPCInvalidCompressedFlag(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	frame := gframe(protoVarint(1, 1))
	frame[0] = 2 // illegal flag
	if err := p.MatchRequest(context.Background(), map[string]any{}, frame, "application/grpc"); err == nil {
		t.Fatalf("invalid compressed flag must fail")
	}
}

func TestGRPCEmptyPayloadEmptyExpectation(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	if err := p.MatchRequest(context.Background(), nil, nil, "application/grpc"); err != nil {
		t.Fatalf("empty/empty: %v", err)
	}
}
