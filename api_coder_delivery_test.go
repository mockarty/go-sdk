package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoderDeliveryCanonicalRoutesAndApproval(t *testing.T) {
	var paths []string
	var putConfig CoderDeliveryConfig
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/coder/delivery-config" {
			_ = json.NewDecoder(r.Body).Decode(&putConfig)
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/coder/missions/m1/approve" {
			var body map[string]bool
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !body["approve"] {
				t.Fatal("approval body dropped")
			}
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/coder/missions/m1/deploy-outcome" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["outcome"] != "not_applied" {
				t.Fatalf("reconciliation body=%v", body)
			}
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/coder/missions/m1/add" {
			var body CoderMissionAddRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Tasks) != 1 || len(body.Tasks[0].RequiredChecks) != 1 || body.Tasks[0].RequiredChecks[0].Args[0] != "go" {
				t.Fatalf("add body=%+v", body)
			}
		}
		if r.URL.Path == "/api/v1/observability/sources" {
			_, _ = w.Write([]byte(`{"contractVersion":"mockarty.observability-query/v1","sources":[]}`))
			return
		}
		if r.URL.Path == "/api/v1/observability/query" {
			var body CoderObservabilityQuery
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Source != "prometheus" || body.Correlation.MissionID != "m1" {
				t.Fatalf("observability body=%+v", body)
			}
			_, _ = w.Write([]byte(`{"contractVersion":"mockarty.observability-query/v1","result":{"evidenceDigest":"sha256:test"}}`))
			return
		}
		if r.URL.Path == "/api/v1/coder/delivery-config" {
			_, _ = w.Write([]byte(`{"namespace":"team-a","infraNotes":"keep","quality":"thorough","missionSettings":{"guard_policy":"warn"},"ci":{"system":"gitlab"},"registry":{"url":"registry.test"},"gitops":{"repoUrl":"https://git.test/ops"},"policy":{"approverNotify":"ops"},"intake":{"projectIds":["p1"]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"m1","missions":[],"approval":"approved"}`))
	}))
	defer server.Close()
	api := NewClient(server.URL, WithNamespace("team-a")).CoderDelivery()
	ctx := context.Background()
	config, _ := api.GetConfig(ctx, "product-1")
	_, _ = api.PutConfig(ctx, *config)
	_ = api.DeleteConfig(ctx, "product-1")
	_, _ = api.StartMission(ctx, CoderMissionStartRequest{Goal: "ship", RepoURL: "https://git.example/app.git"})
	_, _ = api.ListMissions(ctx)
	_, _ = api.GetMission(ctx, "m1")
	_, _ = api.ApproveMission(ctx, "m1", true)
	_, _ = api.AddToMission(ctx, "m1", CoderMissionAddRequest{Tasks: []CoderSubTask{{
		Prompt: "add a test", RequiredChecks: []CoderRequiredCheck{{Name: "unit", Args: []string{"go", "test", "./..."}}},
	}}})
	_, _ = api.ReconcileDeploy(ctx, "m1", CoderDeployNotApplied)
	_, _ = api.ObservabilitySources(ctx)
	_, _ = api.QueryObservability(ctx, CoderObservabilityQuery{Source: "prometheus", Expression: "up", Correlation: CoderObservabilityCorrelation{MissionID: "m1"}})
	if len(paths) != 11 {
		t.Fatalf("paths=%v", paths)
	}
	for _, path := range paths[len(paths)-2:] {
		if path != "GET /api/v1/observability/sources?namespace=team-a" && path != "POST /api/v1/observability/query?namespace=team-a" {
			t.Fatalf("observability request escaped namespace: %s", path)
		}
	}
	if putConfig.InfraNotes != "keep" || putConfig.Quality != "thorough" || putConfig.MissionSettings["guard_policy"] != "warn" || putConfig.CI.System != "gitlab" || putConfig.Registry.URL != "registry.test" || putConfig.GitOps.RepoURL == "" || putConfig.Policy.ApproverNotify != "ops" || len(putConfig.Intake.ProjectIDs) != 1 {
		t.Fatalf("full-replace delivery config lost fields: %+v", putConfig)
	}
}

func TestCoderDeliveryReconcileDeployRejectsImplicitOutcome(t *testing.T) {
	api := NewClient("http://127.0.0.1:1", WithNamespace("team-a")).CoderDelivery()
	if _, err := api.ReconcileDeploy(context.Background(), "m1", ""); err == nil {
		t.Fatal("empty deployment outcome accepted")
	}
	if _, err := api.QueryObservability(context.Background(), CoderObservabilityQuery{Source: "prometheus"}); err == nil {
		t.Fatal("observability query without expression accepted")
	}
}
