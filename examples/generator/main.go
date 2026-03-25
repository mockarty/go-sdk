// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: generator — Mock generation from specifications.
//
// This example demonstrates:
//   - Preview mocks from an OpenAPI spec (dry-run)
//   - Generate mocks from an OpenAPI spec
//   - Generate from other formats (WSDL, Proto, GraphQL, HAR)
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
	// 1. Preview mocks from an OpenAPI spec (dry-run)
	// -----------------------------------------------------------------------
	fmt.Println("--- Preview OpenAPI Mock Generation ---")

	// PreviewOpenAPI shows what mocks would be created without actually
	// creating them. This is useful for reviewing before committing.
	preview, err := client.Generator().PreviewOpenAPI(ctx, &mockarty.GeneratorRequest{
		URL:        "https://petstore.swagger.io/v2/swagger.json",
		PathPrefix: "/petstore",
		ServerName: "petstore-api",
	})
	if err != nil {
		fmt.Printf("Preview returned: %v\n", err)
	} else {
		fmt.Printf("Preview: %d mocks would be generated\n", preview.Count)
		for i, m := range preview.Mocks {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", preview.Count-5)
				break
			}
			route := ""
			method := ""
			if m.HTTP != nil {
				route = m.HTTP.Route
				method = m.HTTP.HttpMethod
			}
			fmt.Printf("  - %s %s %s\n", m.ID, method, route)
		}
	}

	// -----------------------------------------------------------------------
	// 2. Generate mocks from an OpenAPI spec
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Generate Mocks from OpenAPI ---")

	// FromOpenAPI creates mocks from an OpenAPI/Swagger specification.
	// You can pass the spec inline (Spec field) or by URL.
	result, err := client.Generator().FromOpenAPI(ctx, &mockarty.GeneratorRequest{
		URL:        "https://petstore.swagger.io/v2/swagger.json",
		PathPrefix: "/petstore",
		ServerName: "petstore-api",
	})
	if err != nil {
		fmt.Printf("Generate returned: %v\n", err)
	} else {
		fmt.Printf("Generated %d mocks\n", result.Created)
		if result.Message != "" {
			fmt.Printf("Server message: %s\n", result.Message)
		}
		for i, m := range result.Mocks {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", result.Created-5)
				break
			}
			route := ""
			method := ""
			if m.HTTP != nil {
				route = m.HTTP.Route
				method = m.HTTP.HttpMethod
			}
			fmt.Printf("  - %s %s %s\n", m.ID, method, route)
		}

		// Clean up generated mocks
		defer func() {
			for _, m := range result.Mocks {
				_ = client.Mocks().Delete(ctx, m.ID)
			}
			fmt.Println("\nGenerated mocks cleaned up.")
		}()
	}

	// -----------------------------------------------------------------------
	// 3. Generate from other formats
	// -----------------------------------------------------------------------
	// The SDK supports generating mocks from multiple specification formats.
	// Each follows the same pattern as FromOpenAPI.

	fmt.Println("\n--- Other Generator Methods ---")

	// WSDL / SOAP
	fmt.Println("FromWSDL:    generates SOAP mocks from a WSDL specification")
	fmt.Println("  client.Generator().FromWSDL(ctx, &mockarty.GeneratorRequest{")
	fmt.Println("      URL: \"https://example.com/service?wsdl\",")
	fmt.Println("  })")

	// Protocol Buffers / gRPC
	fmt.Println("\nFromProto:   generates gRPC mocks from .proto definitions")
	fmt.Println("  client.Generator().FromProto(ctx, &mockarty.GeneratorRequest{")
	fmt.Println("      Spec: protoFileContent,")
	fmt.Println("  })")

	// GraphQL
	fmt.Println("\nFromGraphQL: generates GraphQL mocks from a schema or introspection")
	fmt.Println("  client.Generator().FromGraphQL(ctx, &mockarty.GeneratorRequest{")
	fmt.Println("      GraphQLURL: \"https://api.example.com/graphql\",")
	fmt.Println("  })")

	// HAR
	fmt.Println("\nFromHAR:     generates mocks from an HTTP Archive (HAR) file")
	fmt.Println("  client.Generator().FromHAR(ctx, &mockarty.GeneratorRequest{")
	fmt.Println("      Spec: harFileContent,")
	fmt.Println("  })")

	// Each also has a Preview method: PreviewWSDL, PreviewProto, PreviewGraphQL, PreviewHAR

	// -----------------------------------------------------------------------
	// 4. Generate with inline spec
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Generate from Inline Spec ---")

	inlineSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "Simple API", "version": "1.0.0"},
		"paths": {
			"/api/ping": {
				"get": {
					"operationId": "ping",
					"responses": {
						"200": {
							"description": "Pong",
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"properties": {
											"message": {"type": "string"}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	inlineResult, err := client.Generator().FromOpenAPI(ctx, &mockarty.GeneratorRequest{
		Spec: inlineSpec,
	})
	if err != nil {
		fmt.Printf("Inline generate returned: %v\n", err)
	} else {
		fmt.Printf("Generated %d mocks from inline spec\n", inlineResult.Created)
		// Clean up
		for _, m := range inlineResult.Mocks {
			_ = client.Mocks().Delete(ctx, m.ID)
		}
	}

	fmt.Println("\nGenerator examples completed!")
}

// logMock is a helper for formatting mock output
func logMock(m mockarty.Mock) string {
	route := ""
	method := ""
	if m.HTTP != nil {
		route = m.HTTP.Route
		method = m.HTTP.HttpMethod
	}
	return fmt.Sprintf("%s %s %s", m.ID, method, route)
}

func init() {
	// Suppress unused warning
	_ = log.Fatalf
}
