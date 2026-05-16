// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package allure provides annotations and an allure-results writer that is
// byte-compatible with the Allure 2 report format consumed by `allure
// generate`.
//
// The package is the Go side of Mockarty's three-SDK Allure parity strategy
// (see docs/research/SDK_FRAMEWORK_PLAN.md §3.3). It mirrors the surface
// already shipped in the Python (`mockarty.allure`) and Java
// (`io.qameta.allure.*` compatible) SDKs so the same Mockarty docs/examples
// translate verbatim across languages.
//
// # Design constraints
//
// SDK is a THIN layer — this package only collects metadata and emits JSON
// files to disk. It never:
//
//   - Talks to a Mockarty admin server.
//   - Embeds a JS runtime or runs a mock server.
//   - Executes test logic.
//
// Anything heavier lives in the CLI binary or the admin server. This package
// is safe to import from any unit-test suite without a network or daemon.
//
// # Usage
//
// The typical entry point is [T], which wraps a [*testing.T]:
//
//	func TestLogin(t *testing.T) {
//	    a := allure.T(t,
//	        allure.WithSuite("auth"),
//	        allure.WithFeature("OAuth"),
//	    )
//	    a.Severity(allure.SeverityCritical)
//	    a.Issue("JIRA-123", "https://jira/JIRA-123")
//
//	    a.Step("submit form", func() {
//	        // ...do work...
//	        a.Attachment("response.json", []byte(`{"ok":true}`), "application/json")
//	    })
//	}
//
// Results are flushed to `./allure-results/<uuid>-result.json` on test
// completion (registered via t.Cleanup). The directory can be overridden via
// the `ALLURE_RESULTS_DIR` env variable (matches the Python/Java SDK
// defaults) or per-test via [WithResultsDir].
//
// # Context-based API
//
// For tests that propagate a context (table-driven, async workers, ...),
// package-level functions accept a context.Context returned by [WithTest]:
//
//	ctx, finish := allure.WithTest(context.Background(), "login")
//	defer finish()
//	allure.Step(ctx, "submit", func() { ... })
//
// Both APIs share the same writer; mixing them within one test is supported.
//
// # Lifecycle hooks
//
// BeforeAll / AfterAll / BeforeEach / AfterEach reproduce TestNG /
// JUnit5 / pytest setup-teardown semantics on top of Go's bare `testing`
// package. Hooks emit Allure Container files so the report renderer
// shows them as Set up / Tear down panels.
//
//	func TestThings(t *testing.T) {
//	    allure.BeforeAll(t, "db", func(t *testing.T) { setupDB(t) })
//	    allure.AfterAll(t, "db", func(t *testing.T) { teardownDB(t) })
//	    allure.BeforeEach(t, "txn", func(t *testing.T) { ... })
//	    for _, name := range []string{"one","two"} {
//	        t.Run(name, func(tt *testing.T) {
//	            allure.RunWithHooks(tt, func(inner *testing.T) {
//	                a := allure.T(inner)
//	                a.Step("body", func() {})
//	            })
//	        })
//	    }
//	}
//
// # Parameterised tests
//
// ParameterizedTest reflects over a struct payload and turns each
// exported field into an Allure parameter. Each iteration produces a
// distinct history-id so the Allure tree clusters runs of the same case
// together while keeping different cases separated.
//
//	type Tc struct{ Input, Want string }
//	cases := []allure.ParameterCase[Tc]{
//	    {Name: "happy", Payload: Tc{"a","b"}},
//	    {Name: "empty", Payload: Tc{"",""}},
//	}
//	allure.ParameterizedTest(t, cases, func(tt *testing.T, c Tc) {
//	    a := allure.T(tt)
//	    a.Step("act", func() { ... })
//	})
//
// # TestMain
//
// An optional convenience entry point that writes executor.json on start
// and flushes leftover containers on exit. Drop it next to your tests:
//
//	func TestMain(m *testing.M) {
//	    os.Exit(allure.TestMain(m))
//	}
//
// # External-runs bridge
//
// Once your tests have produced an allure-results directory, push it to
// Mockarty TCM via externalruns.FromAllureDir + Client.CreateRun:
//
//	runs, _ := externalruns.FromAllureDir("./allure-results")
//	for _, r := range runs {
//	    run, _ := cli.CreateRun(ctx, r.Request)
//	    _ = cli.AddSteps(ctx, run.ID, r.Steps)
//	    _ = cli.FinishRun(ctx, run.ID, r.FinishRequest)
//	}
package allure
