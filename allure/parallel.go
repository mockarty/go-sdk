// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
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
// ParallelStep is RACE-SAFE: each branch's pushStep/popStep call goes
// through the scope mutex, and we serialise the per-branch completion via
// a sync.WaitGroup. The branches MUST NOT panic — panics inside a branch
// are caught and marked "broken" on that branch's step (the panic does
// not crash the test).
func ParallelStep(ctx context.Context, parent string, branches map[string]func()) {
	if parent == "" {
		parent = "(unnamed parallel step)"
	}
	if len(branches) == 0 {
		Step(ctx, parent, nil)
		return
	}
	Step(ctx, parent, func() {
		var wg sync.WaitGroup
		for name, fn := range branches {
			name, fn := name, fn
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					// Swallow panics inside a branch: the wrapping Step (one level
					// up) will not see them, so we record an explicit "broken"
					// child step.
					if r := recover(); r != nil {
						s := fromContext(ctx)
						if s == nil {
							return
						}
						st := s.pushStep(name + " (panicked)")
						_ = st
						s.popStep(StatusBroken, &StatusDetail{
							Message: "branch panicked",
						})
					}
				}()
				Step(ctx, name, fn)
			}()
		}
		wg.Wait()
	})
}
