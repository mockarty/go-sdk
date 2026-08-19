// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: task_automation — drive the issue tracker + TCM from the SDK.
//
// Files a bug in the issue tracker, then creates a test case, runs it, and (on
// failure) files a defect — the kind of end-to-end automation a CI agent does.
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
	client := mockarty.NewClient(
		envOr("MOCKARTY_SERVER", "http://localhost:5770"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(envOr("MOCKARTY_NAMESPACE", "sandbox")))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Issue tracker: claim the next issue, comment, transition.
	it := client.IssueTracker()
	projects, err := it.ListProjects(ctx, "")
	if err != nil {
		log.Fatalf("list projects: %v", err)
	}
	if len(projects) == 0 {
		log.Fatal("no projects in this namespace")
	}
	pid := fmt.Sprint(projects[0]["id"])

	issue, err := it.CreateIssue(ctx, "", mockarty.Issue{
		"projectId": pid, "type": "bug", "title": "Checkout returns 500",
	})
	if err != nil {
		log.Fatalf("create issue: %v", err)
	}
	issueID := fmt.Sprint(issue["id"])
	fmt.Printf("filed issue %v\n", issue["issueKey"])
	_, _ = it.AddComment(ctx, "", issueID, "reproduced on staging")
	_, _ = it.MoveIssue(ctx, "", issueID, "in_progress", "")

	// 2. TCM: create a case, run it, file a defect on failure.
	tcm := client.TCM()
	c, err := tcm.CreateCase(ctx, "", mockarty.TCMObject{"title": "Checkout smoke"})
	if err != nil {
		log.Fatalf("create case: %v", err)
	}
	run, err := tcm.RunCase(ctx, "", fmt.Sprint(c["id"]), nil)
	if err != nil {
		log.Fatalf("run case: %v", err)
	}
	fmt.Printf("case run started: %v\n", run["runId"])
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
