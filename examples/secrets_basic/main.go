// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: secrets_basic — centralised Secrets Storage (Phase A0).
//
// Demonstrates:
//   - Creating a secret store
//   - Writing / reading / rotating / deleting entries
//   - Referencing a secret from a mock response via $.secrets.<store>.<key>
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// 1. Create the store
	store, err := client.Secrets().CreateStore(ctx, mockarty.SecretStore{
		Name:        "payments",
		Description: "API keys for payment providers",
		Backend:     "software",
	})
	if err != nil {
		log.Fatalf("create store: %v", err)
	}
	defer client.Secrets().DeleteStore(ctx, store.ID) //nolint:errcheck
	fmt.Printf("[1] Created store %s (%s)\n", store.Name, store.ID)

	// 2. Add an entry
	if _, err := client.Secrets().CreateEntry(ctx, store.ID, mockarty.SecretEntry{
		Key:         "stripe_api_key",
		Value:       "sk_test_abc123",
		Description: "Stripe test-mode key",
	}); err != nil {
		log.Fatalf("create entry: %v", err)
	}
	fmt.Println("[2] Entry stripe_api_key created")

	// 3. Read it back (requires secret:read permission)
	entry, err := client.Secrets().GetEntry(ctx, store.ID, "stripe_api_key")
	if err != nil {
		log.Fatalf("get entry: %v", err)
	}
	fmt.Printf("[3] Retrieved v%d — value length %d\n", entry.Version, len(entry.Value))

	// 4. Rotate — bumps version, returns new value
	rotated, err := client.Secrets().RotateEntry(ctx, store.ID, "stripe_api_key")
	if err != nil {
		log.Fatalf("rotate: %v", err)
	}
	fmt.Printf("[4] Rotated to v%d\n", rotated.Version)

	// 5. List entries (keys/metadata only — no values)
	entries, err := client.Secrets().ListEntries(ctx, store.ID)
	if err != nil {
		log.Fatalf("list entries: %v", err)
	}
	fmt.Printf("[5] %d entries in store\n", len(entries))

	// In a mock response body, reference the secret as:
	//   {"auth": "Bearer $.secrets.payments.stripe_api_key"}
	// Only mocks whose API key has `secret:read` will resolve the value;
	// others see a redacted placeholder.
}
