package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigureVaultUsesStoredTokenReference(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/sandbox/integrations/vault" || r.Method != http.MethodPut {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClient(server.URL, WithAPIKey("test-key"))
	if err := client.Secrets().ConfigureVault(context.Background(), "sandbox", VaultIntegration{
		URL: "https://vault.example.com", TokenSecretID: "entry-id", Mount: "secret",
	}); err != nil {
		t.Fatal(err)
	}
	if received["tokenSecretId"] != "entry-id" || received["mount"] != "secret" {
		t.Fatalf("wrong Vault body: %#v", received)
	}
	if _, ok := received["token"]; ok {
		t.Fatal("raw token was sent")
	}
	if err := client.Secrets().ConfigureVault(context.Background(), "sandbox", VaultIntegration{URL: "https://vault.example.com"}); err == nil {
		t.Fatal("missing token reference must fail before request")
	}
}
