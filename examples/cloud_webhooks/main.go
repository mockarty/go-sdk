// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/go-sdk"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_CLOUD_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_CLOUD_TOKEN")))
	// Generate and save this UUID before the first attempt; use the same value
	// after a timeout or process restart. Treat it as private retry material.
	requestID := os.Getenv("MOCKARTY_WEBHOOK_REQUEST_ID")
	if requestID == "" {
		panic("MOCKARTY_WEBHOOK_REQUEST_ID is required")
	}
	credential, err := client.CloudWebhooks().CreateWithIdempotencyKey(
		context.Background(), os.Getenv("MOCKARTY_WORKSPACE_ID"), "Build events",
		"https://hooks.example.com/mockarty", []string{"instance.created", "instance.running"}, requestID,
	)
	if err != nil {
		panic(err)
	}
	// Persist this one-time value in your secret manager; it is not returned by list calls.
	fmt.Printf("webhook=%s one_time_secret=%s\n", credential.Webhook.ID, credential.Secret)
}
