// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: stores — Mockarty store operations (Global, Chain, Mock).
//
// This example demonstrates:
//   - Global store: set, get, delete
//   - Chain store: set, get, delete
//   - Mock store: using extract config
//   - Using stores in response templates ($.gS.key, $.cS.key, $.mS.key)
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

	mockIDs := []string{
		"store-demo-global",
		"store-demo-chain",
		"store-demo-extract",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		// Clean up stores
		_ = client.Stores().GlobalDelete(ctx, "environment")
		_ = client.Stores().GlobalDelete(ctx, "featureFlags")
		_ = client.Stores().ChainDelete(ctx, "order-flow", "currentStep")
		_ = client.Stores().ChainDelete(ctx, "order-flow", "orderId")
		fmt.Println("\nAll store examples cleaned up.")
	}()

	// =======================================================================
	// Global Store (gS) — namespace-scoped persistent key-value store
	// =======================================================================
	fmt.Println("--- Global Store ---")

	// Set values in the global store
	if err := client.Stores().GlobalSet(ctx, "environment", "staging"); err != nil {
		log.Fatalf("Failed to set global store key: %v", err)
	}
	fmt.Println("[1] Set global store: environment = staging")

	if err := client.Stores().GlobalSet(ctx, "featureFlags", map[string]any{
		"darkMode":     true,
		"betaFeatures": false,
		"maxRetries":   3,
	}); err != nil {
		log.Fatalf("Failed to set global store key: %v", err)
	}
	fmt.Println("[2] Set global store: featureFlags = {darkMode: true, ...}")

	// Get the entire global store
	globalStore, err := client.Stores().GlobalGet(ctx)
	if err != nil {
		log.Fatalf("Failed to get global store: %v", err)
	}
	fmt.Printf("[3] Global store contents: %v\n", globalStore)

	// Create a mock that reads from the global store
	globalMock := mockarty.NewMockBuilder().
		ID("store-demo-global").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/config").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			// $.gS.key references global store values in the response template
			r.Status(200).JSONBody(map[string]any{
				"environment":  "$.gS.environment",
				"featureFlags": "$.gS.featureFlags",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, globalMock); err != nil {
		log.Fatalf("Failed to create global store mock: %v", err)
	}
	fmt.Println("[4] Created mock that reads from global store ($.gS.*)")

	// Delete a global store key
	if err := client.Stores().GlobalDelete(ctx, "environment"); err != nil {
		log.Fatalf("Failed to delete global store key: %v", err)
	}
	fmt.Println("[5] Deleted global store key: environment")

	// =======================================================================
	// Chain Store (cS) — state shared across mocks with the same chainId
	// =======================================================================
	fmt.Println("\n--- Chain Store ---")

	chainID := "order-flow"

	// Set values in the chain store
	if err := client.Stores().ChainSet(ctx, chainID, "currentStep", "checkout"); err != nil {
		log.Fatalf("Failed to set chain store key: %v", err)
	}
	fmt.Println("[6] Set chain store (order-flow): currentStep = checkout")

	if err := client.Stores().ChainSet(ctx, chainID, "orderId", "ORD-12345"); err != nil {
		log.Fatalf("Failed to set chain store key: %v", err)
	}
	fmt.Println("[7] Set chain store (order-flow): orderId = ORD-12345")

	// Get the chain store
	chainStore, err := client.Stores().ChainGet(ctx, chainID)
	if err != nil {
		log.Fatalf("Failed to get chain store: %v", err)
	}
	fmt.Printf("[8] Chain store (order-flow) contents: %v\n", chainStore)

	// Create a mock that reads from the chain store
	chainMock := mockarty.NewMockBuilder().
		ID("store-demo-chain").
		ChainID(chainID).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/order/status").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			// $.cS.key references chain store values
			r.Status(200).JSONBody(map[string]any{
				"orderId":     "$.cS.orderId",
				"currentStep": "$.cS.currentStep",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, chainMock); err != nil {
		log.Fatalf("Failed to create chain store mock: %v", err)
	}
	fmt.Println("[9] Created mock that reads from chain store ($.cS.*)")

	// Delete chain store keys
	if err := client.Stores().ChainDeleteMany(ctx, chainID, "currentStep", "orderId"); err != nil {
		log.Fatalf("Failed to delete chain store keys: %v", err)
	}
	fmt.Println("[10] Deleted chain store keys: currentStep, orderId")

	// =======================================================================
	// Mock Store (mS) — per-mock-call ephemeral state using Extract
	// =======================================================================
	fmt.Println("\n--- Mock Store (Extract) ---")

	// The Extract config tells Mockarty to pull values from the incoming
	// request and place them into stores. The mock store (mStore) is
	// ephemeral — it only lives for the duration of that single mock call.
	extractMock := mockarty.NewMockBuilder().
		ID("store-demo-extract").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/echo").Method("POST")
		}).
		// Pre-populate mock store with a default value
		MockStore(map[string]any{
			"requestCount": 0,
		}).
		// Extract values from the request into stores
		ExtractConfig(&mockarty.Extract{
			// Extract into mock store (ephemeral, only for this call)
			MStore: map[string]any{
				"userName": "$.body.name",
				"userAge":  "$.body.age",
			},
			// Extract into chain store (persists across calls with same chainId)
			CStore: map[string]any{
				"lastCaller": "$.body.name",
			},
			// Extract into global store (persists globally)
			GStore: map[string]any{
				"lastRequestTime": "$.fake.DateISO",
			},
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"echo": map[string]any{
					// $.mS.key references mock store values (extracted from request)
					"name": "$.mS.userName",
					"age":  "$.mS.userAge",
				},
				"meta": map[string]any{
					"processedAt": "$.fake.DateISO",
				},
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, extractMock); err != nil {
		log.Fatalf("Failed to create extract mock: %v", err)
	}
	fmt.Println("[11] Created mock with Extract config (mStore, cStore, gStore)")

	fmt.Println("\nAll store examples created successfully!")
}
