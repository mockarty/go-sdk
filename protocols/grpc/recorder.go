// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package grpc

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
)

// StepRecorder is the narrow seam through which the gRPC test client
// reports per-RPC steps. The default implementation (NewExternalRunsRecorder)
// ships steps into a mockarty-go externalruns.Client so the TCM external
// run shows a per-RPC timeline at the end of the test job. The interface
// is deliberately tiny so users can plug in their own sinks (logs, custom
// dashboards, allure-go step bridges) without dragging the externalruns
// dependency in.
//
// Implementations MUST be safe for concurrent use — the gRPC client can
// fire multiple RPCs in parallel from a single test, and v1 still wants
// every call to surface as a step.
type StepRecorder interface {
	// Record stores one completed step. Called once per InvokeJSON
	// after the RPC returns (or fails). The implementation should not
	// block — callers typically buffer + flush in the background.
	Record(ctx context.Context, step Step)
}

// Step is the protocol-agnostic record produced for each captured RPC.
// It maps 1:1 to externalruns.Step so the default adapter is a memcpy;
// keeping a separate type here means the recorder API doesn't leak the
// externalruns dependency to users who plug in their own sinks.
type Step struct {
	StartedAt time.Time
	// EndedAt is StartedAt + Duration; kept separately so callers
	// don't have to recompute it when fanning out to multiple sinks.
	EndedAt    time.Time
	Parameters map[string]string
	// Key is a stable identifier for the step. The default builder
	// uses "<method>#<seq>" so retries of the same method don't
	// collide on the server-side de-dup.
	Key string
	// Name is the human-readable label shown in the TCM run. The
	// default builder uses the full method name (e.g.
	// "acme.UserService/GetUser").
	Name string
	// Status maps to externalruns.Status — "passed" / "failed" /
	// "broken" / "skipped". Empty defaults to "passed".
	Status string
	// Message is the error message (or empty on success).
	Message string
	// Duration is EndedAt-StartedAt, pre-computed for the recorder
	// so it doesn't have to repeat the subtraction.
	Duration time.Duration
}

// nopRecorder is the zero-value default — drops every step on the
// floor. The client uses it when no WithRecorder option is passed so
// users who don't care about TCM steps get a working client.
type nopRecorder struct{}

func (nopRecorder) Record(context.Context, Step) {}

// ExternalRunsRecorder feeds captured steps into an externalruns.Client.
// One recorder is bound to ONE runID — typically the run the test job
// already opened via Client.CreateRun before exercising the gRPC service.
//
// Steps are buffered and flushed when the batch reaches batchSize OR the
// flushInterval expires. The recorder owns a background goroutine that
// drains the channel; call Close to flush + stop it. The constructor
// returns the recorder + a Close func so the typical test reads:
//
//	rec := NewExternalRunsRecorder(runs, runID)
//	defer rec.Close()
type ExternalRunsRecorder struct {
	runs   *externalruns.Client
	runID  string
	ch     chan Step
	done   chan struct{}
	closed atomic.Bool
}

// NewExternalRunsRecorder wires a recorder that streams steps into the
// supplied externalruns.Client under the given runID. Nil client / empty
// runID yield a no-op recorder (silently — the test job can still call
// Record from inside a non-CI run without crashing).
func NewExternalRunsRecorder(runs *externalruns.Client, runID string) *ExternalRunsRecorder {
	r := &ExternalRunsRecorder{
		runs:  runs,
		runID: runID,
		ch:    make(chan Step, 64),
		done:  make(chan struct{}),
	}
	go r.loop()
	return r
}

// Record enqueues a step. Non-blocking on the happy path; falls back to
// a synchronous flush when the buffer is full so a long-running test
// can't lose steps to back-pressure on the channel.
func (r *ExternalRunsRecorder) Record(ctx context.Context, s Step) {
	if r == nil || r.runs == nil || r.runID == "" || r.closed.Load() {
		return
	}
	select {
	case r.ch <- s:
	default:
		// Full — drain one batch synchronously then retry.
		r.flushOne(ctx, s)
	}
}

// Close drains pending steps and stops the background goroutine.
// Safe to call multiple times — second + subsequent calls no-op.
func (r *ExternalRunsRecorder) Close() {
	if r == nil || !r.closed.CompareAndSwap(false, true) {
		return
	}
	close(r.ch)
	<-r.done
}

func (r *ExternalRunsRecorder) loop() {
	defer close(r.done)
	const batchSize = 32
	const flushInterval = 250 * time.Millisecond
	buf := make([]externalruns.Step, 0, batchSize)
	tick := time.NewTicker(flushInterval)
	defer tick.Stop()
	flush := func() {
		if len(buf) == 0 || r.runs == nil {
			return
		}
		_ = r.runs.AddSteps(context.Background(), r.runID, buf)
		buf = buf[:0]
	}
	for {
		select {
		case s, ok := <-r.ch:
			if !ok {
				flush()
				return
			}
			buf = append(buf, toExternalStep(s))
			if len(buf) >= batchSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

func (r *ExternalRunsRecorder) flushOne(ctx context.Context, s Step) {
	if r.runs == nil {
		return
	}
	_ = r.runs.AddSteps(ctx, r.runID, []externalruns.Step{toExternalStep(s)})
}

func toExternalStep(s Step) externalruns.Step {
	status := externalruns.Status(s.Status)
	if status == "" {
		status = externalruns.StatusPassed
	}
	ended := s.EndedAt
	if ended.IsZero() && !s.StartedAt.IsZero() {
		ended = s.StartedAt.Add(s.Duration)
	}
	return externalruns.Step{
		StartedAt:  s.StartedAt,
		FinishedAt: &ended,
		Parameters: s.Parameters,
		StepKey:    s.Key,
		Name:       s.Name,
		Status:     status,
		Message:    s.Message,
		DurationMS: s.Duration.Milliseconds(),
	}
}

// stepKey builds a stable per-call key from method + monotonic counter.
// Exported via NewStepKey for tests that build steps manually.
func stepKey(method string, seq uint64) string {
	return fmt.Sprintf("%s#%d", method, seq)
}

// NewStepKey is the exported form of the internal key builder. Test
// code that constructs steps by hand uses it to match the format the
// client emits, so retries de-dup server-side as intended.
func NewStepKey(method string, seq uint64) string { return stepKey(method, seq) }
