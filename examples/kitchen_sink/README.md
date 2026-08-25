# kitchen_sink — end-to-end Tester DSL showcase

One executable that exercises every major Mockarty Go SDK Tester facet
plus the reporting / upstream-tracker side-channels you'd want in a
real CI pipeline. Use this as the canonical reference when wiring
Mockarty into your own test harness.

## What it demonstrates

| Step | Facet | What it proves |
|------|-------|----------------|
| 1+2  | `t.HTTP()` | POST `/auth/login` → extract token → GET `/me` with `{{token}}` interpolation |
| 3    | `t.GraphQL()` | Query with the same token, assert field via JSONPath |
| 4    | `t.HTTP()` | Every `Expect*` kind on a single response (status, header, body_contains, json_path, json_array_len) |
| 5    | `t.Wrap()` | Group steps under one Allure parent for the report |
| 6    | Jira REST | Auto-file a Bug if any step failed (mock endpoint, no real Jira needed) |
| 7    | GitLab REST | Trigger pipeline → poll until `success` (mock endpoint) |
| 8    | `client.ExternalRuns().Report(...)` | Upload the run shape to Mockarty TCM as a synthetic case run |
| 9    | Exit code | Non-zero on failure → `set -e` friendly |

## Run it

```bash
# 1. Start the test backend (HTTP + GraphQL + Jira/GitLab mocks in-process)
mockarty-testbackend &     # exposes 18770 by default

# 2. (optional) Start Mockarty admin for the TCM upload step
mockarty &                  # exposes 5770

# 3. Run the example
TESTBACKEND_URL=http://127.0.0.1:18770 \
MOCKARTY_URL=http://127.0.0.1:5770 \
MOCKARTY_API_KEY=mk_... \
MOCKARTY_NAMESPACE=sandbox \
go run .

# 4. (optional) Upload Allure-style results via the CLI
mockarty-cli allure upload --dir ./allure-results --case-prefix kitchen-sink
```

Skipping `MOCKARTY_URL` makes the example a pure local demo (no TCM
upload). Skipping the testbackend makes most steps fail — which is
fine, you'll see the tracker fall-through path in action.

## Environment variables

| Var | Default | Purpose |
|-----|---------|---------|
| `TESTBACKEND_URL` | `http://127.0.0.1:18770` | Where the example points its HTTP / GraphQL / Jira / GitLab calls |
| `MOCKARTY_URL` | _empty_ | Mockarty admin URL — if set, the run uploads to `/tcm/external-runs` |
| `MOCKARTY_API_KEY` | _empty_ | Required when `MOCKARTY_URL` is set |
| `MOCKARTY_NAMESPACE` | `sandbox` | Target namespace for the upload |
| `ALLURE_RESULTS_DIR` | `./allure-results` | Where the Allure writer drops `*-result.json` |
| `JIRA_PROJECT_KEY` | `QA` | Project the failure ticket is filed under |
| `GITLAB_PROJECT_ID` | `1` | Project the pipeline is triggered against |

## What the testbackend exposes for this example

| Endpoint | Used by step |
|----------|-------------|
| `POST /auth/login` | step 1 |
| `GET  /me` | step 2 |
| `POST /graphql` | step 3 |
| `GET  /items` | step 4 |
| `POST /rest/api/2/issue` | step 6 (Jira mock) |
| `POST /api/v4/projects/{id}/trigger/pipeline` | step 7 (GitLab mock) |
| `GET  /api/v4/projects/{id}/pipelines/{pid}` | step 7 (poll) |

All of these live in `cmd/testbackend/handlers/` and are wired into
the testbackend binary. No external dependencies — works offline.

## Copy this for your own flow

The whole chain lives in `main()` — no helpers hidden behind a
framework. Replace the URLs, drop in your own `Expect*` set, and you
have a CI-friendly test runner that uploads to Mockarty TCM out of
the box.
