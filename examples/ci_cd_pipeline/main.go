// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: ci_cd_pipeline — Full CI/CD automation with Mockarty.
//
// This example demonstrates a complete CI/CD pipeline that:
//  1. Creates an isolated namespace for the test run
//  2. Imports an OpenAPI spec and generates mocks
//  3. Runs contract validation (pact publish + verify)
//  4. Executes a test collection
//  5. Runs fuzzing against the service
//  6. Runs a performance test
//  7. Checks all results and exports reports
//  8. Cleans up everything
//
// Use this as a template for integrating Mockarty into your CI/CD pipeline
// (GitHub Actions, GitLab CI, Jenkins, etc.).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	// In CI/CD, read configuration from environment variables
	baseURL := getEnv("MOCKARTY_URL", "http://localhost:5770")
	apiKey := getEnv("MOCKARTY_API_KEY", "your-api-key")
	targetURL := getEnv("TARGET_URL", "http://localhost:5770")
	specURL := getEnv("SPEC_URL", "http://localhost:5770/swagger/doc.json")

	// Use a unique namespace per CI run to avoid collisions
	buildID := getEnv("BUILD_ID", fmt.Sprintf("ci-%d", time.Now().Unix()))
	namespace := "ci-" + buildID

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client := mockarty.NewClient(baseURL,
		mockarty.WithAPIKey(apiKey),
		mockarty.WithNamespace(namespace),
	)

	fmt.Println("==========================================================")
	fmt.Printf("  Mockarty CI/CD Pipeline - Build %s\n", buildID)
	fmt.Println("==========================================================")
	fmt.Printf("  Mockarty:  %s\n", baseURL)
	fmt.Printf("  Target:    %s\n", targetURL)
	fmt.Printf("  Namespace: %s\n", namespace)
	fmt.Printf("  Spec:      %s\n", specURL)
	fmt.Println("==========================================================")

	// Track overall pipeline result
	pipelineOK := true
	defer func() {
		fmt.Println("\n==========================================================")
		if pipelineOK {
			fmt.Println("  PIPELINE RESULT: PASSED")
		} else {
			fmt.Println("  PIPELINE RESULT: FAILED")
		}
		fmt.Println("==========================================================")
	}()

	// -----------------------------------------------------------------------
	// Step 1: Create an isolated namespace for this CI run
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 1] Creating namespace...")

	err := client.Namespaces().Create(ctx, namespace)
	if err != nil {
		fmt.Printf("  Create namespace returned: %v (may already exist)\n", err)
	} else {
		fmt.Printf("  Created namespace: %s\n", namespace)
	}

	// Clean up namespace at the end of the pipeline
	defer func() {
		fmt.Println("\n[Cleanup] Removing test namespace...")
		// Delete all mocks in the namespace
		list, err := client.Mocks().List(ctx, &mockarty.ListMocksOptions{
			Namespace: namespace,
			Limit:     1000,
		})
		if err == nil {
			for _, m := range list.Items {
				_ = client.Mocks().Delete(ctx, m.ID)
			}
			fmt.Printf("  Deleted %d mocks\n", len(list.Items))
		}
		fmt.Println("  Cleanup complete")
	}()

	// -----------------------------------------------------------------------
	// Step 2: Import OpenAPI spec and generate mocks
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 2] Generating mocks from OpenAPI spec...")

	genResult, err := client.Generator().FromOpenAPI(ctx, &mockarty.GeneratorRequest{
		URL:        specURL,
		Namespace:  namespace,
		PathPrefix: "/mocked",
		ServerName: "ci-test-service",
	})
	if err != nil {
		fmt.Printf("  WARN: Mock generation failed: %v\n", err)
		fmt.Println("  Continuing with existing mocks...")
	} else {
		fmt.Printf("  Generated %d mocks from OpenAPI spec\n", genResult.Created)
		for i, m := range genResult.Mocks {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(genResult.Mocks)-5)
				break
			}
			route := ""
			if m.HTTP != nil {
				route = m.HTTP.HttpMethod + " " + m.HTTP.Route
			}
			fmt.Printf("  - %s (%s)\n", m.ID, route)
		}
	}

	// Verify mocks were created
	mockList, err := client.Mocks().List(ctx, &mockarty.ListMocksOptions{
		Namespace: namespace,
		Limit:     100,
	})
	if err != nil {
		fmt.Printf("  WARN: Could not list mocks: %v\n", err)
	} else {
		fmt.Printf("  Total mocks in namespace: %d\n", mockList.Total)
	}

	// -----------------------------------------------------------------------
	// Step 3: Contract validation (Pact publish + verify)
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 3] Running contract validation...")

	// 3a. Publish a consumer pact
	pact, err := client.Contracts().PublishPact(ctx, &mockarty.Pact{
		Consumer:  "ci-frontend",
		Provider:  "ci-service",
		Version:   buildID,
		Namespace: namespace,
		Spec: `{
			"consumer": {"name": "ci-frontend"},
			"provider": {"name": "ci-service"},
			"interactions": [
				{
					"description": "health check",
					"request": {"method": "GET", "path": "/health"},
					"response": {"status": 200}
				}
			]
		}`,
	})
	if err != nil {
		fmt.Printf("  WARN: Pact publish failed: %v\n", err)
	} else {
		fmt.Printf("  Published pact: %s (consumer=%s, provider=%s)\n",
			pact.ID, pact.Consumer, pact.Provider)

		// 3b. Verify the pact
		verification, err := client.Contracts().VerifyPact(ctx, map[string]any{
			"consumer":  "ci-frontend",
			"provider":  "ci-service",
			"targetUrl": targetURL,
		})
		if err != nil {
			fmt.Printf("  WARN: Pact verification failed: %v\n", err)
		} else {
			fmt.Printf("  Verification status: %s\n", verification.Status)
			if verification.Status == "fail" {
				fmt.Printf("  Violations: %d\n", len(verification.Violations))
				for _, v := range verification.Violations {
					fmt.Printf("    [%s] %s: %s\n", v.Severity, v.Path, v.Message)
				}
			}
		}

		// 3c. Can-I-Deploy check
		deployCheck, err := client.Contracts().CanIDeploy(ctx, map[string]any{
			"service": "ci-service",
			"version": buildID,
		})
		if err != nil {
			fmt.Printf("  WARN: Can-I-Deploy check failed: %v\n", err)
		} else {
			if deployCheck.OK {
				fmt.Println("  Can-I-Deploy: YES - safe to deploy")
			} else {
				fmt.Printf("  Can-I-Deploy: NO - %s\n", deployCheck.Reason)
				pipelineOK = false
			}
		}
	}

	// 3d. Validate mocks against contract
	contractResult, err := client.Contracts().ValidateMocks(ctx, &mockarty.ContractValidationRequest{
		SpecURL:   specURL,
		Namespace: namespace,
	})
	if err != nil {
		fmt.Printf("  WARN: Mock validation failed: %v\n", err)
	} else {
		fmt.Printf("  Mock validation: status=%s, violations=%d\n",
			contractResult.Status, contractResult.Violations)
		if contractResult.Status == "fail" {
			pipelineOK = false
		}
	}

	// -----------------------------------------------------------------------
	// Step 4: Execute test collection
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 4] Executing test collection...")

	// Set up an environment for this CI run
	ciEnv, err := client.Environments().Create(ctx, &mockarty.Environment{
		Name: "CI-" + buildID,
		Variables: map[string]string{
			"BASE_URL": targetURL,
			"API_KEY":  apiKey,
			"BUILD_ID": buildID,
		},
	})
	if err != nil {
		fmt.Printf("  WARN: Create environment failed: %v\n", err)
	} else {
		_ = client.Environments().Activate(ctx, ciEnv.ID)
		defer func() {
			_ = client.Environments().Delete(ctx, ciEnv.ID)
		}()
	}

	// Try to find or create a test collection
	collections, err := client.Collections().List(ctx)
	if err != nil {
		fmt.Printf("  WARN: List collections failed: %v\n", err)
	} else if len(collections) > 0 {
		colID := collections[0].ID
		fmt.Printf("  Executing collection: %s (%s)\n", collections[0].Name, colID)

		testResult, err := client.Collections().Execute(ctx, colID)
		if err != nil {
			fmt.Printf("  WARN: Collection execution failed: %v\n", err)
		} else {
			fmt.Printf("  Test results: total=%d, passed=%d, failed=%d, skipped=%d\n",
				testResult.TotalTests, testResult.PassedTests,
				testResult.FailedTests, testResult.SkippedTests)
			fmt.Printf("  Duration: %d ms\n", testResult.Duration)

			if testResult.FailedTests > 0 {
				fmt.Println("  FAILED TESTS:")
				for _, tr := range testResult.Results {
					if tr.Status != "passed" {
						fmt.Printf("    [FAIL] %s: %s\n", tr.RequestName, tr.Error)
					}
				}
				pipelineOK = false
			}

			// Export test run for CI artifacts
			if testResult.ID != "" {
				exportData, err := client.TestRuns().Export(ctx, testResult.ID)
				if err == nil {
					fmt.Printf("  Exported test report: %d bytes\n", len(exportData))
				}
			}
		}
	} else {
		fmt.Println("  No collections found, skipping test execution")
	}

	// -----------------------------------------------------------------------
	// Step 5: Run fuzzing against the service
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 5] Running fuzzing...")

	fuzzRun, err := client.Fuzzing().Start(ctx, &mockarty.FuzzingConfig{
		Name:          "CI Fuzz - " + buildID,
		TargetBaseURL: targetURL,
		SourceType:    "openapi",
		Strategy:      "all",
		Namespace:     namespace,
	})
	if err != nil {
		fmt.Printf("  WARN: Fuzzing start failed: %v\n", err)
		fmt.Println("  (Fuzzing may require specific license tier)")
	} else {
		fmt.Printf("  Fuzzing run started: id=%s\n", fuzzRun.ID)

		// Wait for fuzzing to complete
		fmt.Println("  Waiting for fuzzing to complete...")
		time.Sleep(20 * time.Second)

		fuzzResult, err := client.Fuzzing().GetResult(ctx, fuzzRun.ID)
		if err != nil {
			fmt.Printf("  WARN: Get fuzzing result failed: %v\n", err)
		} else {
			fmt.Printf("  Fuzzing results: status=%s, requests=%d, findings=%d\n",
				fuzzResult.Status, fuzzResult.TotalRequests, fuzzResult.TotalFindings)

			// Check for critical findings
			if fuzzResult.TotalFindings > 0 {
				findings, err := client.Fuzzing().ListFindings(ctx)
				if err == nil {
					criticalCount := 0
					for _, f := range findings {
						if f.Severity == "critical" || f.Severity == "high" {
							criticalCount++
							fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Title)
						}
					}
					if criticalCount > 0 {
						fmt.Printf("  CRITICAL/HIGH findings: %d - blocking deployment\n", criticalCount)
						pipelineOK = false
					}
				}

				// Export findings for CI artifacts
				exportData, err := client.Fuzzing().ExportFindings(ctx, map[string]any{
					"format": "json",
				})
				if err == nil {
					fmt.Printf("  Exported findings report: %d bytes\n", len(exportData))
				}
			}
		}

		// Stop fuzzing in case it's still running
		_ = client.Fuzzing().Stop(ctx, fuzzRun.ID)
	}

	// -----------------------------------------------------------------------
	// Step 6: Run performance test
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 6] Running performance test...")

	perfTask, err := client.Perf().Run(ctx, &mockarty.PerfConfig{
		Name:     "CI Perf - " + buildID,
		Duration: "10s",
		VUs:      5,
		Script: fmt.Sprintf(`
			import http from 'k6/http';
			import { check } from 'k6';

			export default function() {
				const res = http.get('%s/health');
				check(res, { 'status is 200': (r) => r.status === 200 });
			}
		`, targetURL),
		Thresholds: map[string]any{
			"http_req_duration": "p(95)<500",
			"http_req_failed":   "rate<0.01",
		},
	})
	if err != nil {
		fmt.Printf("  WARN: Performance test start failed: %v\n", err)
		fmt.Println("  (Performance testing may require specific license tier)")
	} else {
		fmt.Printf("  Performance test started: id=%s\n", perfTask.ID)

		// Wait for test to complete
		fmt.Println("  Waiting for performance test to complete...")
		time.Sleep(15 * time.Second)

		perfResult, err := client.Perf().GetResult(ctx, perfTask.ID)
		if err != nil {
			fmt.Printf("  WARN: Get perf result failed: %v\n", err)
		} else {
			fmt.Printf("  Perf results: status=%s, duration=%dms, vus=%d, iterations=%d\n",
				perfResult.Status, perfResult.Duration, perfResult.VUs, perfResult.Iterations)

			if perfResult.Metrics != nil {
				fmt.Println("  Metrics:")
				for key, val := range perfResult.Metrics {
					fmt.Printf("    %s: %v\n", key, val)
				}
			}

			if len(perfResult.Errors) > 0 {
				fmt.Println("  Errors:")
				for _, e := range perfResult.Errors {
					fmt.Printf("    %s (count=%d)\n", e.Message, e.Count)
				}
			}

			// Check thresholds
			if perfResult.Status == "failed" {
				fmt.Println("  Performance test FAILED thresholds")
				pipelineOK = false
			}
		}
	}

	// -----------------------------------------------------------------------
	// Step 7: Check results and export reports
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 7] Collecting results...")

	// Get system stats as a pipeline summary
	stats, err := client.Stats().GetCounts(ctx)
	if err == nil {
		fmt.Println("  Pipeline resource counts:")
		for key, val := range stats {
			fmt.Printf("    %s: %v\n", key, val)
		}
	}

	// Check for any undefined requests (mocks we may have missed)
	undefined, err := client.Undefined().List(ctx)
	if err == nil && len(undefined) > 0 {
		fmt.Printf("\n  WARNING: %d unmatched requests detected:\n", len(undefined))
		for _, u := range undefined {
			fmt.Printf("    - %s %s (count=%d)\n", u.Method, u.Path, u.Count)
		}
		fmt.Println("  Consider creating mocks for these endpoints")
	}

	// -----------------------------------------------------------------------
	// Step 8: Cleanup (handled by deferred functions above)
	// -----------------------------------------------------------------------
	fmt.Println("\n[Step 8] Running cleanup...")
	// Deferred cleanup functions handle:
	// - Deleting mocks in the namespace
	// - Deleting the CI environment
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
