package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	if os.Getenv("MISSION_PRODUCT_ID") != "" {
		// Text originals in one mission must total at most 64 KiB.
		// Use a leaf filename (no path), at most 128 UTF-8 bytes. Supported
		// binary formats are PNG, JPEG, WebP, GIF and PDF; SVG is not accepted.
		// Upload and AddToMission are not automatically retried. An ambiguous
		// failure does not prove whether the mutation applied; the caller must
		// decide whether another request is safe.
		material, err := client.CoderDelivery().UploadMissionMaterial(context.Background(), os.Getenv("MOCKARTY_NAMESPACE"), os.Getenv("MISSION_PRODUCT_ID"), "design.txt", "text/plain", strings.NewReader("Palette: navy and cream. Keep accessible contrast."))
		if err != nil {
			panic(err)
		}
		fmt.Println("Use this reference in POST /api/v1/missions artifacts:", material.Reference)
	}
	mission, err := client.CoderDelivery().StartMission(context.Background(), mockarty.CoderMissionStartRequest{
		Goal: "Deploy the accepted commit", RepoURL: os.Getenv("CODER_REPO_URL"), DeployTarget: "staging",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(mission.ID, mission.Status)
	fmt.Println("independent AQC receipts", len(mission.AQCEvidence), "merge status", mission.MRMergeStatus, "repair attempts", mission.DeployRepairAttempts)
	if os.Getenv("CODER_ADD_GO_CHECK") == "1" {
		mission, err = client.CoderDelivery().AddToMission(context.Background(), mission.ID, mockarty.CoderMissionAddRequest{
			Tasks: []mockarty.CoderSubTask{{Prompt: "Run and fix the Go unit suite", RequiredChecks: []mockarty.CoderRequiredCheck{{Name: "Go unit tests", Args: []string{"go", "test", "./..."}}}}},
		})
		if err != nil {
			panic(err)
		}
		fmt.Println("extended", mission.ID)
	}
	if outcome := os.Getenv("CODER_DEPLOY_RECONCILIATION"); outcome != "" {
		mission, err = client.CoderDelivery().ReconcileDeploy(context.Background(), mission.ID, mockarty.CoderDeployReconciliationOutcome(outcome))
		if err != nil {
			panic(err)
		}
		fmt.Println("reconciled", mission.DeployStopState)
	}
	if os.Getenv("CODER_OBSERVE") == "1" {
		sources, err := client.CoderDelivery().ObservabilitySources(context.Background())
		if err != nil {
			panic(err)
		}
		fmt.Println("observability sources", len(sources.Sources))
	}
}
