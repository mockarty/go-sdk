// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWrapGroupsSteps(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))
	tt.Wrap("user signup", func() {
		tt.HTTP().GET("/users/42").ExpectStatus(200)
		tt.HTTP().GET("/users/42").ExpectStatus(200)
	})
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	if got := len(tt.Report()); got != 2 {
		t.Fatalf("want 2 steps, got %d", got)
	}
}

func TestWrapPanicReraises(t *testing.T) {
	tt := New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Wrap should re-raise panic")
		}
	}()
	tt.Wrap("panicky", func() { panic("boom") })
}

func TestEventuallySucceeds(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	ok := tt.Eventually(2*time.Second, 30*time.Millisecond, func() error {
		tt.HTTP().GET("/").ExpectStatus(200)
		if !tt.OK() {
			return errors.New("not ready")
		}
		return nil
	})
	if !ok {
		t.Fatalf("Eventually should have succeeded: %v", tt.Errors())
	}
	if got := attempts.Load(); got < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", got)
	}
}

func TestEventuallyTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	ok := tt.Eventually(150*time.Millisecond, 30*time.Millisecond, func() error {
		tt.HTTP().GET("/").ExpectStatus(200)
		if !tt.OK() {
			return errors.New("nope")
		}
		return nil
	})
	if ok {
		t.Fatal("Eventually should have timed out")
	}
	// After timeout the last failure landed in tt.Errors so users can
	// see WHY it failed.
	if tt.OK() {
		t.Fatal("expected accumulated errors after timeout")
	}
}

func TestParallelFanout(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))
	tt.Parallel(
		func(b *Tester) { b.HTTP().GET("/users/42").ExpectStatus(200) },
		func(b *Tester) { b.HTTP().GET("/users/42").ExpectStatus(200) },
		func(b *Tester) { b.HTTP().GET("/users/42").ExpectStatus(200) },
	)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	if got := len(tt.Report()); got != 3 {
		t.Fatalf("want 3 steps, got %d", got)
	}
}

func TestParallelMergesFailures(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))
	tt.Parallel(
		func(b *Tester) { b.HTTP().GET("/users/42").ExpectStatus(200) }, // pass
		func(b *Tester) { b.HTTP().GET("/users/42").ExpectStatus(204) }, // fail
	)
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected one branch failure to land in parent")
	}
	if got := len(tt.Report()); got != 2 {
		t.Fatalf("want 2 steps total, got %d", got)
	}
}

func TestParallelEmptyNoop(t *testing.T) {
	tt := New()
	tt.Parallel()
	tt.Finish()
	if !tt.OK() {
		t.Fatal("empty Parallel should be a no-op")
	}
	if got := len(tt.Report()); got != 0 {
		t.Fatalf("want 0 steps, got %d", got)
	}
}
