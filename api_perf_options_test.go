// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPerfConfigOptionsMetricsPushWireShape catches a regression where saved
// perf-config options cannot express the server's metricsPush contract and a
// caller silently sends a flattened, ignored profile instead.
func TestPerfConfigOptionsMetricsPushWireShape(t *testing.T) {
	config := PerfConfig{
		Name:   "checkout soak",
		Script: "export default function () {}",
		Options: &PerfOptions{
			VUs:                 12,
			Duration:            "2m",
			MetricsPush:         []string{"prometheus:https://metrics.example/push"},
			MetricsPushInterval: "10s",
			Stages:              []PerfStage{{Duration: "30s", Target: 12}},
		},
	}

	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal saved perf config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode saved perf config: %v", err)
	}
	options, ok := doc["options"].(map[string]any)
	if !ok {
		t.Fatalf("options envelope missing from %s", raw)
	}
	if got := options["metricsPush"]; len(got.([]any)) != 1 || got.([]any)[0] != "prometheus:https://metrics.example/push" {
		t.Fatalf("metricsPush = %#v, want target inside options", got)
	}
	if got := options["metricsPushInterval"]; got != "10s" {
		t.Fatalf("metricsPushInterval = %#v, want 10s", got)
	}
}

// TestPerfConfigGetModelPutPreservesFutureFields catches a forward-compatibility
// regression where GET -> typed SDK model -> PUT drops fields introduced by a
// newer server, rewrites maxVUs to the old spelling, or rejects partial
// stage/criterion responses that use server defaults.
func TestPerfConfigGetModelPutPreservesFutureFields(t *testing.T) {
	const wire = `{
        "id":"cfg-1","collectionId":"col-1","parentId":"folder-1","namespace":"payments","userId":"user-1",
        "name":"nightly","script":"export default function () {}","sortOrder":4,"isFolder":false,
        "environment":{"region":"eu","attempt":2},"createdAt":"2026-08-22T10:00:00Z","updatedAt":"2026-08-22T11:00:00Z",
        "futureConfig":{"keep":true},
        "options":{"maxVus":17,"futureOption":{"keep":true},"stages":[{"duration":"30s"}],"abortCriteria":[{"metric":"http_req_failed"}]}
    }`

	var config PerfConfig
	if err := json.Unmarshal([]byte(wire), &config); err != nil {
		t.Fatalf("unmarshal GET perf config: %v", err)
	}
	if config.CollectionID != "col-1" || config.ParentID == nil || *config.ParentID != "folder-1" || config.UserID != "user-1" {
		t.Fatalf("saved config identity fields did not decode: %+v", config)
	}
	if config.CreatedAt == nil || !config.CreatedAt.Equal(time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("createdAt = %v, want 2026-08-22T10:00:00Z", config.CreatedAt)
	}
	if config.Options == nil || config.Options.MaxVUs != 17 {
		t.Fatalf("legacy maxVus alias did not decode: %+v", config.Options)
	}
	if len(config.Options.Stages) != 1 || config.Options.Stages[0].Target != 0 || len(config.Options.AbortCriteria) != 1 || config.Options.AbortCriteria[0].Enabled {
		t.Fatalf("partial server defaults did not decode safely: %+v", config.Options)
	}

	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal PUT perf config: %v", err)
	}
	var put map[string]any
	if err := json.Unmarshal(raw, &put); err != nil {
		t.Fatalf("decode PUT perf config: %v", err)
	}
	if put["collectionId"] != "col-1" || put["userId"] != "user-1" || put["sortOrder"] != float64(4) || put["isFolder"] != false {
		t.Fatalf("saved config fields lost on PUT: %#v", put)
	}
	if _, ok := put["futureConfig"]; !ok {
		t.Fatalf("future top-level field lost on PUT: %s", raw)
	}
	options := put["options"].(map[string]any)
	if options["maxVUs"] != float64(17) {
		t.Fatalf("canonical maxVUs = %#v, want 17", options["maxVUs"])
	}
	if _, old := options["maxVus"]; old {
		t.Fatalf("legacy maxVus must not be emitted: %s", raw)
	}
	if _, ok := options["futureOption"]; !ok {
		t.Fatalf("future option lost on PUT: %s", raw)
	}
}

func TestPerfOptionsCanonicalMaxVUsWinsRegardlessOfOrderOrNull(t *testing.T) {
	tests := []struct {
		name string
		wire string
		want int
	}{
		{name: "canonical first", wire: `{"maxVUs":23,"maxVus":7}`, want: 23},
		{name: "canonical last", wire: `{"maxVus":7,"maxVUs":23}`, want: 23},
		{name: "canonical null first", wire: `{"maxVUs":null,"maxVus":7}`, want: 0},
		{name: "canonical null last", wire: `{"maxVus":7,"maxVUs":null}`, want: 0},
		{name: "historical case-insensitive input", wire: `{"MAXVUS":31}`, want: 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var options PerfOptions
			if err := json.Unmarshal([]byte(tt.wire), &options); err != nil {
				t.Fatalf("unmarshal options: %v", err)
			}
			if options.MaxVUs != tt.want {
				t.Fatalf("MaxVUs = %d, want %d", options.MaxVUs, tt.want)
			}
			raw, err := json.Marshal(options)
			if err != nil {
				t.Fatalf("marshal options: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("decode options: %v", err)
			}
			if _, legacy := doc["maxVus"]; legacy {
				t.Fatalf("legacy maxVus emitted: %s", raw)
			}
		})
	}
}

func TestPerfConfigExtrasCannotInjectTypedNamesOrAliases(t *testing.T) {
	config := PerfConfig{
		Name: "typed-name",
		Extra: map[string]json.RawMessage{
			"name":     json.RawMessage(`"injected-name"`),
			"parentId": json.RawMessage(`"injected-parent"`),
			"PARENTID": json.RawMessage(`"case-injected-parent"`),
		},
		Options: &PerfOptions{Extra: map[string]json.RawMessage{
			"metricsPush": json.RawMessage(`["injected"]`),
			"maxVUs":      json.RawMessage(`91`),
			"maxVus":      json.RawMessage(`92`),
			"MAXVUS":      json.RawMessage(`93`),
		}},
	}

	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if doc["name"] != "typed-name" {
		t.Fatalf("typed name overwritten: %s", raw)
	}
	for _, protected := range []string{"parentId", "PARENTID"} {
		if _, injected := doc[protected]; injected {
			t.Fatalf("omitted typed parentId variant %q injected from Extra: %s", protected, raw)
		}
	}
	options := doc["options"].(map[string]any)
	for _, protected := range []string{"metricsPush", "maxVUs", "maxVus", "MAXVUS"} {
		if _, injected := options[protected]; injected {
			t.Fatalf("protected option %q injected from Extra: %s", protected, raw)
		}
	}
}

func TestPerfNestedModelsPreserveUnknownFieldsWithoutTypedInjection(t *testing.T) {
	const wire = `{"stages":[{"duration":"30s","futureStage":null}],"abortCriteria":[{"metric":"http_req_failed","futureCriterion":{"keep":true}}]}`
	var options PerfOptions
	if err := json.Unmarshal([]byte(wire), &options); err != nil {
		t.Fatalf("unmarshal nested models: %v", err)
	}
	options.Stages[0].Extra["duration"] = json.RawMessage(`"injected"`)
	options.Stages[0].Extra["targetRPS"] = json.RawMessage(`99`)
	options.AbortCriteria[0].Extra["enabled"] = json.RawMessage(`true`)

	raw, err := json.Marshal(options)
	if err != nil {
		t.Fatalf("marshal nested models: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode nested models: %v", err)
	}
	stage := doc["stages"].([]any)[0].(map[string]any)
	if stage["duration"] != "30s" || stage["futureStage"] != nil {
		t.Fatalf("stage round trip mismatch: %s", raw)
	}
	if _, injected := stage["targetRPS"]; injected {
		t.Fatalf("omitted targetRPS injected from Extra: %s", raw)
	}
	criterion := doc["abortCriteria"].([]any)[0].(map[string]any)
	if criterion["enabled"] != false {
		t.Fatalf("typed enabled overwritten from Extra: %s", raw)
	}
	if criterion["futureCriterion"].(map[string]any)["keep"] != true {
		t.Fatalf("criterion future field lost: %s", raw)
	}
}
