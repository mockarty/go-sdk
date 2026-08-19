package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEconomicsPriceBookAndUsage(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.Method == http.MethodPost:
			var p LLMPrice
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Provider != "openai" {
				t.Fatalf("bad append request: %#v err=%v", p, err)
			}
			p.ID = "price-1"
			_ = json.NewEncoder(w).Encode(p)
		case r.URL.Path == "/api/v1/admin/llm-prices":
			if r.URL.Query().Get("provider") != "openai" || r.URL.Query().Get("limit") != "20" {
				t.Fatalf("bad price query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(LLMPriceList{Prices: []LLMPrice{{ID: "price-1"}}})
		case r.URL.Path == "/api/v1/admin/llm-usage":
			if r.URL.Query().Get("groupBy") != "module" || r.URL.Query().Get("days") != "30" {
				t.Fatalf("bad usage query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(LLMUsageReport{UnpricedCalls: 2})
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithAPIKey("k"))
	ctx := context.Background()
	price := LLMPrice{Provider: "openai", Model: "gpt", Currency: "USD", EffectiveFrom: time.Now()}
	if got, err := c.Economics().AppendPrice(ctx, price); err != nil || got.ID != "price-1" {
		t.Fatalf("append=%#v err=%v", got, err)
	}
	if got, err := c.Economics().ListPrices(ctx, "openai", "", 20); err != nil || len(got.Prices) != 1 {
		t.Fatalf("list=%#v err=%v", got, err)
	}
	if got, err := c.Economics().GetUsage(ctx, LLMUsageQuery{GroupBy: "module", Days: 30}); err != nil || got.UnpricedCalls != 2 {
		t.Fatalf("usage=%#v err=%v", got, err)
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestEconomicsAppendPriceValidatesRequiredFields(t *testing.T) {
	c := NewClient("http://example.invalid")
	if _, err := c.Economics().AppendPrice(context.Background(), LLMPrice{}); err == nil {
		t.Fatal("empty price accepted")
	}
}

func TestEconomicsStatementAndRefund(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/llm-usage/statement.csv":
			if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("limit") != "50" {
				t.Fatalf("bad statement query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte("event_id,event_kind\ne1,llm_tokens\n"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/llm-usage/e1/refund":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["reason"] != "invalid response" {
				t.Fatalf("bad refund body: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(LLMUsageRefund{ID: "r1", OriginalEventID: "e1", RefundEventID: "e2", Reason: body["reason"]})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	api := NewClient(srv.URL, WithAPIKey("k")).Economics()
	data, err := api.DownloadUsageStatement(context.Background(), LLMUsageStatementQuery{Namespace: "team-a", Limit: 50})
	if err != nil || len(data) == 0 {
		t.Fatalf("statement=%q err=%v", data, err)
	}
	refund, err := api.RefundUsage(context.Background(), "e1", "invalid response")
	if err != nil || refund.ID != "r1" {
		t.Fatalf("refund=%+v err=%v", refund, err)
	}
}

func TestEconomicsBudgets(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case r.Method == http.MethodGet:
			if r.URL.Path != "/api/v1/admin/llm-budgets" || r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("active") != "true" {
				t.Fatalf("bad list request: %s", r.URL.String())
			}
			_ = json.NewEncoder(w).Encode(LLMBudgetList{Budgets: []LLMBudget{{ID: "budget-1"}}})
		case r.Method == http.MethodPost:
			var budget LLMBudget
			_ = json.NewDecoder(r.Body).Decode(&budget)
			budget.ID = "budget-1"
			_ = json.NewEncoder(w).Encode(budget)
		case r.Method == http.MethodPut:
			if r.URL.Path != "/api/v1/admin/llm-budgets/budget-1" {
				t.Fatalf("bad update path: %s", r.URL.Path)
			}
			var budget LLMBudget
			_ = json.NewDecoder(r.Body).Decode(&budget)
			_ = json.NewEncoder(w).Encode(budget)
		}
	}))
	defer srv.Close()
	client := NewClient(srv.URL, WithAPIKey("k"))
	ctx := context.Background()
	now := time.Now().UTC()
	budget := LLMBudget{Namespace: "team-a", ScopeType: "workspace", Currency: "USD", PeriodStart: now, PeriodEnd: now.Add(time.Hour), Enabled: true}
	created, err := client.Economics().CreateBudget(ctx, budget)
	if err != nil || created.ID != "budget-1" {
		t.Fatalf("create=%#v err=%v", created, err)
	}
	created.HardLimitMicros = 10
	if _, err := client.Economics().UpdateBudget(ctx, created); err != nil {
		t.Fatal(err)
	}
	listed, err := client.Economics().ListBudgets(ctx, "team-a", true, 100)
	if err != nil || len(listed.Budgets) != 1 {
		t.Fatalf("list=%#v err=%v", listed, err)
	}
	if calls != 3 {
		t.Fatalf("calls=%d", calls)
	}
}
