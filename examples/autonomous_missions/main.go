// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	api := client.AutonomousMissions()
	ctx := context.Background()
	productID := os.Getenv("MOCKARTY_PRODUCT_ID")

	settings, err := api.GetEffectiveSettings(ctx, mockarty.MissionEffectiveSettingsOptions{ProductID: productID})
	if err != nil {
		log.Fatal(err)
	}
	started, err := api.Start(ctx, mockarty.MissionStartRequest{
		Goal:                   "Take the checkout release to production quality and provide evidence",
		ProductID:              productID,
		Autonomy:               "auto",
		BudgetTokensTotal:      100000,
		ExpectedSettingsDigest: settings.SettingsDigest,
	})
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
