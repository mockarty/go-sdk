// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: mcp_client — drive Mockarty's agent-facing MCP tool surface.
//
// This example demonstrates:
//   - Connecting to the admin node's streamable-HTTP /mcp endpoint
//   - Listing every tool the server advertises (list_mocks, create_mock, …)
//   - Calling a tool with typed arguments and reading its structured result
//
// The MCP client reuses the SDK client's server URL + API key; feature/licence
// gating for the tools is enforced server-side.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	baseURL := envOr("MOCKARTY_SERVER", "http://localhost:5770")
	apiKey := os.Getenv("MOCKARTY_API_KEY")

	client := mockarty.NewClient(baseURL, mockarty.WithAPIKey(apiKey))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mcp := client.MCP()

	// 1. Discover the tools the server exposes.
	tools, err := mcp.ListTools(ctx)
	if err != nil {
		log.Fatalf("list tools: %v", err)
	}
	fmt.Printf("Server advertises %d MCP tools:\n", len(tools))
	for _, t := range tools {
		fmt.Printf("  - %-28s %s\n", t.Name, t.Description)
	}

	// 2. Call a read-only tool and read its JSON result.
	res, err := mcp.CallTool(ctx, "list_mocks", map[string]any{})
	if err != nil {
		log.Fatalf("call list_mocks: %v", err)
	}
	if res.IsError {
		log.Fatalf("tool returned an error: %s", res.Text())
	}
	fmt.Printf("\nlist_mocks result:\n%s\n", res.Text())
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
