// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import "sync"

// Parameterized tests stash their AllureParameter list keyed on
// testing.T.Name() before invoking the user function, then T() reads it
// to seed cfg.parameters. This is the cleanest way to flow data from a
// runner (which sets up the subtest) to the AllureT constructor (which
// the user code invokes) without changing the public T() signature.

var (
	pendingMu     sync.RWMutex
	pendingParams = map[string][]AllureParameter{}
)

func pushPendingParameters(testName string, params []AllureParameter) {
	if testName == "" || len(params) == 0 {
		return
	}
	pendingMu.Lock()
	pendingParams[testName] = params
	pendingMu.Unlock()
}

func popPendingParameters(testName string) {
	if testName == "" {
		return
	}
	pendingMu.Lock()
	delete(pendingParams, testName)
	pendingMu.Unlock()
}

func consumePendingParameters(testName string) []AllureParameter {
	if testName == "" {
		return nil
	}
	pendingMu.RLock()
	defer pendingMu.RUnlock()
	cur := pendingParams[testName]
	if len(cur) == 0 {
		return nil
	}
	out := make([]AllureParameter, len(cur))
	copy(out, cur)
	return out
}
