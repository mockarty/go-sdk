// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/manual_run — CI/CD flow against a manual Test Plan:
//  1. Trigger the plan with executionModeOverride="manual" and recordDetailed=true.
//  2. Poll Me().AwaitingManual() until the first step appears.
//  3. Resolve each pending step pass/fail with notes — the kind of thing a
//     QA-on-call rotation script (or an LLM agent in the loop) would do.
//  4. Wait for the run to terminate, then download the standalone HTML report.
//
// Run with:
//
//	MOCKARTY_URL=http://localhost:5770 \
//	MOCKARTY_API_KEY=mk_xxx \
//	MOCKARTY_PLAN_ID=#42 \
//	go run ./...
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mockarty/mockarty-go"
)

func main() {
	url := envOr("MOCKARTY_URL", "http://localhost:5770")
	key := mustEnv("MOCKARTY_API_KEY")
	planID := mustEnv("MOCKARTY_PLAN_ID")
	ns := envOr("MOCKARTY_NAMESPACE", "sandbox")

	client := mockarty.NewClient(url,
		mockarty.WithAPIKey(key),
		mockarty.WithNamespace(ns),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 1. Trigger a manual run.
	run, err := client.TestPlans().RunManual(ctx, planID, mockarty.RunManualOptions{
		ExecutionModeOverride: "manual",
		RecordDetailed:        true,
		NotifyOnCompletion:    true,
		NotifyEmails:          []string{"qa-lead@example.com"},
	})
	if err != nil {
		log.Fatalf("trigger: %v", err)
	}
	fmt.Printf("Triggered run %s for plan %s (status=%s)\n", run.ID, run.PlanID, run.Status)

	// 2 + 3. Poll the bell counter, resolve every pending step with pass.
	// Production code would consult an actual QA runbook here; this loop just
	// demonstrates the cascade.
	deadline := time.Now().Add(4 * time.Minute)
	resolved := map[string]bool{}
	for time.Now().Before(deadline) {
		am, err := client.Me().AwaitingManual(ctx)
		if err != nil {
			log.Printf("awaiting-manual fetch: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if am.Count == 0 {
			// Either run finished or nothing pending right now.
			st, err := client.TestPlans().GetRunStatus(ctx, run.ID)
			if err == nil && (st.Status == "completed" || st.Status == "failed" || st.Status == "cancelled") {
				break
			}
			time.Sleep(2 * time.Second)
			continue
		}
		for _, item := range am.Items {
			if resolved[item.StepRunID] || item.PlanRunID != run.ID {
				continue
			}
			err := client.TestPlans().ResolveStep(ctx, item.RunID, item.StepUID, mockarty.ResolveStepOptions{
				Resolution: mockarty.StepResolutionPass,
				Note:       "auto-resolved by CI manual_run example",
				NoteFmt:    "plain",
				Namespace:  item.Namespace,
			})
			if err != nil {
				log.Printf("resolve %s: %v", item.StepUID, err)
				continue
			}
			resolved[item.StepRunID] = true
			fmt.Printf("Resolved step %s (case-run %s)\n", item.StepUID, item.RunID)
		}
		time.Sleep(1 * time.Second)
	}

	// 4. Download the standalone HTML report.
	out, err := client.TestPlans().GetRunReportHTML(ctx, ns, planID, run.ID)
	if err != nil {
		log.Fatalf("html report: %v", err)
	}
	if err := os.WriteFile("report.html", out, 0o644); err != nil {
		log.Fatalf("write report: %v", err)
	}
	fmt.Println("Saved report.html")
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s is required", k)
	}
	return v
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
