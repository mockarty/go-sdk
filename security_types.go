// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import "time"

// SecurityFinding mirrors internal/security.Finding for the CI/CD-facing
// subset that operator scripts care about. The full server-side struct
// carries additional fields (KB doc IDs, fingerprint, confidence) that
// are intentionally surfaced here verbatim so callers parsing the
// JSON shape from the SDK or the raw HTTP response see the same data.
//
// Field order honours CLAUDE.md §Struct Field Alignment: 24-byte
// composite (time.Time + []string slice) first, then 16-byte strings,
// then 8-byte float64s, finally the trailing 1-byte bool absorbing
// tail padding.
type SecurityFinding struct {
	CreatedAt      time.Time `json:"createdAt"`
	Target         string    `json:"target"`
	CWEID          string    `json:"cweId,omitempty"`
	ReportID       string    `json:"reportId"`
	Namespace      string    `json:"namespace"`
	Persona        string    `json:"persona"`
	Scanner        string    `json:"scanner"`
	Severity       string    `json:"severity"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	OWASPCategory  string    `json:"owaspCategory,omitempty"`
	ID             string    `json:"id"`
	CVEID          string    `json:"cveId,omitempty"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
	CVSSVector     string    `json:"cvssVector,omitempty"`
	Evidence       string    `json:"evidence,omitempty"`
	ReproducerCurl string    `json:"reproducerCurl,omitempty"`
	Remediation    string    `json:"remediation,omitempty"`
	TriagedStatus  string    `json:"triagedStatus,omitempty"`
	KBDocIDs       []string  `json:"kbDocIds,omitempty"`
	CVSSScore      float64   `json:"cvssScore,omitempty"`
	Confidence     float64   `json:"confidence,omitempty"`
	KEV            bool      `json:"kev,omitempty"`
}

// SecurityTarget mirrors internal/security.Target. URL is the canonical
// address; for the web/api personas it is an HTTP URL, for infra it may
// be tcp://host:port, for cloud it is provider:account/resource.
type SecurityTarget struct {
	Headers map[string]string `json:"headers,omitempty"`
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// SecurityScopeImports references Mockarty entities that bound the
// attack surface for a scan.
type SecurityScopeImports struct {
	RecorderSessionIDs []string `json:"recorderSessionIds,omitempty"`
	TestPlanIDs        []string `json:"testPlanIds,omitempty"`
	CollectionIDs      []string `json:"collectionIds,omitempty"`
	ContractIDs        []string `json:"contractIds,omitempty"`
	MockIDs            []string `json:"mockIds,omitempty"`
}

// SecurityScanProfile mirrors internal/security.ScanProfile for the
// CI/CD-facing subset. Field order honours CLAUDE.md §Struct Field
// Alignment: embedded composite first (ScopeImports), then 24-byte
// slices, then 16-byte string, then 8-byte ints, finally the 1-byte
// bool absorbing tail padding.
type SecurityScanProfile struct {
	ScopeDescription     string               `json:"scopeDescription,omitempty"`
	Intensity            string               `json:"intensity,omitempty"`
	ScopeImports         SecurityScopeImports `json:"scopeImports,omitempty"`
	Targets              []SecurityTarget     `json:"targets,omitempty"`
	ExcludedPaths        []string             `json:"excludedPaths,omitempty"`
	NotificationChannels []string             `json:"notificationChannels,omitempty"`
	MaxTokens            int64                `json:"maxTokens,omitempty"`
	MaxCostUSDMicros     int64                `json:"maxCostUsdMicros,omitempty"`
	NSDailyCapMicros     int64                `json:"nsDailyCapMicros,omitempty"`
	RatePerHost          int                  `json:"ratePerHost,omitempty"`
	RedactTokensInReport bool                 `json:"redactTokensInReport,omitempty"`
}

// SecurityReport mirrors internal/security.Report. Field order: embedded
// composite first, then time.Time slots, then 16-byte strings, finally
// 8-byte int64 cost counters.
type SecurityReport struct {
	CreatedAt     time.Time           `json:"createdAt"`
	StartedAt     time.Time           `json:"startedAt,omitempty"`
	CompletedAt   time.Time           `json:"completedAt,omitempty"`
	ID            string              `json:"id"`
	Namespace     string              `json:"namespace"`
	AgentTaskID   string              `json:"agentTaskId,omitempty"`
	InitiatedBy   string              `json:"initiatedBy,omitempty"`
	Title         string              `json:"title"`
	Status        string              `json:"status"`
	Profile       SecurityScanProfile `json:"profile"`
	CostTokens    int64               `json:"costTokens,omitempty"`
	CostUSDMicros int64               `json:"costUsdMicros,omitempty"`
}

// SecurityScanner is one entry of the /api/v1/security/scanners listing
// — the catalogue of registered scan providers the agent family can run.
type SecurityScanner struct {
	Key       string `json:"key"`
	Persona   string `json:"persona"`
	Intensity string `json:"intensity"`
}

// StartScanRequest is the body of POST /api/v1/security/scans. Either
// supply a fully-populated Profile (recommended) or rely on the server
// defaults (intensity = safe-active).
type StartScanRequest struct {
	Title     string              `json:"title"`
	Namespace string              `json:"namespace"`
	Profile   SecurityScanProfile `json:"profile"`
}

// ListFindingsOptions filters the response of ListFindings. Empty
// fields are ignored — the server returns every finding for the report.
type ListFindingsOptions struct {
	Severity string
}
