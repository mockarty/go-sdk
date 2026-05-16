// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import "time"

// Option mutates the test scope at construction time.
//
// Options are functional (composition-first per Mockarty's architecture
// principles) so the surface is open for extension without breaking the
// constructor signature.
type Option func(*config)

type config struct {
	now        func() time.Time
	resultsDir string
	name       string
	fullName   string
	suite      string
	feature    string
	story      string
	epic       string
	owner      string
	severity   Severity
	labels     []AllureLabel
	links      []AllureLink
	parameters []AllureParameter
}

// WithResultsDir overrides the output directory for this test only.
//
// Without it the writer uses ALLURE_RESULTS_DIR or "./allure-results".
func WithResultsDir(dir string) Option { return func(c *config) { c.resultsDir = dir } }

// WithName overrides the displayed test name (defaults to the testing.T name
// or whatever WithTest receives).
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithFullName sets the fully qualified test name, used by Allure's
// history-aware reports to disambiguate same-named tests in different
// packages.
func WithFullName(fullName string) Option { return func(c *config) { c.fullName = fullName } }

// WithSuite seeds the "suite" label.
func WithSuite(s string) Option { return func(c *config) { c.suite = s } }

// WithFeature seeds the "feature" label.
func WithFeature(f string) Option { return func(c *config) { c.feature = f } }

// WithStory seeds the "story" label.
func WithStory(s string) Option { return func(c *config) { c.story = s } }

// WithEpic seeds the "epic" label.
func WithEpic(e string) Option { return func(c *config) { c.epic = e } }

// WithOwner seeds the "owner" label.
func WithOwner(o string) Option { return func(c *config) { c.owner = o } }

// WithSeverity seeds the "severity" label.
func WithSeverity(s Severity) Option { return func(c *config) { c.severity = s } }

// WithLabel adds an arbitrary label.
func WithLabel(name, value string) Option {
	return func(c *config) {
		c.labels = append(c.labels, AllureLabel{Name: name, Value: value})
	}
}

// WithLink adds a generic link.
func WithLink(name, url string) Option {
	return func(c *config) {
		c.links = append(c.links, AllureLink{Name: name, URL: url, Type: LinkTypeGeneric})
	}
}

// WithIssue adds an "issue"-typed link (renders with the bug-tracker badge).
func WithIssue(name, url string) Option {
	return func(c *config) {
		c.links = append(c.links, AllureLink{Name: name, URL: url, Type: LinkTypeIssue})
	}
}

// WithTmsLink adds a "tms"-typed link (renders with the TMS badge).
func WithTmsLink(name, url string) Option {
	return func(c *config) {
		c.links = append(c.links, AllureLink{Name: name, URL: url, Type: LinkTypeTMS})
	}
}

// WithParameter adds a parameter shown in the test header.
func WithParameter(name, value string) Option {
	return func(c *config) {
		c.parameters = append(c.parameters, AllureParameter{Name: name, Value: value})
	}
}

// WithClock injects a deterministic clock — used by tests that need
// reproducible Start/Stop timestamps. Production callers should leave it
// unset (default = time.Now).
func WithClock(now func() time.Time) Option { return func(c *config) { c.now = now } }
