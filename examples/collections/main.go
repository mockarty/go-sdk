// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: collections — API Tester collection management with environments and scheduling.
//
// This example demonstrates:
//   - Create and list collections
//   - Execute a collection
//   - Execute multiple collections
//   - Export a collection
//   - Use environments with collection execution
//   - Run performance tests from collections
//   - Duplicate collections for different environments
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. List all collections
	// -----------------------------------------------------------------------
	fmt.Println("--- List Collections ---")

	collections, err := client.Collections().List(ctx)
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	}

	fmt.Printf("Found %d collections:\n", len(collections))
	for _, col := range collections {
		fmt.Printf("  - %s (id=%s, protocol=%s, type=%s)\n",
			col.Name, col.ID, col.Protocol, col.CollectionType)
	}

	// If no collections exist, create one for demonstration
	if len(collections) == 0 {
		fmt.Println("\nNo collections found. Creating a sample collection...")

		newCol, err := client.Collections().Create(ctx, &mockarty.Collection{
			Name:           "User API Tests",
			Description:    "Integration tests for the User API",
			Protocol:       "http",
			CollectionType: "test",
			IsShared:       true,
		})
		if err != nil {
			log.Fatalf("Failed to create collection: %v", err)
		}
		fmt.Printf("Created collection: %s (id=%s)\n", newCol.Name, newCol.ID)

		defer func() {
			_ = client.Collections().Delete(ctx, newCol.ID)
			fmt.Println("\nSample collection cleaned up.")
		}()

		collections = append(collections, *newCol)
	}

	// -----------------------------------------------------------------------
	// 2. Set up environments for test execution
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Set Up Environments ---")

	// Create a staging environment for tests
	stagingEnv, err := client.Environments().Create(ctx, &mockarty.Environment{
		Name: "Collection Test - Staging",
		Variables: map[string]string{
			"BASE_URL": "http://localhost:5770",
			"API_KEY":  "test-key-staging",
			"TIMEOUT":  "10000",
		},
	})
	if err != nil {
		fmt.Printf("Create environment returned: %v\n", err)
	} else {
		fmt.Printf("Created staging environment: id=%s\n", stagingEnv.ID)

		defer func() {
			_ = client.Environments().Delete(ctx, stagingEnv.ID)
		}()

		// Activate the environment before running collections.
		// All {{variable}} references in collection requests will resolve
		// from the active environment.
		err = client.Environments().Activate(ctx, stagingEnv.ID)
		if err != nil {
			fmt.Printf("Activate environment returned: %v\n", err)
		} else {
			fmt.Printf("Activated staging environment for test execution\n")
		}
	}

	// -----------------------------------------------------------------------
	// 3. Execute a single collection
	// -----------------------------------------------------------------------
	if len(collections) > 0 {
		fmt.Println("\n--- Execute Collection ---")

		colID := collections[0].ID
		fmt.Printf("Executing collection: %s (id=%s)\n", collections[0].Name, colID)

		result, err := client.Collections().Execute(ctx, colID)
		if err != nil {
			fmt.Printf("Execution returned error (expected for empty collections): %v\n", err)
		} else {
			fmt.Printf("Execution result:\n")
			fmt.Printf("  Status: %s\n", result.Status)
			fmt.Printf("  Total:  %d, Passed: %d, Failed: %d, Skipped: %d\n",
				result.TotalTests, result.PassedTests, result.FailedTests, result.SkippedTests)
			fmt.Printf("  Duration: %d ms\n", result.Duration)

			for _, tr := range result.Results {
				icon := "PASS"
				if tr.Status != "passed" {
					icon = "FAIL"
				}
				fmt.Printf("  [%s] %s (%d ms)\n", icon, tr.RequestName, tr.Duration)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 4. Execute multiple collections
	// -----------------------------------------------------------------------
	if len(collections) >= 2 {
		fmt.Println("\n--- Execute Multiple Collections ---")

		ids := []string{collections[0].ID, collections[1].ID}
		fmt.Printf("Executing %d collections simultaneously...\n", len(ids))

		result, err := client.Collections().ExecuteMultiple(ctx, ids)
		if err != nil {
			fmt.Printf("Multi-execution returned error: %v\n", err)
		} else {
			fmt.Printf("Combined result: %d total, %d passed, %d failed\n",
				result.TotalTests, result.PassedTests, result.FailedTests)
		}
	} else {
		fmt.Println("\n--- Execute Multiple Collections ---")
		fmt.Println("Skipped: need at least 2 collections")
	}

	// -----------------------------------------------------------------------
	// 5. Duplicate a collection
	// -----------------------------------------------------------------------
	if len(collections) > 0 {
		fmt.Println("\n--- Duplicate Collection ---")

		// Duplicate a collection to create a variant for a different environment
		// or to use as a template.
		dup, err := client.Collections().Duplicate(ctx, collections[0].ID)
		if err != nil {
			fmt.Printf("Duplicate returned error: %v\n", err)
		} else {
			fmt.Printf("Duplicated collection: %s (id=%s)\n", dup.Name, dup.ID)

			// Update the duplicate with a different name
			updated, err := client.Collections().Update(ctx, dup.ID, &mockarty.Collection{
				Name:        "User API Tests (Production)",
				Description: "Production variant of User API tests",
			})
			if err != nil {
				fmt.Printf("Update returned error: %v\n", err)
			} else {
				fmt.Printf("Renamed duplicate to: %s\n", updated.Name)
			}

			defer func() {
				_ = client.Collections().Delete(ctx, dup.ID)
			}()
		}
	}

	// -----------------------------------------------------------------------
	// 6. Run performance test from a collection
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Performance Test from Collection ---")

	if len(collections) > 0 {
		// Run the collection as a performance test with multiple virtual users
		perfTask, err := client.Perf().RunCollection(ctx, map[string]any{
			"collectionId": collections[0].ID,
			"vus":          5,
			"duration":     "10s",
		})
		if err != nil {
			fmt.Printf("Perf run from collection returned: %v\n", err)
			fmt.Println("(Performance testing may require specific license tier)")
		} else {
			fmt.Printf("Performance test started: id=%s, status=%s\n",
				perfTask.ID, perfTask.Status)
		}
	}

	// -----------------------------------------------------------------------
	// 7. View test run history
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Test Run History ---")

	runs, err := client.TestRuns().List(ctx)
	if err != nil {
		fmt.Printf("List test runs returned: %v\n", err)
	} else {
		fmt.Printf("Found %d test runs:\n", len(runs))
		for _, r := range runs {
			env := r.Environment
			if env == "" {
				env = "(none)"
			}
			fmt.Printf("  - id=%s, collection=%s, status=%s, passed=%d/%d, env=%s\n",
				r.ID, r.CollectionID, r.Status, r.PassedTests, r.TotalTests, env)
		}
	}

	// -----------------------------------------------------------------------
	// 8. Export a collection
	// -----------------------------------------------------------------------
	if len(collections) > 0 {
		fmt.Println("\n--- Export Collection ---")

		colID := collections[0].ID
		data, err := client.Collections().Export(ctx, colID)
		if err != nil {
			fmt.Printf("Export returned error: %v\n", err)
		} else {
			fmt.Printf("Exported collection %s: %d bytes (Postman-compatible format)\n",
				colID, len(data))

			preview := string(data)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("Preview: %s\n", preview)
		}
	}

	fmt.Println("\nCollection examples completed!")
}
