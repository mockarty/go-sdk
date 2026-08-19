// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ImportAPI provides methods for importing API definitions into collections.
type ImportAPI struct {
	client *Client
}

// ImportResult holds the result of an import operation.
//
// Multi-format reality: Postman + OpenAPI + HAR each return their own
// shape. The richest is Postman's
//
//	{collections: [{id, name, protocol}, ...],
//	 requests:    [{id, collectionId, name, requestData}, ...],
//	 seededMocks: {created, skipped, namespace, ...},
//	 summary:     {totalCollections, importedCollections, ...}}
//
// — and OpenAPI/HAR similarly return a Collections list. The SDK
// projects:
//   - CollectionID is the FIRST collection's id (most-common path: one
//     collection per import call).
//   - Collections holds the full list when the caller needs more than
//     one (multi-folder Postman exports surface here).
//   - Imported counts the total imported requests.
type ImportResult struct {
	CollectionID string               `json:"-"`
	Collections  []ImportedCollection `json:"collections,omitempty"`
	Requests     []ImportedRequest    `json:"requests,omitempty"`
	SeededMocks  *ImportSeededMocks   `json:"seededMocks,omitempty"`
	Summary      map[string]any       `json:"summary,omitempty"`
	Failures     []map[string]any     `json:"failures,omitempty"`
	Name         string               `json:"name,omitempty"`
	Imported     int                  `json:"imported,omitempty"`
	Message      string               `json:"message,omitempty"`
}

// ImportedCollection — one row of the `collections` response array.
type ImportedCollection struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

// ImportedRequest — one row of the `requests` response array.
type ImportedRequest struct {
	ID           string `json:"id,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	Name         string `json:"name,omitempty"`
	RequestData  any    `json:"requestData,omitempty"`
}

// ImportSeededMocks reports the optional mocks-from-saved-responses
// path that Postman's SeedMocks=true triggers.
type ImportSeededMocks struct {
	Namespace   string         `json:"namespace,omitempty"`
	SkipReasons map[string]any `json:"skipReasons,omitempty"`
	Created     int            `json:"created,omitempty"`
	Skipped     int            `json:"skipped,omitempty"`
}

// UnmarshalJSON projects the first collection's id onto the convenience
// CollectionID field so single-collection callers don't have to walk
// the array.
func (r *ImportResult) UnmarshalJSON(data []byte) error {
	type alias ImportResult
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = ImportResult(aux)
	if r.CollectionID == "" && len(r.Collections) > 0 {
		r.CollectionID = r.Collections[0].ID
	}
	// Also project the imported count from the summary when the
	// flat `imported` field is missing.
	if r.Imported == 0 && r.Summary != nil {
		if v, ok := r.Summary["importedRequests"]; ok {
			if n, ok := v.(float64); ok {
				r.Imported = int(n)
			}
		}
	}
	return nil
}

// importContentPayload mirrors the {content: "..."} envelope that
// the OpenAPI / WSDL / HAR / GraphQL / MCP import handlers accept on
// the server side.
type importContentPayload struct {
	Content        string `json:"content"`
	ContentType    string `json:"contentType,omitempty"`
	CollectionName string `json:"collectionName,omitempty"`
}

// Postman imports a Postman collection.
//
// Wire shape: the server expects
//
//	{"collectionJson": <decoded collection>, "collectionName": "...",
//	 "mode": "" | "performance", "seedMocks": false}
//
// — NOT a {"data": "<raw JSON string>"} envelope. The SDK decodes the
// supplied bytes for the caller and assembles the right payload.
func (a *ImportAPI) Postman(ctx context.Context, data []byte) (*ImportResult, error) {
	return a.PostmanWithOptions(ctx, data, PostmanImportOptions{})
}

// PostmanImportOptions enables the richer import knobs the server
// exposes. Optional; passing the zero value is equivalent to the bare
// Postman(ctx, data) call.
type PostmanImportOptions struct {
	// CollectionName overrides the name embedded in the export. Useful
	// for CI/CD scripts that want a deterministic collection name.
	CollectionName string
	// Mode == "performance" turns the collection into perf-script
	// bundles instead of API Tester requests. Empty == default.
	Mode string
	// SeedMocksNamespace pins the namespace the seeded mocks land in.
	// Defaults to the importer's auth-context namespace (typically the
	// caller's default workspace). Useful when the import lives in
	// one workspace but the resulting mocks should appear in another.
	SeedMocksNamespace string
	// SeedMocksMatchHeaders — narrow the seeded mocks' match criteria
	// to include these request-header names (case-insensitive). The
	// importer falls back to URL/method/body matching when empty.
	SeedMocksMatchHeaders []string
	// SeedMocksPriority shifts every seeded mock's Priority above the
	// default so a Postman-seeded fixture wins over wildcards. Zero
	// keeps the importer's built-in default.
	SeedMocksPriority int
	// SeedMocks turns saved Postman responses into Mockarty mocks
	// alongside the imported collection (Stream A3 dogfood path).
	SeedMocks bool
}

// PostmanWithOptions is Postman + the import knobs.
func (a *ImportAPI) PostmanWithOptions(ctx context.Context, data []byte, opts PostmanImportOptions) (*ImportResult, error) {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("mockarty: import postman: invalid collection JSON: %w", err)
	}
	body := map[string]any{"collectionJson": decoded}
	if opts.CollectionName != "" {
		body["collectionName"] = opts.CollectionName
	}
	if opts.Mode != "" {
		body["mode"] = opts.Mode
	}
	if opts.SeedMocks {
		body["seedMocks"] = true
	}
	if opts.SeedMocksNamespace != "" {
		body["seedMocksNamespace"] = opts.SeedMocksNamespace
	}
	if len(opts.SeedMocksMatchHeaders) > 0 {
		body["seedMocksMatchHeaders"] = opts.SeedMocksMatchHeaders
	}
	if opts.SeedMocksPriority != 0 {
		body["seedMocksPriority"] = opts.SeedMocksPriority
	}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/postman", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// OpenAPI imports an OpenAPI/Swagger specification into a collection.
// Server expects {content, contentType: "json"|"yaml"|"", collectionName}.
// The SDK auto-detects YAML vs JSON by the leading byte.
func (a *ImportAPI) OpenAPI(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data), ContentType: detectContentType(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/openapi", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// WSDL imports a WSDL specification into a collection.
func (a *ImportAPI) WSDL(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/wsdl", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// HAR imports an HTTP Archive (HAR) file into a collection.
func (a *ImportAPI) HAR(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/har", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GrpcProto imports a Protocol Buffers (.proto) definition into a collection.
//
// Wire shape: server's importGRPCCollection handler reads
// `protoContent` (NOT the generic `content` envelope shared by other
// importers) AND expects the .proto bytes base64-encoded. Older SDK
// builds sent raw text under `content` and every call 400'd with
// 'either serverAddress or protoContent must be provided' or 'failed
// to decode proto content'. The SDK now base64-encodes the bytes
// transparently.
func (a *ImportAPI) GrpcProto(ctx context.Context, data []byte) (*ImportResult, error) {
	body := map[string]any{
		"protoContent": base64.StdEncoding.EncodeToString(data),
	}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/grpc", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GraphQL imports a GraphQL schema into a collection.
func (a *ImportAPI) GraphQL(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/graphql", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MCP imports an MCP definition into a collection.
func (a *ImportAPI) MCP(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/mcp", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Mockarty imports a Mockarty export into a collection.
func (a *ImportAPI) Mockarty(ctx context.Context, data []byte) (*ImportResult, error) {
	body := importContentPayload{Content: string(data)}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/mockarty", &body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Curl imports a set of cURL commands into a collection. Parity with Python
// curl / Java curl.
func (a *ImportAPI) Curl(ctx context.Context, commands []string) (*ImportResult, error) {
	body := map[string][]string{"commands": commands}
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/curl", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Insomnia imports an Insomnia collection (raw export JSON) into a collection.
// Parity with Python insomnia / Java insomnia.
func (a *ImportAPI) Insomnia(ctx context.Context, data []byte) (*ImportResult, error) {
	var result ImportResult
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/import/insomnia", json.RawMessage(data), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// detectContentType returns "yaml" for likely YAML payloads, "json"
// for JSON. The server defaults to JSON when blank, so we only need to
// flag YAML explicitly. Heuristic: first non-whitespace byte is `{` or
// `[` → JSON; anything else → YAML.
func detectContentType(data []byte) string {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{', '[':
			return "json"
		default:
			return "yaml"
		}
	}
	return ""
}
