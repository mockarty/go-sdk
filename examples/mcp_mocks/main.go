// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: mcp_mocks — MCP (Model Context Protocol) mock examples.
//
// This example demonstrates:
//   - Tool mock (tools/call)
//   - Resource mock (resources/read)
//   - MCP error response
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
		"mcp-weather-tool",
		"mcp-config-resource",
		"mcp-tool-error",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll MCP example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. MCP tool mock (tools/call)
	// -----------------------------------------------------------------------
	// Mocks a weather lookup tool.
	toolMock := mockarty.NewMockBuilder().
		ID("mcp-weather-tool").
		MCPConfig(func(m *mockarty.MCPBuilderCtx) {
			m.Method("tools/call").
				Tool("get_weather").
				Description("Returns current weather for a given city")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Current weather in $.body.arguments.city: 22C, partly cloudy. Humidity: 65%.",
					},
				},
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, toolMock); err != nil {
		log.Fatalf("Failed to create MCP tool mock: %v", err)
	}
	fmt.Println("[1] Created MCP tool mock: get_weather (tools/call)")

	// -----------------------------------------------------------------------
	// 2. MCP resource mock (resources/read)
	// -----------------------------------------------------------------------
	// Mocks a configuration resource.
	resourceMock := mockarty.NewMockBuilder().
		ID("mcp-config-resource").
		MCPConfig(func(m *mockarty.MCPBuilderCtx) {
			m.Method("resources/read").
				Resource("config://app/settings").
				Description("Application configuration settings")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"contents": []map[string]any{
					{
						"uri":      "config://app/settings",
						"mimeType": "application/json",
						"text":     `{"debug": false, "maxRetries": 3, "timeout": "30s"}`,
					},
				},
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, resourceMock); err != nil {
		log.Fatalf("Failed to create MCP resource mock: %v", err)
	}
	fmt.Println("[2] Created MCP resource mock: config://app/settings (resources/read)")

	// -----------------------------------------------------------------------
	// 3. MCP tool error response
	// -----------------------------------------------------------------------
	// Returns an MCP tool-level error when validation fails.
	errorMock := mockarty.NewMockBuilder().
		ID("mcp-tool-error").
		MCPConfig(func(m *mockarty.MCPBuilderCtx) {
			m.Method("tools/call").
				Tool("validate_schema").
				BodyCondition("$.body.arguments.schema", mockarty.AssertEmpty, nil)
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.MCPIsError(true).
				JSONBody(map[string]any{
					"content": []map[string]any{
						{
							"type": "text",
							"text": "Error: schema argument is required and must not be empty",
						},
					},
					"isError": true,
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, errorMock); err != nil {
		log.Fatalf("Failed to create MCP error mock: %v", err)
	}
	fmt.Println("[3] Created MCP tool error mock: validate_schema (empty schema)")

	fmt.Println("\nAll MCP mock examples created successfully!")
}
