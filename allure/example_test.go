// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mockarty/mockarty-go/allure"
)

// TestLogin_AllureBasic mirrors the canonical Allure-pytest "happy path"
// example. After running:
//
//	go test ./allure/...
//
// you'll find one `<uuid>-result.json` and one attachment file in the
// directory passed to allure.WithResultsDir (here a per-test TempDir so the
// example does not litter the working tree).
//
// In real CI you'd set ALLURE_RESULTS_DIR=./allure-results once and drop
// the WithResultsDir option — the SDK falls back to that env variable.
func TestLogin_AllureBasic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	a := allure.T(t,
		allure.WithResultsDir(dir),
		allure.WithFeature("Auth"),
		allure.WithStory("Email + password login"),
		allure.WithSeverity(allure.SeverityCritical),
		allure.WithOwner("qa@example.com"),
	)
	a.Issue("JIRA-123", "https://jira.example.com/browse/JIRA-123")
	a.TmsLink("TC-7", "https://tms.example.com/cases/7")
	a.Description("End-to-end check that POST /login returns 200 with a JWT.")

	a.Step("submit credentials", func() {
		// In a real test this is your HTTP client call. We attach the
		// request/response payloads so Allure renders them next to the
		// step in the UI.
		a.AttachJSON("request.json", []byte(`{"email":"alice@example.com"}`))
		a.AttachJSON("response.json", []byte(`{"token":"jwt-xyz","ttl":3600}`))
	})

	a.Step("verify session", func() {
		a.Parameter("expected_role", "user")
		// StepErr lets you delegate failure reporting to the SDK by returning
		// an error from the closure — passed when nil, failed when not.
		_ = a.StepErr("session is active", func() error { return nil })
	})

	if t.Failed() {
		t.Errorf("expected example to pass — failing the example breaks the docs")
	}
}

// TestLogin_BadPassword shows how a failure surfaces in the produced
// result. The step that returns a non-nil error is marked `failed` and the
// aggregate result follows the strongest step status.
//
// We use t.Run so the SDK's t.Cleanup-driven flush happens before the
// surrounding test inspects state, and we swallow the failure on the inner
// test so the example exits with code 0 (the documented behaviour).
func TestLogin_BadPassword(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := allure.T(inner,
			allure.WithResultsDir(dir),
			allure.WithFeature("Auth"),
			allure.WithSeverity(allure.SeverityNormal),
		)
		a.Description("Wrong password is rejected with 401.")
		// We do NOT call inner.Fail here — the SDK records step status from the
		// StepErr return value and bubbles it into the result for us. We swallow
		// the error at the example level so the docs sample exits with code 0.
		_ = a.StepErr("submit bad creds", func() error {
			return errors.New("expected 401, got 500")
		})
	})
}
