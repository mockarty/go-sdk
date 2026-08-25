package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentTasks_GetProjectsToolReceipts(t *testing.T) {
	receiptKey := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"task":                              map[string]any{"id": "task-1", "status": "running"},
			"toolReceipts":                      []map[string]any{{"receiptKey": receiptKey, "status": "awaiting_reconcile", "version": 3}},
			"canReconcileToolReceipts":          false,
			"toolReceiptRetryAllowed":           false,
			"toolReceiptReconcileBlockedReason": "task_active",
		})
	}))
	defer server.Close()

	task, err := NewClient(server.URL).AgentTasks().Get(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ToolReceipts) != 1 || task.ToolReceipts[0].Version != 3 ||
		task.CanReconcileToolReceipts || task.ToolReceiptRetryAllowed ||
		task.ToolReceiptReconcileBlockedReason != "task_active" {
		t.Fatalf("task detail = %#v", task)
	}
}

func TestAgentTasks_ReconcileToolReceiptWireContract(t *testing.T) {
	receiptKey := "sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/api/v1/agent/tasks/task-1/tool-receipts/" + receiptKey + "/reconcile"
		if r.Method != http.MethodPost || r.URL.Path != want {
			t.Fatalf("request = %s %s, want POST %s", r.Method, r.URL.Path, want)
		}
		var request ToolReceiptReconcileRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.IdempotencyKey != "review-1" || request.ExpectedVersion != 7 || request.Decision != "already_applied" {
			t.Fatalf("request = %#v", request)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"receipt": map[string]any{
			"receiptKey": receiptKey, "status": "done", "version": 8,
		}})
	}))
	defer server.Close()

	receipt, err := NewClient(server.URL).AgentTasks().ReconcileToolReceipt(context.Background(), "task-1", receiptKey,
		ToolReceiptReconcileRequest{IdempotencyKey: "review-1", Decision: "already_applied", Reason: "verified", Result: "external id 42", ExpectedVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "done" || receipt.Version != 8 {
		t.Fatalf("receipt = %#v", receipt)
	}
}
