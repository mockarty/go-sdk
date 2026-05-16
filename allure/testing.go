// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"testing"
)

// AllureT is the Allure-aware wrapper returned by [T]. It exposes the same
// surface as the package-level annotation functions but bound to a single
// *testing.T scope. Test failures observed via t.Failed() bubble into the
// produced result automatically on t.Cleanup.
type AllureT struct {
	t     *testing.T
	scope *scope
	ctx   context.Context
}

// T wraps t and returns an AllureT bound to a fresh scope. The result is
// flushed on t.Cleanup. The scope uses the test's full name (t.Name) for
// the FullName field so re-runs cluster correctly in Allure's history view.
//
// Options are applied in order; pass [WithResultsDir] to redirect output,
// or labels/links via [WithFeature], [WithIssue], etc.
//
// Example:
//
//	func TestLogin(t *testing.T) {
//	    a := allure.T(t, allure.WithFeature("Auth"))
//	    a.Severity(allure.SeverityCritical)
//	    a.Step("submit", func() { ... })
//	}
func T(t *testing.T, opts ...Option) *AllureT {
	t.Helper()
	cfg := config{name: t.Name(), fullName: t.Name()}
	for _, o := range opts {
		o(&cfg)
	}
	dir := ResolveResultsDir(cfg.resultsDir)
	writer := NewFileWriter(dir)
	s := newScope(cfg, writer)
	at := &AllureT{
		t:     t,
		scope: s,
		ctx:   withScope(context.Background(), s),
	}
	t.Cleanup(at.flush)
	return at
}

// Context returns a context carrying the scope so callers can call
// package-level annotation functions (Step, Feature, ...) from helper
// functions that take a context.Context.
func (a *AllureT) Context() context.Context { return a.ctx }

// flush is the cleanup hook — captures the test's pass/fail status and
// writes the result JSON.
func (a *AllureT) flush() {
	if a == nil || a.scope == nil {
		return
	}
	if a.t != nil {
		if a.t.Skipped() {
			a.scope.markFailure(StatusSkipped, "test skipped", "")
		} else if a.t.Failed() {
			// We do not have direct access to testing.T's failure messages —
			// they live in a private buffer. We mark the result as failed and
			// leave step-level details (from Step / StepErr) to carry the
			// granular cause.
			a.scope.mu.Lock()
			if a.scope.result.Status == StatusPassed {
				a.scope.result.Status = StatusFailed
			}
			if a.scope.result.StatusDetails == nil {
				a.scope.result.StatusDetails = &StatusDetail{Message: "testing.T reported failure"}
			}
			a.scope.mu.Unlock()
		}
	}
	_ = a.scope.finish()
}

// ResultsDir returns the directory the writer is flushing to (useful for
// post-test assertions in unit tests of the writer itself).
func (a *AllureT) ResultsDir() string {
	if a == nil || a.scope == nil {
		return ""
	}
	if fw, ok := a.scope.writer.(*FileWriter); ok {
		return fw.Dir()
	}
	return ""
}

// UUID returns the scope's UUID so callers can locate the produced file.
func (a *AllureT) UUID() string {
	if a == nil || a.scope == nil {
		return ""
	}
	a.scope.mu.Lock()
	defer a.scope.mu.Unlock()
	return a.scope.result.UUID
}

// Step runs fn inside a step. See package-level [Step].
func (a *AllureT) Step(name string, fn func()) { Step(a.ctx, name, fn) }

// StepErr runs fn and captures its error as step status. See [StepErr].
func (a *AllureT) StepErr(name string, fn func() error) error {
	return StepErr(a.ctx, name, fn)
}

// Attachment captures an in-memory payload. See [Attachment].
func (a *AllureT) Attachment(name string, content []byte, mime string) {
	Attachment(a.ctx, name, content, mime)
}

// AttachFile captures a file from disk. See [AttachFile].
func (a *AllureT) AttachFile(name, path string, opts ...AttachOption) {
	AttachFile(a.ctx, name, path, opts...)
}

// AttachString is a convenience for plain-text attachments.
func (a *AllureT) AttachString(name, body string) {
	a.Attachment(name, []byte(body), "text/plain")
}

// AttachJSON marshals body using net/http.DetectContentType (after caller
// pre-serialises it) — this stub is a convenience for already-serialised
// JSON strings; pass raw bytes if marshalling is needed.
func (a *AllureT) AttachJSON(name string, raw []byte) {
	a.Attachment(name, raw, "application/json")
}

// Feature attaches the "feature" label.
func (a *AllureT) Feature(value string) { a.scope.addLabel(LabelFeature, value) }

// Story attaches the "story" label.
func (a *AllureT) Story(value string) { a.scope.addLabel(LabelStory, value) }

// Epic attaches the "epic" label.
func (a *AllureT) Epic(value string) { a.scope.addLabel(LabelEpic, value) }

// Severity attaches the "severity" label.
func (a *AllureT) Severity(s Severity) { a.scope.addLabel(LabelSeverity, string(s)) }

// Owner attaches the "owner" label.
func (a *AllureT) Owner(value string) { a.scope.addLabel(LabelOwner, value) }

// Tag attaches a "tag" label (call multiple times for multiple tags).
func (a *AllureT) Tag(values ...string) {
	for _, v := range values {
		a.scope.addLabel(LabelTag, v)
	}
}

// Label attaches an arbitrary key/value label.
func (a *AllureT) Label(name, value string) { a.scope.addLabel(name, value) }

// Description sets the test description.
func (a *AllureT) Description(value string) { a.scope.setDescription(value) }

// Title overrides the displayed test name.
func (a *AllureT) Title(value string) { a.scope.setTitle(value) }

// Issue attaches an "issue"-typed link.
func (a *AllureT) Issue(name, url string) {
	a.scope.addLink(AllureLink{Name: name, URL: url, Type: LinkTypeIssue})
}

// TmsLink attaches a "tms"-typed link.
func (a *AllureT) TmsLink(name, url string) {
	a.scope.addLink(AllureLink{Name: name, URL: url, Type: LinkTypeTMS})
}

// Link attaches a generic link.
func (a *AllureT) Link(name, url string) {
	a.scope.addLink(AllureLink{Name: name, URL: url, Type: LinkTypeGeneric})
}

// Parameter adds a name/value parameter to the current step (or test).
func (a *AllureT) Parameter(name, value string) { a.scope.addParameter(name, value) }
