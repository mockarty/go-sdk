// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// FuzzingAPI provides methods for managing fuzzing tests.
type FuzzingAPI struct {
	client *Client
}

// FuzzingConfig defines the configuration for a fuzzing run.
type FuzzingConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
	SpecURL   string `json:"specUrl,omitempty"`
	Spec      string `json:"spec,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Workers   int    `json:"workers,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// FuzzingRun represents a started fuzzing run.
type FuzzingRun struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

// FuzzingResult holds the results of a completed or in-progress fuzzing run.
type FuzzingResult struct {
	ID            string `json:"id,omitempty"`
	ConfigID      string `json:"configId,omitempty"`
	Status        string `json:"status,omitempty"`
	StartedAt     int64  `json:"startedAt,omitempty"`
	FinishedAt    int64  `json:"finishedAt,omitempty"`
	TotalRequests int    `json:"totalRequests,omitempty"`
	Findings      int    `json:"findings,omitempty"`
}

// Start starts a new fuzzing run with the given configuration.
func (a *FuzzingAPI) Start(ctx context.Context, config *FuzzingConfig) (*FuzzingRun, error) {
	if config.Namespace == "" && a.client.namespace != "" {
		config.Namespace = a.client.namespace
	}
	var run FuzzingRun
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/run", config, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// Stop stops a running fuzzing run by ID.
func (a *FuzzingAPI) Stop(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/run/"+url.PathEscape(id)+"/stop", nil, nil)
}

// GetResult retrieves the result of a fuzzing run by ID.
func (a *FuzzingAPI) GetResult(ctx context.Context, id string) (*FuzzingResult, error) {
	var result FuzzingResult
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/results/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListResults returns all fuzzing results.
func (a *FuzzingAPI) ListResults(ctx context.Context) ([]FuzzingResult, error) {
	var results []FuzzingResult
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteResult deletes a fuzzing result by ID.
func (a *FuzzingAPI) DeleteResult(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/fuzzing/results/"+url.PathEscape(id), nil, nil)
}

// CreateConfig creates a new fuzzing configuration.
func (a *FuzzingAPI) CreateConfig(ctx context.Context, config *FuzzingConfig) (*FuzzingConfig, error) {
	if config.Namespace == "" && a.client.namespace != "" {
		config.Namespace = a.client.namespace
	}
	var result FuzzingConfig
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/configs", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConfig retrieves a fuzzing configuration by ID.
func (a *FuzzingAPI) GetConfig(ctx context.Context, id string) (*FuzzingConfig, error) {
	var config FuzzingConfig
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/configs/"+url.PathEscape(id), nil, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ---------------------------------------------------------------------------
// Finding types
// ---------------------------------------------------------------------------

// FuzzingFinding represents a security finding from a fuzzing run.
type FuzzingFinding struct {
	ID           string `json:"id,omitempty"`
	RunID        string `json:"runId,omitempty"`
	Type         string `json:"type,omitempty"`
	Severity     string `json:"severity,omitempty"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	TriageStatus string `json:"triageStatus,omitempty"`
}

// FuzzingSchedule represents a scheduled fuzzing run.
type FuzzingSchedule struct {
	ID        string `json:"id,omitempty"`
	ConfigID  string `json:"configId,omitempty"`
	Cron      string `json:"cron,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Summary & Quick Fuzz
// ---------------------------------------------------------------------------

// GetSummary returns a summary of all fuzzing activity.
func (a *FuzzingAPI) GetSummary(ctx context.Context) (map[string]any, error) {
	var summary map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/summary", nil, &summary); err != nil {
		return nil, err
	}
	return summary, nil
}

// QuickFuzz starts a quick fuzzing run with minimal configuration.
func (a *FuzzingAPI) QuickFuzz(ctx context.Context, req any) (*FuzzingRun, error) {
	var run FuzzingRun
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/quick-fuzz", req, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

// ListFindings returns all fuzzing findings.
func (a *FuzzingAPI) ListFindings(ctx context.Context) ([]FuzzingFinding, error) {
	var findings []FuzzingFinding
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/findings", nil, &findings); err != nil {
		return nil, err
	}
	return findings, nil
}

// GetFinding retrieves a fuzzing finding by ID.
func (a *FuzzingAPI) GetFinding(ctx context.Context, id string) (*FuzzingFinding, error) {
	var finding FuzzingFinding
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/findings/"+url.PathEscape(id), nil, &finding); err != nil {
		return nil, err
	}
	return &finding, nil
}

// TriageFinding updates the triage status and notes of a finding.
func (a *FuzzingAPI) TriageFinding(ctx context.Context, id string, status string, notes string) error {
	body := struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}{Status: status, Notes: notes}
	return a.client.do(ctx, "PUT", "/api/v1/fuzzing/findings/"+url.PathEscape(id)+"/triage", body, nil)
}

// ReplayFinding replays the request that caused a finding.
func (a *FuzzingAPI) ReplayFinding(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/findings/"+url.PathEscape(id)+"/replay", nil, nil)
}

// AnalyzeFinding runs AI analysis on a finding.
func (a *FuzzingAPI) AnalyzeFinding(ctx context.Context, id string) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/findings/"+url.PathEscape(id)+"/analyze", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// BatchAnalyzeFindings runs AI analysis on multiple findings.
func (a *FuzzingAPI) BatchAnalyzeFindings(ctx context.Context, ids []string) error {
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/findings/batch-analyze", body, nil)
}

// BatchTriageFindings updates the triage status for multiple findings.
func (a *FuzzingAPI) BatchTriageFindings(ctx context.Context, ids []string, status string) error {
	body := struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}{IDs: ids, Status: status}
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/findings/batch-triage", body, nil)
}

// ExportFindings exports findings as raw bytes.
func (a *FuzzingAPI) ExportFindings(ctx context.Context, req any) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "POST", "/api/v1/fuzzing/findings/export", req)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// ImportFromCurl imports a fuzzing target from a cURL command.
func (a *FuzzingAPI) ImportFromCurl(ctx context.Context, data string) error {
	body := struct {
		Data string `json:"data"`
	}{Data: data}
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/import/curl", body, nil)
}

// ImportFromOpenAPI imports fuzzing targets from an OpenAPI specification.
func (a *FuzzingAPI) ImportFromOpenAPI(ctx context.Context, data any) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/import/openapi", data, nil)
}

// ImportFromCollection imports fuzzing targets from a collection.
func (a *FuzzingAPI) ImportFromCollection(ctx context.Context, data any) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/import/collection", data, nil)
}

// ImportFromRecorder imports fuzzing targets from a recorder session.
func (a *FuzzingAPI) ImportFromRecorder(ctx context.Context, data any) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/import/recorder", data, nil)
}

// ImportFromMock imports fuzzing targets from a mock definition.
func (a *FuzzingAPI) ImportFromMock(ctx context.Context, data any) error {
	return a.client.do(ctx, "POST", "/api/v1/fuzzing/import/mock", data, nil)
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

// ListSchedules returns all fuzzing schedules.
func (a *FuzzingAPI) ListSchedules(ctx context.Context) ([]FuzzingSchedule, error) {
	var schedules []FuzzingSchedule
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/schedules", nil, &schedules); err != nil {
		return nil, err
	}
	return schedules, nil
}

// GetSchedule retrieves a fuzzing schedule by ID.
func (a *FuzzingAPI) GetSchedule(ctx context.Context, id string) (*FuzzingSchedule, error) {
	var schedule FuzzingSchedule
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/schedules/"+url.PathEscape(id), nil, &schedule); err != nil {
		return nil, err
	}
	return &schedule, nil
}

// CreateSchedule creates a new fuzzing schedule.
func (a *FuzzingAPI) CreateSchedule(ctx context.Context, schedule *FuzzingSchedule) (*FuzzingSchedule, error) {
	var result FuzzingSchedule
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/schedules", schedule, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateSchedule updates a fuzzing schedule by ID.
func (a *FuzzingAPI) UpdateSchedule(ctx context.Context, id string, schedule *FuzzingSchedule) (*FuzzingSchedule, error) {
	var result FuzzingSchedule
	if err := a.client.do(ctx, "PUT", "/api/v1/fuzzing/schedules/"+url.PathEscape(id), schedule, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSchedule deletes a fuzzing schedule by ID.
func (a *FuzzingAPI) DeleteSchedule(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/fuzzing/schedules/"+url.PathEscape(id), nil, nil)
}
