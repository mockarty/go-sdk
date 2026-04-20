// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: entity_search — look up Mockarty resources by case-insensitive
// name match across all major entity types. Powers UI pickers and CI/CD
// automation that needs to resolve a human-readable name into the canonical
// UUID before issuing further API calls.
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

	// Example 1 — find every Test Plan whose name contains "smoke" in the
	// active namespace. The server already case-insensitive-matches, so the
	// SDK forwards the query verbatim.
	plans, err := client.EntitySearch().Search(ctx, mockarty.EntitySearchRequest{
		Type:  mockarty.EntityTypeTestPlan,
		Query: "smoke",
		Limit: 25,
	})
	if err != nil {
		log.Fatalf("search test plans: %v", err)
	}
	fmt.Printf("Test Plans matching 'smoke' (%d total):\n", plans.Total)
	for _, p := range plans.Items {
		numeric := ""
		if p.NumericID != nil {
			numeric = fmt.Sprintf(" (#%d)", *p.NumericID)
		}
		fmt.Printf("  %s%s  ns=%s  created=%s\n", p.Name, numeric, p.Namespace, p.CreatedAt)
	}

	// Example 2 — paginate through all mocks in the namespace. The server
	// caps `limit` at EntitySearchMaxLimit (200) — passing a larger value is
	// silently clamped, so always honour the documented ceiling.
	const pageSize = mockarty.EntitySearchDefaultLimit
	for offset := 0; ; offset += pageSize {
		page, err := client.EntitySearch().Search(ctx, mockarty.EntitySearchRequest{
			Type:   mockarty.EntityTypeMock,
			Limit:  pageSize,
			Offset: offset,
		})
		if err != nil {
			log.Fatalf("page mocks: %v", err)
		}
		for _, m := range page.Items {
			fmt.Printf("mock %s  id=%s\n", m.Name, m.ID)
		}
		if offset+len(page.Items) >= page.Total || len(page.Items) == 0 {
			break
		}
	}
}
