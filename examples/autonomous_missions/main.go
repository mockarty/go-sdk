// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/mockarty/go-sdk"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	api := client.AutonomousMissions()
	ctx := context.Background()
	if os.Getenv("MOCKARTY_EXAMPLE_ANSWER") != "" {
		missionID := os.Getenv("MOCKARTY_ANSWER_MISSION_ID")
		if missionID == "" {
			log.Fatal("MOCKARTY_ANSWER_MISSION_ID must identify the mission that asked the captured question")
		}
		retryKey := os.Getenv("MOCKARTY_ANSWER_IDEMPOTENCY_KEY")
		if retryKey == "" {
			log.Fatal("MOCKARTY_ANSWER_IDEMPOTENCY_KEY must be unique per answered question and reused on retry")
		}
		answerRequest := mockarty.MissionAnswerRequest{Answer: os.Getenv("MOCKARTY_EXAMPLE_ANSWER"), IdempotencyKey: retryKey}
		// Capture data.question.id/revision/generation from the question the user
		// actually answered. Preserve these values and the retry key on retry.
		if questionID := os.Getenv("MOCKARTY_QUESTION_ID"); questionID != "" {
			revision, parseErr := strconv.ParseInt(os.Getenv("MOCKARTY_QUESTION_REVISION"), 10, 64)
			if parseErr != nil {
				log.Fatal("MOCKARTY_QUESTION_REVISION must be the captured integer revision")
			}
			answerRequest.QuestionID, answerRequest.Revision = questionID, revision
			answerRequest.Text, answerRequest.Answer = answerRequest.Answer, ""
			if generation := os.Getenv("MOCKARTY_QUESTION_GENERATION"); generation != "" {
				answerRequest.Generation, parseErr = strconv.ParseInt(generation, 10, 64)
				if parseErr != nil {
					log.Fatal("MOCKARTY_QUESTION_GENERATION must be an integer")
				}
			}
			// Choice/entity questions use Selected and Entity from the offered schema.
		}
		answered, answerErr := api.Answer(ctx, missionID, answerRequest)
		if answerErr != nil {
			log.Fatal(answerErr)
		}
		fmt.Printf("answer receipt=%s outcome=%s\n", answered.Control.ID, answered.Control.Outcome)
		return
	}
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
	// Opt in deliberately: full preapproves testing of this declared product;
	// chaos and paid effects still need approval.
	if autonomy := os.Getenv("MOCKARTY_EXAMPLE_AUTONOMY"); autonomy != "" {
		request.Autonomy = autonomy
	}
	if targetURL := os.Getenv("MOCKARTY_PRODUCT_URL"); targetURL != "" {
		request.Data = map[string]any{"contextRefs": []map[string]string{{"kind": "product_url", "value": targetURL}}}
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
