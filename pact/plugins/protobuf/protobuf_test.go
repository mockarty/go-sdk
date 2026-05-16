// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package protobuf_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/mockarty/mockarty-go/pact/plugins/protobuf"
)

// makeVarint produces the byte sequence for one VARINT tag+value.
func makeVarint(field int, value uint64) []byte {
	tag := uint64(field)<<3 | 0 // wire type 0
	out := []byte{}
	out = appendVarint(out, tag)
	out = appendVarint(out, value)
	return out
}

// makeLenDelim produces a wire-type-2 length-delimited field.
func makeLenDelim(field int, body []byte) []byte {
	tag := uint64(field)<<3 | 2
	out := appendVarint(nil, tag)
	out = appendVarint(out, uint64(len(body)))
	out = append(out, body...)
	return out
}

func appendVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	dst = append(dst, byte(v))
	return dst
}

func TestProtobufRegistersOnImport(t *testing.T) {
	t.Parallel()
	if protobuf.Name == "" {
		t.Fatalf("Name constant empty")
	}
}

func TestProtobufBytesExactMatch(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	payload := append(makeVarint(1, 42), makeLenDelim(2, []byte("hi"))...)
	cfg := map[string]any{"bytes": base64.StdEncoding.EncodeToString(payload)}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err != nil {
		t.Fatalf("byte-exact mismatch: %v", err)
	}
	other := append(payload, 0)
	if err := p.MatchRequest(context.Background(), cfg, other, "application/x-protobuf"); err == nil {
		t.Fatalf("byte-exact must reject differing payloads")
	}
}

func TestProtobufFieldShapeMatch(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	payload := append(makeVarint(1, 42), makeLenDelim(2, []byte("hello"))...)
	cfg := map[string]any{
		"fields": map[string]any{
			"1": map[string]any{"wire": 0, "value": 42},
			"2": map[string]any{"wire": 2, "value": "hello"},
		},
	}
	if err := p.MatchRequest(context.Background(), cfg, payload, "application/x-protobuf"); err != nil {
		t.Fatalf("field-shape mismatch: %v", err)
	}
}

func TestProtobufFieldShapeMismatches(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	payload := append(makeVarint(1, 42), makeLenDelim(2, []byte("hello"))...)
	cases := []struct {
		cfg map[string]any
		why string
	}{
		{map[string]any{"fields": map[string]any{"3": map[string]any{"wire": 0, "value": 1}}}, "missing field"},
		{map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 0, "value": 99}}}, "wrong value"},
		{map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 2}}}, "wrong wire type"},
		{map[string]any{"fields": map[string]any{"abc": map[string]any{"wire": 0}}}, "non-numeric field key"},
		{map[string]any{"fields": map[string]any{"2": map[string]any{"wire": 2, "value": "goodbye"}}}, "wrong string"},
	}
	for _, c := range cases {
		if err := p.MatchRequest(context.Background(), c.cfg, payload, "application/x-protobuf"); err == nil {
			t.Fatalf("%s: should fail", c.why)
		}
	}
}

func TestProtobufEmptyPayloads(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	if err := p.MatchRequest(context.Background(), nil, nil, "application/x-protobuf"); err != nil {
		t.Fatalf("empty/empty must match: %v", err)
	}
	if err := p.MatchRequest(context.Background(), map[string]any{"bytes": "AA=="}, nil, "application/x-protobuf"); err == nil {
		t.Fatalf("empty actual vs declared bytes must fail")
	}
}

func TestProtobufRejectsMalformed(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	// Truncated varint payload — single byte with continuation bit set.
	bad := []byte{0xFF}
	cfg := map[string]any{"fields": map[string]any{"1": map[string]any{"wire": 0}}}
	if err := p.MatchRequest(context.Background(), cfg, bad, "application/x-protobuf"); err == nil {
		t.Fatalf("malformed payload must fail")
	}
}

func TestProtobufGenerateResponse(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	payload := makeVarint(1, 7)
	cfg := map[string]any{"bytes": base64.StdEncoding.EncodeToString(payload)}
	got, err := p.GenerateResponse(context.Background(), cfg, "application/x-protobuf")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("response length mismatch: %d vs %d", len(got), len(payload))
	}
	// Without bytes → nil signals "use declared body".
	out, err := p.GenerateResponse(context.Background(), map[string]any{}, "application/x-protobuf")
	if err != nil {
		t.Fatalf("empty config generate: %v", err)
	}
	if out != nil {
		t.Fatalf("empty config must return nil")
	}
}

func TestProtobufBytesInvalidBase64(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	if err := p.MatchRequest(context.Background(), map[string]any{"bytes": "!!!notb64"}, []byte{0x08, 0x2A}, "application/x-protobuf"); err == nil {
		t.Fatalf("invalid base64 must fail")
	}
}

func TestProtobufSupportedContentTypes(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	if got := p.SupportedContentTypes(); len(got) < 1 {
		t.Fatalf("must declare at least one content type")
	}
}

func TestProtobufWireTypeFixed32And64(t *testing.T) {
	t.Parallel()
	p := protobuf.New()
	// Tag for field 5 wire-type 5 (FIXED32) + 4 byte payload (LE).
	fixed32 := []byte{(5 << 3) | 5, 0x78, 0x56, 0x34, 0x12}
	cfg := map[string]any{"fields": map[string]any{"5": map[string]any{"wire": 5, "value": 0x12345678}}}
	if err := p.MatchRequest(context.Background(), cfg, fixed32, "application/x-protobuf"); err != nil {
		t.Fatalf("fixed32: %v", err)
	}
	// Tag for field 6 wire-type 1 (FIXED64) + 8 byte payload (LE).
	fixed64 := []byte{(6 << 3) | 1, 0x08, 0, 0, 0, 0, 0, 0, 0}
	cfg = map[string]any{"fields": map[string]any{"6": map[string]any{"wire": 1, "value": 8}}}
	if err := p.MatchRequest(context.Background(), cfg, fixed64, "application/x-protobuf"); err != nil {
		t.Fatalf("fixed64: %v", err)
	}
}

func FuzzProtobufParserDoesNotPanic(f *testing.F) {
	f.Add([]byte{0x08, 0x2A})
	f.Add([]byte{0x12, 0x05, 'h', 'e', 'l', 'l', 'o'})
	f.Add([]byte{0xFF, 0xFF, 0xFF})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, payload []byte) {
		p := protobuf.New()
		// Either succeeds or returns a structured error; must never panic.
		_ = p.MatchRequest(context.Background(), map[string]any{
			"fields": map[string]any{"1": map[string]any{"wire": 0}},
		}, payload, "application/x-protobuf")
	})
}
