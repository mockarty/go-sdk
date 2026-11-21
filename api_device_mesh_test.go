// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The mesh SDK is what makes the device mesh usable from CI: place a command,
// wait for the receipt, fail the job on a failed command. These assertions pin
// the request shapes against the routes the server actually serves, because a
// wrong verb or path here fails at run time in somebody's pipeline.
func TestDeviceMeshRequestShapes(t *testing.T) {
	type captured struct {
		method string
		path   string
		query  string
		body   map[string]any
	}
	var seen []captured

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entry := captured{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &entry.body)
		}
		seen = append(seen, entry)

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/devices") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"devices":[{"deviceId":"desk-1","namespace":"team-a","kind":"desktop","state":"online","capabilities":["api_test"]}]}`))
		case strings.HasSuffix(r.URL.Path, "/grants") && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"deviceId":"desk-1","granteeUserId":"user-2","capabilities":["api_test"]}`))
		case strings.Contains(r.URL.Path, "/grants/") && r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"revoked":true}`))
		case strings.HasSuffix(r.URL.Path, "/commands") && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"command":{"id":"cmd-1","kind":"api_test.collection","state":"queued","targetDeviceId":"desk-1"},"replayed":true}`))
		case strings.HasSuffix(r.URL.Path, "/report"):
			_, _ = w.Write([]byte(`{"id":"cmd-1","state":"failed","failureReason":"engine exited 2"}`))
		case strings.HasSuffix(r.URL.Path, "/reports"):
			_, _ = w.Write([]byte(`{"reports":[{"id":"rep-1","ref":"devicemesh.command:cmd-1","deviceId":"desk-1","state":"failed","failureCode":"engine exited 2"}]}`))
		case strings.HasSuffix(r.URL.Path, "/drain"):
			_, _ = w.Write([]byte(`{"deviceId":"desk-1","state":"draining"}`))
		default:
			_, _ = w.Write([]byte(`{"id":"cmd-1","state":"failed","failureReason":"engine exited 2"}`))
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, WithNamespace("team-a"))
	ctx := context.Background()

	devices, err := client.DeviceMesh().ListDevices(ctx, "")
	if err != nil || len(devices) != 1 || devices[0].DeviceID != "desk-1" || devices[0].Capabilities[0] != "api_test" {
		t.Fatalf("list devices: %v %+v", err, devices)
	}

	if _, err := client.DeviceMesh().CreateGrant(ctx, "team-a", "desk-1", "user-2", []string{"api_test"}); err != nil {
		t.Fatalf("create grant: %v", err)
	}
	if err := client.DeviceMesh().RevokeGrant(ctx, "team-a", "desk-1", "user-2"); err != nil {
		t.Fatalf("revoke grant: %v", err)
	}

	command, replayed, err := client.DeviceMesh().Submit(ctx, "team-a", MeshSubmitInput{
		Kind:                 "api_test.collection",
		Payload:              json.RawMessage(`{"collectionId":"11111111-1111-4111-8111-111111111111"}`),
		TargetDeviceID:       "desk-1",
		RequiredCapabilities: []string{"api_test"},
		IdempotencyKey:       "ci-run-42",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if command.ID != "cmd-1" || !replayed {
		t.Fatalf("submit result: %+v replayed=%v (a replayed key must be reported, not hidden)", command, replayed)
	}

	if _, err := client.DeviceMesh().Report(ctx, "team-a", "desk-1", "cmd-1", MeshCommandReport{
		State: "failed", ResultSummary: "0 of 3 passed", FailureReason: "engine exited 2",
	}); err != nil {
		t.Fatalf("report: %v", err)
	}
	reports, err := client.DeviceMesh().ListReports(ctx, "team-a", "cmd-1")
	if err != nil || len(reports) != 1 || reports[0].FailureCode != "engine exited 2" {
		t.Fatalf("reports: %v %+v", err, reports)
	}
	drained, err := client.DeviceMesh().SetDrain(ctx, "team-a", "desk-1", true, "maintenance")
	if err != nil || drained.State != "draining" {
		t.Fatalf("drain: %v %+v", err, drained)
	}

	// The shapes, one by one: the SDK must call the routes the server serves.
	want := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/namespaces/team-a/device-mesh/devices"},
		{http.MethodPost, "/api/v1/namespaces/team-a/device-mesh/devices/desk-1/grants"},
		{http.MethodDelete, "/api/v1/namespaces/team-a/device-mesh/devices/desk-1/grants/user-2"},
		{http.MethodPost, "/api/v1/namespaces/team-a/device-mesh/commands"},
		{http.MethodPost, "/api/v1/namespaces/team-a/device-mesh/commands/cmd-1/report"},
		{http.MethodGet, "/api/v1/namespaces/team-a/device-mesh/reports"},
		{http.MethodPost, "/api/v1/namespaces/team-a/device-mesh/devices/desk-1/drain"},
	}
	if len(seen) != len(want) {
		t.Fatalf("made %d requests, want %d: %+v", len(seen), len(want), seen)
	}
	for i, expected := range want {
		if seen[i].method != expected.method || seen[i].path != expected.path {
			t.Errorf("request %d = %s %s, want %s %s", i, seen[i].method, seen[i].path, expected.method, expected.path)
		}
	}
	// The device id travels as a query parameter on the report route, which is
	// easy to get wrong and impossible to notice until it 404s in production.
	if !strings.Contains(seen[4].query, "deviceId=desk-1") {
		t.Errorf("report query = %q, want deviceId=desk-1", seen[4].query)
	}
	if !strings.Contains(seen[5].query, "commandId=cmd-1") {
		t.Errorf("reports query = %q, want commandId=cmd-1", seen[5].query)
	}
	if seen[3].body["idempotencyKey"] != "ci-run-42" {
		t.Errorf("submit body = %v, want the idempotency key carried", seen[3].body)
	}
	if seen[6].body["draining"] != true {
		t.Errorf("drain body = %v, want draining:true", seen[6].body)
	}
}

// A missing namespace must fail before any request: a call that silently used
// the wrong workspace would place work on someone else's machines.
func TestDeviceMeshRequiresNamespace(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	client.namespace = ""
	if _, err := client.DeviceMesh().ListDevices(context.Background(), ""); err == nil {
		t.Fatal("a missing namespace must be refused before the request")
	}
	if _, _, err := client.DeviceMesh().Submit(context.Background(), "", MeshSubmitInput{Kind: "ping", IdempotencyKey: "k"}); err == nil {
		t.Fatal("submit without a namespace must be refused")
	}
}
