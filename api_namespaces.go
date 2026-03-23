// Copyright (c) 2024-2026 Mockarty. All rights reserved.
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
	return a.client.do(ctx, "POST", "/mock/namespace/create", &namespaceCreateRequest{Name: name}, nil)
}

// List returns all available namespaces.
func (a *NamespaceAPI) List(ctx context.Context) ([]string, error) {
	var namespaces []string
	if err := a.client.do(ctx, "GET", "/mock/namespace/list", nil, &namespaces); err != nil {
		return nil, err
	}
	return namespaces, nil
}
