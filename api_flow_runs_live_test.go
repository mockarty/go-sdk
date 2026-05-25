// Copyright (c) 2026 Mockarty. All rights reserved.

package mockarty

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// TestFlowRunsExecuteLive drives FlowRuns.Execute against a running
// Mockarty admin. Gated by MOCKARTY_LIVE_TOKEN — set the env var to a
// fresh API key produced by `POST /api/v1/auth/tokens` and the test
// will exercise the full client → server → scriptengine → http
// runner → response chain.
//
// MOCKARTY_LIVE_URL overrides the admin URL (default
// http://127.0.0.1:5770). The probe step targets that same URL's
// /health endpoint so no testbackend dependency.
func TestFlowRunsExecuteLive(t *testing.T) {
	token := os.Getenv("MOCKARTY_LIVE_TOKEN")
	if token == "" {
		t.Skip("set MOCKARTY_LIVE_TOKEN to a fresh API key to run the live smoke test")
	}
	base := os.Getenv("MOCKARTY_LIVE_URL")
	if base == "" {
		base = "http://127.0.0.1:5770"
	}
	c := NewClient(base, WithAPIKey(token))
	flow, _ := json.Marshal(map[string]any{
		"ir_version": 1,
		"name":       "go-sdk-live-smoke",
		"steps": []any{
			map[string]any{
				"kind": "http",
				"name": "probe",
				"http": map[string]any{
					"method": "GET",
					"path":   base + "/health",
					"expects": []any{
						map[string]any{"kind": "status", "args": []any{200}},
					},
				},
			},
		},
	})
	resp, err := c.FlowRuns().Execute(context.Background(), flow)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Status != "passed" {
		t.Fatalf("live run status=%q errors=%v", resp.Status, resp.Errors)
	}
	if resp.DurationMs < 0 {
		t.Fatalf("duration: %d", resp.DurationMs)
	}
}
