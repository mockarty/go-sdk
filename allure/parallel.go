// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"fmt"
	"runtime/debug"

	"context"
	"sync"
)

// ParallelStep runs each entry of branches concurrently inside its own
// child step. Status bubbles up to the parent following the standard
// priority (broken > failed > skipped > passed).
//
// Use it when a test legitimately fans out across goroutines — e.g.
// concurrent API calls under a single logical assertion. The Allure
// renderer will show one parent step with N children, each annotated
// with its goroutine ID (via the "thread" label inherited from the scope).
//
// Implementation note: parallel branches CANNOT use the scope's normal
// pushStep/popStep mechanism — that machinery walks a single shared
// stepStack to determine the current parent, and concurrent pushes from
// N goroutines would non-deterministically nest into each other (the
// goroutine that pushes second sees the first goroutine's still-open
// step as "current parent"). Instead we run each branch in isolation,
// then collect the produced step records into the parent step's Steps
// slice under the scope lock. Panics in branches are caught and recorded
// as broken with the real panic message + stack trace.
func ParallelStep(ctx context.Context, parent string, branches map[string]func()) {
	if parent == "" {
		parent = "(unnamed parallel step)"
	}
	if len(branches) == 0 {
		Step(ctx, parent, nil)
		return
	}
	Step(ctx, parent, func() {
		s := fromContext(ctx)
		if s == nil {
			// No scope = run the branches anyway so user code executes.
			runBranchesNoScope(branches)
			return
		}

		// Build each branch's result step in isolation (no shared state) and
		// merge under the scope lock at the end. This avoids the stepStack
		// nesting race entirely.
		var (
			wg      sync.WaitGroup
			produce = make(chan AllureStep, len(branches))
		)
		for name, fn := range branches {
			name, fn := name, fn
			wg.Add(1)
			go func() {
				defer wg.Done()
				produce <- runIsolatedBranch(s, name, fn)
			}()
		}
		wg.Wait()
		close(produce)

		// Attach all produced steps as children of the current parent step
		// (which Step has open for us). We do this under s.mu to avoid
		// stomping on concurrent step writes from outside ParallelStep.
		s.mu.Lock()
		defer s.mu.Unlock()
		parentStep := s.currentStepLocked()
		for ch := range produce {
			// Bubble strongest child failure into the parent + result.
			if statusPriority(ch.Status) > statusPriority(parentStep.Status) {
				parentStep.Status = ch.Status
				if ch.StatusDetails != nil && parentStep.StatusDetails == nil {
					parentStep.StatusDetails = ch.StatusDetails
				}
			}
			if parentStep != nil {
				parentStep.Steps = append(parentStep.Steps, ch)
			} else {
				s.result.Steps = append(s.result.Steps, ch)
			}
			if statusPriority(ch.Status) > statusPriority(s.result.Status) {
				s.result.Status = ch.Status
				if ch.StatusDetails != nil && s.result.StatusDetails == nil {
					s.result.StatusDetails = ch.StatusDetails
				}
			}
		}
	})
}

// runIsolatedBranch executes one branch in its own logical step and
// returns the captured AllureStep. Panics are caught and rendered as
// StatusBroken with the real message + stack — matching what Step does
// for the sequential path.
func runIsolatedBranch(s *scope, name string, fn func()) AllureStep {
	step := AllureStep{
		Name:        name,
		Status:      StatusPassed,
		Stage:       StageRunning,
		Start:       s.now().UnixMilli(),
		Parameters:  []AllureParameter{},
		Steps:       []AllureStep{},
		Attachments: []AllureAttachment{},
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				step.Status = StatusBroken
				step.StatusDetails = &StatusDetail{
					Message: fmt.Sprintf("%v", r),
					Trace:   string(debug.Stack()),
				}
			}
			step.Stop = s.now().UnixMilli()
			step.Stage = StageFinished
		}()
		if fn != nil {
			fn()
		}
	}()
	return step
}

// runBranchesNoScope is the fallback path when ParallelStep is called
// outside a WithTest scope (degraded mode). The branches still run but
// no step records are produced.
func runBranchesNoScope(branches map[string]func()) {
	var wg sync.WaitGroup
	for _, fn := range branches {
		fn := fn
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { _ = recover() }()
			if fn != nil {
				fn()
			}
		}()
	}
	wg.Wait()
}
