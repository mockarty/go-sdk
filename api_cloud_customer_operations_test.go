// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCloudCustomerAndOperationsCanonicalRoutes(t *testing.T) {
	t.Helper()
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.RequestURI()+" idem="+r.Header.Get("Idempotency-Key"))
		_, _ = io.WriteString(w, `{}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("cloud-token"))
	ctx := context.Background()
	_, _ = client.CloudCustomer().ListLoyaltyRedemptions(ctx, "space/1", "next", 25)
	_, _ = client.CloudCustomer().RedeemLoyalty(ctx, "space/1", "WELCOME", "RU", "redeem-1")
	_, _ = client.CloudCustomer().ListSupportCases(ctx, "space/1", "open", "cursor", 20)
	_, _ = client.CloudCustomer().OpenSupportCase(ctx, "space/1", CloudSupportOpenRequest{Subject: "Help", Category: "billing", Priority: "normal", Message: "Please help", IdempotencyKey: "case-1"})
	_, _ = client.CloudCustomer().GetSupportCase(ctx, "space/1", "case/1")
	_, _ = client.CloudCustomer().ReplySupportCase(ctx, "space/1", "case/1", "Reply", "reply-1")
	_, _ = client.CloudCustomer().GetRiskAppeal(ctx, "risk/1")
	_, _ = client.CloudCustomer().SubmitRiskAppeal(ctx, "risk/1", "This decision needs review", "appeal-1")
	_, _ = client.CloudOperations().ListSupportCases(ctx, "open", "op-next", 50)
	_, _ = client.CloudOperations().GetSupportCase(ctx, "case/1")
	_, _ = client.CloudOperations().ReplySupportCase(ctx, "case/1", "Operator reply", "customer", "op-reply-1")
	_, _ = client.CloudOperations().AssignSupportCase(ctx, "case/1", "user/1", 7)
	_, _ = client.CloudOperations().TransitionSupportCase(ctx, "case/1", "resolved", 8)
	_, _ = client.CloudOperations().ProductAnalytics(ctx, 30)

	want := []string{
		"GET /api/v1/cloud/spaces/space%2F1/loyalty/redemptions?cursor=next&limit=25 idem=",
		"POST /api/v1/cloud/spaces/space%2F1/loyalty/redemptions idem=",
		"GET /api/v1/cloud/spaces/space%2F1/support/cases?cursor=cursor&limit=20&status=open idem=",
		"POST /api/v1/cloud/spaces/space%2F1/support/cases idem=",
		"GET /api/v1/cloud/spaces/space%2F1/support/cases/case%2F1 idem=",
		"POST /api/v1/cloud/spaces/space%2F1/support/cases/case%2F1/messages idem=",
		"GET /api/v1/cloud/risk/cases/risk%2F1/appeal idem=",
		"POST /api/v1/cloud/risk/cases/risk%2F1/appeal idem=appeal-1",
		"GET /api/v1/cloud/operator/support/cases?cursor=op-next&limit=50&status=open idem=",
		"GET /api/v1/cloud/operator/support/cases/case%2F1 idem=",
		"POST /api/v1/cloud/operator/support/cases/case%2F1/messages idem=",
		"POST /api/v1/cloud/operator/support/cases/case%2F1/assign idem=",
		"POST /api/v1/cloud/operator/support/cases/case%2F1/transition idem=",
		"GET /api/v1/cloud/operator/analytics/product?days=30 idem=",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %#v, want %#v", got, want)
	}
}

func TestCloudSpacesRenameRequiresFencing(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	if _, err := client.CloudSpaces().Rename(context.Background(), "space-1", "Renamed", "", ""); err == nil {
		t.Fatal("Rename accepted missing ETag and idempotency key")
	}
}

func TestCloudProductAnalyticsRejectsUnsupportedWindow(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	for _, days := range []int{0, 91} {
		if _, err := client.CloudOperations().ProductAnalytics(context.Background(), days); err == nil {
			t.Fatalf("ProductAnalytics(%d) accepted unsupported window", days)
		}
	}
}
