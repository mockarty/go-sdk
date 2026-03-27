// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: environments — API Tester Environment management.
//
// This example demonstrates:
//   - Create environments for different stages (dev, staging, production)
//   - List all environments
//   - Set variables and activate an environment
//   - Update environment variables
//   - Use environments to switch between API targets
//   - Delete environments
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. Create environments for different stages
	// -----------------------------------------------------------------------
	fmt.Println("--- Create Environments ---")

	// Development environment
	devEnv, err := client.Environments().Create(ctx, &mockarty.Environment{
		Name: "Development",
		Variables: map[string]string{
			"BASE_URL":   "http://localhost:5770",
			"API_KEY":    "dev-key-123",
			"DB_HOST":    "localhost:5432",
			"LOG_LEVEL":  "debug",
			"TIMEOUT_MS": "5000",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create dev environment: %v", err)
	}
	fmt.Printf("Created environment: id=%s, name=%s, vars=%d\n",
		devEnv.ID, devEnv.Name, len(devEnv.Variables))

	// Staging environment
	stagingEnv, err := client.Environments().Create(ctx, &mockarty.Environment{
		Name: "Staging",
		Variables: map[string]string{
			"BASE_URL":   "https://staging.api.example.com",
			"API_KEY":    "staging-key-456",
			"DB_HOST":    "staging-db.internal:5432",
			"LOG_LEVEL":  "info",
			"TIMEOUT_MS": "10000",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create staging environment: %v", err)
	}
	fmt.Printf("Created environment: id=%s, name=%s, vars=%d\n",
		stagingEnv.ID, stagingEnv.Name, len(stagingEnv.Variables))

	// Production environment
	prodEnv, err := client.Environments().Create(ctx, &mockarty.Environment{
		Name: "Production",
		Variables: map[string]string{
			"BASE_URL":   "https://api.example.com",
			"API_KEY":    "prod-key-789",
			"DB_HOST":    "prod-db.internal:5432",
			"LOG_LEVEL":  "warn",
			"TIMEOUT_MS": "30000",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create production environment: %v", err)
	}
	fmt.Printf("Created environment: id=%s, name=%s, vars=%d\n",
		prodEnv.ID, prodEnv.Name, len(prodEnv.Variables))

	// Clean up all environments at the end
	defer func() {
		_ = client.Environments().Delete(ctx, devEnv.ID)
		_ = client.Environments().Delete(ctx, stagingEnv.ID)
		_ = client.Environments().Delete(ctx, prodEnv.ID)
		fmt.Println("\nAll environments cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 2. List all environments
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Environments ---")

	envs, err := client.Environments().List(ctx)
	if err != nil {
		log.Fatalf("Failed to list environments: %v", err)
	}

	fmt.Printf("Found %d environments:\n", len(envs))
	for _, env := range envs {
		activeLabel := ""
		if env.IsActive {
			activeLabel = " [ACTIVE]"
		}
		fmt.Printf("  - %s (id=%s, vars=%d)%s\n",
			env.Name, env.ID, len(env.Variables), activeLabel)
	}

	// -----------------------------------------------------------------------
	// 3. Activate an environment
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Activate Environment ---")

	// Activating an environment makes its variables available in all
	// API Tester requests. Variables like {{BASE_URL}} get resolved
	// from the active environment.
	err = client.Environments().Activate(ctx, stagingEnv.ID)
	if err != nil {
		fmt.Printf("Activate returned: %v\n", err)
	} else {
		fmt.Printf("Activated environment: %s\n", stagingEnv.Name)
	}

	// -----------------------------------------------------------------------
	// 4. Get the currently active environment
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Get Active Environment ---")

	active, err := client.Environments().GetActive(ctx)
	if err != nil {
		fmt.Printf("Get active returned: %v\n", err)
	} else {
		fmt.Printf("Active environment: %s (id=%s)\n", active.Name, active.ID)
		fmt.Println("Variables:")
		for key, val := range active.Variables {
			// Mask sensitive values in output
			display := val
			if key == "API_KEY" && len(val) > 4 {
				display = val[:4] + "****"
			}
			fmt.Printf("  %s = %s\n", key, display)
		}
	}

	// -----------------------------------------------------------------------
	// 5. Update environment variables
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Update Environment ---")

	// Add a new variable and update an existing one
	updatedEnv, err := client.Environments().Update(ctx, stagingEnv.ID, &mockarty.Environment{
		Name: "Staging",
		Variables: map[string]string{
			"BASE_URL":        "https://staging-v2.api.example.com",
			"API_KEY":         "staging-key-456-v2",
			"DB_HOST":         "staging-db.internal:5432",
			"LOG_LEVEL":       "debug", // changed for troubleshooting
			"TIMEOUT_MS":      "15000",
			"FEATURE_FLAG_V2": "true", // new variable
		},
	})
	if err != nil {
		fmt.Printf("Update returned: %v\n", err)
	} else {
		fmt.Printf("Updated environment: %s, now has %d variables\n",
			updatedEnv.Name, len(updatedEnv.Variables))
	}

	// -----------------------------------------------------------------------
	// 6. Get a specific environment by ID
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Get Environment ---")

	env, err := client.Environments().Get(ctx, prodEnv.ID)
	if err != nil {
		fmt.Printf("Get returned: %v\n", err)
	} else {
		fmt.Printf("Production environment: %s\n", env.Name)
		fmt.Printf("  BASE_URL: %s\n", env.Variables["BASE_URL"])
		fmt.Printf("  TIMEOUT_MS: %s\n", env.Variables["TIMEOUT_MS"])
	}

	// -----------------------------------------------------------------------
	// 7. Switch environments for test execution
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Switch Environments ---")

	// In CI/CD, you typically activate different environments before
	// running test collections. This ensures tests hit the right target.
	environments := []struct {
		id   string
		name string
	}{
		{devEnv.ID, "Development"},
		{stagingEnv.ID, "Staging"},
		{prodEnv.ID, "Production"},
	}

	for _, env := range environments {
		err = client.Environments().Activate(ctx, env.id)
		if err != nil {
			fmt.Printf("  Failed to activate %s: %v\n", env.name, err)
			continue
		}

		active, err := client.Environments().GetActive(ctx)
		if err != nil {
			fmt.Printf("  Failed to get active: %v\n", err)
			continue
		}

		fmt.Printf("  Switched to %s: BASE_URL=%s\n",
			active.Name, active.Variables["BASE_URL"])
	}

	// Set back to dev for safety
	_ = client.Environments().Activate(ctx, devEnv.ID)
	fmt.Println("  Reset to Development environment")

	fmt.Println("\nEnvironment examples completed!")
}
