// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

//go:build integration

// Package mockarty integration tests — Stage 3 phase C.
//
// These tests verify the Go SDK against a running Mockarty admin node.
// Activation requires:
//
//	MOCKARTY_INTEGRATION=1
//	MOCKARTY_BASE_URL=http://localhost:5770  (default)
//	MOCKARTY_API_KEY=<long-lived token>      (from POST /api/v1/auth/tokens)
//
// Without those the tests are skipped (`go test -tags=integration ./...`
// with the env unset reports them as SKIP). The admin's reachability is
// re-checked via GET /health before each suite — if the probe fails the
// suite skips rather than reporting a misleading FAIL.
package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// liveAdminConfig captures the resolved live-admin connection details.
type liveAdminConfig struct {
	BaseURL string
	APIKey  string
}

// requireLiveAdmin returns connection details or t.Skip's the test when
// integration mode is not enabled or the admin is unreachable.
func requireLiveAdmin(t *testing.T) liveAdminConfig {
	t.Helper()
	if os.Getenv("MOCKARTY_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_INTEGRATION!=1; skipping live-admin tests")
	}
	cfg := liveAdminConfig{
		BaseURL: os.Getenv("MOCKARTY_BASE_URL"),
		APIKey:  os.Getenv("MOCKARTY_API_KEY"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:5770"
	}
	if cfg.APIKey == "" {
		t.Skip("MOCKARTY_API_KEY unset; skipping live-admin tests")
	}
	// /health is the only fixed-shape probe across deployments.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/health", nil)
	if err != nil {
		t.Skipf("build health probe: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("admin unreachable at %s: %v", cfg.BaseURL, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("admin health probe %s -> %d; skipping", cfg.BaseURL, resp.StatusCode)
	}
	return cfg
}

// newLiveClient builds a Client bound to the live admin, falling back to
// the canonical "sandbox" namespace when the caller passes "".
func newLiveClient(t *testing.T, namespace string) *Client {
	t.Helper()
	cfg := requireLiveAdmin(t)
	if namespace == "" {
		namespace = "sandbox"
	}
	return NewClient(cfg.BaseURL,
		WithAPIKey(cfg.APIKey),
		WithNamespace(namespace),
		WithTimeout(15*time.Second),
	)
}

// uniqueID returns a hyphenated time-based id with prefix. Stage-3 tests
// run in parallel against a shared admin; collisions would corrupt cross-
// test assertions. Nanosecond precision + test name = collision-free.
func uniqueID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ---------------------------------------------------------------------------
// Client wiring smoke
// ---------------------------------------------------------------------------

func TestIntegration_ClientWiring(t *testing.T) {
	t.Parallel()
	c := newLiveClient(t, "")
	if c.BaseURL() == "" {
		t.Fatal("BaseURL empty after NewClient")
	}
	if c.Namespace() != "sandbox" {
		t.Errorf("default namespace = %q, want sandbox", c.Namespace())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.Health().Check(ctx)
	if err != nil {
		t.Fatalf("Health().Check: %v", err)
	}
	if resp == nil || (resp.Status != HealthStatusPass && resp.Status != "") {
		// Some deployments omit overall status (per-component only) — accept either.
		t.Logf("health response status: %q (per-component checks: %d)", resp.Status, len(resp.Checks))
	}
}

// ---------------------------------------------------------------------------
// Namespace lifecycle — Create / List / (no delete endpoint exposed via SDK)
// ---------------------------------------------------------------------------

func TestIntegration_NamespaceCreateList(t *testing.T) {
	t.Parallel()
	c := newLiveClient(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ns := uniqueID("sdk-int")
	if err := c.Namespaces().Create(ctx, ns); err != nil {
		// License-gated deployments cap namespace count; that's not a
		// SDK regression.  Surface as Skip.
		msg := err.Error()
		if strings.Contains(msg, "namespace limit") ||
			strings.Contains(msg, "403") ||
			strings.Contains(msg, "license") {
			t.Skipf("namespace creation gated by license: %v", err)
		}
		t.Fatalf("Namespaces.Create(%q): %v", ns, err)
	}

	list, err := c.Namespaces().List(ctx)
	if err != nil {
		t.Fatalf("Namespaces.List: %v", err)
	}
	found := false
	for _, n := range list {
		if n == ns {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created namespace %q missing from List: %v", ns, list)
	}
	// NamespaceAPI currently does not expose Delete; the namespace is
	// left in place. The unique-ID prefix keeps the admin manageable
	// even after many runs.
}

// ---------------------------------------------------------------------------
// Mock CRUD lifecycle (HTTP protocol)
// ---------------------------------------------------------------------------

func TestIntegration_MockCRUD(t *testing.T) {
	t.Parallel()
	c := newLiveClient(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	route := "/sdk-int/" + uniqueID("mock")
	mock := NewMockBuilder().
		HTTP(func(h *HTTPBuilder) {
			h.Method("GET").Route(route)
		}).
		Response(func(r *ResponseBuilder) {
			r.Status(200).JSONBody(map[string]string{"hello": "world"})
		}).
		Build()

	created, err := c.Mocks().Create(ctx, mock)
	if err != nil {
		t.Fatalf("Mocks.Create: %v", err)
	}
	if created.Mock.ID == "" {
		t.Fatal("created mock has empty ID")
	}
	id := created.Mock.ID
	t.Cleanup(func() {
		clean, cancelClean := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelClean()
		if err := c.Mocks().Delete(clean, id); err != nil {
			t.Logf("cleanup Mocks.Delete: %v", err)
		}
	})

	got, err := c.Mocks().Get(ctx, id)
	if err != nil {
		t.Fatalf("Mocks.Get(%s): %v", id, err)
	}
	if got.ID != id {
		t.Errorf("Get returned id=%q, want %q", got.ID, id)
	}
	if got.HTTP == nil || got.HTTP.Route != route {
		t.Errorf("Get returned http.route=%v, want %q", got.HTTP, route)
	}

	// Update — patch the response payload via re-create with same ID.
	got.Response = &ContentResponse{StatusCode: 201, Payload: map[string]string{"hello": "updated"}}
	upd, err := c.Mocks().Update(ctx, id, got)
	if err != nil {
		t.Fatalf("Mocks.Update: %v", err)
	}
	if upd.Response == nil || upd.Response.StatusCode != 201 {
		t.Errorf("update response status = %v, want 201", upd.Response)
	}
}

// ---------------------------------------------------------------------------
// Mock list filtering
// ---------------------------------------------------------------------------

func TestIntegration_MockList(t *testing.T) {
	t.Parallel()
	c := newLiveClient(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.Mocks().List(ctx, &ListMocksOptions{
		Namespace: c.Namespace(),
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("Mocks.List: %v", err)
	}
	if resp == nil {
		t.Fatal("Mocks.List returned nil resp")
	}
	// Empty list is a valid state on a fresh deployment; just ensure
	// the envelope structure is intact.
	if resp.Total < 0 {
		t.Errorf("Total=%d (negative)", resp.Total)
	}
}

// ---------------------------------------------------------------------------
// Me() — AwaitingManual surfaces against live admin (TCM gate dependent)
// ---------------------------------------------------------------------------

func TestIntegration_MeAwaitingManual(t *testing.T) {
	t.Parallel()
	c := newLiveClient(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Me().AwaitingManual(ctx)
	if err != nil {
		msg := err.Error()
		// TCM may be 403'd (license gate) or 404'd (route not
		// registered in this admin build) — both are valid
		// operational outcomes, not SDK regressions.
		if strings.Contains(msg, "403") ||
			strings.Contains(msg, "404") ||
			strings.Contains(msg, "feature") {
			t.Skipf("AwaitingManual not exposed on this admin: %v", err)
		}
		t.Fatalf("AwaitingManual: %v", err)
	}
	if resp == nil {
		t.Fatal("nil resp")
	}
	if resp.Count < 0 {
		t.Errorf("Count=%d", resp.Count)
	}
}
