// Package externalruns is a thin REST client for Mockarty's TCM external-runs
// endpoint. External-runs let a CI pipeline (or any out-of-Mockarty test
// runner) register a synthetic test run with the Mockarty admin server,
// attach step results and artefacts, and close the run so it surfaces in
// the user's Test Plans / TCM view.
//
// The package is intentionally minimal: it marshals JSON, performs HTTP,
// and returns parsed responses. It does NOT execute tests, run scenarios,
// or interpret reports beyond shape validation. Anything richer (Allure
// adapters, fluent step builders, framework annotations) lives in higher
// layers that compose this client.
//
// # Canonical contract
//
// The Mockarty admin server exposes the following endpoints under the
// per-namespace prefix /api/v1/namespaces/:ns/tcm/external-runs:
//
//	POST   /                        — create a run, returns Run
//	GET    /:run_id                 — fetch a single run
//	POST   /:run_id/steps           — append steps (idempotent on step_key)
//	POST   /:run_id/attachments     — multipart artefact upload
//	POST   /:run_id/finish          — close the run with a final status
//	GET    /                        — list runs (filterable)
//
// Every request carries an X-Mockarty-Schema-Version header so the server
// can refuse old clients after a backwards-incompatible envelope change.
// The constant SchemaVersion below is replicated in the Python and Java
// SDKs — bumping it requires updating all three call-sites at once.
//
// # Auth
//
// The Client sends the API token via the "X-API-Key" header, matching the
// admin server's token middleware (see internal/auth/api_token.go).
//
// # Concurrency
//
// A *Client is safe for concurrent use by multiple goroutines. The
// underlying *http.Client is reused, so connection pooling is preserved.
package externalruns
