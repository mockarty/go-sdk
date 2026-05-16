// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"fmt"
	"runtime/debug"
)

// Step runs fn inside an Allure step named `name`. The step's start/stop
// timestamps are captured automatically. If fn panics, the step is marked
// `broken` with the panic message + stack trace and the panic is re-raised.
//
// Empty name is normalised to "(unnamed step)" so the Allure UI does not
// render a blank row.
//
// Step calls are reentrant — nesting is recorded as child steps in the
// produced JSON.
func Step(ctx context.Context, name string, fn func()) {
	if name == "" {
		name = "(unnamed step)"
	}
	s := fromContext(ctx)
	if s == nil {
		// No scope = caller did not call WithTest. Fail-soft: still run fn so
		// suites that omit annotation setup do not break.
		if fn != nil {
			fn()
		}
		return
	}
	s.pushStep(name)
	defer func() {
		if r := recover(); r != nil {
			s.popStep(StatusBroken, &StatusDetail{
				Message: fmt.Sprintf("%v", r),
				Trace:   string(debug.Stack()),
			})
			panic(r)
		}
		s.popStep("", nil)
	}()
	if fn != nil {
		fn()
	}
}

// StepErr runs fn and records the step status from its returned error:
// nil error = passed, non-nil = failed (with the error's Error() as the
// status message). Useful for tests that already use idiomatic Go error
// returns and prefer to delegate failure reporting.
func StepErr(ctx context.Context, name string, fn func() error) error {
	if name == "" {
		name = "(unnamed step)"
	}
	s := fromContext(ctx)
	if s == nil {
		if fn != nil {
			return fn()
		}
		return nil
	}
	s.pushStep(name)
	var err error
	defer func() {
		if r := recover(); r != nil {
			s.popStep(StatusBroken, &StatusDetail{
				Message: fmt.Sprintf("%v", r),
				Trace:   string(debug.Stack()),
			})
			panic(r)
		}
		if err != nil {
			s.popStep(StatusFailed, &StatusDetail{Message: err.Error()})
			return
		}
		s.popStep("", nil)
	}()
	if fn != nil {
		err = fn()
	}
	return err
}

// StepHandle is the manual step API for callers who cannot wrap their work
// in a closure (e.g. integrating with an existing framework that owns the
// control flow). Get one via [BeginStep], close it via [StepHandle.End].
type StepHandle struct {
	scope *scope
	name  string
}

// BeginStep opens a step and returns a handle that must be closed via End.
// Use Step(ctx, name, func(){...}) when possible — manual handles are only
// for control-flow that cannot use closures.
func BeginStep(ctx context.Context, name string) *StepHandle {
	if name == "" {
		name = "(unnamed step)"
	}
	s := fromContext(ctx)
	if s == nil {
		return &StepHandle{name: name}
	}
	s.pushStep(name)
	return &StepHandle{scope: s, name: name}
}

// End closes the step as passed.
func (h *StepHandle) End() {
	if h == nil || h.scope == nil {
		return
	}
	h.scope.popStep("", nil)
	h.scope = nil
}

// Fail closes the step as failed with the given message.
func (h *StepHandle) Fail(msg string) {
	if h == nil || h.scope == nil {
		return
	}
	h.scope.popStep(StatusFailed, &StatusDetail{Message: msg})
	h.scope = nil
}

// Broken closes the step as broken (unexpected error).
func (h *StepHandle) Broken(msg, trace string) {
	if h == nil || h.scope == nil {
		return
	}
	h.scope.popStep(StatusBroken, &StatusDetail{Message: msg, Trace: trace})
	h.scope = nil
}

// Skip closes the step as skipped.
func (h *StepHandle) Skip(reason string) {
	if h == nil || h.scope == nil {
		return
	}
	h.scope.popStep(StatusSkipped, &StatusDetail{Message: reason})
	h.scope = nil
}
