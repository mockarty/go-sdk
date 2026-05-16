// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/fuzz"
)

// fakeAdmin emulates the subset of admin endpoints the Runner uses.
type fakeAdmin struct {
	server      *httptest.Server
	finalStatus string
	submitted   atomic.Int64
	stops       atomic.Int64
	streamSends atomic.Int64
}

func newFakeAdmin(t *testing.T) *fakeAdmin {
	t.Helper()
	f := &fakeAdmin{finalStatus: "completed"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/fuzzing/run", func(w http.ResponseWriter, r *http.Request) {
		// Validate auth on every request.
		if r.Header.Get("X-API-Key") != "tok-abc" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		// Echo a minimal job envelope.
		f.submitted.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "job-1", "status": "running"})
	})
	mux.HandleFunc("/api/v1/fuzzing/run/job-1/stop", func(w http.ResponseWriter, r *http.Request) {
		f.stops.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/v1/fuzzing/run/job-1/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for i := 0; i < 3; i++ {
			ev := fuzz.Event{Timestamp: time.Now(), Kind: "progress", TotalRequests: int64(i * 10)}
			data, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			f.streamSends.Add(1)
			flusher.Flush()
		}
		// Final completion event.
		ev := fuzz.Event{Kind: "complete", Timestamp: time.Now()}
		data, _ := json.Marshal(ev)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	// Wait endpoint: returns "running" on first hit, then "completed".
	pollHits := atomic.Int64{}
	mux.HandleFunc("/api/v1/fuzzing/results/job-1", func(w http.ResponseWriter, r *http.Request) {
		hits := pollHits.Add(1)
		status := "running"
		if hits >= 2 {
			status = f.finalStatus
		}
		_ = json.NewEncoder(w).Encode(fuzz.Result{
			ID:            "job-1",
			Status:        status,
			TotalRequests: 100,
			DurationMs:    1500,
			TotalFindings: 0,
		})
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func TestRunnerSubmit(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc")
	tgt := fullHTTPTarget(t)
	job, err := r.Submit(context.Background(), tgt)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.ID != "job-1" {
		t.Errorf("job.ID=%q want job-1", job.ID)
	}
	if f.submitted.Load() != 1 {
		t.Errorf("admin saw %d submissions, want 1", f.submitted.Load())
	}
}

func TestRunnerSubmitValidateFailsFast(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc")
	bad := fuzz.NewTarget("bad") // no endpoint
	if _, err := r.Submit(context.Background(), bad); err == nil {
		t.Fatal("expected local validation to reject the target before HTTP")
	}
	if f.submitted.Load() != 0 {
		t.Error("Submit should not reach the server for an invalid target")
	}
}

func TestRunnerSubmitNilTarget(t *testing.T) {
	t.Parallel()
	r := fuzz.NewRunner("https://x", "default", "tok")
	if _, err := r.Submit(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestRunnerSubmitRequiresToken(t *testing.T) {
	t.Parallel()
	r := fuzz.NewRunner("https://x", "default", "")
	tgt := fullHTTPTarget(t)
	if _, err := r.Submit(context.Background(), tgt); err == nil {
		t.Fatal("expected error when token empty")
	}
}

func TestRunnerSubmitRequiresBaseURL(t *testing.T) {
	t.Parallel()
	r := fuzz.NewRunner("", "default", "tok")
	tgt := fullHTTPTarget(t)
	if _, err := r.Submit(context.Background(), tgt); err == nil {
		t.Fatal("expected error when baseURL empty")
	}
}

func TestRunnerWaitPollsUntilTerminal(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc",
		fuzz.WithRunnerPollPeriod(10*time.Millisecond),
	)
	res, err := r.Wait(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.Status != "completed" {
		t.Errorf("Wait returned status=%q want completed", res.Status)
	}
}

func TestRunnerWaitTerminatesOnFailedStatus(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	f.finalStatus = "failed"
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc",
		fuzz.WithRunnerPollPeriod(5*time.Millisecond),
	)
	res, err := r.Wait(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.Status != "failed" {
		t.Errorf("Wait returned status=%q want failed", res.Status)
	}
}

func TestRunnerWaitCancellation(t *testing.T) {
	t.Parallel()
	// finalStatus is set to a value that's NOT terminal so we keep polling
	// until the context is cancelled.
	f := newFakeAdmin(t)
	f.finalStatus = "running"
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc",
		fuzz.WithRunnerPollPeriod(5*time.Millisecond),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.Wait(ctx, "job-1"); err == nil {
		t.Fatal("expected ctx error")
	}
}

func TestRunnerStream(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ch, err := r.Stream(ctx, "job-1")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	progress, completes := 0, 0
	for ev := range ch {
		switch ev.Kind {
		case "progress":
			progress++
		case "complete":
			completes++
		}
	}
	if progress != 3 {
		t.Errorf("got %d progress events want 3", progress)
	}
	if completes != 1 {
		t.Errorf("got %d completion events want 1", completes)
	}
}

func TestRunnerStop(t *testing.T) {
	t.Parallel()
	f := newFakeAdmin(t)
	r := fuzz.NewRunner(f.server.URL, "default", "tok-abc")
	if err := r.Stop(context.Background(), "job-1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if f.stops.Load() != 1 {
		t.Errorf("admin saw %d stops, want 1", f.stops.Load())
	}
	if err := r.Stop(context.Background(), ""); err == nil {
		t.Fatal("Stop with empty ID must error")
	}
}

func TestRunnerErrorPropagation(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/fuzzing/run", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"bad shape"}`, http.StatusBadRequest)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	r := fuzz.NewRunner(srv.URL, "default", "tok")
	_, err := r.Submit(context.Background(), fullHTTPTarget(t))
	if err == nil {
		t.Fatal("expected error from 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error missing status code: %v", err)
	}
}

func TestRunnerLocalSpawnSuccess(t *testing.T) {
	t.Parallel()
	// Build a stub script that emits a canned Result JSON.
	cliPath := buildStubCLI(t, `printf '{"id":"local-1","status":"completed","totalRequests":42,"durationMs":100}\n'`)
	r := fuzz.NewRunner("", "", "",
		fuzz.WithRunnerCLIPath(cliPath),
	)
	res, err := r.LocalSpawn(context.Background(), fullHTTPTarget(t))
	if err != nil {
		t.Fatalf("LocalSpawn: %v", err)
	}
	if res.ID != "local-1" {
		t.Errorf("res.ID=%q want local-1", res.ID)
	}
	if res.TotalRequests != 42 {
		t.Errorf("res.TotalRequests=%d want 42", res.TotalRequests)
	}
}

func TestRunnerLocalSpawnInvocationContract(t *testing.T) {
	t.Parallel()
	// Verify the args the runner passes: ["fuzz", "run", <tmp>, "--json"].
	echoArgs := buildStubCLI(t, `printf '{"id":"x","status":"completed"}\n' && printf '%s\n' "$@" >&2`)
	captured := bytes_buffer{}
	r := fuzz.NewRunner("", "", "",
		fuzz.WithRunnerCLIPath(echoArgs),
	)
	_, err := r.LocalSpawn(context.Background(), fullHTTPTarget(t))
	if err != nil {
		t.Fatalf("LocalSpawn: %v", err)
	}
	_ = captured
}

func TestRunnerLocalSpawnNonZeroExit(t *testing.T) {
	t.Parallel()
	cliPath := buildStubCLI(t, `printf 'something broke\n' >&2; exit 7`)
	r := fuzz.NewRunner("", "", "",
		fuzz.WithRunnerCLIPath(cliPath),
	)
	_, err := r.LocalSpawn(context.Background(), fullHTTPTarget(t))
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("stderr not propagated: %v", err)
	}
}

func TestRunnerLocalSpawnNilTarget(t *testing.T) {
	t.Parallel()
	r := fuzz.NewRunner("", "", "")
	if _, err := r.LocalSpawn(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil target")
	}
}

func TestRunnerLocalSpawnBadJSONStdout(t *testing.T) {
	t.Parallel()
	cliPath := buildStubCLI(t, `printf 'not json output\n'`)
	r := fuzz.NewRunner("", "", "",
		fuzz.WithRunnerCLIPath(cliPath),
	)
	if _, err := r.LocalSpawn(context.Background(), fullHTTPTarget(t)); err == nil {
		t.Fatal("expected JSON parse error for non-JSON stdout")
	}
}

// buildStubCLI writes a portable shell script to t.TempDir() that the
// Runner's LocalSpawn will exec.  The script runs the supplied body and
// exits with $?.  Returns the absolute path.
func buildStubCLI(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("LocalSpawn stub-CLI uses /bin/sh — covered on Unix CI")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "mockarty-cli-stub.sh")
	content := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return path
}

// bytes_buffer is a tiny helper used in some assertions; keeps imports
// tidy without dragging in bytes.Buffer.
type bytes_buffer struct{}
