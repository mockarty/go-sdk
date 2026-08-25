package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsListCapabilitiesDecodesCanonicalDescriptor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/capabilities" || r.URL.Query().Get("namespace") != "team-a" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"capabilities": []any{map[string]any{
				"contractVersion": "mockarty.capability/v1", "key": "mission.coder", "version": "1.0.0",
				"provider": "mockarty.missions", "kind": "mission-component", "title": "Coder", "description": "Codes.",
				"hosts": []string{"admin"}, "policy": map[string]any{"sideEffect": "external_write"},
				"provenance":   map[string]any{"sourceKind": "builtin", "sourceRef": "mockarty:coder", "publisher": "mockarty"},
				"availability": map[string]any{"available": true},
			}}, "count": 1, "skipped": 0,
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, WithNamespace("team-a"))
	catalog, err := client.Stats().ListCapabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Count != 1 || len(catalog.Capabilities) != 1 || catalog.Capabilities[0].ContractVersion != "mockarty.capability/v1" || catalog.Capabilities[0].Policy.SideEffect != "external_write" {
		t.Fatalf("catalog = %+v", catalog)
	}
}
