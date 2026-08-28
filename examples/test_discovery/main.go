// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: test_discovery — sync a test framework's collected case list
// into the Mockarty TCM catalogue. Run this as a CI "collect" step (after
// `pytest --collect-only`, `go test -list`, a JUnit dry-run, etc.) so the
// catalogue tracks the source tree. Cases are matched by FullName; with
// PruneMissing set, cases previously synced under the same Source but
// absent from this manifest are marked orphaned.
//
// Usage:
//
//	MOCKARTY_SERVER=http://127.0.0.1:8080 \
//	MOCKARTY_TOKEN=mk_... \
//	MOCKARTY_NAMESPACE=default \
//	go run .
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
		mockarty.WithRetry(3, 500*time.Millisecond),
	)

	manifest := mockarty.DiscoveryManifest{
		Source:       "pytest:auth-suite",
		Framework:    "pytest",
		PruneMissing: true, // orphan cases that vanished from the source tree
		Cases: []mockarty.DiscoveryManifestCase{
			{
				FullName:    "auth.login::test_valid_credentials",
				Name:        "Valid credentials log in",
				Suite:       "auth",
				Description: "POST /login with a known user returns 200 + token",
				SourceRef:   "tests/auth/test_login.py:12",
				Labels:      []string{"smoke", "auth"},
			},
			{
				FullName:  "auth.login::test_locked_account",
				Name:      "Locked account is rejected",
				Suite:     "auth",
				SourceRef: "tests/auth/test_login.py:31",
				Labels:    []string{"auth"},
			},
		},
	}

	res, err := client.Discovery().Sync(ctx, "", manifest)
	if err != nil {
		log.Fatalf("discovery sync: %v", err)
	}

	fmt.Printf("synced source=%q: created=%d updated=%d orphaned=%d total=%d\n",
		res.Source, res.Created, res.Updated, res.Orphaned, res.Total)
}
