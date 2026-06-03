// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// RunCollection runs a whole collection as a perf suite. The server fans the
// collection out into one task per request and returns {runGroupId, taskIds} —
// the SDK previously decoded that into a single PerfTask, silently dropping
// every id.
func TestPerf_RunCollection_DecodesRunGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/perf/run-collection" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"runGroupId":"grp-1","taskIds":["t1","t2","t3"]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	group, err := c.Perf().RunCollection(context.Background(), map[string]any{
		"collectionId": "col-1",
		"vus":          5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if group.RunGroupID != "grp-1" {
		t.Fatalf("RunGroupID = %q, want grp-1", group.RunGroupID)
	}
	if len(group.TaskIDs) != 3 || group.TaskIDs[0] != "t1" || group.TaskIDs[2] != "t3" {
		t.Fatalf("TaskIDs = %v, want [t1 t2 t3]", group.TaskIDs)
	}
}
