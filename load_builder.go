// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// LoadTest — fluent DSL for describing a load test
// ---------------------------------------------------------------------------
//
// Describe a load test in idiomatic Go and emit either a k6-compatible script
// (ToK6Script) or a perf-config JSON (ToPerfConfig / SaveConfig) that
// `mockarty-cli perf run --from-config <file>` runs locally — the perf engine
// runs in-process, no server required. The same config can also be submitted
// to a Mockarty server via PerfAPI.
//
// Example:
//
//	profile := mockarty.NewLoadTest("checkout").
//	    Target("https://api.example.com").
//	    Get("/health").
//	    Post("/order", map[string]any{"sku": "abc"}).
//	    Stages(
//	        mockarty.Stage("30s", 50),
//	        mockarty.Stage("1m", 50),
//	        mockarty.Stage("10s", 0),
//	    ).
//	    Threshold("http_req_duration", "p(95)<800").
//	    Threshold("http_req_failed", "rate<0.01").
//	    ThinkTime(0.5)
//
//	_ = profile.SaveConfig("checkout.json")
//	//   $ mockarty-cli perf run --from-config checkout.json
//
// The builder is a thin wrapper around the existing perf engine; it does not
// run anything itself.

// LoadStage is one ramp stage: reach Target VUs (or TargetRPS) over Duration.
// Duration is a k6-style string ("30s", "1m"). Mirrors the engine's stage shape.
type LoadStage struct {
	Duration  string `json:"duration"`
	Target    int    `json:"target,omitempty"`
	TargetRPS int    `json:"targetRps,omitempty"`
}

// MarshalJSON emits a VU stage's `target` even when it is 0 (a ramp-down-to-zero
// drain stage) — a bare `{"duration":"10s"}` is not a valid k6 stage. Only an
// arrival-rate stage (TargetRPS>0) swaps `target` for `targetRps`. Mirrors the
// Python (`{"duration","target"}`) and Java (`stagesAsMaps`) serialization.
func (s LoadStage) MarshalJSON() ([]byte, error) {
	if s.TargetRPS > 0 {
		return json.Marshal(struct {
			Duration  string `json:"duration"`
			TargetRPS int    `json:"targetRps"`
		}{s.Duration, s.TargetRPS})
	}
	return json.Marshal(struct {
		Duration string `json:"duration"`
		Target   int    `json:"target"`
	}{s.Duration, s.Target})
}

// Stage is a convenience constructor for a VU-target ramp stage.
func Stage(duration string, target int) LoadStage {
	return LoadStage{Duration: duration, Target: target}
}

// RPSStage is a convenience constructor for an arrival-rate (RPS-target) stage.
func RPSStage(duration string, targetRPS int) LoadStage {
	return LoadStage{Duration: duration, TargetRPS: targetRPS}
}

// loadRequest is one HTTP request in the scenario's iteration body.
type loadRequest struct {
	method  string
	path    string
	body    any
	headers map[string]string
}

// LoadConfig is the perf-config JSON shape consumed by
// `mockarty-cli perf run --from-config` and the server's /api/v1/perf
// endpoints. Field names match the CLI loadProfileConfig / SDK PerfConfig.
type LoadConfig struct {
	Environment map[string]string   `json:"environment,omitempty"`
	Thresholds  map[string][]string `json:"thresholds,omitempty"`
	Name        string              `json:"name,omitempty"`
	Script      string              `json:"script"`
	Duration    string              `json:"duration,omitempty"`
	Stages      []LoadStage         `json:"stages,omitempty"`
	VUs         int                 `json:"vus,omitempty"`
	RPS         int                 `json:"rps,omitempty"`
	MaxVUs      int                 `json:"maxVus,omitempty"`
	Iterations  int                 `json:"iterations,omitempty"`
}

// LoadTest is a fluent builder for a load test. All mutators return the
// receiver for chaining; ToK6Script / ToPerfConfig / SaveConfig are terminal.
type LoadTest struct {
	name       string
	baseURL    string
	requests   []loadRequest
	stages     []LoadStage
	thresholds map[string][]string
	env        map[string]string
	vus        int
	duration   string
	rps        int
	maxVUs     int
	think      float64
	hasThink   bool
}

// NewLoadTest creates a LoadTest with the given name.
func NewLoadTest(name string) *LoadTest {
	if name == "" {
		name = "load-test"
	}
	return &LoadTest{
		name:       name,
		thresholds: map[string][]string{},
		env:        map[string]string{},
	}
}

// Target sets the base URL; request paths are joined onto it and the URL is
// exposed to the script as __ENV.BASE_URL.
func (b *LoadTest) Target(baseURL string) *LoadTest {
	b.baseURL = strings.TrimRight(baseURL, "/")
	if _, ok := b.env["BASE_URL"]; !ok {
		b.env["BASE_URL"] = b.baseURL
	}
	return b
}

// Request appends an arbitrary request to the iteration body.
func (b *LoadTest) Request(method, path string, body any, headers map[string]string) *LoadTest {
	b.requests = append(b.requests, loadRequest{
		method:  strings.ToUpper(method),
		path:    path,
		body:    body,
		headers: headers,
	})
	return b
}

// Get appends a GET request.
func (b *LoadTest) Get(path string) *LoadTest { return b.Request("GET", path, nil, nil) }

// Post appends a POST request with a body (string, or any JSON-serializable value).
func (b *LoadTest) Post(path string, body any) *LoadTest {
	return b.Request("POST", path, body, nil)
}

// Put appends a PUT request with a body.
func (b *LoadTest) Put(path string, body any) *LoadTest { return b.Request("PUT", path, body, nil) }

// Patch appends a PATCH request with a body.
func (b *LoadTest) Patch(path string, body any) *LoadTest {
	return b.Request("PATCH", path, body, nil)
}

// Delete appends a DELETE request.
func (b *LoadTest) Delete(path string) *LoadTest { return b.Request("DELETE", path, nil, nil) }

// VUs sets a constant virtual-user count (ignored when stages are set).
func (b *LoadTest) VUs(n int) *LoadTest { b.vus = n; return b }

// Duration sets a constant run duration ("30s", "5m"); ignored with stages.
func (b *LoadTest) Duration(d string) *LoadTest { b.duration = d; return b }

// RPS targets a steady requests-per-second arrival rate.
func (b *LoadTest) RPS(n int) *LoadTest { b.rps = n; return b }

// MaxVUs caps concurrent VUs (mostly relevant in RPS / arrival-rate mode).
func (b *LoadTest) MaxVUs(n int) *LoadTest { b.maxVUs = n; return b }

// Stages sets the full ramp profile, replacing any prior stages.
func (b *LoadTest) Stages(stages ...LoadStage) *LoadTest {
	b.stages = append([]LoadStage(nil), stages...)
	return b
}

// Threshold adds a pass/fail expression on a metric, e.g.
// Threshold("http_req_duration", "p(95)<500").
func (b *LoadTest) Threshold(metric, expr string) *LoadTest {
	b.thresholds[metric] = append(b.thresholds[metric], expr)
	return b
}

// Env adds an environment variable, exposed as __ENV.<KEY> in the script.
func (b *LoadTest) Env(key, value string) *LoadTest {
	b.env[key] = value
	return b
}

// ThinkTime adds a sleep(seconds) at the end of each iteration.
func (b *LoadTest) ThinkTime(seconds float64) *LoadTest {
	b.think = seconds
	b.hasThink = true
	return b
}

func (b *LoadTest) resolvedRequests() []loadRequest {
	if len(b.requests) > 0 {
		return b.requests
	}
	return []loadRequest{{method: "GET", path: "/"}}
}

// jsStr quotes s as a single-quoted JS string literal.
func jsStr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return "'" + s + "'"
}

// ToK6Script emits a k6-compatible JS script with export const options.
func (b *LoadTest) ToK6Script() string {
	var sb strings.Builder
	sb.WriteString("import http from 'k6/http';\n")
	sb.WriteString("import { check, sleep } from 'k6';\n\n")
	sb.WriteString("export const options = ")
	sb.WriteString(b.optionsJS())
	sb.WriteString(";\n\n")
	// Bake the Target() base URL as a runnable default so the exported script
	// works out of the box (matching the perf engine's own builder pattern),
	// while staying overridable at run time via `-e BASE_URL=...` / __ENV.
	if b.baseURL != "" {
		sb.WriteString("const BASE_URL = __ENV.BASE_URL || " + jsStr(b.baseURL) + ";\n\n")
	}
	sb.WriteString("export default function () {\n")
	sb.WriteString("  let r;\n")
	for _, req := range b.resolvedRequests() {
		sb.WriteString(b.requestJS(req))
	}
	if b.hasThink {
		sb.WriteString(fmt.Sprintf("  sleep(%g);\n", b.think))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func (b *LoadTest) optionsJS() string {
	opts := map[string]any{}
	if len(b.stages) > 0 {
		opts["stages"] = b.stages
	} else {
		if b.vus > 0 {
			opts["vus"] = b.vus
		}
		if b.duration != "" {
			opts["duration"] = b.duration
		}
	}
	if b.rps > 0 {
		opts["rps"] = b.rps
	}
	if b.maxVUs > 0 {
		opts["maxVus"] = b.maxVUs
	}
	if len(b.thresholds) > 0 {
		opts["thresholds"] = b.thresholds
	}
	if len(opts) == 0 {
		opts["vus"] = 1
		opts["duration"] = "30s"
	}
	// Encode without HTML escaping so threshold expressions like
	// "p(95)<500" stay readable (json.Marshal would emit "<").
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(opts)
	return strings.TrimRight(buf.String(), "\n")
}

func (b *LoadTest) requestJS(req loadRequest) string {
	var url string
	if b.baseURL != "" && !strings.HasPrefix(req.path, "http") {
		path := req.path
		if path != "" && !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		url = "`${BASE_URL}" + path + "`"
	} else {
		url = jsStr(req.path)
	}

	headers := map[string]string{}
	for k, v := range req.headers {
		headers[k] = v
	}
	jsonBody := false
	var bodyLit string
	switch v := req.body.(type) {
	case nil:
		// no body
	case string:
		bodyLit = jsStr(v)
	default:
		data, _ := json.Marshal(v)
		bodyLit = jsStr(string(data))
		jsonBody = true
	}
	if jsonBody {
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
	}

	var params string
	if len(headers) > 0 {
		parts := make([]string, 0, len(headers))
		// Deterministic order for stable output.
		for _, k := range sortedKeys(headers) {
			parts = append(parts, jsStr(k)+": "+jsStr(headers[k]))
		}
		params = "{headers: {" + strings.Join(parts, ", ") + "}}"
	}

	method := strings.ToLower(req.method)
	var sb strings.Builder
	if req.body == nil {
		if params != "" {
			sb.WriteString(fmt.Sprintf("  r = http.%s(%s, null, %s);\n", method, url, params))
		} else {
			sb.WriteString(fmt.Sprintf("  r = http.%s(%s);\n", method, url))
		}
	} else {
		if params != "" {
			sb.WriteString(fmt.Sprintf("  r = http.%s(%s, %s, %s);\n", method, url, bodyLit, params))
		} else {
			sb.WriteString(fmt.Sprintf("  r = http.%s(%s, %s);\n", method, url, bodyLit))
		}
	}
	sb.WriteString("  check(r, { 'status < 400': (res) => res.status < 400 });\n")
	return sb.String()
}

// sortedKeys returns map keys in sorted order (small maps, simple insertion).
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

// ToPerfConfig emits the perf-config consumed by the CLI --from-config flag
// and the server /api/v1/perf endpoints. Carries the full profile so a staged
// ramp survives the round-trip.
func (b *LoadTest) ToPerfConfig() *LoadConfig {
	cfg := &LoadConfig{
		Name:   b.name,
		Script: b.ToK6Script(),
		RPS:    b.rps,
		MaxVUs: b.maxVUs,
	}
	if len(b.stages) > 0 {
		cfg.Stages = append([]LoadStage(nil), b.stages...)
	} else {
		cfg.VUs = b.vus
		cfg.Duration = b.duration
	}
	if len(b.thresholds) > 0 {
		cfg.Thresholds = map[string][]string{}
		for k, v := range b.thresholds {
			cfg.Thresholds[k] = append([]string(nil), v...)
		}
	}
	if len(b.env) > 0 {
		cfg.Environment = map[string]string{}
		for k, v := range b.env {
			cfg.Environment[k] = v
		}
	}
	return cfg
}

// ToJSON serializes ToPerfConfig to indented JSON.
func (b *LoadTest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b.ToPerfConfig(), "", "  ")
}

// SaveConfig writes the perf-config JSON to path. Run it with
// `mockarty-cli perf run --from-config <path>`.
func (b *LoadTest) SaveConfig(path string) error {
	data, err := b.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
