// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
	"strconv"
)

// FuzzingAPI provides methods for managing fuzzing tests.
type FuzzingAPI struct {
	client *Client
}

// FuzzingConfig defines the configuration for a fuzzing run.
type FuzzingConfig struct {
	ID                string   `json:"id,omitempty"`
	Name              string   `json:"name,omitempty"`
	Namespace         string   `json:"namespace,omitempty"`
	TargetBaseURL     string   `json:"targetBaseUrl,omitempty"`
	SourceType        string   `json:"sourceType,omitempty"`
	Strategy          string   `json:"strategy,omitempty"`
	PayloadCategories []string `json:"payloadCategories,omitempty"`
	SeedRequests      any      `json:"seedRequests,omitempty"`
	Options           any      `json:"options,omitempty"`
	CreatedAt         string   `json:"createdAt,omitempty"`
	UpdatedAt         string   `json:"updatedAt,omitempty"`
}

// FuzzingRun represents a started fuzzing run.
type FuzzingRun struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

// FuzzingResult holds the results of a completed or in-progress fuzzing run.
type FuzzingResult struct {
	ID               string `json:"id,omitempty"`
	ConfigID         string `json:"configId,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	Status           string `json:"status,omitempty"`
	Strategy         string `json:"strategy,omitempty"`
	TotalRequests    int64  `json:"totalRequests,omitempty"`
	TotalFindings    int    `json:"totalFindings,omitempty"`
	CriticalFindings int    `json:"criticalFindings,omitempty"`
	HighFindings     int    `json:"highFindings,omitempty"`
	MediumFindings   int    `json:"mediumFindings,omitempty"`
	LowFindings      int    `json:"lowFindings,omitempty"`
	InfoFindings     int    `json:"infoFindings,omitempty"`
	StartedAt        string `json:"startedAt,omitempty"`
	CompletedAt      string `json:"completedAt,omitempty"`
	DurationMs       int64  `json:"durationMs,omitempty"`
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

// ListConfigs returns all fuzzing configurations.
func (a *FuzzingAPI) ListConfigs(ctx context.Context) ([]FuzzingConfig, error) {
	var configs []FuzzingConfig
	if err := a.client.do(ctx, "GET", "/api/v1/fuzzing/configs", nil, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// UpdateConfig updates a fuzzing configuration by ID.
func (a *FuzzingAPI) UpdateConfig(ctx context.Context, id string, config *FuzzingConfig) (*FuzzingConfig, error) {
	var result FuzzingConfig
	if err := a.client.do(ctx, "PUT", "/api/v1/fuzzing/configs/"+url.PathEscape(id), config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteConfig deletes a fuzzing configuration by ID.
func (a *FuzzingAPI) DeleteConfig(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/fuzzing/configs/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Finding types
// ---------------------------------------------------------------------------

// FuzzingFinding represents a security finding from a fuzzing run.
type FuzzingFinding struct {
	ID            string `json:"id,omitempty"`
	RunID         string `json:"runId,omitempty"`
	Severity      string `json:"severity,omitempty"`
	Category      string `json:"category,omitempty"`
	Title         string `json:"title,omitempty"`
	Description   string `json:"description,omitempty"`
	RequestMethod string `json:"requestMethod,omitempty"`
	RequestURL    string `json:"requestUrl,omitempty"`
	ResponseStatus int   `json:"responseStatus,omitempty"`
	TriagedStatus string `json:"triagedStatus,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
}

// FuzzingSchedule represents a scheduled fuzzing run.
type FuzzingSchedule struct {
	ID              string `json:"id,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	ConfigID        string `json:"configId,omitempty"`
	Name            string `json:"name,omitempty"`
	CronExpression  string `json:"cronExpression,omitempty"`
	Enabled         bool   `json:"enabled"`
	NotifyOnFailure bool   `json:"notifyOnFailure,omitempty"`
	NextRunAt       string `json:"nextRunAt,omitempty"`
	LastRunAt       string `json:"lastRunAt,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
	UpdatedAt       string `json:"updatedAt,omitempty"`
}

// QuarantineEntry represents a quarantine rule for fuzzing findings.
type QuarantineEntry struct {
	ID              string `json:"id,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Fingerprint     string `json:"fingerprint,omitempty"`
	Category        string `json:"category,omitempty"`
	EndpointPattern string `json:"endpointPattern,omitempty"`
	Title           string `json:"title,omitempty"`
	Reason          string `json:"reason,omitempty"`
	SourceFindingID string `json:"sourceFindingId,omitempty"`
	CreatedBy       string `json:"createdBy,omitempty"`
	CreatedAt       string `json:"createdAt,omitempty"`
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
func (a *FuzzingAPI) ImportFromCurl(ctx context.Context, curl string) error {
	body := struct {
		Curl string `json:"curl"`
	}{Curl: curl}
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

// ---------------------------------------------------------------------------
// Batch Finding Operations
// ---------------------------------------------------------------------------

// BatchManualTriageFuzzFindings updates the triage status and optional note for multiple findings.
func (a *FuzzingAPI) BatchManualTriageFuzzFindings(ctx context.Context, ids []string, status string, note *string) (int, error) {
	body := struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
		Note   *string  `json:"note,omitempty"`
	}{IDs: ids, Status: status, Note: note}
	var resp struct {
		Updated int `json:"updated"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/findings/batch-manual-triage", body, &resp); err != nil {
		return 0, err
	}
	return resp.Updated, nil
}

// BatchDeleteFuzzFindings deletes multiple fuzzing findings by IDs.
func (a *FuzzingAPI) BatchDeleteFuzzFindings(ctx context.Context, ids []string) (int, error) {
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	var resp struct {
		Deleted int `json:"deleted"`
	}
	if err := a.client.do(ctx, "DELETE", "/api/v1/fuzzing/findings/batch", body, &resp); err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// ---------------------------------------------------------------------------
// Quarantine
// ---------------------------------------------------------------------------

// ListFuzzQuarantine returns quarantine entries with pagination.
func (a *FuzzingAPI) ListFuzzQuarantine(ctx context.Context, limit, offset int) ([]QuarantineEntry, int, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/api/v1/fuzzing/quarantine"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp struct {
		Entries []QuarantineEntry `json:"entries"`
		Total   int               `json:"total"`
	}
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Entries, resp.Total, nil
}

// CreateFuzzQuarantine creates a new quarantine entry.
func (a *FuzzingAPI) CreateFuzzQuarantine(ctx context.Context, entry *QuarantineEntry) (*QuarantineEntry, error) {
	var result QuarantineEntry
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/quarantine", entry, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteFuzzQuarantine deletes a quarantine entry by ID.
func (a *FuzzingAPI) DeleteFuzzQuarantine(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/fuzzing/quarantine/"+url.PathEscape(id), nil, nil)
}

// BatchDeleteFuzzQuarantine deletes multiple quarantine entries by IDs.
func (a *FuzzingAPI) BatchDeleteFuzzQuarantine(ctx context.Context, ids []string) (int, error) {
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	var resp struct {
		Deleted int `json:"deleted"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/quarantine/batch-delete", body, &resp); err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// QuarantineFuzzFinding creates a quarantine entry from a finding.
func (a *FuzzingAPI) QuarantineFuzzFinding(ctx context.Context, findingID string, reason string) (*QuarantineEntry, error) {
	body := struct {
		FindingID string `json:"findingId"`
		Reason    string `json:"reason"`
	}{FindingID: findingID, Reason: reason}
	var result QuarantineEntry
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/quarantine/from-finding", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchQuarantineFuzzFindings creates quarantine entries from multiple findings.
func (a *FuzzingAPI) BatchQuarantineFuzzFindings(ctx context.Context, findingIDs []string, reason string) (created, triaged, failed int, err error) {
	body := struct {
		FindingIDs []string `json:"findingIds"`
		Reason     string   `json:"reason"`
	}{FindingIDs: findingIDs, Reason: reason}
	var resp struct {
		Created int `json:"created"`
		Triaged int `json:"triaged"`
		Failed  int `json:"failed"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/fuzzing/quarantine/from-findings", body, &resp); err != nil {
		return 0, 0, 0, err
	}
	return resp.Created, resp.Triaged, resp.Failed, nil
}
