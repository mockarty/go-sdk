// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

// JSON schema types for Allure 2 `allure-results/*.json`.
//
// Field order matches the canonical schema used by `allure generate`
// (see https://github.com/allure-framework/allure2). The fields exposed
// here are the minimum set produced by the Python/Java SDKs — anything
// the official Allure CLI optionally accepts but those SDKs do not emit
// is omitted to keep output byte-stable across the three languages.
//
// Struct fields are ordered for fieldalignment cleanliness (8 → 4 → 1).
//
// Types are prefixed `Allure*` to coexist with the package-level function
// API (Step, Attachment, Label, Parameter, ...).

// Status is the result outcome enum used by Allure.
type Status string

// Status enum values (lowercase string, matches Allure JSON literals).
const (
	StatusPassed  Status = "passed"
	StatusFailed  Status = "failed"  // assertion failed
	StatusBroken  Status = "broken"  // unexpected error
	StatusSkipped Status = "skipped" // test skipped
)

// Stage marks the lifecycle phase of a result or step. We always emit
// "finished" — the other values (scheduled, running, pending) are reserved
// for live-update writers we do not implement here.
type Stage string

// Stage enum values.
const (
	StageFinished Stage = "finished"
)

// Severity is the priority label Allure renders as a colored chip.
type Severity string

// Severity enum values (lowercase strings, matches Allure JSON literals).
const (
	SeverityBlocker  Severity = "blocker"
	SeverityCritical Severity = "critical"
	SeverityNormal   Severity = "normal"
	SeverityMinor    Severity = "minor"
	SeverityTrivial  Severity = "trivial"
)

// Result is the top-level `<uuid>-result.json` document.
//
// Collection fields (labels/links/parameters/steps/attachments) drop
// `omitempty` so we always emit `[]` instead of dropping the key. This
// matches allure-pytest's wire format (which always emits empty arrays)
// and is what makes a Python+Go run interleavable in a single Allure
// report. Scalar fields keep omitempty to suppress noise.
type Result struct {
	StatusDetails   *StatusDetail      `json:"statusDetails,omitempty"`
	UUID            string             `json:"uuid"`
	HistoryID       string             `json:"historyId"`
	TestCaseID      string             `json:"testCaseId,omitempty"`
	FullName        string             `json:"fullName,omitempty"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	DescriptionHTML string             `json:"descriptionHtml,omitempty"`
	Status          Status             `json:"status"`
	Stage           Stage              `json:"stage"`
	Labels          []AllureLabel      `json:"labels"`
	Links           []AllureLink       `json:"links"`
	Parameters      []AllureParameter  `json:"parameters"`
	Steps           []AllureStep       `json:"steps"`
	Attachments     []AllureAttachment `json:"attachments"`
	Start           int64              `json:"start"`
	Stop            int64              `json:"stop"`
}

// AllureStep is a nested action within a Result. Steps may contain child
// steps (recursive), attachments, and parameters — same as the parent
// Result. As with Result, collection fields always serialise (even when
// empty) for byte-shape parity with allure-pytest's wire format.
type AllureStep struct {
	StatusDetails *StatusDetail      `json:"statusDetails,omitempty"`
	Name          string             `json:"name"`
	Status        Status             `json:"status"`
	Stage         Stage              `json:"stage"`
	Parameters    []AllureParameter  `json:"parameters"`
	Steps         []AllureStep       `json:"steps"`
	Attachments   []AllureAttachment `json:"attachments"`
	Start         int64              `json:"start"`
	Stop          int64              `json:"stop"`
}

// StatusDetail carries an error message and an optional stack trace.
type StatusDetail struct {
	Message string `json:"message,omitempty"`
	Trace   string `json:"trace,omitempty"`
	Known   bool   `json:"known,omitempty"`
	Muted   bool   `json:"muted,omitempty"`
	Flaky   bool   `json:"flaky,omitempty"`
}

// AllureLabel is a `key=value` metadata pair. Allure recognises canonical
// names such as "feature", "story", "epic", "severity", "owner", "suite",
// "tag", "package", "testClass", "framework", "language", "host", "thread",
// "AS_ID".
type AllureLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AllureLink is an external pointer (issue tracker, TMS, ...). `Type` is
// one of "issue", "tms", or empty/"link".
type AllureLink struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
	Type string `json:"type,omitempty"`
}

// AllureParameter is a name/value pair surfaced in the test header.
type AllureParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AllureAttachment references a file written next to the result JSON.
type AllureAttachment struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Type   string `json:"type,omitempty"`
}

// Container is the `<uuid>-container.json` schema. Containers group child
// results (e.g. a class-level setUp/tearDown). We use them when multiple
// tests share a fixture; standalone tests don't need one.
type Container struct {
	UUID     string       `json:"uuid"`
	Name     string       `json:"name,omitempty"`
	Children []string     `json:"children"`
	Befores  []AllureStep `json:"befores,omitempty"`
	Afters   []AllureStep `json:"afters,omitempty"`
	Start    int64        `json:"start,omitempty"`
	Stop     int64        `json:"stop,omitempty"`
}

// Executor is the `executor.json` schema, written once per run.
type Executor struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	URL        string `json:"url,omitempty"`
	ReportName string `json:"reportName,omitempty"`
	ReportURL  string `json:"reportUrl,omitempty"`
	BuildName  string `json:"buildName,omitempty"`
	BuildOrder string `json:"buildOrder,omitempty"`
	BuildURL   string `json:"buildUrl,omitempty"`
}

// Category is the `categories.json` schema, optionally written once per run.
type Category struct {
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	DescriptionHTML string   `json:"descriptionHtml,omitempty"`
	MessageRegex    string   `json:"messageRegex,omitempty"`
	TraceRegex      string   `json:"traceRegex,omitempty"`
	MatchedStatuses []Status `json:"matchedStatuses,omitempty"`
}

// Canonical Allure label keys.
const (
	LabelFeature     = "feature"
	LabelStory       = "story"
	LabelEpic        = "epic"
	LabelSeverity    = "severity"
	LabelOwner       = "owner"
	LabelTag         = "tag"
	LabelSuite       = "suite"
	LabelParentSuite = "parentSuite"
	LabelSubSuite    = "subSuite"
	LabelHost        = "host"
	LabelThread      = "thread"
	LabelFramework   = "framework"
	LabelLanguage    = "language"
	LabelPackage     = "package"
	LabelTestClass   = "testClass"
	LabelTestMethod  = "testMethod"
	LabelAllureID    = "AS_ID"
)

// Canonical Allure link types.
const (
	LinkTypeIssue   = "issue"
	LinkTypeTMS     = "tms"
	LinkTypeGeneric = "link"
)
