// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package testplanprobe is a fixture suite for the Allure test-plan
// end-to-end tests. It is a real, compilable go-test package whose tests go
// through allure.T, so the driver (allure/testplan_e2e_test.go) can compile
// it once and run the binary under different ALLURE_TESTPLAN_PATH values,
// asserting which tests executed AND what exit code the process produced.
//
// Run on its own (no plan configured) it is an ordinary green suite, so it
// costs nothing in the normal `go test ./...` sweep.
package testplanprobe

import (
	"os"
	"testing"

	"github.com/mockarty/mockarty-go/allure"
)

func TestMain(m *testing.M) {
	// Never write allure-results into the source tree.
	cleanup := func() {}
	if os.Getenv(allure.ResultsDirEnv) == "" {
		dir, err := os.MkdirTemp("", "allure-probe-")
		if err == nil {
			_ = os.Setenv(allure.ResultsDirEnv, dir)
			cleanup = func() { _ = os.RemoveAll(dir) }
		}
	}
	code := allure.TestMain(m)
	cleanup()
	os.Exit(code)
}

// TestAlpha is addressable by selector only.
func TestAlpha(t *testing.T) {
	allure.T(t)
}

// TestBeta carries a pinned Allure id, so a plan can select it by "id".
func TestBeta(t *testing.T) {
	allure.T(t, allure.WithAllureID("777"))
}

// TestGamma is a second selector-only test, used to prove the non-selected
// ones really are skipped.
func TestGamma(t *testing.T) {
	allure.T(t)
}

// TestDelta does not use allure.T at all — it enforces the plan through the
// standalone helper, proving suites that only want selection (no Allure
// reporting) are covered too.
func TestDelta(t *testing.T) {
	allure.SkipIfNotSelected(t)
}
