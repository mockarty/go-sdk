// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"testing"
)

// TestSaveMockResponse_DecodesCanonicalWireShape proves the SDK decodes
// the actual admin-node wire shape (envelope with `id`, `isNew`, `mock`,
// `success`, `message`) into all fields. Surfaced 2026-05-17 by the live
// Go-SDK demo where Overwritten was always false because the server
// emits `isNew`, not `overwritten`.
func TestSaveMockResponse_DecodesCanonicalWireShape(t *testing.T) {
	payload := []byte(`{
		"id": "mock_1779037659",
		"mock": {"id": "mock_1779037659", "namespace": "sandbox"},
		"isNew": true,
		"success": true,
		"message": "Mock created successfully"
	}`)

	var got SaveMockResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.ID != "mock_1779037659" {
		t.Errorf("ID = %q; want canonical envelope id", got.ID)
	}
	if !got.IsNew {
		t.Errorf("IsNew = false; want true")
	}
	if !got.Overwritten {
		t.Errorf("Overwritten mirror = false; want true (deprecated alias must equal IsNew)")
	}
	if !got.Success {
		t.Errorf("Success = false; want true")
	}
	if got.Message != "Mock created successfully" {
		t.Errorf("Message = %q; want the server message", got.Message)
	}
	if got.Mock.ID != "mock_1779037659" {
		t.Errorf("Mock.ID = %q; want round-trip", got.Mock.ID)
	}
}

// TestSaveMockResponse_AcceptsLegacyOverwrittenField proves we still
// decode the older draft wire shape (a hypothetical downgraded server
// emitting `overwritten` only). This guards against the bug fix
// over-correcting.
func TestSaveMockResponse_AcceptsLegacyOverwrittenField(t *testing.T) {
	payload := []byte(`{"mock":{"id":"m1"},"overwritten":true}`)

	var got SaveMockResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.IsNew {
		t.Errorf("IsNew = false; want legacy overwritten promoted to IsNew")
	}
	if !got.Overwritten {
		t.Errorf("Overwritten = false; want true")
	}
}

// TestSaveMockResponse_NewIsFalse confirms a non-overwrite save sets the
// flag to false on both accessors (regression guard against constant-true).
func TestSaveMockResponse_NewIsFalse(t *testing.T) {
	payload := []byte(`{"mock":{},"isNew":false}`)
	var got SaveMockResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.IsNew || got.Overwritten {
		t.Errorf("IsNew=%v Overwritten=%v; want both false", got.IsNew, got.Overwritten)
	}
}
