package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkflowDefinitionsLifecycleUsesExactNamespaceVersionAndCAS(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("X-API-Key"); got != "key" {
			t.Fatalf("X-API-Key=%q", got)
		}
		switch calls {
		case 1:
			if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/v1/namespaces/team-a/workflow-definitions" {
				t.Fatalf("create %s %s", r.Method, r.URL.EscapedPath())
			}
			var definition WorkflowDefinition
			if err := json.NewDecoder(r.Body).Decode(&definition); err != nil || definition.ID != "release-flow" {
				t.Fatalf("create body=%+v err=%v", definition, err)
			}
			if definition.Namespace != "team-a" {
				t.Fatalf("create body namespace=%q, want client default team-a", definition.Namespace)
			}
			if len(definition.Nodes) != 1 || definition.Nodes[0].Capability.Key != "mission.inspect" {
				t.Fatalf("create capability wire was not decoded: %+v", definition.Nodes)
			}
			_ = json.NewEncoder(w).Encode(StoredWorkflowDefinition{Definition: definition, Revision: 1})
		case 2:
			if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/v1/namespaces/team-a/workflow-definitions/release-flow/versions/1.0.0/dry-run" {
				t.Fatalf("dry-run %s %s", r.Method, r.URL.EscapedPath())
			}
			var body map[string]int64
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["expectedRevision"] != 1 {
				t.Fatalf("dry-run body=%v", body)
			}
			_ = json.NewEncoder(w).Encode(WorkflowDryRunResult{Ready: true, DefinitionDigest: "sha256:dry"})
		case 3:
			if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/v1/namespaces/team-a/workflow-definitions/release-flow/versions/1.0.0/publish" {
				t.Fatalf("publish %s %s", r.Method, r.URL.EscapedPath())
			}
			_ = json.NewEncoder(w).Encode(StoredWorkflowDefinition{Revision: 2})
		case 4:
			if r.Method != http.MethodGet || r.URL.Query().Get("status") != "published" || r.URL.Query().Get("limit") != "25" {
				t.Fatalf("list %s %s", r.Method, r.URL.String())
			}
			_ = json.NewEncoder(w).Encode(WorkflowDefinitionList{Definitions: []WorkflowDefinitionSummary{{ID: "release-flow"}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("key"), WithNamespace("team-a"))
	definition := WorkflowDefinition{ContractVersion: "mockarty.workflow/v1", ID: "release-flow", Version: "1.0.0", Status: "draft",
		Nodes: []WorkflowNode{{ID: "start", Capability: WorkflowCapabilityID{Key: "mission.inspect", Version: "1.0.0"}}}}
	created, err := client.WorkflowDefinitions().CreateDraft(context.Background(), definition)
	if err != nil || created.Revision != 1 {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	if definition.Namespace != "" {
		t.Fatalf("CreateDraft mutated caller definition namespace=%q", definition.Namespace)
	}
	if dryRun, err := client.WorkflowDefinitions().DryRun(context.Background(), "team-a", "release-flow", "1.0.0", created.Revision); err != nil || !dryRun.Ready {
		t.Fatalf("dry-run=%+v err=%v", dryRun, err)
	}
	if published, err := client.WorkflowDefinitions().Publish(context.Background(), "team-a", "release-flow", "1.0.0", created.Revision); err != nil || published.Revision != 2 {
		t.Fatalf("publish=%+v err=%v", published, err)
	}
	if listed, err := client.WorkflowDefinitions().List(context.Background(), "", WorkflowDefinitionListOptions{Status: "published", Limit: 25}); err != nil || len(listed.Definitions) != 1 {
		t.Fatalf("list=%+v err=%v", listed, err)
	}
}

func TestWorkflowDefinitionsRejectsGlobalOrIncompleteIdentityBeforeNetwork(t *testing.T) {
	api := NewClient("http://example.invalid", WithNamespace("*")).WorkflowDefinitions()
	if _, err := api.List(context.Background(), "", WorkflowDefinitionListOptions{}); err == nil {
		t.Fatal("global namespace accepted")
	}
	if _, err := api.Get(context.Background(), "team-a", "", "1.0.0"); err == nil {
		t.Fatal("empty workflow id accepted")
	}
	definition := WorkflowDefinition{Namespace: "team-a", ID: "flow", Version: "1.0.0"}
	if _, err := api.UpdateDraft(context.Background(), definition, 0); err == nil {
		t.Fatal("zero update revision accepted")
	}
	if _, err := api.DryRun(context.Background(), "team-a", "flow", "1.0.0", -1); err == nil {
		t.Fatal("negative dry-run revision accepted")
	}
	if _, err := api.Publish(context.Background(), "team-a", "flow", "1.0.0", 0); err == nil {
		t.Fatal("zero publish revision accepted")
	}
}
