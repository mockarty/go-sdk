// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: stats — Platform statistics and system status.
//
// This example demonstrates:
//   - Get general platform statistics
//   - Get resource counts (mocks, namespaces, etc.)
//   - Get system status and health information
//   - Get available feature flags and capabilities
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. General platform statistics
	// -----------------------------------------------------------------------
	fmt.Println("--- Platform Statistics ---")

	stats, err := client.Stats().GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Println("General statistics:")
	for key, val := range stats {
		fmt.Printf("  %s: %v\n", key, val)
	}

	// -----------------------------------------------------------------------
	// 2. Resource counts
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Resource Counts ---")

	counts, err := client.Stats().GetCounts(ctx)
	if err != nil {
		fmt.Printf("Get counts returned: %v\n", err)
	} else {
		fmt.Println("Resource counts:")
		for key, val := range counts {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 3. System status
	// -----------------------------------------------------------------------
	fmt.Println("\n--- System Status ---")

	status, err := client.Stats().GetStatus(ctx)
	if err != nil {
		fmt.Printf("Get status returned: %v\n", err)
	} else {
		fmt.Println("System status:")
		for key, val := range status {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 4. Feature flags and capabilities
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Features & Capabilities ---")

	features, err := client.Stats().GetFeatures(ctx)
	if err != nil {
		fmt.Printf("Get features returned: %v\n", err)
	} else {
		fmt.Println("Available features:")
		for key, val := range features {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 5. Health check (from HealthAPI)
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Health Check ---")

	health, err := client.Health().Check(ctx)
	if err != nil {
		fmt.Printf("Health check returned: %v\n", err)
	} else {
		fmt.Printf("Health status: %s\n", health.Status)
		fmt.Printf("  Release: %s\n", health.ReleaseID)
		if len(health.Checks) > 0 {
			fmt.Println("  Checks:")
			for name, details := range health.Checks {
				for _, d := range details {
					fmt.Printf("    %s: %s (%s)\n", name, d.Status, d.Output)
				}
			}
		}
	}

	// -----------------------------------------------------------------------
	// 6. Practical usage: CI/CD gate check
	// -----------------------------------------------------------------------
	fmt.Println("\n--- CI/CD Gate Check ---")

	// Use stats to verify the system is ready before running tests.
	// This is useful in CI/CD pipelines as a pre-flight check.
	systemStatus, err := client.Stats().GetStatus(ctx)
	if err != nil {
		fmt.Printf("Gate check failed: %v\n", err)
		fmt.Println("System might not be ready for testing")
	} else {
		fmt.Println("Pre-flight check passed:")
		fmt.Printf("  Status: %v\n", systemStatus)

		// Check resource counts to ensure test data is loaded
		resourceCounts, _ := client.Stats().GetCounts(ctx)
		if resourceCounts != nil {
			fmt.Printf("  Resources available: %v\n", resourceCounts)
		}
	}

	fmt.Println("\nStats examples completed!")
}
