package mockarty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ciTestID = "11111111-1111-4111-8111-111111111111"

func ciTestEnvelope() string {
	b, _ := json.Marshal(map[string]any{"format": "mockarty.native-ci-job/v1", "namespace": "qa", "token": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "launchId": ciTestID, "groupId": ciTestID, "attemptId": ciTestID, "invocationId": ciTestID, "items": []any{map[string]any{"itemUid": ciTestID, "caseId": ciTestID, "versionId": ciTestID, "groupId": ciTestID, "attemptId": ciTestID, "jobConfigId": ciTestID, "selector": map[string]string{"id": "selected", "selector": "suite.test"}}}})
	return base64.RawURLEncoding.EncodeToString(b)
}
func TestSelectedCIReportUsesPinnedItemAndPrivateMetadata(t *testing.T) {
	raw := ciTestEnvelope()
	t.Setenv("MOCKARTY_CI_CONTEXT", raw)
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	run := ExternalRunRequest{CaseName: "ordinary", ExternalID: "selected", FullName: "suite.test", Status: "passed", AutoCreate: true, Metadata: map[string]any{"environment": map[string]any{"MOCKARTY_CI_CONTEXT": raw, "safe": "kept"}}}
	_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().Report(context.Background(), "", run)
	if err != nil {
		t.Fatal(err)
	}
	if body["caseId"] != ciTestID || body["ciContext"] == nil || body["autoCreate"] == true {
		t.Fatalf("selected result lost pinned identity or grants ordinary creation")
	}
	meta, _ := json.Marshal(body["metadata"])
	if strings.Contains(string(meta), raw) || strings.Contains(string(meta), "MOCKARTY_CI_CONTEXT") {
		t.Fatal("private job capability leaked into metadata")
	}
	if run.Metadata["environment"].(map[string]any)["MOCKARTY_CI_CONTEXT"] != raw {
		t.Fatal("mutated caller metadata")
	}
}
func TestSelectedCIRefusesUnknownForeignAndSecretAttachments(t *testing.T) {
	for _, scenario := range []string{"unknown", "foreign", "attachment", "malformed"} {
		t.Run(scenario, func(t *testing.T) {
			raw := ciTestEnvelope()
			ns := "qa"
			run := ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed"}
			switch scenario {
			case "unknown":
				run.ExternalID = "not-selected"
			case "foreign":
				ns = "other"
			case "attachment":
				run.Attachments = []ExternalAttachment{{Name: "log", BodyB64: base64.StdEncoding.EncodeToString([]byte(raw))}}
			case "malformed":
				raw = "private-invalid-context"
			}
			t.Setenv("MOCKARTY_CI_CONTEXT", raw)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) }))
			defer srv.Close()
			_, err := NewClient(srv.URL, WithNamespace(ns)).ExternalRuns().Report(context.Background(), "", run)
			if err == nil || calls != 0 || strings.Contains(err.Error(), raw) {
				t.Fatal("unsafe selected report accepted, dispatched or exposed context")
			}
		})
	}
}
func TestSelectedCICompletionAndReportsNeverAutoRetry(t *testing.T) {
	job, err := LoadCIJobContext(ciTestEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(job.String(), job.Token) || strings.Contains(job.Callback().String(), job.Token) {
		t.Fatal("unsafe context formatter")
	}
	calls := 0
	var body map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(503)
		w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer srv.Close()
	api := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns()
	_, err = api.CompleteGroup(context.Background(), "", job.Callback(), "failed")
	if err == nil || calls != 1 || !strings.HasSuffix(path, "/groups/"+ciTestID+"/complete") || body["callback"].(map[string]any)["itemUid"] != "00000000-0000-0000-0000-000000000000" {
		t.Fatal("completion transport or no-retry contract")
	}
	t.Setenv("MOCKARTY_CI_CONTEXT", ciTestEnvelope())
	calls = 0
	_, err = api.Report(context.Background(), "", ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed"})
	if err == nil || calls != 1 {
		t.Fatal("selected report repeated POST")
	}
}
func TestSelectedCIContextBoundsAndAmbiguousOccurrence(t *testing.T) {
	for _, raw := range []string{"", strings.Repeat("a", (1<<20)+1), "e30", ciTestEnvelope() + "="} {
		if _, err := LoadCIJobContext(raw); err == nil {
			t.Fatal("invalid envelope")
		}
	}
	j, e := LoadCIJobContext(ciTestEnvelope())
	if e != nil {
		t.Fatal(e)
	}
	i := j.Items[0]
	i.ItemUID = "22222222-2222-4222-8222-222222222222"
	j.Items = append(j.Items, i)
	if _, e = j.BindReport(ExternalRunRequest{ExternalID: "selected", FullName: "suite.test"}); e == nil {
		t.Fatal("ambiguous occurrence accepted")
	}
}
func TestSelectedCIRejectsDuplicateAndTrailingJSON(t *testing.T) {
	b, _ := base64.RawURLEncoding.DecodeString(ciTestEnvelope())
	for _, value := range []string{strings.Replace(string(b), `"format":`, `"format":"bad","format":`, 1), string(b) + ` {}`} {
		if _, e := LoadCIJobContext(base64.RawURLEncoding.EncodeToString([]byte(value))); e == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
	j, _ := LoadCIJobContext(ciTestEnvelope())
	if strings.Contains(fmt.Sprintf("%#v", j), j.Token) || strings.Contains(fmt.Sprintf("%#v", j.Callback()), j.Token) {
		t.Fatal("diagnostic formatting leaked capability")
	}
}

func TestSelectedCIReportPreservesExactJSONNumbers(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	const exact = "9007199254740993"
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
			t.Error(err)
		}
		body = append([]byte(nil), value...)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	run := ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed", Metadata: map[string]any{"integer": json.Number(exact), "nested": map[string]any{"number": int64(9007199254740993)}}}
	_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().Report(context.Background(), "", run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), exact) != 2 {
		t.Fatal("selected outgoing payload changed exact integer values")
	}
}
