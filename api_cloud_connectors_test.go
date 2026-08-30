package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudConnectorsWriteOnlySecretLifecycle(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet && r.Header.Get("Idempotency-Key") == "" {
			t.Fatal("missing Idempotency-Key")
		}
		switch {
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"connectors": []any{map[string]any{"key": "oauth/github", "revision": 1}}})
		case strings.HasSuffix(r.URL.Path, "/test"):
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "passed", "code": "smtp_ok", "attempt_id": "attempt-1"})
		case strings.HasSuffix(r.URL.Path, "/revoke"):
			w.WriteHeader(http.StatusNoContent)
		default:
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["secrets"].(map[string]any)["client_secret"] != "write-only" {
				t.Fatal("write-only client_secret missing")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "oauth/github", "revision": 2, "secret_configured": true})
		}
	}))
	t.Cleanup(server.Close)
	api := NewClient(server.URL).CloudConnectors()
	items, err := api.List(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("list=(%+v,%v)", items, err)
	}
	updated, err := api.Update(context.Background(), "oauth", "github", "", CloudConnectorUpdate{
		Config: map[string]string{"client_id": "client"}, Secrets: map[string]string{"client_secret": "write-only"},
		ExpectedRevision: 1, Enabled: true,
	}, "connector-update-1")
	if err != nil || updated.Revision != 2 {
		t.Fatalf("update=(%+v,%v)", updated, err)
	}
	encoded, _ := json.Marshal(updated)
	if strings.Contains(string(encoded), "write-only") || strings.Contains(string(encoded), `"secrets"`) {
		t.Fatalf("response model leaked secret: %s", encoded)
	}
	if _, err = api.Test(context.Background(), "smtp", "smtp", "", "connector-test-1"); err != nil {
		t.Fatal(err)
	}
	if err = api.Revoke(context.Background(), "8bb0c85e-508b-4d83-b7c7-b8b87c910ecd", "connector-revoke-1"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 4 {
		t.Fatalf("requests=%v", requests)
	}
}

func TestCloudConnectorsRejectUnknownPathAndUnsafeMutation(t *testing.T) {
	api := NewClient("http://127.0.0.1:1").CloudConnectors()
	for _, tc := range []struct{ kind, provider, slot string }{
		{"custom", "github", ""}, {"oauth", "../github", ""}, {"payment", "stripe", ""},
	} {
		if _, err := api.Update(context.Background(), tc.kind, tc.provider, tc.slot, CloudConnectorUpdate{Config: map[string]string{}, ExpectedRevision: 1}, "key"); err == nil {
			t.Fatalf("unsafe connector path accepted: %+v", tc)
		}
	}
	if _, err := api.Update(context.Background(), "oauth", "github", "", CloudConnectorUpdate{Config: map[string]string{}, ExpectedRevision: 0}, "key"); err == nil {
		t.Fatal("zero revision accepted")
	}
}
