// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"testing"
)

// CreateMockT creates a mock and registers a cleanup function that deletes
// it when the test completes. This is the recommended way to create mocks
// in tests.
func (c *Client) CreateMockT(t *testing.T, mock *Mock) *Mock {
	t.Helper()

	resp, err := c.Mocks().Create(context.Background(), mock)
	if err != nil {
		t.Fatalf("mockarty: create mock: %v", err)
	}

	t.Cleanup(func() {
		_ = c.Mocks().Delete(context.Background(), resp.Mock.ID)
	})

	return &resp.Mock
}

// SetupNamespaceT creates a namespace (if it doesn't already exist) and
// registers a cleanup function that removes all mocks in the namespace
// when the test completes.
func (c *Client) SetupNamespaceT(t *testing.T, namespace string) {
	t.Helper()

	// Create namespace; ignore error if it already exists.
	_ = c.Namespaces().Create(context.Background(), namespace)

	t.Cleanup(func() {
		// List all mocks in the namespace and delete them.
		list, err := c.Mocks().List(context.Background(), &ListMocksOptions{
			Namespace: namespace,
			Limit:     1000,
		})
		if err != nil {
			return
		}
		for _, m := range list.Items {
			_ = c.Mocks().Delete(context.Background(), m.ID)
		}
	})
}

// MustCreateMock is a convenience that creates a mock from a builder,
// registers cleanup, and returns the created mock. Fails the test on error.
func (c *Client) MustCreateMock(t *testing.T, builder *MockBuilder) *Mock {
	t.Helper()
	return c.CreateMockT(t, builder.Build())
}
