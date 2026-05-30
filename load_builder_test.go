// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

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
		"http.get(`${__ENV.BASE_URL}/health`)",
		`"vus":5`,
		`"duration":"30s"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q\n---\n%s", want, script)
		}
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

	if !strings.Contains(script, "http.post(`${__ENV.BASE_URL}/cart`") {
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
	if !strings.Contains(script, "http.get(`${__ENV.BASE_URL}/`)") {
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
