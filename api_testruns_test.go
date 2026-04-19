// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTestRuns_CreateMergedRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/test-runs/merges" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"name":"nightly-merge"`) {
			t.Fatalf("expected name in body, got %s", body)
		}
		if !strings.Contains(string(body), `"sourceRunIds":["src-1","src-2"]`) {
			t.Fatalf("expected sourceRunIds in body, got %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
            "run": {"ID":"merge-id","Name":"nightly-merge","Mode":"merged","Status":"completed","Namespace":"default"},
            "sources": [
                {"ID":"src-1","Mode":"fuzz","Status":"completed"},
                {"ID":"src-2","Mode":"chaos","Status":"failed"}
            ]
        }`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	view, err := c.TestRuns().CreateMergedRun(context.Background(), "nightly-merge", []string{"src-1", "src-2"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Run == nil || view.Run.ID != "merge-id" || view.Run.Mode != "merged" {
		t.Fatalf("unexpected parent: %+v", view.Run)
	}
	if len(view.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(view.Sources))
	}
	if view.Sources[0].ID != "src-1" || view.Sources[1].ID != "src-2" {
		t.Fatalf("unexpected sources: %+v", view.Sources)
	}
}

func TestTestRuns_GetMergedRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/test-runs/merges/merge-42" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
            "run": {"ID":"merge-42","Mode":"merged","Status":"completed"},
            "sources": [{"ID":"s1","Status":"passed"}]
        }`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	view, err := c.TestRuns().GetMergedRun(context.Background(), "merge-42")
	if err != nil {
		t.Fatal(err)
	}
	if view.Run.ID != "merge-42" {
		t.Fatalf("unexpected parent id %q", view.Run.ID)
	}
	if len(view.Sources) != 1 || view.Sources[0].ID != "s1" {
		t.Fatalf("unexpected sources: %+v", view.Sources)
	}
}

func TestTestRuns_AddMergeSource(t *testing.T) {
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/test-runs/merges/m1/sources" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["sourceRunId"] != "src-new" {
			t.Fatalf("expected sourceRunId=src-new, got %+v", body)
		}
		hit.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.TestRuns().AddMergeSource(context.Background(), "m1", "src-new"); err != nil {
		t.Fatal(err)
	}
	if hit.Load() != 1 {
		t.Error("handler not called")
	}
}

func TestTestRuns_RemoveMergeSource(t *testing.T) {
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/test-runs/merges/m1/sources/src-old" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		hit.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.TestRuns().RemoveMergeSource(context.Background(), "m1", "src-old"); err != nil {
		t.Fatal(err)
	}
	if hit.Load() != 1 {
		t.Error("handler not called")
	}
}
