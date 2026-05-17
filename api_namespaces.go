// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
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

// List returns all available namespaces.
//
// The admin server returns the list inside an envelope:
//
//	{"namespaces": ["sandbox", ...]}
//
// We decode the envelope and surface the bare slice so callers don't
// have to know about the wire shape.
func (a *NamespaceAPI) List(ctx context.Context) ([]string, error) {
	var resp struct {
		Namespaces []string `json:"namespaces"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Namespaces, nil
}
