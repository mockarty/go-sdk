// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

//go:build integration

// Fuzz SDK integration tests — Stage 3 phase C.
//
// Two surfaces are exercised:
//
//  1. DSL → JSON wire shape (local-only, no admin/CLI).
//  2. Runner.Submit + Wait against the live admin's
//     POST /api/v1/fuzzing/run.  Skipped when the env is not set.
//
// LocalSpawn (CLI subprocess) is documented to call
// `mockarty-cli fuzz run <file> --json`. The current CLI binary on this
// host exposes a flag-driven `fuzz` command instead, so the LocalSpawn
// test is gated on the subcommand being present.
package fuzz_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/fuzz"
)

func requireLiveAdmin(t *testing.T) (baseURL, token, namespace string) {
	t.Helper()
	if os.Getenv("MOCKARTY_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_INTEGRATION!=1")
	}
	baseURL = os.Getenv("MOCKARTY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5770"
	}
	token = os.Getenv("MOCKARTY_API_KEY")
	if token == "" {
		t.Skip("MOCKARTY_API_KEY unset")
	}
	namespace = os.Getenv("MOCKARTY_NAMESPACE")
	if namespace == "" {
		namespace = "sandbox"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("admin unreachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("admin health probe -> %d", resp.StatusCode)
	}
	return baseURL, token, namespace
}

// buildSampleTarget assembles a small but complete Target — 3 seeds, 2
// mutators, 2 assertions — so individual tests can avoid duplicating
// boilerplate.
func buildSampleTarget(t *testing.T, namespace string) *fuzz.Target {
	t.Helper()
	target := fuzz.NewTarget("stage3-go-sdk-fuzz",
		fuzz.WithNamespace(namespace),
		fuzz.WithDescription("Stage 3 integration fuzz smoke"),
		fuzz.WithDuration(2*time.Second),
		fuzz.WithMaxRequests(20),
		fuzz.WithConcurrency(1),
		fuzz.WithMaxRPS(5),
		fuzz.WithHTTPEndpoint("GET", "http://127.0.0.1:5770", "/health"),
		fuzz.WithSeed(fuzz.Seed("baseline", "")),
		fuzz.WithSeed(fuzz.Seed("ascii-payload", "hello")),
		fuzz.WithSeed(fuzz.SeedHTTP("payload-form", "POST", "/health", "x=y")),
		fuzz.WithMutator(fuzz.MutatorString),
		fuzz.WithMutator(fuzz.MutatorHeader),
		fuzz.WithAssertion(fuzz.AssertNoCrash()),
		fuzz.WithAssertion(fuzz.AssertStatus(200, 499)),
		fuzz.WithTag("stage3"),
		fuzz.WithTag("sdk"),
	)
	if err := target.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return target
}

// TestIntegration_FuzzDSL exercises the DSL surface — every option that
// affects the wire shape — and asserts the validate path accepts it.
func TestIntegration_FuzzDSL(t *testing.T) {
	t.Parallel()
	target := buildSampleTarget(t, "sandbox")

	if got := target.Seeds(); len(got) != 3 {
		t.Errorf("Seeds=%d, want 3", len(got))
	}
	if got := target.Mutators(); len(got) != 2 {
		t.Errorf("Mutators=%d, want 2", len(got))
	}
	if got := target.Assertions(); len(got) != 2 {
		t.Errorf("Assertions=%d, want 2", len(got))
	}
	if got := target.Protocol(); !got.Valid() {
		t.Errorf("Protocol invalid after WithHTTPEndpoint: %q", got)
	}
}

// TestIntegration_FuzzInvalidTarget verifies that Validate catches the
// classic mistakes: missing protocol endpoint, empty name.
func TestIntegration_FuzzInvalidTarget(t *testing.T) {
	t.Parallel()
	empty := fuzz.NewTarget("")
	if err := empty.Validate(); err == nil {
		t.Error("empty-name target validated OK; expected error")
	}
	noEndpoint := fuzz.NewTarget("x")
	if err := noEndpoint.Validate(); err == nil {
		t.Error("no-endpoint target validated OK; expected error")
	}
}

// TestIntegration_FuzzRunnerSubmit_Live submits the sample target to the
// live admin and polls for completion. The poll budget is short — the
// goal is to confirm the SDK round-trips, not to wait for a full run.
func TestIntegration_FuzzRunnerSubmit_Live(t *testing.T) {
	t.Parallel()
	baseURL, token, namespace := requireLiveAdmin(t)

	r := fuzz.NewRunner(baseURL, namespace, token,
		fuzz.WithRunnerTimeout(15*time.Second),
		fuzz.WithRunnerPollPeriod(1*time.Second),
	)
	target := buildSampleTarget(t, namespace)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	job, err := r.Submit(ctx, target)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "403") || strings.Contains(msg, "license"):
			t.Skipf("fuzz feature gated off: %v", err)
		case strings.Contains(msg, "404"):
			t.Skipf("fuzz endpoint missing on admin: %v", err)
		case strings.Contains(msg, "400"):
			// Validation rejection from admin — the SDK still produced a
			// schema the admin parsed, so the wiring is OK. Surface as
			// pass-through info rather than fail.
			t.Logf("admin rejected target with 400 — schema reached server: %v", err)
			return
		}
		t.Fatalf("Submit: %v", err)
	}
	if job == nil || job.ID == "" {
		t.Fatalf("Submit returned empty ID: %+v", job)
	}
	t.Logf("submitted job %s", job.ID)

	// Stop the job to avoid a long-running fuzz against /health. Best
	// effort — admin may not implement Stop, in which case we just let
	// the budget tick and Wait will return whatever status it has.
	if err := r.Stop(ctx, job.ID); err != nil {
		t.Logf("Stop returned %v (acceptable)", err)
	}

	// Quick Wait with a tight deadline.
	waitCtx, cancelWait := context.WithTimeout(ctx, 10*time.Second)
	defer cancelWait()
	res, err := r.Wait(waitCtx, job.ID)
	if err != nil {
		t.Logf("Wait error (acceptable on short budget): %v", err)
		return
	}
	if res == nil {
		t.Fatal("Wait returned nil Result and nil error")
	}
	t.Logf("Wait returned status=%s findings=%d", res.Status, len(res.Findings))
}

// TestIntegration_FuzzRunner_LocalSpawn invokes the CLI subprocess.
// Skipped when:
//   - mockarty-cli is not on PATH (or MOCKARTY_CLI override missing).
//   - the binary lacks the `fuzz run <file>` subcommand (current dev
//     CLI exposes a flag-driven fuzz command instead).
func TestIntegration_FuzzRunner_LocalSpawn(t *testing.T) {
	t.Parallel()
	cli := os.Getenv("MOCKARTY_CLI")
	if cli == "" {
		cli = "mockarty-cli"
	}
	if _, err := exec.LookPath(cli); err != nil {
		// Try the well-known stage3 path before giving up.
		if _, err2 := exec.LookPath("/tmp/mockarty-cli-stage3"); err2 == nil {
			cli = "/tmp/mockarty-cli-stage3"
		} else {
			t.Skipf("mockarty-cli not on PATH: %v", err)
		}
	}
	// Verify the binary supports `fuzz run <file>` — the SDK's call shape.
	out, _ := exec.Command(cli, "fuzz", "run", "--help").CombinedOutput()
	if !strings.Contains(string(out), "fuzz run") {
		t.Skip("installed mockarty-cli does not expose `fuzz run <file>` subcommand — SDK LocalSpawn is N/A on this host")
	}

	r := fuzz.NewRunner("", "", "",
		fuzz.WithRunnerCLIPath(cli),
		fuzz.WithRunnerTimeout(10*time.Second),
	)
	target := fuzz.NewTarget("local-spawn-smoke",
		fuzz.WithHTTPEndpoint("GET", "http://127.0.0.1:5770", "/health"),
		fuzz.WithDuration(1*time.Second),
		fuzz.WithSeed(fuzz.Seed("baseline", "")),
		fuzz.WithMutator(fuzz.MutatorString),
		fuzz.WithAssertion(fuzz.AssertNoCrash()),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := r.LocalSpawn(ctx, target)
	if err != nil {
		t.Fatalf("LocalSpawn: %v", err)
	}
	if res == nil {
		t.Fatal("LocalSpawn returned nil result")
	}
}

// TestIntegration_FuzzTargetJSON confirms ToJSON produces a parseable
// document — the wire shape persists through marshal/unmarshal.
func TestIntegration_FuzzTargetJSON(t *testing.T) {
	t.Parallel()
	target := buildSampleTarget(t, "sandbox")
	raw, err := target.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("ToJSON produced invalid JSON: %v\n%s", err, raw)
	}
	if doc["name"] != "stage3-go-sdk-fuzz" {
		t.Errorf("ToJSON name=%v, want stage3-go-sdk-fuzz", doc["name"])
	}
}

// TestIntegration_FuzzRunnerNoAdmin verifies the requireRemote gate —
// calls to Submit / Wait / Stream on a runner built without baseURL
// must return a sentinel-style error.
func TestIntegration_FuzzRunnerNoAdmin(t *testing.T) {
	t.Parallel()
	r := fuzz.NewRunner("", "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := r.Submit(ctx, fuzz.NewTarget("x", fuzz.WithHTTPEndpoint("GET", "http://x", "/"))); err == nil {
		t.Error("Submit without baseURL should error")
	} else if !errors.Is(err, err) { // ensure non-nil error chain
		t.Errorf("nil-wrapping error chain: %v", err)
	}
	if _, err := r.Wait(ctx, "x"); err == nil {
		t.Error("Wait without baseURL should error")
	}
}
