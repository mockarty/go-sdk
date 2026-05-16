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
// # Phase 2 (forthcoming)
//
// A future release will add reflection-based discovery of a hypothetical
// `github.com/allure-framework/allure-go` import so existing Allure-Go test
// suites can swap a single import and keep working. Phase 1 (this file set)
// is the baseline writer + idiomatic Go annotation API.
package allure
