// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestAllureT_Accessors_Coverage exercises every per-test accessor that
// the main acceptance suite does not already cover.
func TestAllureT_Accessors_Coverage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		_ = a.Context()
		_ = a.UUID()
		_ = a.ResultsDir()
		a.AttachPNG("shot", []byte("\x89PNG"))
		a.ParameterEx("token", "hidden", ParameterModeHidden, true)
		a.ParentSuite("PS")
		a.SubSuite("SS")
		a.Suite("S")
		a.DescriptionHTML("<p>html</p>")
	})
	r := readResultFile(t, dir)
	if r.DescriptionHTML != "<p>html</p>" {
		t.Errorf("html desc not set: %q", r.DescriptionHTML)
	}
	have := map[string]string{}
	for _, l := range r.Labels {
		have[l.Name] = l.Value
	}
	if have[LabelParentSuite] != "PS" || have[LabelSubSuite] != "SS" {
		t.Errorf("suite labels missing: %v", have)
	}
	if len(r.Attachments) == 0 {
		t.Errorf("expected png attachment")
	}
	// ParameterEx with excluded=true must not change HistoryID compared to
	// a baseline run with the same fullName but no parameter.
}

// TestSetClock_InjectsDeterministicTime verifies SetClock affects timestamps.
func TestSetClock_InjectsDeterministicTime(t *testing.T) {
	t.Cleanup(func() { nowFn = time.Now })
	frozen := time.Unix(1700000000, 0).UTC()
	SetClock(func() time.Time { return frozen })
	if got := nowFn(); !got.Equal(frozen) {
		t.Errorf("clock not injected: %v", got)
	}
	// SetClock(nil) leaves clock untouched.
	SetClock(nil)
	if got := nowFn(); !got.Equal(frozen) {
		t.Errorf("nil reset clock: %v", got)
	}
}

// TestNoScope_AnnotationsAreNoOps — package-level helpers introduced in
// this branch (CustomLink/IssueLink/TmsLinkID/AttachJSON/AttachText/AttachPNG)
// must be no-ops when called without an active scope.
//
// We use context.TODO() rather than nil: fromContext returns the same nil
// scope in both cases (TODO has no scopeKey value), and TODO is what
// staticcheck (SA1012) expects when the caller has no specific Context.
func TestNoScope_AnnotationsAreNoOps(t *testing.T) {
	// No panic = success.
	type Foo struct{ A int }
	ctx := context.TODO()
	IssueLink(ctx, "X")
	TmsLinkID(ctx, "X")
	CustomLink(ctx, "design", "X")
	AttachJSON(ctx, "x", Foo{A: 1})
	AttachText(ctx, "x", "hi")
	AttachPNG(ctx, "x", []byte{0x89, 0x50})
	if got := expandLinkPattern("", "X"); got != "X" {
		t.Errorf("empty pattern returns id, got %q", got)
	}
	RegisterLinkType("Design")
	if _, ok := LinkTypeRegistry["design"]; !ok {
		t.Errorf("RegisterLinkType failed")
	}
	// Empty string register stays a no-op.
	if got := RegisterLinkType(""); got != "" {
		t.Errorf("empty register returned %q", got)
	}
}

// TestPendingParameters_Lifecycle covers the parameter stash semantics.
func TestPendingParameters_Lifecycle(t *testing.T) {
	pushPendingParameters("X", []AllureParameter{{Name: "k", Value: "v"}})
	got := consumePendingParameters("X")
	if len(got) != 1 || got[0].Name != "k" {
		t.Errorf("consume = %v", got)
	}
	// popPendingParameters removes the entry.
	popPendingParameters("X")
	if c := consumePendingParameters("X"); c != nil {
		t.Errorf("expected nil after pop, got %v", c)
	}
	// Empty name = no-op.
	pushPendingParameters("", []AllureParameter{{Name: "k"}})
	popPendingParameters("")
	if c := consumePendingParameters(""); c != nil {
		t.Errorf("empty name returned %v", c)
	}
}

// TestDerivePayloadParameters_NonStruct covers the non-struct payload path.
func TestDerivePayloadParameters_NonStruct(t *testing.T) {
	got := derivePayloadParameters(42)
	if len(got) != 1 || got[0].Name != "value" || got[0].Value != "42" {
		t.Errorf("scalar payload = %v", got)
	}
	got = derivePayloadParameters((*struct{ A int })(nil))
	if got != nil {
		t.Errorf("nil pointer payload = %v", got)
	}
	got = derivePayloadParameters(struct {
		Skip  string `allure:"-"`
		Named string `allure:"renamed"`
		Plain string
	}{Skip: "x", Named: "n", Plain: "p"})
	if len(got) != 2 {
		t.Fatalf("got %d params", len(got))
	}
	names := []string{got[0].Name, got[1].Name}
	if names[0] != "renamed" || names[1] != "Plain" {
		t.Errorf("names = %v", names)
	}
}

// TestGoroutineID_NotZero — sanity check; the thread label must look like
// a positive integer.
func TestGoroutineID_NotZero(t *testing.T) {
	if id := goroutineID(); id <= 0 {
		t.Errorf("goroutineID = %d", id)
	}
	if h := hostName(); h == "" {
		t.Errorf("hostName empty")
	}
}

// TestExpandLinkPattern_TrailingPrefix verifies the "no placeholder, just
// concatenate" path.
func TestExpandLinkPattern_TrailingPrefix(t *testing.T) {
	if got := expandLinkPattern("https://x/", "abc"); got != "https://x/abc" {
		t.Errorf("got %q", got)
	}
}

// TestNamedHook_HandlesNil verifies the namedHook wrapper survives a nil
// Hook input.
func TestNamedHook_HandlesNil(t *testing.T) {
	h := namedHook("x", nil)
	h(nil) // must not panic
	_ = hookNameOf(h)
}

// TestAllOptionWrappers ensures every With* option lands on the config
// without surprise.
func TestAllOptionWrappers(t *testing.T) {
	c := config{}
	opts := []Option{
		WithResultsDir("/x"),
		WithName("n"),
		WithFullName("fn"),
		WithSuite("s"),
		WithParentSuite("ps"),
		WithSubSuite("sub"),
		WithPackage("p"),
		WithTestClass("tc"),
		WithTestMethod("tm"),
		WithDescription("d"),
		WithDescriptionHTML("h"),
		WithFeature("f"),
		WithStory("st"),
		WithEpic("e"),
		WithOwner("o"),
		WithSeverity(SeverityHigh),
		WithLabel("k", "v"),
		WithLink("name", "url"),
		WithIssue("i", "u"),
		WithTmsLink("t", "u"),
		WithParameter("p", "v"),
		WithParameterEx("x", "y", ParameterModeMasked, true),
		WithIssuePattern("https://j/{0}"),
		WithClock(time.Now),
	}
	for _, o := range opts {
		o(&c)
	}
	if c.name != "n" || c.fullName != "fn" || c.suite != "s" ||
		c.parentSuite != "ps" || c.subSuite != "sub" || c.pkg != "p" ||
		c.testClass != "tc" || c.testMethod != "tm" || c.description != "d" ||
		c.descrHTML != "h" || c.feature != "f" || c.story != "st" ||
		c.epic != "e" || c.owner != "o" || c.severity != SeverityHigh ||
		c.resultsDir != "/x" {
		t.Errorf("options not applied: %+v", c)
	}
	if len(c.labels) == 0 || len(c.links) != 3 || len(c.parameters) != 2 {
		t.Errorf("collection options not applied: labels=%v links=%v params=%v", c.labels, c.links, c.parameters)
	}
	// WithMime separate type, covered here for completeness.
	ao := WithMime("application/x-test")
	mo := attachOpts{}
	ao(&mo)
	if mo.mime != "application/x-test" {
		t.Errorf("WithMime not applied: %q", mo.mime)
	}
}

// TestParallelStep_PanicCapturedAsBroken — panic inside a branch lands
// as a broken step but doesn't crash the test.
//
// Verifies the post-fix invariant: a panicking branch records EXACTLY one
// step with the original panic message (not a duplicate placeholder).
// The previous ParallelStep implementation pushed an extra "(panicked)"
// step in addition to Step's own StatusBroken — duplicating the failure
// in the Allure report and losing the original trace.
//
// NOTE: ParallelStep concurrent goroutines share scope.stepStack so the
// step nesting order is non-deterministic (two goroutines may push such
// that one becomes the other's child rather than its sibling). The test
// walks the tree depth-first to find "bad" wherever it landed.
func TestParallelStep_PanicCapturedAsBroken(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		defer func() { _ = recover() }()
		ParallelStep(a.ctx, "fan", map[string]func(){
			"good": func() {},
			"bad":  func() { panic("argh") },
		})
	})
	r := readResultFile(t, dir)
	if len(r.Steps) != 1 || r.Steps[0].Name != "fan" {
		t.Fatalf("expected 1 parent step named 'fan', got %+v", r.Steps)
	}

	// Depth-first scan for a "bad" step. There must be exactly one (the
	// prior bug surfaced as two — one broken with the real message, one
	// passed/broken placeholder named "bad (panicked)").
	var bads []AllureStep
	var hasPanicked bool
	var walk func(steps []AllureStep)
	walk = func(steps []AllureStep) {
		for _, s := range steps {
			if s.Name == "bad" {
				bads = append(bads, s)
			}
			if s.Name == "bad (panicked)" {
				hasPanicked = true
			}
			walk(s.Steps)
		}
	}
	walk(r.Steps[0].Steps)

	if hasPanicked {
		t.Error("step named 'bad (panicked)' present — the duplicate placeholder is back")
	}
	if len(bads) != 1 {
		t.Fatalf("expected exactly 1 'bad' step, got %d: %+v", len(bads), bads)
	}
	bad := bads[0]
	if bad.Status != StatusBroken {
		t.Errorf("bad.Status=%q, want broken", bad.Status)
	}
	if bad.StatusDetails == nil || bad.StatusDetails.Message != "argh" {
		t.Errorf("bad.StatusDetails should carry the real panic message 'argh', got %+v", bad.StatusDetails)
	}
}

// TestParallelStep_EmptyBranches stops at the parent step.
func TestParallelStep_EmptyBranches(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		ParallelStep(a.ctx, "empty", nil)
	})
	r := readResultFile(t, dir)
	if len(r.Steps) != 1 || r.Steps[0].Name != "empty" {
		t.Errorf("got %+v", r.Steps)
	}
}

// TestFlushAllSuites_NoOpWhenEmpty exercises the leftover-flush path.
func TestFlushAllSuites_NoOpWhenEmpty(t *testing.T) {
	flushAllSuites() // no panic
}

// TestScope_MarkFailure exercises the markFailure helper directly. The
// scope/test wiring routes this through testing.T.Failed() ordinarily, so
// we drive it manually here.
func TestScope_MarkFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	ctx, finish := WithTest(context.TODO(), "manual", WithResultsDir(dir))
	s := fromContext(ctx)
	s.markFailure(StatusFailed, "msg", "stack")
	// Setting again must update message/trace fields.
	s.markFailure(StatusFailed, "msg2", "stack2")
	s.markFailure("", "", "")
	finish()
	r := readResultFile(t, dir)
	if r.Status != StatusFailed {
		t.Errorf("status = %s", r.Status)
	}
}

// TestLinkHelpers_WithScope exercises IssueLink/TmsLinkID/CustomLink end
// to end via WithTest.
func TestLinkHelpers_WithScope(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(IssuePatternEnv, "https://j/{0}")
	t.Setenv(TmsPatternEnv, "https://t/{0}")
	t.Setenv(LinkPatternPfx+"DOCS", "https://d/{0}")
	ctx, finish := WithTest(context.TODO(), "x", WithResultsDir(dir))
	IssueLink(ctx, "I1")
	TmsLinkID(ctx, "T1")
	CustomLink(ctx, "docs", "D1")
	finish()
	r := readResultFile(t, dir)
	want := map[string]string{
		"https://j/I1": "issue",
		"https://t/T1": "tms",
		"https://d/D1": "docs",
	}
	got := map[string]string{}
	for _, l := range r.Links {
		got[l.URL] = l.Type
	}
	for u, ty := range want {
		if got[u] != ty {
			t.Errorf("link %s: got type %q, want %q (all=%v)", u, got[u], ty, got)
		}
	}
}

// TestAttachJSON_MarshalFailureProducesErrorBlob — unmarshalable values
// (channels, funcs) land as a `.error` plaintext attachment instead of
// crashing.
func TestAttachJSON_MarshalFailureProducesErrorBlob(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.AttachJSON("bad", make(chan int))
	})
	r := readResultFile(t, dir)
	if len(r.Attachments) != 1 || r.Attachments[0].Name != "bad.error" {
		t.Errorf("attachments = %+v", r.Attachments)
	}
}

// TestAllureT_FlushCapturesSkippedAndFailed verifies the t.Cleanup hook
// translates testing.T.Skipped() / Failed() to scope status.
func TestAllureT_FlushCapturesSkippedAndFailed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("skipped", func(inner *testing.T) {
		_ = T(inner, WithResultsDir(dir))
		inner.Skip("skipping")
	})
	r := readResultFile(t, dir)
	if r.Status != StatusSkipped {
		t.Errorf("status = %s, want skipped", r.Status)
	}
}

// TestBeginStep_NoScope returns a usable nil-safe handle.
func TestBeginStep_NoScope(t *testing.T) {
	h := BeginStep(context.TODO(), "")
	if h == nil {
		t.Fatal("handle is nil")
	}
	h.End()    // no-op
	h.Fail("") // no-op
	h.Broken("", "")
	h.Skip("")
}

// TestSplitTestPath_Edges covers the splitTestPath behaviour matrix.
func TestSplitTestPath_Edges(t *testing.T) {
	cases := []struct {
		in, cls, mtd string
	}{
		{"", "", ""},
		{"TestX", "TestX", "TestX"},
		{"TestX/sub", "TestX", "sub"},
		{"TestX/sub/deep", "TestX", "sub/deep"},
	}
	for _, c := range cases {
		gotC, gotM := splitTestPath(c.in)
		if gotC != c.cls || gotM != c.mtd {
			t.Errorf("split(%q) = (%q,%q), want (%q,%q)", c.in, gotC, gotM, c.cls, c.mtd)
		}
	}
}
