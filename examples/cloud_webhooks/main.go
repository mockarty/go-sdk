// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_CLOUD_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_CLOUD_TOKEN")))
	requestID := uuid.NewString() // Keep this value if retrying an ambiguous response.
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
