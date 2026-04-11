// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// RecorderAPI provides methods for managing traffic recording sessions.
type RecorderAPI struct {
	client *Client
}

// RecorderSession represents a recording session configuration and status.
type RecorderSession struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	TargetURL  string `json:"targetUrl,omitempty"`
	Status     string `json:"status,omitempty"` // idle, recording, stopped
	Namespace  string `json:"namespace,omitempty"`
	CreatedAt  int64  `json:"createdAt,omitempty"`
	EntryCount int    `json:"entryCount,omitempty"`
}

// RecorderEntry represents a single recorded request/response pair.
type RecorderEntry struct {
	ID         string `json:"id,omitempty"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Duration   int64  `json:"duration,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

// StartRecording creates and starts a new recording session.
func (a *RecorderAPI) StartRecording(ctx context.Context, session *RecorderSession) (*RecorderSession, error) {
	if session.Namespace == "" && a.client.namespace != "" {
		session.Namespace = a.client.namespace
	}
	var result RecorderSession
	if err := a.client.do(ctx, "POST", "/api/v1/recorder/start", session, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSession retrieves a recording session by ID.
func (a *RecorderAPI) GetSession(ctx context.Context, id string) (*RecorderSession, error) {
	var session RecorderSession
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/"+url.PathEscape(id), nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListSessions returns all recording sessions.
func (a *RecorderAPI) ListSessions(ctx context.Context) ([]RecorderSession, error) {
	var sessions []RecorderSession
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// StopRecording stops recording on a session.
func (a *RecorderAPI) StopRecording(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(id)+"/stop", nil, nil)
}

// RestartRecording restarts recording on a session.
func (a *RecorderAPI) RestartRecording(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(id)+"/restart", nil, nil)
}

// DeleteSession deletes a recording session by ID.
func (a *RecorderAPI) DeleteSession(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/recorder/"+url.PathEscape(id), nil, nil)
}

// GetEntries retrieves all recorded entries for a session.
func (a *RecorderAPI) GetEntries(ctx context.Context, sessionID string) ([]RecorderEntry, error) {
	var entries []RecorderEntry
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/entries", nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// CreateMocksFromSession creates mocks from all recorded entries in a session.
func (a *RecorderAPI) CreateMocksFromSession(ctx context.Context, sessionID string, req any) ([]Mock, error) {
	var mocks []Mock
	if err := a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/mocks", req, &mocks); err != nil {
		return nil, err
	}
	return mocks, nil
}

// ExportSession exports a recording session as raw bytes (HAR format).
func (a *RecorderAPI) ExportSession(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Session-level Replay
// ---------------------------------------------------------------------------

// ReplayOptions configures a session-level replay run.
//
// All fields are optional. When TargetURL is empty, the recorder replays
// against each captured entry's original URL. EntryIDs filters down to a
// subset; an empty slice replays everything in the session.
type ReplayOptions struct {
	TargetURL       string   `json:"targetUrl,omitempty"`
	EntryIDs        []string `json:"entryIds,omitempty"`
	Concurrency     int      `json:"concurrency,omitempty"`     // default 1
	TimeoutMs       int      `json:"timeoutMs,omitempty"`       // per request
	IncludeNonHTTP  bool     `json:"includeNonHttp,omitempty"`  // include WS/SSE entries (debug)
	FollowRedirects bool     `json:"followRedirects,omitempty"` // default false
}

// ReplayResult is the per-entry outcome of a replay run.
type ReplayResult struct {
	EntryID         string `json:"entryId"`
	OriginalStatus  int    `json:"originalStatus"`
	NewStatus       int    `json:"newStatus"`
	StatusMatch     bool   `json:"statusMatch"`
	DurationMs      int64  `json:"durationMs"`
	ReplayedURL     string `json:"replayedUrl"`
	ResponsePreview string `json:"responsePreview,omitempty"`
	Error           string `json:"error,omitempty"`
	Skipped         bool   `json:"skipped,omitempty"`
	SkippedReason   string `json:"skippedReason,omitempty"`
}

// ReplaySummary aggregates the outcome of a replay run.
type ReplaySummary struct {
	SessionID    string         `json:"sessionId"`
	TotalEntries int            `json:"totalEntries"`
	Matched      int            `json:"matched"`
	Mismatched   int            `json:"mismatched"`
	Failed       int            `json:"failed"`
	Skipped      int            `json:"skipped"`
	Results      []ReplayResult `json:"results"`
}

// ReplaySession re-runs every (or a subset of) captured entry against either
// the original URL or a new TargetURL and returns a per-entry summary.
//
//	summary, err := client.Recorder().ReplaySession(ctx, "sess-123", &mockarty.ReplayOptions{
//	    TargetURL:   "http://staging.example.com",
//	    Concurrency: 5,
//	})
func (a *RecorderAPI) ReplaySession(ctx context.Context, id string, opts *ReplayOptions) (*ReplaySummary, error) {
	body := opts
	if body == nil {
		body = &ReplayOptions{}
	}
	var out ReplaySummary
	if err := a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(id)+"/replay", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Correlation engine
// ---------------------------------------------------------------------------

// CorrelationOptions configures a correlation analysis run. Defaults are
// chosen to surface common REST patterns; override for noisier traffic.
type CorrelationOptions struct {
	EntryIDs                 []string `json:"entryIds,omitempty"`
	MinValueLength           int      `json:"minValueLength,omitempty"`           // default 4
	MaxValueLength           int      `json:"maxValueLength,omitempty"`           // default 512
	ExcludeNumeric           bool     `json:"excludeNumeric,omitempty"`           // default false
	MaxCorrelationsPerSource int      `json:"maxCorrelationsPerSource,omitempty"` // default 50
}

// CorrelationLocation pins a value to a section/path inside one entry.
type CorrelationLocation struct {
	EntryID  string `json:"entryId"`
	Sequence int    `json:"sequence"`
	Section  string `json:"section"`
	Path     string `json:"path"`
}

// Correlation links a value found in one entry's response to its later use
// in another entry's request (URL, header, body, cookie, etc.).
type Correlation struct {
	Value      string                `json:"value"`
	ValueType  string                `json:"valueType"` // string|number|uuid|jwt|token
	Source     CorrelationLocation   `json:"source"`
	Targets    []CorrelationLocation `json:"targets"`
	Confidence float64               `json:"confidence"`
	Reason     string                `json:"reason"`
}

// CorrelationReport is the result of CorrelateSession.
type CorrelationReport struct {
	SessionID    string        `json:"sessionId"`
	TotalEntries int           `json:"totalEntries"`
	Scanned      int           `json:"scanned"`
	Correlations []Correlation `json:"correlations"`
	Summary      struct {
		ByValueType map[string]int `json:"byValueType"`
		BySection   map[string]int `json:"bySection"`
	} `json:"summary"`
}

// CorrelateSession runs the deterministic value-matching correlation engine
// against a captured session and returns a report linking response values
// (tokens, IDs, cookies) to their later re-use sites.
//
//	report, err := client.Recorder().CorrelateSession(ctx, "sess-123", nil)
//	for _, c := range report.Correlations {
//	    fmt.Printf("%s (%s) → %d targets\n", c.Value, c.ValueType, len(c.Targets))
//	}
func (a *RecorderAPI) CorrelateSession(ctx context.Context, id string, opts *CorrelationOptions) (*CorrelationReport, error) {
	body := opts
	if body == nil {
		body = &CorrelationOptions{}
	}
	var out CorrelationReport
	if err := a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(id)+"/correlate", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Recorder Config type
// ---------------------------------------------------------------------------

// RecorderConfig represents a saved recorder configuration.
type RecorderConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
	Port      int    `json:"port,omitempty"`
}

// ---------------------------------------------------------------------------
// Configs
// ---------------------------------------------------------------------------

// ListConfigs returns all recorder configurations.
func (a *RecorderAPI) ListConfigs(ctx context.Context) ([]RecorderConfig, error) {
	var configs []RecorderConfig
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/configs", nil, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// SaveConfig creates or updates a recorder configuration.
func (a *RecorderAPI) SaveConfig(ctx context.Context, config *RecorderConfig) (*RecorderConfig, error) {
	var result RecorderConfig
	if err := a.client.do(ctx, "POST", "/api/v1/recorder/configs", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteConfig deletes a recorder configuration by ID.
func (a *RecorderAPI) DeleteConfig(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/recorder/configs/"+url.PathEscape(id), nil, nil)
}

// ExportConfig exports a recorder configuration as raw bytes.
func (a *RecorderAPI) ExportConfig(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/recorder/configs/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// CA (Certificate Authority)
// ---------------------------------------------------------------------------

// GetCAStatus returns the CA certificate status.
func (a *RecorderAPI) GetCAStatus(ctx context.Context) (map[string]any, error) {
	var status map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/ca/status", nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

// GenerateCA generates a new CA certificate.
func (a *RecorderAPI) GenerateCA(ctx context.Context) error {
	return a.client.do(ctx, "POST", "/api/v1/recorder/ca/generate", nil, nil)
}

// DownloadCA downloads the CA certificate as raw bytes.
func (a *RecorderAPI) DownloadCA(ctx context.Context) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/recorder/ca/download", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Entry Operations
// ---------------------------------------------------------------------------

// AnnotateEntry adds or updates an annotation on a recorded entry.
func (a *RecorderAPI) AnnotateEntry(ctx context.Context, sessionID string, entryID string, annotation any) error {
	return a.client.do(ctx, "PATCH", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/entries/"+url.PathEscape(entryID), annotation, nil)
}

// ReplayEntry replays a recorded entry.
func (a *RecorderAPI) ReplayEntry(ctx context.Context, sessionID string, entryID string) error {
	return a.client.do(ctx, "POST", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/entries/"+url.PathEscape(entryID)+"/replay", nil, nil)
}

// ---------------------------------------------------------------------------
// Modifications
// ---------------------------------------------------------------------------

// GetModifications returns the request/response modifications for a session.
func (a *RecorderAPI) GetModifications(ctx context.Context, sessionID string) (map[string]any, error) {
	var mods map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/modifications", nil, &mods); err != nil {
		return nil, err
	}
	return mods, nil
}

// UpdateModifications updates the request/response modifications for a session.
func (a *RecorderAPI) UpdateModifications(ctx context.Context, sessionID string, mods any) error {
	return a.client.do(ctx, "PUT", "/api/v1/recorder/"+url.PathEscape(sessionID)+"/modifications", mods, nil)
}

// ---------------------------------------------------------------------------
// Ports
// ---------------------------------------------------------------------------

// GetPorts returns available recorder proxy ports.
func (a *RecorderAPI) GetPorts(ctx context.Context) (map[string]any, error) {
	var ports map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/recorder/ports", nil, &ports); err != nil {
		return nil, err
	}
	return ports, nil
}
