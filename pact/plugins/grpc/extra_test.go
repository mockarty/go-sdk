// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package grpc_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/mockarty/mockarty-go/pact/plugins/grpc"
)

func TestGRPCGenerateResponseRespectsInnerNil(t *testing.T) {
	t.Parallel()
	// response.bytes missing -> inner protobuf returns nil bytes ->
	// outer grpc also returns nil (no framing).
	p := grpc.New()
	out, err := p.GenerateResponse(context.Background(), map[string]any{
		"response": map[string]any{"fields": map[string]any{}},
	}, "application/grpc")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out != nil {
		t.Fatalf("nil inner must propagate nil outer, got %d bytes", len(out))
	}
}

func TestGRPCFramedTooShort(t *testing.T) {
	t.Parallel()
	p := grpc.New()
	// header says 100-byte payload, only 1 byte follows.
	bad := []byte{0, 0, 0, 0, 100, 0x08}
	if err := p.MatchRequest(context.Background(), map[string]any{}, bad, "application/grpc"); err == nil {
		t.Fatalf("declared length > available must fail")
	}
}

func TestGRPCRerootPathVariants(t *testing.T) {
	t.Parallel()
	// Drive both rerootPath branches via tampered payloads — the
	// MatchError path string differs based on whether the inner error
	// arose at $.body (byte-exact) vs $.fields (per-field).
	p := grpc.New()

	// Exact-byte mode mismatch.
	good := []byte{0x08, 0x07}
	frameGood := gframe(good)
	frameBad := append([]byte(nil), frameGood...)
	frameBad[len(frameBad)-1] ^= 0xFF
	if err := p.MatchRequest(context.Background(), map[string]any{
		"request": map[string]any{"bytes": base64.StdEncoding.EncodeToString(good)},
	}, frameBad, "application/grpc"); err == nil {
		t.Fatalf("byte-exact mismatch must surface error")
	}

	// Field-mode mismatch — declared wire 2, actual wire 0.
	if err := p.MatchRequest(context.Background(), map[string]any{
		"request": map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 2}}},
	}, frameGood, "application/grpc"); err == nil {
		t.Fatalf("field-mode mismatch must surface error")
	}
}
