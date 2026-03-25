// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: agent_tasks — AI Agent Task API usage.
//
// This example demonstrates:
//   - Submit a task to the AI agent
//   - Track task status until completion
//   - Retrieve and inspect the result
//   - Rerun a completed task
//   - Export task results
//   - List, cancel, and clean up agent tasks
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. Submit a task to the AI agent
	// -----------------------------------------------------------------------
	fmt.Println("--- Submit Agent Task ---")

	// The AI agent can generate mocks, analyze APIs, suggest test scenarios,
	// and more. Provide a natural language prompt describing what you need.
	task, err := client.AgentTasks().Submit(ctx, &mockarty.AgentTask{
		Prompt: "Create a REST API mock for a user management service with " +
			"CRUD endpoints: GET /users, GET /users/:id, POST /users, " +
			"PUT /users/:id, DELETE /users/:id. Each endpoint should return " +
			"realistic fake data using Faker functions.",
	})
	if err != nil {
		log.Fatalf("Failed to submit agent task: %v", err)
	}
	fmt.Printf("Task submitted: id=%s, status=%s\n", task.ID, task.Status)

	// -----------------------------------------------------------------------
	// 2. Track task status until completion
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Track Task Status ---")

	// Poll the task status. In production, you might use a longer interval
	// or implement a webhook-based notification approach.
	for i := 0; i < 30; i++ {
		current, err := client.AgentTasks().Get(ctx, task.ID)
		if err != nil {
			fmt.Printf("Get task returned: %v\n", err)
			break
		}

		fmt.Printf("  [%d] status=%s\n", i+1, current.Status)

		if current.Status == "completed" || current.Status == "failed" || current.Status == "cancelled" {
			task = current
			break
		}

		time.Sleep(2 * time.Second)
	}

	// -----------------------------------------------------------------------
	// 3. Inspect the result
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Task Result ---")

	if task.Status == "completed" {
		fmt.Printf("Task completed successfully!\n")
		fmt.Printf("  Result: %v\n", task.Result)
	} else if task.Status == "failed" {
		fmt.Printf("Task failed.\n")
		fmt.Printf("  Result: %v\n", task.Result)
	} else {
		fmt.Printf("Task is still in status: %s\n", task.Status)
	}

	// -----------------------------------------------------------------------
	// 4. Export task results
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Export Task ---")

	exportData, err := client.AgentTasks().Export(ctx, task.ID)
	if err != nil {
		fmt.Printf("Export task returned: %v\n", err)
	} else {
		fmt.Printf("Exported task result: %d bytes\n", len(exportData))

		preview := string(exportData)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Printf("Preview: %s\n", preview)
	}

	// -----------------------------------------------------------------------
	// 5. Rerun a completed task
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Rerun Task ---")

	// Rerunning creates a new execution of the same prompt.
	// Useful when the AI produces non-deterministic results and you
	// want to try a different generation.
	rerunTask, err := client.AgentTasks().Rerun(ctx, task.ID)
	if err != nil {
		fmt.Printf("Rerun task returned: %v\n", err)
	} else {
		fmt.Printf("Rerun task created: id=%s, status=%s\n",
			rerunTask.ID, rerunTask.Status)

		// Cancel the rerun to avoid consuming resources
		err = client.AgentTasks().Cancel(ctx, rerunTask.ID)
		if err != nil {
			fmt.Printf("Cancel rerun returned: %v\n", err)
		} else {
			fmt.Println("Rerun task cancelled")
		}
	}

	// -----------------------------------------------------------------------
	// 6. Submit another task — analyze an existing API
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Submit Analysis Task ---")

	analysisTask, err := client.AgentTasks().Submit(ctx, &mockarty.AgentTask{
		Prompt: "Analyze the OpenAPI spec at http://localhost:5770/swagger/doc.json " +
			"and suggest which endpoints should have contract tests. " +
			"Focus on critical business endpoints.",
	})
	if err != nil {
		fmt.Printf("Submit analysis task returned: %v\n", err)
	} else {
		fmt.Printf("Analysis task submitted: id=%s\n", analysisTask.ID)
	}

	// -----------------------------------------------------------------------
	// 7. List all agent tasks
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List All Tasks ---")

	tasks, err := client.AgentTasks().List(ctx)
	if err != nil {
		fmt.Printf("List tasks returned: %v\n", err)
	} else {
		fmt.Printf("Found %d agent tasks:\n", len(tasks))
		for _, t := range tasks {
			createdAt := "unknown"
			if t.CreatedAt > 0 {
				createdAt = time.Unix(t.CreatedAt, 0).Format(time.RFC3339)
			}
			prompt := t.Prompt
			if len(prompt) > 60 {
				prompt = prompt[:60] + "..."
			}
			fmt.Printf("  - id=%s, status=%s, created=%s\n    prompt: %s\n",
				t.ID, t.Status, createdAt, prompt)
		}
	}

	// -----------------------------------------------------------------------
	// 8. Delete a specific task
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Delete Task ---")

	err = client.AgentTasks().Delete(ctx, task.ID)
	if err != nil {
		fmt.Printf("Delete task returned: %v\n", err)
	} else {
		fmt.Printf("Deleted task: %s\n", task.ID)
	}

	// -----------------------------------------------------------------------
	// 9. Clear all tasks
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Clear All Tasks ---")

	err = client.AgentTasks().ClearAll(ctx)
	if err != nil {
		fmt.Printf("Clear all tasks returned: %v\n", err)
	} else {
		fmt.Println("All agent tasks cleared")
	}

	fmt.Println("\nAgent task examples completed!")
}
