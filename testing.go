// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"testing"
)

// CreateMockT creates a mock and registers a cleanup function that deletes
// it when the test completes. This is the recommended way to create mocks
// in tests.
func (c *Client) CreateMockT(t *testing.T, mock *Mock) *Mock {
	t.Helper()

	resp, err := c.Mocks().Create(context.Background(), mock)
	if err != nil {
		t.Fatalf("mockarty: create mock: %v", err)
	}

	t.Cleanup(func() {
		_ = c.Mocks().Delete(context.Background(), resp.Mock.ID)
	})

	return &resp.Mock
}

// SetupNamespaceT creates a namespace (if it doesn't already exist) and
// registers a cleanup function that removes all mocks in the namespace
// when the test completes.
func (c *Client) SetupNamespaceT(t *testing.T, namespace string) {
	t.Helper()

	// Create namespace; ignore error if it already exists.
	_ = c.Namespaces().Create(context.Background(), namespace)

	t.Cleanup(func() {
		// List all mocks in the namespace and delete them.
		list, err := c.Mocks().List(context.Background(), &ListMocksOptions{
			Namespace: namespace,
			Limit:     1000,
		})
		if err != nil {
			return
		}
		for _, m := range list.Items {
			_ = c.Mocks().Delete(context.Background(), m.ID)
		}
	})
}

// MustCreateMock is a convenience that creates a mock from a builder,
// registers cleanup, and returns the created mock. Fails the test on error.
func (c *Client) MustCreateMock(t *testing.T, builder *MockBuilder) *Mock {
	t.Helper()
	return c.CreateMockT(t, builder.Build())
}

// RunPlanT triggers a Test Plan run, blocks until it reaches a terminal
// state (or the test deadline trips), and fails the test on a non-success
// outcome. The plan run id is returned so the caller can attach it as a
// metadata reference (e.g. as a CI artifact link).
//
// This is the process-level glue point: a single line in a Go test boots
// a Mockarty plan, waits, and surfaces the verdict. Use it when your test
// is the orchestrator and the plan run is its work unit.
//
// The method intentionally does NOT mutate the plan or its items — write
// access to plan content is a separate, deliberate operation. If the
// caller needs cancellation, pass a ctx with a timeout into the
// underlying TestPlansAPI directly; this helper uses ctx.Background and
// the test deadline.
func (c *Client) RunPlanT(t *testing.T, planID string) (runID string) {
	t.Helper()
	ctx := context.Background()
	run, err := c.TestPlans().Run(ctx, planID, RunOptions{})
	if err != nil {
		t.Fatalf("mockarty: trigger plan run for %q: %v", planID, err)
	}
	if run == nil || run.ID == "" {
		t.Fatalf("mockarty: trigger plan run for %q returned no run id", planID)
	}
	final, err := c.TestPlans().WaitForRun(ctx, run.ID, 0)
	if err != nil {
		t.Fatalf("mockarty: wait plan run %q: %v", run.ID, err)
	}
	switch final.Status {
	case "succeeded", "completed", "passed":
		if final.FailedItems > 0 {
			t.Fatalf("mockarty: plan run %s ended %s but %d items failed",
				final.ID, final.Status, final.FailedItems)
		}
	default:
		t.Fatalf("mockarty: plan run %s ended in %s (%d/%d failed)",
			final.ID, final.Status, final.FailedItems, final.TotalItems)
	}
	return run.ID
}
