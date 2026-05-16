// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package externalruns_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
)

// ExampleClient_endToEnd walks the typical CI/CD lifecycle: create →
// stream steps → upload a report artefact → finish. The httptest server
// mimics the Mockarty admin endpoints so the example is self-contained
// and runs in `go test ./externalruns/...`.
func ExampleClient_endToEnd() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/external-runs"):
			_ = json.NewEncoder(w).Encode(externalruns.Run{
				ID:        "run-42",
				Namespace: "team-alpha",
				Status:    externalruns.StatusRunning,
				StartedAt: time.Now().UTC(),
				SchemaVer: externalruns.SchemaVersion,
			})
		case strings.HasSuffix(r.URL.Path, "/steps"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/attachments"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/finish"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/run-42"):
			_ = json.NewEncoder(w).Encode(externalruns.Run{
				ID:        "run-42",
				Namespace: "team-alpha",
				Status:    externalruns.StatusPassed,
				StepCount: 2,
				SchemaVer: externalruns.SchemaVersion,
			})
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := externalruns.NewClient(srv.URL, "team-alpha", "secret-api-token")
	if err != nil {
		fmt.Println("config error:", err)
		return
	}

	run, err := client.CreateRun(ctx, externalruns.CreateRunRequest{
		Name:        "nightly-smoke",
		Framework:   "go-test",
		Tags:        []string{"smoke", "nightly"},
		Environment: map[string]string{"ci": "github-actions"},
	})
	if err != nil {
		fmt.Println("create:", err)
		return
	}
	fmt.Printf("run created status=%s\n", run.Status)

	steps := []externalruns.Step{
		{StepKey: "login", Name: "Login flow", Status: externalruns.StatusPassed, DurationMS: 132},
		{StepKey: "checkout", Name: "Checkout flow", Status: externalruns.StatusPassed, DurationMS: 481},
	}
	if err := client.AddSteps(ctx, run.ID, steps); err != nil {
		fmt.Println("steps:", err)
		return
	}

	allureJSON := []byte(`{"format":"allure-results","tests":2}`)
	if err := client.AttachReport(ctx, run.ID, "allure-report.json", allureJSON, "application/json"); err != nil {
		fmt.Println("attach:", err)
		return
	}

	if err := client.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{
		Status:  externalruns.StatusPassed,
		Summary: "all green",
	}); err != nil {
		fmt.Println("finish:", err)
		return
	}

	final, err := client.GetRun(ctx, run.ID)
	if err != nil {
		fmt.Println("get:", err)
		return
	}
	fmt.Printf("finished status=%s step_count=%d\n", final.Status, final.StepCount)

	// Output:
	// run created status=running
	// finished status=passed step_count=2
}
