// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Tester is a fluent test builder. Construct one via New and then chain
// protocol facets. Errors from assertions are accumulated on the Tester
// itself — call OK / Errors / Report at the end of the test.
//
// Tester is single-goroutine by contract. Fan-out belongs to higher-level
// helpers (a .Parallel(...) facet is on the roadmap).
type Tester struct {
	ctx      context.Context
	http     *http.Client
	baseURL  string
	headers  http.Header
	vars     map[string]string
	failFast bool

	mu      sync.Mutex
	steps   []StepRecord
	errs    []error
	pending committable
}

// committable is what flushPending knows about a step: the only thing
// it needs to do is commit the previous chain. Each protocol step
// satisfies this trivially via its commit method.
type committable interface {
	commit()
}

// StepRecord is the in-memory record of one executed step. It is also
// what Report returns; downstream code can serialise to Allure or to
// Mockarty's extended Run shape.
type StepRecord struct {
	StartedAt    time.Time
	EndedAt      time.Time
	Name         string
	Protocol     string
	Method       string
	URL          string
	StatusOrCode int
	Failures     []string
}

// Option configures a Tester.
type Option func(*Tester)

// WithContext binds a parent context. If the context carries an Allure
// scope (see github.com/mockarty/mockarty-go/allure) every step is
// recorded into the scope automatically.
func WithContext(ctx context.Context) Option { return func(t *Tester) { t.ctx = ctx } }

// WithBaseURL sets the base URL prefixed to every relative path used in
// HTTP facet calls. Pass full URLs (with scheme) to override per-call.
func WithBaseURL(u string) Option { return func(t *Tester) { t.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient overrides the default *http.Client. Use this to plug in
// a custom transport (mTLS, retry, proxy).
func WithHTTPClient(c *http.Client) Option { return func(t *Tester) { t.http = c } }

// WithHeader adds a default header applied to every HTTP request unless
// the per-step Header() call overrides it.
func WithHeader(k, v string) Option {
	return func(t *Tester) {
		if t.headers == nil {
			t.headers = http.Header{}
		}
		t.headers.Set(k, v)
	}
}

// WithFailFast stops chained execution at the first failing step. By
// default the Tester runs every step and accumulates all failures so the
// reporter can show a complete picture.
func WithFailFast() Option { return func(t *Tester) { t.failFast = true } }

// New constructs a Tester ready for chaining. The default HTTP client has
// a 30s timeout and follows redirects.
func New(opts ...Option) *Tester {
	t := &Tester{
		ctx:     context.Background(),
		http:    &http.Client{Timeout: 30 * time.Second},
		vars:    map[string]string{},
		headers: http.Header{},
	}
	for _, o := range opts {
		o(t)
	}
	if t.http == nil {
		t.http = &http.Client{Timeout: 30 * time.Second}
	}
	return t
}

// HTTP returns the HTTP facet. Each call returns a fresh facet so chaining
// multiple HTTP calls works naturally.
func (t *Tester) HTTP() *HTTPFacet { return &HTTPFacet{t: t} }

// Vars returns the live variable store. Test code can pre-populate
// variables (e.g. WithVar before chains start).
func (t *Tester) Vars() map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := make(map[string]string, len(t.vars))
	for k, v := range t.vars {
		cp[k] = v
	}
	return cp
}

// SetVar stores a variable usable as {{name}} in subsequent requests.
func (t *Tester) SetVar(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.vars[name] = value
}

// OK reports whether every executed step passed.
func (t *Tester) OK() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.errs) == 0
}

// Errors returns a copy of accumulated step failures.
func (t *Tester) Errors() []error {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]error, len(t.errs))
	copy(out, t.errs)
	return out
}

// Report returns a snapshot of every executed step. Useful for emitting
// the Mockarty extended Run shape or feeding a custom reporter.
func (t *Tester) Report() []StepRecord {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]StepRecord, len(t.steps))
	copy(out, t.steps)
	return out
}

// recordStep is the single write-path for step results. It is called
// after the step has completed (success or failure).
func (t *Tester) recordStep(rec StepRecord) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, rec)
	for _, f := range rec.Failures {
		t.errs = append(t.errs, fmt.Errorf("%s: %s", rec.Name, f))
	}
}

// shouldAbort returns true when failFast is on and at least one error
// has already been recorded. Subsequent chained steps short-circuit.
func (t *Tester) shouldAbort() bool {
	if !t.failFast {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.errs) > 0
}

// snapshotVars returns a read-only snapshot of the variable store for
// use inside step builders. Snapshot semantics keep concurrent reads
// race-free even though Tester is single-goroutine by contract.
func (t *Tester) snapshotVars() map[string]string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := make(map[string]string, len(t.vars))
	for k, v := range t.vars {
		cp[k] = v
	}
	return cp
}

// setPending stores the chain step as pending so the next chain start
// can flush it. Tester only tracks one pending step at a time — chains
// are linear, not concurrent.
func (t *Tester) setPending(s committable) {
	t.mu.Lock()
	t.pending = s
	t.mu.Unlock()
}

// clearPending removes s from the pending slot if it is still there.
// Called from Step.Done so the explicit terminator does not commit
// twice on a subsequent flushPending.
func (t *Tester) clearPending(s committable) {
	t.mu.Lock()
	if t.pending == s {
		t.pending = nil
	}
	t.mu.Unlock()
}

// flushPending commits the previous chain step (if any). Called from
// every facet entry so back-to-back chains commit naturally.
func (t *Tester) flushPending() {
	t.mu.Lock()
	p := t.pending
	t.pending = nil
	t.mu.Unlock()
	if p != nil {
		p.commit()
	}
}

// Finish commits any in-flight chain step. Call at the end of a test so
// the final record lands in the report.
func (t *Tester) Finish() *Tester { t.flushPending(); return t }
