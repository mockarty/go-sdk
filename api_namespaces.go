// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
)

// NamespaceAPI provides methods for namespace management.
type NamespaceAPI struct {
	client *Client
}

// namespaceCreateRequest mirrors the server's NamespaceCreateRequest.
type namespaceCreateRequest struct {
	Name string `json:"name"`
}

// Create creates a new namespace.
func (a *NamespaceAPI) Create(ctx context.Context, name string) error {
	return a.client.do(ctx, "POST", "/api/v1/namespaces", &namespaceCreateRequest{Name: name}, nil)
}

// CopyMocks was removed — it POSTed to a non-existent
// /api/v1/namespaces/copy-mocks route (404). Use Mocks().CopyToNamespace(ids,
// target) for the real, server-backed mock-copy operation.

// List returns all available namespaces.
//
// The admin server returns the list inside an envelope:
//
//	{"namespaces": ["sandbox", ...]}
//
// We decode the envelope and surface the bare slice so callers don't
// have to know about the wire shape. A bare JSON array shape (older
// admin builds, mock test servers) is also accepted as a fallback so
// the SDK keeps working against either wire form.
func (a *NamespaceAPI) List(ctx context.Context) ([]string, error) {
	var raw json.RawMessage
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces", nil, &raw); err != nil {
		return nil, err
	}
	var envelope struct {
		Namespaces []string `json:"namespaces"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Namespaces != nil {
		return envelope.Namespaces, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, &unmarshalError{shape: "namespaces list", raw: raw}
}

// unmarshalError carries the raw payload so debugging is easier when
// the server emits an unexpected shape.
type unmarshalError struct {
	shape string
	raw   []byte
}

func (e *unmarshalError) Error() string {
	if len(e.raw) > 200 {
		return "mockarty: cannot decode " + e.shape + " (truncated): " + string(e.raw[:200]) + "..."
	}
	return "mockarty: cannot decode " + e.shape + ": " + string(e.raw)
}
