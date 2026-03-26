// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: undefined_requests — Managing unmatched requests.
//
// This example demonstrates:
//   - List all unmatched (undefined) requests
//   - Create mocks from unmatched requests
//   - Ignore unmatched requests
//   - Delete specific unmatched requests
//   - Clear all unmatched requests
//
// Undefined requests are requests that arrived at Mockarty but did not match
// any configured mock. They are captured automatically and can be used to
// discover missing mocks during integration testing.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	// 0. Generate some unmatched requests for demonstration
	// -----------------------------------------------------------------------
	fmt.Println("--- Generate Unmatched Requests ---")

	// Send some requests that won't match any mocks to populate the
	// undefined requests list.
	httpClient := &http.Client{Timeout: 5 * time.Second}
	unmatchedPaths := []string{
		"/api/products/42",
		"/api/orders/latest",
		"/api/inventory/check",
		"/api/notifications/unread",
	}

	for _, path := range unmatchedPaths {
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://localhost:5770"+path, nil)
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Printf("  Request to %s failed: %v\n", path, err)
			continue
		}
		resp.Body.Close()
		fmt.Printf("  Sent request: GET %s -> %d\n", path, resp.StatusCode)
	}

	time.Sleep(1 * time.Second) // allow time for capture

	// -----------------------------------------------------------------------
	// 1. List all unmatched requests
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Undefined Requests ---")

	requests, err := client.Undefined().List(ctx)
	if err != nil {
		log.Fatalf("Failed to list undefined requests: %v", err)
	}

	fmt.Printf("Found %d unmatched requests:\n", len(requests))
	for _, r := range requests {
		ts := "unknown"
		if r.Timestamp > 0 {
			ts = time.Unix(r.Timestamp, 0).Format(time.RFC3339)
		}
		fmt.Printf("  - %s %s (protocol=%s, count=%d, at=%s)\n",
			r.Method, r.Path, r.Protocol, r.Count, ts)
	}

	if len(requests) == 0 {
		fmt.Println("No unmatched requests found.")
		fmt.Println("Send requests to Mockarty that don't match any mock to populate this list.")
		return
	}

	// -----------------------------------------------------------------------
	// 2. Create a mock from an unmatched request
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Create Mock from Undefined Request ---")

	// Pick the first unmatched request and auto-generate a mock for it.
	// This is a great way to quickly build mocks during development:
	// send real requests, then convert the captures into mocks.
	firstRequest := requests[0]
	mock, err := client.Undefined().CreateMock(ctx, firstRequest.ID)
	if err != nil {
		fmt.Printf("Create mock returned: %v\n", err)
	} else {
		route := ""
		method := ""
		if mock.HTTP != nil {
			route = mock.HTTP.Route
			method = mock.HTTP.HttpMethod
		}
		fmt.Printf("Created mock from unmatched request:\n")
		fmt.Printf("  Mock ID: %s\n", mock.ID)
		fmt.Printf("  Route: %s %s\n", method, route)

		// Clean up the generated mock
		defer func() {
			_ = client.Mocks().Delete(ctx, mock.ID)
			fmt.Println("\nGenerated mock cleaned up.")
		}()
	}

	// -----------------------------------------------------------------------
	// 3. Ignore an unmatched request
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Ignore Undefined Request ---")

	// Ignoring marks a request so it no longer appears in the undefined
	// requests dashboard. Useful for known endpoints you don't want to mock
	// (health checks, metrics, etc.).
	if len(requests) > 1 {
		ignoreReq := requests[1]
		err = client.Undefined().Ignore(ctx, ignoreReq.ID)
		if err != nil {
			fmt.Printf("Ignore returned: %v\n", err)
		} else {
			fmt.Printf("Ignored unmatched request: %s %s\n", ignoreReq.Method, ignoreReq.Path)
		}
	}

	// -----------------------------------------------------------------------
	// 4. Delete specific unmatched requests
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Delete Specific Undefined Requests ---")

	if len(requests) >= 3 {
		deleteIDs := []string{requests[2].ID}
		if len(requests) >= 4 {
			deleteIDs = append(deleteIDs, requests[3].ID)
		}

		err = client.Undefined().Delete(ctx, deleteIDs)
		if err != nil {
			fmt.Printf("Delete returned: %v\n", err)
		} else {
			fmt.Printf("Deleted %d unmatched requests\n", len(deleteIDs))
		}
	}

	// -----------------------------------------------------------------------
	// 5. List remaining requests
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Remaining Undefined Requests ---")

	remaining, err := client.Undefined().List(ctx)
	if err != nil {
		fmt.Printf("List returned: %v\n", err)
	} else {
		fmt.Printf("Remaining: %d unmatched requests\n", len(remaining))
		for _, r := range remaining {
			fmt.Printf("  - %s %s\n", r.Method, r.Path)
		}
	}

	// -----------------------------------------------------------------------
	// 6. Clear all unmatched requests
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Clear All Undefined Requests ---")

	err = client.Undefined().ClearAll(ctx)
	if err != nil {
		fmt.Printf("Clear all returned: %v\n", err)
	} else {
		fmt.Println("All unmatched requests cleared")
	}

	// Verify clearing worked
	after, err := client.Undefined().List(ctx)
	if err != nil {
		fmt.Printf("List returned: %v\n", err)
	} else {
		fmt.Printf("Remaining after clear: %d requests\n", len(after))
	}

	fmt.Println("\nUndefined requests examples completed!")
}
