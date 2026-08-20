package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLLMSecurityNamespaceManagement(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.URL.EscapedPath() != "/api/v1/namespaces/team%2Fblue/llm-security/policy" {
			t.Fatalf("path = %q", request.URL.EscapedPath())
		}
		if request.Method == http.MethodPut {
			var body LLMSecurityPolicyRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.Mode != "merge" {
				t.Fatalf("body = %#v, err = %v", body, err)
			}
		}
		_ = json.NewEncoder(writer).Encode(LLMSecurityPolicyResponse{Namespace: "team/blue", Local: true})
	}))
	defer server.Close()

	api := NewClient(server.URL).LLMSecurity()
	if _, err := api.GetNamespacePolicy(context.Background(), "team/blue"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.SaveNamespacePolicy(context.Background(), "team/blue", LLMSecurityPolicyRequest{}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestLLMSecuritySandboxRejectsEmptyText(t *testing.T) {
	api := NewClient("http://example.invalid").LLMSecurity()
	if _, err := api.TestNamespaceText(context.Background(), "team", LLMSecuritySandboxRequest{}); err == nil {
		t.Fatal("empty text accepted")
	}
}

func TestLLMSecurityListEventsIsScopedAndBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.EscapedPath() != "/api/v1/namespaces/team%2Fblue/llm-security/events" ||
			request.URL.Query().Get("limit") != "25" {
			t.Fatalf("request = %s %s", request.Method, request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"events":[{"createdAt":"2026-08-20T00:00:00Z","mode":"enforce",` +
			`"source":"agent","ruleId":"pi.rule","category":"prompt_injection","decision":"block",` +
			`"surface":"input","trustClass":"user","correlationId":"req-sdk-123","id":1,` +
			`"latencyUs":2,"policyRevision":3,"matches":1,"score":900}]}`))
	}))
	defer server.Close()
	response, err := NewClient(server.URL).LLMSecurity().ListNamespaceEvents(context.Background(), "team/blue", 25)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Events) != 1 || response.Events[0].CorrelationID != "req-sdk-123" {
		t.Fatalf("events = %+v", response.Events)
	}
	if _, err = NewClient(server.URL).LLMSecurity().ListNamespaceEvents(context.Background(), "team", 501); err == nil {
		t.Fatal("oversized limit accepted")
	}
}
