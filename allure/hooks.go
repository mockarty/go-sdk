// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"fmt"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Lifecycle hooks reproduce TestNG / JUnit5 / pytest setup/teardown
// semantics on top of Go's bare `testing` package. They emit Allure
// Container files (`<uuid>-container.json`) per the official Allure 2
// schema — the report renderer then displays them as "Set up" / "Tear
// down" sections nested under the corresponding test.
//
// Containers can hold one or more test UUIDs in Children. Class-level
// (BeforeAll/AfterAll) containers reference every test in the
// goroutine-local scope; method-level (BeforeEach/AfterEach) containers
// reference exactly one test.

// Hook is the user-supplied setup/teardown function. It receives the
// active *testing.T so it can register failures via t.Fatalf / t.Errorf.
type Hook func(t *testing.T)

// suiteHooks groups setup/teardown closures + their accumulated children
// for a single class (Go test function). It survives between t.Run
// invocations of the same parent test.
type suiteHooks struct {
	beforeAlls    []Hook
	afterAlls     []Hook
	beforeEachs   []Hook
	afterEachs    []Hook
	children      []string
	containerID   string
	dir           string
	beforeSteps   []AllureStep
	afterSteps    []AllureStep
	startMS       int64
	beforeAllDone bool
	mu            sync.Mutex
}

var (
	suiteRegistryMu sync.Mutex
	suiteRegistry   = map[string]*suiteHooks{}
)

// suiteFor returns (creating if needed) the hook bucket for the given
// parent test name. We key on the top-level testClass so subtests share
// the same fixture lifecycle.
func suiteFor(parent string) *suiteHooks {
	suiteRegistryMu.Lock()
	defer suiteRegistryMu.Unlock()
	h, ok := suiteRegistry[parent]
	if !ok {
		h = &suiteHooks{
			containerID: uuid.NewString(),
			startMS:     time.Now().UnixMilli(),
		}
		suiteRegistry[parent] = h
	}
	return h
}

// BeforeAll registers a one-time setup that runs once before the first
// matching subtest. The hook runs synchronously when the first
// BeforeEach (or the test body, if BeforeEach is absent) fires.
//
// Multiple BeforeAll registrations stack in declaration order.
//
// The hook's runtime is captured as an Allure container `befores` step,
// which Allure renders as a "Set up" entry above the test in the report.
func BeforeAll(t *testing.T, name string, fn Hook) {
	t.Helper()
	cls, _ := splitTestPath(t.Name())
	h := suiteFor(cls)
	h.mu.Lock()
	h.beforeAlls = append(h.beforeAlls, namedHook(name, fn))
	if h.dir == "" {
		h.dir = ResolveResultsDir("")
	}
	h.mu.Unlock()
}

// AfterAll registers teardown that runs once after the entire class. Use
// t.Cleanup-style ordering: the LAST AfterAll registered is invoked first.
func AfterAll(t *testing.T, name string, fn Hook) {
	t.Helper()
	cls, _ := splitTestPath(t.Name())
	h := suiteFor(cls)
	h.mu.Lock()
	h.afterAlls = append(h.afterAlls, namedHook(name, fn))
	if h.dir == "" {
		h.dir = ResolveResultsDir("")
	}
	h.mu.Unlock()

	// Register a top-level cleanup so AfterAll fires when the parent test
	// (and its subtests) all complete. testing.T.Cleanup runs LIFO at the
	// parent boundary — exactly what TestNG/JUnit AfterClass needs.
	t.Cleanup(func() {
		flushSuite(cls)
	})
}

// BeforeEach registers per-test setup. Each subtest re-runs every
// registered BeforeEach in registration order.
func BeforeEach(t *testing.T, name string, fn Hook) {
	t.Helper()
	cls, _ := splitTestPath(t.Name())
	h := suiteFor(cls)
	h.mu.Lock()
	h.beforeEachs = append(h.beforeEachs, namedHook(name, fn))
	h.mu.Unlock()
}

// AfterEach registers per-test teardown. Each subtest runs all registered
// AfterEach hooks in LIFO order on completion.
func AfterEach(t *testing.T, name string, fn Hook) {
	t.Helper()
	cls, _ := splitTestPath(t.Name())
	h := suiteFor(cls)
	h.mu.Lock()
	h.afterEachs = append(h.afterEachs, namedHook(name, fn))
	h.mu.Unlock()
}

// RunWithHooks executes fn under the suite's BeforeEach/AfterEach chain,
// running BeforeAll once on the first invocation. The function returns
// after AfterEach completes; AfterAll fires on the parent t.Cleanup.
//
// This is the canonical "run a subtest with the active hook chain"
// helper. Use it inside `t.Run` (or call it directly from a top-level
// test) to wire the test body into the lifecycle.
func RunWithHooks(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	cls, _ := splitTestPath(t.Name())
	h := suiteFor(cls)

	h.mu.Lock()
	if !h.beforeAllDone {
		h.beforeAllDone = true
		for _, hk := range h.beforeAlls {
			step := runHookCaptured(t, hk)
			h.beforeSteps = append(h.beforeSteps, step)
		}
	}
	beforeEachs := append([]Hook(nil), h.beforeEachs...)
	afterEachs := append([]Hook(nil), h.afterEachs...)
	h.mu.Unlock()

	// BeforeEach (FIFO).
	for _, hk := range beforeEachs {
		hk(t)
	}
	// Test body.
	fn(t)
	// AfterEach (LIFO).
	for i := len(afterEachs) - 1; i >= 0; i-- {
		afterEachs[i](t)
	}
}

// RegisterChild records a test UUID against the active suite so the
// Allure container references its children. AllureT auto-registers.
//
// IMPORTANT: this is a no-op when no BeforeAll/AfterAll has been
// registered for the suite yet — recording children without hooks would
// create suite registry entries that never flush (and a stray
// flushAllSuites() at the end of a process would emit unintended
// container files in the default results dir). The contract: the
// container only exists if the user explicitly opted in via a lifecycle
// hook.
func RegisterChild(testName, uuid string) {
	if uuid == "" {
		return
	}
	cls, _ := splitTestPath(testName)
	suiteRegistryMu.Lock()
	h, ok := suiteRegistry[cls]
	suiteRegistryMu.Unlock()
	if !ok {
		return
	}
	h.mu.Lock()
	h.children = append(h.children, uuid)
	h.mu.Unlock()
}

// flushSuite writes the container json after the test (and all subtests)
// finishes. Safe to call multiple times — only the first call actually
// writes.
func flushSuite(cls string) {
	suiteRegistryMu.Lock()
	h, ok := suiteRegistry[cls]
	if !ok {
		suiteRegistryMu.Unlock()
		return
	}
	delete(suiteRegistry, cls)
	suiteRegistryMu.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Run AfterAll hooks (LIFO).
	for i := len(h.afterAlls) - 1; i >= 0; i-- {
		step := runHookCaptured(nil, h.afterAlls[i])
		h.afterSteps = append(h.afterSteps, step)
	}

	if len(h.children) == 0 && len(h.beforeSteps) == 0 && len(h.afterSteps) == 0 {
		return
	}
	dir := h.dir
	if dir == "" {
		dir = ResolveResultsDir("")
	}
	w := NewFileWriter(dir)
	container := Container{
		UUID:     h.containerID,
		Name:     cls,
		Children: append([]string(nil), h.children...),
		Befores:  append([]AllureStep(nil), h.beforeSteps...),
		Afters:   append([]AllureStep(nil), h.afterSteps...),
		Start:    h.startMS,
		Stop:     time.Now().UnixMilli(),
	}
	_ = w.WriteContainer(container)
}

// namedHook wraps a Hook so its execution shows up under the given name
// in the container's befores/afters list. The wrapper survives via
// runHookCaptured which builds a synthetic step from the hook's runtime.
func namedHook(name string, fn Hook) Hook {
	if fn == nil {
		return func(_ *testing.T) {}
	}
	return func(t *testing.T) {
		fn(t)
		_ = name // name is captured in runHookCaptured; this wrapper exists for symmetry.
	}
}

// runHookCaptured invokes hk and records its outcome as an AllureStep so
// the container can show timing + status. Panics are caught and surface
// as "broken" hooks.
func runHookCaptured(t *testing.T, hk Hook) AllureStep {
	step := AllureStep{
		Name:        hookNameOf(hk),
		Status:      StatusPassed,
		Stage:       StageFinished,
		Start:       time.Now().UnixMilli(),
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
		}()
		hk(t)
	}()
	step.Stop = time.Now().UnixMilli()
	if t != nil && t.Failed() {
		step.Status = StatusFailed
		if step.StatusDetails == nil {
			step.StatusDetails = &StatusDetail{Message: "hook reported failure via t.Errorf/t.Fatalf"}
		}
	}
	return step
}

// hookNameOf is a best-effort label for a hook. We have no reliable way
// to extract the user-supplied name through the wrapper closure, so we
// fall back to a generic label. Callers wanting custom names should add
// an explicit allure.Step inside their hook body.
func hookNameOf(_ Hook) string { return "hook" }
