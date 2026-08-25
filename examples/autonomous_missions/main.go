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
		Goal:                   "Verify the checkout API and fuzz the payment endpoint",
		ProductID:              productID,
		Kind:                   "testing",
		Autonomy:               "auto",
		BudgetTokensTotal:      100000,
		ExpectedSettingsDigest: settings.SettingsDigest,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("mission=%s status=%s created=%t\n", started.Mission.ID, started.Mission.Status, started.Created)
}
