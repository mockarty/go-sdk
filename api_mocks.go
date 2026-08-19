// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
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

// CreateOption tunes a Create call. Use CreateNew or Overwrite to resolve a
// duplicate-entity (HTTP 409) conflict when a similar mock already exists
// (same endpoint, possibly different conditions).
type CreateOption func(*createOptions)

type createOptions struct {
	intent string
}

// CreateNew forces creating a brand-new mock even when a similar one exists,
// instead of getting a 409 duplicate_entity conflict. Use it to seed several
// condition-differentiated mocks on the same route/method.
func CreateNew() CreateOption { return func(o *createOptions) { o.intent = "create_new" } }

// Overwrite replaces the existing duplicate in place (reusing its id) instead
// of getting a 409. The server still guards against a concurrent modification.
func Overwrite() CreateOption { return func(o *createOptions) { o.intent = "overwrite" } }

// Create creates a new mock or overwrites an existing one with the same ID.
// When a similar mock already exists the server returns 409 duplicate_entity;
// pass CreateNew() or Overwrite() to resolve the conflict.
func (a *MockAPI) Create(ctx context.Context, mock *Mock, opts ...CreateOption) (*SaveMockResponse, error) {
	if mock.Namespace == "" && a.client.namespace != "" {
		mock.Namespace = a.client.namespace
	}

	var co createOptions
	for _, o := range opts {
		o(&co)
	}
	path := "/api/v1/mocks"
	if co.intent != "" {
		path += "?intent=" + url.QueryEscape(co.intent)
	}

	var resp SaveMockResponse
	if err := a.client.do(ctx, "POST", path, mock, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Get retrieves a mock by ID.
func (a *MockAPI) Get(ctx context.Context, id string) (*Mock, error) {
	var mock Mock
	if err := a.client.do(ctx, "GET", "/api/v1/mocks/"+url.PathEscape(id), nil, &mock); err != nil {
		return nil, err
	}
	return &mock, nil
}

// List retrieves a list of mocks filtered by the given options.
func (a *MockAPI) List(ctx context.Context, opts *ListMocksOptions) (*MockListResponse, error) {
	params := url.Values{}

	// Default page size matches the Python/Java SDKs (Page.limit = 50) so a
	// bare List(ctx, nil) returns an identical first page across all three
	// SDKs. Callers page further via opts.Offset / opts.Limit.
	namespace := a.client.namespace
	offset, limit := 0, 50
	if opts != nil {
		if opts.Namespace != "" {
			namespace = opts.Namespace
		}
		if len(opts.Tags) > 0 {
			params.Set("tags", strings.Join(opts.Tags, ","))
		}
		if opts.Search != "" {
			params.Set("search", opts.Search)
		}
		if opts.Offset > 0 {
			offset = opts.Offset
		}
		if opts.Limit > 0 {
			limit = opts.Limit
		}
	}
	if namespace != "" {
		params.Set("namespace", namespace)
	}
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))

	path := "/api/v1/mocks"
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
	return a.client.do(ctx, "DELETE", "/api/v1/mocks/"+url.PathEscape(id), nil, nil)
}

// GetChain returns all mocks in a chain by chain ID. 3-language parity: Python
// mocks.get_chain / Java mocks.getChain — the Go SDK was missing chain ops.
func (a *MockAPI) GetChain(ctx context.Context, chainID string) ([]*Mock, error) {
	var mocks []*Mock
	if err := a.client.do(ctx, "GET", "/api/v1/mocks/chains/"+url.PathEscape(chainID), nil, &mocks); err != nil {
		return nil, err
	}
	return mocks, nil
}

// DeleteChain deletes every mock in a chain by chain ID. Parity with Python
// mocks.delete_chain / Java mocks.deleteChain.
func (a *MockAPI) DeleteChain(ctx context.Context, chainID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/mocks/chains/"+url.PathEscape(chainID), nil, nil)
}

// Purge permanently deletes a mock by ID (bypasses the recycle bin). Parity
// with Python mocks.purge / Java mocks.purge.
func (a *MockAPI) Purge(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/mocks/"+url.PathEscape(id)+"/purge", nil, nil)
}

// Restore restores a soft-deleted mock by ID (uses batch/restore endpoint).
//
// Wire shape: server's batch endpoints read the ID list from the
// `mockIds` field, NOT `ids`. The older SDK shape sent `ids` and
// every Restore call 400'd with 'at least one mock ID is required'.
func (a *MockAPI) Restore(ctx context.Context, id string) error {
	body := struct {
		MockIds []string `json:"mockIds"`
	}{MockIds: []string{id}}
	return a.client.do(ctx, "POST", "/api/v1/mocks/batch/restore", body, nil)
}

// BatchCreate creates multiple mocks in one call.
//
// Wire shape: server expects `{"mocks": [...]}` envelope, NOT a bare
// array. Old SDK sent the array and got 400 'mocks field must be an
// array' (server tried to bind into a map and failed).
func (a *MockAPI) BatchCreate(ctx context.Context, mocks []*Mock) error {
	for i, m := range mocks {
		if m.Namespace == "" && a.client.namespace != "" {
			mocks[i].Namespace = a.client.namespace
		}
	}
	body := map[string]any{"mocks": mocks}
	return a.client.do(ctx, "POST", "/api/v1/mocks/batch", body, nil)
}

// BatchDelete soft-deletes multiple mocks by their IDs.
//
// Wire shape: server reads `mockIds` (not `ids`).
func (a *MockAPI) BatchDelete(ctx context.Context, ids []string) error {
	body := struct {
		MockIds []string `json:"mockIds"`
	}{MockIds: ids}
	return a.client.do(ctx, "DELETE", "/api/v1/mocks/batch", body, nil)
}

// BatchRestore restores multiple soft-deleted mocks.
//
// Wire shape: server reads `mockIds` (not `ids`).
func (a *MockAPI) BatchRestore(ctx context.Context, ids []string) error {
	body := struct {
		MockIds []string `json:"mockIds"`
	}{MockIds: ids}
	return a.client.do(ctx, "POST", "/api/v1/mocks/batch/restore", body, nil)
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

	path := "/api/v1/mocks/" + url.PathEscape(id) + "/logs"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var logs MockLogs
	if err := a.client.do(ctx, "GET", path, nil, &logs); err != nil {
		return nil, err
	}
	return &logs, nil
}

// ListVersions returns the version history for a mock.
//
// Wire shape: server returns `{mock_id, versions: [...], count}`.
// The SDK unwraps the envelope and returns the slice.
func (a *MockAPI) ListVersions(ctx context.Context, id string) ([]*Mock, error) {
	var env struct {
		Versions []*Mock `json:"versions"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/mocks/"+url.PathEscape(id)+"/versions", nil, &env); err != nil {
		return nil, err
	}
	return env.Versions, nil
}

// GetVersion returns a specific version of a mock.
func (a *MockAPI) GetVersion(ctx context.Context, id, version string) (*Mock, error) {
	var mock Mock
	path := "/api/v1/mocks/" + url.PathEscape(id) + "/versions/" + url.PathEscape(version)
	if err := a.client.do(ctx, "GET", path, nil, &mock); err != nil {
		return nil, err
	}
	return &mock, nil
}

// RestoreVersion restores a specific version of a mock.
func (a *MockAPI) RestoreVersion(ctx context.Context, id, version string) error {
	path := "/api/v1/mocks/" + url.PathEscape(id) + "/versions/" + url.PathEscape(version) + "/restore"
	return a.client.do(ctx, "POST", path, nil, nil)
}

// Patch partially updates a mock by ID.
func (a *MockAPI) Patch(ctx context.Context, id string, patch map[string]any) (*Mock, error) {
	var resp SaveMockResponse
	if err := a.client.do(ctx, "PATCH", "/api/v1/mocks/"+url.PathEscape(id), patch, &resp); err != nil {
		return nil, err
	}
	return &resp.Mock, nil
}

// DeleteLogs deletes request logs for a mock.
func (a *MockAPI) DeleteLogs(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/mocks/"+url.PathEscape(id)+"/logs", nil, nil)
}

// CopyToNamespace copies mocks to another namespace.
//
// Wire shape: server reads `mockIds` + `targetNamespace`.
func (a *MockAPI) CopyToNamespace(ctx context.Context, ids []string, targetNamespace string) error {
	body := struct {
		MockIds         []string `json:"mockIds"`
		TargetNamespace string   `json:"targetNamespace"`
	}{MockIds: ids, TargetNamespace: targetNamespace}
	return a.client.do(ctx, "POST", "/api/v1/mocks/copy-to-namespace", body, nil)
}

// MoveToFolder moves mocks to a folder.
//
// Wire shape: server's moveMocksToFolder handler reads `mockIds`
// + `folderId`. The earlier SDK sent `ids` and every call 400'd
// with 'no mock IDs provided'.
func (a *MockAPI) MoveToFolder(ctx context.Context, ids []string, folderID string) error {
	body := struct {
		MockIds  []string `json:"mockIds"`
		FolderID string   `json:"folderId"`
	}{MockIds: ids, FolderID: folderID}
	return a.client.do(ctx, "PATCH", "/api/v1/mocks/batch/move", body, nil)
}

// BatchUpdateTags add+remove tags on multiple mocks in one call.
//
// Wire shape: server expects `{mockIds, tagsToAdd, tagsToRemove}`.
// The single `tags` parameter is treated as "tags to add"; removing
// tags requires constructing the body manually via BatchTagsRequest
// (exported for advanced callers — see comment).
func (a *MockAPI) BatchUpdateTags(ctx context.Context, ids []string, tags []string) error {
	body := struct {
		MockIds      []string `json:"mockIds"`
		TagsToAdd    []string `json:"tagsToAdd"`
		TagsToRemove []string `json:"tagsToRemove"`
	}{MockIds: ids, TagsToAdd: tags}
	return a.client.do(ctx, "PATCH", "/api/v1/mocks/batch/tags", body, nil)
}
