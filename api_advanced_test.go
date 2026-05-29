// Copyright (c) 2026 Mockarty. All rights reserved.
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
		"POST /api/v1/generators/openapi": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/openapi": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/soap": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/grpc": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/graphql": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/har": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/socket": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/openapi/preview": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/soap/preview": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/grpc/preview": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/graphql/preview": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/har/preview": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/generators/openapi": func(w http.ResponseWriter, r *http.Request) {
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
	// Request body is wrapped in a {config:{...}} envelope; the response
	// carries {taskId, resultId, status} — FuzzingRun.ID maps from
	// `resultId` (NOT `id`).
	var gotBody struct {
		Config FuzzingConfig `json:"config"`
	}
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/fuzzing/run": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"taskId":"task-1","resultId":"fuzz-run-1","status":"running"}`))
		},
	})

	run, err := client.Fuzzing().Start(context.Background(), &FuzzingConfig{
		Name:          "security-fuzz",
		TargetBaseURL: "https://api.example.com",
		Strategy:      "all",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.ID != "fuzz-run-1" {
		t.Errorf("expected ID fuzz-run-1 (from resultId), got %q", run.ID)
	}
	if run.Status != "running" {
		t.Errorf("expected status running, got %q", run.Status)
	}
	if gotBody.Config.Name != "security-fuzz" {
		t.Errorf("expected config.name security-fuzz, got %q", gotBody.Config.Name)
	}
}

func TestFuzzingAPI_Start_DefaultNamespace(t *testing.T) {
	var gotBody struct {
		Config FuzzingConfig `json:"config"`
	}
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/fuzzing/run": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"taskId":"task-1","resultId":"fuzz-1","status":"running"}`))
		},
	})

	_, _ = client.Fuzzing().Start(context.Background(), &FuzzingConfig{
		TargetBaseURL: "https://api.example.com",
	})
	if gotBody.Config.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Config.Namespace)
	}
}

func TestFuzzingAPI_Stop(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
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
		"GET /api/v1/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"fuzz-1","status":"completed","totalRequests":1000,"totalFindings":3}`))
		},
	})

	result, err := client.Fuzzing().GetResult(context.Background(), "fuzz-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalRequests != 1000 {
		t.Errorf("expected 1000 total requests, got %d", result.TotalRequests)
	}
	if result.TotalFindings != 3 {
		t.Errorf("expected 3 findings, got %d", result.TotalFindings)
	}
}

func TestFuzzingAPI_ListResults(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/fuzzing/results": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Server emits {results,total,limit,offset} envelope.
			_, _ = w.Write([]byte(`{"results":[{"id":"r1","status":"completed"},` +
				`{"id":"r2","status":"running"}],"total":2}`))
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
		"DELETE /api/v1/fuzzing/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/fuzzing/configs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"my-config","targetBaseUrl":"https://api.example.com"}`))
		},
	})

	config, err := client.Fuzzing().CreateConfig(context.Background(), &FuzzingConfig{
		Name:          "my-config",
		TargetBaseURL: "https://api.example.com",
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
		"GET /api/v1/fuzzing/configs/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"cfg-1","name":"my-config","strategy":"all"}`))
		},
	})

	config, err := client.Fuzzing().GetConfig(context.Background(), "cfg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Strategy != "all" {
		t.Errorf("expected strategy 'all', got %q", config.Strategy)
	}
}

// ---------------------------------------------------------------------------
// Contract API Tests
// ---------------------------------------------------------------------------

func TestContractAPI_ValidateMocks(t *testing.T) {
	var gotBody ContractValidationRequest
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/contract/validate-mocks": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			// `violations` is a LIST of issue objects + an explicit
			// `violationCount`. (Legacy stale fixture used `violations:2`
			// — an int — which no longer matches the struct.)
			_, _ = w.Write([]byte(`{"id":"result-1","status":"fail","violationCount":2,` +
				`"violations":[` +
				`{"path":"/users","message":"missing field","severity":"error"},` +
				`{"path":"/orders","message":"type mismatch","severity":"error"}]}`))
		},
	})

	result, err := client.Contracts().ValidateMocks(context.Background(), &ContractValidationRequest{
		SpecURL: "https://example.com/openapi.yaml",
		BaseURL: "https://api.example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "fail" {
		t.Errorf("expected status fail, got %q", result.Status)
	}
	if len(result.Violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(result.Violations))
	}
	if result.ViolationCount != 2 {
		t.Errorf("expected violationCount 2, got %d", result.ViolationCount)
	}
	if gotBody.Namespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotBody.Namespace)
	}
}

func TestContractAPI_ListConfigs(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/contract/configs": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/contract/configs": func(w http.ResponseWriter, r *http.Request) {
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
		"DELETE /api/v1/contract/configs/": func(w http.ResponseWriter, r *http.Request) {
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
		"GET /api/v1/contract/results": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// `violations` is a list of issue objects + `violationCount`
			// scalar (the old fixture used `violations:1` — an int — which
			// no longer matches the struct).
			_, _ = w.Write([]byte(`[{"id":"r1","status":"pass"},` +
				`{"id":"r2","status":"fail","violationCount":1,` +
				`"violations":[{"path":"/x","message":"bad","severity":"error"}]}]`))
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
		"POST /api/v1/recorder/start": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			// Server wraps the session in a {session:{...}} envelope.
			_, _ = w.Write([]byte(`{"session":{"id":"session-1","name":"My Recording","status":"recording"}}`))
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
		"GET /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"session":{"id":"session-1","name":"My Recording","status":"recording","entryCount":42}}`))
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
		"GET /api/v1/recorder/sessions": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sessions":[{"id":"s1","status":"idle"},{"id":"s2","status":"recording"}]}`))
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
		"POST /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
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
		"DELETE /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
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
		"GET /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entries":[{"id":"e1","method":"GET","path":"/api/users","statusCode":200},` +
				`{"id":"e2","method":"POST","path":"/api/users","statusCode":201}],"total":2}`))
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
		"POST /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
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

func TestRecorderAPI_ExportSessionAsPostman_NoEntryFilter(t *testing.T) {
	var sawBody []byte
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
			sawBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"info":{"name":"x","schema":"https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},"item":[]}`))
		},
	})

	data, err := client.Recorder().ExportSessionAsPostman(context.Background(), "sess-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty Postman JSON")
	}
	// No entryIDs supplied → body must be empty (the admin handler
	// treats nil + missing entryIds identically as "export all").
	if len(sawBody) != 0 {
		t.Errorf("expected empty request body when no entryIDs given, got %q", sawBody)
	}
}

func TestRecorderAPI_ExportSessionAsPostman_WithEntryFilter(t *testing.T) {
	var sawBody []byte
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/recorder/": func(w http.ResponseWriter, r *http.Request) {
			sawBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"info":{},"item":[]}`))
		},
	})

	_, err := client.Recorder().ExportSessionAsPostman(
		context.Background(), "sess-9",
		"entry-1", "entry-2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Server must have received {"entryIds":["entry-1","entry-2"]}.
	var got struct {
		EntryIDs []string `json:"entryIds"`
	}
	if err := json.Unmarshal(sawBody, &got); err != nil {
		t.Fatalf("body not JSON: %v (raw=%q)", err, sawBody)
	}
	if len(got.EntryIDs) != 2 || got.EntryIDs[0] != "entry-1" || got.EntryIDs[1] != "entry-2" {
		t.Errorf("entryIds mismatch: %v", got.EntryIDs)
	}
}

// ---------------------------------------------------------------------------
// Template API Tests
// ---------------------------------------------------------------------------

func TestTemplateAPI_List(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/templates": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Server emits {templates:[<filename>,...]} — a bare string
			// list of filenames, not {name,size} objects.
			_, _ = w.Write([]byte(`{"templates":["response.json","error.xml"],"total":2}`))
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
		"GET /api/v1/templates/": func(w http.ResponseWriter, r *http.Request) {
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
	// Upload streams the template bytes RAW (not wrapped in a
	// {content:...} JSON envelope) so a subsequent Get round-trips the
	// exact bytes.
	var gotRawBody string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/templates/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			gotRawBody = string(body)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Templates().Upload(context.Background(), "new-template.json", []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotRawBody != `{"hello":"world"}` {
		t.Errorf("expected raw body bytes, got %q", gotRawBody)
	}
}

func TestTemplateAPI_Delete(t *testing.T) {
	var gotMethod string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/templates/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/postman": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Real server shape: {collections:[{id,...}], requests, summary}.
			// CollectionID is derived from collections[0].id and Imported
			// from summary.importedRequests (the flat collectionId/imported
			// fields in the old fixture never existed on the wire).
			_, _ = w.Write([]byte(`{"collections":[{"id":"col-1","name":"Postman Collection","protocol":"http"}],` +
				`"requests":[],"summary":{"importedRequests":15,"importedCollections":1}}`))
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
		"POST /api/v1/api-tester/import/openapi": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/wsdl": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/har": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/grpc": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/graphql": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/mcp": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/mockarty": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/import/postman": func(w http.ResponseWriter, r *http.Request) {
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
		"GET /api/v1/api-tester/test-runs": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			// Server emits a {runs,total,limit,offset} envelope — not a
			// bare array (the old fixture predates the unified test-runs
			// view + envelope unwrap).
			_, _ = w.Write([]byte(`{"runs":[{"id":"run-1","status":"completed","totalTests":10,` +
				`"passedTests":8,"failedTests":2}],"total":1,"limit":50,"offset":0}`))
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
		"GET /api/v1/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
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
		"DELETE /api/v1/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
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
		"GET /api/v1/api-tester/test-runs/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/reports/import": func(w http.ResponseWriter, r *http.Request) {
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

func TestTestRunAPI_ListActive(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/test-runs/active": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"runs":[{"id":"00000000-0000-0000-0000-000000000001","status":"running"},{"id":"00000000-0000-0000-0000-000000000002","status":"pending"}]}`))
		},
	})

	runs, err := client.TestRuns().ListActive(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 active runs, got %d", len(runs))
	}
	if runs[0].Status != "running" {
		t.Errorf("expected first run status 'running', got %q", runs[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Collection API Extended Tests (new CRUD methods)
// ---------------------------------------------------------------------------

func TestCollectionAPI_Create(t *testing.T) {
	var gotBody Collection
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/api-tester/collections": func(w http.ResponseWriter, r *http.Request) {
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
		"PUT /api/v1/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
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
		"DELETE /api/v1/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
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
		"POST /api/v1/api-tester/collections/": func(w http.ResponseWriter, r *http.Request) {
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
		"DELETE /api/v1/api-tester/collections/batch": func(w http.ResponseWriter, r *http.Request) {
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

	// Bind both reads to local variables before comparing — `c.Foo() != c.Foo()`
	// inline trips staticcheck SA4000 ("always false") because the
	// compiler can prove the two calls are pure. The local binding hides
	// that and exercises the accessor twice as intended.
	if a, b := c.Generator(), c.Generator(); a != b {
		t.Error("Generator() should return the same instance")
	}
	if a, b := c.Fuzzing(), c.Fuzzing(); a != b {
		t.Error("Fuzzing() should return the same instance")
	}
	if a, b := c.Contracts(), c.Contracts(); a != b {
		t.Error("Contracts() should return the same instance")
	}
	if a, b := c.Recorder(), c.Recorder(); a != b {
		t.Error("Recorder() should return the same instance")
	}
	if a, b := c.Templates(), c.Templates(); a != b {
		t.Error("Templates() should return the same instance")
	}
	if a, b := c.Import(), c.Import(); a != b {
		t.Error("Import() should return the same instance")
	}
	if a, b := c.TestRuns(), c.TestRuns(); a != b {
		t.Error("TestRuns() should return the same instance")
	}
	if a, b := c.Tags(), c.Tags(); a != b {
		t.Error("Tags() should return the same instance")
	}
	if a, b := c.Folders(), c.Folders(); a != b {
		t.Error("Folders() should return the same instance")
	}
	if a, b := c.Undefined(), c.Undefined(); a != b {
		t.Error("Undefined() should return the same instance")
	}
	if a, b := c.Stats(), c.Stats(); a != b {
		t.Error("Stats() should return the same instance")
	}
	if a, b := c.AgentTasks(), c.AgentTasks(); a != b {
		t.Error("AgentTasks() should return the same instance")
	}
	if a, b := c.NamespaceSettings(), c.NamespaceSettings(); a != b {
		t.Error("NamespaceSettings() should return the same instance")
	}
	if a, b := c.Proxy(), c.Proxy(); a != b {
		t.Error("Proxy() should return the same instance")
	}
	if a, b := c.Environments(), c.Environments(); a != b {
		t.Error("Environments() should return the same instance")
	}
}
