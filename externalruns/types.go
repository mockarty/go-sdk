package externalruns

import "time"

// Status is the canonical outcome enum shared with the Mockarty server,
// the Python SDK (mockarty.external_runs.Status) and the Java SDK
// (ru.mockarty.api.external_runs.Status). Adding a value here is a
// schema bump — see schema.go.
type Status string

const (
	StatusPassed  Status = "passed"
	StatusFailed  Status = "failed"
	StatusBroken  Status = "broken"
	StatusSkipped Status = "skipped"
	StatusRunning Status = "running"
	StatusUnknown Status = "unknown"
)

// Valid reports whether s is a known status value.
func (s Status) Valid() bool {
	switch s {
	case StatusPassed, StatusFailed, StatusBroken, StatusSkipped, StatusRunning, StatusUnknown:
		return true
	}
	return false
}

// CreateRunRequest is the body of POST /tcm/external-runs.
//
// Required:
//
//	Name             — human-readable label, shown in the TCM run list.
//	Framework        — the framework that produced the run (e.g. "go-test",
//	                   "pytest", "junit5"). Used for icon + filter only.
//
// Optional:
//
//	SuiteID          — TCM suite this run belongs to (skip to attach later).
//	ExternalID       — caller-supplied de-dup key. If a run with the same
//	                   (namespace, external_id) exists, the server returns
//	                   it instead of creating a duplicate.
//	StartedAt        — when the test run actually started; defaults to
//	                   the server's now() if zero.
//	Environment      — free-form key/value labels (CI job, git sha, etc.).
//	Tags             — TCM-level labels for filtering.
type CreateRunRequest struct {
	StartedAt   time.Time         `json:"started_at,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Name        string            `json:"name"`
	Framework   string            `json:"framework"`
	SuiteID     string            `json:"suite_id,omitempty"`
	ExternalID  string            `json:"external_id,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// Run is the canonical run envelope returned by every endpoint.
type Run struct {
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	ID          string            `json:"id"`
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Framework   string            `json:"framework"`
	SuiteID     string            `json:"suite_id,omitempty"`
	ExternalID  string            `json:"external_id,omitempty"`
	Status      Status            `json:"status"`
	Tags        []string          `json:"tags,omitempty"`
	Steps       []Step            `json:"steps,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	StepCount   int               `json:"step_count"`
	SchemaVer   int               `json:"schema_version"`
}

// Step is one logical step inside a run.
//
// StepKey is a caller-supplied stable identifier; submitting the same key
// twice updates the existing step rather than appending a duplicate. This
// lets a streaming reporter retry on transient failure without polluting
// the run.
type Step struct {
	StartedAt  time.Time         `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	StepKey    string            `json:"step_key"`
	Name       string            `json:"name"`
	Status     Status            `json:"status"`
	Message    string            `json:"message,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
	ParentKey  string            `json:"parent_key,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
}

// Attachment is the server-side metadata returned after a successful
// AttachReport call. The client uploads bytes via multipart and the server
// echoes the stored URL and content addressing.
type Attachment struct {
	UploadedAt time.Time `json:"uploaded_at"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MimeType   string    `json:"mime_type"`
	URL        string    `json:"url"`
	SHA256     string    `json:"sha256"`
	Size       int64     `json:"size_bytes"`
}

// FinishRunRequest closes a run. If Status is zero the server infers it
// from the step outcomes (any failed -> failed; any broken -> broken; all
// skipped -> skipped; otherwise passed).
type FinishRunRequest struct {
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Status     Status    `json:"status,omitempty"`
	Summary    string    `json:"summary,omitempty"`
}

// ListRunsOptions are query-string filters for GET /tcm/external-runs.
//
// An empty value means "no filter". Limit defaults to 50 (server-side) and
// is capped at 500; pass Limit<=0 to accept the server default.
type ListRunsOptions struct {
	SuiteID    string
	Framework  string
	Status     Status
	ExternalID string
	Cursor     string
	Limit      int
}

// RunList is the response from GET /tcm/external-runs.
//
// NextCursor is empty when there is no further page.
type RunList struct {
	NextCursor string `json:"next_cursor,omitempty"`
	Runs       []Run  `json:"runs"`
	Total      int    `json:"total"`
}
