// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package protobuf_test

import (
	"context"
	"testing"

	"github.com/mockarty/mockarty-go/pact/plugins/protobuf"
)

// These tests drive the integer-coercion helpers in the protobuf
// plugin so every Go numeric type is exercised across both `wire` and
// `value` slots.

func TestProtobufVersionExposed(t *testing.T) {
	t.Parallel()
	if protobuf.Version == "" {
		t.Fatalf("Version constant must not be empty")
	}
	p := protobuf.New()
	if p.Version() == "" {
		t.Fatalf("Version() method must mirror constant")
	}
}

func TestProtobufWireValueTypeCoercion(t *testing.T) {
	t.Parallel()
	// VARINT field 1 = 42, wire types come in via int / float64 / int32.
	payload := makeVarint(1, 42)
	p := protobuf.New()
	cases := []map[string]any{
		{"fields": map[string]any{"1": map[string]any{"wire": int32(0), "value": int32(42)}}},
		{"fields": map[string]any{"1": map[string]any{"wire": int64(0), "value": int64(42)}}},
		{"fields": map[string]any{"1": map[string]any{"wire": float64(0), "value": float64(42)}}},
		{"fields": map[string]any{"1": map[string]any{"wire": uint(0), "value": uint(42)}}},
		{"fields": map[string]any{"1": map[string]any{"wire": uint32(0), "value": uint32(42)}}},
		{"fields": map[string]any{"1": map[string]any{"wire": uint64(0), "value": uint64(42)}}},
	}
	for i, c := range cases {
		if err := p.MatchRequest(context.Background(), c, payload, "application/x-protobuf"); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
	}
}

func TestProtobufNegativeWireRejected(t *testing.T) {
	t.Parallel()
	payload := makeVarint(1, 42)
	p := protobuf.New()
	// wire string is not a number — coercion failure.
	c := map[string]any{"fields": map[string]any{"1": map[string]any{"wire": "zero"}}}
	if err := p.MatchRequest(context.Background(), c, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("string wire must error")
	}
	// negative value via float64.
	c2 := map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 0, "value": float64(-1)}}}
	if err := p.MatchRequest(context.Background(), c2, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("negative value must error")
	}
}

func TestProtobufExpectedFieldNotObject(t *testing.T) {
	t.Parallel()
	payload := makeVarint(1, 42)
	p := protobuf.New()
	// Per-field declaration must be an object — string is rejected.
	bad := map[string]any{"fields": map[string]any{"1": "scalar-not-object"}}
	if err := p.MatchRequest(context.Background(), bad, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("non-object field must error")
	}
}

func TestProtobufWireMismatchPath(t *testing.T) {
	t.Parallel()
	payload := makeVarint(1, 42)
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 2}}}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("wire type mismatch should fail")
	}
}

func TestProtobufLenDelimBytesValue(t *testing.T) {
	t.Parallel()
	body := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	payload := makeLenDelim(3, body)
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"3": map[string]any{"wire": 2, "value": body}}}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err != nil {
		t.Fatalf("len-delim bytes value: %v", err)
	}
	// Wrong bytes.
	cfg2 := map[string]any{"fields": map[string]any{"3": map[string]any{"wire": 2, "value": []byte{0x00}}}}
	if err := p.MatchRequest(context.Background(), cfg2, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("wrong bytes must fail")
	}
	// Wrong value type.
	cfg3 := map[string]any{"fields": map[string]any{"3": map[string]any{"wire": 2, "value": 123}}}
	if err := p.MatchRequest(context.Background(), cfg3, payload, "application/x-protobuf"); err == nil {
		t.Fatalf("non-string/bytes value must fail")
	}
}

func TestProtobufGroupWireTypeRejected(t *testing.T) {
	t.Parallel()
	// Tag for field 1 wire-type 3 (SGROUP — deprecated).
	bad := []byte{(1 << 3) | 3, 0}
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 3}}}
	if err := p.MatchRequest(context.Background(), cfg, bad, "application/x-protobuf"); err == nil {
		t.Fatalf("group wire type must fail")
	}
}

func TestProtobufUnknownWireTypeRejected(t *testing.T) {
	t.Parallel()
	// Wire-type 6 doesn't exist.
	bad := []byte{(1 << 3) | 6, 0}
	p := protobuf.New()
	if err := p.MatchRequest(context.Background(), map[string]any{}, bad, "application/x-protobuf"); err == nil {
		t.Fatalf("unknown wire type must fail")
	}
}

func TestProtobufFixed64ValueMismatch(t *testing.T) {
	t.Parallel()
	fixed64 := []byte{(6 << 3) | 1, 0x08, 0, 0, 0, 0, 0, 0, 0}
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"6": map[string]any{"wire": 1, "value": 99}}}
	if err := p.MatchRequest(context.Background(), cfg, fixed64, "application/x-protobuf"); err == nil {
		t.Fatalf("fixed64 value mismatch must fail")
	}
}

func TestProtobufFixed32ValueMismatch(t *testing.T) {
	t.Parallel()
	fixed32 := []byte{(5 << 3) | 5, 0x78, 0x56, 0x34, 0x12}
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"5": map[string]any{"wire": 5, "value": 0}}}
	if err := p.MatchRequest(context.Background(), cfg, fixed32, "application/x-protobuf"); err == nil {
		t.Fatalf("fixed32 value mismatch must fail")
	}
}

func TestProtobufDecodeBytesFromByteSlice(t *testing.T) {
	t.Parallel()
	// Pass []byte directly (not base64) — accepted by decodeExpectedBytes.
	p := protobuf.New()
	payload := makeVarint(1, 1)
	cfg := map[string]any{"bytes": payload}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err != nil {
		t.Fatalf("[]byte bytes path: %v", err)
	}
}

func TestProtobufDecodeBytesRejectsUnsupportedType(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	cfg := map[string]any{"bytes": 12345}
	if err := p.MatchRequest(context.Background(), cfg, []byte{0x08, 0x01}, "application/x-protobuf"); err == nil {
		t.Fatalf("unsupported bytes type must error")
	}
}

func TestProtobufFieldShapeNoFieldsKey(t *testing.T) {
	t.Parallel()
	// Empty expected fields == match-any-well-formed-payload.
	p := protobuf.New()
	payload := makeVarint(1, 1)
	cfg := map[string]any{"fields": "not-an-object"}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err != nil {
		t.Fatalf("non-object fields treated as no-assertion: %v", err)
	}
}

func TestProtobufTruncatedLenDelim(t *testing.T) {
	t.Parallel()
	// Tag for field 2 wire-type 2, length=5, but only 2 bytes follow.
	bad := []byte{(2 << 3) | 2, 5, 'h', 'i'}
	p := protobuf.New()
	cfg := map[string]any{"fields": map[string]any{"2": map[string]any{"wire": 2}}}
	if err := p.MatchRequest(context.Background(), cfg, bad, "application/x-protobuf"); err == nil {
		t.Fatalf("truncated length-delim must fail")
	}
}

func TestProtobufTruncatedFixed(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	// fixed32 needs 4 bytes; supply 2.
	bad := []byte{(5 << 3) | 5, 0x01, 0x02}
	if err := p.MatchRequest(context.Background(), map[string]any{}, bad, "application/x-protobuf"); err == nil {
		t.Fatalf("truncated fixed32 must fail")
	}
	// fixed64 needs 8.
	bad64 := []byte{(6 << 3) | 1, 0x01, 0x02, 0x03}
	if err := p.MatchRequest(context.Background(), map[string]any{}, bad64, "application/x-protobuf"); err == nil {
		t.Fatalf("truncated fixed64 must fail")
	}
}
