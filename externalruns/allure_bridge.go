// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package externalruns

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FromAllureDir reads an allure-results directory and converts each
// `<uuid>-result.json` into a CreateRunRequest + step list pair ready for
// submission via Client.CreateRun + Client.AddSteps.
//
// The bridge mirrors the format the Go SDK's allure/ package emits (which
// is byte-shape compatible with allure-pytest and allure-java) so callers
// can run their tests, then push the produced results into Mockarty TCM
// with zero glue code.
//
// Returns ErrInvalidConfig when dir is empty or does not exist. Returns
// nil + empty slice when the directory exists but has no result files.
func FromAllureDir(dir string) ([]AllureRun, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("%w: dir is required", ErrInvalidConfig)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read allure-results dir %q: %w", dir, err)
	}
	out := make([]AllureRun, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), "-result.json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var r allureResult
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		run := r.toExternalRun()
		// Carry attachments forward as paths so callers can stream them up
		// via Client.AttachReport.
		for _, att := range r.Attachments {
			run.AttachmentPaths = append(run.AttachmentPaths, filepath.Join(dir, att.Source))
		}
		out = append(out, run)
	}
	return out, nil
}

// AllureRun is the per-test conversion result: ready-to-POST run request
// + steps + attachment file paths.
type AllureRun struct {
	Request         CreateRunRequest
	FinishRequest   FinishRunRequest
	Steps           []Step
	AttachmentPaths []string
}

// allureResult mirrors the relevant subset of the Allure 2 result JSON we
// need to convert. We deliberately do NOT pull in the full SDK model
// here — keeping the dependency direction one-way (externalruns has no
// build-time reference to allure/).
type allureResult struct {
	StatusDetails *struct {
		Message string `json:"message,omitempty"`
		Trace   string `json:"trace,omitempty"`
	} `json:"statusDetails,omitempty"`
	UUID        string `json:"uuid"`
	HistoryID   string `json:"historyId"`
	FullName    string `json:"fullName,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Labels      []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"labels,omitempty"`
	Parameters []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"parameters,omitempty"`
	Steps       []allureStep `json:"steps,omitempty"`
	Attachments []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Type   string `json:"type,omitempty"`
	} `json:"attachments,omitempty"`
	Start int64 `json:"start"`
	Stop  int64 `json:"stop"`
}

type allureStep struct {
	StatusDetails *struct {
		Message string `json:"message,omitempty"`
		Trace   string `json:"trace,omitempty"`
	} `json:"statusDetails,omitempty"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Parameters []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"parameters,omitempty"`
	Steps []allureStep `json:"steps,omitempty"`
	Start int64        `json:"start"`
	Stop  int64        `json:"stop"`
}

// toExternalRun converts the allureResult into the ExternalRuns request
// shape. Labels become tags, parameters carry forward to the first step.
func (r allureResult) toExternalRun() AllureRun {
	framework := "go-test"
	for _, l := range r.Labels {
		if l.Name == "framework" && l.Value != "" {
			framework = l.Value
			break
		}
	}
	tags := make([]string, 0, len(r.Labels))
	for _, l := range r.Labels {
		if l.Name == "tag" && l.Value != "" {
			tags = append(tags, l.Value)
		}
	}
	env := map[string]string{}
	for _, l := range r.Labels {
		switch l.Name {
		case "host", "thread", "package", "testClass", "testMethod", "AS_ID", "suite", "parentSuite", "subSuite":
			if l.Value != "" {
				env[l.Name] = l.Value
			}
		}
	}
	req := CreateRunRequest{
		Name:        firstNonEmptyAllure(r.FullName, r.Name),
		Framework:   framework,
		ExternalID:  r.HistoryID,
		StartedAt:   epochMillisToTime(r.Start),
		Tags:        tags,
		Environment: env,
	}
	steps := make([]Step, 0, countSteps(r.Steps))
	collectSteps("", r.Steps, &steps)
	final := FinishRunRequest{
		FinishedAt: epochMillisToTime(r.Stop),
		Status:     statusOf(r.Status),
	}
	if r.StatusDetails != nil && r.StatusDetails.Message != "" {
		final.Summary = r.StatusDetails.Message
	}
	return AllureRun{Request: req, FinishRequest: final, Steps: steps}
}

func countSteps(s []allureStep) int {
	n := len(s)
	for i := range s {
		n += countSteps(s[i].Steps)
	}
	return n
}

func collectSteps(parentKey string, ss []allureStep, out *[]Step) {
	for i, st := range ss {
		key := fmt.Sprintf("%s%d", parentKey, i)
		if parentKey != "" {
			key = parentKey + "." + fmt.Sprintf("%d", i)
		}
		step := Step{
			StepKey:    key,
			ParentKey:  parentKey,
			Name:       st.Name,
			Status:     statusOf(st.Status),
			StartedAt:  epochMillisToTime(st.Start),
			DurationMS: st.Stop - st.Start,
		}
		fin := epochMillisToTime(st.Stop)
		if !fin.IsZero() {
			step.FinishedAt = &fin
		}
		if st.StatusDetails != nil {
			step.Message = st.StatusDetails.Message
			step.StackTrace = st.StatusDetails.Trace
		}
		if len(st.Parameters) > 0 {
			step.Parameters = make(map[string]string, len(st.Parameters))
			for _, p := range st.Parameters {
				step.Parameters[p.Name] = p.Value
			}
		}
		*out = append(*out, step)
		collectSteps(key, st.Steps, out)
	}
}

func statusOf(s string) Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "passed":
		return StatusPassed
	case "failed":
		return StatusFailed
	case "broken":
		return StatusBroken
	case "skipped":
		return StatusSkipped
	case "running":
		return StatusRunning
	}
	return StatusUnknown
}

func epochMillisToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
}

func firstNonEmptyAllure(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ErrEmptyDir is returned by FromAllureDir when the directory is valid
// but contains no `*-result.json` files. Callers usually treat this as
// "no tests ran" rather than as a fatal error.
var ErrEmptyDir = errors.New("externalruns: allure-results dir is empty")
