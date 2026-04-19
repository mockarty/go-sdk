// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: trash/bulk_restore — restore multiple cascade groups in a single
// call and surface per-group outcomes (restored / failed / not-found).
//
// Environment:
//
//	MOCKARTY_SERVER       — e.g. http://localhost:5770
//	MOCKARTY_TOKEN        — API key
//	MOCKARTY_NAMESPACE    — namespace to operate on
//	MOCKARTY_CASCADE_IDS  — comma-separated cascade group IDs to restore
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	raw := os.Getenv("MOCKARTY_CASCADE_IDS")
	if raw == "" {
		log.Fatal("MOCKARTY_CASCADE_IDS is required (comma-separated)")
	}
	ids := strings.Split(raw, ",")
	for i, id := range ids {
		ids[i] = strings.TrimSpace(id)
	}

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
	)

	result, err := client.Trash().BulkRestore(ctx, os.Getenv("MOCKARTY_NAMESPACE"),
		mockarty.BulkRestoreRequest{
			CascadeGroupIDs: ids,
			Reason:          "restore batch " + time.Now().Format(time.RFC3339),
		})
	if err != nil {
		log.Fatalf("bulk restore: %v", err)
	}

	fmt.Printf("Restored: %d  Failed: %d  NotFound: %d\n",
		len(result.Restored), len(result.Failed), len(result.NotFound))

	for _, o := range result.Restored {
		fmt.Printf("  OK   %s (%s) — %d rows\n", o.CascadeGroupID, o.EntityType, o.RestoredCount)
	}
	for _, o := range result.Failed {
		fmt.Printf("  FAIL %s (%s) — %s\n", o.CascadeGroupID, o.EntityType, o.Error)
	}
	for _, id := range result.NotFound {
		fmt.Printf("  MISS %s\n", id)
	}
}
