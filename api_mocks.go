// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MockAPI provides methods to manage mocks.
type MockAPI struct {
	client *Client
}

// ListMocksOptions configures the List request.
type ListMocksOptions struct {
	Namespace string
	Tags      []string
	Search    string
	Offset    int
	Limit     int
}

// LogsOptions configures the Logs request.
type LogsOptions struct {
	Limit  int
	Offset int
}

// Create creates a new mock or overwrites an existing one with the same ID.
func (a *MockAPI) Create(ctx context.Context, mock *Mock) (*SaveMockResponse, error) {
	if mock.Namespace == "" && a.client.namespace != "" {
		mock.Namespace = a.client.namespace
	}

	var resp SaveMockResponse
	if err := a.client.do(ctx, "POST", "/mock/create", mock, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Get retrieves a mock by ID.
func (a *MockAPI) Get(ctx context.Context, id string) (*Mock, error) {
	var mock Mock
	if err := a.client.do(ctx, "GET", "/mock/get/"+url.PathEscape(id), nil, &mock); err != nil {
		return nil, err
	}
	return &mock, nil
}

// List retrieves a list of mocks filtered by the given options.
func (a *MockAPI) List(ctx context.Context, opts *ListMocksOptions) (*MockListResponse, error) {
	params := url.Values{}

	if opts != nil {
		if opts.Namespace != "" {
			params.Set("namespace", opts.Namespace)
		}
		if len(opts.Tags) > 0 {
			params.Set("tags", strings.Join(opts.Tags, ","))
		}
		if opts.Search != "" {
			params.Set("search", opts.Search)
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
	} else {
		// Use default namespace
		if a.client.namespace != "" {
			params.Set("namespace", a.client.namespace)
		}
	}

	path := "/mock/list"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp MockListResponse
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Update updates a mock by ID. The mock.ID field is used if id is empty.
func (a *MockAPI) Update(ctx context.Context, id string, mock *Mock) (*Mock, error) {
	if id != "" {
		mock.ID = id
	}
	resp, err := a.Create(ctx, mock)
	if err != nil {
		return nil, err
	}
	return &resp.Mock, nil
}

// Delete soft-deletes a mock by ID.
func (a *MockAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/mock/delete/"+url.PathEscape(id), nil, nil)
}

// Restore restores a soft-deleted mock by ID.
func (a *MockAPI) Restore(ctx context.Context, id string) error {
	return a.client.do(ctx, "GET", "/mock/restore/"+url.PathEscape(id), nil, nil)
}

// Purge permanently deletes a mock by ID.
func (a *MockAPI) Purge(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/mock/purge/"+url.PathEscape(id), nil, nil)
}

// BatchCreate creates multiple mocks in one call.
func (a *MockAPI) BatchCreate(ctx context.Context, mocks []*Mock) error {
	for i, m := range mocks {
		if m.Namespace == "" && a.client.namespace != "" {
			mocks[i].Namespace = a.client.namespace
		}
	}

	for _, m := range mocks {
		if _, err := a.Create(ctx, m); err != nil {
			return fmt.Errorf("mockarty: batch create mock %q: %w", m.ID, err)
		}
	}
	return nil
}

// BatchDelete soft-deletes multiple mocks by their IDs.
func (a *MockAPI) BatchDelete(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := a.Delete(ctx, id); err != nil {
			return fmt.Errorf("mockarty: batch delete mock %q: %w", id, err)
		}
	}
	return nil
}

// BatchRestore restores multiple soft-deleted mocks.
func (a *MockAPI) BatchRestore(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := a.Restore(ctx, id); err != nil {
			return fmt.Errorf("mockarty: batch restore mock %q: %w", id, err)
		}
	}
	return nil
}

// Logs retrieves request logs for a mock.
func (a *MockAPI) Logs(ctx context.Context, id string, opts *LogsOptions) (*MockLogs, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
	}

	path := "/mock/logs/" + url.PathEscape(id)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var logs MockLogs
	if err := a.client.do(ctx, "GET", path, nil, &logs); err != nil {
		return nil, err
	}
	return &logs, nil
}

// Versions retrieves the chain mocks (related versions) for a given chain ID.
func (a *MockAPI) Versions(ctx context.Context, chainID string) ([]*Mock, error) {
	var mocks []*Mock
	if err := a.client.do(ctx, "GET", "/mock/chain/get/"+url.PathEscape(chainID), nil, &mocks); err != nil {
		return nil, err
	}
	return mocks, nil
}
