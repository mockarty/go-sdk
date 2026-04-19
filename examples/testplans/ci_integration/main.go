// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/ci_integration — full CI scenario:
//   1. Attach a webhook to a Plan
//   2. Trigger the Plan
//   3. Wait for the result
//   4. Download the Allure zip to ./report.zip
//   5. Exit with a non-zero status on failure so the CI job fails fast.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	planID := os.Getenv("PLAN_ID")
	if planID == "" {
		log.Fatal("PLAN_ID is required")
	}

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
		mockarty.WithRetry(3, 500*time.Millisecond),
	)
	api := client.TestPlans()

	// 1. Attach (or re-use) a CI webhook.
	_, err := api.AddWebhook(ctx, planID, mockarty.Webhook{
		URL:     os.Getenv("CI_WEBHOOK_URL"),
		Events:  []string{"run.completed", "run.failed"},
		Secret:  os.Getenv("CI_WEBHOOK_SECRET"),
		Enabled: true,
	})
	if err != nil {
		log.Printf("attach webhook: %v (continuing)", err)
	}

	// 2. Trigger.
	run, err := api.Run(ctx, planID, mockarty.RunOptions{})
	if err != nil {
		log.Fatalf("trigger: %v", err)
	}
	fmt.Printf("Triggered run %s\n", run.ID)

	// 3. Wait.
	final, err := api.WaitForRun(ctx, run.ID, 10*time.Second)
	passOrFail := "PASS"
	exitCode := 0
	switch {
	case err == nil:
		// pass
	case errors.Is(err, mockarty.ErrRunFailed):
		passOrFail = "FAIL"
		exitCode = 1
	case errors.Is(err, mockarty.ErrRunCancelled):
		passOrFail = "CANCELLED"
		exitCode = 2
	default:
		log.Fatalf("wait: %v", err)
	}

	// 4. Download the Allure zip for the CI artifact store.
	f, err := os.Create("report.zip")
	if err != nil {
		log.Fatalf("create report.zip: %v", err)
	}
	defer f.Close()
	if err := api.DownloadReportZip(ctx, run.ID, f); err != nil {
		log.Printf("download report: %v", err)
	}

	fmt.Printf("Final status: %s (run=%s, failed=%d, total=%d)\n",
		passOrFail, final.ID, final.FailedItems, final.TotalItems,
	)
	os.Exit(exitCode)
}
