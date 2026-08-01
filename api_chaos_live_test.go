// Copyright (c) 2026 Mockarty. All rights reserved.

package mockarty

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestChaosLiveDogfood drives the chaos SDK surface against a running admin
// and a real kind cluster, exercising the methods added in the 2026-07-31
// parity pass (connect-status round-trip, approve flow, schedules). Gated by
// MOCKARTY_LIVE_TOKEN; MOCKARTY_LIVE_KUBECONFIG points at a kind kubeconfig.
//
//	MOCKARTY_LIVE_TOKEN=<mk_...> \
//	MOCKARTY_LIVE_URL=http://127.0.0.1:5870 \
//	MOCKARTY_LIVE_KUBECONFIG=/tmp/kind_kc.yaml \
//	go test ./... -run TestChaosLiveDogfood -v
func TestChaosLiveDogfood(t *testing.T) {
	token := os.Getenv("MOCKARTY_LIVE_TOKEN")
	if token == "" {
		t.Skip("set MOCKARTY_LIVE_TOKEN to run the chaos dogfood")
	}
	base := os.Getenv("MOCKARTY_LIVE_URL")
	if base == "" {
		base = "http://127.0.0.1:5870"
	}
	c := NewClient(base, WithAPIKey(token), WithNamespace("sandbox"))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Presets come back through the typed model.
	presets, err := c.Chaos().ListPresets(ctx)
	if err != nil {
		t.Fatalf("ListPresets: %v", err)
	}
	if len(presets) == 0 {
		t.Fatal("expected built-in presets")
	}
	t.Logf("presets: %d", len(presets))

	// 2. Schedules list round-trips (was absent from the SDK before this pass).
	if _, err := c.Chaos().ListSchedules(ctx); err != nil {
		t.Fatalf("ListSchedules: %v", err)
	}

	kubeconfigPath := os.Getenv("MOCKARTY_LIVE_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Log("no MOCKARTY_LIVE_KUBECONFIG — skipping profile/connect/approve flow")
		return
	}
	kc, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		t.Fatalf("read kubeconfig: %v", err)
	}

	// 3. Create a profile from the kind kubeconfig, connect, and verify the
	//    connected/status/cluster_type fields the SDK model now carries actually
	//    populate (they were silently dropped before the repo fix).
	prof, err := c.Chaos().CreateProfile(ctx, &ChaosProfile{
		Name:           "go-sdk-dogfood",
		KubeconfigData: string(kc),
	})
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	t.Cleanup(func() { _ = c.Chaos().DeleteProfile(context.Background(), prof.ID) })

	res, err := c.Chaos().ConnectProfile(ctx, prof.ID)
	if err != nil {
		t.Fatalf("ConnectProfile: %v", err)
	}
	if !res.Connected {
		t.Fatalf("connect reported not connected: %+v", res)
	}

	profiles, err := c.Chaos().ListProfiles(ctx)
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	var mine *ChaosProfile
	for i := range profiles {
		if profiles[i].ID == prof.ID {
			mine = &profiles[i]
		}
	}
	if mine == nil {
		t.Fatal("created profile not in list")
	}
	if !mine.Connected || mine.Status != "connected" {
		t.Fatalf("SDK model lost connected/status: connected=%v status=%q", mine.Connected, mine.Status)
	}
	if mine.ClusterType == "" {
		t.Fatalf("SDK model lost cluster_type")
	}
	t.Logf("profile status round-trip OK: status=%s cluster_type=%s connected=%v", mine.Status, mine.ClusterType, mine.Connected)

	// 4. Approval flow: create a gated experiment, verify it sits pending_approval,
	//    then Approve it (the approve method was absent from the SDK before).
	exp, err := c.Chaos().Create(ctx, &ChaosExperiment{
		Name:           "go-sdk-approval",
		Namespace:      "sandbox",
		InfraProfileID: prof.ID,
		DurationSec:    20,
		Faults:         []FaultConfig{{Type: "pod_kill_random", IntervalSec: 5}},
		Target:         TargetConfig{Mode: "random_one", Namespace: "default", Selector: map[string]string{"app": "none"}},
		Safety:         SafetyConfig{RequireApproval: true},
	})
	if err != nil {
		t.Fatalf("Create gated experiment: %v", err)
	}
	t.Cleanup(func() { _ = c.Chaos().Delete(context.Background(), exp.ID) })
	if !strings.Contains(strings.ToLower(string(exp.Status)), "approval") {
		t.Fatalf("gated experiment must start pending_approval, got %q", exp.Status)
	}

	if err := c.Chaos().Approve(ctx, exp.ID, "dogfood approve"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	after, err := c.Chaos().Get(ctx, exp.ID)
	if err != nil {
		t.Fatalf("Get after approve: %v", err)
	}
	if after.ApprovedBy == "" {
		t.Fatalf("approval trail not recorded on the experiment: %+v", after)
	}
	t.Logf("approval flow OK: approvedBy=%s", after.ApprovedBy)
}
