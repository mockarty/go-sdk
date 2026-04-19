// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: trash/bulk_purge — hard-delete cascade groups. IRREVERSIBLE.
// Requires the exact confirmation phrase
// (mockarty.TrashPurgeConfirmationPhrase); the SDK pre-validates it
// client-side via ErrTrashConfirmationMissing.
//
// Environment:
//
//	MOCKARTY_SERVER       — e.g. http://localhost:5770
//	MOCKARTY_TOKEN        — API key
//	MOCKARTY_NAMESPACE    — namespace to operate on
//	MOCKARTY_CASCADE_IDS  — comma-separated cascade group IDs to purge
//	MOCKARTY_PURGE_REASON — (optional) reason recorded in the audit log
package main

import (
	"context"
	"errors"
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

	req := mockarty.BulkPurgeRequest{
		CascadeGroupIDs: ids,
		Confirmation:    mockarty.TrashPurgeConfirmationPhrase, // "I understand this is permanent"
		Reason:          os.Getenv("MOCKARTY_PURGE_REASON"),
	}

	result, err := client.Trash().BulkPurge(ctx, os.Getenv("MOCKARTY_NAMESPACE"), req)
	if errors.Is(err, mockarty.ErrTrashConfirmationMissing) {
		log.Fatal("confirmation phrase mismatch — refusing to send purge request")
	}
	if err != nil {
		log.Fatalf("bulk purge: %v", err)
	}

	fmt.Printf("Purged: %d  Failed: %d  NotFound: %d\n",
		len(result.Purged), len(result.Failed), len(result.NotFound))

	for _, o := range result.Purged {
		fmt.Printf("  OK   %s (%s) — %d rows\n", o.CascadeGroupID, o.EntityType, o.RowsDeleted)
	}
	for _, o := range result.Failed {
		fmt.Printf("  FAIL %s (%s) — %s\n", o.CascadeGroupID, o.EntityType, o.Error)
	}
	for _, id := range result.NotFound {
		fmt.Printf("  MISS %s\n", id)
	}
}
