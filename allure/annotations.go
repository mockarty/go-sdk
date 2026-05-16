// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
)

// WithTest opens a new test scope and returns a context carrying it plus a
// finish function the caller must invoke (typically via `defer`) to flush
// the result.
//
// This is the context-based entry point — package-level annotation funcs
// (Step, Feature, ...) read the scope from the context.
//
// Example:
//
//	ctx, finish := allure.WithTest(context.Background(), "Login")
//	defer finish()
//	allure.Feature(ctx, "Auth")
//	allure.Step(ctx, "submit", func() { ... })
func WithTest(ctx context.Context, name string, opts ...Option) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := config{name: name}
	for _, o := range opts {
		o(&cfg)
	}
	dir := ResolveResultsDir(cfg.resultsDir)
	writer := NewFileWriter(dir)
	s := newScope(cfg, writer)
	return withScope(ctx, s), func() { _ = s.finish() }
}

// withScopeOrNoop returns scope from ctx; nil-safe variants of annotation
// funcs degrade to no-ops when there is no scope (fail-soft principle).
func withScopeOrNoop(ctx context.Context) *scope { return fromContext(ctx) }

// Feature attaches the "feature" label.
func Feature(ctx context.Context, value string) {
	withScopeOrNoop(ctx).addLabel(LabelFeature, value)
}

// Story attaches the "story" label.
func Story(ctx context.Context, value string) {
	withScopeOrNoop(ctx).addLabel(LabelStory, value)
}

// Epic attaches the "epic" label.
func Epic(ctx context.Context, value string) {
	withScopeOrNoop(ctx).addLabel(LabelEpic, value)
}

// SeverityLevel attaches the "severity" label.
func SeverityLevel(ctx context.Context, s Severity) {
	withScopeOrNoop(ctx).addLabel(LabelSeverity, string(s))
}

// Owner attaches the "owner" label.
func Owner(ctx context.Context, value string) {
	withScopeOrNoop(ctx).addLabel(LabelOwner, value)
}

// Tag attaches a "tag" label (call multiple times for multiple tags).
func Tag(ctx context.Context, value string) {
	withScopeOrNoop(ctx).addLabel(LabelTag, value)
}

// Label attaches an arbitrary key/value label.
func Label(ctx context.Context, name, value string) {
	withScopeOrNoop(ctx).addLabel(name, value)
}

// Description sets the test description (plain text). Replaces any prior
// description on the same scope.
func Description(ctx context.Context, value string) {
	withScopeOrNoop(ctx).setDescription(value)
}

// Title overrides the displayed test name.
func Title(ctx context.Context, value string) {
	withScopeOrNoop(ctx).setTitle(value)
}

// Issue attaches an "issue"-typed link.
func Issue(ctx context.Context, name, url string) {
	withScopeOrNoop(ctx).addLink(AllureLink{Name: name, URL: url, Type: LinkTypeIssue})
}

// TmsLink attaches a "tms"-typed link.
func TmsLink(ctx context.Context, name, url string) {
	withScopeOrNoop(ctx).addLink(AllureLink{Name: name, URL: url, Type: LinkTypeTMS})
}

// LinkURL attaches a generic link.
func LinkURL(ctx context.Context, name, url string) {
	withScopeOrNoop(ctx).addLink(AllureLink{Name: name, URL: url, Type: LinkTypeGeneric})
}

// Parameter adds a name/value parameter to the current step (or the test
// header if no step is open).
func Parameter(ctx context.Context, name, value string) {
	withScopeOrNoop(ctx).addParameter(name, value)
}
