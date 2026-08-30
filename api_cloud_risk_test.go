package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudRiskOperatorContract(t *testing.T) {
	var releaseBody map[string]any
	var releaseIdempotency string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/cloud/operator/risk/cases":
			if r.URL.Query().Get("status") != "open" || r.URL.Query().Get("limit") != "25" {
				t.Errorf("case query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"cases":[{"id":"case-1","status":"open","revision":1}]}`))
		case "/api/v1/cloud/operator/risk/cases/case-1":
			_, _ = w.Write([]byte(`{"case":{"id":"case-1"},"events":[],"enforcements":[{"id":"enf-1","revision":2}]}`))
		case "/api/v1/cloud/operator/risk/cases/case-1/enforcements/enf-1/release":
			releaseIdempotency = r.Header.Get("Idempotency-Key")
			if err := json.NewDecoder(r.Body).Decode(&releaseBody); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"enforcement":{"id":"enf-1","status":"released","revision":3}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	api := NewClient(server.URL).CloudRisk()
	cases, err := api.ListCases(context.Background(), "open", 25)
	if err != nil || len(cases) != 1 || cases[0].ID != "case-1" {
		t.Fatalf("cases=%#v err=%v", cases, err)
	}
	detail, err := api.GetCase(context.Background(), "case-1")
	if err != nil || len(detail.Enforcements) != 1 {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	released, err := api.ReleaseEnforcement(context.Background(), "case-1", "enf-1", 2, "customer verified")
	if err != nil || released.Status != "released" || releaseBody["revision"] != float64(2) || !strings.HasPrefix(releaseIdempotency, "risk-release:") {
		t.Fatalf("released=%#v body=%#v err=%v", released, releaseBody, err)
	}
}
