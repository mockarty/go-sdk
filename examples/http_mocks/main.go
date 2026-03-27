// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: http_mocks — Comprehensive HTTP protocol mock examples.
//
// This example demonstrates:
//   - GET with path parameters
//   - POST with body conditions
//   - PUT with header conditions
//   - Faker template expressions in responses
//   - Response delay
//   - OneOf responses (sequential and random)
//   - TTL and usage limiter
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

	// Clean up all examples at the end.
	mockIDs := []string{
		"get-user-by-id",
		"create-user",
		"update-user",
		"user-with-faker",
		"slow-endpoint",
		"sequential-responses",
		"random-responses",
		"limited-mock",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. GET with path parameters
	// -----------------------------------------------------------------------
	// Route: GET /api/users/:id
	// The :id segment is available as $.pathParam.id in the response template.
	getUserMock := mockarty.NewMockBuilder().
		ID("get-user-by-id").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/users/:id").
				Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"id":    "$.pathParam.id",
					"name":  "John Doe",
					"email": "john@example.com",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, getUserMock); err != nil {
		log.Fatalf("Failed to create GET mock: %v", err)
	}
	fmt.Println("[1] Created GET /api/users/:id mock with path params")

	// -----------------------------------------------------------------------
	// 2. POST with body conditions
	// -----------------------------------------------------------------------
	// This mock only matches when $.body.email equals "admin@example.com".
	createUserMock := mockarty.NewMockBuilder().
		ID("create-user").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/users").
				Method("POST").
				BodyCondition("$.body.email", mockarty.AssertEquals, "admin@example.com")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(201).
				JSONBody(map[string]any{
					"id":      "$.fake.UUID",
					"email":   "$.body.email",
					"message": "User created successfully",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, createUserMock); err != nil {
		log.Fatalf("Failed to create POST mock: %v", err)
	}
	fmt.Println("[2] Created POST /api/users mock with body condition (email equals)")

	// -----------------------------------------------------------------------
	// 3. PUT with header conditions
	// -----------------------------------------------------------------------
	// This mock only matches when the Authorization header contains "Bearer".
	updateUserMock := mockarty.NewMockBuilder().
		ID("update-user").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/users/:id").
				Method("PUT").
				HeaderCondition("Authorization", mockarty.AssertContains, "Bearer")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"id":      "$.pathParam.id",
					"updated": true,
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, updateUserMock); err != nil {
		log.Fatalf("Failed to create PUT mock: %v", err)
	}
	fmt.Println("[3] Created PUT /api/users/:id mock with header condition")

	// -----------------------------------------------------------------------
	// 4. Response with Faker templates
	// -----------------------------------------------------------------------
	// Mockarty evaluates $.fake.* expressions server-side to produce
	// dynamic, realistic test data on every call.
	fakerMock := mockarty.NewMockBuilder().
		ID("user-with-faker").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/users/random").
				Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"id":        "$.fake.UUID",
					"email":     "$.fake.Email",
					"firstName": "$.fake.FirstName",
					"lastName":  "$.fake.LastName",
					"phone":     "$.fake.PhoneNumber",
					"address": map[string]any{
						"street": "$.fake.StreetAddress",
						"city":   "$.fake.City",
						"zip":    "$.fake.Zip",
					},
					"createdAt": "$.fake.DateISO",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, fakerMock); err != nil {
		log.Fatalf("Failed to create Faker mock: %v", err)
	}
	fmt.Println("[4] Created GET /api/users/random mock with Faker templates")

	// -----------------------------------------------------------------------
	// 5. Response with delay
	// -----------------------------------------------------------------------
	// The server waits 2000 ms before returning the response.
	// Useful for testing timeouts and loading states.
	slowMock := mockarty.NewMockBuilder().
		ID("slow-endpoint").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/slow").
				Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				Delay(2000). // 2 seconds
				JSONBody(map[string]any{
					"message": "This response was delayed by 2 seconds",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, slowMock); err != nil {
		log.Fatalf("Failed to create slow mock: %v", err)
	}
	fmt.Println("[5] Created GET /api/slow mock with 2s delay")

	// -----------------------------------------------------------------------
	// 6. OneOf responses — sequential (round-robin)
	// -----------------------------------------------------------------------
	// Each subsequent call returns the next response in order.
	sequentialMock := mockarty.NewMockBuilder().
		ID("sequential-responses").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/status").
				Method("GET")
		}).
		OneOfConfig(mockarty.OneOfOrderSequential,
			// Response 1: success
			func(r *mockarty.ResponseBuilder) {
				r.Status(200).
					JSONBody(map[string]any{"status": "ok"})
			},
			// Response 2: degraded
			func(r *mockarty.ResponseBuilder) {
				r.Status(200).
					JSONBody(map[string]any{"status": "degraded"})
			},
			// Response 3: error
			func(r *mockarty.ResponseBuilder) {
				r.Status(503).
					JSONBody(map[string]any{"status": "down", "error": "service unavailable"})
			},
		).
		Build()

	if _, err := client.Mocks().Create(ctx, sequentialMock); err != nil {
		log.Fatalf("Failed to create sequential OneOf mock: %v", err)
	}
	fmt.Println("[6] Created GET /api/status mock with sequential OneOf (3 responses)")

	// -----------------------------------------------------------------------
	// 7. OneOf responses — random
	// -----------------------------------------------------------------------
	// Each call returns a random response from the list.
	// Great for simulating flaky services.
	randomMock := mockarty.NewMockBuilder().
		ID("random-responses").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/flaky").
				Method("GET")
		}).
		OneOfConfig(mockarty.OneOfOrderRandom,
			func(r *mockarty.ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"result": "success"})
			},
			func(r *mockarty.ResponseBuilder) {
				r.Status(500).JSONBody(map[string]any{"error": "internal server error"})
			},
			func(r *mockarty.ResponseBuilder) {
				r.Status(429).JSONBody(map[string]any{"error": "too many requests"})
			},
		).
		Build()

	if _, err := client.Mocks().Create(ctx, randomMock); err != nil {
		log.Fatalf("Failed to create random OneOf mock: %v", err)
	}
	fmt.Println("[7] Created GET /api/flaky mock with random OneOf")

	// -----------------------------------------------------------------------
	// 8. Mock with TTL and usage limiter
	// -----------------------------------------------------------------------
	// TTL: mock auto-deletes after 300 seconds (5 minutes).
	// UseLimiter: mock stops matching after 10 uses.
	limitedMock := mockarty.NewMockBuilder().
		ID("limited-mock").
		TTL(300).
		UseLimiter(10).
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/limited").
				Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"message": "This mock expires after 5 min or 10 uses",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, limitedMock); err != nil {
		log.Fatalf("Failed to create limited mock: %v", err)
	}
	fmt.Println("[8] Created GET /api/limited mock with TTL=300s and UseLimiter=10")

	fmt.Println("\nAll HTTP mock examples created successfully!")
}
