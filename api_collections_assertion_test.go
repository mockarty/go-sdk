// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCollectionAssertionRetainsSkipped(t *testing.T) {
	var result TestResult
	if err := json.Unmarshal([]byte(`{"assertions":[{"name":"optional","passed":false,"skipped":true}]}`), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Assertions) != 1 {
		t.Fatal("assertion lost")
	}
	raw, err := json.Marshal(result.Assertions[0])
	if err != nil {
		t.Fatal(err)
	}
	var assertion map[string]interface{}
	if err := json.Unmarshal(raw, &assertion); err != nil {
		t.Fatal(err)
	}
	if assertion["skipped"] != true {
		t.Fatalf("skipped flag lost: %s", raw)
	}
}

func TestCollectionExecutionRetainsScriptOutcome(t *testing.T) {
	payload := `{"status":"skipped","totalTests":1,"passedTests":0,"failedTests":0,"skippedTests":1,"report":{"status":"skipped","skippedTests":1,"requestReports":[{"status":"skipped","skippedTests":1,"preRequestResult":{"success":true,"skipRequest":true},"tests":[{"name":"optional","passed":false,"skipped":true}]}]}}`
	var result TestRunResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	report, ok := decoded["report"].(map[string]any)
	if !ok {
		t.Fatalf("collection execution report lost: %s", raw)
	}
	if report["skippedTests"] != float64(1) {
		t.Fatalf("skipped counter lost: %s", raw)
	}
	step := report["requestReports"].([]any)[0].(map[string]any)
	pre := step["preRequestResult"].(map[string]any)
	if pre["skipRequest"] != true || step["skippedTests"] != float64(1) {
		t.Fatalf("script outcome lost: %s", raw)
	}
}

func TestCollectionExecutePreservesInteractionJSON(t *testing.T) {
	for _, value := range []string{`{"body":"ok"}`, `[1,"event"]`, `"text"`, `42`, `0`, `null`} {
		t.Run(value, func(t *testing.T) {
			payload := fmt.Sprintf(`{"status":"passed","report":{"perfReport":%s,"requestReports":[{"interactions":[{"kind":"http","requestData":%s,"responseData":%s}]}]}}`, value, value, value)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/v1/api-tester/collections/collection-id/execute" {
					t.Errorf("wrong execute request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(payload))
			}))
			defer srv.Close()
			result, err := NewClient(srv.URL).Collections().Execute(t.Context(), "collection-id")
			if err != nil {
				t.Fatal(err)
			}
			if result.Report == nil || len(result.Report.RequestReports) != 1 || len(result.Report.RequestReports[0].Interactions) != 1 {
				t.Fatalf("interaction report lost: %+v", result)
			}
			var want any
			if err := json.Unmarshal([]byte(value), &want); err != nil {
				t.Fatal(err)
			}
			interaction := result.Report.RequestReports[0].Interactions[0]
			if !reflect.DeepEqual(want, interaction.RequestData) || !reflect.DeepEqual(want, interaction.ResponseData) || !reflect.DeepEqual(want, result.Report.PerfReport) {
				t.Fatalf("arbitrary JSON changed for %s: %+v", value, result.Report)
			}
		})
	}
}
