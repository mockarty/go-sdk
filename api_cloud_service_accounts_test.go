package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudServiceAccountsManagementAndWorkloadRoutes(t *testing.T) {
	var requests []string
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			keys = append(keys, key)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/self"):
			_, _ = w.Write([]byte(`{"service_account":{"id":"sa","name":"ci"},"credential_kind":"service_account"}`))
		case strings.Contains(r.URL.Path, "/shared/projects"):
			_, _ = w.Write([]byte(`{"id":"project-a","name":"A","body":{},"revision":1,"projects":[]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/service-accounts"):
			_, _ = w.Write([]byte(`{"service_accounts":[],"capacity":{"remaining":1}}`))
		default:
			_, _ = w.Write([]byte(`{"id":"sa","name":"ci","service_account":{"id":"sa"},"secret":"msa_once"}`))
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, WithAPIKey("token"))
	ctx := context.Background()
	_, _ = client.CloudServiceAccounts().List(ctx, "space-a")
	_, _ = client.CloudServiceAccounts().Get(ctx, "space-a", "account-a")
	_, _ = client.CloudServiceAccounts().Create(ctx, "space-a", CloudServiceAccountCreateRequest{Name: "ci", Capabilities: []string{"space.read"}}, "create-1")
	_, _ = client.CloudServiceAccounts().Bind(ctx, "space-a", "account-a", "space-b", []string{"space.read"}, "bind-1")
	_, _ = client.CloudServiceAccounts().IssueCredential(ctx, "space-a", "account-a", 60, "issue-1")
	_, _ = client.CloudServiceAccounts().RotateCredential(ctx, "space-a", "account-a", 60, "rotate-1")
	_, _ = client.CloudServiceAccounts().Revoke(ctx, "space-a", "account-a", "revoke-1")
	_, _ = client.CloudServiceAccount().Self(ctx)
	_, _ = client.CloudServiceAccount().Authorize(ctx, "space-a", "shared.project.write")
	_, _ = client.CloudServiceAccount().ListProjects(ctx, "space-a", "", 50)
	_, _ = client.CloudServiceAccount().GetProject(ctx, "space-a", "project-a")
	_, _ = client.CloudServiceAccount().CreateProject(ctx, "space-a", "A", json.RawMessage(`{}`), "project-create-1")
	_, _ = client.CloudServiceAccount().UpdateProject(ctx, "space-a", CloudSharedProject{ID: "project-a", Name: "B", Revision: 1, Body: json.RawMessage(`{}`)}, "project-update-1")
	_ = client.CloudServiceAccount().DeleteProject(ctx, "space-a", "project-a", 2, "project-delete-1")
	if len(requests) != 14 {
		t.Fatalf("requests=%v", requests)
	}
	if len(keys) != 8 {
		t.Fatalf("idempotency keys=%v", keys)
	}
	for _, request := range requests[9:] {
		if !strings.Contains(request, "/api/v1/cloud/service-account/spaces/") {
			t.Fatalf("machine route escaped dedicated group: %s", request)
		}
	}
}

func TestCloudServiceAccountMutationsRequireIdempotencyKey(t *testing.T) {
	client := NewClient("http://127.0.0.1")
	if _, err := client.CloudServiceAccounts().Create(context.Background(), "s", CloudServiceAccountCreateRequest{}, ""); err == nil {
		t.Fatal("management mutation accepted empty key")
	}
	if _, err := client.CloudServiceAccount().CreateProject(context.Background(), "s", "n", json.RawMessage(`{}`), ""); err == nil {
		t.Fatal("workload mutation accepted empty key")
	}
}
