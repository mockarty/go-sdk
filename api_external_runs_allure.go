// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AllureUploadOptions configures UploadAllureDir.
type AllureUploadOptions struct {
	// Namespace targets a Mockarty workspace (empty = client default).
	Namespace string
	// PlanID associates every uploaded run with a test plan (optional).
	PlanID string
	// Framework is the wire "framework" label (default "allure").
	Framework string
	// AutoCreate lets the server create a missing case on resolution miss.
	AutoCreate bool
	// OnError controls error policy: "warn" (default) skips a bad file and
	// continues; "raise" returns on the first read/parse/upload error.
	OnError string
}

// UploadAllureDir reads an allure-results directory, translates every
// *-result.json into an external-run payload (via allureResultToExternalRun),
// and reports each to TCM. Returns the per-result server responses.
//
// Parity with Python upload_allure_dir / Java uploadAllureDir — the Go SDK was
// missing the Allure-folder ingestion path used by CI pipelines.
func (a *ExternalRunsAPI) UploadAllureDir(ctx context.Context, directory string, opts AllureUploadOptions) ([]*ExternalRunResponse, error) {
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("mockarty: allure-results directory not found: %s", directory)
	}
	if opts.Framework == "" {
		opts.Framework = "allure"
	}
	matches, err := filepath.Glob(filepath.Join(directory, "*-result.json"))
	if err != nil {
		return nil, fmt.Errorf("mockarty: scan allure dir: %w", err)
	}
	sort.Strings(matches)

	out := make([]*ExternalRunResponse, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: read %s: %w", path, err)
			}
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: parse %s: %w", path, err)
			}
			continue
		}
		resp, err := a.Report(ctx, opts.Namespace, allureResultToExternalRun(doc, directory, opts))
		if err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: upload %s: %w", path, err)
			}
			continue
		}
		out = append(out, resp)
	}
	return out, nil
}

// allureResultToExternalRun maps an Allure-2 TestResult document to an
// ExternalRunRequest. Mirrors the Python allure_result_to_external_payload
// translation so all three SDKs produce identical wire payloads.
func allureResultToExternalRun(doc map[string]any, directory string, opts AllureUploadOptions) ExternalRunRequest {
	name := allureStr(doc, "name")
	if name == "" {
		name = allureStr(doc, "fullName")
	}
	if name == "" {
		name = "unnamed"
	}
	fullName := allureStr(doc, "fullName")

	status := strings.ToLower(allureStr(doc, "status"))
	switch status {
	case "passed", "failed", "broken", "skipped":
	default:
		status = "failed"
	}
	if status == "broken" { // server enum has no "broken"
		status = "failed"
	}

	var durationMs int64
	start, sOK := allureNum(doc["start"])
	stop, eOK := allureNum(doc["stop"])
	if sOK && eOK && stop >= start {
		durationMs = stop - start
	}

	var errMsg string
	if sd, ok := doc["statusDetails"].(map[string]any); ok {
		if msg, _ := sd["message"].(string); msg != "" {
			errMsg = msg
			if tr, _ := sd["trace"].(string); tr != "" {
				errMsg = msg + "\n" + tr
			}
		}
	}

	labels := map[string]string{}
	if labs, ok := doc["labels"].([]any); ok {
		for _, l := range labs {
			if lm, ok := l.(map[string]any); ok {
				if n := allureStr(lm, "name"); n != "" {
					labels[n] = allureStr(lm, "value")
				}
			}
		}
	}

	var steps []ExternalStep
	if ss, ok := doc["steps"].([]any); ok {
		for _, s := range ss {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			st := strings.ToLower(allureStr(sm, "status"))
			if st == "broken" {
				st = "failed"
			}
			switch st {
			case "passed", "failed", "skipped":
			default:
				st = "failed"
			}
			var stepErr string
			if sd, ok := sm["statusDetails"].(map[string]any); ok {
				stepErr, _ = sd["message"].(string)
			}
			steps = append(steps, ExternalStep{Name: allureStr(sm, "name"), Status: st, Error: stepErr})
		}
	}

	var atts []ExternalAttachment
	if as, ok := doc["attachments"].([]any); ok {
		for _, at := range as {
			am, ok := at.(map[string]any)
			if !ok {
				continue
			}
			src := allureStr(am, "source")
			if src == "" {
				continue
			}
			body, err := os.ReadFile(filepath.Join(directory, src))
			if err != nil {
				continue // skip a missing attachment rather than fail the upload
			}
			attName := allureStr(am, "name")
			if attName == "" {
				attName = src
			}
			ct := allureStr(am, "type")
			if ct == "" {
				ct = "application/octet-stream"
			}
			atts = append(atts, ExternalAttachment{
				Name:        attName,
				ContentType: ct,
				BodyB64:     base64.StdEncoding.EncodeToString(body),
			})
		}
	}

	run := ExternalRunRequest{
		Status:          status,
		TestCaseID:      allureStr(doc, "testCaseId"),
		CaseName:        name,
		PlanID:          opts.PlanID,
		AutoCreate:      opts.AutoCreate,
		Framework:       opts.Framework,
		ExternalID:      allureStr(doc, "uuid"),
		TestDisplayName: name,
		DurationMs:      durationMs,
		Error:           errMsg,
		FullName:        fullName,
		Steps:           steps,
		Attachments:     atts,
	}
	if len(labels) > 0 {
		run.Labels = labels
	}
	if fullName != "" {
		run.Metadata = map[string]any{"allureFullName": fullName}
	}
	return run
}

// allureStr reads a string field from a decoded JSON object, "" if absent.
func allureStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// allureNum reads an epoch-millis number (Allure start/stop) from decoded JSON.
func allureNum(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int64:
		return n, true
	}
	return 0, false
}
