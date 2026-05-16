// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// readResultFile returns the single *-result.json found in dir, decoded.
// Fails the test if zero or multiple result files exist (callers should
// pick a directory that contains exactly one result).
func readResultFile(t *testing.T, dir string) Result {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var resultPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-result.json") {
			if resultPath != "" {
				t.Fatalf("multiple result files in %s", dir)
			}
			resultPath = filepath.Join(dir, e.Name())
		}
	}
	if resultPath == "" {
		t.Fatalf("no result file in %s (entries=%d)", dir, len(entries))
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var r Result
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("decode result %s: %v\nbody=%s", resultPath, err, string(data))
	}
	return r
}

func TestT_FlushesResultOnCleanup(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	a := T(t, WithResultsDir(dir), WithFeature("Auth"))
	a.Severity(SeverityCritical)
	a.Issue("JIRA-1", "https://jira/JIRA-1")
	a.TmsLink("TMS-9", "https://tms/9")
	a.Description("Login should succeed with valid credentials")
	a.Step("submit form", func() {
		a.Attachment("response.json", []byte(`{"ok":true}`), "application/json")
	})

	// AllureT writes on t.Cleanup so we trigger that manually for the
	// assertion below — t.Cleanup runs in LIFO order at sub-test boundary,
	// so we cannot peek the file before the wrapper finishes the test. Use
	// t.Run to scope the flush.
	t.Run("inner", func(inner *testing.T) {
		aa := T(inner, WithResultsDir(dir), WithFeature("Inner"))
		aa.Step("inner step", func() {})
	})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	results := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-result.json") {
			results++
		}
	}
	if results == 0 {
		t.Fatalf("expected at least one result, got %d (entries=%v)", results, entries)
	}
}

func TestT_ResultShape(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner,
			WithResultsDir(dir),
			WithFeature("Login"),
			WithStory("Email/password"),
			WithEpic("Onboarding"),
			WithOwner("qa@example.com"),
			WithSuite("auth-smoke"),
			WithSeverity(SeverityCritical),
			WithLabel("layer", "api"),
			WithIssue("JIRA-7", "https://jira/JIRA-7"),
			WithTmsLink("TMS-1", "https://tms/1"),
			WithParameter("env", "staging"),
		)
		a.Description("Verifies POST /login happy-path.")
		a.Step("submit form", func() {
			a.Parameter("user", "alice")
			a.Attachment("payload.json", []byte(`{"email":"a@b.c"}`), "application/json")
		})
	})
	r := readResultFile(t, dir)

	if r.UUID == "" {
		t.Errorf("UUID empty")
	}
	if r.HistoryID == "" {
		t.Errorf("HistoryID empty")
	}
	if r.Status != StatusPassed {
		t.Errorf("status = %s, want passed", r.Status)
	}
	if r.Stage != StageFinished {
		t.Errorf("stage = %s, want finished", r.Stage)
	}
	if r.Description != "Verifies POST /login happy-path." {
		t.Errorf("description = %q", r.Description)
	}
	wantLabels := map[string]string{
		LabelFramework: "mockarty",
		LabelLanguage:  "go",
		LabelSuite:     "auth-smoke",
		LabelFeature:   "Login",
		LabelStory:     "Email/password",
		LabelEpic:      "Onboarding",
		LabelOwner:     "qa@example.com",
		LabelSeverity:  string(SeverityCritical),
		"layer":        "api",
	}
	gotLabels := map[string]string{}
	for _, l := range r.Labels {
		gotLabels[l.Name] = l.Value
	}
	for k, v := range wantLabels {
		if gotLabels[k] != v {
			t.Errorf("label %s = %q, want %q (all=%v)", k, gotLabels[k], v, gotLabels)
		}
	}
	if len(r.Links) != 2 {
		t.Errorf("links len = %d, want 2", len(r.Links))
	}
	gotLinkTypes := map[string]string{}
	for _, l := range r.Links {
		gotLinkTypes[l.Type] = l.URL
	}
	if gotLinkTypes[LinkTypeIssue] != "https://jira/JIRA-7" {
		t.Errorf("issue link missing or wrong: %v", r.Links)
	}
	if gotLinkTypes[LinkTypeTMS] != "https://tms/1" {
		t.Errorf("tms link missing or wrong: %v", r.Links)
	}
	if len(r.Parameters) == 0 || r.Parameters[0].Name != "env" {
		t.Errorf("top-level parameter missing: %v", r.Parameters)
	}
	if len(r.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(r.Steps))
	}
	step := r.Steps[0]
	if step.Name != "submit form" {
		t.Errorf("step name = %q", step.Name)
	}
	if step.Status != StatusPassed {
		t.Errorf("step status = %s", step.Status)
	}
	if step.Stop < step.Start {
		t.Errorf("step stop (%d) < start (%d)", step.Stop, step.Start)
	}
	if len(step.Parameters) != 1 || step.Parameters[0].Name != "user" {
		t.Errorf("step parameter missing: %v", step.Parameters)
	}
	if len(step.Attachments) != 1 {
		t.Fatalf("step attachments = %d, want 1", len(step.Attachments))
	}
	att := step.Attachments[0]
	if att.Name != "payload.json" || att.Type != "application/json" {
		t.Errorf("attachment metadata wrong: %+v", att)
	}
	// Attachment file must exist on disk
	if _, err := os.Stat(filepath.Join(dir, att.Source)); err != nil {
		t.Errorf("attachment file missing: %v", err)
	}
}

func TestT_FailureFromStepErr(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		_ = a.StepErr("doomed", func() error { return errors.New("boom") })
	})
	r := readResultFile(t, dir)
	if r.Status != StatusFailed {
		t.Errorf("status = %s, want failed", r.Status)
	}
	if len(r.Steps) != 1 || r.Steps[0].Status != StatusFailed {
		t.Errorf("step status not failed: %+v", r.Steps)
	}
	if r.Steps[0].StatusDetails == nil || r.Steps[0].StatusDetails.Message != "boom" {
		t.Errorf("step detail wrong: %+v", r.Steps[0].StatusDetails)
	}
}

func TestT_PanicCapturedAsBroken(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		// We deliberately recover at this level so the surrounding *testing.T
		// stays green (we are testing the writer behaviour, not the panic).
		defer func() {
			_ = recover()
		}()
		a := T(inner, WithResultsDir(dir))
		a.Step("explodes", func() { panic("kaboom") })
	})
	r := readResultFile(t, dir)
	if r.Status != StatusBroken {
		t.Errorf("status = %s, want broken", r.Status)
	}
	if len(r.Steps) != 1 || r.Steps[0].Status != StatusBroken {
		t.Errorf("step status = %s, want broken", r.Steps[0].Status)
	}
	if r.Steps[0].StatusDetails == nil || r.Steps[0].StatusDetails.Message != "kaboom" {
		t.Errorf("broken detail wrong: %+v", r.Steps[0].StatusDetails)
	}
	if r.Steps[0].StatusDetails.Trace == "" {
		t.Errorf("expected stack trace in broken step")
	}
}

func TestNestedSteps_FiveLevelsDeep(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.Step("l1", func() {
			a.Step("l2", func() {
				a.Step("l3", func() {
					a.Step("l4", func() {
						a.Step("l5", func() {
							a.Attachment("deep.txt", []byte("found me"), "text/plain")
						})
					})
				})
			})
		})
	})
	r := readResultFile(t, dir)
	// Walk down five levels.
	cur := r.Steps
	for i := 1; i <= 5; i++ {
		if len(cur) != 1 {
			t.Fatalf("at level %d: expected 1 child, got %d", i, len(cur))
		}
		if cur[0].Name != "l"+itoa(i) {
			t.Errorf("level %d name = %q, want l%d", i, cur[0].Name, i)
		}
		if i == 5 {
			if len(cur[0].Attachments) != 1 {
				t.Errorf("attachment missing at deepest step")
			}
		}
		cur = cur[0].Steps
	}
}

func TestConcurrentSteps_RaceSafe(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		var wg sync.WaitGroup
		const N = 100
		wg.Add(N)
		for i := 0; i < N; i++ {
			go func(n int) {
				defer wg.Done()
				// Each goroutine uses its own context-derived scope to record a
				// step under the SAME scope object. We exercise the locked mutators
				// (addLabel + addAttachment) which are the hot path in real life.
				a.Label("goroutine", "g")
				a.Attachment("blob", []byte("data"), "application/octet-stream")
			}(i)
		}
		wg.Wait()
	})
	r := readResultFile(t, dir)
	// We expect at least N "goroutine" labels (plus the bootstrap labels).
	count := 0
	for _, l := range r.Labels {
		if l.Name == "goroutine" {
			count++
		}
	}
	if count != 100 {
		t.Errorf("expected 100 concurrent labels, got %d", count)
	}
	if len(r.Attachments) != 100 {
		t.Errorf("expected 100 attachments, got %d", len(r.Attachments))
	}
}

func TestEmptyStepName_NormalisedNotBlank(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.Step("", func() {})
	})
	r := readResultFile(t, dir)
	if len(r.Steps) != 1 {
		t.Fatalf("steps = %d", len(r.Steps))
	}
	if r.Steps[0].Name == "" {
		t.Errorf("empty step name was not normalised")
	}
}

func TestBinaryAttachment_PreservesBytes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	payload := make([]byte, 4096)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	var attSource string
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.Attachment("blob.bin", payload, "application/octet-stream")
		// Capture source from in-memory result for cross-check.
		// We need to wait for flush; use UUID to find the file after t.Run.
		_ = a.UUID()
	})
	r := readResultFile(t, dir)
	if len(r.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(r.Attachments))
	}
	attSource = r.Attachments[0].Source
	got, err := os.ReadFile(filepath.Join(dir, attSource))
	if err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("attachment bytes = %d, want %d", len(got), len(payload))
	}
	for i := range payload {
		if got[i] != payload[i] {
			t.Fatalf("byte %d differs: got %#x want %#x", i, got[i], payload[i])
		}
	}
}

func TestVeryLongDescription(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	// >1 MB UTF-8 description.
	const sz = 1 << 20
	var sb strings.Builder
	sb.Grow(sz + 8)
	for sb.Len() < sz {
		sb.WriteString("Описание тестового сценария. ")
	}
	desc := sb.String()
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.Description(desc)
	})
	r := readResultFile(t, dir)
	if len(r.Description) < sz {
		t.Errorf("description truncated: got %d bytes, want >= %d", len(r.Description), sz)
	}
}

func TestResolveResultsDir_EnvFallback(t *testing.T) {
	t.Setenv(ResultsDirEnv, "/tmp/from-env")
	if got := ResolveResultsDir(""); got != "/tmp/from-env" {
		t.Errorf("env fallback failed: got %q", got)
	}
	if got := ResolveResultsDir("/explicit"); got != "/explicit" {
		t.Errorf("explicit ignored: got %q", got)
	}
	t.Setenv(ResultsDirEnv, "")
	if got := ResolveResultsDir(""); got != DefaultResultsDir {
		t.Errorf("default fallback failed: got %q", got)
	}
}

func TestWithTest_ContextAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	ctx, finish := WithTest(context.Background(), "ctx-test", WithResultsDir(dir))
	Feature(ctx, "Ctx")
	Story(ctx, "Closure")
	SeverityLevel(ctx, SeverityNormal)
	Owner(ctx, "qa@example.com")
	Tag(ctx, "smoke")
	Label(ctx, "extra", "y")
	Issue(ctx, "JIRA-1", "https://jira/JIRA-1")
	TmsLink(ctx, "TMS-1", "https://tms/1")
	LinkURL(ctx, "Wiki", "https://wiki/")
	Description(ctx, "ctx-based test")
	Step(ctx, "go", func() {
		Parameter(ctx, "k", "v")
		Attachment(ctx, "data.txt", []byte("hi"), "text/plain")
	})
	finish()

	r := readResultFile(t, dir)
	if r.Name != "ctx-test" {
		t.Errorf("name = %q", r.Name)
	}
	if r.Status != StatusPassed {
		t.Errorf("status = %s", r.Status)
	}
	if len(r.Links) != 3 {
		t.Errorf("links len = %d, want 3", len(r.Links))
	}
	if len(r.Steps) != 1 || len(r.Steps[0].Parameters) != 1 {
		t.Errorf("step parameter missing: %+v", r.Steps)
	}
}

func TestPackageLevelFunctions_NoScope_DoNotPanic(t *testing.T) {
	// Calling annotation funcs with a nil/empty context must be a no-op.
	Feature(context.Background(), "x")
	Story(context.Background(), "x")
	Epic(context.Background(), "x")
	Owner(context.Background(), "x")
	Tag(context.Background(), "x")
	Label(context.Background(), "x", "x")
	Issue(context.Background(), "x", "x")
	TmsLink(context.Background(), "x", "x")
	LinkURL(context.Background(), "x", "x")
	Parameter(context.Background(), "x", "x")
	Description(context.Background(), "x")
	Title(context.Background(), "x")
	SeverityLevel(context.Background(), SeverityMinor)
	Step(context.Background(), "noop", func() {})
	Attachment(context.Background(), "x", []byte("x"), "text/plain")
	if err := StepErr(context.Background(), "noop", func() error { return nil }); err != nil {
		t.Errorf("noop StepErr returned %v", err)
	}
}

func TestManualStepHandle_Lifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		ctx, finish := WithTest(context.Background(), "manual", WithResultsDir(dir))
		defer finish()
		h := BeginStep(ctx, "manual-pass")
		h.End()
		h2 := BeginStep(ctx, "manual-fail")
		h2.Fail("nope")
		h3 := BeginStep(ctx, "manual-broken")
		h3.Broken("disaster", "stack")
		h4 := BeginStep(ctx, "manual-skipped")
		h4.Skip("not impl")
		_ = inner
	})
	r := readResultFile(t, dir)
	want := []struct {
		name   string
		status Status
	}{
		{"manual-pass", StatusPassed},
		{"manual-fail", StatusFailed},
		{"manual-broken", StatusBroken},
		{"manual-skipped", StatusSkipped},
	}
	if len(r.Steps) != len(want) {
		t.Fatalf("steps = %d, want %d", len(r.Steps), len(want))
	}
	for i, w := range want {
		if r.Steps[i].Name != w.name {
			t.Errorf("step[%d] name = %q, want %q", i, r.Steps[i].Name, w.name)
		}
		if r.Steps[i].Status != w.status {
			t.Errorf("step[%d] status = %s, want %s", i, r.Steps[i].Status, w.status)
		}
	}
	// Aggregate status follows the strongest failure (broken > failed).
	if r.Status != StatusBroken {
		t.Errorf("aggregate status = %s, want broken", r.Status)
	}
}

func TestExecutorAndCategoriesWriter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	w := NewFileWriter(dir)
	if err := w.WriteExecutor(Executor{Name: "Mockarty CI", Type: "mockarty"}); err != nil {
		t.Fatalf("executor: %v", err)
	}
	cats := []Category{
		{Name: "Bugs", MatchedStatuses: []Status{StatusFailed}},
		{Name: "Crashes", MatchedStatuses: []Status{StatusBroken}, MessageRegex: ".*panic.*"},
	}
	if err := w.WriteCategories(cats); err != nil {
		t.Fatalf("categories: %v", err)
	}
	// Files must exist with expected names.
	for _, f := range []string{"executor.json", "categories.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("file missing %s: %v", f, err)
		}
	}
}

func TestContainerWriter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	w := NewFileWriter(dir)
	c := Container{UUID: "abc", Children: []string{"x", "y"}}
	if err := w.WriteContainer(c); err != nil {
		t.Fatalf("container: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc-container.json")); err != nil {
		t.Errorf("container file missing: %v", err)
	}
	// UUID required.
	if err := w.WriteContainer(Container{}); err == nil {
		t.Errorf("expected error for empty container UUID")
	}
}

func TestT_AccessorWrappers_Coverage(t *testing.T) {
	// Exercises the *AllureT methods that simply delegate to package-level
	// counterparts so they are not flagged as 0% coverage. Covers the small
	// gap that the contextual test left behind.
	dir := filepath.Join(t.TempDir(), "allure-results")
	tmpFile := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(tmpFile, []byte("file contents"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		_ = a.ResultsDir()
		_ = a.UUID()
		a.Feature("Auth")
		a.Story("OAuth")
		a.Epic("Onboarding")
		a.Owner("qa")
		a.Tag("smoke", "regression")
		a.Title("Renamed")
		a.Link("Wiki", "https://w/")
		a.AttachString("note", "hi")
		a.AttachJSON("payload", []byte(`{"k":1}`))
		a.AttachFile("file", tmpFile)
		a.AttachFile("missing", "/no/such/path") // failure path -> .error blob
	})
	r := readResultFile(t, dir)
	// We added: AttachString(note), AttachJSON(payload), AttachFile(file),
	// AttachFile(missing→missing.error). Exactly 4 attachments expected.
	if len(r.Attachments) != 4 {
		t.Errorf("attachments = %d, want 4 (incl. .error blob from missing path)", len(r.Attachments))
	}
	foundError := false
	for _, a := range r.Attachments {
		if a.Name == "missing.error" {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected `.error` attachment for missing file")
	}
}

func TestExtForMime_Table(t *testing.T) {
	cases := []struct {
		mime string
		want string
	}{
		{"application/json", ".json"},
		{"application/problem+json", ".json"},
		{"text/xml", ".xml"},
		{"text/plain", ".txt"},
		{"", ".txt"},
		{"text/html", ".html"},
		{"text/csv", ".csv"},
		{"application/yaml", ".yaml"},
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/jpg", ".jpg"},
		{"image/gif", ".gif"},
		{"image/svg+xml", ".svg"},
		{"application/pdf", ".pdf"},
		{"application/octet-stream", ".bin"},
		{"  IMAGE/PNG  ", ".png"},
		{"x/y", ".dat"},
	}
	for _, c := range cases {
		if got := extForMime(c.mime); got != c.want {
			t.Errorf("extForMime(%q) = %q, want %q", c.mime, got, c.want)
		}
	}
}

// itoa is a tiny helper to avoid pulling in strconv just for level naming.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
