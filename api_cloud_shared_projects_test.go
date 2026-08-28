package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudSharedProjectsCRUDUsesPublicCloudProxy(t *testing.T) {
	var calls []string
	var requestIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.RequestURI())
		if r.Header.Get("X-API-Key") != "" || r.Header.Get("Authorization") != "Bearer mk_test" {
			t.Fatalf("credential headers=%v", r.Header)
		}
		if r.Method != http.MethodGet {
			requestIDs = append(requestIDs, r.Header.Get("X-Request-ID"))
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/shared/projects") && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"projects":[],"next_cursor":"","has_more":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"project-a","name":"A","body":{},"revision":1,"created_at":"2026-08-28T00:00:00Z","updated_at":"2026-08-28T00:00:00Z"}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, WithAPIKey("mk_test"), WithHTTPClient(server.Client()))
	api := client.CloudSharedProjects()
	if _, err := api.List(context.Background(), "space-a", "", 50); err != nil {
		t.Fatal(err)
	}
	const createRequestID = "11111111-1111-4111-8111-111111111111"
	project, err := api.Create(context.Background(), "space-a", "A", json.RawMessage(`{}`), createRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.Update(context.Background(), "space-a", *project); err != nil {
		t.Fatal(err)
	}
	if err := api.Delete(context.Background(), "space-a", "project-a", 1); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 || strings.Contains(strings.Join(calls, "\n"), "/runtime/") {
		t.Fatalf("calls=%v", calls)
	}
	if len(requestIDs) != 3 || requestIDs[0] != createRequestID || requestIDs[1] == "" || requestIDs[2] == "" {
		t.Fatalf("mutation request ids=%v", requestIDs)
	}
}

func TestCloudSharedProjectsRejectsNonCanonicalRequestID(t *testing.T) {
	api := (&Client{}).CloudSharedProjects()
	if api != nil {
		t.Fatal("zero-value client unexpectedly exposes an initialized API")
	}
	client := NewClient("http://127.0.0.1:1")
	if _, err := client.CloudSharedProjects().Create(context.Background(), "space-a", "A", json.RawMessage(`{}`), "retry"); err == nil {
		t.Fatal("noncanonical request id accepted")
	}
	if _, err := client.CloudSharedProjects().Create(context.Background(), "space-a", "A", json.RawMessage(`{}`), "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"); err == nil {
		t.Fatal("multiple request ids accepted")
	}
}
