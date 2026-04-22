// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TestPlansAPI provides methods for managing test plans — the master
// orchestrator for functional / fuzz / chaos / load / contract runs.
//
// A Test Plan bundles one or more items (each referencing a collection, fuzz
// config, chaos experiment, perf config, or contract) and runs them together
// with optional scheduling, webhooks, and a merged Allure report.
//
// See docs/research/TEST_PLANS_ARCHITECTURE_2026-04-19.md for the full spec.
type TestPlansAPI struct {
	client *Client
}

// ---------------------------------------------------------------------------
// Typed errors (in addition to the generic sentinels in errors.go)
// ---------------------------------------------------------------------------

// ErrPlanNotFound is returned when a plan lookup 404s. Alias for ErrNotFound
// to match domain-specific call sites ("plan" vs "resource").
var ErrPlanNotFound = errors.New("mockarty: test plan not found")

// ErrRunCancelled is returned from WaitForRun / StreamRun when the run has
// been cancelled by the user or by the server.
var ErrRunCancelled = errors.New("mockarty: test plan run cancelled")

// ErrRunFailed is returned from WaitForRun when the run finishes with a
// non-passing terminal status.
var ErrRunFailed = errors.New("mockarty: test plan run failed")

// ErrWebhookDeliveryFailed is returned from TestWebhook when the server
// reports that the ping failed to reach its endpoint.
var ErrWebhookDeliveryFailed = errors.New("mockarty: webhook delivery failed")

// ErrPreconditionFailed is returned by Patch when the server rejects the
// update due to an If-Match mismatch (412 Precondition Failed). Callers
// should re-fetch the plan, reconcile, and retry.
var ErrPreconditionFailed = errors.New("mockarty: precondition failed (etag mismatch)")

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// PlanItemType enumerates the item kinds a Test Plan can reference.
type PlanItemType string

const (
	PlanItemTypeFunctional PlanItemType = "functional"
	PlanItemTypeLoad       PlanItemType = "load"
	PlanItemTypeFuzz       PlanItemType = "fuzz"
	PlanItemTypeChaos      PlanItemType = "chaos"
	PlanItemTypeContract   PlanItemType = "contract"
	// PlanItemTypeTestPlan references another Test Plan by its ID — the
	// server executes the nested plan as a child run and maps its final
	// status onto this item. Cycles across plans are rejected at save-time
	// with a 400 carrying a human-readable "A → B → C → A" trace. See
	// docs/user-plans-nesting.md for the composition rules.
	PlanItemTypeTestPlan PlanItemType = "test_plan"
)

// ReportFormat enumerates the shapes the report endpoint can return.
type ReportFormat string

const (
	ReportFormatAllure  ReportFormat = "allure"
	ReportFormatJSON    ReportFormat = "json"
	ReportFormatSummary ReportFormat = "summary"
)

// ScheduleKind enumerates the firing modes of a Schedule.
type ScheduleKind string

const (
	ScheduleKindCron     ScheduleKind = "cron"
	ScheduleKindOnce     ScheduleKind = "once"
	ScheduleKindInterval ScheduleKind = "interval"
)

// ExecutionMode enumerates the typed Plan-level execution strategy. Mirrors
// the server's testplan.ExecutionMode* constants (added in migration 077).
const (
	ExecutionModeFIFO     = "fifo"
	ExecutionModeParallel = "parallel"
	ExecutionModeDAG      = "dag"
)

// TestPlanItem is a single step within a Test Plan.
//
// Fields are alignment-sorted (8-byte first) per project convention.
type TestPlanItem struct {
	// ResourceID is the UUID of the source entity — its type is determined by
	// Type (collection / fuzz_config / chaos_experiment / perf_config /
	// contract).
	ResourceID string `json:"refId"`

	// Name is an optional human-readable label shown in UI / report.
	Name string `json:"name,omitempty"`

	// Type selects the executor that handles this item.
	Type PlanItemType `json:"type"`

	// DependsOn lists other item refIds that must pass before this one
	// runs. Only honoured in DAG mode.
	DependsOn []string `json:"dependsOn,omitempty"`

	// StartOffsetMs is the delay (relative to run.startedAt) before this item
	// is dispatched. Only honoured in `timed` mode.
	StartOffsetMs int64 `json:"startOffsetMs,omitempty"`

	// DelayAfterMs is an optional cooldown applied after the item completes.
	DelayAfterMs int64 `json:"delayAfterMs,omitempty"`

	// Order is the 1-based position in the plan.
	Order int `json:"order"`
}

// TestPlan is the top-level planning artefact.
//
// ExecutionMode is the typed, post-077 successor to the legacy Schedule
// sentinel. Set it to one of "fifo" / "parallel" / "dag" — the server
// dual-writes both columns so older clients reading Schedule still see a
// consistent value. Empty defaults to "dag" when typed Gates are present
// on items, otherwise FIFO. The Schedule field remains for backward
// compatibility (cron strings + legacy parallel/dag sentinels) and will
// stay supported per the post-release contract.
type TestPlan struct {
	CreatedAt     time.Time      `json:"createdAt,omitempty"`
	UpdatedAt     time.Time      `json:"updatedAt,omitempty"`
	ClosedAt      *time.Time     `json:"closedAt,omitempty"`
	ID            string         `json:"id,omitempty"`
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Schedule      string         `json:"schedule,omitempty"`      // legacy "mode" column: "" / "parallel" / "dag" / cron
	ExecutionMode string         `json:"executionMode,omitempty"` // typed mode: "fifo" / "parallel" / "dag"
	CreatedBy     string         `json:"createdBy,omitempty"`
	Items         []TestPlanItem `json:"items"`
	NumericID     int64          `json:"numericId,omitempty"`
}

// TestPlanRun is the aggregate execution of a Plan.
type TestPlanRun struct {
	StartedAt      time.Time       `json:"startedAt,omitempty"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	ClosedAt       *time.Time      `json:"closedAt,omitempty"`
	ID             string          `json:"id,omitempty"`
	PlanID         string          `json:"planId,omitempty"`
	Namespace      string          `json:"namespace,omitempty"`
	Status         string          `json:"status,omitempty"`
	TriggeredBy    string          `json:"triggeredBy,omitempty"`
	ReportURL      string          `json:"reportUrl,omitempty"`
	ItemsState     []PlanItemState `json:"itemsState,omitempty"`
	TotalItems     int             `json:"totalItems,omitempty"`
	CompletedItems int             `json:"completedItems,omitempty"`
	FailedItems    int             `json:"failedItems,omitempty"`
}

// PlanItemState tracks the execution state of one plan item within a run.
type PlanItemState struct {
	StartedAt   *time.Time      `json:"startedAt,omitempty"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	RunID       string          `json:"runId,omitempty"`
	Type        PlanItemType    `json:"type"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	SkipReason  string          `json:"skipReason,omitempty"`
	Summary     json.RawMessage `json:"summary,omitempty"`
	Order       int             `json:"order"`
}

// PlanRunStatus is a compact status payload emitted by /status.
type PlanRunStatus struct {
	Status         string `json:"status"`
	TotalItems     int    `json:"totalItems"`
	CompletedItems int    `json:"completedItems"`
	FailedItems    int    `json:"failedItems"`
}

// Schedule represents one firing rule for a Plan.
type Schedule struct {
	CreatedAt       time.Time    `json:"createdAt,omitempty"`
	LastFiredAt     *time.Time   `json:"lastFiredAt,omitempty"`
	NextFireAt      *time.Time   `json:"nextFireAt,omitempty"`
	RunAt           *time.Time   `json:"runAt,omitempty"`
	ID              string       `json:"id,omitempty"`
	PlanID          string       `json:"planId,omitempty"`
	Kind            ScheduleKind `json:"kind"`
	CronExpr        string       `json:"cronExpr,omitempty"`
	Timezone        string       `json:"timezone,omitempty"`
	IntervalSeconds int          `json:"intervalSeconds,omitempty"`
	Enabled         bool         `json:"enabled"`
}

// Webhook represents a CI integration target for a Plan.
//
// Secret is write-only — the server never returns it on read. The SecretHash
// field mirrors the stored hash so callers can verify that a rotation
// succeeded.
type Webhook struct {
	LastCalledAt   *time.Time        `json:"lastCalledAt,omitempty"`
	CreatedAt      time.Time         `json:"createdAt,omitempty"`
	ID             string            `json:"id,omitempty"`
	PlanID         string            `json:"planId,omitempty"`
	URL            string            `json:"url"`
	Secret         string            `json:"secret,omitempty"` // write-only
	SecretHash     string            `json:"secretHash,omitempty"`
	Events         []string          `json:"events"`
	Headers        map[string]string `json:"headers,omitempty"`
	TimeoutMs      int               `json:"timeoutMs,omitempty"`
	RetryCount     int               `json:"retryCount,omitempty"`
	RetryBackoffMs int               `json:"retryBackoffMs,omitempty"`
	LastStatus     int               `json:"lastStatus,omitempty"`
	Enabled        bool              `json:"enabled"`
}

// RunEvent is a single SSE event emitted by StreamRun.
//
// Kind is the lowercase event tag (run.started / run.completed / item.started
// / item.finished / heartbeat). Raw carries the JSON payload verbatim so
// callers can decode into their own typed struct.
type RunEvent struct {
	ReceivedAt time.Time       `json:"receivedAt"`
	Kind       string          `json:"kind"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// ItemSummary is the unified report of a single item. Mirrors
// internal/testplan.ItemSummary so SDK callers can decode PlanItemState.Summary.
type ItemSummary struct {
	StartedAt   time.Time               `json:"startedAt"`
	FinishedAt  time.Time               `json:"finishedAt"`
	Steps       []ItemSummaryStep       `json:"steps,omitempty"`
	Labels      map[string]string       `json:"labels,omitempty"`
	Parameters  map[string]string       `json:"parameters,omitempty"`
	Attachments []ItemSummaryAttachment `json:"attachments,omitempty"`
	Metrics     map[string]float64      `json:"metrics,omitempty"`
	Status      string                  `json:"status"`
	DurationMs  int64                   `json:"durationMs"`
}

// ItemSummaryStep is a single step inside an ItemSummary (maps to an Allure
// step).
type ItemSummaryStep struct {
	StartedAt   time.Time               `json:"startedAt"`
	FinishedAt  time.Time               `json:"finishedAt"`
	Steps       []ItemSummaryStep       `json:"steps,omitempty"`
	Attachments []ItemSummaryAttachment `json:"attachments,omitempty"`
	Name        string                  `json:"name"`
	Status      string                  `json:"status"`
	Error       string                  `json:"error,omitempty"`
	DurationMs  int64                   `json:"durationMs"`
}

// ItemSummaryAttachment is a file attachment attached to an item or step.
type ItemSummaryAttachment struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // MIME
	Source string `json:"source"`
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

// ListPlansOptions filters a ListPlans call. All fields are optional.
type ListPlansOptions struct {
	Namespace string
	Status    string
	Limit     int
	Offset    int
}

// RunOptions configures a Run trigger.
type RunOptions struct {
	// Items optionally restricts the run to a subset of item orders (1-based).
	// nil / empty runs every item in the plan.
	Items []int `json:"items,omitempty"`
	// Mode overrides the plan-level mode for this run (sequential / parallel
	// / dag / timed). Empty string uses the plan default.
	Mode string `json:"mode,omitempty"`
}

// PatchPlanRequest is the partial-update payload for TestPlansAPI.Patch. Any
// nil pointer means "no change"; a non-nil pointer (including pointer to a
// zero value like "" or false) overwrites the server's current value.
//
// ScheduleCron is a 5-/6-field cron expression OR one of the sentinel modes
// `parallel` / `dag` — the server validates both forms via Plan.Validate.
// Prefer ExecutionMode for the typed mode field — ScheduleCron stays
// supported for cron schedules and legacy clients (post-release contract).
type PatchPlanRequest struct {
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	ScheduleCron  *string `json:"schedule_cron,omitempty"`
	ExecutionMode *string `json:"execution_mode,omitempty"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// PatchOptions tunes a TestPlansAPI.Patch call.
//
// IfMatch is the RFC 7232 strong-validator string the server will compare
// against the plan's current etag. When empty the SDK fetches the plan
// first and uses its UpdatedAt — safe default for single-writer flows but
// vulnerable to lost updates in concurrent scenarios. Always pass the
// etag returned by a prior Create/Get/Patch when you care about
// consistency.
type PatchOptions struct {
	IfMatch   string
	Namespace string // overrides the client default
}

// AdHocItem is a single step in an ad-hoc run request.
//
// The server accepts both the spec-level vocabulary (collection /
// perf_config / fuzz_config / chaos_experiment / contract_config) and the
// canonical short names (functional / load / fuzz / chaos / contract) that
// PlanItemType uses. Stringifying PlanItemType directly into Type always
// works.
type AdHocItem struct {
	RefID        string       `json:"ref_id"`
	Type         PlanItemType `json:"type"`
	DependsOn    []string     `json:"depends_on,omitempty"`
	Order        int          `json:"order"`
	DelayAfterMs int64        `json:"delay_after_ms,omitempty"`
}

// CreateAdHocRunRequest builds a POST /test-runs/ad-hoc body. Schedule
// follows the same vocabulary as TestPlan.Schedule (empty = FIFO, or
// `parallel` / `dag`, or a cron expression).
type CreateAdHocRunRequest struct {
	Namespace   string      `json:"-"` // URL segment, not body
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	Schedule    string      `json:"schedule,omitempty"`
	Items       []AdHocItem `json:"items"`
}

// AdHocRunResponse mirrors the 202 envelope returned by the server. The
// `_links` block lists canonical follow-up URLs — keep them opaque and
// treat them as hints, not a stable contract.
type AdHocRunResponse struct {
	Links  map[string]string `json:"_links,omitempty"`
	RunID  string            `json:"run_id"`
	PlanID string            `json:"plan_id"`
	Status string            `json:"status"`
	Adhoc  bool              `json:"adhoc"`
}

// AllureReport is the decoded JSON shape returned by
// TestPlansAPI.GetRunReport. The concrete schema mirrors
// internal/testplan.ItemSummary batched into a top-level envelope — the
// server keeps the payload loosely typed so the SDK uses json.RawMessage
// for forward compatibility (new fields appear without SDK bumps).
type AllureReport struct {
	Summary map[string]any    `json:"summary,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Raw     json.RawMessage   `json:"-"`
	Items   []ItemSummary     `json:"items,omitempty"`
	RunID   string            `json:"runId,omitempty"`
	PlanID  string            `json:"planId,omitempty"`
	Status  string            `json:"status,omitempty"`
}

// UnifiedReport is the strongly-typed decode target for
// TestPlansAPI.GetRunReportUnified — the native Mockarty envelope served
// from GET .../report.unified.json. Field names mirror the server-side
// unifiedReport in internal/testplan/report_formats.go; new fields on the
// server surface via UnifiedReport.Raw for forward compat.
type UnifiedReport struct {
	StartedAt     time.Time           `json:"startedAt"`
	PlanName      string              `json:"planName"`
	RunID         string              `json:"runId"`
	Results       []UnifiedItemResult `json:"results"`
	Counts        UnifiedReportCounts `json:"counts"`
	Raw           json.RawMessage     `json:"-"`
	GeneratedAtMs int64               `json:"generatedAt"`
	DurationMs    int64               `json:"durationMs"`
}

// UnifiedReportCounts tallies per-status item counts in a unified report.
type UnifiedReportCounts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Broken  int `json:"broken"`
}

// UnifiedItemResult is one result entry inside a UnifiedReport. Mirrors
// internal/testplan.AllureResult — kept as a loose struct so new upstream
// fields round-trip through UnifiedReport.Raw for callers who need them.
type UnifiedItemResult struct {
	Name          string              `json:"name"`
	UUID          string              `json:"uuid"`
	HistoryID     string              `json:"historyId"`
	FullName      string              `json:"fullName"`
	Description   string              `json:"description,omitempty"`
	Status        string              `json:"status"`
	Stage         string              `json:"stage"`
	StatusDetails map[string]any      `json:"statusDetails,omitempty"`
	Labels        []map[string]string `json:"labels,omitempty"`
	Parameters    []map[string]string `json:"parameters,omitempty"`
	Attachments   []map[string]string `json:"attachments,omitempty"`
	Start         int64               `json:"start"`
	Stop          int64               `json:"stop"`
}

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

const testPlansBase = "/api/v1/test-plans"

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

// Create creates a new test plan.
func (a *TestPlansAPI) Create(ctx context.Context, p TestPlan) (*TestPlan, error) {
	if p.Namespace == "" {
		p.Namespace = a.client.namespace
	}
	var out TestPlan
	if err := a.client.do(ctx, http.MethodPost, testPlansBase, p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a plan by UUID or numeric_id (numeric IDs may be passed with or
// without a leading '#').
func (a *TestPlansAPI) Get(ctx context.Context, idOrNumeric string) (*TestPlan, error) {
	key := strings.TrimPrefix(strings.TrimSpace(idOrNumeric), "#")
	if key == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var out TestPlan
	err := a.client.do(ctx, http.MethodGet, testPlansBase+"/"+url.PathEscape(key), nil, &out)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %s", ErrPlanNotFound, key)
		}
		return nil, err
	}
	return &out, nil
}

// Update replaces a plan in full (PUT semantics).
func (a *TestPlansAPI) Update(ctx context.Context, id string, p TestPlan) (*TestPlan, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	p.ID = id
	if p.Namespace == "" {
		p.Namespace = a.client.namespace
	}
	var out TestPlan
	if err := a.client.do(ctx, http.MethodPut, testPlansBase+"/"+url.PathEscape(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete soft-deletes a plan and its children (cascade).
func (a *TestPlansAPI) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("mockarty: empty plan id")
	}
	return a.client.do(ctx, http.MethodDelete, testPlansBase+"/"+url.PathEscape(id), nil, nil)
}

// List returns plans matching the given filters.
func (a *TestPlansAPI) List(ctx context.Context, opts ListPlansOptions) ([]TestPlan, error) {
	q := url.Values{}
	ns := opts.Namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns != "" {
		q.Set("namespace", ns)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		q.Set("limit", intToString(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", intToString(opts.Offset))
	}

	path := testPlansBase
	if s := q.Encode(); s != "" {
		path += "?" + s
	}

	// Server envelope: {"items":[...], "count": N}
	var env struct {
		Items []TestPlan `json:"items"`
		Count int        `json:"count"`
	}
	if err := a.client.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Items, nil
}

// ---------------------------------------------------------------------------
// Runs
// ---------------------------------------------------------------------------

// Run triggers a plan run. Returns the pending run record (202 Accepted).
func (a *TestPlansAPI) Run(ctx context.Context, idOrNumeric string, opts RunOptions) (*TestPlanRun, error) {
	key := strings.TrimPrefix(strings.TrimSpace(idOrNumeric), "#")
	if key == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	// Server emits {runId, planId, status}; decode into a typed run shape.
	var env struct {
		RunID  string `json:"runId"`
		PlanID string `json:"planId"`
		Status string `json:"status"`
	}
	if err := a.client.do(ctx, http.MethodPost, testPlansBase+"/"+url.PathEscape(key)+"/run", opts, &env); err != nil {
		return nil, err
	}
	return &TestPlanRun{
		ID:     env.RunID,
		PlanID: env.PlanID,
		Status: env.Status,
	}, nil
}

// GetRun fetches a run by ID.
func (a *TestPlansAPI) GetRun(ctx context.Context, runID string) (*TestPlanRun, error) {
	if runID == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	var out TestPlanRun
	if err := a.client.do(ctx, http.MethodGet, testPlansBase+"/runs/"+url.PathEscape(runID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRunStatus returns the compact status payload.
func (a *TestPlansAPI) GetRunStatus(ctx context.Context, runID string) (*PlanRunStatus, error) {
	if runID == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	var out PlanRunStatus
	if err := a.client.do(ctx, http.MethodGet, testPlansBase+"/runs/"+url.PathEscape(runID)+"/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelRun cancels a running run.
func (a *TestPlansAPI) CancelRun(ctx context.Context, runID string) error {
	if runID == "" {
		return fmt.Errorf("mockarty: empty run id")
	}
	return a.client.do(ctx, http.MethodPost, testPlansBase+"/runs/"+url.PathEscape(runID)+"/cancel", nil, nil)
}

// ListRuns returns runs for a plan.
func (a *TestPlansAPI) ListRuns(ctx context.Context, planID string, limit, offset int) ([]TestPlanRun, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", intToString(limit))
	}
	if offset > 0 {
		q.Set("offset", intToString(offset))
	}
	path := testPlansBase + "/" + url.PathEscape(planID) + "/runs"
	if s := q.Encode(); s != "" {
		path += "?" + s
	}
	var env struct {
		Items []TestPlanRun `json:"items"`
		Count int           `json:"count"`
	}
	if err := a.client.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Items, nil
}

// ---------------------------------------------------------------------------
// Compare runs (Phase-4 task #82)
// ---------------------------------------------------------------------------

// CompareItemSide mirrors the per-run snapshot the server emits.
type CompareItemSide struct {
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Status      string     `json:"status"`
	SkipReason  string     `json:"skipReason,omitempty"`
	Error       string     `json:"error,omitempty"`
	DurationMs  int64      `json:"durationMs"`
	Attempts    int        `json:"attempts"`
	Present     bool       `json:"present"`
}

// CompareItemDiff captures the per-item delta. RegressionType is one of:
// "unchanged", "pass_to_fail", "fail_to_pass", "skipped_to_ran",
// "ran_to_skipped", "fail_to_fail", "pass_to_pass", "added", "removed".
type CompareItemDiff struct {
	RegressionType   string `json:"regressionType"`
	DurationDeltaMs  int64  `json:"durationDeltaMs"`
	StatusChanged    bool   `json:"statusChanged"`
	IsRegression     bool   `json:"isRegression"`
	IsImprovement    bool   `json:"isImprovement"`
	DurationWorsened bool   `json:"durationWorsened"`
}

// CompareItem is the tuple emitted for each item that appears in either run.
type CompareItem struct {
	A       CompareItemSide `json:"a"`
	B       CompareItemSide `json:"b"`
	Diff    CompareItemDiff `json:"diff"`
	Type    string          `json:"type,omitempty"`
	Name    string          `json:"name,omitempty"`
	ItemUID string          `json:"itemUid"`
}

// CompareRunSide describes the run-level envelope on each side of the diff.
type CompareRunSide struct {
	StartedAt      time.Time  `json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	PlanName       string     `json:"planName"`
	Status         string     `json:"status"`
	Namespace      string     `json:"namespace"`
	ID             string     `json:"id"`
	PlanID         string     `json:"planId"`
	PlanNumericID  int64      `json:"planNumericId,omitempty"`
	TotalItems     int        `json:"totalItems"`
	CompletedItems int        `json:"completedItems"`
	FailedItems    int        `json:"failedItems"`
	PassedItems    int        `json:"passedItems"`
	SkippedItems   int        `json:"skippedItems"`
	DurationMs     int64      `json:"durationMs"`
}

// CompareItemRef is the compact pointer used in summary added/removed slices.
type CompareItemRef struct {
	Type    string `json:"type,omitempty"`
	Name    string `json:"name,omitempty"`
	ItemUID string `json:"itemUid"`
}

// CompareSummary aggregates the diff for fast CI consumption.
type CompareSummary struct {
	AddedItems     []CompareItemRef `json:"addedItems"`
	RemovedItems   []CompareItemRef `json:"removedItems"`
	TotalA         int              `json:"totalA"`
	TotalB         int              `json:"totalB"`
	PassToFail     int              `json:"passToFail"`
	FailToPass     int              `json:"failToPass"`
	SkippedToRan   int              `json:"skippedToRan"`
	RanToSkipped   int              `json:"ranToSkipped"`
	Regressions    int              `json:"regressions"`
	Improvements   int              `json:"improvements"`
	UnchangedItems int              `json:"unchangedItems"`
	DifferentPlans bool             `json:"differentPlans"`
}

// CompareResult is the full diff envelope the server returns.
type CompareResult struct {
	RunA    CompareRunSide `json:"runA"`
	RunB    CompareRunSide `json:"runB"`
	Items   []CompareItem  `json:"items"`
	Summary CompareSummary `json:"summary"`
}

// CompareRuns diffs two test plan runs.
//
// Both runs MUST live in the caller's namespace (the server returns 404 on
// cross-tenant probes — same no-leak semantics as GetRun). Comparing runs of
// different plans IS allowed; CompareSummary.DifferentPlans flags the case so
// callers can render a banner. Pass the older/baseline run as runA and the
// newer/target run as runB to keep regression/improvement signs intuitive.
func (a *TestPlansAPI) CompareRuns(ctx context.Context, runA, runB string) (*CompareResult, error) {
	if runA == "" || runB == "" {
		return nil, fmt.Errorf("mockarty: empty run id (run_a or run_b)")
	}
	if runA == runB {
		return nil, fmt.Errorf("mockarty: run_a and run_b must differ")
	}
	q := url.Values{}
	q.Set("run_a", runA)
	q.Set("run_b", runB)
	path := testPlansBase + "/runs/compare?" + q.Encode()
	var out CompareResult
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WaitForRun polls the run until it reaches a terminal state or ctx expires.
//
// pollInterval ≤0 falls back to 2s. Returns the final run and a typed error
// for cancelled / failed terminal states so CI pipelines can `errors.Is`.
func (a *TestPlansAPI) WaitForRun(ctx context.Context, runID string, pollInterval time.Duration) (*TestPlanRun, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	for {
		run, err := a.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		switch run.Status {
		case "completed":
			return run, nil
		case "failed":
			return run, fmt.Errorf("%w: run %s", ErrRunFailed, runID)
		case "cancelled":
			return run, fmt.Errorf("%w: run %s", ErrRunCancelled, runID)
		}
		select {
		case <-ctx.Done():
			return run, ctx.Err()
		case <-t.C:
		}
	}
}

// StreamRun subscribes to SSE events for the given run.
//
// The returned channel is closed when the stream terminates (EOF, error, or
// ctx cancellation). Callers should drain the channel in a goroutine and
// exit on close.
func (a *TestPlansAPI) StreamRun(ctx context.Context, runID string) (<-chan RunEvent, error) {
	if runID == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.client.baseURL+testPlansBase+"/runs/"+url.PathEscape(runID)+"/stream", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if a.client.apiKey != "" {
		req.Header.Set(headerAPIKey, a.client.apiKey)
	}
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mockarty: stream run: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
			RequestID:  resp.Header.Get(headerRequestID),
		}
	}

	out := make(chan RunEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		parseSSE(ctx, resp.Body, out)
	}()
	return out, nil
}

// parseSSE decodes a text/event-stream body into RunEvents.
// Compliant with the WHATWG EventSource spec: blocks separated by blank
// lines, fields `event:` and `data:` accumulate until dispatch.
func parseSSE(ctx context.Context, body io.Reader, out chan<- RunEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var event string
	var data strings.Builder

	flush := func() {
		payload := strings.TrimRight(data.String(), "\n")
		data.Reset()
		if event == "" && payload == "" {
			return
		}
		ev := RunEvent{
			ReceivedAt: time.Now().UTC(),
			Kind:       event,
		}
		if payload != "" {
			ev.Raw = json.RawMessage(payload)
		}
		event = ""
		select {
		case <-ctx.Done():
		case out <- ev:
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // comment / heartbeat
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	// Dispatch any trailing event without a final blank line.
	flush()
}

// GetReport returns the merged Allure report in the requested format.
func (a *TestPlansAPI) GetReport(ctx context.Context, runID string, format ReportFormat) ([]byte, error) {
	if runID == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	q := url.Values{}
	if format != "" {
		q.Set("format", string(format))
	}
	path := testPlansBase + "/runs/" + url.PathEscape(runID) + "/report"
	if s := q.Encode(); s != "" {
		path += "?" + s
	}
	return a.client.doJSON(ctx, http.MethodGet, path, nil)
}

// DownloadReportZip streams the Allure archive (result.json + attachments)
// into w. Caller owns w.
func (a *TestPlansAPI) DownloadReportZip(ctx context.Context, runID string, w io.Writer) error {
	if runID == "" {
		return fmt.Errorf("mockarty: empty run id")
	}
	if w == nil {
		return fmt.Errorf("mockarty: nil writer")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.client.baseURL+testPlansBase+"/runs/"+url.PathEscape(runID)+"/report.zip", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/zip")
	if a.client.apiKey != "" {
		req.Header.Set(headerAPIKey, a.client.apiKey)
	}
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mockarty: download report: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
			RequestID:  resp.Header.Get(headerRequestID),
		}
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

// AddSchedule attaches a new schedule to a plan.
func (a *TestPlansAPI) AddSchedule(ctx context.Context, planID string, s Schedule) (*Schedule, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var out Schedule
	if err := a.client.do(ctx, http.MethodPost,
		testPlansBase+"/"+url.PathEscape(planID)+"/schedules", s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSchedules lists schedules attached to a plan.
func (a *TestPlansAPI) ListSchedules(ctx context.Context, planID string) ([]Schedule, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var env struct {
		Items []Schedule `json:"items"`
	}
	path := testPlansBase + "/" + url.PathEscape(planID) + "/schedules"
	if err := a.client.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Items, nil
}

// UpdateSchedule patches an existing schedule.
func (a *TestPlansAPI) UpdateSchedule(ctx context.Context, planID, schedID string, s Schedule) (*Schedule, error) {
	if planID == "" || schedID == "" {
		return nil, fmt.Errorf("mockarty: empty plan or schedule id")
	}
	var out Schedule
	path := testPlansBase + "/" + url.PathEscape(planID) + "/schedules/" + url.PathEscape(schedID)
	if err := a.client.do(ctx, http.MethodPatch, path, s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSchedule removes a schedule from a plan.
func (a *TestPlansAPI) DeleteSchedule(ctx context.Context, planID, schedID string) error {
	if planID == "" || schedID == "" {
		return fmt.Errorf("mockarty: empty plan or schedule id")
	}
	return a.client.do(ctx, http.MethodDelete,
		testPlansBase+"/"+url.PathEscape(planID)+"/schedules/"+url.PathEscape(schedID), nil, nil)
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

// AddWebhook attaches a new webhook to a plan. The Secret field is sent once
// and never returned; the server stores a bcrypt hash.
func (a *TestPlansAPI) AddWebhook(ctx context.Context, planID string, w Webhook) (*Webhook, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var out Webhook
	if err := a.client.do(ctx, http.MethodPost,
		testPlansBase+"/"+url.PathEscape(planID)+"/webhooks", w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListWebhooks lists webhooks attached to a plan.
func (a *TestPlansAPI) ListWebhooks(ctx context.Context, planID string) ([]Webhook, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var env struct {
		Items []Webhook `json:"items"`
	}
	path := testPlansBase + "/" + url.PathEscape(planID) + "/webhooks"
	if err := a.client.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Items, nil
}

// UpdateWebhook patches an existing webhook.
func (a *TestPlansAPI) UpdateWebhook(ctx context.Context, planID, wID string, w Webhook) (*Webhook, error) {
	if planID == "" || wID == "" {
		return nil, fmt.Errorf("mockarty: empty plan or webhook id")
	}
	var out Webhook
	path := testPlansBase + "/" + url.PathEscape(planID) + "/webhooks/" + url.PathEscape(wID)
	if err := a.client.do(ctx, http.MethodPatch, path, w, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhook removes a webhook from a plan.
func (a *TestPlansAPI) DeleteWebhook(ctx context.Context, planID, wID string) error {
	if planID == "" || wID == "" {
		return fmt.Errorf("mockarty: empty plan or webhook id")
	}
	return a.client.do(ctx, http.MethodDelete,
		testPlansBase+"/"+url.PathEscape(planID)+"/webhooks/"+url.PathEscape(wID), nil, nil)
}

// TestWebhook triggers a server-side ping of the webhook.
func (a *TestPlansAPI) TestWebhook(ctx context.Context, planID, wID string) error {
	if planID == "" || wID == "" {
		return fmt.Errorf("mockarty: empty plan or webhook id")
	}
	var result struct {
		Success bool   `json:"success"`
		Status  int    `json:"status"`
		Error   string `json:"error"`
	}
	if err := a.client.do(ctx, http.MethodPost,
		testPlansBase+"/"+url.PathEscape(planID)+"/webhooks/"+url.PathEscape(wID)+"/test",
		nil, &result); err != nil {
		return err
	}
	if !result.Success {
		msg := result.Error
		if msg == "" {
			msg = fmt.Sprintf("status=%d", result.Status)
		}
		return fmt.Errorf("%w: %s", ErrWebhookDeliveryFailed, msg)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Notifications (backlog #63)
//
// A PlanNotification is a plan-scoped subscription that routes run lifecycle
// events (run_started / run_finished / run_failed / item_failed) through the
// admin's configured notification channels (Slack, Email, Telegram, Discord,
// Teams, Webex, PagerDuty, OpsGenie, Mattermost, Google Chat, generic
// webhook). It's orthogonal to Webhook (which targets arbitrary URLs with
// HMAC signatures); users can enable either or both surfaces per plan.
// ---------------------------------------------------------------------------

// PlanNotification is one row in test_plan_notifications.
//
// Recipients is optional: when empty, the server fans out to every enabled
// binding on the channel. When populated, the dispatcher sends directly to
// each recipient (e.g. Slack channel ID, email address, chat ID).
type PlanNotification struct {
	CreatedAt      time.Time `json:"createdAt,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt,omitempty"`
	ID             string    `json:"id,omitempty"`
	TestPlanID     string    `json:"testPlanId,omitempty"`
	Namespace      string    `json:"namespace,omitempty"`
	ChannelID      string    `json:"channelId"`
	Trigger        string    `json:"trigger"`
	RecipientsJSON string    `json:"recipientsJson,omitempty"`
	LastStatus     string    `json:"lastStatus,omitempty"`
	LastError      string    `json:"lastError,omitempty"`
	Recipients     []string  `json:"recipients,omitempty"`
	Enabled        bool      `json:"enabled"`
}

// CreateNotification attaches a new plan-level notification subscription.
func (a *TestPlansAPI) CreateNotification(ctx context.Context, planID string, n PlanNotification) (*PlanNotification, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	body := map[string]interface{}{
		"channelId":  n.ChannelID,
		"trigger":    n.Trigger,
		"recipients": n.Recipients,
		"enabled":    n.Enabled,
	}
	var out PlanNotification
	if err := a.client.do(ctx, http.MethodPost,
		testPlansBase+"/"+url.PathEscape(planID)+"/notifications", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListNotifications lists notifications attached to a plan.
func (a *TestPlansAPI) ListNotifications(ctx context.Context, planID string) ([]PlanNotification, error) {
	if planID == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var env struct {
		Items []PlanNotification `json:"items"`
	}
	path := testPlansBase + "/" + url.PathEscape(planID) + "/notifications"
	if err := a.client.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Items, nil
}

// GetNotification fetches a single notification by id.
func (a *TestPlansAPI) GetNotification(ctx context.Context, planID, notifID string) (*PlanNotification, error) {
	if planID == "" || notifID == "" {
		return nil, fmt.Errorf("mockarty: empty plan or notification id")
	}
	var out PlanNotification
	path := testPlansBase + "/" + url.PathEscape(planID) + "/notifications/" + url.PathEscape(notifID)
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateNotification replaces a notification's mutable fields (trigger,
// channelId, enabled, recipients). Unset fields are preserved.
func (a *TestPlansAPI) UpdateNotification(ctx context.Context, planID, notifID string, n PlanNotification) (*PlanNotification, error) {
	if planID == "" || notifID == "" {
		return nil, fmt.Errorf("mockarty: empty plan or notification id")
	}
	body := map[string]interface{}{}
	if n.ChannelID != "" {
		body["channelId"] = n.ChannelID
	}
	if n.Trigger != "" {
		body["trigger"] = n.Trigger
	}
	if n.Recipients != nil {
		body["recipients"] = n.Recipients
	}
	// Always include enabled so callers can flip the flag explicitly. Omitting
	// it would make disabling impossible through this helper.
	body["enabled"] = n.Enabled

	var out PlanNotification
	path := testPlansBase + "/" + url.PathEscape(planID) + "/notifications/" + url.PathEscape(notifID)
	if err := a.client.do(ctx, http.MethodPut, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteNotification soft-deletes a plan notification subscription.
func (a *TestPlansAPI) DeleteNotification(ctx context.Context, planID, notifID string) error {
	if planID == "" || notifID == "" {
		return fmt.Errorf("mockarty: empty plan or notification id")
	}
	return a.client.do(ctx, http.MethodDelete,
		testPlansBase+"/"+url.PathEscape(planID)+"/notifications/"+url.PathEscape(notifID),
		nil, nil)
}

// ---------------------------------------------------------------------------
// TP-6b: PATCH + ad-hoc runs + namespace-scoped reports
// ---------------------------------------------------------------------------

// namespaceScopedBase builds `/api/v1/namespaces/<ns>` for the TP-6 / TP-6b
// endpoints that require namespace isolation. Falls back to the client
// default namespace when explicit is empty.
func (a *TestPlansAPI) namespaceScopedBase(explicit string) string {
	ns := explicit
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		ns = defaultNamespace
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns)
}

// planRefEscape trims the optional `#` numeric-id prefix and URL-encodes
// the remainder. The server accepts either a bare UUID or a positive
// integer; we don't care which here.
func planRefEscape(planRef string) (string, error) {
	key := strings.TrimPrefix(strings.TrimSpace(planRef), "#")
	if key == "" {
		return "", fmt.Errorf("mockarty: empty plan id")
	}
	return url.PathEscape(key), nil
}

// Patch applies a partial update to a plan. Uses the
// PATCH /api/v1/namespaces/:namespace/test-plans/:idOrNumericID endpoint
// and requires an If-Match header. When opts.IfMatch is empty the SDK
// performs a pre-fetch to obtain the current etag (convenient for
// one-shot CLI tooling; unsafe for concurrent writers — pass an explicit
// etag in that case).
//
// Returns ErrPreconditionFailed on 412 so callers can errors.Is and loop
// with a fresh fetch.
func (a *TestPlansAPI) Patch(ctx context.Context, planRef string, req PatchPlanRequest, opts PatchOptions) (*TestPlan, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if req.Name == nil && req.Description == nil && req.ScheduleCron == nil &&
		req.ExecutionMode == nil && req.Enabled == nil {
		return nil, fmt.Errorf("mockarty: Patch requires at least one field")
	}

	ifMatch := strings.TrimSpace(opts.IfMatch)
	if ifMatch == "" {
		// Pre-fetch to recover the current updated_at. Use the namespace-
		// scoped Get path implicitly via the legacy Get — it returns the
		// same plan doc.
		current, gerr := a.Get(ctx, planRef)
		if gerr != nil {
			return nil, fmt.Errorf("mockarty: Patch pre-fetch: %w", gerr)
		}
		ifMatch = fmt.Sprintf(`"%d"`, current.UpdatedAt.UnixMilli())
	}

	path := a.namespaceScopedBase(opts.Namespace) + "/test-plans/" + key

	httpReq, herr := a.newJSONRequest(ctx, http.MethodPatch, path, req)
	if herr != nil {
		return nil, herr
	}
	httpReq.Header.Set("If-Match", ifMatch)

	resp, derr := a.client.httpClient.Do(httpReq)
	if derr != nil {
		return nil, fmt.Errorf("mockarty: patch plan: %w", derr)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", ErrPreconditionFailed, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
			RequestID:  resp.Header.Get(headerRequestID),
		}
		var envelope struct {
			Error     string `json:"error"`
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		}
		if json.Unmarshal(body, &envelope) == nil {
			if envelope.Error != "" {
				apiErr.Message = envelope.Error
			}
			if envelope.Code != "" {
				apiErr.Code = envelope.Code
			}
			if envelope.RequestID != "" {
				apiErr.RequestID = envelope.RequestID
			}
		}
		return nil, apiErr
	}

	var out TestPlan
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("mockarty: patch plan decode: %w", err)
	}
	return &out, nil
}

// CreateAdHocRun creates a hidden ("ad-hoc") Plan row and dispatches a
// master run through the orchestrator in a single call. The server
// returns 202 with {run_id, plan_id, status}; downstream polling uses
// GetRun / WaitForRun / StreamRun against the returned RunID.
//
// The orchestrator must be wired on the admin node — single-binary / SQLite
// deployments without it return 503 (ErrUnavailable).
func (a *TestPlansAPI) CreateAdHocRun(ctx context.Context, req CreateAdHocRunRequest) (*AdHocRunResponse, error) {
	if len(req.Items) == 0 {
		return nil, fmt.Errorf("mockarty: CreateAdHocRun requires at least one item")
	}
	for i, it := range req.Items {
		if strings.TrimSpace(it.RefID) == "" {
			return nil, fmt.Errorf("mockarty: CreateAdHocRun items[%d].ref_id is required", i)
		}
		if strings.TrimSpace(string(it.Type)) == "" {
			return nil, fmt.Errorf("mockarty: CreateAdHocRun items[%d].type is required", i)
		}
	}
	path := a.namespaceScopedBase(req.Namespace) + "/test-runs/ad-hoc"
	var out AdHocRunResponse
	if err := a.client.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRunReport fetches the namespace-scoped Allure JSON report for a run.
// Uses GET /api/v1/namespaces/:ns/test-plans/:planRef/runs/:runID/report.
//
// The raw bytes are preserved in AllureReport.Raw so callers can run a
// second decode pass with their own types — the server's schema is
// loosely typed on purpose (new Allure fields roll out server-side
// without SDK bumps).
func (a *TestPlansAPI) GetRunReport(ctx context.Context, namespace, planRef, runID string) (*AllureReport, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report"
	raw, err := a.client.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	rep := &AllureReport{Raw: json.RawMessage(raw)}
	// Decode best-effort — unknown top-level shapes still yield a non-nil
	// Raw that callers can reparse. Don't fail the whole call on a
	// rename.
	_ = json.Unmarshal(raw, rep)
	return rep, nil
}

// GetRunReportZIP streams the Allure ZIP archive for a run. Caller owns the
// returned reader and MUST close it. Use io.Copy to persist to disk.
//
// Uses GET /api/v1/namespaces/:ns/test-plans/:planRef/runs/:runID/report.zip.
func (a *TestPlansAPI) GetRunReportZIP(ctx context.Context, namespace, planRef, runID string) (io.ReadCloser, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report.zip"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.client.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zip")
	if a.client.apiKey != "" {
		req.Header.Set(headerAPIKey, a.client.apiKey)
	}
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mockarty: get run report zip: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
			RequestID:  resp.Header.Get(headerRequestID),
		}
	}
	return resp.Body, nil
}

// GetRunReportJUnit fetches the JUnit XML report for a namespace-scoped
// run via GET .../report.junit.xml. The bytes are returned verbatim so CI
// integrations can feed them straight into Jenkins / GitLab / any JUnit
// consumer. Use namespace="" to fall back to the client default.
func (a *TestPlansAPI) GetRunReportJUnit(ctx context.Context, namespace, planRef, runID string) ([]byte, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report.junit.xml"
	return a.fetchReportBytes(ctx, path, "application/xml")
}

// GetRunReportMarkdown fetches the Markdown summary report for a
// namespace-scoped run via GET .../report.md. Intended for Slack
// attachments, email bodies, and wiki pastes. Use namespace="" to fall
// back to the client default.
func (a *TestPlansAPI) GetRunReportMarkdown(ctx context.Context, namespace, planRef, runID string) ([]byte, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report.md"
	return a.fetchReportBytes(ctx, path, "text/markdown")
}

// GetRunReportUnified fetches the native Mockarty-shape JSON report
// (no Allure translation) via GET .../report.unified.json and decodes it
// into UnifiedReport. The raw bytes are preserved on UnifiedReport.Raw so
// callers can re-parse with domain-specific types if the server adds new
// fields later.
func (a *TestPlansAPI) GetRunReportUnified(ctx context.Context, namespace, planRef, runID string) (*UnifiedReport, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report.unified.json"
	raw, err := a.fetchReportBytes(ctx, path, "application/json")
	if err != nil {
		return nil, err
	}
	rep := &UnifiedReport{Raw: json.RawMessage(raw)}
	if err := json.Unmarshal(raw, rep); err != nil {
		// Best-effort: unknown top-level shape still yields a non-nil Raw
		// so callers can fall back to manual decoding.
		return rep, fmt.Errorf("mockarty: decode unified report: %w", err)
	}
	return rep, nil
}

// GetRunReportHTML fetches the standalone, print-friendly HTML report for
// a namespace-scoped run via GET .../report.html. The response is a
// self-contained HTML document (inlined CSS, no external assets) that
// users can open in any browser and export to PDF via Save-as-PDF.
// Intended for air-gapped deployments and shareable run artifacts. Use
// namespace="" to fall back to the client default.
func (a *TestPlansAPI) GetRunReportHTML(ctx context.Context, namespace, planRef, runID string) ([]byte, error) {
	key, err := planRefEscape(planRef)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/test-plans/" + key +
		"/runs/" + url.PathEscape(runID) +
		"/report.html"
	return a.fetchReportBytes(ctx, path, "text/html")
}

// fetchReportBytes is a shared helper for the non-Allure report endpoints
// that return bytes verbatim. Mirrors doJSON's retry/error semantics while
// letting callers specify the Accept header so log-aggregators can see the
// requested format.
func (a *TestPlansAPI) fetchReportBytes(ctx context.Context, path, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.client.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if a.client.apiKey != "" {
		req.Header.Set(headerAPIKey, a.client.apiKey)
	}
	resp, err := a.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mockarty: fetch report: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("mockarty: read report: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(body)),
			RequestID:  resp.Header.Get(headerRequestID),
		}
	}
	return body, nil
}

// newJSONRequest is a small internal helper for endpoints that need
// access to the full http.Request (to attach extra headers like
// If-Match). Keeps the retry loop in Client.doRaw untouched for the
// common case.
func (a *TestPlansAPI) newJSONRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("mockarty: marshal request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.client.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("mockarty: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if a.client.apiKey != "" {
		req.Header.Set(headerAPIKey, a.client.apiKey)
	}
	return req, nil
}
