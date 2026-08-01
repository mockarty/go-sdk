// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

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

// TestSaveMockResponse_DecodesFlatShape proves the G2 envelope unification:
// a flat-only response (mock fields at the top level, no "mock" wrapper) still
// populates Mock from the top-level fields. Guards the forward path for when
// the deprecated wrapper is eventually removed.
func TestSaveMockResponse_DecodesFlatShape(t *testing.T) {
	payload := []byte(`{
		"id": "mock_42",
		"namespace": "sandbox",
		"serverName": "api",
		"isNew": true,
		"success": true,
		"message": "Mock created successfully"
	}`)

	var got SaveMockResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "mock_42" {
		t.Errorf("ID = %q; want mock_42", got.ID)
	}
	if got.Mock.ID != "mock_42" {
		t.Errorf("Mock.ID = %q; want reconstructed from flat top-level fields", got.Mock.ID)
	}
	if got.Mock.Namespace != "sandbox" {
		t.Errorf("Mock.Namespace = %q; want sandbox (flat field reconstructed)", got.Mock.Namespace)
	}
	if !got.IsNew || !got.Success {
		t.Errorf("IsNew=%v Success=%v; want both true", got.IsNew, got.Success)
	}
}

// TestSaveMockResponse_WrapperWinsOverFlat proves that when BOTH the wrapper
// and flat fields are present (the dual-emit transition shape), the explicit
// "mock" wrapper is used verbatim and the flat fallback does not clobber it.
func TestSaveMockResponse_WrapperWinsOverFlat(t *testing.T) {
	payload := []byte(`{
		"id": "m9",
		"namespace": "flat-ns",
		"mock": {"id": "m9", "namespace": "wrapper-ns"},
		"isNew": false
	}`)
	var got SaveMockResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Mock.Namespace != "wrapper-ns" {
		t.Errorf("Mock.Namespace = %q; want wrapper-ns (wrapper must win when present)", got.Mock.Namespace)
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

// TestMockListResponse_DecodesCanonicalWireShape proves the SDK decodes
// the actual admin-node wire shape — `mocks` (not `items`) and `count`
// (not `total`). Surfaced 2026-05-17 alongside the SaveMockResponse fix
// by a sibling-instance hunt: Mocks.List() had been silently returning
// empty slices because the SDK was binding `items`/`total` which the
// server never emits.
func TestMockListResponse_DecodesCanonicalWireShape(t *testing.T) {
	payload := []byte(`{
		"count": 2,
		"limit": 50,
		"message": "Mock list retrieved successfully",
		"mocks": [{"id":"m1","namespace":"ns"},{"id":"m2","namespace":"ns"}]
	}`)
	var got MockListResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 2 {
		t.Errorf("Count = %d; want 2", got.Count)
	}
	if got.Total != got.Count {
		t.Errorf("Total mirror = %d; want = Count = 2", got.Total)
	}
	if got.Limit != 50 {
		t.Errorf("Limit = %d; want 50", got.Limit)
	}
	if len(got.Items) != 2 {
		t.Fatalf("len(Items) = %d; want 2", len(got.Items))
	}
	if got.Items[0].ID != "m1" || got.Items[1].ID != "m2" {
		t.Errorf("Items = %+v; want m1, m2", got.Items)
	}
	if got.Message == "" {
		t.Errorf("Message empty; want server message")
	}
}

// TestMockListResponse_AcceptsLegacyItemsTotal proves a downgraded
// server emitting the older `{items, total}` shape still decodes.
func TestMockListResponse_AcceptsLegacyItemsTotal(t *testing.T) {
	payload := []byte(`{"items":[{"id":"m1"}],"total":1}`)
	var got MockListResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "m1" {
		t.Errorf("Items = %+v; want legacy items promoted", got.Items)
	}
	if got.Count != 1 || got.Total != 1 {
		t.Errorf("Count=%d Total=%d; want 1 each", got.Count, got.Total)
	}
}

// TestMockListResponse_EmptyResultStillDecodes — defensive guard
// against the new UnmarshalJSON failing on a zero-result page.
func TestMockListResponse_EmptyResultStillDecodes(t *testing.T) {
	payload := []byte(`{"count":0,"mocks":[],"limit":50,"message":"ok"}`)
	var got MockListResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 0 || len(got.Items) != 0 {
		t.Errorf("Count=%d len(Items)=%d; want 0", got.Count, len(got.Items))
	}
}
