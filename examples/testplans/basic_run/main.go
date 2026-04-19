// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/basic_run — create a Test Plan, trigger it, and wait for
// completion via the unified SDK.
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
	)

	plan, err := client.TestPlans().Create(ctx, mockarty.TestPlan{
		Name:        "Nightly smoke",
		Description: "Functional + fuzz sweep",
		Schedule:    "", // sequential (FIFO) by default
		Items: []mockarty.TestPlanItem{
			{
				Order:      1,
				Type:       mockarty.PlanItemTypeFunctional,
				ResourceID: os.Getenv("MOCKARTY_COLLECTION_ID"),
				Name:       "Smoke collection",
			},
			{
				Order:      2,
				Type:       mockarty.PlanItemTypeFuzz,
				ResourceID: os.Getenv("MOCKARTY_FUZZ_CONFIG_ID"),
			},
		},
	})
	if err != nil {
		log.Fatalf("create plan: %v", err)
	}
	fmt.Printf("Created plan %s (#%d)\n", plan.ID, plan.NumericID)

	run, err := client.TestPlans().Run(ctx, plan.ID, mockarty.RunOptions{})
	if err != nil {
		log.Fatalf("run plan: %v", err)
	}
	fmt.Printf("Triggered run %s (status=%s)\n", run.ID, run.Status)

	final, err := client.TestPlans().WaitForRun(ctx, run.ID, 5*time.Second)
	if err != nil {
		log.Fatalf("wait: %v", err)
	}
	fmt.Printf("Run finished: %s (%d/%d passed)\n",
		final.Status,
		final.CompletedItems-final.FailedItems,
		final.TotalItems,
	)
}
