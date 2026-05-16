// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mockarty/mockarty-go/fuzz"
)

// Example_basic shows the full DSL + Submit flow against an in-process
// admin stub.  In production code, the httptest.Server is replaced by
// the real Mockarty admin URL.
func Example_basic() {
	// Stand up a stub admin server so the godoc example is self-
	// contained and runnable.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/fuzzing/run", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "job-x", "status": "running"})
	})
	mux.HandleFunc("/api/v1/fuzzing/results/job-x", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(fuzz.Result{ID: "job-x", Status: "completed", TotalRequests: 99})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Describe the campaign in idiomatic Go.
	target := fuzz.NewTarget("login-stress",
		fuzz.WithDescription("Stress-test the login endpoint"),
		fuzz.WithHTTPEndpoint("POST", "https://api.example.com", "/api/v1/login"),
		fuzz.WithSeedCorpus(
			fuzz.Seed("valid", `{"username":"admin","password":"secret"}`),
			fuzz.Seed("missing-pw", `{"username":"admin"}`),
		),
		fuzz.WithMutator(fuzz.MutatorJSON),
		fuzz.WithDuration(5*time.Minute),
		fuzz.WithStopOnFinding(true),
		fuzz.WithReporter(fuzz.ReporterAllure),
		fuzz.WithAssertion(fuzz.AssertNoCrash()),
		fuzz.WithAssertion(fuzz.AssertResponseTimeUnder(5*time.Second)),
	)

	// 2. Submit to the admin server.
	runner := fuzz.NewRunner(srv.URL, "default", "tok-abc",
		fuzz.WithRunnerPollPeriod(5*time.Millisecond),
	)
	job, err := runner.Submit(context.Background(), target)
	if err != nil {
		fmt.Println("submit:", err)
		return
	}

	// 3. Wait for completion.
	res, err := runner.Wait(context.Background(), job.ID)
	if err != nil {
		fmt.Println("wait:", err)
		return
	}
	fmt.Printf("job %s finished with status=%s total=%d\n", res.ID, res.Status, res.TotalRequests)
	// Output: job job-x finished with status=completed total=99
}

// Example_offlineWriteTo shows the "write JSON then run via CLI" path
// — useful for committing fuzz targets to a repo and running them from
// CI without an admin server.
func Example_offlineWriteTo() {
	target := fuzz.NewTarget("api-fuzz",
		fuzz.WithHTTPEndpoint("GET", "https://api.example.com", "/api/v1/users"),
		fuzz.WithMutator(fuzz.MutatorURL),
		fuzz.WithMutator(fuzz.MutatorString),
		fuzz.WithAssertion(fuzz.AssertStatusClass(2)),
	)
	// Render to bytes so this example can stay free of disk IO.
	data, err := target.ToJSON()
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	// The JSON is what `mockarty-cli fuzz run target.json` consumes.
	fmt.Println("bytes >", len(data) > 0)
	// Output: bytes > true
}
