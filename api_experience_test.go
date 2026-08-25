package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExperienceSearchAndRecord(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("query") != "card retries" || r.URL.Query().Get("kinds") != "pitfall" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(ExperienceSearchResponse{Results: []ExperienceItem{{ID: "e1", Kind: ExperienceKindPitfall}}, Total: 1, Available: true})
		case http.MethodPost:
			var req ExperienceRecordRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Source != "run m-1" {
				t.Fatalf("bad record request: %#v err=%v", req, err)
			}
			_ = json.NewEncoder(w).Encode(ExperienceRecordResponse{
				ID: "e1", Kind: ExperienceKindPitfall, Provenance: "external",
				State: "candidate", ReviewRequired: true,
			})
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithAPIKey("k"))
	got, err := c.Experience().Search(context.Background(), ExperienceSearchRequest{Query: "card retries", Kinds: []string{ExperienceKindPitfall}})
	if err != nil || len(got.Results) != 1 {
		t.Fatalf("search=%#v err=%v", got, err)
	}
	recorded, err := c.Experience().Record(context.Background(), ExperienceRecordRequest{Kind: ExperienceKindPitfall, Text: "use idempotency key", Source: "run m-1"})
	if err != nil || recorded.Provenance != "external" || recorded.State != "candidate" || !recorded.ReviewRequired {
		t.Fatalf("record=%#v err=%v", recorded, err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestExperienceValidatesRequiredFields(t *testing.T) {
	c := NewClient("http://example.invalid")
	if _, err := c.Experience().Search(context.Background(), ExperienceSearchRequest{}); err == nil {
		t.Fatal("empty query accepted")
	}
	if _, err := c.Experience().Record(context.Background(), ExperienceRecordRequest{Text: "x"}); err == nil {
		t.Fatal("empty source accepted")
	}
}

func TestExperienceReviewAutomation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/autotester/context/knowledge/review":
			_ = json.NewEncoder(w).Encode(ExperienceReviewPage{Items: []ExperienceReviewItem{{ID: "k-1", State: "candidate", Version: 1}}, NextCursor: "next"})
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(ExperienceReviewDetail{Item: ExperienceReviewItem{
				Metadata: map[string]string{"instruction": "untrusted"}, ID: "k-1", ContentSHA256: "abc123", State: "candidate", Version: 1,
			}, Relations: []ExperienceReviewRelation{}, History: []ExperienceReviewMutation{}})
		case r.Method == http.MethodPost:
			var req ExperienceReviewRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req.Decision != "reject" || req.ExpectedVersion != 1 || req.IdempotencyKey != "review-1" {
				t.Fatalf("review request = %#v", req)
			}
			_ = json.NewEncoder(w).Encode(ExperienceReviewResponse{Item: ExperienceReviewItem{ID: "k-1", State: "deleted", Version: 2}})
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithAPIKey("k"))
	page, err := c.Experience().ListReview(context.Background(), ExperienceReviewListRequest{Limit: 20})
	if err != nil || len(page.Items) != 1 || page.NextCursor != "next" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	detail, err := c.Experience().GetReview(context.Background(), "k-1")
	if err != nil || detail.Item.ID != "k-1" || detail.Item.ContentSHA256 != "abc123" ||
		detail.Item.Metadata["instruction"] != "untrusted" || detail.Relations == nil || detail.History == nil {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	result, err := c.Experience().Review(context.Background(), "k-1", ExperienceReviewRequest{
		Decision: "reject", ExpectedVersion: 1, Reason: "unsupported", IdempotencyKey: "review-1",
	})
	if err != nil || result.Item.State != "deleted" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestExperienceReviewRejectRefusesPublishModifiers(t *testing.T) {
	c := NewClient("http://example.invalid")
	expires := time.Now().Add(time.Hour)
	_, err := c.Experience().Review(context.Background(), "k-1", ExperienceReviewRequest{
		Decision: "reject", ExpectedVersion: 1, Reason: "unsupported", IdempotencyKey: "review-2",
		ExpiresAt: &expires,
	})
	if err == nil {
		t.Fatal("reject with publication expiry accepted")
	}
}
