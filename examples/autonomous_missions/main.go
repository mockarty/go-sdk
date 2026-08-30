// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	api := client.AutonomousMissions()
	ctx := context.Background()
	productID := os.Getenv("MOCKARTY_PRODUCT_ID")
	if archiveMissionID := os.Getenv("MOCKARTY_ARCHIVE_MISSION_ID"); archiveMissionID != "" {
		archive, archiveErr := api.ExportArchive(ctx, archiveMissionID)
		if archiveErr != nil {
			log.Fatal(archiveErr)
		}
		restored, restoreErr := api.RestoreArchive(ctx, archive)
		if restoreErr != nil {
			log.Fatal(restoreErr)
		}
		fmt.Printf("archive=%s mission=%s created=%t\n", archive.Digest, restored.ID, restored.Created)
	}

	settings, err := api.GetEffectiveSettings(ctx, mockarty.MissionEffectiveSettingsOptions{ProductID: productID})
	if err != nil {
		log.Fatal(err)
	}
	request := mockarty.MissionStartRequest{
		Goal:                   "Take the checkout release to production quality and provide evidence",
		ProductID:              productID,
		Autonomy:               "auto",
		BudgetTokensTotal:      100000,
		ExpectedSettingsDigest: settings.SettingsDigest,
	}
	if targetDigest := os.Getenv("MOCKARTY_TARGET_DIGEST"); targetDigest != "" {
		targetRevision, parseErr := strconv.ParseInt(os.Getenv("MOCKARTY_TARGET_REVISION"), 10, 64)
		if parseErr != nil {
			log.Fatal("MOCKARTY_TARGET_REVISION must be an integer when MOCKARTY_TARGET_DIGEST is set")
		}
		request.Targets = []mockarty.MissionRevisionReference{{
			Kind: "repo", ID: os.Getenv("MOCKARTY_TARGET_ID"), Revision: targetRevision, Digest: targetDigest,
		}}
	}
	started, err := api.Start(ctx, request)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("mission=%s status=%s created=%t\n", started.Mission.ID, started.Mission.Status, started.Created)
	if os.Getenv("MOCKARTY_EXAMPLE_ANSWER") != "" {
		answered, answerErr := api.Answer(ctx, started.Mission.ID, mockarty.MissionAnswerRequest{
			Answer: os.Getenv("MOCKARTY_EXAMPLE_ANSWER"), IdempotencyKey: "autonomous-missions-example-answer",
		})
		if answerErr != nil {
			log.Fatal(answerErr)
		}
		fmt.Printf("answer receipt=%s outcome=%s\n", answered.Control.ID, answered.Control.Outcome)
	}
	if os.Getenv("MOCKARTY_EXAMPLE_CANCEL") == "1" {
		cancelled, cancelErr := api.Cancel(ctx, started.Mission.ID, mockarty.MissionCancelRequest{
			Reason: "example run no longer needed", IdempotencyKey: "autonomous-missions-example-cancel",
		})
		if cancelErr != nil {
			log.Fatal(cancelErr)
		}
		fmt.Printf("cancel receipt=%s outcome=%s reason=%s\n",
			cancelled.Control.ID, cancelled.Control.Outcome, cancelled.Control.Reason)
		for _, binding := range cancelled.ExecutionBindings {
			fmt.Printf("child=%s kind=%s state=%s\n", binding.ExternalID, binding.Kind, binding.State)
		}
	}
}
