// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: trash/list_filter — list soft-deleted entities in a namespace with
// entity-type / search / time-window filters, then render the per-type summary.
//
// Environment:
//
//	MOCKARTY_SERVER     — e.g. http://localhost:5770
//	MOCKARTY_TOKEN      — API key
//	MOCKARTY_NAMESPACE  — namespace to inspect
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")),
	)
	ns := os.Getenv("MOCKARTY_NAMESPACE")

	// 1) Aggregate counts per entity type (for badge rendering).
	summary, err := client.Trash().TrashSummary(ctx, ns)
	if err != nil {
		log.Fatalf("summary: %v", err)
	}
	fmt.Printf("Recycle Bin — %s  (total %d)\n", ns, summary.Total)
	for _, c := range summary.Counts {
		fmt.Printf("  %-18s %d\n", c.EntityType, c.Count)
	}

	// 2) List the most recently trashed mocks & stores.
	list, err := client.Trash().ListTrash(ctx, ns, mockarty.TrashListOptions{
		EntityTypes: []string{"mock", "store"},
		FromTime:    time.Now().Add(-7 * 24 * time.Hour),
		Limit:       50,
	})
	if err != nil {
		log.Fatalf("list: %v", err)
	}

	fmt.Printf("\nLast 7 days (%d of %d shown):\n", len(list.Items), list.Total)
	for _, it := range list.Items {
		status := "restorable"
		if !it.RestoreAvailable {
			status = "BLOCKED"
		}
		fmt.Printf("  [%s] %s  %s (%s) — closed by %s at %s  cascade=%s\n",
			status, it.EntityType, it.Name, it.ID, it.ClosedBy,
			it.ClosedAt.Format(time.RFC3339), it.CascadeGroupID)
	}
}
