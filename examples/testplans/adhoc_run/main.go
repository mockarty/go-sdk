// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: testplans/adhoc_run — end-to-end demo of the TP-6b surface.
//
// Flow:
//  1. Create a curated Test Plan (legacy POST /test-plans).
//  2. Patch the plan with If-Match optimistic concurrency.
//  3. Dispatch an ad-hoc run (POST /namespaces/:ns/test-runs/ad-hoc) —
//     the server provisions a hidden plan row and kicks the orchestrator.
//  4. Poll until the run reaches a terminal status.
//  5. Fetch the merged Allure JSON report.
//  6. Stream the Allure ZIP to ./adhoc-report.zip.
//  7. Soft-delete the curated plan from step 1.
//
// Required environment:
//
//	MOCKARTY_SERVER      http(s)://host:port of the admin node.
//	MOCKARTY_TOKEN       API key with mock:write in the namespace.
//	MOCKARTY_NAMESPACE   Target namespace.
//	MOCKARTY_COLLECTION_ID  UUID of a Functional collection.
//	MOCKARTY_FUZZ_CONFIG_ID UUID of a Fuzz config (optional).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	namespace := os.Getenv("MOCKARTY_NAMESPACE")
	if namespace == "" {
		namespace = "sandbox"
	}
	collectionID := os.Getenv("MOCKARTY_COLLECTION_ID")
	if collectionID == "" {
		log.Fatal("MOCKARTY_COLLECTION_ID is required")
	}
	fuzzID := os.Getenv("MOCKARTY_FUZZ_CONFIG_ID")

	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_TOKEN")),
		mockarty.WithNamespace(namespace),
		mockarty.WithRetry(3, 500*time.Millisecond),
	)
	api := client.TestPlans()

	// ---------------------------------------------------------------------
	// 1. Create a curated plan so we have something to Patch + Delete.
	// ---------------------------------------------------------------------
	plan, err := api.Create(ctx, mockarty.TestPlan{
		Name:        "SDK adhoc demo",
		Description: "Created by examples/testplans/adhoc_run",
		Items: []mockarty.TestPlanItem{
			{Order: 1, Type: mockarty.PlanItemTypeFunctional, ResourceID: collectionID, Name: "Smoke"},
		},
	})
	if err != nil {
		log.Fatalf("create: %v", err)
	}
	fmt.Printf("Step 1  Created plan %s (#%d)\n", plan.ID, plan.NumericID)

	// ---------------------------------------------------------------------
	// 2. Patch — use the plan's UpdatedAt for If-Match. Loop once on 412
	//    to demonstrate the recommended pattern.
	// ---------------------------------------------------------------------
	newDesc := "Renamed by Patch demo"
	patchOpts := mockarty.PatchOptions{
		IfMatch: fmt.Sprintf(`"%d"`, plan.UpdatedAt.UnixMilli()),
	}
	patched, perr := api.Patch(ctx, plan.ID, mockarty.PatchPlanRequest{Description: &newDesc}, patchOpts)
	switch {
	case errors.Is(perr, mockarty.ErrPreconditionFailed):
		// Re-fetch & retry once — the scheduler touches UpdatedAt on every
		// firing, so stale etags happen in busy namespaces.
		fresh, ferr := api.Get(ctx, plan.ID)
		if ferr != nil {
			log.Fatalf("refetch: %v", ferr)
		}
		patchOpts.IfMatch = fmt.Sprintf(`"%d"`, fresh.UpdatedAt.UnixMilli())
		patched, perr = api.Patch(ctx, plan.ID, mockarty.PatchPlanRequest{Description: &newDesc}, patchOpts)
	}
	if perr != nil {
		log.Fatalf("patch: %v", perr)
	}
	fmt.Printf("Step 2  Patched plan — description=%q\n", patched.Description)

	// ---------------------------------------------------------------------
	// 3. Ad-hoc run — fire a new combo without saving a plan row.
	// ---------------------------------------------------------------------
	items := []mockarty.AdHocItem{
		{Order: 1, Type: mockarty.PlanItemTypeFunctional, RefID: collectionID},
	}
	if fuzzID != "" {
		items = append(items, mockarty.AdHocItem{
			Order: 2, Type: mockarty.PlanItemTypeFuzz, RefID: fuzzID,
			DelayAfterMs: 500,
		})
	}
	adhoc, aerr := api.CreateAdHocRun(ctx, mockarty.CreateAdHocRunRequest{
		Namespace:   namespace,
		Name:        "adhoc-demo",
		Description: "created from Go SDK example",
		Schedule:    "", // FIFO
		Items:       items,
	})
	if aerr != nil {
		log.Fatalf("ad-hoc run: %v", aerr)
	}
	fmt.Printf("Step 3  Dispatched ad-hoc run %s for plan %s (status=%s)\n",
		adhoc.RunID, adhoc.PlanID, adhoc.Status)

	// ---------------------------------------------------------------------
	// 4. Poll until the run reaches a terminal state.
	// ---------------------------------------------------------------------
	final, werr := api.WaitForRun(ctx, adhoc.RunID, 5*time.Second)
	switch {
	case werr == nil:
		fmt.Printf("Step 4  Run PASSED (%d/%d items)\n",
			final.CompletedItems-final.FailedItems, final.TotalItems)
	case errors.Is(werr, mockarty.ErrRunFailed):
		fmt.Printf("Step 4  Run FAILED (%d failed of %d)\n", final.FailedItems, final.TotalItems)
	case errors.Is(werr, mockarty.ErrRunCancelled):
		fmt.Printf("Step 4  Run CANCELLED\n")
	default:
		log.Fatalf("wait: %v", werr)
	}

	// ---------------------------------------------------------------------
	// 5. Fetch the merged Allure JSON — useful for CI dashboards that
	//    render their own tables on top of the raw summary.
	// ---------------------------------------------------------------------
	report, rerr := api.GetRunReport(ctx, namespace, adhoc.PlanID, adhoc.RunID)
	if rerr != nil {
		log.Printf("Step 5  GetRunReport: %v", rerr)
	} else {
		fmt.Printf("Step 5  Report has %d items; raw payload = %d bytes\n",
			len(report.Items), len(report.Raw))
	}

	// ---------------------------------------------------------------------
	// 6. Stream the Allure ZIP to disk (the CI artifact store target).
	// ---------------------------------------------------------------------
	rc, zerr := api.GetRunReportZIP(ctx, namespace, adhoc.PlanID, adhoc.RunID)
	if zerr != nil {
		log.Printf("Step 6  GetRunReportZIP: %v", zerr)
	} else {
		defer rc.Close()
		f, cerr := os.Create("adhoc-report.zip")
		if cerr != nil {
			log.Fatalf("create zip: %v", cerr)
		}
		defer f.Close()
		n, copyErr := io.Copy(f, rc)
		if copyErr != nil {
			log.Printf("Step 6  copy zip: %v", copyErr)
		} else {
			fmt.Printf("Step 6  Wrote adhoc-report.zip (%d bytes)\n", n)
		}
	}

	// ---------------------------------------------------------------------
	// 7. Soft-delete the curated plan (the ad-hoc one is hidden already).
	// ---------------------------------------------------------------------
	if err := api.Delete(ctx, plan.ID); err != nil {
		log.Fatalf("delete: %v", err)
	}
	fmt.Printf("Step 7  Soft-deleted curated plan %s\n", plan.ID)
}
