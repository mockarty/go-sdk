// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: test_runs_report — fetch the unified test-run report in every
// supported format for any mode (functional / load / fuzz / chaos / contract
// / merged).
//
// Usage:
//
//	MOCKARTY_SERVER=http://127.0.0.1:8080 \
//	MOCKARTY_TOKEN=mk_... \
//	MOCKARTY_NAMESPACE=default \
//	RUN_ID=<uuid> \
//	go run .
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	runID := mustEnv("RUN_ID")

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
		mockarty.WithRetry(3, 500*time.Millisecond),
	)
	api := client.TestRuns()

	targets := []struct {
		format string
		out    string
	}{
		{mockarty.TestRunReportFormatUnifiedJSON, "run.unified.json"},
		{mockarty.TestRunReportFormatAllureJSON, "run.allure.json"},
		{mockarty.TestRunReportFormatAllureZip, "run.allure.zip"},
		{mockarty.TestRunReportFormatJUnit, "run.junit.xml"},
		{mockarty.TestRunReportFormatMarkdown, "run.md"},
		{mockarty.TestRunReportFormatHTML, "run.html"},
	}
	for _, t := range targets {
		data, err := api.GetTestRunReport(ctx, runID, t.format)
		if err != nil {
			log.Fatalf("%s: %v", t.format, err)
		}
		if err := os.WriteFile(t.out, data, 0o644); err != nil {
			log.Fatalf("write %s: %v", t.out, err)
		}
		fmt.Printf("wrote %s (%d bytes, format=%s)\n", t.out, len(data), t.format)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}
