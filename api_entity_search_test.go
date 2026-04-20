// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEntitySearch_RequiresType(t *testing.T) {
	c := NewClient("http://example.invalid", WithAPIKey("k"))
	_, err := c.EntitySearch().Search(context.Background(), EntitySearchRequest{})
	if err == nil {
		t.Fatalf("expected error for empty type")
	}
}

func TestEntitySearch_BuildsQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/api/v1/entity-search"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		q := r.URL.Query()
		if got := q.Get("type"); got != EntityTypeTestPlan {
			t.Errorf("type = %q, want %q", got, EntityTypeTestPlan)
		}
		if got := q.Get("namespace"); got != "production" {
			t.Errorf("namespace = %q", got)
		}
		if got := q.Get("q"); got != "smoke" {
			t.Errorf("q = %q", got)
		}
		if got := q.Get("limit"); got != "25" {
			t.Errorf("limit = %q", got)
		}
		if got := q.Get("offset"); got != "5" {
			t.Errorf("offset = %q", got)
		}
		numericID := int64(42)
		_ = json.NewEncoder(w).Encode(EntitySearchResponse{
			Items: []EntitySearchResult{
				{
					ID:        "11111111-2222-3333-4444-555555555555",
					Type:      EntityTypeTestPlan,
					Name:      "smoke-suite",
					Namespace: "production",
					CreatedAt: "2026-04-19T12:00:00Z",
					NumericID: &numericID,
				},
			},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	resp, err := c.EntitySearch().Search(context.Background(), EntitySearchRequest{
		Type:      EntityTypeTestPlan,
		Namespace: "production",
		Query:     "smoke",
		Limit:     25,
		Offset:    5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("unexpected response %#v", resp)
	}
	got := resp.Items[0]
	if got.Name != "smoke-suite" {
		t.Errorf("name = %q", got.Name)
	}
	if got.NumericID == nil || *got.NumericID != 42 {
		t.Errorf("numericId = %v", got.NumericID)
	}
}

func TestEntitySearch_NormalisesNilItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a server bug returning nil items — SDK MUST normalise to []
		// so callers do not need a nil-guard before iteration.
		_, _ = w.Write([]byte(`{"items":null,"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	resp, err := c.EntitySearch().Search(context.Background(), EntitySearchRequest{
		Type: EntityTypeMock,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Items == nil {
		t.Fatalf("items must be non-nil after normalisation")
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty slice, got %d", len(resp.Items))
	}
}

func TestEntitySearch_OmitsZeroParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Required field present.
		if got := q.Get("type"); got != EntityTypeMock {
			t.Errorf("type = %q", got)
		}
		// Optional fields must NOT appear when zero.
		for _, k := range []string{"namespace", "q", "limit", "offset"} {
			if got := q.Get(k); got != "" {
				t.Errorf("expected %q omitted, got %q", k, got)
			}
		}
		_ = json.NewEncoder(w).Encode(EntitySearchResponse{Items: []EntitySearchResult{}, Total: 0})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	if _, err := c.EntitySearch().Search(context.Background(), EntitySearchRequest{
		Type: EntityTypeMock,
	}); err != nil {
		t.Fatal(err)
	}
}
