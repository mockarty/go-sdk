// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: fuzzing — Fuzzing API usage with findings, schedules, and imports.
//
// This example demonstrates:
//   - Create a fuzzing configuration
//   - Start a fuzzing run and poll for results
//   - List and triage security findings
//   - Replay and analyze findings with AI
//   - Import fuzzing targets from cURL, OpenAPI, and collections
//   - Create and manage fuzzing schedules
//   - Quick fuzz and summary
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. Create a fuzzing configuration
	// -----------------------------------------------------------------------
	fmt.Println("--- Create Fuzzing Config ---")

	config, err := client.Fuzzing().CreateConfig(ctx, &mockarty.FuzzingConfig{
		Name:      "User API Fuzz Test",
		TargetURL: "http://localhost:5770",
		SpecURL:   "http://localhost:5770/swagger/doc.json",
		Duration:  "30s",
		Workers:   4,
	})
	if err != nil {
		log.Fatalf("Failed to create fuzzing config: %v", err)
	}
	fmt.Printf("Created fuzzing config: id=%s, name=%s\n", config.ID, config.Name)

	// Retrieve the config to verify
	retrieved, err := client.Fuzzing().GetConfig(ctx, config.ID)
	if err != nil {
		fmt.Printf("Get config returned: %v\n", err)
	} else {
		fmt.Printf("Retrieved config: name=%s, targetUrl=%s, workers=%d\n",
			retrieved.Name, retrieved.TargetURL, retrieved.Workers)
	}

	// -----------------------------------------------------------------------
	// 2. Start a fuzzing run
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Start Fuzzing Run ---")

	run, err := client.Fuzzing().Start(ctx, &mockarty.FuzzingConfig{
		Name:      "Quick Fuzz",
		TargetURL: "http://localhost:5770",
		SpecURL:   "http://localhost:5770/swagger/doc.json",
		Duration:  "10s",
		Workers:   2,
	})
	if err != nil {
		fmt.Printf("Start fuzzing returned: %v\n", err)
		fmt.Println("(Fuzzing requires a license with fuzzing enabled)")
	} else {
		fmt.Printf("Fuzzing run started: id=%s, status=%s\n", run.ID, run.Status)

		// Poll for results
		fmt.Println("\n--- Poll Fuzzing Results ---")
		fmt.Println("Waiting for fuzzing run to complete...")
		time.Sleep(15 * time.Second)

		result, err := client.Fuzzing().GetResult(ctx, run.ID)
		if err != nil {
			fmt.Printf("Get result returned: %v\n", err)
		} else {
			fmt.Printf("Fuzzing result:\n")
			fmt.Printf("  ID: %s\n", result.ID)
			fmt.Printf("  Status: %s\n", result.Status)
			fmt.Printf("  Total requests: %d\n", result.TotalRequests)
			fmt.Printf("  Findings: %d\n", result.Findings)

			if result.StartedAt > 0 {
				fmt.Printf("  Started: %s\n", time.Unix(result.StartedAt, 0).Format(time.RFC3339))
			}
			if result.FinishedAt > 0 {
				fmt.Printf("  Finished: %s\n", time.Unix(result.FinishedAt, 0).Format(time.RFC3339))
			}
		}

		// Stop the run if still going
		_ = client.Fuzzing().Stop(ctx, run.ID)
	}

	// -----------------------------------------------------------------------
	// 3. Findings management
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Fuzzing Findings ---")

	// List all security findings discovered during fuzzing runs
	findings, err := client.Fuzzing().ListFindings(ctx)
	if err != nil {
		fmt.Printf("List findings returned: %v\n", err)
	} else {
		fmt.Printf("Found %d security findings:\n", len(findings))
		for _, f := range findings {
			fmt.Printf("  - [%s] %s: %s (triage=%s)\n",
				f.Severity, f.Type, f.Title, f.TriageStatus)
		}

		// If we have findings, demonstrate triage, replay, and analysis
		if len(findings) > 0 {
			findingID := findings[0].ID

			// Get a single finding with full details
			fmt.Println("\n--- Get Finding Details ---")
			detail, err := client.Fuzzing().GetFinding(ctx, findingID)
			if err != nil {
				fmt.Printf("Get finding returned: %v\n", err)
			} else {
				fmt.Printf("Finding detail:\n")
				fmt.Printf("  ID: %s\n", detail.ID)
				fmt.Printf("  Type: %s\n", detail.Type)
				fmt.Printf("  Severity: %s\n", detail.Severity)
				fmt.Printf("  Description: %s\n", detail.Description)
			}

			// Triage a finding — mark it as confirmed, false_positive, or ignored
			fmt.Println("\n--- Triage Finding ---")
			err = client.Fuzzing().TriageFinding(ctx, findingID, "confirmed",
				"Verified: SQL injection in user search endpoint")
			if err != nil {
				fmt.Printf("Triage finding returned: %v\n", err)
			} else {
				fmt.Println("Finding triaged as 'confirmed'")
			}

			// Replay the request that triggered the finding
			fmt.Println("\n--- Replay Finding ---")
			err = client.Fuzzing().ReplayFinding(ctx, findingID)
			if err != nil {
				fmt.Printf("Replay finding returned: %v\n", err)
			} else {
				fmt.Println("Finding replayed successfully")
			}

			// Run AI analysis on the finding
			fmt.Println("\n--- Analyze Finding with AI ---")
			analysis, err := client.Fuzzing().AnalyzeFinding(ctx, findingID)
			if err != nil {
				fmt.Printf("Analyze finding returned: %v\n", err)
			} else {
				fmt.Printf("AI analysis result: %v\n", analysis)
			}

			// Batch operations on findings
			if len(findings) >= 2 {
				batchIDs := []string{findings[0].ID, findings[1].ID}

				fmt.Println("\n--- Batch Triage Findings ---")
				err = client.Fuzzing().BatchTriageFindings(ctx, batchIDs, "confirmed")
				if err != nil {
					fmt.Printf("Batch triage returned: %v\n", err)
				} else {
					fmt.Println("Batch triaged 2 findings as 'confirmed'")
				}

				fmt.Println("\n--- Batch Analyze Findings ---")
				err = client.Fuzzing().BatchAnalyzeFindings(ctx, batchIDs)
				if err != nil {
					fmt.Printf("Batch analyze returned: %v\n", err)
				} else {
					fmt.Println("Batch analysis started for 2 findings")
				}
			}

			// Export findings
			fmt.Println("\n--- Export Findings ---")
			exportData, err := client.Fuzzing().ExportFindings(ctx, map[string]any{
				"format": "json",
			})
			if err != nil {
				fmt.Printf("Export findings returned: %v\n", err)
			} else {
				fmt.Printf("Exported findings: %d bytes\n", len(exportData))
			}
		}
	}

	// -----------------------------------------------------------------------
	// 4. Import fuzzing targets
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Import Fuzzing Targets ---")

	// Import from a cURL command — useful for quick one-off fuzzing
	err = client.Fuzzing().ImportFromCurl(ctx,
		`curl -X POST http://localhost:5770/api/users -H "Content-Type: application/json" -d '{"name":"test","email":"test@example.com"}'`)
	if err != nil {
		fmt.Printf("Import from cURL returned: %v\n", err)
	} else {
		fmt.Println("Imported fuzzing target from cURL command")
	}

	// Import from an OpenAPI spec — discovers all endpoints to fuzz
	err = client.Fuzzing().ImportFromOpenAPI(ctx, map[string]any{
		"specUrl": "http://localhost:5770/swagger/doc.json",
	})
	if err != nil {
		fmt.Printf("Import from OpenAPI returned: %v\n", err)
	} else {
		fmt.Println("Imported fuzzing targets from OpenAPI spec")
	}

	// Import from an existing collection — reuse API tester collections
	err = client.Fuzzing().ImportFromCollection(ctx, map[string]any{
		"collectionId": "my-api-tests",
	})
	if err != nil {
		fmt.Printf("Import from collection returned: %v\n", err)
	} else {
		fmt.Println("Imported fuzzing targets from collection")
	}

	// Import from a recorder session
	err = client.Fuzzing().ImportFromRecorder(ctx, map[string]any{
		"sessionId": "recorded-session-1",
	})
	if err != nil {
		fmt.Printf("Import from recorder returned: %v\n", err)
	} else {
		fmt.Println("Imported fuzzing targets from recorder session")
	}

	// Import from a mock definition
	err = client.Fuzzing().ImportFromMock(ctx, map[string]any{
		"mockId": "user-api-mock",
	})
	if err != nil {
		fmt.Printf("Import from mock returned: %v\n", err)
	} else {
		fmt.Println("Imported fuzzing targets from mock")
	}

	// -----------------------------------------------------------------------
	// 5. Fuzzing schedules
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Fuzzing Schedules ---")

	// Create a schedule to run fuzzing automatically (e.g. nightly)
	schedule, err := client.Fuzzing().CreateSchedule(ctx, &mockarty.FuzzingSchedule{
		ConfigID: config.ID,
		Cron:     "0 2 * * *", // every night at 2 AM
		Enabled:  true,
	})
	if err != nil {
		fmt.Printf("Create schedule returned: %v\n", err)
	} else {
		fmt.Printf("Created fuzzing schedule: id=%s, cron=%s, enabled=%t\n",
			schedule.ID, schedule.Cron, schedule.Enabled)

		// Update the schedule
		updated, err := client.Fuzzing().UpdateSchedule(ctx, schedule.ID, &mockarty.FuzzingSchedule{
			ConfigID: config.ID,
			Cron:     "0 3 * * 1", // weekly on Monday at 3 AM
			Enabled:  true,
		})
		if err != nil {
			fmt.Printf("Update schedule returned: %v\n", err)
		} else {
			fmt.Printf("Updated schedule cron to: %s\n", updated.Cron)
		}

		// Get a specific schedule
		got, err := client.Fuzzing().GetSchedule(ctx, schedule.ID)
		if err != nil {
			fmt.Printf("Get schedule returned: %v\n", err)
		} else {
			fmt.Printf("Retrieved schedule: cron=%s, enabled=%t\n", got.Cron, got.Enabled)
		}

		// Delete the schedule
		err = client.Fuzzing().DeleteSchedule(ctx, schedule.ID)
		if err != nil {
			fmt.Printf("Delete schedule returned: %v\n", err)
		} else {
			fmt.Println("Schedule deleted")
		}
	}

	// List all schedules
	schedules, err := client.Fuzzing().ListSchedules(ctx)
	if err != nil {
		fmt.Printf("List schedules returned: %v\n", err)
	} else {
		fmt.Printf("Found %d fuzzing schedules:\n", len(schedules))
		for _, s := range schedules {
			fmt.Printf("  - id=%s, config=%s, cron=%s, enabled=%t\n",
				s.ID, s.ConfigID, s.Cron, s.Enabled)
		}
	}

	// -----------------------------------------------------------------------
	// 6. Quick Fuzz — minimal configuration for fast exploration
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Quick Fuzz ---")

	quickRun, err := client.Fuzzing().QuickFuzz(ctx, map[string]any{
		"targetUrl": "http://localhost:5770/api/v1/mocks",
		"method":    "GET",
	})
	if err != nil {
		fmt.Printf("Quick fuzz returned: %v\n", err)
	} else {
		fmt.Printf("Quick fuzz started: id=%s, status=%s\n", quickRun.ID, quickRun.Status)
	}

	// -----------------------------------------------------------------------
	// 7. Fuzzing summary
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Fuzzing Summary ---")

	summary, err := client.Fuzzing().GetSummary(ctx)
	if err != nil {
		fmt.Printf("Get summary returned: %v\n", err)
	} else {
		fmt.Println("Fuzzing summary:")
		for key, val := range summary {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 8. List all fuzzing results
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List All Fuzzing Results ---")

	results, err := client.Fuzzing().ListResults(ctx)
	if err != nil {
		fmt.Printf("List results returned: %v\n", err)
	} else {
		fmt.Printf("Found %d fuzzing results:\n", len(results))
		for _, r := range results {
			fmt.Printf("  - id=%s, status=%s, requests=%d, findings=%d\n",
				r.ID, r.Status, r.TotalRequests, r.Findings)
		}
	}

	fmt.Println("\nFuzzing examples completed!")
}
