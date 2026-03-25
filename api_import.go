// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
)

// ImportAPI provides methods for importing API definitions into collections.
type ImportAPI struct {
	client *Client
}

// ImportResult holds the result of an import operation.
type ImportResult struct {
	CollectionID string `json:"collectionId,omitempty"`
	Name         string `json:"name,omitempty"`
	Imported     int    `json:"imported,omitempty"`
	Message      string `json:"message,omitempty"`
}

// importPayload wraps raw bytes for import endpoints.
type importPayload struct {
	Data string `json:"data"`
}

// Postman imports a Postman collection.
func (a *ImportAPI) Postman(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/postman", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// OpenAPI imports an OpenAPI/Swagger specification into a collection.
func (a *ImportAPI) OpenAPI(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/openapi", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// WSDL imports a WSDL specification into a collection.
func (a *ImportAPI) WSDL(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/wsdl", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// HAR imports an HTTP Archive (HAR) file into a collection.
func (a *ImportAPI) HAR(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/har", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GrpcProto imports a Protocol Buffers (.proto) definition into a collection.
func (a *ImportAPI) GrpcProto(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/grpc", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GraphQL imports a GraphQL schema into a collection.
func (a *ImportAPI) GraphQL(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/graphql", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MCP imports an MCP definition into a collection.
func (a *ImportAPI) MCP(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/mcp", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Mockarty imports a Mockarty export into a collection.
func (a *ImportAPI) Mockarty(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/mockarty", &importPayload{Data: string(data)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
