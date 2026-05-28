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
//
// Wire shape mirrors the server's `runner.PerfTestResult`. Two ID
// fields: `ID` is the result row's own UUID, `TaskID` is the runner
// task that produced it — use TaskID to correlate with the value
// returned by Run() / RunWithOptions().
type PerfResult struct {
	ID               string         `json:"id"`
	TaskID           string         `json:"taskId,omitempty"`
	ConfigID         string         `json:"configId,omitempty"`
	Namespace        string         `json:"namespace,omitempty"`
	Name             string         `json:"name,omitempty"`
	Status           string         `json:"status"`
	DurationMs       int64          `json:"durationMs"`
	TotalRequests    int64          `json:"totalRequests"`
	FailedRequests   int64          `json:"failedRequests"`
	TotalVUs         int            `json:"totalVUs"`
	StartedAt        string         `json:"startedAt"`
	CompletedAt      string         `json:"completedAt,omitempty"`
	ThresholdsPassed *bool          `json:"thresholdsPassed,omitempty"`
	ThresholdsData   map[string]any `json:"thresholdsData,omitempty"`
	MetricsData      map[string]any `json:"metricsData,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Errors           []PerfError    `json:"errors,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	StoppedReason    string         `json:"stoppedReason,omitempty"`
	// Deprecated aliases for back-compat with the previous shape. Kept
	// so older callers don't break, but new code should use the snake-
	// case-matching DurationMs / TotalVUs / TotalRequests above.
	Duration   int64          `json:"-"`
	VUs        int            `json:"-"`
	Iterations int            `json:"-"`
	Metrics    map[string]any `json:"-"`
	Thresholds map[string]any `json:"-"`
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
//
// Wire shape: the server replies with `{"taskId": "<uuid>"}` — NOT a
// full PerfTask struct — so we decode the envelope and project the id
// onto our richer PerfTask shape. Status is populated by the caller
// via a follow-up GET (currently raw HTTP; see Results() for the
// SDK-native poll path).
func (a *PerfAPI) Run(ctx context.Context, config *PerfConfig) (*PerfTask, error) {
	var env struct {
		TaskID string `json:"taskId"`
		// Forward-compatible: if the server starts returning the full
		// PerfTask envelope (id/name/status/startedAt), prefer those.
		ID        string `json:"id,omitempty"`
		Name      string `json:"name,omitempty"`
		Status    string `json:"status,omitempty"`
		StartedAt string `json:"startedAt,omitempty"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/perf/run", config, &env); err != nil {
		return nil, err
	}
	id := env.TaskID
	if id == "" {
		id = env.ID
	}
	return &PerfTask{
		ID:        id,
		Name:      env.Name,
		Status:    env.Status,
		StartedAt: env.StartedAt,
	}, nil
}

// PerfRunRequest is the inbound shape for RunWithOptions — kept distinct
// from PerfConfig so the launch's per-call routing knobs (RunnerID,
// CITriggerID, labels) don't pollute the persistent config model.
type PerfRunRequest struct {
	ConfigID string `json:"configId,omitempty"`
	Script   string `json:"script,omitempty"`
	// RunnerID — pin to a specific runner ID, "local" to force local,
	// or empty for auto-assign.
	RunnerID string `json:"runnerId,omitempty"`
	// RequiredRunnerLabels — Phase 3 label DSL (AND'd with the expr).
	RequiredRunnerLabels []string `json:"requiredRunnerLabels,omitempty"`
	// RunnerLabelExpr — Phase 3 label DSL expression.
	RunnerLabelExpr string `json:"runnerLabelExpr,omitempty"`
	// CITriggerID — Phase 4 CI Triggers. When set, the server fires the
	// trigger to mint a dispatch_token before queuing the task; the
	// ephemeral runner the CI pipeline spawns claims by token.
	CITriggerID string `json:"ciTriggerId,omitempty"`
	IsDebug     bool   `json:"isDebug,omitempty"`
}

// RunWithOptions launches a perf test with per-call routing options.
// Useful for CI/CD scripts that want to dispatch through a CI trigger:
//
//	task, err := client.Perf().RunWithOptions(ctx, PerfRunRequest{
//	    ConfigID:    "my-config",
//	    CITriggerID: "my-trigger",
//	})
func (a *PerfAPI) RunWithOptions(ctx context.Context, req PerfRunRequest) (*PerfTask, error) {
	// Same wire envelope as Run() — server returns {"taskId": "..."}.
	var env struct {
		TaskID    string `json:"taskId"`
		ID        string `json:"id,omitempty"`
		Name      string `json:"name,omitempty"`
		Status    string `json:"status,omitempty"`
		StartedAt string `json:"startedAt,omitempty"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/perf/run", req, &env); err != nil {
		return nil, err
	}
	id := env.TaskID
	if id == "" {
		id = env.ID
	}
	return &PerfTask{
		ID:        id,
		Name:      env.Name,
		Status:    env.Status,
		StartedAt: env.StartedAt,
	}, nil
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
