// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Generator API Tests
// ---------------------------------------------------------------------------

func TestGeneratorAPI_FromOpenAPI(t *testing.T) {
	var gotBody GeneratorRequest
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/openapi/generate": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":3,"message":"3 mocks generated"}`))
		},
	})

	resp, err := client.Generator().FromOpenAPI(context.Background(), &GeneratorRequest{
		Spec:      `{"openapi":"3.0.0"}`,
		Namespace: "test-ns",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 3 {
		t.Errorf("expected 3 created, got %d", resp.Created)
	}
	if gotBody.Spec == "" {
		t.Error("expected spec in request body")
	}
}

func TestGeneratorAPI_FromOpenAPI_DefaultNamespace(t *testing.T) {
	var gotBody GeneratorRequest
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/openapi/generate": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":1}`))
		},
	})

	_, _ = client.Generator().FromOpenAPI(context.Background(), &GeneratorRequest{
		URL: "https://example.com/spec.yaml",
	})
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Namespace)
	}
}

func TestGeneratorAPI_FromWSDL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/soap/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":2,"message":"2 SOAP mocks generated"}`))
		},
	})

	resp, err := client.Generator().FromWSDL(context.Background(), &GeneratorRequest{
		Spec: "<wsdl:definitions/>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 2 {
		t.Errorf("expected 2 created, got %d", resp.Created)
	}
}

func TestGeneratorAPI_FromProto(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/grpc/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":5}`))
		},
	})

	resp, err := client.Generator().FromProto(context.Background(), &GeneratorRequest{
		Spec: "syntax = \"proto3\";",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 5 {
		t.Errorf("expected 5 created, got %d", resp.Created)
	}
}

func TestGeneratorAPI_FromGraphQL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/graphql/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":4}`))
		},
	})

	resp, err := client.Generator().FromGraphQL(context.Background(), &GeneratorRequest{
		GraphQLURL: "https://example.com/graphql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 4 {
		t.Errorf("expected 4 created, got %d", resp.Created)
	}
}

func TestGeneratorAPI_FromHAR(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/har/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":10}`))
		},
	})

	resp, err := client.Generator().FromHAR(context.Background(), &GeneratorRequest{
		Spec: `{"log":{}}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 10 {
		t.Errorf("expected 10 created, got %d", resp.Created)
	}
}

func TestGeneratorAPI_FromSocket(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/socket/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"created":1}`))
		},
	})

	resp, err := client.Generator().FromSocket(context.Background(), &GeneratorRequest{
		ServerName: "ws-server",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Created != 1 {
		t.Errorf("expected 1 created, got %d", resp.Created)
	}
}

func TestGeneratorAPI_PreviewOpenAPI(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/openapi/preview": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"mocks":[{"id":"preview-1"}],"count":1}`))
		},
	})

	resp, err := client.Generator().PreviewOpenAPI(context.Background(), &GeneratorRequest{
		Spec: `{"openapi":"3.0.0"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
	if len(resp.Mocks) != 1 {
		t.Errorf("expected 1 mock in preview, got %d", len(resp.Mocks))
	}
}

func TestGeneratorAPI_PreviewWSDL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/soap/preview": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"mocks":[],"count":0}`))
		},
	})

	resp, err := client.Generator().PreviewWSDL(context.Background(), &GeneratorRequest{
		Spec: "<wsdl/>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("expected count 0, got %d", resp.Count)
	}
}

func TestGeneratorAPI_PreviewProto(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/grpc/preview": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"mocks":[{"id":"grpc-1"},{"id":"grpc-2"}],"count":2}`))
		},
	})

	resp, err := client.Generator().PreviewProto(context.Background(), &GeneratorRequest{
		Spec: "syntax = \"proto3\";",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 2 {
		t.Errorf("expected count 2, got %d", resp.Count)
	}
}

func TestGeneratorAPI_PreviewGraphQL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/graphql/preview": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"mocks":[{"id":"gql-1"}],"count":1}`))
		},
	})

	resp, err := client.Generator().PreviewGraphQL(context.Background(), &GeneratorRequest{
		GraphQLURL: "https://example.com/graphql",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
}

func TestGeneratorAPI_PreviewHAR(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/har/preview": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"mocks":[{"id":"har-1"}],"count":1}`))
		},
	})

	resp, err := client.Generator().PreviewHAR(context.Background(), &GeneratorRequest{
		Spec: `{"log":{}}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
}

func TestGeneratorAPI_ServerError(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/generator/openapi/generate": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid spec"}`))
		},
	})

	_, err := client.Generator().FromOpenAPI(context.Background(), &GeneratorRequest{Spec: "bad"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Fuzzing API Tests
// ---------------------------------------------------------------------------

func TestFuzzingAPI_Start(t *testing.T) {
	var gotBody FuzzingConfig
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/fuzzing/run": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"fuzz-run-1","status":"running"}`))
		},
	})

	run, err := client.Fuzzing().Start(context.Background(), &FuzzingConfig{
		Name:      "security-fuzz",
		TargetURL: "https://api.example.com",
		Duration:  "5m",
		Workers:   4,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "fuzz-run-1" {
		t.Errorf("expected ID fuzz-run-1, got %q", run.ID)
	}
	if run.Status != "running" {
		t.Errorf("expected status running, got %q", run.Status)
	}
	if gotBody.Name != "security-fuzz" {
		t.Errorf("expected name security-fuzz, got %q", gotBody.Name)
	}
}

func TestFuzzingAPI_Start_DefaultNamespace(t *testing.T) {
	var gotBody FuzzingConfig
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/fuzzing/run": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"fuzz-1","status":"running"}`))
		},
	})

	_, _ = client.Fuzzing().Start(context.Background(), &FuzzingConfig{
		TargetURL: "https://api.example.com",
	})
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Namespace)
	}
}

func TestFuzzingAPI_Stop(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Fuzzing().Stop(context.Background(), "fuzz-run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "fuzz-run-1/stop") {
		t.Errorf("expected path to contain fuzz-run-1/stop, got %s", gotPath)
	}
}

func TestFuzzingAPI_GetResult(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"fuzz-1","status":"completed","totalRequests":1000,"findings":3}`))
		},
	})

	result, err := client.Fuzzing().GetResult(context.Background(), "fuzz-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 1000 {
		t.Errorf("expected 1000 total requests, got %d", result.TotalRequests)
	}
	if result.Findings != 3 {
		t.Errorf("expected 3 findings, got %d", result.Findings)
	}
}

func TestFuzzingAPI_ListResults(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/fuzzing/results": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"r1","status":"completed"},{"id":"r2","status":"running"}]`))
		},
	})

	results, err := client.Fuzzing().ListResults(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestFuzzingAPI_DeleteResult(t *testing.T) {
	var gotMethod, gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Fuzzing().DeleteResult(context.Background(), "fuzz-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "fuzz-1") {
		t.Errorf("expected path to contain fuzz-1, got %s", gotPath)
	}
}

func TestFuzzingAPI_CreateConfig(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/fuzzing/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"my-config","targetUrl":"https://api.example.com"}`))
		},
	})

	config, err := client.Fuzzing().CreateConfig(context.Background(), &FuzzingConfig{
		Name:      "my-config",
		TargetURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.ID != "cfg-1" {
		t.Errorf("expected ID cfg-1, got %q", config.ID)
	}
}

func TestFuzzingAPI_GetConfig(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/fuzzing/configs/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"my-config","workers":8}`))
		},
	})

	config, err := client.Fuzzing().GetConfig(context.Background(), "cfg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Workers != 8 {
		t.Errorf("expected 8 workers, got %d", config.Workers)
	}
}

// ---------------------------------------------------------------------------
// Contract API Tests
// ---------------------------------------------------------------------------

func TestContractAPI_ValidateMocks(t *testing.T) {
	var gotBody ContractValidationRequest
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/contract/validate-mocks": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"result-1","status":"fail","violations":2}`))
		},
	})

	result, err := client.Contracts().ValidateMocks(context.Background(), &ContractValidationRequest{
		SpecURL:   "https://example.com/openapi.yaml",
		TargetURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "fail" {
		t.Errorf("expected status fail, got %q", result.Status)
	}
	if result.Violations != 2 {
		t.Errorf("expected 2 violations, got %d", result.Violations)
	}
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Namespace)
	}
}

func TestContractAPI_ListConfigs(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/contract/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"c1","name":"Config A"},{"id":"c2","name":"Config B"}]`))
		},
	})

	configs, err := client.Contracts().ListConfigs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
}

func TestContractAPI_SaveConfig(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/contract/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"My Contract Config"}`))
		},
	})

	config, err := client.Contracts().SaveConfig(context.Background(), &ContractConfig{
		Name:    "My Contract Config",
		SpecURL: "https://example.com/openapi.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.ID != "cfg-1" {
		t.Errorf("expected ID cfg-1, got %q", config.ID)
	}
}

func TestContractAPI_DeleteConfig(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/contract/configs/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Contracts().DeleteConfig(context.Background(), "cfg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "cfg-1") {
		t.Errorf("expected path to contain cfg-1, got %s", gotPath)
	}
}

func TestContractAPI_ListResults(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/contract/results": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"r1","status":"pass"},{"id":"r2","status":"fail","violations":1}]`))
		},
	})

	results, err := client.Contracts().ListResults(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Recorder API Tests
// ---------------------------------------------------------------------------

func TestRecorderAPI_StartRecording(t *testing.T) {
	var gotBody RecorderSession
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/recorder/start": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"session-1","name":"My Recording","status":"recording"}`))
		},
	})

	session, err := client.Recorder().StartRecording(context.Background(), &RecorderSession{
		Name:      "My Recording",
		TargetURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "session-1" {
		t.Errorf("expected ID session-1, got %q", session.ID)
	}
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Namespace)
	}
}

func TestRecorderAPI_GetSession(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"session-1","name":"My Recording","status":"recording","entryCount":42}`))
		},
	})

	session, err := client.Recorder().GetSession(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.EntryCount != 42 {
		t.Errorf("expected 42 entries, got %d", session.EntryCount)
	}
}

func TestRecorderAPI_ListSessions(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/recorder/sessions": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"s1","status":"idle"},{"id":"s2","status":"recording"}]`))
		},
	})

	sessions, err := client.Recorder().ListSessions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestRecorderAPI_StopRecording(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Recorder().StopRecording(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "session-1/stop") {
		t.Errorf("expected path to contain session-1/stop, got %s", gotPath)
	}
}

func TestRecorderAPI_DeleteSession(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Recorder().DeleteSession(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestRecorderAPI_GetEntries(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"e1","method":"GET","path":"/api/users","statusCode":200},{"id":"e2","method":"POST","path":"/api/users","statusCode":201}]`))
		},
	})

	entries, err := client.Recorder().GetEntries(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Method != "GET" {
		t.Errorf("expected first entry method GET, got %q", entries[0].Method)
	}
}

func TestRecorderAPI_CreateMocksFromSession(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"mock-1","http":{"route":"/api/users","httpMethod":"GET"}}]`))
		},
	})

	mocks, err := client.Recorder().CreateMocksFromSession(context.Background(), "session-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mocks) != 1 {
		t.Errorf("expected 1 mock, got %d", len(mocks))
	}
	if !strings.Contains(gotPath, "session-1/mocks") {
		t.Errorf("expected path to contain session-1/mocks, got %s", gotPath)
	}
}

func TestRecorderAPI_ExportSession(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/recorder/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"log":{"entries":[]}}`))
		},
	})

	data, err := client.Recorder().ExportSession(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}
}

// ---------------------------------------------------------------------------
// Template API Tests
// ---------------------------------------------------------------------------

func TestTemplateAPI_List(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/templates": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"name":"response.json","size":1024},{"name":"error.xml","size":512}]`))
		},
	})

	files, err := client.Templates().List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if files[0].Name != "response.json" {
		t.Errorf("expected first file name response.json, got %q", files[0].Name)
	}
}

func TestTemplateAPI_Get(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/templates/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"key":"value"}`))
		},
	})

	data, err := client.Templates().Get(context.Background(), "response.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("unexpected content: %s", string(data))
	}
}

func TestTemplateAPI_Upload(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/templates/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Templates().Upload(context.Background(), "new-template.json", []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["content"] != `{"hello":"world"}` {
		t.Errorf("unexpected content in body: %v", gotBody["content"])
	}
}

func TestTemplateAPI_Delete(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/templates/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Templates().Delete(context.Background(), "old-template.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

// ---------------------------------------------------------------------------
// Import API Tests
// ---------------------------------------------------------------------------

func TestImportAPI_Postman(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/postman": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-1","name":"Postman Collection","imported":15}`))
		},
	})

	result, err := client.Import().Postman(context.Background(), []byte(`{"info":{"name":"test"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 15 {
		t.Errorf("expected 15 imported, got %d", result.Imported)
	}
	if result.CollectionID != "col-1" {
		t.Errorf("expected collectionId col-1, got %q", result.CollectionID)
	}
}

func TestImportAPI_OpenAPI(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/openapi": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-2","imported":8}`))
		},
	})

	result, err := client.Import().OpenAPI(context.Background(), []byte(`openapi: "3.0.0"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 8 {
		t.Errorf("expected 8 imported, got %d", result.Imported)
	}
}

func TestImportAPI_WSDL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/wsdl": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-3","imported":4}`))
		},
	})

	result, err := client.Import().WSDL(context.Background(), []byte(`<wsdl:definitions/>`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 4 {
		t.Errorf("expected 4 imported, got %d", result.Imported)
	}
}

func TestImportAPI_HAR(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/har": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-4","imported":20}`))
		},
	})

	result, err := client.Import().HAR(context.Background(), []byte(`{"log":{}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 20 {
		t.Errorf("expected 20 imported, got %d", result.Imported)
	}
}

func TestImportAPI_GrpcProto(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/grpc": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-5","imported":6}`))
		},
	})

	result, err := client.Import().GrpcProto(context.Background(), []byte(`syntax = "proto3";`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 6 {
		t.Errorf("expected 6 imported, got %d", result.Imported)
	}
}

func TestImportAPI_GraphQL(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/graphql": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-6","imported":3}`))
		},
	})

	result, err := client.Import().GraphQL(context.Background(), []byte(`type Query { hello: String }`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 3 {
		t.Errorf("expected 3 imported, got %d", result.Imported)
	}
}

func TestImportAPI_MCP(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/mcp": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-7","imported":2}`))
		},
	})

	result, err := client.Import().MCP(context.Background(), []byte(`{"tools":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 2 {
		t.Errorf("expected 2 imported, got %d", result.Imported)
	}
}

func TestImportAPI_Mockarty(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/mockarty": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"collectionId":"col-8","imported":12}`))
		},
	})

	result, err := client.Import().Mockarty(context.Background(), []byte(`{"mocks":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Imported != 12 {
		t.Errorf("expected 12 imported, got %d", result.Imported)
	}
}

func TestImportAPI_ServerError(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/import/postman": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid format"}`))
		},
	})

	_, err := client.Import().Postman(context.Background(), []byte(`bad data`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestRun API Tests
// ---------------------------------------------------------------------------

func TestTestRunAPI_List(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/api-tester/test-runs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"run-1","status":"completed","totalTests":10,"passedTests":8,"failedTests":2}]`))
		},
	})

	runs, err := client.TestRuns().List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}
	if runs[0].TotalTests != 10 {
		t.Errorf("expected 10 total tests, got %d", runs[0].TotalTests)
	}
}

func TestTestRunAPI_Get(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"run-1","status":"completed","passedTests":8,"failedTests":2}`))
		},
	})

	run, err := client.TestRuns().Get(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.PassedTests != 8 {
		t.Errorf("expected 8 passed, got %d", run.PassedTests)
	}
}

func TestTestRunAPI_Cancel(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.TestRuns().Cancel(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "run-1/cancel") {
		t.Errorf("expected path to contain run-1/cancel, got %s", gotPath)
	}
}

func TestTestRunAPI_Delete(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.TestRuns().Delete(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestTestRunAPI_Export(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"results":[{"id":"r1","status":"passed"}]}`))
		},
	})

	data, err := client.TestRuns().Export(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty export data")
	}
}

func TestTestRunAPI_ImportReport(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/reports/import": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.TestRuns().ImportReport(context.Background(), []byte(`{"report":"data"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["data"] == nil {
		t.Error("expected data field in body")
	}
}

// ---------------------------------------------------------------------------
// Admin API Tests
// ---------------------------------------------------------------------------

func TestAdminAPI_ListUsers(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/users": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"u1","username":"alice","role":"admin"},{"id":"u2","username":"bob","role":"user"}]`))
		},
	})

	users, err := client.Admin().ListUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Errorf("expected first user alice, got %q", users[0].Username)
	}
}

func TestAdminAPI_CreateUser(t *testing.T) {
	var gotBody CreateUserRequest
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/users": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"u3","username":"charlie","email":"charlie@example.com","role":"user"}`))
		},
	})

	user, err := client.Admin().CreateUser(context.Background(), &CreateUserRequest{
		Username: "charlie",
		Email:    "charlie@example.com",
		Password: "s3cret",
		Role:     "user",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "charlie" {
		t.Errorf("expected username charlie, got %q", user.Username)
	}
	if gotBody.Password != "s3cret" {
		t.Errorf("expected password in body, got %q", gotBody.Password)
	}
}

func TestAdminAPI_UpdateUser(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /ui/api/admin/users/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","role":"support"}`))
		},
	})

	user, err := client.Admin().UpdateUser(context.Background(), "u1", &UpdateUserRequest{
		Role: "support",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != "support" {
		t.Errorf("expected role support, got %q", user.Role)
	}
}

func TestAdminAPI_DeleteUser(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/admin/users/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().DeleteUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestAdminAPI_SetUserPassword(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/users/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().SetUserPassword(context.Background(), "u1", "new-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["password"] != "new-password" {
		t.Errorf("expected password in body, got %v", gotBody["password"])
	}
}

func TestAdminAPI_DisableEnableUser(t *testing.T) {
	var gotPaths []string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/users/": func(w http.ResponseWriter, r *http.Request) {
			gotPaths = append(gotPaths, r.URL.Path)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().DisableUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}

	err = client.Admin().EnableUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	if len(gotPaths) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(gotPaths))
	}
	if !strings.Contains(gotPaths[0], "/disable") {
		t.Errorf("expected first call to /disable, got %s", gotPaths[0])
	}
	if !strings.Contains(gotPaths[1], "/enable") {
		t.Errorf("expected second call to /enable, got %s", gotPaths[1])
	}
}

func TestAdminAPI_ListNamespaces(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/namespaces": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"name":"sandbox","mockCount":10,"userCount":3}]`))
		},
	})

	namespaces, err := client.Admin().ListNamespaces(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(namespaces) != 1 {
		t.Errorf("expected 1 namespace, got %d", len(namespaces))
	}
	if namespaces[0].MockCount != 10 {
		t.Errorf("expected 10 mocks, got %d", namespaces[0].MockCount)
	}
}

func TestAdminAPI_DeleteNamespace(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().DeleteNamespace(context.Background(), "old-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "old-ns") {
		t.Errorf("expected path to contain old-ns, got %s", gotPath)
	}
}

func TestAdminAPI_RestoreNamespace(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().RestoreNamespace(context.Background(), "old-ns")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "old-ns/restore") {
		t.Errorf("expected path to contain old-ns/restore, got %s", gotPath)
	}
}

func TestAdminAPI_NamespaceUsers(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"userId":"u1","username":"alice","role":"owner"}]`))
		},
		"POST /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		"DELETE /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		"PUT /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	users, err := client.Admin().ListNamespaceUsers(context.Background(), "sandbox")
	if err != nil {
		t.Fatalf("unexpected error listing users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}

	err = client.Admin().AddNamespaceUser(context.Background(), "sandbox", &AddNamespaceUserRequest{
		UserID: "u2",
		Role:   "editor",
	})
	if err != nil {
		t.Fatalf("unexpected error adding user: %v", err)
	}

	err = client.Admin().RemoveNamespaceUser(context.Background(), "sandbox", "u2")
	if err != nil {
		t.Fatalf("unexpected error removing user: %v", err)
	}

	err = client.Admin().UpdateNamespaceUserRole(context.Background(), "sandbox", "u1", "viewer")
	if err != nil {
		t.Fatalf("unexpected error updating role: %v", err)
	}
}

func TestAdminAPI_ListBackupConfigs(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/backups/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"cfg-1","name":"daily","schedule":"0 2 * * *","retention":30,"enabled":true}]`))
		},
	})

	configs, err := client.Admin().ListBackupConfigs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing configs: %v", err)
	}
	if len(configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(configs))
	}
}

func TestAdminAPI_GetBackupConfig(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/backups/configs/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"daily"}`))
		},
	})

	config, err := client.Admin().GetBackupConfig(context.Background(), "cfg-1")
	if err != nil {
		t.Fatalf("unexpected error getting config: %v", err)
	}
	if config.Name != "daily" {
		t.Errorf("expected name daily, got %q", config.Name)
	}
}

func TestAdminAPI_CreateBackupConfig(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/backups/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-2","name":"weekly"}`))
		},
	})

	created, err := client.Admin().CreateBackupConfig(context.Background(), &BackupConfig{
		Name:     "weekly",
		Schedule: "0 3 * * 0",
	})
	if err != nil {
		t.Fatalf("unexpected error creating config: %v", err)
	}
	if created.ID != "cfg-2" {
		t.Errorf("expected ID cfg-2, got %q", created.ID)
	}
}

func TestAdminAPI_Backup(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/backups/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"bkp-1","configId":"cfg-1","status":"completed"}`))
		},
		"GET /ui/api/admin/backups/download": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`backup-binary-data`))
		},
		"POST /ui/api/admin/backups/restore": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
		"POST /ui/api/admin/backups/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	backup, err := client.Admin().CreateBackup(context.Background(), "cfg-1")
	if err != nil {
		t.Fatalf("unexpected error creating backup: %v", err)
	}
	if backup.Status != "completed" {
		t.Errorf("expected status completed, got %q", backup.Status)
	}

	data, err := client.Admin().DownloadBackup(context.Background(), "bkp-1")
	if err != nil {
		t.Fatalf("unexpected error downloading backup: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty backup data")
	}

	err = client.Admin().RestoreBackup(context.Background(), "bkp-1")
	if err != nil {
		t.Fatalf("unexpected error restoring backup: %v", err)
	}

	err = client.Admin().DeleteBackup(context.Background(), "bkp-1")
	if err != nil {
		t.Fatalf("unexpected error deleting backup: %v", err)
	}
}

func TestAdminAPI_LicenseStatus(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/licenses/status": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"active":true,"type":"enterprise","maxUsers":100,"maxMocks":10000}`))
		},
	})

	status, err := client.Admin().GetLicenseStatus(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Active {
		t.Error("expected active license")
	}
	if status.Type != "enterprise" {
		t.Errorf("expected type enterprise, got %q", status.Type)
	}
}

func TestAdminAPI_ListLicenses(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/licenses": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"lic-1","type":"enterprise","status":"active"}]`))
		},
	})

	licenses, err := client.Admin().ListLicenses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(licenses) != 1 {
		t.Errorf("expected 1 license, got %d", len(licenses))
	}
}

func TestAdminAPI_ActivateLicense(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/admin/licenses/activate": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Admin().ActivateLicense(context.Background(), "LICENSE-KEY-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["key"] != "LICENSE-KEY-123" {
		t.Errorf("expected key in body, got %v", gotBody["key"])
	}
}

func TestAdminAPI_LicenseUsage(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/licenses/usage": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"users":5,"mocks":250,"namespaces":3}`))
		},
	})

	usage, err := client.Admin().GetLicenseUsage(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.Users != 5 {
		t.Errorf("expected 5 users, got %d", usage.Users)
	}
	if usage.Mocks != 250 {
		t.Errorf("expected 250 mocks, got %d", usage.Mocks)
	}
}

func TestAdminAPI_CombinedLimits(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/licenses/combined-limits": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"maxUsers":100,"maxMocks":10000,"aiEnabled":true,"perfEnabled":true,"fuzzEnabled":false}`))
		},
	})

	limits, err := client.Admin().GetCombinedLimits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !limits.AIEnabled {
		t.Error("expected AI enabled")
	}
	if limits.FuzzEnabled {
		t.Error("expected fuzz disabled")
	}
}

func TestAdminAPI_Webhooks(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/webhooks": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":"wh-1","url":"https://example.com/hook","events":["mock.create"],"enabled":true}]`))
		},
		"POST /ui/api/admin/webhooks": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"wh-2","url":"https://example.com/hook2","enabled":true}`))
		},
		"DELETE /ui/api/admin/webhooks/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	webhooks, err := client.Admin().ListWebhooks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing webhooks: %v", err)
	}
	if len(webhooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(webhooks))
	}

	created, err := client.Admin().CreateWebhook(context.Background(), &AdminWebhook{
		URL:    "https://example.com/hook2",
		Events: []string{"mock.update"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating webhook: %v", err)
	}
	if created.ID != "wh-2" {
		t.Errorf("expected ID wh-2, got %q", created.ID)
	}

	err = client.Admin().DeleteWebhook(context.Background(), "wh-1")
	if err != nil {
		t.Fatalf("unexpected error deleting webhook: %v", err)
	}
}

func TestAdminAPI_ExportAuditLogs(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/audit/export": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"action":"mock.create","user":"alice","timestamp":"2024-01-01T00:00:00Z"}]`))
		},
	})

	data, err := client.Admin().ExportAuditLogs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty audit export")
	}
}

func TestAdminAPI_CleanupPolicy(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/cleanup-policy") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"mockRetentionDays":30,"logRetentionDays":7,"autoCleanup":true}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
		"PUT /ui/api/admin/namespaces/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	policy, err := client.Admin().GetCleanupPolicy(context.Background(), "sandbox")
	if err != nil {
		t.Fatalf("unexpected error getting policy: %v", err)
	}
	if policy.MockRetentionDays != 30 {
		t.Errorf("expected 30 days retention, got %d", policy.MockRetentionDays)
	}
	if !policy.AutoCleanup {
		t.Error("expected auto cleanup enabled")
	}

	err = client.Admin().UpdateCleanupPolicy(context.Background(), "sandbox", &CleanupPolicy{
		MockRetentionDays: 60,
		AutoCleanup:       false,
	})
	if err != nil {
		t.Fatalf("unexpected error updating policy: %v", err)
	}
}

func TestAdminAPI_DatabaseHealth(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /ui/api/admin/database/health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","tables":42,"connections":5}`))
		},
	})

	health, err := client.Admin().GetDatabaseHealth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if health.Status != "healthy" {
		t.Errorf("expected status healthy, got %q", health.Status)
	}
	if health.Tables != 42 {
		t.Errorf("expected 42 tables, got %d", health.Tables)
	}
}

// ---------------------------------------------------------------------------
// Collection API Extended Tests (new CRUD methods)
// ---------------------------------------------------------------------------

func TestCollectionAPI_Create(t *testing.T) {
	var gotBody Collection
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/collections": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"col-new","name":"My Collection","namespace":"sandbox"}`))
		},
	})

	col, err := client.Collections().Create(context.Background(), &Collection{
		Name:     "My Collection",
		Protocol: "http",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if col.ID != "col-new" {
		t.Errorf("expected ID col-new, got %q", col.ID)
	}
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace sandbox, got %q", gotBody.Namespace)
	}
}

func TestCollectionAPI_Update(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /ui/api/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"col-1","name":"Updated Collection"}`))
		},
	})

	col, err := client.Collections().Update(context.Background(), "col-1", &Collection{
		Name: "Updated Collection",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if col.Name != "Updated Collection" {
		t.Errorf("expected name Updated Collection, got %q", col.Name)
	}
}

func TestCollectionAPI_Delete(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /ui/api/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Collections().Delete(context.Background(), "col-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", gotMethod)
	}
}

func TestCollectionAPI_Duplicate(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"col-dup","name":"Copy of Collection"}`))
		},
	})

	col, err := client.Collections().Duplicate(context.Background(), "col-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if col.ID != "col-dup" {
		t.Errorf("expected ID col-dup, got %q", col.ID)
	}
	if !strings.Contains(gotPath, "col-1/duplicate") {
		t.Errorf("expected path to contain col-1/duplicate, got %s", gotPath)
	}
}

func TestCollectionAPI_BatchDelete(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /ui/api/api-tester/collections/batch-delete": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Collections().BatchDelete(context.Background(), []string{"col-1", "col-2", "col-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids, ok := gotBody["ids"].([]any)
	if !ok {
		t.Fatal("expected ids array in body")
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 IDs, got %d", len(ids))
	}
}

// ---------------------------------------------------------------------------
// Sub-API Singleton Tests for new APIs
// ---------------------------------------------------------------------------

func TestClient_NewSubAPISingletons(t *testing.T) {
	c := NewClient("http://localhost")

	if c.Generator() != c.Generator() {
		t.Error("Generator() should return the same instance")
	}
	if c.Fuzzing() != c.Fuzzing() {
		t.Error("Fuzzing() should return the same instance")
	}
	if c.Contracts() != c.Contracts() {
		t.Error("Contracts() should return the same instance")
	}
	if c.Recorder() != c.Recorder() {
		t.Error("Recorder() should return the same instance")
	}
	if c.Templates() != c.Templates() {
		t.Error("Templates() should return the same instance")
	}
	if c.Import() != c.Import() {
		t.Error("Import() should return the same instance")
	}
	if c.TestRuns() != c.TestRuns() {
		t.Error("TestRuns() should return the same instance")
	}
	if c.Admin() != c.Admin() {
		t.Error("Admin() should return the same instance")
	}
}
