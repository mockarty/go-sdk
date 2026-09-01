// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: external_runs_lifecycle — stream a long test's results incrementally.
//
// Unlike Report (one-shot upload of a finished run), the lifecycle API lets a
// long-running suite report as it goes: StartRun → AppendSteps (repeatedly) →
// FinishRun. The finished view carries the resolved TCM case/run ids the ingest
// matched or created, so a CI agent can chain straight to the case.
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
	baseURL := envOr("MOCKARTY_SERVER", "http://localhost:5770")
	client := mockarty.NewClient(baseURL,
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(envOr("MOCKARTY_NAMESPACE", "sandbox")))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	er := client.ExternalRuns()

	run, err := er.StartRun(ctx, "", mockarty.StartRunRequest{
		Name:      "checkout smoke",
		Framework: "custom",
		FullName:  "suites.checkout.smoke",
	})
	if err != nil {
		log.Fatalf("start run: %v", err)
	}
	fmt.Printf("started run %s\n", run.ID)

	// Report steps as the test progresses.
	for _, step := range []mockarty.LifecycleStep{
		{StepKey: "login", Name: "log in", Status: "passed", DurationMS: 120},
		{StepKey: "cart", Name: "add to cart", Status: "passed", DurationMS: 80},
		{StepKey: "pay", Name: "pay", Status: "failed", Message: "gateway 500", DurationMS: 210},
	} {
		run, err = er.AppendStepsAtRevision(ctx, "", run.ID, run.Revision, []mockarty.LifecycleStep{step})
		if err != nil {
			log.Fatalf("append step %s: %v", step.StepKey, err)
		}
	}

	fin, err := er.FinishRunAtRevision(ctx, "", run.ID, run.Revision, mockarty.FinishRunRequest{
		Status: "failed", Summary: "payment gateway returned 500",
	})
	if err != nil {
		log.Fatalf("finish run: %v", err)
	}
	fmt.Printf("finished: status=%s case=%s run=%s\n",
		fin.Status, fin.ResolvedCaseID, fin.ResolvedRunID)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
