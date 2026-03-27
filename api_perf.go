// Copyright (c) 2026 Mockarty. All rights reserved.
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
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name"`
	Script      string         `json:"script"`
	Duration    string         `json:"duration,omitempty"`   // e.g. "30s", "5m"
	VUs         int            `json:"vus,omitempty"`        // virtual users
	Iterations  int            `json:"iterations,omitempty"` // total iterations (alternative to duration)
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
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Duration    int64          `json:"duration"` // milliseconds
	VUs         int            `json:"vus"`
	Iterations  int            `json:"iterations"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Thresholds  map[string]any `json:"thresholds,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Errors      []PerfError    `json:"errors,omitempty"`
	StartedAt   string         `json:"startedAt"`
	CompletedAt string         `json:"completedAt,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
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
	if err := a.client.do(ctx, "POST", "/api/v1/perf/run", config, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Stop stops a running performance test.
func (a *PerfAPI) Stop(ctx context.Context, taskID string) error {
	return a.client.do(ctx, "POST", "/api/v1/perf/stop/"+url.PathEscape(taskID), nil, nil)
}

// Results returns all performance test results.
func (a *PerfAPI) Results(ctx context.Context) ([]PerfResult, error) {
	var results []PerfResult
	if err := a.client.do(ctx, "GET", "/api/v1/perf-results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetResult retrieves a single performance test result by ID.
func (a *PerfAPI) GetResult(ctx context.Context, id string) (*PerfResult, error) {
	var result PerfResult
	if err := a.client.do(ctx, "GET", "/api/v1/perf-results/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Compare retrieves a comparison between multiple performance test runs.
func (a *PerfAPI) Compare(ctx context.Context, ids []string) (*PerfComparison, error) {
	params := url.Values{}
	params.Set("ids", strings.Join(ids, ","))

	var comparison PerfComparison
	if err := a.client.do(ctx, "GET", "/api/v1/perf-results/compare?"+params.Encode(), nil, &comparison); err != nil {
		return nil, err
	}
	return &comparison, nil
}

// ---------------------------------------------------------------------------
// Schedule type
// ---------------------------------------------------------------------------

// PerfSchedule represents a scheduled performance test run.
type PerfSchedule struct {
	ID        string `json:"id,omitempty"`
	ConfigID  string `json:"configId,omitempty"`
	Cron      string `json:"cron,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Config CRUD
// ---------------------------------------------------------------------------

// ListConfigs returns all performance test configurations.
func (a *PerfAPI) ListConfigs(ctx context.Context) ([]PerfConfig, error) {
	var configs []PerfConfig
	if err := a.client.do(ctx, "GET", "/api/v1/perf-configs", nil, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// GetConfig retrieves a performance test configuration by ID.
func (a *PerfAPI) GetConfig(ctx context.Context, id string) (*PerfConfig, error) {
	var config PerfConfig
	if err := a.client.do(ctx, "GET", "/api/v1/perf-configs/"+url.PathEscape(id), nil, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateConfig creates a new performance test configuration.
func (a *PerfAPI) CreateConfig(ctx context.Context, config *PerfConfig) (*PerfConfig, error) {
	var result PerfConfig
	if err := a.client.do(ctx, "POST", "/api/v1/perf-configs", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateConfig updates a performance test configuration by ID.
func (a *PerfAPI) UpdateConfig(ctx context.Context, id string, config *PerfConfig) (*PerfConfig, error) {
	var result PerfConfig
	if err := a.client.do(ctx, "PUT", "/api/v1/perf-configs/"+url.PathEscape(id), config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteConfig deletes a performance test configuration by ID.
func (a *PerfAPI) DeleteConfig(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/perf-configs/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

// ListSchedules returns all performance test schedules.
func (a *PerfAPI) ListSchedules(ctx context.Context) ([]PerfSchedule, error) {
	var schedules []PerfSchedule
	if err := a.client.do(ctx, "GET", "/api/v1/perf-schedules", nil, &schedules); err != nil {
		return nil, err
	}
	return schedules, nil
}

// CreateSchedule creates a new performance test schedule.
func (a *PerfAPI) CreateSchedule(ctx context.Context, s *PerfSchedule) (*PerfSchedule, error) {
	var result PerfSchedule
	if err := a.client.do(ctx, "POST", "/api/v1/perf-schedules", s, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateSchedule updates a performance test schedule by ID.
func (a *PerfAPI) UpdateSchedule(ctx context.Context, id string, s *PerfSchedule) (*PerfSchedule, error) {
	var result PerfSchedule
	if err := a.client.do(ctx, "PUT", "/api/v1/perf-schedules/"+url.PathEscape(id), s, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSchedule deletes a performance test schedule by ID.
func (a *PerfAPI) DeleteSchedule(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/perf-schedules/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Result History & Trends
// ---------------------------------------------------------------------------

// GetResultHistory returns the result history for a configuration.
func (a *PerfAPI) GetResultHistory(ctx context.Context, configID string) ([]PerfResult, error) {
	var results []PerfResult
	if err := a.client.do(ctx, "GET", "/api/v1/perf-results/history/"+url.PathEscape(configID), nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetResultTrend returns the performance trend for a configuration.
func (a *PerfAPI) GetResultTrend(ctx context.Context, configID string) (map[string]any, error) {
	var trend map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/perf-results/trend/"+url.PathEscape(configID), nil, &trend); err != nil {
		return nil, err
	}
	return trend, nil
}

// DeleteResult deletes a performance test result by ID.
func (a *PerfAPI) DeleteResult(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/perf-results/"+url.PathEscape(id), nil, nil)
}

// RunCollection starts a performance test from a collection.
func (a *PerfAPI) RunCollection(ctx context.Context, req any) (*PerfTask, error) {
	var task PerfTask
	if err := a.client.do(ctx, "POST", "/api/v1/perf/run-collection", req, &task); err != nil {
		return nil, err
	}
	return &task, nil
}
