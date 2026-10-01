package mockarty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestSelectedCISelectorPresentTypes(t *testing.T) {
	for _, key := range []string{"id", "selector"} {
		for _, value := range []string{"null", "false", "0", "[]", "{}"} {
			t.Run(key+value, func(t *testing.T) {
				b, _ := base64.RawURLEncoding.DecodeString(ciTestEnvelope())
				var body map[string]any
				json.Unmarshal(b, &body)
				selector := body["items"].([]any)[0].(map[string]any)["selector"].(map[string]any)
				selector[key] = json.RawMessage(value)
				b, _ = json.Marshal(body)
				if _, e := LoadCIJobContext(base64.RawURLEncoding.EncodeToString(b)); e == nil {
					t.Fatal("nonstring selector accepted")
				}
			})
		}
	}
}

func TestSelectedCIAllureDirectoryFiltersBeforeMedia(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "selected-result.json"), []byte(`{"uuid":"selected","fullName":"suite.test","name":"case","status":"passed"}`), 0600)
	os.WriteFile(filepath.Join(dir, "outsider-result.json"), []byte(`{"uuid":"other","fullName":"other.test","name":"other","status":"passed","attachments":[{"name":"absent","source":"missing.txt"}]}`), 0600)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer srv.Close()
	out, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{AutoCreate: true})
	if err != nil || len(out) != 1 || calls != 1 {
		t.Fatal("unselected missing media affected selected upload")
	}
}

func TestSelectedCIAllureDirectoryRejectsDuplicateBeforeHTTP(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	dir := t.TempDir()
	doc := []byte(`{"uuid":"selected","fullName":"suite.test","name":"case","status":"passed"}`)
	for _, name := range []string{"a-result.json", "b-result.json"} {
		os.WriteFile(filepath.Join(dir, name), doc, 0600)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer srv.Close()
	_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{})
	if err == nil || calls != 0 {
		t.Fatal("duplicate selected occurrence dispatched")
	}
}

func TestSelectedCIAllureDirectoryPreflightsAllSelectedMedia(t *testing.T) {
	raw, _ := base64.RawURLEncoding.DecodeString(ciTestEnvelope())
	var job map[string]any
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	first := job["items"].([]any)[0].(map[string]any)
	second := map[string]any{}
	for key, value := range first {
		second[key] = value
	}
	second["itemUid"] = "22222222-2222-4222-8222-222222222222"
	second["selector"] = map[string]string{"id": "second", "selector": "suite.second"}
	job["items"] = []any{first, second}
	raw, _ = json.Marshal(job)
	t.Setenv(CIContextEnvironment, base64.RawURLEncoding.EncodeToString(raw))
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a-result.json": `{"uuid":"selected","fullName":"suite.test","name":"first","status":"passed"}`,
		"b-result.json": `{"uuid":"second","fullName":"suite.second","name":"second","status":"passed","attachments":[{"name":"secret","source":"secret.txt"}]}`,
		"secret.txt":    job["token"].(string),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer srv.Close()
	_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{})
	if err == nil || calls != 0 {
		t.Fatal("later selected secret permitted an earlier HTTP report")
	}
}

func TestSelectedCIAllureDirectoryRejectsTrailingJSON(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "a-result.json"), []byte(`{"uuid":"selected","fullName":"suite.test","name":"case","status":"passed"} {}`), 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer srv.Close()
	_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{})
	if err == nil || calls != 0 {
		t.Fatal("trailing Allure JSON was reported")
	}
}

func TestSelectedCIReportBatchValidatesEveryBodyBeforeHTTP(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	for _, kind := range []string{"status", "step", "inline"} {
		t.Run(kind, func(t *testing.T) {
			good := ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed"}
			bad := good
			switch kind {
			case "status":
				bad.Status = "invalid"
			case "step":
				bad.Steps = []ExternalStep{{Name: " ", Status: "passed"}}
			case "inline":
				bad.Attachments = []ExternalAttachment{{Name: "large", BodyB64: base64.StdEncoding.EncodeToString(make([]byte, (256<<10)+1))}}
			}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
			defer srv.Close()
			_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().ReportBatch(context.Background(), "", []ExternalRunRequest{good, bad})
			if err == nil || calls != 0 {
				t.Fatal("invalid later selected body reached HTTP")
			}
		})
	}
}

func TestSelectedCIReportBatchStepBoundary(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	for _, count := range []int{1000, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			good := ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed"}
			bad := good
			for i := 0; i < count; i++ {
				bad.Steps = append(bad.Steps, ExternalStep{Name: "step", Status: "passed"})
			}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
			defer srv.Close()
			_, err := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().ReportBatch(context.Background(), "", []ExternalRunRequest{good, bad})
			if count == 1000 {
				if err != nil || calls != 1 {
					t.Fatal("valid selected step limit refused")
				}
			} else if err == nil || calls != 0 {
				t.Fatal("oversized later selected steps reached HTTP")
			}
		})
	}
}

func TestSelectedCIExactInlineAggregateLimit(t *testing.T) {
	t.Setenv(CIContextEnvironment, ciTestEnvelope())
	run := ExternalRunRequest{ExternalID: "selected", FullName: "suite.test", Status: "passed"}
	for i := 0; i < 4; i++ {
		run.Attachments = append(run.Attachments, ExternalAttachment{Name: fmt.Sprint(i), BodyB64: base64.StdEncoding.EncodeToString(make([]byte, 256<<10))})
	}
	run.Metadata = map[string]any{"bodyB64": "customer notes", "nested": map[string]any{"bodyB64": run.Attachments[0].BodyB64}}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var outgoing ExternalRunRequest
		if e := json.NewDecoder(r.Body).Decode(&outgoing); e != nil {
			t.Error(e)
		}
		if outgoing.Metadata["bodyB64"] != "customer notes" {
			t.Error("arbitrary metadata changed")
		}
		metadata, _ := json.Marshal(outgoing.Metadata)
		job, _ := LoadCIJobContext(ciTestEnvelope())
		if strings.Contains(string(metadata), job.Token) {
			t.Error("metadata escaped redaction")
		}
		if len(outgoing.Attachments) != 4 {
			t.Error("attachment count changed")
		}
		for _, attachment := range outgoing.Attachments {
			if attachment.BodyB64 != run.Attachments[0].BodyB64 {
				t.Error("opaque bytes changed")
			}
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, e := NewClient(srv.URL, WithNamespace("qa")).ExternalRuns().Report(context.Background(), "", run); e != nil || calls != 1 {
		t.Fatal("valid exact decoded aggregate limit refused")
	}
}
