// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"context"
)

// GeneratorAPI provides methods for generating mocks from API specifications.
type GeneratorAPI struct {
	client *Client
}

// GeneratorRequest defines the input for mock generation from a specification.
//
// Wire field renamed: the server reads `content` (inline spec body),
// NOT `spec`. Older SDK sent `spec` and every generator call 400'd
// with 'either url or content must be provided'. The Go-friendly
// .Spec field is preserved; only the JSON tag changes.
type GeneratorRequest struct {
	Spec       string `json:"content,omitempty"`
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

// SpecExportRequest defines the input for building a specification FROM mocks —
// the reverse of GeneratorRequest, which describes mocks with a specification.
type SpecExportRequest struct {
	// Format is required: "openapi" or "asyncapi".
	Format string `json:"format"`
	// Namespace defaults to the client's namespace when empty.
	Namespace string `json:"namespace,omitempty"`
	// FolderID, Tags and MockIDs narrow which mocks are described. Tags are
	// ANDed: a mock must carry every one of them.
	FolderID string   `json:"folderId,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	MockIDs  []string `json:"mockIds,omitempty"`
	Title    string   `json:"title,omitempty"`
	Version  string   `json:"version,omitempty"`
	// ServerURL fills servers[] in OpenAPI. Leave it empty when the address is
	// unknown: a document that invents a server is wrong in a way its reader
	// cannot see.
	ServerURL string `json:"serverUrl,omitempty"`
}

// SpecExportWarning names a mock the document could not fully express.
//
// Read these BEFORE trusting a document: a mock whose response is built by a
// script, rotates through oneOf variants, proxies a real upstream or reads a
// template file has no place in a specification, and the operation it belongs to
// stays in the document with a default response. An empty Warnings slice means
// every mock translated, not that some were skipped silently — Stats accounts for
// those.
type SpecExportWarning struct {
	MockID string `json:"mockId"`
	Route  string `json:"route,omitempty"`
	Reason string `json:"reason"`
}

// SpecExportStats accounts for every mock considered, so a small document can be
// told apart from a filter that matched nothing.
type SpecExportStats struct {
	Considered int `json:"considered"`
	Exported   int `json:"exported"`
	Skipped    int `json:"skipped"`
	Paths      int `json:"paths,omitempty"`
	Operations int `json:"operations,omitempty"`
	Channels   int `json:"channels,omitempty"`
}

// SpecExportResponse is a built specification plus what it could not carry.
type SpecExportResponse struct {
	// Spec is the document itself (OpenAPI 3.0 or AsyncAPI 2.6, JSON).
	Spec     map[string]any      `json:"spec"`
	Format   string              `json:"format"`
	Warnings []SpecExportWarning `json:"warnings"`
	Stats    SpecExportStats     `json:"stats"`
}

// ExportSpec builds an API specification FROM the namespace's mocks.
func (a *GeneratorAPI) ExportSpec(ctx context.Context, req *SpecExportRequest) (*SpecExportResponse, error) {
	if req.Namespace == "" && a.client.namespace != "" {
		req.Namespace = a.client.namespace
	}
	var resp SpecExportResponse
	if err := a.client.do(ctx, "POST", "/api/v1/generators/spec", req, &resp); err != nil {
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
