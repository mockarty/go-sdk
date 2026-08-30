package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestCloudRefundsListRefundsDecodesRedactedOperatorProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.RequestURI() != "/api/v1/cloud/operator/refunds" {
			t.Fatalf("request = %s %s", r.Method, r.URL.RequestURI())
		}
		_, _ = w.Write([]byte(`{"payments":[{"operation_id":"payment-ignored"}],"refunds":[{"operation_id":"refund-1","generation":4,"status":"operator_required","amount_minor":1500,"currency":"RUB","provider":"yookassa"}],"total":1,"refund_total":1,"request_id":"req-list-1"}`))
	}))
	defer server.Close()

	refunds, err := NewClient(server.URL).CloudRefunds().ListRefunds(context.Background())
	if err != nil || len(refunds) != 1 {
		t.Fatalf("refunds=%#v err=%v", refunds, err)
	}
	got := refunds[0]
	if got.OperationID != "refund-1" || got.Generation != 4 || got.Status != "operator_required" ||
		got.AmountMinor != 1500 || got.Currency != "RUB" || got.Provider != "yookassa" {
		t.Fatalf("redacted refund=%#v", got)
	}
}

func TestCloudRefundsListRefundsFailsClosedOnMalformedProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"payments":[],"refunds":[{"operation_id":"refund-1","generation":4,"status":"operator_required","amount_minor":1500,"currency":"RUB"}]}`))
	}))
	defer server.Close()
	if _, err := NewClient(server.URL).CloudRefunds().ListRefunds(context.Background()); err == nil {
		t.Fatal("malformed redacted refund projection accepted")
	}
}

func TestCloudRefundsResolveRefundContract(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/v1/cloud/operator/refunds/refund%2F1/resolve" {
			t.Fatalf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if got := r.Header.Get("Idempotency-Key"); got != "a" {
			t.Fatalf("Idempotency-Key = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"refund":{"operation_id":"refund/1","status":"accepted","generation":5},"replayed":true,"request_id":"req-1"}`))
	}))
	defer server.Close()

	got, err := NewClient(server.URL).CloudRefunds().ResolveRefund(context.Background(), "refund/1",
		CloudRefundRetry, "provider_recovery_retry", 4, "a")
	if err != nil || got.Refund.OperationID != "refund/1" || got.Refund.Generation != 5 || !got.Replayed {
		t.Fatalf("resolution=%#v err=%v", got, err)
	}
	if body["action"] != "retry" || body["reason_code"] != "provider_recovery_retry" || body["generation"] != float64(4) {
		t.Fatalf("body=%#v", body)
	}
}

func TestCloudRefundsDoesNotExposeInteractiveSelfServiceCreation(t *testing.T) {
	apiType := reflect.TypeOf(NewClient("http://127.0.0.1:1").CloudRefunds())
	for _, forbidden := range []string{"RequestRefund", "CreateRefund"} {
		if _, ok := apiType.MethodByName(forbidden); ok {
			t.Fatalf("%s exposes the browser-session step-up refund endpoint", forbidden)
		}
	}
}

func TestCloudRefundsRejectsUnsafeResolution(t *testing.T) {
	api := NewClient("http://127.0.0.1:1").CloudRefunds()
	tests := []struct {
		name           string
		operationID    string
		action         CloudRefundResolutionAction
		reasonCode     string
		generation     int64
		idempotencyKey string
	}{
		{name: "missing operation", action: CloudRefundReject, reasonCode: "provider_reject", idempotencyKey: "refund-1"},
		{name: "blank operation", operationID: " ", action: CloudRefundReject, reasonCode: "provider_reject", idempotencyKey: "refund-1"},
		{name: "manual success", operationID: "op-1", action: "succeeded", reasonCode: "operator_says_paid", idempotencyKey: "refund-1"},
		{name: "unsafe reason", operationID: "op-1", action: CloudRefundReject, reasonCode: "Customer said no", idempotencyKey: "refund-1"},
		{name: "negative generation", operationID: "op-1", action: CloudRefundRetry, reasonCode: "provider_retry", generation: -1, idempotencyKey: "refund-1"},
		{name: "empty key", operationID: "op-1", action: CloudRefundRetry, reasonCode: "provider_retry"},
		{name: "key too long", operationID: "op-1", action: CloudRefundRetry, reasonCode: "provider_retry", idempotencyKey: strings.Repeat("a", 129)},
		{name: "unsafe key", operationID: "op-1", action: CloudRefundRetry, reasonCode: "provider_retry", idempotencyKey: "unsafe key"},
		{name: "mutating key forbidden", operationID: "op-1", action: CloudRefundRetry, reasonCode: "provider_retry", idempotencyKey: " refund-1 "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := api.ResolveRefund(context.Background(), test.operationID, test.action, test.reasonCode,
				test.generation, test.idempotencyKey); err == nil {
				t.Fatal("unsafe refund resolution accepted")
			}
		})
	}
}
