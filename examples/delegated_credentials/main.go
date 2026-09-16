// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Issue a project-scoped delegated credential, hand it to a CI job, then
// rotate and revoke it.
//
// Run against a live Mockarty admin node:
//
//	MOCKARTY_BASE_URL=http://127.0.0.1:5770 \
//	MOCKARTY_API_KEY=mk_... \
//	go run ./examples/delegated_credentials
//
// The point of the example is the difference from an API key: an API key's
// namespace is a DEFAULT its holder can change, and it carries no expiry of
// its own. A delegated credential freezes both, and the holder — the CI job in
// this story — has no path to widen either. That is why the token below is
// usable and the scope is not.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))

	ctx := context.Background()
	// The ISSUER is a human (or a Space owner's automation): a session or an
	// API key. The credential it mints acts in the issuer's namespaces but only
	// within the frozen set, and only until the frozen instant.
	api := client.DelegatedCredentials()

	expires := time.Now().UTC().Add(30 * 24 * time.Hour)
	issued, err := api.Create(ctx, mockarty.DelegatedCredentialCreateRequest{
		Name:           "ci-reader",
		Description:    "nightly regression job — read-only, one namespace, 30 days",
		Namespaces:     []string{"staging"},
		AllowedActions: []string{"read"}, // omit to grant read+write+delete
		ExpiresAt:      &expires,
	})
	if err != nil {
		panic(fmt.Sprintf("issue delegated credential: %v", err))
	}

	// Shown ONCE. Store it in the CI secret store; the row keeps only a bcrypt
	// hash and a sha256 index, so it cannot be read back. Lost it? Rotate.
	fmt.Printf("issued %s (%s)\n", issued.Credential.ID, issued.Credential.TokenPrefix)
	fmt.Printf("  frozen scope: namespaces=%v actions=%v expires=%s\n",
		issued.Credential.Namespaces, issued.Credential.AllowedActions,
		issued.Credential.ExpiresAt.Format(time.RFC3339))
	fmt.Printf("  hand this to the job: %s\n", issued.Token)

	// The issuer can see what it issued — and so can a delegated credential, for
	// the credentials IT minted, but never for its issuer's.
	listed, err := api.List(ctx)
	if err != nil {
		panic(fmt.Sprintf("list: %v", err))
	}
	fmt.Printf("\n%d credential(s) visible to this caller\n", listed.Count)
	for _, cred := range listed.Credentials {
		state := "active"
		if cred.IsRevoked() {
			state = "revoked"
		} else if cred.IsExpired(time.Now().UTC()) {
			state = "expired"
		}
		fmt.Printf("  %s %-18s %s %v -> %s\n",
			cred.ID, cred.Name, state, cred.Namespaces, cred.ExpiresAt.Format(time.RFC3339))
	}

	// The leaked-secret path. Rotation issues a fresh bearer and leaves the
	// scope EXACTLY as it was: a live credential's authority is never edited,
	// because a silent widening would leave no new credential to audit.
	rotated, err := api.Rotate(ctx, issued.Credential.ID)
	if err != nil {
		panic(fmt.Sprintf("rotate: %v", err))
	}
	if len(rotated.Credential.Namespaces) != len(issued.Credential.Namespaces) {
		panic("rotation must not change the scope")
	}
	fmt.Printf("\nrotated — the previous token stopped working immediately:\n  %s\n", rotated.Token)

	// Revocation is terminal: there is no un-revoke, because "temporarily
	// revoked" is exactly the window a shared credential should not have.
	// Revoking twice is a no-op rather than an error.
	if err := api.Revoke(ctx, issued.Credential.ID); err != nil {
		panic(fmt.Sprintf("revoke: %v", err))
	}
	if err := api.Revoke(ctx, issued.Credential.ID); err != nil {
		panic(fmt.Sprintf("second revoke must be a no-op, got: %v", err))
	}
	fmt.Printf("\nrevoked %s (idempotent)\n", issued.Credential.ID)

	// A delegated credential can issue another one, but only ever a NARROWER
	// one — a chain of delegations cannot widen. Doing that here needs the
	// delegated token itself as the client credential, which is the follow-up
	// step in a real deployment: the CI job mints a per-run credential scoped
	// to a single namespace and a single hour.
	fmt.Println("\nthe held token can act only in its frozen namespaces, only with its frozen actions, only until its expiry")
}
