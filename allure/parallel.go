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
// a sync.WaitGroup. Branches MAY panic — Step (one level down) already
// marks the panicking child step as "broken" and re-raises the panic.
// We swallow that re-raised panic here so a single branch failure does
// NOT crash the test, while keeping the original "broken" step the user
// can see in the Allure report.
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
				// Swallow any panic re-raised by Step's inner defer. Step has
				// ALREADY popped the panicking child step as StatusBroken with
				// the actual panic message + stack trace — recording a second
				// "(panicked)" step here would duplicate the failure in the
				// Allure report (two child rows for one event, the second
				// carrying only a generic "branch panicked" string).
				defer func() { _ = recover() }()
				Step(ctx, name, fn)
			}()
		}
		wg.Wait()
	})
}
