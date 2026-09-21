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
	if teamSpaceID := os.Getenv("MOCKARTY_TEAM_AUDIT_SPACE_ID"); teamSpaceID != "" {
		audit, err := client.CloudSpaces().ListTeamAudit(context.Background(), teamSpaceID,
			os.Getenv("MOCKARTY_TEAM_AUDIT_BEFORE_ID"), 50)
		if err != nil {
			panic(err)
		}
		fmt.Printf("team=%s events=%d chain_ok=%t has_more=%t next_before_id=%s\n",
			audit.TeamID, len(audit.Records), audit.ChainOK, audit.HasMore, audit.NextBeforeID)
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
	if organization, err := client.CloudSpaces().Organization(context.Background(), space.ID); err == nil {
		fmt.Printf("account=%s kind=%s occupied=%d/%d pending=%d\n", organization.AccountID,
			organization.AccountKind, organization.SeatsUsed, organization.SeatsTotal, len(organization.PendingInvites))
		if storage := organization.SyncStorage; storage != nil {
			if storage.Reconciled {
				fmt.Printf("team_sync_storage_bytes=%d/%d per_space_limit_bytes=%d\n",
					storage.UsedBytes, storage.LimitBytes, storage.SpaceLimitBytes)
			} else {
				fmt.Println("team_sync_storage=pending_reconciliation")
			}
		}
		if organization.AccountKind == "team" {
			if memberID := os.Getenv("MOCKARTY_TEAM_REMOVE_MEMBER_ID"); memberID != "" {
				if err := client.CloudSpaces().RemoveTeamMember(context.Background(), space.ID, memberID,
					os.Getenv("MOCKARTY_TEAM_MEMBER_REMOVE_KEY")); err != nil {
					panic(err)
				}
				fmt.Println("Team member removed")
			}
			if successorID := os.Getenv("MOCKARTY_TEAM_SUCCESSOR_ID"); successorID != "" {
				if err := client.CloudSpaces().TransferTeamOwner(context.Background(), space.ID, successorID,
					os.Getenv("MOCKARTY_TEAM_OWNER_TRANSFER_KEY")); err != nil {
					panic(err)
				}
				fmt.Println("Team ownership transferred")
			}
			if name := os.Getenv("MOCKARTY_TEAM_SPACE_NAME"); name != "" {
				created, err := client.CloudSpaces().CreateTeamSpace(context.Background(), mockarty.CloudTeamSpaceRequest{
					TeamID: organization.AccountID, Name: name, Audience: "team",
				}, os.Getenv("MOCKARTY_TEAM_SPACE_KEY"))
				if err != nil {
					panic(err)
				}
				fmt.Printf("created_team_space=%s\n", created.SpaceID)
			}
			if email := os.Getenv("MOCKARTY_TEAM_MEMBER_EMAIL"); email != "" {
				if err := client.CloudSpaces().InviteTeamMember(context.Background(), mockarty.CloudTeamMemberInviteRequest{
					SpaceID: space.ID, Email: email, Role: "member",
				}, os.Getenv("MOCKARTY_TEAM_INVITE_KEY")); err != nil {
					panic(err)
				}
				fmt.Println("Team invitation queued")
			}
		}
	}
	if inviteID := os.Getenv("MOCKARTY_TEAM_INVITE_ID"); inviteID != "" {
		key := os.Getenv("MOCKARTY_IDEMPOTENCY_KEY")
		if key == "" {
			panic("set MOCKARTY_IDEMPOTENCY_KEY and preserve it for an exact retry")
		}
		if err := client.CloudSpaces().RevokeTeamInvite(context.Background(), space.ID, inviteID, key); err != nil {
			panic(err)
		}
		fmt.Println("Team invitation revoked")
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
