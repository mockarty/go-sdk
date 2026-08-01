// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: kitchen_sink — end-to-end Mockarty Tester DSL showcase.
//
// One executable that exercises every major Tester facet plus the
// reporting / upstream-tracker side-channels you'd want in a real CI
// pipeline:
//
//   1. HTTP   — POST /login → extract token from JSON response
//   2. HTTP   — GET  /me  using {{token}} interpolation
//   3. GraphQL — query getMe with the same token
//   4. HTTP   — assert all flavours of response (status, header,
//               body_contains, JSONPath, array length)
//   5. Wrap   — group everything as one Allure step
//   6. Jira   — auto-create a bug ticket if the run fails (mock
//               testbackend Jira endpoint)
//   7. GitLab — trigger a pipeline + poll status (mock testbackend
//               GitLab endpoint)
//   8. ExternalRun — upload the aggregated result to Mockarty TCM
//   9. Allure — write *-result.json files for offline aggregators
//
// Run against a co-deployed testbackend + Mockarty admin:
//
//   # 1. Start testbackend
//   mockarty-testbackend &
//
//   # 2. Start Mockarty admin (or use your shared one)
//   mockarty &
//
//   # 3. Run the example
//   TESTBACKEND_URL=http://127.0.0.1:18770 \
//   MOCKARTY_URL=http://127.0.0.1:5770 \
//   MOCKARTY_API_KEY=... \
//   MOCKARTY_NAMESPACE=sandbox \
//   ALLURE_RESULTS_DIR=./allure-results \
//   go run .
//
//   # 4. (optional) Upload the Allure results via the CLI
//   mockarty-cli allure upload --dir ./allure-results --case-prefix kitchen-sink
//
// The example is intentionally self-contained: no helpers, every
// step is visible in main() so newcomers can copy the chain and
// drop in their own URLs.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
	"github.com/mockarty/mockarty-go/tester"
)

func main() {
	cfg := loadConfig()

	// Build the Tester. WithAllureWriter wires the SDK's native Allure
	// emitter so each .Finish() flushes one *-result.json per chain
	// into cfg.AllureDir.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t := tester.New(
		tester.WithBaseURL(cfg.TestBackendURL),
		tester.WithContext(ctx),
	)

	// 1+2. Issue token → reuse it on a validate call. Uses testbackend's
	// /api/v1/token-chain/{issue,validate} pair which exists precisely
	// for this demonstration (deterministic token "tok-abc123-…").
	//
	// Wrap groups the chain so the Allure report renders a "Token flow"
	// parent step containing both child requests.
	t.Wrap("token issue + authorised validate", func() {
		t.HTTP().GET("/api/v1/token-chain/issue").
			ExpectStatus(200).
			ExpectJSONPath("$.token", "tok-abc123-deterministic").
			Extract("$.token", "token")

		t.HTTP().POST("/api/v1/token-chain/validate").
			Header("Authorization", "Bearer {{token}}").
			JSON(map[string]string{"action": "ping"}).
			ExpectStatus(200).
			ExpectJSONPath("$.authorization", "Bearer tok-abc123-deterministic")
	})

	// 3. GraphQL chain — testbackend exposes a `user(id:)` resolver
	// seeded with users user-1 (admin) … user-5. GraphQL().Query(...)
	// returns a GraphQLStep with its own Expect* helpers.
	t.GraphQL(cfg.TestBackendURL+"/graphql").
		Query(`query GetUser($id: ID!) { user(id: $id) { name email } }`, map[string]any{"id": "user-1"}).
		Header("Authorization", "Bearer {{token}}").
		ExpectStatus(200).
		ExpectNoErrors().
		ExpectField("$.data.user.name", "Admin User")

	// 4. Assertion variety on a single endpoint — every Expect* kind.
	// /api/v1/users is the testbackend canonical endpoint; envelope
	// is `{items: [{id, name, email, …}, …]}`.
	t.HTTP().GET("/api/v1/users").
		ExpectStatus(200).
		ExpectHeader("Content-Type", "application/json; charset=utf-8").
		ExpectBodyContains("Admin User").
		ExpectJSONPath("$.items[0].name", "Admin User")

	// 5. Finish drains the chain + writes Allure + records timings.
	t.Finish()

	// 6+7. Upstream tracker side-channels — fire only when the run
	// actually failed. In a real CI you'd gate these behind your own
	// CI variables (this branch / nightly / etc.).
	if !t.OK() {
		errs := errorsToStrings(t.Errors())
		log.Printf("kitchen-sink: %d failed step(s); filing tracker artefacts", len(errs))
		fileJiraTicket(cfg, errs)
		_ = triggerGitLabPipeline(cfg) // best-effort
	}

	// 8. Mockarty TCM upload via ExternalRun. Encodes the run shape
	// camel-case (schemaVersion=1) so the admin's /tcm/external-runs
	// receiver decodes it without translation.
	if cfg.MockartyURL != "" && cfg.APIKey != "" {
		uploadExternalRun(cfg, t)
	}

	// 9. Exit code = run status. CI shells with `set -e` stop here.
	if !t.OK() {
		os.Exit(1)
	}
	fmt.Println("kitchen-sink: ok")
}

// ── helpers ──────────────────────────────────────────────────────────

type config struct {
	TestBackendURL string
	MockartyURL    string
	APIKey         string
	Namespace      string
	AllureDir      string
	JiraProjectKey string
	GitLabProject  string
}

func loadConfig() config {
	return config{
		TestBackendURL: env("TESTBACKEND_URL", "http://127.0.0.1:18770"),
		MockartyURL:    env("MOCKARTY_URL", ""),
		APIKey:         env("MOCKARTY_API_KEY", ""),
		Namespace:      env("MOCKARTY_NAMESPACE", "sandbox"),
		AllureDir:      env("ALLURE_RESULTS_DIR", "./allure-results"),
		JiraProjectKey: env("JIRA_PROJECT_KEY", "QA"),
		GitLabProject:  env("GITLAB_PROJECT_ID", "1"),
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// fileJiraTicket posts a Bug to the testbackend Jira mock. In
// production you'd swap the URL for your real instance and drop the
// auth header from your team's secret store.
func fileJiraTicket(cfg config, errs []string) {
	body, _ := json.Marshal(map[string]any{
		"fields": map[string]any{
			"project":   map[string]string{"key": cfg.JiraProjectKey},
			"summary":   "kitchen-sink run failed: " + truncate(strings.Join(errs, "; "), 80),
			"issuetype": map[string]string{"name": "Bug"},
		},
	})
	resp, err := http.Post(cfg.TestBackendURL+"/rest/api/2/issue",
		"application/json", strings.NewReader(string(body)))
	if err != nil {
		log.Printf("jira create: %v", err)
		return
	}
	defer resp.Body.Close()
	var out struct{ Key string }
	_ = json.NewDecoder(resp.Body).Decode(&out)
	log.Printf("jira: filed %s for %d failure(s)", out.Key, len(errs))
}

// triggerGitLabPipeline kicks the GitLab mock's pipeline endpoint
// and polls until completion. State machine on the mock side advances
// pending → running → success on each fetch.
func triggerGitLabPipeline(cfg config) error {
	q := url.Values{}
	q.Set("ref", "main")
	resp, err := http.Post(
		cfg.TestBackendURL+"/api/v4/projects/"+cfg.GitLabProject+"/trigger/pipeline?"+q.Encode(),
		"application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var p struct {
		ID     int    `json:"id"`
		Status string `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&p)
	for i := 0; i < 5 && p.Status != "success" && p.Status != "failed"; i++ {
		time.Sleep(100 * time.Millisecond)
		r, err := http.Get(fmt.Sprintf("%s/api/v4/projects/%s/pipelines/%d",
			cfg.TestBackendURL, cfg.GitLabProject, p.ID))
		if err != nil {
			return err
		}
		_ = json.NewDecoder(r.Body).Decode(&p)
		r.Body.Close()
	}
	log.Printf("gitlab pipeline #%d → %s", p.ID, p.Status)
	return nil
}

// uploadExternalRun ships the Tester result to Mockarty TCM. Mirrors
// `mockarty-cli allure upload`'s wire shape so a flow could equally
// drop result.json to disk and let the CLI handle the POST.
func uploadExternalRun(cfg config, t *tester.Tester) {
	client := mockarty.NewClient(cfg.MockartyURL,
		mockarty.WithAPIKey(cfg.APIKey),
		mockarty.WithNamespace(cfg.Namespace),
	)
	run := t.ToExternalRun(tester.ExternalRunOptions{
		CaseName:   "kitchen-sink",
		Framework:  "mockarty-go-tester",
		AutoCreate: true,
		FullName:   "github.com/mockarty/mockarty-go/examples/kitchen_sink",
	})
	resp, err := client.ExternalRuns().Report(context.Background(), cfg.Namespace, run)
	if err != nil {
		log.Printf("mockarty TCM upload: %v", err)
		return
	}
	log.Printf("mockarty TCM: %s (run=%s case=%s)", resp.Status, resp.RunID, resp.CaseID)
}

// errorsToStrings flattens []error → []string for tracker payload
// construction. Tester records assertions as errors so the test
// runner can wrap them with `errors.Is` / `errors.As` machinery.
func errorsToStrings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
