package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPageAnalyzerLifecycleUsesExactRoutes(t *testing.T) {
	expected := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/page-analyzer/configs?namespace=team-a"},
		{http.MethodPost, "/api/v1/page-analyzer/configs?namespace=team-a"},
		{http.MethodPut, "/api/v1/page-analyzer/configs/cfg-1?namespace=team-a"},
		{http.MethodDelete, "/api/v1/page-analyzer/configs/cfg-1?namespace=team-a"},
		{http.MethodPost, "/api/v1/page-analyzer/run?namespace=team-a"},
		{http.MethodGet, "/api/v1/page-analyzer/results?limit=25&namespace=team-a&offset=5"},
		{http.MethodGet, "/api/v1/page-analyzer/results/res-1?namespace=team-a"},
		{http.MethodDelete, "/api/v1/page-analyzer/results/res-1?namespace=team-a"},
		{http.MethodPost, "/api/v1/page-analyzer/results/res-1/ai-analyze?namespace=team-a"},
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls >= len(expected) {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		want := expected[calls]
		calls++
		if r.Method != want.method || r.URL.RequestURI() != want.path {
			t.Fatalf("request=(%s %s), want=(%s %s)", r.Method, r.URL.RequestURI(), want.method, want.path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_ = json.NewEncoder(w).Encode(PageAnalyzerConfigList{Configs: []PageAnalyzerConfig{}})
		case 5:
			_ = json.NewEncoder(w).Encode(PageAnalyzerRunResponse{ResultID: "res-1", Status: "pending", Mode: "http"})
		case 6:
			_ = json.NewEncoder(w).Encode(PageAnalyzerResultList{Results: []PageAnalyzerResult{}, Limit: 25, Offset: 5})
		case 9:
			_ = json.NewEncoder(w).Encode(PageAnalyzerAIResponse{Analysis: "ok"})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	api := NewClient(server.URL, WithAPIKey("key"), WithNamespace("team-a")).PageAnalyzer()
	ctx := context.Background()
	_, _ = api.ListConfigs(ctx)
	_, _ = api.SaveConfig(ctx, PageAnalyzerConfigRequest{Name: "home", TargetURL: "https://example.com"})
	updateURL := "https://example.com"
	_, _ = api.UpdateConfig(ctx, "cfg-1", PageAnalyzerConfigUpdateRequest{Name: "home", TargetURL: &updateURL})
	_ = api.DeleteConfig(ctx, "cfg-1")
	_, _ = api.Run(ctx, PageAnalyzerRunRequest{TargetURL: "https://example.com"})
	_, _ = api.ListResults(ctx, 25, 5)
	_, _ = api.GetResult(ctx, "res-1")
	_ = api.DeleteResult(ctx, "res-1")
	_, _ = api.AnalyzeWithAI(ctx, "res-1", PageAnalyzerAIRequest{})
	if calls != len(expected) {
		t.Fatalf("calls=%d, want=%d", calls, len(expected))
	}
}

func TestPageAnalyzerUpdateRequestPreservesOmittedTargetAndAuthClear(t *testing.T) {
	emptyAuth := map[string]string{}
	raw, err := json.Marshal(PageAnalyzerConfigUpdateRequest{Name: "rename only"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "targetUrl") || strings.Contains(string(raw), "authConfig") {
		t.Fatalf("omitted secret fields must stay omitted: %s", raw)
	}

	raw, err = json.Marshal(PageAnalyzerConfigUpdateRequest{Name: "clear auth", AuthConfig: &emptyAuth})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"authConfig":{}`) {
		t.Fatalf("explicit empty authConfig must stay present for clear semantics: %s", raw)
	}
}

func TestPageAnalyzerRunRequestOmitsOptionsButPreservesExplicitFalse(t *testing.T) {
	raw, err := json.Marshal(PageAnalyzerRunRequest{TargetURL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "options") {
		t.Fatalf("omitted options must stay omitted so server defaults apply: %s", raw)
	}

	raw, err = json.Marshal(PageAnalyzerRunRequest{
		TargetURL: "https://example.com",
		Options:   &PageAnalyzerOptions{CheckResources: false, FollowRedirects: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"checkResources":false`) || !strings.Contains(string(raw), `"followRedirects":false`) {
		t.Fatalf("explicit false options must stay present: %s", raw)
	}
}
