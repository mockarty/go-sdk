// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTestBasicScript(t *testing.T) {
	script := NewLoadTest("smoke").
		Target("http://127.0.0.1:8080").
		Get("/health").
		VUs(5).
		Duration("30s").
		ToK6Script()

	for _, want := range []string{
		"import http from 'k6/http'",
		"export const options",
		"export default function",
		"http.get(`${BASE_URL}/health`)",
		`"vus":5`,
		`"duration":"30s"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q\n---\n%s", want, script)
		}
	}
}

func TestLoadTestMaxVUsUsesCanonicalWireSpelling(t *testing.T) {
	profile := NewLoadTest("arrival").
		Target("http://127.0.0.1:8080").
		Get("/").
		RPS(100).
		MaxVUs(50)

	script := profile.ToK6Script()
	if !strings.Contains(script, `"maxVUs":50`) {
		t.Fatalf("script missing canonical maxVUs: %s", script)
	}
	if strings.Contains(script, `"maxVus"`) {
		t.Fatalf("script emitted legacy maxVus: %s", script)
	}
	raw, err := json.Marshal(profile.ToPerfConfig())
	if err != nil {
		t.Fatalf("marshal perf config: %v", err)
	}
	if !strings.Contains(string(raw), `"maxVUs":50`) || strings.Contains(string(raw), `"maxVus"`) {
		t.Fatalf("perf config must emit canonical maxVUs only: %s", raw)
	}

	var legacy LoadConfig
	if err := json.Unmarshal([]byte(`{"maxVus":41}`), &legacy); err != nil {
		t.Fatalf("decode legacy maxVus input: %v", err)
	}
	if legacy.MaxVUs != 41 {
		t.Fatalf("legacy maxVus input decoded as %d, want 41", legacy.MaxVUs)
	}
}

func TestLoadTestStagesWinOverVUs(t *testing.T) {
	cfg := NewLoadTest("ramp").
		Target("http://x").
		Get("/").
		VUs(99).
		Stages(Stage("10s", 20), Stage("30s", 20), Stage("5s", 0)).
		ToPerfConfig()

	if len(cfg.Stages) != 3 {
		t.Fatalf("len(Stages) = %d, want 3", len(cfg.Stages))
	}
	if cfg.Stages[0].Duration != "10s" || cfg.Stages[0].Target != 20 {
		t.Errorf("Stages[0] = %+v", cfg.Stages[0])
	}
	// VUs must NOT be emitted when stages are present (omitempty + 0).
	if cfg.VUs != 0 {
		t.Errorf("VUs = %d, want 0 (stages win)", cfg.VUs)
	}
}

func TestLoadTestThresholds(t *testing.T) {
	cfg := NewLoadTest("t").
		Target("http://x").
		Threshold("http_req_duration", "p(95)<500").
		Threshold("http_req_duration", "p(99)<900").
		Threshold("http_req_failed", "rate<0.01").
		ToPerfConfig()

	got := cfg.Thresholds["http_req_duration"]
	if len(got) != 2 || got[0] != "p(95)<500" || got[1] != "p(99)<900" {
		t.Errorf("http_req_duration thresholds = %v", got)
	}
	if cfg.Thresholds["http_req_failed"][0] != "rate<0.01" {
		t.Errorf("http_req_failed = %v", cfg.Thresholds["http_req_failed"])
	}
}

func TestLoadTestPostBodyJSON(t *testing.T) {
	script := NewLoadTest("t").
		Target("http://api").
		Post("/cart", map[string]any{"sku": "abc", "qty": 2}).
		ToK6Script()

	if !strings.Contains(script, "http.post(`${BASE_URL}/cart`") {
		t.Errorf("missing post call:\n%s", script)
	}
	if !strings.Contains(script, "application/json") {
		t.Errorf("missing json content-type:\n%s", script)
	}
	if !strings.Contains(script, "sku") || !strings.Contains(script, "abc") {
		t.Errorf("missing serialized body:\n%s", script)
	}
}

func TestLoadTestEnvAndBaseURL(t *testing.T) {
	cfg := NewLoadTest("t").
		Target("https://staging.example.com").
		Env("TOKEN", "secret").
		Get("/").
		ToPerfConfig()

	if cfg.Environment["BASE_URL"] != "https://staging.example.com" {
		t.Errorf("BASE_URL = %q", cfg.Environment["BASE_URL"])
	}
	if cfg.Environment["TOKEN"] != "secret" {
		t.Errorf("TOKEN = %q", cfg.Environment["TOKEN"])
	}
}

func TestLoadTestDefaultRequestGetRoot(t *testing.T) {
	script := NewLoadTest("t").Target("http://x").ToK6Script()
	if !strings.Contains(script, "http.get(`${BASE_URL}/`)") {
		t.Errorf("default request not GET /:\n%s", script)
	}
}

func TestLoadTestToJSONFullProfile(t *testing.T) {
	raw, err := NewLoadTest("checkout").
		Target("http://127.0.0.1:8080").
		Get("/health").
		Post("/order", map[string]any{"item": 1}).
		Stages(Stage("2s", 3)).
		Threshold("http_req_failed", "rate<0.1").
		ThinkTime(0.5).
		ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("emitted JSON invalid: %v", err)
	}
	if parsed["name"] != "checkout" {
		t.Errorf("name = %v", parsed["name"])
	}
	script, _ := parsed["script"].(string)
	if !strings.HasPrefix(script, "import http") {
		t.Errorf("script prefix: %q", script[:min(20, len(script))])
	}
	if !strings.Contains(script, "sleep(0.5)") {
		t.Errorf("missing think time in script")
	}
}

func TestLoadTestRPSStage(t *testing.T) {
	cfg := NewLoadTest("t").Target("http://x").Get("/").
		Stages(RPSStage("10s", 100)).ToPerfConfig()
	if cfg.Stages[0].TargetRPS != 100 {
		t.Errorf("TargetRPS = %d, want 100", cfg.Stages[0].TargetRPS)
	}
}

func TestLoadTestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "load.json")
	if err := NewLoadTest("x").Target("http://x").Get("/").SaveConfig(path); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved JSON invalid: %v", err)
	}
	if parsed["name"] != "x" {
		t.Errorf("name = %v", parsed["name"])
	}
}

// TestToK6Script_NoDuplicateLetR guards the multi-request k6 export: a flat
// sequence of `let r = ...` per request is a JS SyntaxError (redeclaration in
// the same scope). The script must declare `let r;` once, then assign.
func TestToK6Script_NoDuplicateLetR(t *testing.T) {
	script := NewLoadTest("noletdup").Target("http://x").Get("/a").Post("/b", nil).Put("/c", nil).ToK6Script()
	if got := strings.Count(script, "let r"); got != 1 {
		t.Fatalf("expected exactly one `let r` declaration, got %d:\n%s", got, script)
	}
	if c := strings.Count(script, "r = http."); c != 3 {
		t.Fatalf("expected 3 `r = http.` assignments for 3 requests, got %d", c)
	}
}

// TestLoadTestBakesBaseURLDefault verifies the exported k6 script is
// self-contained: Target() is baked as `const BASE_URL = __ENV.BASE_URL ||
// '<target>'` (runnable standalone, overridable via -e/__ENV) and threshold
// expressions are NOT HTML-escaped (p(95)<500, never p(95)<500).
func TestLoadTestBakesBaseURLDefault(t *testing.T) {
	script := NewLoadTest("t").
		Target("http://127.0.0.1:5870").
		Get("/health").
		Threshold("http_req_duration", "p(95)<500").
		ToK6Script()

	if !strings.Contains(script, "const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:5870';") {
		t.Errorf("missing baked BASE_URL default in:\n%s", script)
	}
	if strings.Contains(script, "${__ENV.BASE_URL}") {
		t.Errorf("script should reference ${BASE_URL}, not ${__ENV.BASE_URL}:\n%s", script)
	}
	if strings.Contains(script, "\\u003c") {
		t.Errorf("threshold got HTML-escaped (\\u003c) — want literal '<':\n%s", script)
	}
	if !strings.Contains(script, "p(95)<500") {
		t.Errorf("threshold expression not emitted verbatim:\n%s", script)
	}
}

// TestLoadTestNoBaseURLNoConst — without Target() there is no BASE_URL const
// and requests use literal paths (nothing to bake).
func TestLoadTestNoBaseURLNoConst(t *testing.T) {
	script := NewLoadTest("t").Get("/health").ToK6Script()
	if strings.Contains(script, "const BASE_URL") {
		t.Errorf("no Target() set — should not emit BASE_URL const:\n%s", script)
	}
}

// TestLoadTestDrainStageTargetZero guards that a ramp-down-to-zero stage emits
// "target":0 (a bare {"duration":".."} is not a valid k6 stage). The int
// `omitempty` on LoadStage.Target used to drop the 0.
func TestLoadTestDrainStageTargetZero(t *testing.T) {
	script := NewLoadTest("t").Target("http://x").Get("/").
		Stages(Stage("10s", 5), Stage("10s", 0)).
		ToK6Script()
	if !strings.Contains(script, `{"duration":"10s","target":0}`) {
		t.Errorf("drain stage must keep target:0, got:\n%s", script)
	}
	if strings.Contains(script, `{"duration":"10s"}`) {
		t.Errorf("stage emitted without a target (invalid k6):\n%s", script)
	}
	// An RPS stage still swaps target for targetRps.
	rps := NewLoadTest("t").Target("http://x").Get("/").Stages(RPSStage("10s", 100)).ToK6Script()
	if !strings.Contains(rps, `"targetRps":100`) {
		t.Errorf("RPS stage must emit targetRps:\n%s", rps)
	}
}

// TestLoadTestPerEndpointChecks verifies custom per-request checks replace the
// default status<400 assertion and that a request without checks keeps it.
func TestLoadTestPerEndpointChecks(t *testing.T) {
	script := NewLoadTest("checks").Target("http://x").
		Post("/order", map[string]any{"sku": "a"}).ExpectStatus(201).Check("has id", "res.json().id !== undefined").
		Get("/health").
		ToK6Script()

	want := "  check(r, { 'status is 201': (res) => res.status === 201, 'has id': (res) => res.json().id !== undefined });"
	if !strings.Contains(script, want) {
		t.Errorf("custom check line missing.\nwant: %s\ngot script:\n%s", want, script)
	}
	// The second request (no checks) keeps the default.
	if !strings.Contains(script, "check(r, { 'status < 400': (res) => res.status < 400 });") {
		t.Errorf("default check missing for uncustomised request:\n%s", script)
	}
}

// TestLoadTestCheckNoRequestIsNoop guards Check/ExpectStatus called before any
// request — must not panic and must not emit a stray check.
func TestLoadTestCheckNoRequestIsNoop(t *testing.T) {
	// No request added yet; Check is a no-op, Target-only build uses the
	// default GET "/" which still gets the default check.
	script := NewLoadTest("t").Target("http://x").Check("x", "true").ExpectStatus(200).ToK6Script()
	if !strings.Contains(script, "check(r, { 'status < 400'") {
		t.Errorf("expected default check on the implicit GET / request:\n%s", script)
	}
}
