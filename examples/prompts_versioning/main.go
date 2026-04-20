// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: prompts_versioning — Prompts Storage with FIFO-20 history.
//
// Demonstrates:
//   - Creating a prompt
//   - Updating it multiple times (each update becomes a version)
//   - Listing history and rolling back to an earlier version
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

	p, err := client.Prompts().CreatePrompt(ctx, mockarty.Prompt{
		Name:  "tcm-step-summarizer",
		Body:  "Summarize the following test step in one sentence: {{.step}}",
		Model: "claude-opus-4-7",
		Tags:  []string{"tcm", "summary"},
	})
	if err != nil {
		log.Fatalf("create: %v", err)
	}
	defer client.Prompts().DeletePrompt(ctx, p.ID) //nolint:errcheck
	fmt.Printf("[1] Created prompt %s v%d\n", p.Name, p.Version)

	// Two edits → versions 2 and 3
	p, _ = client.Prompts().UpdatePrompt(ctx, p.ID, mockarty.Prompt{
		Name: p.Name,
		Body: "Summarize in ≤15 words: {{.step}}",
	})
	p, _ = client.Prompts().UpdatePrompt(ctx, p.ID, mockarty.Prompt{
		Name: p.Name,
		Body: "One sentence summary, verb-first: {{.step}}",
	})
	fmt.Printf("[2] Current version: %d\n", p.Version)

	versions, err := client.Prompts().ListVersions(ctx, p.ID)
	if err != nil {
		log.Fatalf("list versions: %v", err)
	}
	fmt.Printf("[3] History: %d versions\n", len(versions))

	rolled, err := client.Prompts().Rollback(ctx, p.ID, 1)
	if err != nil {
		log.Fatalf("rollback: %v", err)
	}
	fmt.Printf("[4] Rolled back; new version %d with original body\n", rolled.Version)
}
