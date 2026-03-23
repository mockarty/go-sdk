// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// CollectionAPI provides methods for managing API Tester collections.
type CollectionAPI struct {
	client *Client
}

// Collection represents an API tester collection.
type Collection struct {
	ID             string         `json:"id"`
	Namespace      string         `json:"namespace"`
	UserID         string         `json:"userId"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Protocol       string         `json:"protocol"`
	CollectionType string         `json:"collectionType"`
	IsShared       bool           `json:"isShared"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

// TestRunResult represents the result of executing a test collection.
type TestRunResult struct {
	ID           string         `json:"id"`
	CollectionID string         `json:"collectionId"`
	Status       string         `json:"status"` // "running", "completed", "failed"
	TotalTests   int            `json:"totalTests"`
	PassedTests  int            `json:"passedTests"`
	FailedTests  int            `json:"failedTests"`
	SkippedTests int            `json:"skippedTests"`
	Duration     int64          `json:"duration"` // milliseconds
	Results      []TestResult   `json:"results,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	StartedAt    string         `json:"startedAt"`
	CompletedAt  string         `json:"completedAt,omitempty"`
}

// TestResult represents the result of a single test step.
type TestResult struct {
	ID          string         `json:"id"`
	RequestName string         `json:"requestName"`
	Status      string         `json:"status"` // "passed", "failed", "skipped", "error"
	Duration    int64          `json:"duration"`
	Assertions  []Assertion    `json:"assertions,omitempty"`
	Error       string         `json:"error,omitempty"`
	Request     map[string]any `json:"request,omitempty"`
	Response    map[string]any `json:"response,omitempty"`
}

// Assertion represents a single test assertion.
type Assertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message,omitempty"`
}

// List returns all collections accessible in the configured namespace.
func (a *CollectionAPI) List(ctx context.Context) ([]Collection, error) {
	params := url.Values{}
	if a.client.namespace != "" {
		params.Set("namespace", a.client.namespace)
	}

	path := "/ui/api/api-tester/collections"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var collections []Collection
	if err := a.client.do(ctx, "GET", path, nil, &collections); err != nil {
		return nil, err
	}
	return collections, nil
}

// Get retrieves a collection by ID.
func (a *CollectionAPI) Get(ctx context.Context, id string) (*Collection, error) {
	var col Collection
	if err := a.client.do(ctx, "GET", "/ui/api/api-tester/collections/"+url.PathEscape(id), nil, &col); err != nil {
		return nil, err
	}
	return &col, nil
}

// Execute runs a test collection and returns the results.
func (a *CollectionAPI) Execute(ctx context.Context, id string) (*TestRunResult, error) {
	var result TestRunResult
	if err := a.client.do(ctx, "POST", "/ui/api/api-tester/collections/"+url.PathEscape(id)+"/run", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExecuteMultiple runs multiple collections and returns the combined results.
func (a *CollectionAPI) ExecuteMultiple(ctx context.Context, ids []string) (*TestRunResult, error) {
	body := struct {
		CollectionIDs []string `json:"collectionIds"`
	}{CollectionIDs: ids}

	var result TestRunResult
	if err := a.client.do(ctx, "POST", "/ui/api/api-tester/collections/run-multiple", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Export exports a collection as JSON bytes (Postman-compatible format).
func (a *CollectionAPI) Export(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/ui/api/api-tester/collections/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Create creates a new collection.
func (a *CollectionAPI) Create(ctx context.Context, col *Collection) (*Collection, error) {
	if col.Namespace == "" && a.client.namespace != "" {
		col.Namespace = a.client.namespace
	}
	var result Collection
	if err := a.client.do(ctx, "POST", "/ui/api/api-tester/collections", col, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update updates an existing collection by ID.
func (a *CollectionAPI) Update(ctx context.Context, id string, col *Collection) (*Collection, error) {
	var result Collection
	if err := a.client.do(ctx, "PUT", "/ui/api/api-tester/collections/"+url.PathEscape(id), col, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes a collection by ID.
func (a *CollectionAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/ui/api/api-tester/collections/"+url.PathEscape(id), nil, nil)
}

// Duplicate duplicates a collection by ID.
func (a *CollectionAPI) Duplicate(ctx context.Context, id string) (*Collection, error) {
	var result Collection
	if err := a.client.do(ctx, "POST", "/ui/api/api-tester/collections/"+url.PathEscape(id)+"/duplicate", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchDelete deletes multiple collections by their IDs.
func (a *CollectionAPI) BatchDelete(ctx context.Context, ids []string) error {
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	return a.client.do(ctx, "POST", "/ui/api/api-tester/collections/batch-delete", body, nil)
}
