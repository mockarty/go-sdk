// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testing_helpers — Using Mockarty test helpers in Go tests.
//
// This file demonstrates patterns for using the Mockarty SDK in Go tests:
//   - CreateMockT with automatic cleanup
//   - SetupNamespaceT for isolated test namespaces
//   - MustCreateMock for builder-based test mocks
//   - Table-driven test patterns with mocks
//
// Note: This file is structured as a normal main package for compilation,
// but the patterns shown are meant to be used in _test.go files.
// See the comments for the actual test versions.
package main

import (
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	fmt.Println("=== Mockarty Testing Helpers ===")
	fmt.Println()
	fmt.Println("This example demonstrates patterns for using the Mockarty SDK")
	fmt.Println("in Go tests. The patterns below should be used in *_test.go files.")
	fmt.Println()

	// Show configuration from environment (common in CI/CD)
	baseURL := os.Getenv("MOCKARTY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5770"
	}
	apiKey := os.Getenv("MOCKARTY_API_KEY")
	if apiKey == "" {
		apiKey = "your-api-key"
	}

	fmt.Printf("Mockarty URL: %s\n", baseURL)
	fmt.Printf("API Key: %s\n", apiKey)
	fmt.Println()

	printCreateMockTExample()
	printSetupNamespaceTExample()
	printMustCreateMockExample()
	printTableDrivenExample()
}

func printCreateMockTExample() {
	fmt.Print(`--- Pattern 1: CreateMockT with automatic cleanup ---

// In your _test.go file:

func TestUserAPI(t *testing.T) {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
        mockarty.WithNamespace("test-"+t.Name()),
    )

    // CreateMockT creates the mock and automatically registers
    // a t.Cleanup() function that deletes it when the test ends.
    // No manual cleanup needed!
    mock := client.CreateMockT(t, &mockarty.Mock{
        ID: "test-get-user",
        HTTP: &mockarty.HttpRequestContext{
            Route:      "/api/users/123",
            HttpMethod: "GET",
        },
        Response: &mockarty.ContentResponse{
            StatusCode: 200,
            Payload: map[string]any{
                "id":   "123",
                "name": "Test User",
            },
        },
    })

    // Use the mock...
    t.Log("Created mock:", mock.ID)

    // Mock is automatically deleted when the test completes.
}

`)
}

func printSetupNamespaceTExample() {
	fmt.Print(`--- Pattern 2: SetupNamespaceT for test isolation ---

// In your _test.go file:

func TestOrderWorkflow(t *testing.T) {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
        mockarty.WithNamespace("test-orders"),
    )

    // SetupNamespaceT creates the namespace and registers a cleanup
    // function that deletes ALL mocks in the namespace when the test ends.
    // This ensures complete isolation between test runs.
    client.SetupNamespaceT(t, "test-orders")

    // Create multiple mocks in this namespace — all will be cleaned up
    client.CreateMockT(t, &mockarty.Mock{
        ID:        "create-order",
        Namespace: "test-orders",
        HTTP: &mockarty.HttpRequestContext{
            Route:      "/api/orders",
            HttpMethod: "POST",
        },
        Response: &mockarty.ContentResponse{
            StatusCode: 201,
            Payload:    map[string]any{"orderId": "ORD-001"},
        },
    })

    client.CreateMockT(t, &mockarty.Mock{
        ID:        "get-order",
        Namespace: "test-orders",
        HTTP: &mockarty.HttpRequestContext{
            Route:      "/api/orders/:id",
            HttpMethod: "GET",
        },
        Response: &mockarty.ContentResponse{
            StatusCode: 200,
            Payload:    map[string]any{"orderId": "$.pathParam.id", "status": "PENDING"},
        },
    })

    // Run your integration tests...
    // All mocks are cleaned up automatically.
}

`)
}

func printMustCreateMockExample() {
	// Show the builder pattern (can actually construct the builder here)
	_ = mockarty.NewMockBuilder().
		ID("example-builder").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/products/:id").Method("GET")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"id":    "$.pathParam.id",
				"name":  "$.fake.ProductName",
				"price": "$.fake.Float64",
			})
		}).
		Build()

	fmt.Print(`--- Pattern 3: MustCreateMock with builder ---

// In your _test.go file:

func TestProductAPI(t *testing.T) {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
        mockarty.WithNamespace("sandbox"),
    )

    // MustCreateMock combines the builder pattern with automatic cleanup.
    // It calls Build() on the builder and passes the result to CreateMockT.
    mock := client.MustCreateMock(t,
        mockarty.NewMockBuilder().
            ID("test-product").
            HTTP(func(h *mockarty.HTTPBuilder) {
                h.Route("/api/products/:id").Method("GET")
            }).
            Response(func(r *mockarty.ResponseBuilder) {
                r.Status(200).JSONBody(map[string]any{
                    "id":    "$.pathParam.id",
                    "name":  "$.fake.ProductName",
                    "price": "$.fake.Float64",
                })
            }),
    )

    t.Log("Created product mock:", mock.ID, "route:", mock.HTTP.Route)
}

`)
}

func printTableDrivenExample() {
	fmt.Print(`--- Pattern 4: Table-driven tests with mocks ---

// In your _test.go file:

func TestStatusEndpoints(t *testing.T) {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
        mockarty.WithNamespace("test-status"),
    )
    client.SetupNamespaceT(t, "test-status")

    tests := []struct {
        name       string
        mockID     string
        route      string
        method     string
        statusCode uint32
        body       map[string]any
    }{
        {
            name:       "health check returns 200",
            mockID:     "health-200",
            route:      "/api/health",
            method:     "GET",
            statusCode: 200,
            body:       map[string]any{"status": "ok"},
        },
        {
            name:       "not found returns 404",
            mockID:     "not-found-404",
            route:      "/api/missing",
            method:     "GET",
            statusCode: 404,
            body:       map[string]any{"error": "not found"},
        },
        {
            name:       "unauthorized returns 401",
            mockID:     "unauth-401",
            route:      "/api/admin/secret",
            method:     "GET",
            statusCode: 401,
            body:       map[string]any{"error": "unauthorized"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            client.MustCreateMock(t,
                mockarty.NewMockBuilder().
                    ID(tt.mockID).
                    Namespace("test-status").
                    HTTP(func(h *mockarty.HTTPBuilder) {
                        h.Route(tt.route).Method(tt.method)
                    }).
                    Response(func(r *mockarty.ResponseBuilder) {
                        r.Status(tt.statusCode).JSONBody(tt.body)
                    }),
            )

            // Now call the mock endpoint and verify the response.
            // resp, err := http.Get(client.BaseURL() + tt.route)
            // ... assert status code and body ...
        })
    }
}

`)
}
