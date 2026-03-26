// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: advanced — Advanced mock patterns with versioning, proxy, and batch tags.
//
// This example demonstrates:
//   - Chain mocks (request workflows with chainId + chain store)
//   - Condition combinations (body + header + query)
//   - Extract from request to stores
//   - Mock with path prefix and server name
//   - Batch operations
//   - Mock versions (history, restore)
//   - Proxy usage with mocks
//   - Batch tag updates
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
	// 1. Chain mocks — E-commerce order workflow
	// -----------------------------------------------------------------------
	fmt.Println("--- Chain Mocks (Order Workflow) ---")

	chainID := "order-workflow"

	// Step 1: Create Order
	createOrder := mockarty.NewMockBuilder().
		ID("chain-create-order").
		ChainID(chainID).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/orders").Method("POST")
		}).
		ExtractConfig(&mockarty.Extract{
			CStore: map[string]any{
				"orderId":     "$.fake.UUID",
				"customerRef": "$.body.customerRef",
				"totalAmount": "$.body.totalAmount",
				"status":      "CREATED",
			},
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(201).JSONBody(map[string]any{
				"orderId":     "$.cS.orderId",
				"status":      "CREATED",
				"customerRef": "$.cS.customerRef",
				"totalAmount": "$.cS.totalAmount",
				"createdAt":   "$.fake.DateISO",
			})
		}).
		Build()

	// Step 2: Pay for Order
	payOrder := mockarty.NewMockBuilder().
		ID("chain-pay-order").
		ChainID(chainID).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/orders/:id/pay").Method("POST")
		}).
		ExtractConfig(&mockarty.Extract{
			CStore: map[string]any{
				"status":    "PAID",
				"paymentId": "$.fake.UUID",
				"paidAt":    "$.fake.DateISO",
			},
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"orderId":   "$.cS.orderId",
				"status":    "PAID",
				"paymentId": "$.cS.paymentId",
				"paidAt":    "$.cS.paidAt",
			})
		}).
		Build()

	// Step 3: Ship Order
	shipOrder := mockarty.NewMockBuilder().
		ID("chain-ship-order").
		ChainID(chainID).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/orders/:id/ship").Method("POST")
		}).
		ExtractConfig(&mockarty.Extract{
			CStore: map[string]any{
				"status":     "SHIPPED",
				"trackingId": "$.fake.UUID",
				"carrier":    "$.body.carrier",
			},
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"orderId":    "$.cS.orderId",
				"status":     "SHIPPED",
				"trackingId": "$.cS.trackingId",
				"carrier":    "$.cS.carrier",
				"shippedAt":  "$.fake.DateISO",
			})
		}).
		Build()

	// Step 4: Get Order Status
	getOrderStatus := mockarty.NewMockBuilder().
		ID("chain-get-order-status").
		ChainID(chainID).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/orders/:id/status").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"orderId":     "$.cS.orderId",
				"status":      "$.cS.status",
				"customerRef": "$.cS.customerRef",
				"totalAmount": "$.cS.totalAmount",
				"paymentId":   "$.cS.paymentId",
				"trackingId":  "$.cS.trackingId",
				"carrier":     "$.cS.carrier",
			})
		}).
		Build()

	chainMocks := []*mockarty.Mock{createOrder, payOrder, shipOrder, getOrderStatus}
	for _, m := range chainMocks {
		if _, err := client.Mocks().Create(ctx, m); err != nil {
			log.Fatalf("Failed to create chain mock %s: %v", m.ID, err)
		}
	}
	fmt.Println("[1] Created 4 chain mocks for order workflow")
	fmt.Println("    POST /api/orders -> POST /:id/pay -> POST /:id/ship -> GET /:id/status")

	defer func() {
		for _, m := range chainMocks {
			_ = client.Mocks().Delete(ctx, m.ID)
		}
	}()

	// -----------------------------------------------------------------------
	// 2. Condition combinations (body + header + query)
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Condition Combinations ---")

	comboMock := mockarty.NewMockBuilder().
		ID("combo-conditions").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/accounts").
				Method("POST").
				HeaderCondition("Content-Type", mockarty.AssertContains, "application/json").
				HeaderCondition("Authorization", mockarty.AssertContains, "Bearer").
				BodyCondition("$.body.type", mockarty.AssertEquals, "premium").
				QueryCondition("format", mockarty.AssertEquals, "full")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"accountId":   "$.fake.UUID",
				"type":        "premium",
				"features":    []string{"analytics", "priority_support", "custom_domain"},
				"format":      "full",
				"activatedAt": "$.fake.DateISO",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, comboMock); err != nil {
		log.Fatalf("Failed to create combo mock: %v", err)
	}
	fmt.Println("[2] Created mock with combined conditions (header + body + query)")

	defer func() {
		_ = client.Mocks().Delete(ctx, "combo-conditions")
	}()

	// -----------------------------------------------------------------------
	// 3. Extract from request to stores
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Extract Request Data to Stores ---")

	extractMock := mockarty.NewMockBuilder().
		ID("extract-to-stores").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/register").Method("POST")
		}).
		ExtractConfig(&mockarty.Extract{
			GStore: map[string]any{
				"lastRegisteredUser": "$.body.email",
				"registrationCount":  "$.increment($.gS.registrationCount)",
			},
			CStore: map[string]any{
				"userId":    "$.fake.UUID",
				"userEmail": "$.body.email",
			},
			MStore: map[string]any{
				"welcomeToken": "$.fake.UUID",
			},
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(201).JSONBody(map[string]any{
				"userId":       "$.cS.userId",
				"email":        "$.cS.userEmail",
				"welcomeToken": "$.mS.welcomeToken",
				"userNumber":   "$.gS.registrationCount",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, extractMock); err != nil {
		log.Fatalf("Failed to create extract mock: %v", err)
	}
	fmt.Println("[3] Created mock with extract config (gStore, cStore, mStore)")

	defer func() {
		_ = client.Mocks().Delete(ctx, "extract-to-stores")
	}()

	// -----------------------------------------------------------------------
	// 4. Mock with path prefix and server name
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Path Prefix and Server Name ---")

	prefixedMock := mockarty.NewMockBuilder().
		ID("prefixed-api").
		PathPrefix("/v2").
		ServerName("payment-service").
		Tags("payment", "v2").
		Priority(5).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/payments/:id").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"paymentId": "$.pathParam.id",
				"amount":    "$.fake.Float64",
				"currency":  "USD",
				"status":    "completed",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, prefixedMock); err != nil {
		log.Fatalf("Failed to create prefixed mock: %v", err)
	}
	fmt.Println("[4] Created mock with pathPrefix=/v2, serverName=payment-service")

	defer func() {
		_ = client.Mocks().Delete(ctx, "prefixed-api")
	}()

	// -----------------------------------------------------------------------
	// 5. Batch operations
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Batch Operations ---")

	batchMocks := []*mockarty.Mock{
		mockarty.NewMockBuilder().
			ID("batch-1").
			HTTP(func(h *mockarty.HTTPBuilder) {
				h.Route("/api/batch/resource-a").Method("GET")
			}).
			Response(func(r *mockarty.ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"resource": "A"})
			}).
			Build(),

		mockarty.NewMockBuilder().
			ID("batch-2").
			HTTP(func(h *mockarty.HTTPBuilder) {
				h.Route("/api/batch/resource-b").Method("GET")
			}).
			Response(func(r *mockarty.ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"resource": "B"})
			}).
			Build(),

		mockarty.NewMockBuilder().
			ID("batch-3").
			HTTP(func(h *mockarty.HTTPBuilder) {
				h.Route("/api/batch/resource-c").Method("GET")
			}).
			Response(func(r *mockarty.ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"resource": "C"})
			}).
			Build(),
	}

	if err := client.Mocks().BatchCreate(ctx, batchMocks); err != nil {
		log.Fatalf("Failed to batch create mocks: %v", err)
	}
	fmt.Println("[5] Batch created 3 mocks in a single API call")

	defer func() {
		batchIDs := []string{"batch-1", "batch-2", "batch-3"}
		if err := client.Mocks().BatchDelete(ctx, batchIDs); err != nil {
			fmt.Printf("Batch delete returned: %v\n", err)
		} else {
			fmt.Println("[cleanup] Batch deleted 3 mocks")
		}
	}()

	// -----------------------------------------------------------------------
	// 6. Mock versions — track history and restore
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Mock Versions ---")

	// Update a mock several times to build a version history.
	versionedMock := mockarty.NewMockBuilder().
		ID("versioned-mock").
		Tags("v1").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/config").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"version": 1,
				"feature": "basic",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, versionedMock); err != nil {
		log.Fatalf("Failed to create versioned mock: %v", err)
	}
	fmt.Println("[6a] Created versioned mock v1")

	// Update to v2
	versionedMock.Tags = []string{"v2"}
	versionedMock.Response.Payload = map[string]any{
		"version": 2,
		"feature": "enhanced",
	}
	if _, err := client.Mocks().Create(ctx, versionedMock); err != nil {
		fmt.Printf("Update to v2 returned: %v\n", err)
	} else {
		fmt.Println("[6b] Updated to v2")
	}

	// Update to v3
	versionedMock.Tags = []string{"v3"}
	versionedMock.Response.Payload = map[string]any{
		"version": 3,
		"feature": "premium",
	}
	if _, err := client.Mocks().Create(ctx, versionedMock); err != nil {
		fmt.Printf("Update to v3 returned: %v\n", err)
	} else {
		fmt.Println("[6c] Updated to v3")
	}

	defer func() {
		_ = client.Mocks().Delete(ctx, "versioned-mock")
	}()

	// List version history
	versions, err := client.Mocks().ListVersions(ctx, "versioned-mock")
	if err != nil {
		fmt.Printf("List versions returned: %v\n", err)
	} else {
		fmt.Printf("Version history (%d versions):\n", len(versions))
		for i, v := range versions {
			fmt.Printf("  [%d] tags=%v, payload=%v\n", i+1, v.Tags, v.Response)
		}
	}

	// Restore a previous version
	if len(versions) >= 2 {
		// Get the version ID of the first (oldest) version
		oldVersion := versions[0]
		fmt.Printf("\n[6d] Restoring version with tags=%v\n", oldVersion.Tags)

		err = client.Mocks().RestoreVersion(ctx, "versioned-mock", "1")
		if err != nil {
			fmt.Printf("Restore version returned: %v\n", err)
		} else {
			fmt.Println("Version restored successfully")

			restored, err := client.Mocks().Get(ctx, "versioned-mock")
			if err == nil {
				fmt.Printf("Current mock state: tags=%v\n", restored.Tags)
			}
		}
	}

	// -----------------------------------------------------------------------
	// 7. Proxy usage with mock
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy with Mock ---")

	// Create a proxy mock that forwards to a real service and adds a delay.
	// This combination lets you test timeout handling in your application.
	proxyMock := mockarty.NewMockBuilder().
		ID("proxy-timeout-test").
		Tags("proxy", "timeout-test").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/slow-endpoint").Method("GET")
		}).
		ProxyTo("https://httpbin.org").
		Build()

	// Add a 5-second delay to simulate a slow upstream
	proxyMock.Response = &mockarty.ContentResponse{
		Delay: 5000,
		Headers: map[string][]string{
			"X-Proxy-Delay": {"5000ms"},
		},
	}

	if _, err := client.Mocks().Create(ctx, proxyMock); err != nil {
		fmt.Printf("Create proxy mock returned: %v\n", err)
	} else {
		fmt.Println("[7] Created proxy mock with 5s delay for timeout testing")
	}

	defer func() {
		_ = client.Mocks().Delete(ctx, "proxy-timeout-test")
	}()

	// -----------------------------------------------------------------------
	// 8. Batch tag updates
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Batch Tag Updates ---")

	// Apply tags to multiple mocks at once.
	// Useful for organizing mocks after import or generation.
	allBatchIDs := []string{"batch-1", "batch-2", "batch-3"}

	err = client.Mocks().BatchUpdateTags(ctx, allBatchIDs, []string{"batch-group", "automated", "v1"})
	if err != nil {
		fmt.Printf("Batch update tags returned: %v\n", err)
	} else {
		fmt.Printf("[8] Updated tags on %d mocks to [batch-group, automated, v1]\n", len(allBatchIDs))
	}

	// Verify tags were applied
	for _, id := range allBatchIDs {
		mock, err := client.Mocks().Get(ctx, id)
		if err != nil {
			continue
		}
		fmt.Printf("  %s: tags=%v\n", id, mock.Tags)
	}

	// -----------------------------------------------------------------------
	// 9. List mocks with filtering
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Mocks with Filtering ---")

	list, err := client.Mocks().List(ctx, &mockarty.ListMocksOptions{
		Namespace: "sandbox",
		Tags:      []string{"payment"},
		Limit:     10,
	})
	if err != nil {
		fmt.Printf("List returned: %v\n", err)
	} else {
		fmt.Printf("Mocks with tag 'payment': %d (total=%d)\n", len(list.Items), list.Total)
		for _, m := range list.Items {
			fmt.Printf("  - %s (tags=%v)\n", m.ID, m.Tags)
		}
	}

	// -----------------------------------------------------------------------
	// 10. Mock request logs
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Mock Request Logs ---")

	logs, err := client.Mocks().Logs(ctx, "prefixed-api", &mockarty.LogsOptions{
		Limit: 5,
	})
	if err != nil {
		fmt.Printf("Logs returned: %v\n", err)
	} else {
		fmt.Printf("Logs for 'prefixed-api': %d entries\n", len(logs.Requests))
		for _, entry := range logs.Requests {
			fmt.Printf("  - [%s] req=%v\n", entry.CalledAt, entry.Req)
		}
	}

	// -----------------------------------------------------------------------
	// 11. Chain versions
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Chain Versions ---")

	chainVersions, err := client.Mocks().Versions(ctx, chainID)
	if err != nil {
		fmt.Printf("Versions returned: %v\n", err)
	} else {
		fmt.Printf("Mocks in chain '%s': %d\n", chainID, len(chainVersions))
		for _, m := range chainVersions {
			route := ""
			if m.HTTP != nil {
				route = m.HTTP.HttpMethod + " " + m.HTTP.Route
			}
			fmt.Printf("  - %s (%s)\n", m.ID, route)
		}
	}

	// -----------------------------------------------------------------------
	// 12. Partial update (patch)
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Partial Update (Patch) ---")

	patched, err := client.Mocks().Patch(ctx, "prefixed-api", map[string]any{
		"tags":     []string{"payment", "v2", "patched"},
		"priority": 10,
	})
	if err != nil {
		fmt.Printf("Patch returned: %v\n", err)
	} else {
		fmt.Printf("Patched mock: tags=%v, priority=%d\n", patched.Tags, patched.Priority)
	}

	fmt.Println("\nAll advanced examples created successfully!")
}
