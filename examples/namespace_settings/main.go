// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: namespace_settings — Namespace-level settings management.
//
// This example demonstrates:
//   - List and manage namespace users (RBAC)
//   - Add and remove users with different roles
//   - Update user roles within a namespace
//   - Configure cleanup policies for resource retention
//   - Create and manage namespace-scoped webhooks
package main

import (
	"context"
	"fmt"
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

	ns := "sandbox"

	// -----------------------------------------------------------------------
	// 1. List namespace users
	// -----------------------------------------------------------------------
	fmt.Println("--- List Namespace Users ---")

	users, err := client.NamespaceSettings().ListUsers(ctx, ns)
	if err != nil {
		fmt.Printf("List users returned: %v\n", err)
	} else {
		fmt.Printf("Found %d users in namespace '%s':\n", len(users), ns)
		for _, u := range users {
			fmt.Printf("  - %s (email=%s, role=%s, userId=%s)\n",
				u.Username, u.Email, u.Role, u.UserID)
		}
	}

	// -----------------------------------------------------------------------
	// 2. Add a user to the namespace
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Add User to Namespace ---")

	// Roles: owner, editor, viewer
	// - owner: full access (CRUD mocks, manage users, settings)
	// - editor: create/update/delete mocks, run tests
	// - viewer: read-only access
	err = client.NamespaceSettings().AddUser(ctx, ns, &mockarty.AddNamespaceUserRequest{
		UserID: "user-dev-001",
		Role:   "editor",
	})
	if err != nil {
		fmt.Printf("Add user returned: %v\n", err)
	} else {
		fmt.Println("Added user-dev-001 as 'editor'")
	}

	// Add a QA team member as viewer
	err = client.NamespaceSettings().AddUser(ctx, ns, &mockarty.AddNamespaceUserRequest{
		UserID: "user-qa-001",
		Role:   "viewer",
	})
	if err != nil {
		fmt.Printf("Add user returned: %v\n", err)
	} else {
		fmt.Println("Added user-qa-001 as 'viewer'")
	}

	defer func() {
		_ = client.NamespaceSettings().RemoveUser(ctx, ns, "user-dev-001")
		_ = client.NamespaceSettings().RemoveUser(ctx, ns, "user-qa-001")
		fmt.Println("\nUsers cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 3. Update a user's role
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Update User Role ---")

	// Promote the QA user to editor so they can modify mocks
	err = client.NamespaceSettings().UpdateUserRole(ctx, ns, "user-qa-001", "editor")
	if err != nil {
		fmt.Printf("Update role returned: %v\n", err)
	} else {
		fmt.Println("Promoted user-qa-001 from 'viewer' to 'editor'")
	}

	// -----------------------------------------------------------------------
	// 4. List users again to verify changes
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Verify User Changes ---")

	users, err = client.NamespaceSettings().ListUsers(ctx, ns)
	if err != nil {
		fmt.Printf("List users returned: %v\n", err)
	} else {
		fmt.Printf("Users after changes:\n")
		for _, u := range users {
			fmt.Printf("  - %s (role=%s)\n", u.UserID, u.Role)
		}
	}

	// -----------------------------------------------------------------------
	// 5. Configure cleanup policy
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Cleanup Policy ---")

	// Get current policy
	currentPolicy, err := client.NamespaceSettings().GetCleanupPolicy(ctx, ns)
	if err != nil {
		fmt.Printf("Get cleanup policy returned: %v\n", err)
	} else {
		fmt.Printf("Current cleanup policy:\n")
		fmt.Printf("  Mock retention: %d days\n", currentPolicy.MockRetentionDays)
		fmt.Printf("  Log retention: %d days\n", currentPolicy.LogRetentionDays)
		fmt.Printf("  Test run retention: %d days\n", currentPolicy.TestRunRetentionDays)
		fmt.Printf("  Auto cleanup: %t\n", currentPolicy.AutoCleanup)
	}

	// Update the cleanup policy.
	// This controls how long resources are retained before automatic deletion.
	err = client.NamespaceSettings().UpdateCleanupPolicy(ctx, ns, &mockarty.CleanupPolicy{
		MockRetentionDays:    90,   // keep mocks for 90 days
		LogRetentionDays:     30,   // keep request logs for 30 days
		TestRunRetentionDays: 60,   // keep test run results for 60 days
		AutoCleanup:          true, // enable automatic cleanup
	})
	if err != nil {
		fmt.Printf("Update cleanup policy returned: %v\n", err)
	} else {
		fmt.Println("Updated cleanup policy")
	}

	// Verify
	updated, err := client.NamespaceSettings().GetCleanupPolicy(ctx, ns)
	if err != nil {
		fmt.Printf("Get cleanup policy returned: %v\n", err)
	} else {
		fmt.Printf("Updated cleanup policy:\n")
		fmt.Printf("  Mock retention: %d days\n", updated.MockRetentionDays)
		fmt.Printf("  Log retention: %d days\n", updated.LogRetentionDays)
		fmt.Printf("  Test run retention: %d days\n", updated.TestRunRetentionDays)
		fmt.Printf("  Auto cleanup: %t\n", updated.AutoCleanup)
	}

	// -----------------------------------------------------------------------
	// 6. Namespace webhooks
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Namespace Webhooks ---")

	// Create a webhook that fires when mocks are created or updated.
	// Useful for notifications, audit logging, or CI/CD triggers.
	mockChangedHook, err := client.NamespaceSettings().CreateWebhook(ctx, ns, &mockarty.NamespaceWebhook{
		Operation: "mock.changed",
		URL:       "https://hooks.example.com/mockarty/mock-changed",
		Method:    "POST",
		Headers: map[string]string{
			"Authorization": "Bearer webhook-secret-token",
			"Content-Type":  "application/json",
		},
		Enabled: true,
	})
	if err != nil {
		fmt.Printf("Create webhook returned: %v\n", err)
	} else {
		fmt.Printf("Created webhook: id=%s, operation=%s, url=%s\n",
			mockChangedHook.ID, mockChangedHook.Operation, mockChangedHook.URL)
	}

	// Create a webhook for test run completion
	testRunHook, err := client.NamespaceSettings().CreateWebhook(ctx, ns, &mockarty.NamespaceWebhook{
		Operation: "testrun.completed",
		URL:       "https://hooks.example.com/mockarty/test-completed",
		Method:    "POST",
		Headers: map[string]string{
			"Authorization": "Bearer webhook-secret-token",
		},
		Enabled: true,
	})
	if err != nil {
		fmt.Printf("Create webhook returned: %v\n", err)
	} else {
		fmt.Printf("Created webhook: id=%s, operation=%s\n",
			testRunHook.ID, testRunHook.Operation)
	}

	defer func() {
		if mockChangedHook != nil {
			_ = client.NamespaceSettings().DeleteWebhook(ctx, ns, mockChangedHook.ID)
		}
		if testRunHook != nil {
			_ = client.NamespaceSettings().DeleteWebhook(ctx, ns, testRunHook.ID)
		}
		fmt.Println("Webhooks cleaned up.")
	}()

	// List all webhooks
	fmt.Println("\n--- List Webhooks ---")

	webhooks, err := client.NamespaceSettings().ListWebhooks(ctx, ns)
	if err != nil {
		fmt.Printf("List webhooks returned: %v\n", err)
	} else {
		fmt.Printf("Found %d webhooks:\n", len(webhooks))
		for _, w := range webhooks {
			enabledStr := "enabled"
			if !w.Enabled {
				enabledStr = "disabled"
			}
			fmt.Printf("  - %s -> %s (%s, %s)\n",
				w.Operation, w.URL, w.Method, enabledStr)
		}
	}

	// -----------------------------------------------------------------------
	// 7. Remove a user
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Remove User ---")

	err = client.NamespaceSettings().RemoveUser(ctx, ns, "user-dev-001")
	if err != nil {
		fmt.Printf("Remove user returned: %v\n", err)
	} else {
		fmt.Println("Removed user-dev-001 from namespace")
	}

	fmt.Println("\nNamespace settings examples completed!")
}
