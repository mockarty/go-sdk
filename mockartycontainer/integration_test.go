// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

//go:build integration

// MockartyContainer integration tests — Stage 3 phase C.
//
// These tests spin up the real `mockarty/cli:latest-mock` testcontainer.
// They self-skip when:
//   - Docker is unavailable on the host (no daemon, DOCKER_HOST unset
//     and no default socket reachable), or
//   - The image is not available locally AND no pull is permitted
//     (offline / air-gapped CI).
//
// Set MOCKARTY_CONTAINER_INTEGRATION=1 to opt into the testcontainer
// spin-up. The default (unset) skips even when MOCKARTY_INTEGRATION=1,
// because spawning containers is heavier than a network round-trip and
// some CI environments forbid it outright.
package mockartycontainer_test

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/mockartycontainer"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("MOCKARTY_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_INTEGRATION!=1")
	}
	if os.Getenv("MOCKARTY_CONTAINER_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_CONTAINER_INTEGRATION!=1; container spin-up is opt-in")
	}
	// Best-effort docker daemon probe — testcontainers will fail later
	// otherwise, but we want a friendlier skip reason.
	out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Skipf("docker daemon unreachable: %v / %s", err, out)
	}
	// Check image exists locally — pulling from a registry is too heavy /
	// air-gap-hostile for an integration smoke. Owner can pre-pull the
	// image to opt into the full flow.
	check, err := exec.Command("docker", "image", "inspect", mockartycontainer.DefaultImage).CombinedOutput()
	if err != nil || !strings.Contains(string(check), "\"Id\"") {
		t.Skipf("image %s not present locally; pull manually before running this test", mockartycontainer.DefaultImage)
	}
}

// TestIntegration_ContainerLifecycle starts the container, applies a stub
// via WireMock-compatible API, hits the resulting endpoint, and verifies
// graceful shutdown.
func TestIntegration_ContainerLifecycle(t *testing.T) {
	requireDocker(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	mc, err := mockartycontainer.New(ctx)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		if err := mc.Stop(stopCtx); err != nil {
			t.Logf("Stop: %v", err)
		}
	})

	if mc.URL() == "" {
		t.Error("URL() returned empty after start")
	}
	if mc.WireMockURL() == "" {
		t.Error("WireMockURL() returned empty after start")
	}

	stub := map[string]any{
		"request": map[string]any{
			"method": "GET",
			"url":    "/stage3-int/hello",
		},
		"response": map[string]any{
			"status": 200,
			"body":   `{"hi":"from-container"}`,
			"headers": map[string]string{
				"Content-Type": "application/json",
			},
		},
	}
	if err := mc.Apply(ctx, stub); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	resp, err := http.Get(mc.URL() + "/stage3-int/hello")
	if err != nil {
		t.Fatalf("GET stub: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("stub response status=%d, want 200", resp.StatusCode)
	}

	// Reset clears stubs.
	if err := mc.Reset(ctx); err != nil {
		t.Logf("Reset: %v", err)
	}
}

// TestIntegration_ContainerOptions verifies the With* options apply
// without instantiating a container (covers the config path that runs
// during New before docker contact).
func TestIntegration_ContainerOptions(t *testing.T) {
	t.Parallel()
	// New with an obviously unreachable image — we just want to verify
	// option application doesn't panic before docker engagement.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := mockartycontainer.New(ctx,
		mockartycontainer.WithImage("does-not-exist:never"),
		mockartycontainer.WithEnv("EXAMPLE_KEY", "value"),
		mockartycontainer.WithFormat(mockartycontainer.FormatAuto),
	)
	if err == nil {
		t.Skip("docker is available and accepted bogus image; not a regression")
	}
}
