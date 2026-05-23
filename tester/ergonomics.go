// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"sync"
	"time"

	"github.com/mockarty/mockarty-go/allure"
)

// Wrap runs fn inside a parent Allure step named `name`. Any chains
// fired from inside fn record as child steps of the parent.
//
// Example:
//
//	t.Wrap("user signup flow", func() {
//	    t.HTTP().POST("/signup").JSON(...).ExpectStatus(201)
//	    t.HTTP().POST("/login").JSON(...).ExpectStatus(200).Extract("$.token", "token")
//	})
//
// Wrap is idempotent w.r.t. flushPending — the pending step from the
// previous chain commits before fn runs, and any pending step left
// over inside fn flushes when fn returns.
func (t *Tester) Wrap(name string, fn func()) *Tester {
	t.flushPending()
	if fn == nil {
		return t
	}
	handle := allure.BeginStep(t.ctx, name)
	defer func() {
		t.flushPending()
		if r := recover(); r != nil {
			handle.Broken("panic in Wrap", "")
			panic(r)
		}
		handle.End()
	}()
	fn()
	return t
}

// Eventually retries fn until it returns nil or `within` elapses,
// sleeping `interval` between attempts. When zero, interval defaults
// to 100ms.
//
// fn is intended to encapsulate one or more Tester chains; the final
// step record set reflects the LAST attempt only (intermediate failures
// are dropped to keep the report readable).
//
// Returns true when fn succeeded, false on timeout.
func (t *Tester) Eventually(within, interval time.Duration, fn func() error) bool {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	deadline := time.Now().Add(within)
	var lastErr error
	for {
		// Snapshot the current step / error tail so we can roll back
		// after a failing attempt — Eventually's contract is "only the
		// successful run's steps appear in the report".
		t.flushPending()
		t.mu.Lock()
		stepBookmark := len(t.steps)
		errBookmark := len(t.errs)
		t.mu.Unlock()

		err := fn()
		t.flushPending()

		if err == nil {
			t.mu.Lock()
			// Successful run: drop any failures recorded by the chain
			// during the attempt (they were on the way to success).
			t.errs = t.errs[:errBookmark]
			t.mu.Unlock()
			return true
		}
		lastErr = err
		// Roll back to bookmarks so a subsequent attempt starts clean.
		t.mu.Lock()
		t.steps = t.steps[:stepBookmark]
		t.errs = t.errs[:errBookmark]
		t.mu.Unlock()

		if time.Now().After(deadline) {
			// Final failure: re-run fn one last time so the
			// report shows what eventually broke.
			t.flushPending()
			t.mu.Lock()
			t.steps = t.steps[:stepBookmark]
			t.errs = t.errs[:errBookmark]
			t.mu.Unlock()
			_ = fn()
			t.flushPending()
			if lastErr != nil {
				t.mu.Lock()
				t.errs = append(t.errs, lastErr)
				t.mu.Unlock()
			}
			return false
		}
		time.Sleep(interval)
	}
}

// Parallel runs each fn in its own goroutine, each with a branch-local
// Tester that shares the parent's transport/baseURL/headers/var-store
// but has its OWN pending-step slot. After all branches complete their
// step records and errors are merged back into the parent in the order
// the branches were given.
//
//	t.Parallel(
//	  func(t *tester.Tester) { t.HTTP().GET("/a").ExpectStatus(200) },
//	  func(t *tester.Tester) { t.HTTP().GET("/b").ExpectStatus(200) },
//	)
//
// Variable writes from a branch do NOT propagate back to the parent
// or sibling branches — each branch sees a snapshot of the parent's
// var store at spawn time and writes are isolated. Branches that need
// to share state across the fan-out must coordinate externally
// (channels, shared maps with their own locks, etc).
//
// Step ordering inside each branch is preserved; the merge into the
// parent report is deterministic (branch 0's steps, then branch 1's,
// regardless of which branch finished first).
func (t *Tester) Parallel(fns ...func(*Tester)) *Tester {
	if len(fns) == 0 {
		return t
	}
	t.flushPending()
	var wg sync.WaitGroup
	results := make([][]StepRecord, len(fns))
	errs := make([][]error, len(fns))
	wg.Add(len(fns))
	for i, fn := range fns {
		i, fn := i, fn
		go func() {
			defer wg.Done()
			if fn == nil {
				return
			}
			branch := t.spawnBranch()
			fn(branch)
			branch.flushPending()
			branch.mu.Lock()
			results[i] = append(results[i], branch.steps...)
			errs[i] = append(errs[i], branch.errs...)
			branch.mu.Unlock()
		}()
	}
	wg.Wait()

	t.mu.Lock()
	for _, r := range results {
		t.steps = append(t.steps, r...)
	}
	for _, e := range errs {
		t.errs = append(t.errs, e...)
	}
	t.mu.Unlock()
	return t
}

// spawnBranch makes a child Tester that copies the parent's transport
// + baseURL + default headers + var store snapshot. Each branch's
// SetVar / chain state is isolated — concurrent branches do not see
// each other's variable writes. Branches that need to share state
// must coordinate externally.
//
// Step records and errors are gathered by Parallel and merged into the
// parent atomically after all branches complete.
func (t *Tester) spawnBranch() *Tester {
	t.mu.Lock()
	varCopy := make(map[string]string, len(t.vars))
	for k, v := range t.vars {
		varCopy[k] = v
	}
	t.mu.Unlock()
	return &Tester{
		ctx:      t.ctx,
		http:     t.http,
		baseURL:  t.baseURL,
		headers:  t.headers,
		vars:     varCopy,
		failFast: t.failFast,
	}
}
