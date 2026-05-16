package externalruns

// SchemaVersion is the wire-format version this client speaks. It is sent
// on every request via the X-Mockarty-Schema-Version header and is
// replicated byte-for-byte across the Go, Python (mockarty/external_runs.py)
// and Java (ru.mockarty.api.ExternalRunsApi) SDKs.
//
// History:
//
//	1 — initial release (run + steps + attachments + finish).
//
// Bumping this constant is a coordinated change: the server gains the
// ability to translate the new envelope, all three SDKs ship the bump in
// lockstep, and a server still answering an old version returns a 426
// Upgrade Required so old clients fail fast.
const SchemaVersion = 1

// SchemaVersionHeader is the canonical header name. Exposed as a constant
// so test fixtures and middleware avoid drift.
const SchemaVersionHeader = "X-Mockarty-Schema-Version"

// AuthHeader is the canonical header used to send the API token.
const AuthHeader = "X-API-Key"
