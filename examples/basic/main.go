// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: basic — Quick start with the Mockarty Go SDK.
//
// This example shows how to:
//   - Create a Mockarty client
//   - Create a simple HTTP mock
//   - Verify the mock was created
//   - Clean up by deleting the mock
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

	// -----------------------------------------------------------------------
	// 1. Create a client
	// -----------------------------------------------------------------------
	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 2. Build a simple HTTP mock using the fluent builder
	// -----------------------------------------------------------------------
	mock := mockarty.NewMockBuilder().
		ID("hello-world").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/hello").
				Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"message": "Hello from Mockarty!",
					"time":    "$.fake.Date",
				})
		}).
		Build()

	// -----------------------------------------------------------------------
	// 3. Create the mock on the server
	// -----------------------------------------------------------------------
	resp, err := client.Mocks().Create(ctx, mock)
	if err != nil {
		log.Fatalf("Failed to create mock: %v", err)
	}

	fmt.Printf("Mock created: id=%s, overwritten=%v\n", resp.Mock.ID, resp.Overwritten)

	// -----------------------------------------------------------------------
	// 4. Verify the mock exists by fetching it
	// -----------------------------------------------------------------------
	fetched, err := client.Mocks().Get(ctx, "hello-world")
	if err != nil {
		log.Fatalf("Failed to get mock: %v", err)
	}

	fmt.Printf("Fetched mock: id=%s, route=%s, method=%s\n",
		fetched.ID, fetched.HTTP.Route, fetched.HTTP.HttpMethod)

	// -----------------------------------------------------------------------
	// 5. Clean up — delete the mock
	// -----------------------------------------------------------------------
	if err := client.Mocks().Delete(ctx, "hello-world"); err != nil {
		log.Fatalf("Failed to delete mock: %v", err)
	}

	fmt.Println("Mock deleted successfully.")
}
