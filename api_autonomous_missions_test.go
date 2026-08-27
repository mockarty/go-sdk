// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAutonomousMissionsSubmitAndReadFlow(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/autotester/intents":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			budget, _ := body["budget"].(map[string]any)
			if body["goal"] != "verify checkout" || body["productUrl"] != "https://shop.example" || budget["tokens_total"] != float64(12000) {
				t.Fatalf("submit body = %#v", body)
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"missionId": "m-1", "status": "accepted"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/autotester/missions":
			if r.URL.Query().Get("status") != "active" || r.URL.Query().Get("limit") != "25" {
				t.Fatalf("list query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"missions": []map[string]any{{"id": "m-1", "status": "active", "goal": "verify checkout", "budget": map[string]any{}}}, "total": 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/autotester/missions/m-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "m-1", "status": "active", "goal": "verify checkout", "budget": map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/autotester/missions/m-1/flow":
			_ = json.NewEncoder(w).Encode(map[string]any{"mission": map[string]any{"id": "m-1", "status": "done", "goal": "verify checkout", "budget": map[string]any{}}, "steps": []map[string]any{}, "artifacts": []map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	api := NewClient(srv.URL, WithAPIKey("mk_writer"), WithNamespace("team-a")).AutonomousMissions()
	ctx := context.Background()
	accepted, err := api.Submit(ctx, AutonomousMissionSubmitRequest{
		Goal:       " verify checkout ",
		ProductURL: "https://shop.example",
		Autonomy:   "auto",
		Budget:     AutonomousMissionBudgetHint{TokensTotal: 12000},
	})
	if err != nil || accepted.MissionID != "m-1" {
		t.Fatalf("submit = %+v err=%v", accepted, err)
	}
	if page, err := api.List(ctx, "active", 25); err != nil || page.Total != 1 || len(page.Missions) != 1 {
		t.Fatalf("list = %+v err=%v", page, err)
	}
	if mission, err := api.Get(ctx, "m-1"); err != nil || mission.ID != "m-1" {
		t.Fatalf("get = %+v err=%v", mission, err)
	}
	if flow, err := api.GetFlow(ctx, "m-1"); err != nil || flow.Mission.Status != "done" {
		t.Fatalf("flow = %+v err=%v", flow, err)
	}
	if calls != 4 {
		t.Fatalf("calls=%d, want 4", calls)
	}
}

func TestAutonomousMissionsValidateBeforeNetwork(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"missionId":"unexpected","status":"accepted"}`))
	}))
	defer srv.Close()
	api := NewClient(srv.URL).AutonomousMissions()
	ctx := context.Background()
	if _, err := api.Submit(ctx, AutonomousMissionSubmitRequest{}); err == nil {
		t.Fatal("empty goal accepted")
	}
	if _, err := api.Submit(ctx, AutonomousMissionSubmitRequest{Goal: "x", Autonomy: "root"}); err == nil {
		t.Fatal("unknown autonomy accepted")
	}
	for _, budget := range []AutonomousMissionBudgetHint{
		{TokensTotal: -1},
		{TokensPerDay: -1},
		{USDCap: -1},
		{USDCap: math.NaN()},
		{USDCap: math.Inf(1)},
		{USDCap: math.Inf(-1)},
	} {
		if _, err := api.Submit(ctx, AutonomousMissionSubmitRequest{Goal: "x", Budget: budget}); err == nil {
			t.Fatalf("invalid budget accepted: %+v", budget)
		}
	}
	if _, err := api.Get(ctx, " "); err == nil {
		t.Fatal("empty mission id accepted")
	}
	if calls != 0 {
		t.Fatalf("validation issued %d network calls", calls)
	}
}

func TestAutonomousMissionsEffectiveSettingsAndStart(t *testing.T) {
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/missions/settings/effective":
			if r.URL.Query().Get("productId") != "product/checkout" || r.URL.Query().Get("runWindowMinutes") != "90" {
				t.Fatalf("effective settings query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"namespace": "team-a", "productId": "product/checkout", "settingsDigest": digest, "count": 1,
				"settings": []map[string]any{{"key": "mission_run_window_minutes", "value": "90", "layer": "mission", "builtin": "480", "runtimeApplied": true}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/missions":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["goal"] != "ship checkout" || body["productId"] != "product/checkout" || body["expectedSettingsDigest"] != digest {
				t.Fatalf("start body = %#v", body)
			}
			if _, present := body["kind"]; present {
				t.Fatalf("goal-first start sent executor kind: %#v", body)
			}
			if _, present := body["chain"]; present {
				t.Fatalf("goal-first start sent executor chain: %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": true,
				"mission": map[string]any{"id": "m-unified", "namespace": "team-a", "productId": "product/checkout", "kind": "testing", "goal": "ship checkout", "origin": "ui", "status": "queued", "chain": []map[string]any{}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/missions/m-unified/cancel":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["reason"] != "release withdrawn" || body["idempotencyKey"] != "cancel-1" {
				t.Fatalf("cancel body = %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"mission": map[string]any{"id": "m-unified", "namespace": "team-a", "kind": "testing", "goal": "ship checkout", "origin": "ui", "status": "canceled", "chain": []map[string]any{}},
				"control": map[string]any{"id": "control-1", "missionId": "m-unified", "idempotencyKey": "cancel-1", "action": "cancel", "phase": "committed", "outcome": "applied", "reason": "release withdrawn", "createdAt": "2026-08-27T00:00:00Z", "updatedAt": "2026-08-27T00:00:01Z"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	api := NewClient(srv.URL, WithAPIKey("mk_writer"), WithNamespace("team-a")).AutonomousMissions()
	ctx := context.Background()
	settings, err := api.GetEffectiveSettings(ctx, MissionEffectiveSettingsOptions{ProductID: "product/checkout", RunWindowMinutes: 90})
	if err != nil || settings.SettingsDigest != digest || len(settings.Settings) != 1 || !settings.Settings[0].RuntimeApplied {
		t.Fatalf("effective settings = %+v err=%v", settings, err)
	}
	started, err := api.Start(ctx, MissionStartRequest{Goal: " ship checkout ", ProductID: "product/checkout", ExpectedSettingsDigest: digest})
	if err != nil || !started.Created || started.Mission.ID != "m-unified" {
		t.Fatalf("start = %+v err=%v", started, err)
	}
	if started.Mission.Chain == nil {
		t.Fatal("start response chain must be non-nil")
	}
	cancelled, err := api.Cancel(ctx, started.Mission.ID, MissionCancelRequest{
		Reason: " release withdrawn ", IdempotencyKey: " cancel-1 ",
	})
	if err != nil || cancelled.Control.Reason != "release withdrawn" ||
		cancelled.Control.IdempotencyKey != "cancel-1" || cancelled.Mission.Status != "canceled" {
		t.Fatalf("cancel = %+v err=%v", cancelled, err)
	}
	if calls != 3 {
		t.Fatalf("calls=%d, want 3", calls)
	}
}

func TestAutonomousMissionsUnifiedValidationBeforeNetwork(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	api := NewClient(srv.URL).AutonomousMissions()
	ctx := context.Background()
	for _, window := range []int{-1, 20161} {
		if _, err := api.GetEffectiveSettings(ctx, MissionEffectiveSettingsOptions{RunWindowMinutes: window}); err == nil {
			t.Fatalf("invalid run window %d accepted", window)
		}
	}
	for _, req := range []MissionStartRequest{
		{},
		{Goal: "x", ExpectedSettingsDigest: "sha256:bad"},
		{Goal: "x", BudgetTokensTotal: -1},
		{Goal: "x", BudgetTokensPerDay: -1},
		{Goal: "x", BudgetUSDCap: math.NaN()},
	} {
		if _, err := api.Start(ctx, req); err == nil {
			t.Fatalf("invalid unified start accepted: %+v", req)
		}
	}
	if _, err := api.Cancel(ctx, " ", MissionCancelRequest{}); err == nil {
		t.Fatal("empty cancel mission id accepted")
	}
	if calls != 0 {
		t.Fatalf("validation issued %d network calls", calls)
	}
}
