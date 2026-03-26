// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: grpc_mocks — gRPC protocol mock examples.
//
// This example demonstrates:
//   - Unary method mock with service/method
//   - Conditions on request fields
//   - Metadata conditions
//   - Error response with gRPC status code
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
		"grpc-get-user",
		"grpc-user-condition",
		"grpc-metadata-auth",
		"grpc-not-found",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll gRPC example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. Basic unary gRPC mock
	// -----------------------------------------------------------------------
	// Mocks the UserService.GetUser RPC method.
	basicGRPC := mockarty.NewMockBuilder().
		ID("grpc-get-user").
		GRPC(func(g *mockarty.GRPCBuilder) {
			g.Service("user.UserService").
				Method("GetUser").
				MethodType("unary")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"id":    "$.fake.UUID",
				"name":  "$.fake.FirstName",
				"email": "$.fake.Email",
				"role":  "developer",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, basicGRPC); err != nil {
		log.Fatalf("Failed to create basic gRPC mock: %v", err)
	}
	fmt.Println("[1] Created gRPC mock: UserService.GetUser (unary)")

	// -----------------------------------------------------------------------
	// 2. gRPC mock with body (request field) conditions
	// -----------------------------------------------------------------------
	// Only matches when the request field "user_id" equals "admin-001".
	conditionGRPC := mockarty.NewMockBuilder().
		ID("grpc-user-condition").
		Priority(10). // higher priority to match before the basic mock
		GRPC(func(g *mockarty.GRPCBuilder) {
			g.Service("user.UserService").
				Method("GetUser").
				MethodType("unary").
				BodyCondition("$.body.user_id", mockarty.AssertEquals, "admin-001")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"id":    "admin-001",
				"name":  "Super Admin",
				"email": "admin@company.com",
				"role":  "admin",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, conditionGRPC); err != nil {
		log.Fatalf("Failed to create conditional gRPC mock: %v", err)
	}
	fmt.Println("[2] Created gRPC mock with body condition (user_id == admin-001)")

	// -----------------------------------------------------------------------
	// 3. gRPC mock with metadata conditions
	// -----------------------------------------------------------------------
	// Only matches when the x-auth-token metadata key is present and non-empty.
	metadataGRPC := mockarty.NewMockBuilder().
		ID("grpc-metadata-auth").
		GRPC(func(g *mockarty.GRPCBuilder) {
			g.Service("auth.AuthService").
				Method("ValidateToken").
				MetaCondition("x-auth-token", mockarty.AssertNotEmpty, nil)
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"valid":     true,
				"userId":    "$.fake.UUID",
				"expiresIn": 3600,
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, metadataGRPC); err != nil {
		log.Fatalf("Failed to create metadata gRPC mock: %v", err)
	}
	fmt.Println("[3] Created gRPC mock with metadata condition (x-auth-token notEmpty)")

	// -----------------------------------------------------------------------
	// 4. gRPC error response with status code
	// -----------------------------------------------------------------------
	// Returns a gRPC NOT_FOUND error (status code 5) with error details.
	errorGRPC := mockarty.NewMockBuilder().
		ID("grpc-not-found").
		GRPC(func(g *mockarty.GRPCBuilder) {
			g.Service("order.OrderService").
				Method("GetOrder").
				BodyCondition("$.body.order_id", mockarty.AssertEquals, "nonexistent")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(5). // gRPC NOT_FOUND = 5
					Error("order not found").
					ErrorDetailsList([]mockarty.ErrorDetails{
					{
						Type: mockarty.ErrorDetailsResourceInfo,
						Details: map[string]interface{}{
							"resourceType": "Order",
							"resourceName": "nonexistent",
							"description":  "The requested order does not exist",
						},
					},
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, errorGRPC); err != nil {
		log.Fatalf("Failed to create error gRPC mock: %v", err)
	}
	fmt.Println("[4] Created gRPC mock with NOT_FOUND error and error details")

	fmt.Println("\nAll gRPC mock examples created successfully!")
}
