// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// SecurityAPI exposes the CI/CD-useful subset of the Mockarty Security
// Agent endpoints — start a scan, poll status, list findings, download
// SARIF, list scanners, cancel a scan. Admin-only routes (LLM profile
// CRUD, agent disable, template editing) are deliberately omitted; use
// the admin UI for those.
//
// All routes are gated server-side by the `security_agent` feature
// flag; a 403 from any method means the namespace lacks the feature.
type SecurityAPI struct {
	client *Client
}

// Security returns the Security Agent API.
func (c *Client) Security() *SecurityAPI { return &SecurityAPI{client: c} }

// StartScan kicks off an orchestrated security scan and returns the
// freshly-created report (with the server-assigned ID + status). The
// caller polls GetReport until status reaches one of "done", "failed",
// or "cancelled".
//
// When the server has no orchestrator wired (air-gapped pull-runner
// deployments) StartScan degrades to creating a queued report row
// that an external runner picks up later — the returned ID is still
// valid for GetReport / ListFindings / ExportReport / CancelScan.
func (a *SecurityAPI) StartScan(ctx context.Context, req StartScanRequest) (*SecurityReport, error) {
	if req.Namespace == "" && a.client.namespace != "" {
		req.Namespace = a.client.namespace
	}
	if strings.TrimSpace(req.Namespace) == "" {
		return nil, fmt.Errorf("mockarty: security.StartScan: namespace is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("mockarty: security.StartScan: title is required")
	}
	var envelope struct {
		Report SecurityReport `json:"report"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/security/scans", req, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Report, nil
}

// GetReport returns the current state of a scan report. Poll this to
// observe the status flip (running → done/failed) and to read final
// cost counters.
func (a *SecurityAPI) GetReport(ctx context.Context, reportID string) (*SecurityReport, error) {
	if strings.TrimSpace(reportID) == "" {
		return nil, fmt.Errorf("mockarty: security.GetReport: reportID is required")
	}
	var envelope struct {
		Report SecurityReport `json:"report"`
	}
	path := "/api/v1/security/reports/" + url.PathEscape(reportID)
	if err := a.client.do(ctx, "GET", path, nil, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Report, nil
}

// ListFindings returns every finding recorded against the given report.
// The optional `opts.Severity` filter is applied client-side because
// the server endpoint does not currently support a `severity` query
// parameter — keeping the SDK API stable here lets the server add one
// later without breaking callers.
func (a *SecurityAPI) ListFindings(ctx context.Context, reportID string, opts ListFindingsOptions) ([]SecurityFinding, error) {
	if strings.TrimSpace(reportID) == "" {
		return nil, fmt.Errorf("mockarty: security.ListFindings: reportID is required")
	}
	var envelope struct {
		Findings []SecurityFinding `json:"findings"`
	}
	path := "/api/v1/security/reports/" + url.PathEscape(reportID) + "/findings"
	if err := a.client.do(ctx, "GET", path, nil, &envelope); err != nil {
		return nil, err
	}
	sev := strings.ToLower(strings.TrimSpace(opts.Severity))
	if sev == "" {
		return envelope.Findings, nil
	}
	filtered := envelope.Findings[:0]
	for _, f := range envelope.Findings {
		if strings.ToLower(f.Severity) == sev {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

// ExportReport downloads the report serialised in the requested format
// — one of "sarif", "vex", "html", "pdf", "allure". Returns the raw
// bytes; the caller persists them to disk.
func (a *SecurityAPI) ExportReport(ctx context.Context, reportID, format string) ([]byte, error) {
	if strings.TrimSpace(reportID) == "" {
		return nil, fmt.Errorf("mockarty: security.ExportReport: reportID is required")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "sarif"
	}
	switch format {
	case "sarif", "vex", "cyclonedx", "cyclonedx-vex", "html", "pdf", "allure":
	default:
		return nil, fmt.Errorf("mockarty: security.ExportReport: unsupported format %q (want one of sarif|vex|html|pdf|allure)", format)
	}
	path := "/api/v1/security/reports/" + url.PathEscape(reportID) + "/export?format=" + url.QueryEscape(format)
	return a.client.doJSON(ctx, "GET", path, nil)
}

// ListScanners enumerates every registered scan provider the family
// can run. Useful before StartScan to verify the persona/intensity
// combo the operator wants is actually available in this build.
func (a *SecurityAPI) ListScanners(ctx context.Context) ([]SecurityScanner, error) {
	var envelope struct {
		Scanners []SecurityScanner `json:"scanners"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/security/scanners", nil, &envelope); err != nil {
		return nil, err
	}
	return envelope.Scanners, nil
}

// CancelScan signals an in-flight scan to wind down. The server flips
// the report row to status="cancelled" and propagates a cancel signal
// to every active remote runner in the namespace. Idempotent: a second
// call on the same report returns 409 Conflict (already terminal).
func (a *SecurityAPI) CancelScan(ctx context.Context, reportID string) error {
	if strings.TrimSpace(reportID) == "" {
		return fmt.Errorf("mockarty: security.CancelScan: reportID is required")
	}
	path := "/api/v1/security/reports/" + url.PathEscape(reportID) + "/cancel"
	// The server returns {"status": "cancelled", "signalled": <bool>};
	// discard the body — the HTTP 200 is enough.
	var sink json.RawMessage
	return a.client.do(ctx, "POST", path, nil, &sink)
}
