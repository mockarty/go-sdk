// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
)

// GeneratorAPI provides methods for generating mocks from API specifications.
type GeneratorAPI struct {
	client *Client
}

// GeneratorRequest defines the input for mock generation from a specification.
type GeneratorRequest struct {
	Spec       string `json:"spec,omitempty"`
	URL        string `json:"url,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	PathPrefix string `json:"pathPrefix,omitempty"`
	ServerName string `json:"serverName,omitempty"`
	GraphQLURL string `json:"graphqlUrl,omitempty"`
}

// GeneratorPreview holds the preview of mocks that would be generated.
type GeneratorPreview struct {
	Mocks []Mock `json:"mocks"`
	Count int    `json:"count"`
}

// GeneratorResponse holds the result of a mock generation operation.
type GeneratorResponse struct {
	Created int    `json:"created"`
	Mocks   []Mock `json:"mocks,omitempty"`
	Message string `json:"message,omitempty"`
}

// FromOpenAPI generates mocks from an OpenAPI/Swagger specification.
func (a *GeneratorAPI) FromOpenAPI(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/openapi", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromWSDL generates mocks from a WSDL/SOAP specification.
func (a *GeneratorAPI) FromWSDL(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/soap", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromProto generates mocks from a Protocol Buffers (.proto) specification.
func (a *GeneratorAPI) FromProto(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/grpc", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromGraphQL generates mocks from a GraphQL schema or introspection endpoint.
func (a *GeneratorAPI) FromGraphQL(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/graphql", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromHAR generates mocks from an HTTP Archive (HAR) file.
func (a *GeneratorAPI) FromHAR(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/har", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromSocket generates mocks for Socket (WebSocket/TCP/UDP) from a specification.
func (a *GeneratorAPI) FromSocket(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/socket", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewOpenAPI previews mocks that would be generated from an OpenAPI specification.
func (a *GeneratorAPI) PreviewOpenAPI(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/openapi/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewWSDL previews mocks that would be generated from a WSDL specification.
func (a *GeneratorAPI) PreviewWSDL(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/soap/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewProto previews mocks that would be generated from a Proto specification.
func (a *GeneratorAPI) PreviewProto(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/grpc/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewGraphQL previews mocks that would be generated from a GraphQL schema.
func (a *GeneratorAPI) PreviewGraphQL(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/graphql/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewHAR previews mocks that would be generated from an HAR file.
func (a *GeneratorAPI) PreviewHAR(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/har/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// FromMCP generates mocks from an MCP (Model Context Protocol) server specification.
func (a *GeneratorAPI) FromMCP(ctx context.Context, spec *GeneratorRequest) (*GeneratorResponse, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/mcp", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PreviewMCP previews mocks that would be generated from an MCP specification.
func (a *GeneratorAPI) PreviewMCP(ctx context.Context, spec *GeneratorRequest) (*GeneratorPreview, error) {
	if spec.Namespace == "" && a.client.namespace != "" {
		spec.Namespace = a.client.namespace
	}
	var resp GeneratorPreview
	if err := a.client.do(ctx, "POST", "/api/v1/generators/mcp/preview", spec, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// LoadGraphQLSchemaFromURL loads a GraphQL schema from an introspection endpoint.
func (a *GeneratorAPI) LoadGraphQLSchemaFromURL(ctx context.Context, url string) (map[string]any, error) {
	var resp map[string]any
	if err := a.client.do(ctx, "POST", "/api/v1/generators/graphql/schema", map[string]string{"graphqlUrl": url}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
