package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudInstancesLifecycleUsesExplicitSpaceAndIdempotency(t *testing.T) {
	var requests []struct{ method, path, query, key string }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct{ method, path, query, key string }{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Idempotency-Key")})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloud/instances":
			_ = json.NewEncoder(w).Encode(map[string]any{"instance": map[string]any{"id": "instance-1", "workspace_id": "space-1"}, "bootstrap": map[string]any{"available": true, "password": "one-time", "one_time": true}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloud/instances":
			_ = json.NewEncoder(w).Encode(map[string]any{"instances": []any{}, "total": 0})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"instance": map[string]any{"id": "instance-1"}})
		}
	}))
	t.Cleanup(server.Close)
	api := NewClient(server.URL, WithAPIKey("cloud-session")).CloudInstances()
	created, err := api.Create(context.Background(), "space-1", "Managed", "create-1")
	if err != nil || created.Instance == nil || created.Bootstrap == nil || created.Bootstrap.Password != "one-time" {
		t.Fatalf("create=(%+v,%v)", created, err)
	}
	if _, err := api.List(context.Background(), "space-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Get(context.Background(), "instance-1"); err != nil {
		t.Fatal(err)
	}
	if err := api.Stop(context.Background(), "instance-1", "stop-1"); err != nil {
		t.Fatal(err)
	}
	if err := api.Start(context.Background(), "instance-1", "start-1"); err != nil {
		t.Fatal(err)
	}
	if err := api.Delete(context.Background(), "instance-1", "delete-1"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 6 || requests[0].key != "create-1" || requests[1].query != "workspace_id=space-1" ||
		requests[3].path != "/api/v1/cloud/instances/instance-1/stop" || requests[5].method != http.MethodDelete {
		t.Fatalf("requests=%+v", requests)
	}
}
