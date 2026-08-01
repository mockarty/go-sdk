// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The version endpoints return revision ROWS, not mocks: the mock body of a
// revision hangs off the row's "mock" key. Decoding a row straight into Mock —
// which the SDK used to do — produced entries whose ID was the version-row id
// and whose request/response were empty, with no error to show for it.

func TestMocks_ListVersions_DecodesVersionRows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/mocks/versioned-mock/versions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"mock_id": "versioned-mock",
			"count": 2,
			"versions": [
				{"id":"ver-2","mock_id":"versioned-mock","version":2,"created_at":1700000200,
				 "lifecycle_state":"active","tags":["v2"],
				 "mock":{"id":"versioned-mock","namespace":"sandbox","tags":["v2"]}},
				{"id":"ver-1","mock_id":"versioned-mock","version":1,"created_at":1700000100,
				 "lifecycle_state":"active","tags":["v1"],
				 "mock":{"id":"versioned-mock","namespace":"sandbox","tags":["v1"]}}
			]
		}`))
	}))
	defer srv.Close()

	versions, err := NewClient(srv.URL).Mocks().ListVersions(context.Background(), "versioned-mock")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("got %d versions, want 2", len(versions))
	}
	if versions[0].Version != 2 || versions[1].Version != 1 {
		t.Errorf("version numbers = %d/%d, want 2/1", versions[0].Version, versions[1].Version)
	}
	if versions[0].ID != "ver-2" || versions[0].MockID != "versioned-mock" {
		t.Errorf("row identity = %q/%q", versions[0].ID, versions[0].MockID)
	}
	if versions[0].CreatedAt != 1700000200 {
		t.Errorf("created_at = %d", versions[0].CreatedAt)
	}
	// The mock body of the revision must be reachable — this is what the old
	// []*Mock decode silently threw away.
	if versions[0].Mock == nil || versions[0].Mock.Namespace != "sandbox" {
		t.Fatalf("revision mock body missing: %+v", versions[0].Mock)
	}
	if len(versions[1].Tags) != 1 || versions[1].Tags[0] != "v1" {
		t.Errorf("tags = %v", versions[1].Tags)
	}
}

func TestMocks_GetVersion_UnwrapsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/mocks/versioned-mock/versions/2" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"version": {"id":"ver-2","mock_id":"versioned-mock","version":2,
			            "mock":{"id":"versioned-mock","namespace":"sandbox"}},
			"previous_version": {"id":"ver-1","mock_id":"versioned-mock","version":1,
			            "mock":{"id":"versioned-mock","namespace":"sandbox"}}
		}`))
	}))
	defer srv.Close()

	api := NewClient(srv.URL).Mocks()
	got, err := api.GetVersion(context.Background(), "versioned-mock", "2")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Version != 2 || got.Mock == nil || got.Mock.Namespace != "sandbox" {
		t.Fatalf("decoded version = %+v (mock %+v)", got, got.Mock)
	}

	current, previous, err := api.GetVersionWithPrevious(context.Background(), "versioned-mock", "2")
	if err != nil {
		t.Fatalf("GetVersionWithPrevious: %v", err)
	}
	if current.Version != 2 {
		t.Errorf("current = %d, want 2", current.Version)
	}
	if previous == nil || previous.Version != 1 {
		t.Fatalf("previous = %+v, want version 1", previous)
	}
}

// A version-1 lookup has no predecessor; that must be a nil previous, not a
// zero-valued row that reads as "version 0".
func TestMocks_GetVersion_NoPrevious(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":{"id":"ver-1","version":1},"previous_version":null}`))
	}))
	defer srv.Close()

	_, previous, err := NewClient(srv.URL).Mocks().
		GetVersionWithPrevious(context.Background(), "versioned-mock", "1")
	if err != nil {
		t.Fatalf("GetVersionWithPrevious: %v", err)
	}
	if previous != nil {
		t.Fatalf("previous = %+v, want nil", previous)
	}
}

// An unknown revision comes back as an envelope with a null "version". Silently
// returning an empty MockVersion would read as "revision 0 exists".
func TestMocks_GetVersion_MissingRevisionIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":null,"previous_version":null}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL).Mocks().
		GetVersion(context.Background(), "versioned-mock", "9"); err == nil {
		t.Fatal("expected an error for a missing revision")
	}
}
