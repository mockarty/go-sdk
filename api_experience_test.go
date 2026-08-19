package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
			_ = json.NewEncoder(w).Encode(ExperienceRecordResponse{ID: "e1", Kind: ExperienceKindPitfall, Provenance: "external"})
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithAPIKey("k"))
	got, err := c.Experience().Search(context.Background(), ExperienceSearchRequest{Query: "card retries", Kinds: []string{ExperienceKindPitfall}})
	if err != nil || len(got.Results) != 1 {
		t.Fatalf("search=%#v err=%v", got, err)
	}
	recorded, err := c.Experience().Record(context.Background(), ExperienceRecordRequest{Kind: ExperienceKindPitfall, Text: "use idempotency key", Source: "run m-1"})
	if err != nil || recorded.Provenance != "external" {
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
