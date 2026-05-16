// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import "time"

// JobID identifies a submitted campaign on the admin server.
type JobID struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
}

// Result is the campaign outcome returned by Runner.Wait / Runner.LocalSpawn.
// Fields project from the server-side fuzzing.FuzzResult plus the
// per-finding chain.
type Result struct {
	StartedAt        time.Time `json:"startedAt"`
	CompletedAt      time.Time `json:"completedAt,omitempty"`
	ID               string    `json:"id"`
	ConfigID         string    `json:"configId,omitempty"`
	Namespace        string    `json:"namespace,omitempty"`
	Status           string    `json:"status"`
	Strategy         string    `json:"strategy,omitempty"`
	Findings         []Finding `json:"findings,omitempty"`
	TotalRequests    int64     `json:"totalRequests"`
	DurationMs       int64     `json:"durationMs"`
	TotalFindings    int       `json:"totalFindings"`
	CriticalFindings int       `json:"criticalFindings"`
	HighFindings     int       `json:"highFindings"`
	MediumFindings   int       `json:"mediumFindings"`
	LowFindings      int       `json:"lowFindings"`
	InfoFindings     int       `json:"infoFindings"`
}

// Finding is one issue surfaced by the engine.  The ReproSeed is the
// exact seed (with mutator applied) the user can re-feed to the engine
// to deterministically reproduce the failure.
type Finding struct {
	CreatedAt       time.Time `json:"createdAt"`
	ID              string    `json:"id"`
	RunID           string    `json:"runId,omitempty"`
	Severity        string    `json:"severity"`
	Category        string    `json:"category"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	RequestMethod   string    `json:"requestMethod,omitempty"`
	RequestURL      string    `json:"requestUrl,omitempty"`
	RequestBody     string    `json:"requestBody,omitempty"`
	ResponseBody    string    `json:"responseBody,omitempty"`
	MutationApplied string    `json:"mutationApplied,omitempty"`
	ReproSeed       string    `json:"reproSeed,omitempty"`
	ResponseStatus  int       `json:"responseStatus,omitempty"`
}

// Event is one streamed update during a running campaign.  The runner
// translates the server's SSE / WS protocol into a uniform Event stream.
type Event struct {
	Timestamp         time.Time `json:"timestamp"`
	Finding           *Finding  `json:"finding,omitempty"`
	Kind              string    `json:"kind"`
	Message           string    `json:"message,omitempty"`
	TotalRequests     int64     `json:"totalRequests,omitempty"`
	CompletedRequests int64     `json:"completedRequests,omitempty"`
	RequestsPerSecond float64   `json:"requestsPerSecond,omitempty"`
}
