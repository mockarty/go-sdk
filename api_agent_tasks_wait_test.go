// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAgentTasks_WaitForResult_Completed(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		status := "running"
		var result any
		if n >= 3 {
			status = "completed"
			result = map[string]any{"summary": "done"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task": AgentTask{ID: "t1", Status: status, Result: result},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.AgentTasks().WaitForResult(context.Background(), "t1", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForResult: %v", err)
	}
	if task.Status != "completed" {
		t.Errorf("expected completed, got %s", task.Status)
	}
	if calls.Load() < 3 {
		t.Errorf("expected ≥3 polls, got %d", calls.Load())
	}
}

func TestAgentTasks_WaitForResult_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"task": AgentTask{ID: "t1", Status: "failed"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.AgentTasks().WaitForResult(context.Background(), "t1", time.Millisecond)
	if !errors.Is(err, ErrAgentTaskFailed) {
		t.Fatalf("expected ErrAgentTaskFailed, got %v", err)
	}
	if task == nil || task.Status != "failed" {
		t.Error("expected the failed task carried through")
	}
}

func TestAgentTasks_WaitForResult_Cancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"task": AgentTask{ID: "t1", Status: "cancelled"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.AgentTasks().WaitForResult(context.Background(), "t1", time.Millisecond)
	if !errors.Is(err, ErrAgentTaskCancelled) {
		t.Fatalf("expected ErrAgentTaskCancelled, got %v", err)
	}
}

func TestAgentTasks_WaitForResult_CtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"task": AgentTask{ID: "t1", Status: "running"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	_, err := c.AgentTasks().WaitForResult(ctx, "t1", time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx deadline, got %v", err)
	}
}

func TestAgentTasks_SubmitAndWait(t *testing.T) {
	var submitted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			submitted.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{"task": AgentTask{ID: "t9", Status: "queued"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"task": AgentTask{ID: "t9", Status: "completed", Result: "ok"}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	task, err := c.AgentTasks().SubmitAndWait(context.Background(), &AgentTask{Title: "audit", Prompt: "do it"}, time.Millisecond)
	if err != nil {
		t.Fatalf("SubmitAndWait: %v", err)
	}
	if !submitted.Load() {
		t.Error("Submit was not called")
	}
	if task.Status != "completed" || task.Result != "ok" {
		t.Errorf("unexpected final task: %+v", task)
	}
}
