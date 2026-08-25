// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	page, err := client.CloudSpaces().List(context.Background(), "", 25)
	if err != nil {
		panic(err)
	}
	for _, space := range page.Items {
		fmt.Printf("%s %s role=%s members=%d pending=%d revision=%d\n",
			space.ID, space.Name, space.Role, space.Usage.AcceptedHumans, space.Usage.PendingInvites, space.Revision)
	}
	if token := os.Getenv("MOCKARTY_INVITE_TOKEN"); token != "" {
		preview, err := client.CloudSpaces().PreviewInvite(context.Background(), token)
		if err != nil {
			panic(err)
		}
		key := os.Getenv("MOCKARTY_IDEMPOTENCY_KEY")
		if key == "" {
			panic("set MOCKARTY_IDEMPOTENCY_KEY and preserve it for an exact retry")
		}
		accepted, err := client.CloudSpaces().AcceptInvite(context.Background(), token, preview.ETag, key)
		if err != nil {
			panic(err)
		}
		fmt.Printf("accepted_space=%s role=%s revision=%d\n", accepted.SpaceID, accepted.Role, accepted.Revision)
	}
	if len(page.Items) == 0 {
		return
	}
	space, err := client.CloudSpaces().Get(context.Background(), page.Items[0].ID)
	if err != nil {
		panic(err)
	}
	members, err := client.CloudSpaces().ListMembers(context.Background(), space.ID, "", 25)
	if err != nil {
		panic(err)
	}
	invites, err := client.CloudSpaces().ListInvites(context.Background(), space.ID, "", 25)
	if err != nil {
		panic(err)
	}
	fmt.Printf("selected=%s members=%d invites=%d\n", space.ID, len(members.Items), len(invites.Items))

	// Opt in to a real mutation by setting MOCKARTY_INVITE_EMAIL. Preserve the
	// same key when retrying an ambiguous timeout.
	if email := os.Getenv("MOCKARTY_INVITE_EMAIL"); email != "" {
		etag := fmt.Sprintf(`"space-%s-r%d"`, space.ID, space.Revision)
		key := os.Getenv("MOCKARTY_IDEMPOTENCY_KEY")
		if key == "" {
			panic("set MOCKARTY_IDEMPOTENCY_KEY and preserve it for an exact retry")
		}
		created, err := client.CloudSpaces().CreateInvite(context.Background(), space.ID,
			mockarty.CloudSpaceInviteRequest{Email: email, Role: "viewer"}, etag, key)
		if err != nil {
			panic(err)
		}
		fmt.Printf("invite=%s token=%s next_revision=%d\n", created.Invite.ID, created.Invite.Token, created.Revision)
	}
}
