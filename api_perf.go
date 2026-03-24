// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
	"strings"
)

// PerfAPI provides methods for managing performance tests.
type PerfAPI struct {
	client *Client
}

// PerfConfig defines the configuration for a performance test run.
type PerfConfig struct {
	Name        string         `json:"name"`
	Script      string         `json:"script"`
	Duration    string         `json:"duration,omitempty"`    // e.g. "30s", "5m"
	VUs         int            `json:"vus,omitempty"`         // virtual users
	Iterations  int            `json:"iterations,omitempty"`  // total iterations (alternative to duration)
	Thresholds  map[string]any `json:"thresholds,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Environment map[string]any `json:"environment,omitempty"` // environment variables for script
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// PerfTask represents a running or completed performance test.
type PerfTask struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"` // "queued", "running", "completed", "failed", "stopped"
	StartedAt string `json:"startedAt,omitempty"`
}

// PerfResult holds the results of a completed performance test.
type PerfResult struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Duration    int64             `json:"duration"` // milliseconds
	VUs         int               `json:"vus"`
	Iterations  int               `json:"iterations"`
	Metrics     map[string]any    `json:"metrics,omitempty"`
	Thresholds  map[string]any    `json:"thresholds,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Errors      []PerfError       `json:"errors,omitempty"`
	StartedAt   string            `json:"startedAt"`
	CompletedAt string            `json:"completedAt,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// PerfError represents an error encountered during a performance test.
type PerfError struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// PerfComparison holds the comparison between multiple performance runs.
type PerfComparison struct {
	Results    []PerfResult   `json:"results"`
	Comparison map[string]any `json:"comparison,omitempty"`
}

// Run starts a new performance test.
func (a *PerfAPI) Run(ctx context.Context, config *PerfConfig) (*PerfTask, error) {
	var task PerfTask
	if err := a.client.do(ctx, "POST", "/ui/api/perf/run", config, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Stop stops a running performance test.
func (a *PerfAPI) Stop(ctx context.Context, taskID string) error {
	return a.client.do(ctx, "POST", "/ui/api/perf/stop/"+url.PathEscape(taskID), nil, nil)
}

// Results returns all performance test results.
func (a *PerfAPI) Results(ctx context.Context) ([]PerfResult, error) {
	var results []PerfResult
	if err := a.client.do(ctx, "GET", "/ui/api/perf-results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetResult retrieves a single performance test result by ID.
func (a *PerfAPI) GetResult(ctx context.Context, id string) (*PerfResult, error) {
	var result PerfResult
	if err := a.client.do(ctx, "GET", "/ui/api/perf-results/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Compare retrieves a comparison between multiple performance test runs.
func (a *PerfAPI) Compare(ctx context.Context, ids []string) (*PerfComparison, error) {
	params := url.Values{}
	params.Set("ids", strings.Join(ids, ","))

	var comparison PerfComparison
	if err := a.client.do(ctx, "GET", "/ui/api/perf-results/compare?"+params.Encode(), nil, &comparison); err != nil {
		return nil, err
	}
	return &comparison, nil
}
