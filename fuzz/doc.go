// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package fuzz is a pure-Go DSL that describes a Mockarty fuzz target in
// idiomatic Go and transpiles it to the canonical JSON config consumed by
// the Mockarty admin server's POST /api/v1/fuzzing/run endpoint and by the
// mockarty-cli `fuzz run` subcommand.
//
// The package is the Go side of Mockarty's Wave 3 SDK strategy (see
// docs/research/SDK_FRAMEWORK_PLAN.md §5.7a). It is intentionally THIN:
//
//   - It DOES NOT embed a fuzz engine, mutator, or detector. Mockarty's
//     existing fuzz runtime under internal/fuzzing/ runs the actual
//     campaign, server-side or under the CLI.
//   - It DOES build a fluent description of a target (protocol, seeds,
//     mutators, coverage hints, assertions, stop conditions) and emit
//     a canonical JSON config the runtime understands.
//   - It DOES offer two execution paths over the same DSL: Runner.Submit
//     posts to the admin server's REST API; Runner.LocalSpawn writes the
//     JSON to a temp file and invokes mockarty-cli as a subprocess.
//
// # Universality (Owner Q11)
//
//   - In-code: build a target via the fluent DSL, call Submit / LocalSpawn.
//   - On disk: build a target, call target.WriteTo("target.json"), check
//     it into the repo, run `mockarty-cli fuzz run target.json` from CI.
//   - In the UI / via REST: the admin server accepts the same JSON shape.
//
// All three paths converge on the canonical fuzz config — the SDK is just
// a Go-flavoured front door to that schema.
//
// # Quick start
//
//	target := fuzz.NewTarget("login-stress",
//	    fuzz.WithDescription("Stress-test the login endpoint"),
//	    fuzz.WithHTTPEndpoint("POST", "https://api.example.com", "/api/v1/login"),
//	    fuzz.WithSeedCorpus(
//	        fuzz.Seed("valid", `{"username":"admin","password":"secret"}`),
//	        fuzz.Seed("missing-pw", `{"username":"admin"}`),
//	    ),
//	    fuzz.WithMutator(fuzz.MutatorJSON),
//	    fuzz.WithDuration(5*time.Minute),
//	    fuzz.WithStopOnFinding(true),
//	    fuzz.WithReporter(fuzz.ReporterAllure),
//	    fuzz.WithAssertion(fuzz.AssertNoCrash()),
//	    fuzz.WithAssertion(fuzz.AssertResponseTimeUnder(5*time.Second)),
//	)
//
//	// Path A — submit to a running admin server.
//	r := fuzz.NewRunner("https://mockarty.example.com", "default", "tok-abc")
//	job, err := r.Submit(ctx, target)
//	if err != nil { /* ... */ }
//	res, err := r.Wait(ctx, job.ID)
//
//	// Path B — write JSON to disk for `mockarty-cli fuzz run`.
//	_ = target.WriteTo("login-stress.json")
//
//	// Path C — local CLI subprocess for offline iteration.
//	res, err := r.LocalSpawn(ctx, target)
//
// # Schema parity
//
// The emitted JSON matches the FuzzingConfig type that the existing SDK
// FuzzingAPI client already accepts (api_fuzzing.go) and that the server
// stores in fuzz_configs (internal/fuzzing/config.go FuzzConfig). The DSL
// surfaces the fields most commonly tuned from code and falls back to
// server-side defaults for the rest.
//
// # Phase 2 (forthcoming)
//
//   - Custom JavaScript mutators registered via WithCustomMutator.
//   - Distributed fuzz fan-out across runner pools (target → multiple
//     runners with sharded seed corpora).
//   - Dictionary-based mutators (AFL-style keyword corpora).
//   - Streaming Allure step emission via the allure/ sibling package.
package fuzz
