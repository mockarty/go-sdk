// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// TestRunAPI provides methods for managing test runs.
type TestRunAPI struct {
	client *Client
}

// TestRun represents a test run execution and its results.
type TestRun struct {
	ID           string `json:"id,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	Status       string `json:"status,omitempty"` // running, completed, failed, cancelled
	TotalTests   int    `json:"totalTests,omitempty"`
	PassedTests  int    `json:"passedTests,omitempty"`
	FailedTests  int    `json:"failedTests,omitempty"`
	Duration     int64  `json:"duration,omitempty"` // ms
	StartedAt    int64  `json:"startedAt,omitempty"`
	FinishedAt   int64  `json:"finishedAt,omitempty"`
	Environment  string `json:"environment,omitempty"`
}

// List returns all test runs.
func (a *TestRunAPI) List(ctx context.Context) ([]TestRun, error) {
	var runs []TestRun
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/test-runs", nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// activeTestRunsEnvelope mirrors the `{"runs": [...]}` envelope emitted by
// the /api/v1/test-runs/active endpoint used by the Runs Tray UI.
type activeTestRunsEnvelope struct {
	Runs []TestRun `json:"runs"`
}

// ListActive returns the list of pending/running test runs visible to the caller
// in the current namespace. Useful for CI/CD gating on parallel runs.
func (a *TestRunAPI) ListActive(ctx context.Context) ([]TestRun, error) {
	var envelope activeTestRunsEnvelope
	if err := a.client.do(ctx, "GET", "/api/v1/test-runs/active", nil, &envelope); err != nil {
		return nil, err
	}
	return envelope.Runs, nil
}

// Get retrieves a test run by ID.
func (a *TestRunAPI) Get(ctx context.Context, id string) (*TestRun, error) {
	var run TestRun
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/test-runs/"+url.PathEscape(id), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// Cancel cancels a running test run by ID.
func (a *TestRunAPI) Cancel(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/api-tester/test-runs/"+url.PathEscape(id)+"/cancel", nil, nil)
}

// Delete deletes a test run by ID.
func (a *TestRunAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/api-tester/test-runs/"+url.PathEscape(id), nil, nil)
}

// Export exports a test run result as raw bytes.
func (a *TestRunAPI) Export(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/api-tester/test-runs/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ImportReport imports a test report.
func (a *TestRunAPI) ImportReport(ctx context.Context, data []byte) error {
	body := struct {
		Data string `json:"data"`
	}{Data: string(data)}
	return a.client.do(ctx, "POST", "/api/v1/api-tester/reports/import", body, nil)
}
