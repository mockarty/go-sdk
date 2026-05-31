// Copyright (c) 2026 Mockarty. All rights reserved.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscoverySync(t *testing.T) {
	var capturedPath string
	var capturedMethod string
	var capturedKey string
	var capturedBody DiscoveryManifest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedMethod = r.Method
		capturedKey = r.Header.Get("X-API-Key")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)
		_ = json.NewEncoder(w).Encode(DiscoverySyncResult{
			Source: "pytest:auth-suite", Created: 2, Updated: 1, Orphaned: 3, Total: 3,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("mk_test"), WithNamespace("test-ns"))
	resp, err := c.Discovery().Sync(context.Background(), "", DiscoveryManifest{
		Source:       "pytest:auth-suite",
		Framework:    "pytest",
		PruneMissing: true,
		Cases: []DiscoveryManifestCase{
			{
				FullName:    "pkg.Mod::test_x",
				Name:        "Test X",
				Suite:       "auth",
				Description: "checks login",
				SourceRef:   "file.py:12",
				Labels:      []string{"smoke"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// Response parsed correctly.
	if resp.Source != "pytest:auth-suite" {
		t.Fatalf("Source: %q", resp.Source)
	}
	if resp.Created != 2 || resp.Updated != 1 || resp.Orphaned != 3 || resp.Total != 3 {
		t.Fatalf("counts: %+v", resp)
	}

	// Right path, method, auth header.
	if capturedPath != "/api/v1/namespaces/test-ns/tcm/discovery" {
		t.Fatalf("path: %q", capturedPath)
	}
	if capturedMethod != http.MethodPost {
		t.Fatalf("method: %q", capturedMethod)
	}
	if capturedKey != "mk_test" {
		t.Fatalf("X-API-Key not sent: %q", capturedKey)
	}

	// Right JSON shape on the wire.
	if capturedBody.Source != "pytest:auth-suite" {
		t.Fatalf("body.Source: %q", capturedBody.Source)
	}
	if capturedBody.Framework != "pytest" {
		t.Fatalf("body.Framework: %q", capturedBody.Framework)
	}
	if !capturedBody.PruneMissing {
		t.Fatalf("body.PruneMissing not sent")
	}
	if len(capturedBody.Cases) != 1 {
		t.Fatalf("body.Cases len: %d", len(capturedBody.Cases))
	}
	if capturedBody.Cases[0].FullName != "pkg.Mod::test_x" {
		t.Fatalf("body.Cases[0].FullName: %q", capturedBody.Cases[0].FullName)
	}
	if capturedBody.Cases[0].SourceRef != "file.py:12" {
		t.Fatalf("body.Cases[0].SourceRef: %q", capturedBody.Cases[0].SourceRef)
	}
	if len(capturedBody.Cases[0].Labels) != 1 || capturedBody.Cases[0].Labels[0] != "smoke" {
		t.Fatalf("body.Cases[0].Labels: %v", capturedBody.Cases[0].Labels)
	}
}

// TestDiscoverySyncWireKeys asserts the exact JSON keys on the wire so a
// rename can't silently drift this SDK out of sync with Python/Java.
func TestDiscoverySyncWireKeys(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &raw)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("ns"))
	_, err := c.Discovery().Sync(context.Background(), "", DiscoveryManifest{
		Source:       "go:list",
		PruneMissing: true,
		Cases:        []DiscoveryManifestCase{{FullName: "x"}},
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if _, ok := raw["source"]; !ok {
		t.Fatalf("missing wire key `source`: %v", raw)
	}
	if _, ok := raw["pruneMissing"]; !ok {
		t.Fatalf("missing wire key `pruneMissing`: %v", raw)
	}
	cases, ok := raw["cases"].([]any)
	if !ok || len(cases) != 1 {
		t.Fatalf("bad `cases`: %v", raw["cases"])
	}
	caseObj, ok := cases[0].(map[string]any)
	if !ok {
		t.Fatalf("case not an object: %v", cases[0])
	}
	if _, ok := caseObj["fullName"]; !ok {
		t.Fatalf("missing wire key `fullName`: %v", caseObj)
	}
}

func TestDiscoverySyncNamespaceOverride(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithNamespace("default-ns"))
	_, _ = c.Discovery().Sync(context.Background(), "override-ns", DiscoveryManifest{
		Source: "s", Cases: []DiscoveryManifestCase{{FullName: "x"}},
	})
	if !strings.Contains(capturedPath, "/override-ns/") {
		t.Fatalf("override ignored: %q", capturedPath)
	}
}

func TestDiscoverySyncRequiresNamespace(t *testing.T) {
	c := NewClient("http://unreachable.test")
	c.namespace = ""
	_, err := c.Discovery().Sync(context.Background(), "", DiscoveryManifest{
		Source: "s", Cases: []DiscoveryManifestCase{{FullName: "x"}},
	})
	if err == nil {
		t.Fatal("want error when namespace missing")
	}
}

// TestDiscoverySyncErrorSurfaces asserts a 400 (e.g. blank fullName /
// missing source) is surfaced as an error, not swallowed.
func TestDiscoverySyncErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"source is required","code":"invalid_request"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("ns"))
	resp, err := c.Discovery().Sync(context.Background(), "", DiscoveryManifest{
		Cases: []DiscoveryManifestCase{{FullName: ""}},
	})
	if err == nil {
		t.Fatal("want error on 400")
	}
	if resp != nil {
		t.Fatalf("want nil result on error, got %+v", resp)
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode: %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Message, "source is required") {
		t.Fatalf("Message: %q", apiErr.Message)
	}
}
