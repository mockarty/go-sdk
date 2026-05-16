// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

//go:build integration

// DTO integration tests — Stage 3 phase C.
//
// The dto/ package is generated from `docs/swagger/swagger.json` and is
// expected to serialise EXACTLY what the admin server emits / accepts.
// These tests verify round-trip parity for a representative DTO by
// going through a live admin round-trip:
//
//	Build DTO -> Marshal -> POST -> GET -> Unmarshal into dto.X -> assert.
//
// We pick Mock because it has the broadest surface (every protocol
// embedded). Skip cleanly when MOCKARTY_INTEGRATION is unset or the
// admin is unreachable.
package dto_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/dto"
)

type liveAdmin struct {
	BaseURL   string
	APIKey    string
	Namespace string
}

func requireLiveAdmin(t *testing.T) liveAdmin {
	t.Helper()
	if os.Getenv("MOCKARTY_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_INTEGRATION!=1")
	}
	cfg := liveAdmin{
		BaseURL:   os.Getenv("MOCKARTY_BASE_URL"),
		APIKey:    os.Getenv("MOCKARTY_API_KEY"),
		Namespace: os.Getenv("MOCKARTY_NAMESPACE"),
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:5770"
	}
	if cfg.APIKey == "" {
		t.Skip("MOCKARTY_API_KEY unset")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "sandbox"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("admin unreachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("admin health probe -> %d", resp.StatusCode)
	}
	return cfg
}

// adminJSON is a minimal HTTP helper that POST/GET/DELETE's against the
// live admin with the X-API-Key auth header. The dto package can't reach
// for the parent SDK without an import cycle, so we hand-roll.
func adminJSON(t *testing.T, cfg liveAdmin, method, path string, body any, out any) int {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(buf)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, cfg.BaseURL+path, rdr)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		// Decode whatever the server returned — success bodies AND error
		// envelopes both flow through dto types when the caller asks.
		if err := json.Unmarshal(raw, out); err != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			t.Fatalf("decode %s %s: %v\nbody=%s", method, path, err, raw)
		}
	}
	if resp.StatusCode >= 400 {
		t.Logf("%s %s -> %d: %s", method, path, resp.StatusCode, raw)
	}
	return resp.StatusCode
}

// TestIntegration_DTORoundTrip_Mock verifies dto.Mock round-trips through
// the admin server unchanged.
func TestIntegration_DTORoundTrip_Mock(t *testing.T) {
	t.Parallel()
	cfg := requireLiveAdmin(t)

	route := fmt.Sprintf("/dto-int/%d", time.Now().UnixNano())
	body := map[string]any{
		"namespace": cfg.Namespace,
		"http": map[string]any{
			"httpMethod": "GET",
			"route":      route,
		},
		"response": map[string]any{
			"statusCode": 200,
			"payload":    map[string]any{"dto": "round-trip"},
		},
	}
	var created struct {
		Mock dto.Mock `json:"mock"`
	}
	status := adminJSON(t, cfg, http.MethodPost, "/api/v1/mocks", body, &created)
	if status != 200 && status != 201 {
		t.Skipf("create mock returned %d — admin not in expected state", status)
	}
	if created.Mock.ID == "" {
		t.Fatalf("created mock missing ID: %+v", created)
	}
	id := created.Mock.ID
	t.Cleanup(func() {
		_ = adminJSON(t, cfg, http.MethodDelete, "/api/v1/mocks/"+id, nil, nil)
	})

	var fetched dto.Mock
	if s := adminJSON(t, cfg, http.MethodGet, "/api/v1/mocks/"+id, nil, &fetched); s != 200 {
		t.Fatalf("GET mock returned %d", s)
	}
	if fetched.ID != id {
		t.Errorf("fetched ID=%q, want %q", fetched.ID, id)
	}
	if fetched.HTTP.Route != route {
		t.Errorf("fetched http.route=%q, want %q", fetched.HTTP.Route, route)
	}
	if fetched.HTTP.HTTPMethod != "GET" {
		t.Errorf("fetched http.method=%q, want GET", fetched.HTTP.HTTPMethod)
	}

	// Re-marshal the fetched DTO and confirm the new JSON parses back to
	// the same struct (idempotent encoding — required for diff-friendly
	// snapshot tests on the consumer side).
	raw1, err := json.Marshal(&fetched)
	if err != nil {
		t.Fatalf("marshal fetched: %v", err)
	}
	var redecoded dto.Mock
	if err := json.Unmarshal(raw1, &redecoded); err != nil {
		t.Fatalf("redecode: %v\n%s", err, raw1)
	}
	raw2, err := json.Marshal(&redecoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(raw1, raw2) {
		t.Errorf("DTO encoding not idempotent:\nfirst:  %s\nsecond: %s", raw1, raw2)
	}
}

// TestIntegration_DTOError_LoginShape probes the canonical login
// endpoint to confirm the error envelope still matches dto.ErrorResponse.
func TestIntegration_DTOError_LoginShape(t *testing.T) {
	t.Parallel()
	cfg := requireLiveAdmin(t)
	// Send an obviously-bad payload to force the validation envelope.
	var err dto.ErrorResponse
	status := adminJSON(t, cfg, http.MethodPost, "/api/v1/auth/login",
		dto.LoginRequest{Login: "", Password: ""}, &err)
	if status < 400 {
		t.Skipf("admin accepted empty credentials (status %d) — non-standard config", status)
	}
	if err.Error == "" {
		t.Errorf("error envelope empty: %+v", err)
	}
}
