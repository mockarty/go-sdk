// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: tags_and_folders — Tag and Folder API usage for organizing mocks.
//
// This example demonstrates:
//   - Create tags to categorize mocks
//   - List all tags with usage counts
//   - Create a folder hierarchy for mock organization
//   - Move mocks into folders
//   - Batch update tags on mocks
//   - Rename and rearrange folders
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

	// -----------------------------------------------------------------------
	// 1. Create tags
	// -----------------------------------------------------------------------
	fmt.Println("--- Create Tags ---")

	// Tags help categorize mocks by service, team, version, or any dimension.
	tagNames := []string{"user-service", "payment-api", "v2", "critical", "staging"}
	for _, name := range tagNames {
		tag, err := client.Tags().Create(ctx, name)
		if err != nil {
			fmt.Printf("  Create tag '%s' returned: %v\n", name, err)
		} else {
			fmt.Printf("  Created tag: %s\n", tag.Name)
		}
	}

	// -----------------------------------------------------------------------
	// 2. List all tags
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Tags ---")

	tags, err := client.Tags().List(ctx)
	if err != nil {
		log.Fatalf("Failed to list tags: %v", err)
	}

	fmt.Printf("Found %d tags:\n", len(tags))
	for _, t := range tags {
		fmt.Printf("  - %s (used by %d mocks)\n", t.Name, t.Count)
	}

	// -----------------------------------------------------------------------
	// 3. Create a folder hierarchy
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Create Folder Hierarchy ---")

	// Root folder for an entire service
	apiFolder, err := client.Folders().Create(ctx, &mockarty.MockFolder{
		Name: "User Service API",
	})
	if err != nil {
		log.Fatalf("Failed to create root folder: %v", err)
	}
	fmt.Printf("Created root folder: id=%s, name=%s\n", apiFolder.ID, apiFolder.Name)

	// Sub-folder for CRUD operations
	crudFolder, err := client.Folders().Create(ctx, &mockarty.MockFolder{
		Name:     "CRUD Operations",
		ParentID: apiFolder.ID,
	})
	if err != nil {
		log.Fatalf("Failed to create CRUD folder: %v", err)
	}
	fmt.Printf("Created sub-folder: id=%s, name=%s, parent=%s\n",
		crudFolder.ID, crudFolder.Name, crudFolder.ParentID)

	// Sub-folder for error scenarios
	errorsFolder, err := client.Folders().Create(ctx, &mockarty.MockFolder{
		Name:     "Error Scenarios",
		ParentID: apiFolder.ID,
	})
	if err != nil {
		log.Fatalf("Failed to create errors folder: %v", err)
	}
	fmt.Printf("Created sub-folder: id=%s, name=%s, parent=%s\n",
		errorsFolder.ID, errorsFolder.Name, errorsFolder.ParentID)

	// Sub-folder for auth scenarios
	authFolder, err := client.Folders().Create(ctx, &mockarty.MockFolder{
		Name:     "Auth & Permissions",
		ParentID: apiFolder.ID,
	})
	if err != nil {
		log.Fatalf("Failed to create auth folder: %v", err)
	}
	fmt.Printf("Created sub-folder: id=%s, name=%s, parent=%s\n",
		authFolder.ID, authFolder.Name, authFolder.ParentID)

	// Clean up folders at the end
	defer func() {
		_ = client.Folders().Delete(ctx, authFolder.ID)
		_ = client.Folders().Delete(ctx, errorsFolder.ID)
		_ = client.Folders().Delete(ctx, crudFolder.ID)
		_ = client.Folders().Delete(ctx, apiFolder.ID)
		fmt.Println("\nAll folders cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 4. List folders
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Folders ---")

	folders, err := client.Folders().List(ctx)
	if err != nil {
		log.Fatalf("Failed to list folders: %v", err)
	}

	fmt.Printf("Found %d folders:\n", len(folders))
	for _, f := range folders {
		parent := "(root)"
		if f.ParentID != "" {
			parent = f.ParentID
		}
		fmt.Printf("  - %s (id=%s, parent=%s, order=%d)\n",
			f.Name, f.ID, parent, f.SortOrder)
	}

	// -----------------------------------------------------------------------
	// 5. Create mocks and organize them with tags and folders
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Create and Organize Mocks ---")

	// Create mocks that will be organized with tags and folders
	mockIDs := []string{}
	mocks := []struct {
		id     string
		route  string
		method string
		tags   []string
		folder string
	}{
		{"user-list", "/api/users", "GET", []string{"user-service", "v2"}, crudFolder.ID},
		{"user-get", "/api/users/:id", "GET", []string{"user-service", "v2"}, crudFolder.ID},
		{"user-create", "/api/users", "POST", []string{"user-service", "v2", "critical"}, crudFolder.ID},
		{"user-not-found", "/api/users/:id", "GET", []string{"user-service", "v2"}, errorsFolder.ID},
		{"user-unauthorized", "/api/users", "GET", []string{"user-service", "critical"}, authFolder.ID},
	}

	for _, m := range mocks {
		mock := mockarty.NewMockBuilder().
			ID(m.id).
			Tags(m.tags...).
			FolderID(m.folder).
			HTTP(func(h *mockarty.HTTPBuilder) {
				h.Route(m.route).Method(m.method)
			}).
			Response(func(r *mockarty.ResponseBuilder) {
				r.Status(200).JSONBody(map[string]any{"mock": m.id})
			}).
			Build()

		if _, err := client.Mocks().Create(ctx, mock); err != nil {
			fmt.Printf("  Failed to create mock %s: %v\n", m.id, err)
		} else {
			fmt.Printf("  Created mock: %s (tags=%v, folder=%s)\n", m.id, m.tags, m.folder)
			mockIDs = append(mockIDs, m.id)
		}
	}

	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("All example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 6. Batch update tags on mocks
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Batch Update Tags ---")

	// Add the "staging" tag to all mocks in one operation
	err = client.Mocks().BatchUpdateTags(ctx, mockIDs, []string{"staging", "user-service", "v2"})
	if err != nil {
		fmt.Printf("Batch update tags returned: %v\n", err)
	} else {
		fmt.Printf("Updated tags on %d mocks (added 'staging' tag)\n", len(mockIDs))
	}

	// Verify tags were applied
	for _, id := range mockIDs[:2] { // check first two
		mock, err := client.Mocks().Get(ctx, id)
		if err != nil {
			continue
		}
		fmt.Printf("  %s tags: %v\n", id, mock.Tags)
	}

	// -----------------------------------------------------------------------
	// 7. Move mocks between folders
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Move Mocks to Folder ---")

	// Move the "not found" mock from errors to CRUD folder
	err = client.Mocks().MoveToFolder(ctx, []string{"user-not-found"}, crudFolder.ID)
	if err != nil {
		fmt.Printf("Move to folder returned: %v\n", err)
	} else {
		fmt.Println("Moved 'user-not-found' mock to CRUD folder")
	}

	// -----------------------------------------------------------------------
	// 8. Rename a folder
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Update Folder ---")

	updated, err := client.Folders().Update(ctx, errorsFolder.ID, &mockarty.MockFolder{
		Name:      "Error & Edge Case Scenarios",
		SortOrder: 2,
	})
	if err != nil {
		fmt.Printf("Update folder returned: %v\n", err)
	} else {
		fmt.Printf("Renamed folder to: %s (order=%d)\n", updated.Name, updated.SortOrder)
	}

	// -----------------------------------------------------------------------
	// 9. Move a folder to a different parent
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Move Folder ---")

	// Move auth folder under CRUD folder (making it a sub-sub-folder)
	err = client.Folders().Move(ctx, authFolder.ID, crudFolder.ID)
	if err != nil {
		fmt.Printf("Move folder returned: %v\n", err)
	} else {
		fmt.Println("Moved 'Auth & Permissions' folder under 'CRUD Operations'")
	}

	// -----------------------------------------------------------------------
	// 10. List mocks filtered by tag
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Mocks by Tag ---")

	list, err := client.Mocks().List(ctx, &mockarty.ListMocksOptions{
		Namespace: "sandbox",
		Tags:      []string{"critical"},
		Limit:     50,
	})
	if err != nil {
		fmt.Printf("List by tag returned: %v\n", err)
	} else {
		fmt.Printf("Mocks with tag 'critical': %d\n", list.Total)
		for _, m := range list.Items {
			fmt.Printf("  - %s (tags=%v, folder=%s)\n", m.ID, m.Tags, m.FolderID)
		}
	}

	// -----------------------------------------------------------------------
	// 11. List tags again to see updated counts
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Updated Tag Counts ---")

	tags, err = client.Tags().List(ctx)
	if err != nil {
		fmt.Printf("List tags returned: %v\n", err)
	} else {
		for _, t := range tags {
			fmt.Printf("  - %s (used by %d mocks)\n", t.Name, t.Count)
		}
	}

	fmt.Println("\nTags and folders examples completed!")
}
