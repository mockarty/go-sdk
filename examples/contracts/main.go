// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: contracts — Contract testing and Pact API usage.
//
// This example demonstrates:
//   - Validate mocks against an OpenAPI spec
//   - Save and list contract testing configs
//   - Publish and verify consumer-driven contracts (Pacts)
//   - Can-I-Deploy check before releasing a service
//   - Generate mocks from a Pact definition
//   - Detect contract drift
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
	// 1. Validate mocks against an OpenAPI spec
	// -----------------------------------------------------------------------
	fmt.Println("--- Validate Mocks Against Spec ---")

	validationResult, err := client.Contracts().ValidateMocks(ctx, &mockarty.ContractValidationRequest{
		SpecURL: "http://localhost:5770/swagger/doc.json",
	})
	if err != nil {
		fmt.Printf("Validation returned: %v\n", err)
	} else {
		fmt.Printf("Validation result:\n")
		fmt.Printf("  ID: %s\n", validationResult.ID)
		fmt.Printf("  Status: %s\n", validationResult.Status)
		fmt.Printf("  Violations: %d\n", validationResult.Violations)

		if len(validationResult.Details) > 0 {
			fmt.Println("  Violation details:")
			for i, v := range validationResult.Details {
				if i >= 5 {
					fmt.Printf("  ... and %d more\n", len(validationResult.Details)-5)
					break
				}
				fmt.Printf("    [%s] %s: %s\n", v.Severity, v.Path, v.Message)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 2. Save a contract testing configuration
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Save Contract Config ---")

	savedConfig, err := client.Contracts().SaveConfig(ctx, &mockarty.ContractConfig{
		Name:      "User API Contract",
		SpecURL:   "https://petstore.swagger.io/v2/swagger.json",
		TargetURL: "http://localhost:5770",
		Schedule:  "0 */6 * * *", // every 6 hours
	})
	if err != nil {
		fmt.Printf("Save config returned: %v\n", err)
	} else {
		fmt.Printf("Saved contract config: id=%s, name=%s\n", savedConfig.ID, savedConfig.Name)

		defer func() {
			_ = client.Contracts().DeleteConfig(ctx, savedConfig.ID)
			fmt.Println("\nContract config cleaned up.")
		}()
	}

	// -----------------------------------------------------------------------
	// 3. List contract testing configurations
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Contract Configs ---")

	configs, err := client.Contracts().ListConfigs(ctx)
	if err != nil {
		log.Fatalf("Failed to list contract configs: %v", err)
	}

	fmt.Printf("Found %d contract configs:\n", len(configs))
	for _, c := range configs {
		fmt.Printf("  - %s (id=%s, schedule=%s)\n", c.Name, c.ID, c.Schedule)
	}

	// -----------------------------------------------------------------------
	// 4. Publish a consumer-driven contract (Pact)
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Publish Pact ---")

	// A Pact describes the expectations of a consumer (e.g. frontend)
	// about a provider (e.g. user-service API).
	pact, err := client.Contracts().PublishPact(ctx, &mockarty.Pact{
		Consumer: "web-frontend",
		Provider: "user-service",
		Version:  "1.2.0",
		Spec: `{
			"consumer": {"name": "web-frontend"},
			"provider": {"name": "user-service"},
			"interactions": [
				{
					"description": "get user by id",
					"request": {"method": "GET", "path": "/api/users/42"},
					"response": {
						"status": 200,
						"body": {"id": 42, "name": "John Doe", "email": "john@example.com"}
					}
				},
				{
					"description": "create user",
					"request": {
						"method": "POST",
						"path": "/api/users",
						"body": {"name": "Jane Doe", "email": "jane@example.com"}
					},
					"response": {
						"status": 201,
						"body": {"id": 43, "name": "Jane Doe"}
					}
				}
			]
		}`,
	})
	if err != nil {
		fmt.Printf("Publish pact returned: %v\n", err)
	} else {
		fmt.Printf("Published pact: id=%s, consumer=%s, provider=%s, version=%s\n",
			pact.ID, pact.Consumer, pact.Provider, pact.Version)

		defer func() {
			_ = client.Contracts().DeletePact(ctx, pact.ID)
			fmt.Println("Pact cleaned up.")
		}()
	}

	// -----------------------------------------------------------------------
	// 5. List all pacts
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Pacts ---")

	pacts, err := client.Contracts().ListPacts(ctx)
	if err != nil {
		fmt.Printf("List pacts returned: %v\n", err)
	} else {
		fmt.Printf("Found %d pacts:\n", len(pacts))
		for _, p := range pacts {
			fmt.Printf("  - %s -> %s (version=%s, id=%s)\n",
				p.Consumer, p.Provider, p.Version, p.ID)
		}
	}

	// -----------------------------------------------------------------------
	// 6. Verify a pact against the provider
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Verify Pact ---")

	// Pact verification checks that the provider actually satisfies
	// the consumer's expectations defined in the pact.
	verifyResult, err := client.Contracts().VerifyPact(ctx, map[string]any{
		"consumer":  "web-frontend",
		"provider":  "user-service",
		"targetUrl": "http://localhost:5770",
	})
	if err != nil {
		fmt.Printf("Verify pact returned: %v\n", err)
	} else {
		fmt.Printf("Verification result: status=%s, pactId=%s\n",
			verifyResult.Status, verifyResult.PactID)
		if len(verifyResult.Violations) > 0 {
			fmt.Printf("  Violations found: %d\n", len(verifyResult.Violations))
			for _, v := range verifyResult.Violations {
				fmt.Printf("    [%s] %s: %s\n", v.Severity, v.Path, v.Message)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 7. Can-I-Deploy check
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Can-I-Deploy ---")

	// Before deploying a service, check that all its consumer contracts
	// are satisfied. This prevents breaking changes from reaching production.
	deployResult, err := client.Contracts().CanIDeploy(ctx, map[string]any{
		"service": "user-service",
		"version": "2.0.0",
	})
	if err != nil {
		fmt.Printf("Can-I-Deploy returned: %v\n", err)
	} else {
		if deployResult.OK {
			fmt.Println("  Safe to deploy: all consumer contracts are satisfied")
		} else {
			fmt.Printf("  NOT safe to deploy: %s\n", deployResult.Reason)
		}
	}

	// -----------------------------------------------------------------------
	// 8. Generate mocks from a pact
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Generate Mocks from Pact ---")

	if pact != nil {
		mocks, err := client.Contracts().GenerateMocksFromPact(ctx, pact.ID)
		if err != nil {
			fmt.Printf("Generate mocks from pact returned: %v\n", err)
		} else {
			fmt.Printf("Generated %d mocks from pact:\n", len(mocks))
			for _, m := range mocks {
				route := ""
				if m.HTTP != nil {
					route = m.HTTP.HttpMethod + " " + m.HTTP.Route
				}
				fmt.Printf("  - %s (%s)\n", m.ID, route)
			}

			// Clean up generated mocks
			for _, m := range mocks {
				_ = client.Mocks().Delete(ctx, m.ID)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 9. List verification results
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Verification Results ---")

	verifications, err := client.Contracts().ListVerifications(ctx)
	if err != nil {
		fmt.Printf("List verifications returned: %v\n", err)
	} else {
		fmt.Printf("Found %d verification results:\n", len(verifications))
		for _, v := range verifications {
			fmt.Printf("  - pactId=%s, status=%s, violations=%d\n",
				v.PactID, v.Status, len(v.Violations))
		}
	}

	// -----------------------------------------------------------------------
	// 10. Detect contract drift
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Detect Contract Drift ---")

	// Drift detection compares the current live behavior of a service
	// against its contract to find silent breaking changes.
	driftResult, err := client.Contracts().DetectDrift(ctx, map[string]any{
		"specUrl":   "https://petstore.swagger.io/v2/swagger.json",
		"targetUrl": "http://localhost:5770",
	})
	if err != nil {
		fmt.Printf("Detect drift returned: %v\n", err)
	} else {
		fmt.Printf("Drift detection result: %v\n", driftResult)
	}

	// -----------------------------------------------------------------------
	// 11. Verify provider against contract spec
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Verify Provider ---")

	providerResult, err := client.Contracts().VerifyProvider(ctx, &mockarty.ContractValidationRequest{
		SpecURL:   "https://petstore.swagger.io/v2/swagger.json",
		TargetURL: "http://localhost:5770",
	})
	if err != nil {
		fmt.Printf("Verify provider returned: %v\n", err)
	} else {
		fmt.Printf("Provider verification: status=%s, violations=%d\n",
			providerResult.Status, providerResult.Violations)
	}

	// -----------------------------------------------------------------------
	// 12. List all contract validation results
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Contract Results ---")

	results, err := client.Contracts().ListResults(ctx)
	if err != nil {
		log.Fatalf("Failed to list contract results: %v", err)
	}

	fmt.Printf("Found %d validation results:\n", len(results))
	for _, r := range results {
		validatedAt := "unknown"
		if r.ValidatedAt > 0 {
			validatedAt = time.Unix(r.ValidatedAt, 0).Format(time.RFC3339)
		}
		fmt.Printf("  - id=%s, config=%s, status=%s, violations=%d, at=%s\n",
			r.ID, r.ConfigID, r.Status, r.Violations, validatedAt)
	}

	fmt.Println("\nContract testing examples completed!")
}
