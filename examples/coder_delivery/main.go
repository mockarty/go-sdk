package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	mission, err := client.CoderDelivery().StartMission(context.Background(), mockarty.CoderMissionStartRequest{
		Goal: "Deploy the accepted commit", RepoURL: os.Getenv("CODER_REPO_URL"), DeployTarget: "staging",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(mission.ID, mission.Status)
	if outcome := os.Getenv("CODER_DEPLOY_RECONCILIATION"); outcome != "" {
		mission, err = client.CoderDelivery().ReconcileDeploy(context.Background(), mission.ID, mockarty.CoderDeployReconciliationOutcome(outcome))
		if err != nil {
			panic(err)
		}
		fmt.Println("reconciled", mission.DeployStopState)
	}
}
