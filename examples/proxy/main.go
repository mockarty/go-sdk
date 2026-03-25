// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: proxy — Proxy mode and Proxy API usage.
//
// This example demonstrates:
//   - Create mocks that proxy to real services
//   - Proxy with response delay (testing timeouts)
//   - Use the Proxy API to send HTTP, SOAP, and gRPC requests through Mockarty
//   - Proxy with header substitution
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
		"proxy-basic",
		"proxy-with-delay",
		"proxy-with-headers",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll proxy example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. Basic proxy to a real service
	// -----------------------------------------------------------------------
	fmt.Println("--- Basic Proxy Mock ---")

	// Instead of returning a mocked response, Mockarty forwards the request
	// to the real service and returns the real response. The request is still
	// logged for observation.
	basicProxy := mockarty.NewMockBuilder().
		ID("proxy-basic").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/external/users/:id").
				Method("GET")
		}).
		ProxyTo("https://api.example.com").
		Build()

	if _, err := client.Mocks().Create(ctx, basicProxy); err != nil {
		log.Fatalf("Failed to create basic proxy mock: %v", err)
	}
	fmt.Println("[1] Created proxy mock: /api/external/users/:id -> https://api.example.com")

	// -----------------------------------------------------------------------
	// 2. Proxy with delay
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy with Delay ---")

	// Forwards to the real service but adds a delay AFTER receiving the
	// real response. Useful for testing how your application handles slow
	// dependencies without modifying the actual service.
	delayProxy := mockarty.NewMockBuilder().
		ID("proxy-with-delay").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/external/products").
				Method("GET")
		}).
		ProxyTo("https://api.example.com").
		Build()

	// The delay in proxy mode is configured at the response level.
	delayProxy.Response = &mockarty.ContentResponse{
		Delay: 3000, // 3 second delay added to the real response time
	}

	if _, err := client.Mocks().Create(ctx, delayProxy); err != nil {
		log.Fatalf("Failed to create delay proxy mock: %v", err)
	}
	fmt.Println("[2] Created proxy mock with 3s delay: /api/external/products -> https://api.example.com")

	// -----------------------------------------------------------------------
	// 3. Proxy with header substitution
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy with Header Substitution ---")

	// Proxy the request but replace/add specific response headers.
	// Useful for modifying CORS headers, adding debug headers, etc.
	headerProxy := mockarty.NewMockBuilder().
		ID("proxy-with-headers").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/external/orders/:id").
				Method("GET")
		}).
		ProxyTo("https://api.example.com").
		Build()

	headerProxy.Response = &mockarty.ContentResponse{
		Headers: map[string][]string{
			"X-Proxy-By":                 {"mockarty"},
			"Access-Control-Allow-Origin": {"*"},
		},
	}

	if _, err := client.Mocks().Create(ctx, headerProxy); err != nil {
		log.Fatalf("Failed to create header proxy mock: %v", err)
	}
	fmt.Println("[3] Created proxy mock with header substitution: /api/external/orders/:id")

	// -----------------------------------------------------------------------
	// 4. Proxy API — HTTP proxy request
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy API: HTTP ---")

	// The Proxy API sends a request through Mockarty's proxy infrastructure.
	// This is different from mock-level proxy: it directly proxies a single
	// request without requiring a mock to be configured.
	httpResult, err := client.Proxy().HTTP(ctx, map[string]any{
		"url":    "https://httpbin.org/get",
		"method": "GET",
		"headers": map[string]string{
			"Accept": "application/json",
		},
	})
	if err != nil {
		fmt.Printf("HTTP proxy returned: %v\n", err)
	} else {
		fmt.Printf("HTTP proxy result: %v\n", httpResult)
	}

	// -----------------------------------------------------------------------
	// 5. Proxy API — SOAP proxy request
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy API: SOAP ---")

	soapResult, err := client.Proxy().SOAP(ctx, map[string]any{
		"url":    "https://soap.example.com/ws",
		"action": "GetUser",
		"body":   `<GetUser><UserId>42</UserId></GetUser>`,
	})
	if err != nil {
		fmt.Printf("SOAP proxy returned: %v\n", err)
	} else {
		fmt.Printf("SOAP proxy result: %v\n", soapResult)
	}

	// -----------------------------------------------------------------------
	// 6. Proxy API — gRPC proxy request
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Proxy API: gRPC ---")

	grpcResult, err := client.Proxy().GRPC(ctx, map[string]any{
		"target":  "grpc.example.com:443",
		"service": "user.UserService",
		"method":  "GetUser",
		"body": map[string]any{
			"user_id": 42,
		},
	})
	if err != nil {
		fmt.Printf("gRPC proxy returned: %v\n", err)
	} else {
		fmt.Printf("gRPC proxy result: %v\n", grpcResult)
	}

	fmt.Println("\nAll proxy examples created successfully!")
	fmt.Println("\nNote: Proxy mode forwards requests to real services.")
	fmt.Println("Make sure the target URLs are reachable from your Mockarty instance.")
}
