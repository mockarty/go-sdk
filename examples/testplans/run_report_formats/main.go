// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/run_report_formats — fetch a Test Plan run and export
// every available report format to disk.
//
// Formats produced:
//   - report.json          Allure-compatible JSON
//   - report.zip           Allure directory as a zip
//   - report.junit.xml     Standards-compliant JUnit XML
//   - report.md            Markdown human summary
//   - report.unified.json  Native Mockarty unified envelope (typed decode)
//
// Usage:
//
//	MOCKARTY_SERVER=http://127.0.0.1:8080 \
//	MOCKARTY_TOKEN=mk_... \
//	MOCKARTY_NAMESPACE=default \
//	PLAN_REF=my-plan-or-uuid-or-numeric-id \
//	RUN_ID=<uuid> \
//	go run .
//
// Note: PLAN_REF accepts a plan slug, UUID, or numeric ID. The run must
// belong to that plan and be in a terminal state (passed/failed/skipped/
// broken) to yield useful report data.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	namespace := mustEnv("MOCKARTY_NAMESPACE")
	planRef := mustEnv("PLAN_REF")
	runID := mustEnv("RUN_ID")

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(namespace),
		mockarty.WithRetry(3, 500*time.Millisecond),
	)
	api := client.TestPlans()

	// 1. Allure JSON — compatible with the Allure CLI `allure open`.
	allure, err := api.GetRunReport(ctx, namespace, planRef, runID)
	if err != nil {
		log.Fatalf("allure json: %v", err)
	}
	if err := os.WriteFile("report.json", allure.Raw, 0o644); err != nil {
		log.Fatalf("write report.json: %v", err)
	}
	fmt.Printf("wrote report.json (%d bytes)\n", len(allure.Raw))

	// 2. Allure ZIP — the full allure-results directory streamed as a zip.
	rc, err := api.GetRunReportZIP(ctx, namespace, planRef, runID)
	if err != nil {
		log.Fatalf("allure zip: %v", err)
	}
	zipFile, err := os.Create("report.zip")
	if err != nil {
		rc.Close()
		log.Fatalf("create report.zip: %v", err)
	}
	n, copyErr := io.Copy(zipFile, rc)
	rc.Close()
	if copyErr != nil {
		zipFile.Close()
		log.Fatalf("copy report.zip: %v", copyErr)
	}
	if err := zipFile.Close(); err != nil {
		log.Fatalf("close report.zip: %v", err)
	}
	fmt.Printf("wrote report.zip (%d bytes)\n", n)

	// 3. JUnit XML — for GitLab, Jenkins, GitHub Actions publishers.
	if err := writeFile(ctx, "report.junit.xml", func() ([]byte, error) {
		return api.GetRunReportJUnit(ctx, namespace, planRef, runID)
	}); err != nil {
		log.Fatalf("junit xml: %v", err)
	}

	// 4. Markdown — single-document summary for Slack / email / wiki.
	if err := writeFile(ctx, "report.md", func() ([]byte, error) {
		return api.GetRunReportMarkdown(ctx, namespace, planRef, runID)
	}); err != nil {
		log.Fatalf("markdown: %v", err)
	}

	// 5. Unified JSON — typed decode via mockarty.UnifiedReport. Also saved
	// to disk via the Raw bytes for downstream tooling.
	report, err := api.GetRunReportUnified(ctx, namespace, planRef, runID)
	if err != nil {
		log.Fatalf("unified json: %v", err)
	}
	if err := os.WriteFile("report.unified.json", report.Raw, 0o644); err != nil {
		log.Fatalf("write report.unified.json: %v", err)
	}
	fmt.Printf("wrote report.unified.json — plan=%q runID=%s items=%d "+
		"(passed=%d failed=%d skipped=%d broken=%d) duration=%dms\n",
		report.PlanName, report.RunID,
		report.Counts.Total, report.Counts.Passed, report.Counts.Failed,
		report.Counts.Skipped, report.Counts.Broken,
		report.DurationMs,
	)
}

func writeFile(_ context.Context, name string, fetch func() ([]byte, error)) error {
	data, err := fetch()
	if err != nil {
		return err
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", name, len(data))
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s is required", key)
	}
	return v
}
