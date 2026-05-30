// Copyright (c) 2026 Mockarty. All rights reserved.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const samplePactV3 = `{
  "consumer": {"name": "WebApp"},
  "provider": {"name": "UserService"},
  "interactions": [
    {
      "description": "a request for user 1",
      "request": {"method": "GET", "path": "/users/1"},
      "response": {"status": 200, "body": {"id": 1, "name": "Ada"}}
    }
  ],
  "metadata": {"pactSpecification": {"version": "3.0.0"}}
}`

// TestParsePactMeta verifies the offline pact → wire-payload mapping:
// version is derived from the pact body, and consumer/provider absence
// is rejected before any network round-trip.
func TestParsePactMeta(t *testing.T) {
	tests := []struct {
		name        string
		pact        string
		wantErr     bool
		wantVersion string
	}{
		{
			name:        "v3 derives version from metadata",
			pact:        samplePactV3,
			wantVersion: "3.0.0",
		},
		{
			name: "v4 pact",
			pact: `{"consumer":{"name":"C"},"provider":{"name":"P"},
			        "interactions":[{"type":"Synchronous/HTTP","description":"d",
			        "request":{"method":"GET","path":"/"},"response":{"status":200}}],
			        "metadata":{"pactSpecification":{"version":"4.0"}}}`,
			wantVersion: "4.0",
		},
		{
			name: "missing consumer rejected",
			pact: `{"provider":{"name":"P"},"interactions":[]}`,
			wantErr: true,
		},
		{
			name:    "missing provider rejected",
			pact:    `{"consumer":{"name":"C"},"interactions":[]}`,
			wantErr: true,
		},
		{
			name:    "malformed JSON rejected",
			pact:    `{not json`,
			wantErr: true,
		},
		{
			name:        "no version is allowed (server may reject later)",
			pact:        `{"consumer":{"name":"C"},"provider":{"name":"P"}}`,
			wantVersion: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := parsePactMeta([]byte(tt.pact))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.specVersion != tt.wantVersion {
				t.Fatalf("version = %q, want %q", meta.specVersion, tt.wantVersion)
			}
		})
	}
}

// TestImportPactWirePayload asserts the bridge POSTs the exact wire shape
// the server's pactPublish handler binds: {pactContent, version} to
// /api/v1/contract/pacts with the namespace threaded onto the query.
func TestImportPactWirePayload(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotMethod = r.Method
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":"abc","consumer":{"name":"WebApp"},"provider":{"name":"UserService"},"version":"3.0.0"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"), WithNamespace("team-a"))
	pact, err := c.Contracts().ImportPact(context.Background(), []byte(samplePactV3), nil)
	if err != nil {
		t.Fatalf("ImportPact: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/contract/pacts?namespace=team-a" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["pactContent"] != samplePactV3 {
		t.Fatalf("pactContent not forwarded verbatim")
	}
	if gotBody["version"] != "3.0.0" {
		t.Fatalf("version = %v, want 3.0.0 (derived from body)", gotBody["version"])
	}
	if pact.ID != "abc" {
		t.Fatalf("decoded pact id = %q", pact.ID)
	}
}

// TestImportPactExplicitVersionOverride confirms an explicit version beats
// the pact body's spec version, and an explicit namespace beats the
// client default.
func TestImportPactExplicitVersionOverride(t *testing.T) {
	var gotBody map[string]any
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"), WithNamespace("default-ns"))
	_, err := c.Contracts().ImportPact(context.Background(), []byte(samplePactV3),
		&ImportPactOptions{Version: "git-sha-123", Namespace: "ci-ns"})
	if err != nil {
		t.Fatalf("ImportPact: %v", err)
	}
	if gotBody["version"] != "git-sha-123" {
		t.Fatalf("version = %v, want explicit override", gotBody["version"])
	}
	if gotQuery != "namespace=ci-ns" {
		t.Fatalf("query = %q, want namespace=ci-ns", gotQuery)
	}
}

// TestImportPactFileReadsDisk exercises the file-reading entry point.
func TestImportPactFileReadsDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "webapp-userservice.json")
	if err := os.WriteFile(path, []byte(samplePactV3), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		gotContent, _ = b["pactContent"].(string)
		_, _ = w.Write([]byte(`{"id":"f"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	if _, err := c.Contracts().ImportPactFile(context.Background(), path, nil); err != nil {
		t.Fatalf("ImportPactFile: %v", err)
	}
	if gotContent != samplePactV3 {
		t.Fatalf("file content not forwarded")
	}

	// Missing file fails fast.
	if _, err := c.Contracts().ImportPactFile(context.Background(), filepath.Join(dir, "nope.json"), nil); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

// TestImportPactLive publishes a real pact to a running Mockarty admin and
// asserts a contract is created. Gated by MOCKARTY_LIVE_TOKEN like the
// other *_live_test.go smoke tests.
func TestImportPactLive(t *testing.T) {
	token := os.Getenv("MOCKARTY_LIVE_TOKEN")
	if token == "" {
		t.Skip("set MOCKARTY_LIVE_TOKEN to a fresh API key to run the live smoke test")
	}
	base := os.Getenv("MOCKARTY_LIVE_URL")
	if base == "" {
		base = "http://127.0.0.1:5770"
	}
	c := NewClient(base, WithAPIKey(token), WithNamespace("sandbox"))

	live := `{"consumer":{"name":"GoSDKLiveConsumer"},"provider":{"name":"GoSDKLiveProvider"},
	          "interactions":[{"description":"smoke","request":{"method":"GET","path":"/ping"},
	          "response":{"status":200,"body":{"ok":true}}}],
	          "metadata":{"pactSpecification":{"version":"3.0.0"}}}`

	pact, err := c.Contracts().ImportPact(context.Background(), []byte(live),
		&ImportPactOptions{Version: "1.0.0-live"})
	if err != nil {
		t.Fatalf("ImportPact live: %v", err)
	}
	if pact.ID == "" {
		t.Fatalf("expected a contract id back, got empty")
	}
	if pact.Consumer.Name != "GoSDKLiveConsumer" {
		t.Fatalf("consumer = %q, want GoSDKLiveConsumer", pact.Consumer.Name)
	}
	if pact.Provider.Name != "GoSDKLiveProvider" {
		t.Fatalf("provider = %q, want GoSDKLiveProvider", pact.Provider.Name)
	}
	t.Cleanup(func() {
		_ = c.Contracts().DeletePact(context.Background(), pact.ID)
	})
}
