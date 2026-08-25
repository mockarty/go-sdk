// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

// PerfAPI provides methods for managing performance tests.
type PerfAPI struct {
	client *Client
}

// PerfConfig defines the configuration for a performance test run.
type PerfConfig struct {
	Options      *PerfOptions               `json:"options,omitempty"`
	ParentID     *string                    `json:"parentId,omitempty"`
	CreatedAt    *time.Time                 `json:"createdAt,omitempty"`
	UpdatedAt    *time.Time                 `json:"updatedAt,omitempty"`
	Environment  map[string]any             `json:"environment,omitempty"` // environment variables for script
	Thresholds   map[string]any             `json:"thresholds,omitempty"`
	Metadata     map[string]any             `json:"metadata,omitempty"`
	Extra        map[string]json.RawMessage `json:"-"`
	Tags         []string                   `json:"tags,omitempty"`
	CollectionID string                     `json:"collectionId,omitempty"`
	Namespace    string                     `json:"namespace,omitempty"`
	UserID       string                     `json:"userId,omitempty"`
	ID           string                     `json:"id,omitempty"`
	Name         string                     `json:"name"`
	Script       string                     `json:"script"`
	Duration     string                     `json:"duration,omitempty"` // legacy inline-run field, e.g. "30s", "5m"
	SortOrder    int                        `json:"sortOrder"`
	VUs          int                        `json:"vus,omitempty"`        // legacy inline-run field: virtual users
	Iterations   int                        `json:"iterations,omitempty"` // legacy inline-run field
	IsFolder     bool                       `json:"isFolder"`
}

// PerfOptions is the typed `options` envelope of a saved performance
// configuration. It mirrors the server's runner.PerfOptions wire contract.
// The legacy flattened PerfConfig fields remain available for inline runs;
// use Options when saving a reusable configuration with CreateConfig.
type PerfOptions struct {
	Thresholds          map[string][]string        `json:"thresholds,omitempty"`
	Stages              []PerfStage                `json:"stages,omitempty"`
	AbortCriteria       []AbortCriterion           `json:"abortCriteria,omitempty"`
	MetricsPush         []string                   `json:"metricsPush,omitempty"`
	Extra               map[string]json.RawMessage `json:"-"`
	Duration            string                     `json:"duration,omitempty"`
	GracefulStop        string                     `json:"gracefulStop,omitempty"`
	GracefulRampDown    string                     `json:"gracefulRampDown,omitempty"`
	MetricsPushInterval string                     `json:"metricsPushInterval,omitempty"`
	StartAtUnixMs       int64                      `json:"startAtUnixMs,omitempty"`
	VUs                 int                        `json:"vus,omitempty"`
	Iterations          int                        `json:"iterations,omitempty"`
	RPS                 int                        `json:"rps,omitempty"`
	MaxVUs              int                        `json:"maxVUs,omitempty"`
	ArrivalRate         bool                       `json:"arrivalRate,omitempty"`
	EmitHistograms      bool                       `json:"emitHistograms,omitempty"`
}

// PerfStage describes one virtual-user or arrival-rate ramp stage.
type PerfStage struct {
	Extra     map[string]json.RawMessage `json:"-"`
	Duration  string                     `json:"duration"`
	Target    int                        `json:"target"`
	TargetRPS int                        `json:"targetRPS,omitempty"`
}

// AbortCriterion describes one automatic performance-test stop condition.
type AbortCriterion struct {
	Extra     map[string]json.RawMessage `json:"-"`
	Metric    string                     `json:"metric"`
	Stat      string                     `json:"stat"`
	Condition string                     `json:"condition"`
	Duration  string                     `json:"duration,omitempty"`
	Name      string                     `json:"name,omitempty"`
	Value     float64                    `json:"value"`
	Enabled   bool                       `json:"enabled"`
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
	// RequiredRunnerLabels — runner-label DSL (AND'd with the expr).
	RequiredRunnerLabels []string `json:"requiredRunnerLabels,omitempty"`
	// RunnerLabelExpr — runner-label DSL expression.
	RunnerLabelExpr string `json:"runnerLabelExpr,omitempty"`
	// CITriggerID — CI Triggers. When set, the server fires the
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
func (a *PerfAPI) ListResults(ctx context.Context) ([]PerfResult, error) {
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

// PerfSchedule represents a scheduled performance test run. Field names and
// JSON tags mirror the server's runner.PerfSchedule contract: the cron field is
// `cronExpression` (not `cron`) and timestamps are RFC3339 (time.Time, not the
// legacy int64 — the server emits time.Time, so an int64 field fails to decode).
type PerfSchedule struct {
	ID             string     `json:"id,omitempty"`
	ConfigID       string     `json:"configId,omitempty"`
	Name           string     `json:"name,omitempty"`
	Namespace      string     `json:"namespace,omitempty"`
	CronExpression string     `json:"cronExpression,omitempty"`
	Enabled        bool       `json:"enabled"`
	CreatedAt      time.Time  `json:"createdAt,omitempty"`
	UpdatedAt      time.Time  `json:"updatedAt,omitempty"`
	NextRunAt      *time.Time `json:"nextRunAt,omitempty"`
	LastRunAt      *time.Time `json:"lastRunAt,omitempty"`
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
//
// Server returns a bare JSON list; when the repo finds zero rows it
// emits JSON null, which Go decodes into a nil slice. Coerce nil →
// empty slice so callers can iterate without nil-guards.
func (a *PerfAPI) ListSchedules(ctx context.Context) ([]PerfSchedule, error) {
	var schedules []PerfSchedule
	if err := a.client.do(ctx, "GET", "/api/v1/perf-schedules", nil, &schedules); err != nil {
		return nil, err
	}
	if schedules == nil {
		return []PerfSchedule{}, nil
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

// PerfRunGroup is the result of running a whole collection as a perf suite.
// A collection fans out into one perf task per request, so the server returns
// a run-group id plus the per-request task ids — NOT a single PerfTask.
type PerfRunGroup struct {
	RunGroupID string   `json:"runGroupId"`
	TaskIDs    []string `json:"taskIds"`
}

// RunCollection starts a performance suite from a collection — every request
// in the collection becomes its own perf task. Returns the run-group id (to
// correlate results via GET /perf-results/group/:runGroupId) and the spawned
// task ids.
//
// POST /api/v1/perf/run-collection
func (a *PerfAPI) RunCollection(ctx context.Context, req any) (*PerfRunGroup, error) {
	var group PerfRunGroup
	if err := a.client.do(ctx, "POST", "/api/v1/perf/run-collection", req, &group); err != nil {
		return nil, err
	}
	return &group, nil
}
