// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: security_scan — kick off a Mockarty Security Agent scan
// from a CI/CD pipeline, wait for completion, then download the SARIF
// report and upload it to GitHub Code Scanning (or your scanner of
// choice).
//
// This is the canonical CI/CD recipe. Admin operations (LLM profile
// CRUD, remote agent on/off, template editing) live in the admin UI,
// not the SDK.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client := mockarty.NewClient(
		envOr("MOCKARTY_URL", "http://localhost:5770"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(envOr("MOCKARTY_NAMESPACE", "sandbox")),
	)

	// 1) Start a passive scan against the configured target. Passive is
	//    the safest intensity — no exploitation, only observation. Use
	//    "safe-active" for routine CI scans, "intrusive" only against
	//    pre-prod, and "destructive" against ephemeral test envs.
	rep, err := client.Security().StartScan(ctx, mockarty.StartScanRequest{
		Title:     "ci-nightly",
		Namespace: envOr("MOCKARTY_NAMESPACE", "sandbox"),
		Profile: mockarty.SecurityScanProfile{
			Intensity:        "passive",
			ScopeDescription: envOr("SCAN_TARGET", "https://api.example.com"),
			Targets: []mockarty.SecurityTarget{
				{URL: envOr("SCAN_TARGET", "https://api.example.com"), Method: "GET"},
			},
			MaxCostUSDMicros:     5_000_000, // 5 USD ceiling
			RedactTokensInReport: true,
		},
	})
	if err != nil {
		log.Fatalf("StartScan: %v", err)
	}
	fmt.Printf("started: report %s (status=%s)\n", rep.ID, rep.Status)

	// 2) Poll the report until it reaches a terminal state. The server
	//    cleans up the queued row on cancellation, so a missing report
	//    means the run never started.
	for {
		select {
		case <-ctx.Done():
			log.Fatalf("timeout waiting for scan: %v", ctx.Err())
		case <-time.After(10 * time.Second):
		}
		got, err := client.Security().GetReport(ctx, rep.ID)
		if err != nil {
			log.Fatalf("GetReport: %v", err)
		}
		fmt.Printf("  status=%s tokens=%d cost_usd_micros=%d\n",
			got.Status, got.CostTokens, got.CostUSDMicros)
		if got.Status == "done" || got.Status == "failed" || got.Status == "cancelled" {
			rep = got
			break
		}
	}
	if rep.Status != "done" {
		log.Fatalf("scan ended in non-success state: %s", rep.Status)
	}

	// 3) List high+critical findings — typical CI gate.
	highs, err := client.Security().ListFindings(ctx, rep.ID, mockarty.ListFindingsOptions{
		Severity: "high",
	})
	if err != nil {
		log.Fatalf("ListFindings: %v", err)
	}
	crits, _ := client.Security().ListFindings(ctx, rep.ID, mockarty.ListFindingsOptions{
		Severity: "critical",
	})
	fmt.Printf("findings: %d high, %d critical\n", len(highs), len(crits))

	// 4) Download SARIF and persist for the CI step that uploads to
	//    GitHub / GitLab / Bitbucket Code Scanning.
	sarif, err := client.Security().ExportReport(ctx, rep.ID, "sarif")
	if err != nil {
		log.Fatalf("ExportReport: %v", err)
	}
	out := envOr("SARIF_OUTPUT", "mockarty-security.sarif.json")
	if err := os.WriteFile(out, sarif, 0o644); err != nil {
		log.Fatalf("write %s: %v", out, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", out, len(sarif))

	// 5) CI exit code: fail the pipeline on any critical finding.
	if len(crits) > 0 {
		os.Exit(2)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
