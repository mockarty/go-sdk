// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: graphql_mocks — GraphQL mock examples.
//
// This example demonstrates:
//   - Query mock with operation and field
//   - Mutation mock
//   - GraphQL error responses
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
		"graphql-users-query",
		"graphql-create-user",
		"graphql-error",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll GraphQL example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. GraphQL query mock
	// -----------------------------------------------------------------------
	// Matches a GraphQL query on the "users" field.
	queryMock := mockarty.NewMockBuilder().
		ID("graphql-users-query").
		GraphQLConfig(func(g *mockarty.GraphQLBuilder) {
			g.Operation("query").
				Field("users").
				Path("/graphql")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"data": map[string]any{
						"users": []map[string]any{
							{
								"id":    "$.fake.UUID",
								"name":  "$.fake.FirstName",
								"email": "$.fake.Email",
							},
							{
								"id":    "$.fake.UUID",
								"name":  "$.fake.FirstName",
								"email": "$.fake.Email",
							},
						},
					},
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, queryMock); err != nil {
		log.Fatalf("Failed to create GraphQL query mock: %v", err)
	}
	fmt.Println("[1] Created GraphQL query mock: users")

	// -----------------------------------------------------------------------
	// 2. GraphQL mutation mock
	// -----------------------------------------------------------------------
	// Matches a GraphQL mutation on the "createUser" field.
	mutationMock := mockarty.NewMockBuilder().
		ID("graphql-create-user").
		GraphQLConfig(func(g *mockarty.GraphQLBuilder) {
			g.Operation("mutation").
				Field("createUser").
				Path("/graphql")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"data": map[string]any{
						"createUser": map[string]any{
							"id":        "$.fake.UUID",
							"name":      "$.body.variables.input.name",
							"email":     "$.body.variables.input.email",
							"createdAt": "$.fake.DateISO",
						},
					},
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, mutationMock); err != nil {
		log.Fatalf("Failed to create GraphQL mutation mock: %v", err)
	}
	fmt.Println("[2] Created GraphQL mutation mock: createUser")

	// -----------------------------------------------------------------------
	// 3. GraphQL error response
	// -----------------------------------------------------------------------
	// Returns a GraphQL error response following the June 2018 spec.
	errorMock := mockarty.NewMockBuilder().
		ID("graphql-error").
		GraphQLConfig(func(g *mockarty.GraphQLBuilder) {
			g.Operation("query").
				Field("secretData").
				Path("/graphql")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200). // GraphQL errors still return 200
					JSONBody(map[string]any{
					"data": nil,
				}).
				GraphQLErrorList([]mockarty.GraphQLError{
					{
						Message: "You are not authorized to access this resource",
						Path:    []interface{}{"secretData"},
						Locations: []mockarty.GraphQLErrorLocation{
							{Line: 2, Column: 3},
						},
						Extensions: map[string]interface{}{
							"code":      "FORBIDDEN",
							"timestamp": "$.fake.DateISO",
						},
					},
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, errorMock); err != nil {
		log.Fatalf("Failed to create GraphQL error mock: %v", err)
	}
	fmt.Println("[3] Created GraphQL error mock: secretData (FORBIDDEN)")

	fmt.Println("\nAll GraphQL mock examples created successfully!")
}
