// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// AddWireMockStub registers a WireMock-format stub at runtime via the
// WireMock-compat admin API exposed at /__admin/mappings. The payload
// is anything JSON-serialisable that resolves to a WireMock mapping
// envelope ({"request": ..., "response": ...}) — a hand-rolled
// map[string]any, a generated DTO, or a JSON string.
//
// This is the canonical entry point for existing WireMock test bodies:
// the JSON shape they already build is accepted verbatim.
//
// Example:
//
//	err := c.AddWireMockStub(ctx, map[string]any{
//	    "request":  map[string]any{"method": "GET", "url": "/api/users/1"},
//	    "response": map[string]any{"status": 200, "body": `{"id":1}`},
//	})
func (m *MockartyContainer) AddWireMockStub(ctx context.Context, stub any) error {
	if stub == nil {
		return errors.New("mockartycontainer: stub must not be nil")
	}
	body, err := marshalStub(stub)
	if err != nil {
		return err
	}
	return m.post(ctx, m.WireMockURL()+"/mappings", body)
}

// AddMockartyMock registers a native Mockarty mock via the SDK-compat
// admin endpoint at /api/v1/mocks. Use this when the caller already
// has access to the Mockarty model.Mock shape (typed SDK builder,
// generated DTO, multi-protocol mock with gRPC/MCP/Kafka contexts).
//
// For raw WireMock JSON, prefer AddWireMockStub.
func (m *MockartyContainer) AddMockartyMock(ctx context.Context, mock any) error {
	if mock == nil {
		return errors.New("mockartycontainer: mock must not be nil")
	}
	body, err := marshalStub(mock)
	if err != nil {
		return err
	}
	return m.post(ctx, m.MockartyURL()+"/mocks", body)
}

// marshalStub converts user input into a JSON byte slice. Strings and
// []byte are passed through verbatim (caller already serialised); any
// other value is run through encoding/json.
func marshalStub(stub any) ([]byte, error) {
	switch v := stub.(type) {
	case []byte:
		if len(bytes.TrimSpace(v)) == 0 {
			return nil, errors.New("mockartycontainer: stub bytes are empty")
		}
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, errors.New("mockartycontainer: stub string is empty")
		}
		return []byte(s), nil
	default:
		b, err := json.Marshal(stub)
		if err != nil {
			return nil, fmt.Errorf("mockartycontainer: marshal stub: %w", err)
		}
		return b, nil
	}
}
