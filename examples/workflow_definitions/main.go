// Workflow Definition lifecycle: draft -> dry-run -> immutable publish.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(
		getenv("MOCKARTY_URL", "http://127.0.0.1:5770"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_TOKEN")),
		mockarty.WithNamespace(getenv("MOCKARTY_NAMESPACE", "sandbox")),
	)
	definition := mockarty.WorkflowDefinition{
		ContractVersion: "mockarty.workflow/v1",
		Namespace:       getenv("MOCKARTY_NAMESPACE", "sandbox"),
		ID:              "release-check",
		Version:         "1.0.0",
		Status:          "draft",
		EntryNode:       "inspect",
		Nodes: []mockarty.WorkflowNode{{
			ID: "inspect", Capability: mockarty.WorkflowCapabilityID{Key: "mission.inspect", Version: "1.0.0"},
		}},
	}
	created, err := client.WorkflowDefinitions().CreateDraft(context.Background(), definition)
	if err != nil {
		log.Fatal(err)
	}
	dryRun, err := client.WorkflowDefinitions().DryRun(context.Background(), definition.Namespace, definition.ID, definition.Version, created.Revision)
	if err != nil {
		log.Fatal(err)
	}
	if !dryRun.Ready {
		log.Fatalf("workflow is blocked: %+v", dryRun.Blockers)
	}
	published, err := client.WorkflowDefinitions().Publish(context.Background(), definition.Namespace, definition.ID, definition.Version, created.Revision)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("published %s@%s revision=%d\n", published.Definition.ID, published.Definition.Version, published.Revision)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
