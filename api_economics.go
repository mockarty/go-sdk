// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type EconomicsAPI struct{ client *Client }

type LLMPrice struct {
	CreatedAt                  time.Time `json:"createdAt,omitempty"`
	EffectiveFrom              time.Time `json:"effectiveFrom"`
	InputMicrosPerMillion      int64     `json:"inputMicrosPerMillion"`
	OutputMicrosPerMillion     int64     `json:"outputMicrosPerMillion"`
	CacheReadMicrosPerMillion  int64     `json:"cacheReadMicrosPerMillion"`
	CacheWriteMicrosPerMillion int64     `json:"cacheWriteMicrosPerMillion"`
	ID                         string    `json:"id,omitempty"`
	Provider                   string    `json:"provider"`
	Model                      string    `json:"model"`
	Currency                   string    `json:"currency"`
	Source                     string    `json:"source,omitempty"`
}

type LLMPriceList struct {
	Prices []LLMPrice `json:"prices"`
}

type ResourcePrice struct {
	CreatedAt             time.Time `json:"createdAt,omitempty"`
	EffectiveFrom         time.Time `json:"effectiveFrom"`
	ProviderMicrosPerUnit int64     `json:"providerMicrosPerUnit"`
	CustomerMicrosPerUnit int64     `json:"customerMicrosPerUnit"`
	ID                    string    `json:"id,omitempty"`
	EventKind             string    `json:"eventKind"`
	Provider              string    `json:"provider"`
	Resource              string    `json:"resource"`
	Unit                  string    `json:"unit"`
	Currency              string    `json:"currency"`
	Source                string    `json:"source,omitempty"`
}

type ResourcePriceList struct {
	ResourcePrices []ResourcePrice `json:"resourcePrices"`
}

type ResourcePriceQuery struct {
	EventKind string
	Provider  string
	Resource  string
	Unit      string
	Limit     int
}

type LLMUsageQuery struct {
	GroupBy string
	Days    int
}

type LLMUsageStatementQuery struct {
	From      time.Time
	To        time.Time
	Namespace string
	ProfileID string
	Limit     int
}

type LLMUsageRefund struct {
	CreatedAt       time.Time `json:"createdAt"`
	ID              string    `json:"id"`
	OriginalEventID string    `json:"originalEventId"`
	RefundEventID   string    `json:"refundEventId"`
	ActorID         string    `json:"actorId"`
	Namespace       string    `json:"namespace"`
	Reason          string    `json:"reason"`
}

type LLMUsageTotals struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	TotalTokens  int64 `json:"totalTokens"`
	Calls        int64 `json:"calls"`
}

type LLMUsageCost struct {
	ProviderCostMicros int64  `json:"providerCostMicros"`
	PlatformFeeMicros  int64  `json:"platformFeeMicros"`
	MarkupMicros       int64  `json:"markupMicros"`
	TaxMicros          int64  `json:"taxMicros"`
	DiscountMicros     int64  `json:"discountMicros"`
	CustomerCostMicros int64  `json:"customerCostMicros"`
	MarginMicros       int64  `json:"marginMicros"`
	IncludedMicros     int64  `json:"includedMicros"`
	PrepaidMicros      int64  `json:"prepaidMicros"`
	OverageMicros      int64  `json:"overageMicros"`
	Calls              int64  `json:"calls"`
	BYOKCalls          int64  `json:"byokCalls"`
	ResourceEvents     int64  `json:"resourceEvents"`
	ResourceQuantity   int64  `json:"resourceQuantity"`
	Currency           string `json:"currency"`
}

type LLMUsageForecast struct {
	ObservedMicros       int64   `json:"observedMicros"`
	DailyRunRateMicros   int64   `json:"dailyRunRateMicros"`
	Projected30DayMicros int64   `json:"projected30DayMicros"`
	Recent24HoursMicros  int64   `json:"recent24HoursMicros"`
	PriorDailyMicros     int64   `json:"priorDailyMicros"`
	RecentBaselineRatio  float64 `json:"recentToBaselineRatio"`
	Currency             string  `json:"currency"`
	Status               string  `json:"status"`
}

type LLMUsageOutcomeCost struct {
	ProviderCostMicros int64  `json:"providerCostMicros"`
	CustomerCostMicros int64  `json:"customerCostMicros"`
	Calls              int64  `json:"calls"`
	ResourceEvents     int64  `json:"resourceEvents"`
	ResourceQuantity   int64  `json:"resourceQuantity"`
	Outcome            string `json:"outcome"`
	Currency           string `json:"currency"`
}

type ResourceUsageTotal struct {
	Events    int64  `json:"events"`
	Quantity  int64  `json:"quantity"`
	EventKind string `json:"eventKind"`
	Unit      string `json:"unit"`
}

type LLMUsageReconciliation struct {
	Reserved          int64 `json:"reserved"`
	Settled           int64 `json:"settled"`
	Released          int64 `json:"released"`
	Expired           int64 `json:"expired"`
	MissingUsageEvent int64 `json:"missingUsageEvent"`
	OrphanUsageEvent  int64 `json:"orphanUsageEvent"`
}

type LLMUsageGroup struct {
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	TotalTokens  int64  `json:"totalTokens"`
	Calls        int64  `json:"calls"`
	Key          string `json:"key"`
	Label        string `json:"label"`
}

type LLMUsageReport struct {
	Totals         LLMUsageTotals         `json:"totals"`
	Reconciliation LLMUsageReconciliation `json:"reconciliation"`
	Rows           []LLMUsageGroup        `json:"rows"`
	Costs          []LLMUsageCost         `json:"costs"`
	Forecast       []LLMUsageForecast     `json:"forecast"`
	OutcomeCosts   []LLMUsageOutcomeCost  `json:"outcomeCosts"`
	ResourceTotals []ResourceUsageTotal   `json:"resourceTotals"`
	UnpricedCalls  int64                  `json:"unpricedCalls"`
	UnpricedEvents int64                  `json:"unpricedEvents"`
}

type LLMBudget struct {
	CreatedAt       time.Time `json:"createdAt,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt,omitempty"`
	PeriodStart     time.Time `json:"periodStart"`
	PeriodEnd       time.Time `json:"periodEnd"`
	IncludedMicros  int64     `json:"includedMicros"`
	PrepaidMicros   int64     `json:"prepaidMicros"`
	SoftLimitMicros int64     `json:"softLimitMicros"`
	HardLimitMicros int64     `json:"hardLimitMicros"`
	SpentMicros     int64     `json:"spentMicros,omitempty"`
	ReservedMicros  int64     `json:"reservedMicros,omitempty"`
	ID              string    `json:"id,omitempty"`
	Namespace       string    `json:"namespace"`
	ScopeType       string    `json:"scopeType"`
	ScopeID         string    `json:"scopeId,omitempty"`
	Currency        string    `json:"currency"`
	OverageAllowed  bool      `json:"overageAllowed"`
	RequirePriced   bool      `json:"requirePriced"`
	Enabled         bool      `json:"enabled"`
}

type LLMBudgetList struct {
	Budgets []LLMBudget `json:"budgets"`
}

func (a *EconomicsAPI) ListPrices(ctx context.Context, provider, model string, limit int) (LLMPriceList, error) {
	q := url.Values{}
	if strings.TrimSpace(provider) != "" {
		q.Set("provider", strings.TrimSpace(provider))
	}
	if strings.TrimSpace(model) != "" {
		q.Set("model", strings.TrimSpace(model))
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v1/admin/llm-prices"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out LLMPriceList
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return LLMPriceList{}, err
	}
	if out.Prices == nil {
		out.Prices = []LLMPrice{}
	}
	return out, nil
}

func (a *EconomicsAPI) AppendPrice(ctx context.Context, price LLMPrice) (LLMPrice, error) {
	if strings.TrimSpace(price.Provider) == "" || strings.TrimSpace(price.Model) == "" ||
		strings.TrimSpace(price.Currency) == "" || price.EffectiveFrom.IsZero() {
		return LLMPrice{}, fmt.Errorf("mockarty: economics append price: provider, model, currency and effective time are required")
	}
	var out LLMPrice
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/admin/llm-prices", price, &out); err != nil {
		return LLMPrice{}, err
	}
	return out, nil
}

func (a *EconomicsAPI) ListResourcePrices(ctx context.Context, query ResourcePriceQuery) (ResourcePriceList, error) {
	kind := strings.TrimSpace(query.EventKind)
	unit := strings.TrimSpace(query.Unit)
	if !validResourcePriceKindUnit(kind, unit, false) {
		return ResourcePriceList{}, fmt.Errorf("mockarty: economics list resource prices: event kind must be tool_call or runner_seconds and unit must match")
	}
	q := url.Values{"eventKind": []string{kind}}
	for key, value := range map[string]string{
		"provider": query.Provider, "resource": query.Resource, "unit": unit,
	} {
		if value = strings.TrimSpace(value); value != "" {
			q.Set(key, value)
		}
	}
	if query.Limit > 0 {
		q.Set("limit", strconv.Itoa(query.Limit))
	}
	var out ResourcePriceList
	if err := a.client.do(ctx, http.MethodGet, "/api/v1/admin/llm-prices?"+q.Encode(), nil, &out); err != nil {
		return ResourcePriceList{}, err
	}
	if out.ResourcePrices == nil {
		out.ResourcePrices = []ResourcePrice{}
	}
	return out, nil
}

func (a *EconomicsAPI) AppendResourcePrice(ctx context.Context, price ResourcePrice) (ResourcePrice, error) {
	if strings.TrimSpace(price.Provider) == "" || strings.TrimSpace(price.Resource) == "" ||
		strings.TrimSpace(price.Currency) == "" || price.EffectiveFrom.IsZero() ||
		price.ProviderMicrosPerUnit < 0 || price.CustomerMicrosPerUnit < 0 ||
		!validResourcePriceKindUnit(strings.TrimSpace(price.EventKind), strings.TrimSpace(price.Unit), true) {
		return ResourcePrice{}, fmt.Errorf("mockarty: economics append resource price: provider, resource, currency, effective time and a matching kind/unit are required")
	}
	var out ResourcePrice
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/admin/llm-prices", price, &out); err != nil {
		return ResourcePrice{}, err
	}
	return out, nil
}

func validResourcePriceKindUnit(kind, unit string, requireUnit bool) bool {
	if requireUnit && unit == "" {
		return false
	}
	return (kind == "tool_call" && (unit == "" || unit == "calls")) ||
		(kind == "runner_seconds" && (unit == "" || unit == "seconds"))
}

func (a *EconomicsAPI) GetUsage(ctx context.Context, query LLMUsageQuery) (LLMUsageReport, error) {
	q := url.Values{}
	if query.GroupBy != "" {
		q.Set("groupBy", query.GroupBy)
	}
	if query.Days > 0 {
		q.Set("days", strconv.Itoa(query.Days))
	}
	path := "/api/v1/admin/llm-usage"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out LLMUsageReport
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return LLMUsageReport{}, err
	}
	if out.Rows == nil {
		out.Rows = []LLMUsageGroup{}
	}
	if out.Costs == nil {
		out.Costs = []LLMUsageCost{}
	}
	if out.Forecast == nil {
		out.Forecast = []LLMUsageForecast{}
	}
	if out.OutcomeCosts == nil {
		out.OutcomeCosts = []LLMUsageOutcomeCost{}
	}
	if out.ResourceTotals == nil {
		out.ResourceTotals = []ResourceUsageTotal{}
	}
	return out, nil
}

func (a *EconomicsAPI) DownloadUsageStatement(ctx context.Context, query LLMUsageStatementQuery) ([]byte, error) {
	q := url.Values{}
	if !query.From.IsZero() {
		q.Set("from", query.From.UTC().Format(time.RFC3339Nano))
	}
	if !query.To.IsZero() {
		q.Set("to", query.To.UTC().Format(time.RFC3339Nano))
	}
	if value := strings.TrimSpace(query.Namespace); value != "" {
		q.Set("namespace", value)
	}
	if value := strings.TrimSpace(query.ProfileID); value != "" {
		q.Set("profileId", value)
	}
	if query.Limit > 0 {
		q.Set("limit", strconv.Itoa(query.Limit))
	}
	path := "/api/v1/admin/llm-usage/statement.csv"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return a.client.doJSON(ctx, http.MethodGet, path, nil)
}

func (a *EconomicsAPI) RefundUsage(ctx context.Context, eventID, reason string) (LLMUsageRefund, error) {
	eventID = strings.TrimSpace(eventID)
	reason = strings.TrimSpace(reason)
	if eventID == "" || len(reason) < 3 {
		return LLMUsageRefund{}, fmt.Errorf("mockarty: economics refund: event id and reason are required")
	}
	var out LLMUsageRefund
	path := "/api/v1/admin/llm-usage/" + url.PathEscape(eventID) + "/refund"
	if err := a.client.do(ctx, http.MethodPost, path, map[string]string{"reason": reason}, &out); err != nil {
		return LLMUsageRefund{}, err
	}
	return out, nil
}

func (a *EconomicsAPI) ListBudgets(ctx context.Context, namespace string, active bool, limit int) (LLMBudgetList, error) {
	q := url.Values{}
	if strings.TrimSpace(namespace) != "" {
		q.Set("namespace", strings.TrimSpace(namespace))
	}
	if active {
		q.Set("active", "true")
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v1/admin/llm-budgets"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out LLMBudgetList
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return LLMBudgetList{}, err
	}
	if out.Budgets == nil {
		out.Budgets = []LLMBudget{}
	}
	return out, nil
}

func (a *EconomicsAPI) CreateBudget(ctx context.Context, budget LLMBudget) (LLMBudget, error) {
	if err := validateLLMBudget(budget); err != nil {
		return LLMBudget{}, err
	}
	var out LLMBudget
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/admin/llm-budgets", budget, &out); err != nil {
		return LLMBudget{}, err
	}
	return out, nil
}

func (a *EconomicsAPI) UpdateBudget(ctx context.Context, budget LLMBudget) (LLMBudget, error) {
	if strings.TrimSpace(budget.ID) == "" {
		return LLMBudget{}, fmt.Errorf("mockarty: economics update budget: id is required")
	}
	if err := validateLLMBudget(budget); err != nil {
		return LLMBudget{}, err
	}
	var out LLMBudget
	path := "/api/v1/admin/llm-budgets/" + url.PathEscape(strings.TrimSpace(budget.ID))
	if err := a.client.do(ctx, http.MethodPut, path, budget, &out); err != nil {
		return LLMBudget{}, err
	}
	return out, nil
}

func validateLLMBudget(budget LLMBudget) error {
	if strings.TrimSpace(budget.Namespace) == "" || strings.TrimSpace(budget.ScopeType) == "" ||
		strings.TrimSpace(budget.Currency) == "" || budget.PeriodStart.IsZero() || !budget.PeriodEnd.After(budget.PeriodStart) {
		return fmt.Errorf("mockarty: economics budget: namespace, scope, currency and a valid period are required")
	}
	return nil
}
