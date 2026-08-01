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
	var skipped []string
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: read %s: %w", path, err)
			}
			skipped = append(skipped, filepath.Base(path)+": "+err.Error())
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: parse %s: %w", path, err)
			}
			skipped = append(skipped, filepath.Base(path)+": "+err.Error())
			continue
		}
		resp, err := a.Report(ctx, opts.Namespace, allureResultToExternalRun(doc, directory, opts))
		if err != nil {
			if opts.OnError == "raise" {
				return nil, fmt.Errorf("mockarty: upload %s: %w", path, err)
			}
			skipped = append(skipped, filepath.Base(path)+": "+err.Error())
			continue
		}
		out = append(out, resp)
	}
	if len(skipped) > 0 {
		// "warn" policy keeps going past a bad file, but the caller must still
		// learn that N results never reached Mockarty — returning only the
		// successes made a half-lost upload indistinguishable from a clean one.
		// The successful responses are returned ALONGSIDE the error so a caller
		// that wants best-effort semantics can still use them.
		return out, &AllureUploadPartialError{Uploaded: len(out), Skipped: skipped}
	}
	if len(matches) == 0 {
		return out, fmt.Errorf("mockarty: no *-result.json in %s — nothing was reported", directory)
	}
	return out, nil
}

// AllureUploadPartialError reports results that did not reach Mockarty during a
// best-effort (OnError="warn") directory upload.
type AllureUploadPartialError struct {
	Skipped  []string
	Uploaded int
}

func (e *AllureUploadPartialError) Error() string {
	return fmt.Sprintf("mockarty: %d result(s) uploaded, %d not reported: %s",
		e.Uploaded, len(e.Skipped), strings.Join(e.Skipped, "; "))
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

	// Status vocabulary is shared with the server (passed/failed/broken/
	// skipped/cancelled/error). "broken" used to be flattened onto "failed"
	// under a stale "server enum has no broken" comment — it does, and the
	// distinction (an assertion failed vs the test itself blew up) is what
	// Allure TestOps and Test IT both report on.
	status := strings.ToLower(strings.TrimSpace(allureStr(doc, "status")))
	switch status {
	case "passed", "failed", "broken", "skipped", "cancelled":
	case "error":
		status = "broken"
	default:
		// Absent or "unknown" — we never observed an outcome. Not a pass,
		// and not an assertion failure either.
		status = "broken"
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
			st := strings.ToLower(strings.TrimSpace(allureStr(sm, "status")))
			switch st {
			case "passed", "failed", "skipped", "broken":
			case "error":
				st = "broken"
			default:
				st = "broken"
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
		TestCaseID:      allureTestCaseID(doc, labels),
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

// allureTestCaseID resolves the author-pinned identity of a result. Allure's
// own field is `testCaseId`; Allure TestOps adapters express `@AllureId(123)`
// as the `AS_ID` label instead, so a suite migrated from TestOps carried its
// identity somewhere this translator never looked — every upload looked new and
// the autotest-to-case link was lost. Mirrors the server-side
// allure.ResolveTestCaseID and the CLI, which must stay in lockstep.
func allureTestCaseID(doc map[string]any, labels map[string]string) string {
	if id := strings.TrimSpace(allureStr(doc, "testCaseId")); id != "" {
		return id
	}
	for name, value := range labels {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "as_id", "allure_id", "allureid":
			if v := strings.TrimSpace(value); v != "" {
				return v
			}
		}
	}
	return ""
}
