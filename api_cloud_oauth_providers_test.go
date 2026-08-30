package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudOAuthProvidersWriteOnlySecretReference(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer operator-session" || r.Header.Get(headerAPIKey) != "" {
			t.Fatalf("operator auth headers Authorization=%q X-API-Key=%q", r.Header.Get("Authorization"), r.Header.Get(headerAPIKey))
		}
		if r.Method == http.MethodPut {
			var raw map[string]any
			_ = json.NewDecoder(r.Body).Decode(&raw)
			body = raw["client_secret"].(string)
			if r.Header.Get("Idempotency-Key") != "oauth-1" {
				t.Fatal("missing idempotency key")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"provider": "github", "client_id": "client", "config_revision": 1, "enabled": true, "secret_configured": true})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"providers": []any{map[string]any{"provider": "github"}}})
	}))
	t.Cleanup(server.Close)
	api := NewClient(server.URL, WithAPIKey("operator-session")).CloudOAuthProviders()
	t.Setenv("CLOUD_API_PROVIDER_SECRET_OAUTH_GITHUB", "write-only")
	ref := "env://CLOUD_API_PROVIDER_SECRET_OAUTH_GITHUB"
	provider, err := api.Update(context.Background(), "github", CloudOAuthProviderUpdate{ClientID: "client", ClientSecretRef: ref, ExpectedRevision: 1, Enabled: true}, "oauth-1")
	if err != nil || provider.ConfigRevision != 1 || body != "write-only" {
		t.Fatalf("update=(%+v,%q,%v)", provider, body, err)
	}
	encoded, _ := json.Marshal(provider)
	if strings.Contains(string(encoded), "write-only") || strings.Contains(string(encoded), "client_secret") {
		t.Fatalf("response model leaked secret reference: %s", encoded)
	}
	if providers, err := api.List(context.Background()); err != nil || len(providers) != 1 {
		t.Fatalf("list=(%+v,%v)", providers, err)
	}
}
