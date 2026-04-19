// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/sse_stream — subscribe to a live run and render each
// event as it arrives.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	runID := os.Getenv("RUN_ID")
	if runID == "" {
		log.Fatal("RUN_ID is required")
	}

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
	)
	events, err := client.TestPlans().StreamRun(ctx, runID)
	if err != nil {
		log.Fatalf("stream: %v", err)
	}

	for ev := range events {
		fmt.Printf("[%s] %-18s %s\n",
			ev.ReceivedAt.Format(time.RFC3339),
			ev.Kind,
			string(ev.Raw),
		)
	}
	fmt.Println("stream closed.")
}
