// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: load_test — describe a load test with the LoadTest builder DSL and
// emit a perf-config that `mockarty-cli perf run --from-config` runs locally.
//
// The builder is a thin wrapper around the existing perf engine; it does not
// run anything itself. It produces either a k6-compatible script or a
// perf-config JSON carrying the full load profile (stages/thresholds/env).
package main

import (
	"fmt"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	profile := mockarty.NewLoadTest("checkout-load").
		Target("http://127.0.0.1:8080").
		Get("/health").
		Post("/cart", map[string]any{"sku": "abc", "qty": 2}).
		Stages(
			mockarty.Stage("30s", 50), // ramp to 50 VUs
			mockarty.Stage("1m", 50),  // hold 50 VUs
			mockarty.Stage("10s", 0),  // ramp down
		).
		Threshold("http_req_duration", "p(95)<800").
		Threshold("http_req_failed", "rate<0.01").
		ThinkTime(0.5)

	// 1) Inspect the generated k6 script.
	fmt.Println("--- k6 script ---")
	fmt.Println(profile.ToK6Script())

	// 2) Save a perf-config and run it locally with the CLI:
	//      mockarty-cli perf run --from-config checkout.json
	if err := profile.SaveConfig("checkout.json"); err != nil {
		panic(err)
	}
	fmt.Println("wrote checkout.json — run it with:")
	fmt.Println("  mockarty-cli perf run --from-config checkout.json")

	// 3) (Optional) submit the same config to a Mockarty server via the SDK:
	// Saved configurations use a typed Options envelope. It keeps run controls
	// (including an opt-in metrics sink) together when the profile is reused.
	saved := &mockarty.PerfConfig{
		Name:   "checkout soak",
		Script: profile.ToK6Script(),
		Options: &mockarty.PerfOptions{
			Stages: []mockarty.PerfStage{
				{Duration: "30s", Target: 50},
				{Duration: "1m", Target: 50},
				{Duration: "10s", Target: 0},
			},
			MetricsPush:         []string{"prometheus:https://metrics.example.test/push"},
			MetricsPushInterval: "10s",
		},
	}
	_ = saved // client.Perf().CreateConfig(ctx, saved)
}
