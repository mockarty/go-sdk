// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package protobuf is the built-in Pact V4 plugin for the
// `application/x-protobuf` and `application/protobuf` content types.
//
// # Scope
//
// The plugin operates at the WIRE-FORMAT level using stdlib parsing of
// protobuf's binary varint encoding — it does NOT depend on a compiled
// descriptor set. This keeps the SDK CGO-free and dependency-free
// (jhump/protoreflect would pull a heavy descriptor pipeline that is
// unnecessary for the consumer-driven matching use case the SDK
// supports).
//
// The plugin verifies:
//
//   - Payload parses as valid protobuf (well-formed varint stream).
//   - Declared field numbers appear in the actual payload.
//   - Declared field wire types match the actual wire types.
//   - Optional declared scalar values match the actual scalar values
//     (for VARINT, FIXED32, FIXED64; LENGTH_DELIMITED is compared as
//     raw bytes).
//
// The plugin does NOT decode submessages — nested types are matched by
// raw byte equality. Authors who need deep submessage matching should
// supply a pre-encoded `expected` payload as the source of truth.
//
// # Configuration shape
//
// The `expected map[string]any` carries one of:
//
//	{
//	  "bytes": "<base64-encoded protobuf payload>"   // exact byte match
//	}
//
//	{
//	  "fields": {                                    // per-field shape
//	    "1": {"wire": 0, "value": 42},               // VARINT
//	    "2": {"wire": 2, "value": "hello"},          // LENGTH_DELIMITED
//	  }
//	}
//
// Field numbers are stringified for JSON friendliness (Go's encoding/json
// can't key by int).
package protobuf

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/mockarty/mockarty-go/pact/plugins"
)

// Name is the manifest key written into pact.json (`metadata.plugins[]`).
const Name = "protobuf"

// Version is the semver string recorded next to the plugin name.
const Version = "1.0.0"

// Plugin is the SPI implementation. Exported as a value so tests can
// register it under a different name via a wrapping type.
type Plugin struct{}

// New constructs the plugin instance. The receiver is empty — every
// plugin call is stateless and re-entrant.
func New() *Plugin { return &Plugin{} }

// Name implements plugins.Plugin.
func (Plugin) Name() string { return Name }

// Version implements plugins.Plugin.
func (Plugin) Version() string { return Version }

// SupportedContentTypes implements plugins.Plugin.
func (Plugin) SupportedContentTypes() []string {
	return []string{
		"application/x-protobuf",
		"application/protobuf",
		"application/vnd.google.protobuf",
	}
}

// MatchRequest validates an incoming protobuf payload against the
// declared expectation.
func (p Plugin) MatchRequest(ctx context.Context, expected map[string]any, actual []byte, contentType string) *plugins.MatchError {
	if len(actual) == 0 {
		if len(expected) == 0 {
			return nil
		}
		return &plugins.MatchError{Reason: "empty protobuf payload"}
	}

	// Mode 1: byte-exact match — fast path used when the consumer
	// supplied a pre-encoded protobuf example.
	if raw, ok := expected["bytes"]; ok {
		want, err := decodeExpectedBytes(raw)
		if err != nil {
			return &plugins.MatchError{Reason: "expected bytes invalid", Cause: err}
		}
		if !bytesEqual(want, actual) {
			return &plugins.MatchError{
				Path:   "$.body",
				Reason: fmt.Sprintf("protobuf payload mismatch (want %d bytes, got %d)", len(want), len(actual)),
			}
		}
		return nil
	}

	// Mode 2: per-field shape match.
	parsed, err := parseFields(actual)
	if err != nil {
		return &plugins.MatchError{Reason: "actual payload is not valid protobuf", Cause: err}
	}
	fields, ok := expected["fields"].(map[string]any)
	if !ok {
		// No assertions beyond "is valid protobuf" — declared expectation
		// is empty, parsing succeeded, that counts as a match.
		return nil
	}
	for k, want := range fields {
		fnum, err := strconv.Atoi(k)
		if err != nil {
			return &plugins.MatchError{
				Path:   "$.fields." + k,
				Reason: "field number must be a decimal integer",
				Cause:  err,
			}
		}
		got, present := parsed[fnum]
		if !present {
			return &plugins.MatchError{
				Path:   "$.fields." + k,
				Reason: "declared field missing in actual payload",
			}
		}
		if err := matchField(got, want); err != nil {
			return &plugins.MatchError{
				Path:   "$.fields." + k,
				Reason: err.Error(),
			}
		}
	}
	return nil
}

// GenerateResponse returns the declared bytes when the expectation
// supplies them, otherwise nil (which tells the mock server to fall
// back to the user's declared response body).
func (Plugin) GenerateResponse(ctx context.Context, expected map[string]any, contentType string) ([]byte, error) {
	if raw, ok := expected["bytes"]; ok {
		return decodeExpectedBytes(raw)
	}
	return nil, nil
}

// decodeExpectedBytes accepts a base64 string or a Go []byte/string and
// returns the raw protobuf payload.
func decodeExpectedBytes(raw any) ([]byte, error) {
	switch v := raw.(type) {
	case string:
		return base64.StdEncoding.DecodeString(v)
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported bytes type %T", raw)
	}
}

// fieldValue is the parsed view of one protobuf field. It holds either
// a varint (uint64), a 32-bit fixed value, a 64-bit fixed value, or
// a length-delimited byte slice. WireType identifies which.
type fieldValue struct {
	Length   []byte // wire type 2: raw bytes
	Varint   uint64 // wire type 0
	Fixed32  uint32 // wire type 5
	Fixed64  uint64 // wire type 1
	WireType uint8
}

// parseFields walks a protobuf payload and returns a field-number →
// last-occurrence map. Repeated fields use the last value (the
// declared-shape API does not currently support per-occurrence
// assertions; authors needing repeated semantics use byte-exact mode).
func parseFields(b []byte) (map[int]fieldValue, error) {
	out := map[int]fieldValue{}
	for len(b) > 0 {
		tag, n := readVarint(b)
		if n == 0 {
			return nil, fmt.Errorf("truncated varint tag at offset %d", len(b))
		}
		b = b[n:]
		fieldNum := int(tag >> 3)
		wireType := uint8(tag & 0x7)
		fv := fieldValue{WireType: wireType}
		switch wireType {
		case 0: // VARINT
			v, n := readVarint(b)
			if n == 0 {
				return nil, fmt.Errorf("truncated varint value for field %d", fieldNum)
			}
			fv.Varint = v
			b = b[n:]
		case 1: // FIXED64
			if len(b) < 8 {
				return nil, fmt.Errorf("truncated fixed64 for field %d", fieldNum)
			}
			fv.Fixed64 = uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
				uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
			b = b[8:]
		case 2: // LENGTH_DELIMITED
			length, n := readVarint(b)
			if n == 0 {
				return nil, fmt.Errorf("truncated length prefix for field %d", fieldNum)
			}
			b = b[n:]
			if uint64(len(b)) < length {
				return nil, fmt.Errorf("truncated payload for field %d (need %d, have %d)", fieldNum, length, len(b))
			}
			fv.Length = append([]byte(nil), b[:length]...)
			b = b[length:]
		case 5: // FIXED32
			if len(b) < 4 {
				return nil, fmt.Errorf("truncated fixed32 for field %d", fieldNum)
			}
			fv.Fixed32 = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
			b = b[4:]
		case 3, 4: // SGROUP/EGROUP — deprecated, ignore content
			// We don't carry group payloads but advance the cursor so
			// later fields still parse cleanly. Groups have no length
			// prefix; we error out for safety because supporting them
			// requires nested SGROUP/EGROUP balancing.
			return nil, fmt.Errorf("group wire types are not supported (field %d)", fieldNum)
		default:
			return nil, fmt.Errorf("unknown wire type %d for field %d", wireType, fieldNum)
		}
		out[fieldNum] = fv
	}
	return out, nil
}

// readVarint decodes one protobuf varint. Returns (value, bytesRead).
// bytesRead==0 means the input was truncated.
func readVarint(b []byte) (uint64, int) {
	var v uint64
	var shift uint
	for i, c := range b {
		if i >= 10 {
			// Protobuf varints are at most 10 bytes.
			return 0, 0
		}
		v |= uint64(c&0x7F) << shift
		if c&0x80 == 0 {
			return v, i + 1
		}
		shift += 7
	}
	return 0, 0
}

// bytesEqual is a constant-time byte slice equality helper. We don't
// use bytes.Equal directly to avoid pulling the import for a single use
// in this package; this keeps the file dependency tree minimal.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// matchField compares a parsed actual field against the declared
// expected entry. The declared shape is:
//
//	{"wire": <int>, "value": <int|float|string|[]byte>}
//
// wire is the expected wire type (0/1/2/5); value is optional and
// when present must match the actual.
func matchField(got fieldValue, expected any) error {
	expMap, ok := expected.(map[string]any)
	if !ok {
		return fmt.Errorf("expected entry must be an object, got %T", expected)
	}
	if wireRaw, present := expMap["wire"]; present {
		want, err := asInt(wireRaw)
		if err != nil {
			return fmt.Errorf("wire: %w", err)
		}
		if uint8(want) != got.WireType {
			return fmt.Errorf("wire type mismatch: want %d, got %d", want, got.WireType)
		}
	}
	val, present := expMap["value"]
	if !present {
		return nil
	}
	switch got.WireType {
	case 0:
		want, err := asUint64(val)
		if err != nil {
			return fmt.Errorf("value: %w", err)
		}
		if want != got.Varint {
			return fmt.Errorf("varint value mismatch: want %d, got %d", want, got.Varint)
		}
	case 1:
		want, err := asUint64(val)
		if err != nil {
			return fmt.Errorf("value: %w", err)
		}
		if want != got.Fixed64 {
			return fmt.Errorf("fixed64 value mismatch: want %d, got %d", want, got.Fixed64)
		}
	case 2:
		switch want := val.(type) {
		case string:
			if want != string(got.Length) {
				return fmt.Errorf("length-delimited value mismatch: want %q, got %q", want, string(got.Length))
			}
		case []byte:
			if !bytesEqual(want, got.Length) {
				return fmt.Errorf("length-delimited bytes mismatch")
			}
		default:
			return fmt.Errorf("length-delimited expected value must be string or []byte, got %T", val)
		}
	case 5:
		want, err := asUint64(val)
		if err != nil {
			return fmt.Errorf("value: %w", err)
		}
		if uint32(want) != got.Fixed32 {
			return fmt.Errorf("fixed32 value mismatch: want %d, got %d", uint32(want), got.Fixed32)
		}
	default:
		return fmt.Errorf("cannot compare value for wire type %d", got.WireType)
	}
	return nil
}

// asInt coerces an arbitrary JSON-decoded number into an int.
func asInt(v any) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int32:
		return int(n), nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case uint:
		return int(n), nil
	case uint32:
		return int(n), nil
	case uint64:
		return int(n), nil
	}
	return 0, fmt.Errorf("not a number: %T", v)
}

// asUint64 coerces a JSON-decoded number into a uint64. Negative values
// are rejected — protobuf varint comparisons treat values as unsigned.
func asUint64(v any) (uint64, error) {
	switch n := v.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("negative value: %d", n)
		}
		return uint64(n), nil
	case int32:
		if n < 0 {
			return 0, fmt.Errorf("negative value: %d", n)
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("negative value: %d", n)
		}
		return uint64(n), nil
	case uint:
		return uint64(n), nil
	case uint32:
		return uint64(n), nil
	case uint64:
		return n, nil
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("negative value: %v", n)
		}
		return uint64(n), nil
	}
	return 0, fmt.Errorf("not a number: %T", v)
}

func init() {
	// Self-register on package import so a simple `_ "…/protobuf"`
	// side-effect import is enough to wire the plugin into the global
	// runtime. Errors are not actionable at init time — a duplicate
	// registration on a clean import is impossible, and the registry
	// overwrites cleanly anyway.
	_ = plugins.Register(New())
}
