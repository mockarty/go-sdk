// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ExperienceKindMissionLesson   = "mission_lesson"
	ExperienceKindPitfall         = "pitfall"
	ExperienceKindProductFact     = "product_fact"
	ExperienceKindDefectRootCause = "defect_root_cause"
)

type ExperienceAPI struct{ client *Client }

type ExperienceItem struct {
	Metadata   map[string]string `json:"metadata,omitempty"`
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Title      string            `json:"title"`
	Text       string            `json:"text"`
	Source     string            `json:"source"`
	Provenance string            `json:"provenance"`
	MissionID  string            `json:"missionId,omitempty"`
	EventSeq   int64             `json:"eventSeq,omitempty"`
	Score      float64           `json:"score"`
}

type ExperienceSearchRequest struct {
	Query    string
	Kinds    []string
	MinTrust string
	Limit    int
}

type ExperienceSearchResponse struct {
	Results   []ExperienceItem `json:"results"`
	Engine    string           `json:"engine"`
	Total     int              `json:"total"`
	Available bool             `json:"available"`
}

type ExperienceRecordRequest struct {
	Metadata  map[string]string `json:"metadata,omitempty"`
	Kind      string            `json:"kind,omitempty"`
	Title     string            `json:"title,omitempty"`
	Text      string            `json:"text"`
	Source    string            `json:"source"`
	MissionID string            `json:"missionId,omitempty"`
	EventSeq  int64             `json:"eventSeq,omitempty"`
}

type ExperienceRecordResponse struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Provenance     string `json:"provenance"`
	State          string `json:"state"`
	ReviewRequired bool   `json:"reviewRequired"`
}

type ExperienceReviewItem struct {
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	PublishedAt      *time.Time        `json:"publishedAt,omitempty"`
	ExpiresAt        *time.Time        `json:"expiresAt,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Content          string            `json:"content"`
	ContentSHA256    string            `json:"contentSha256"`
	Kind             string            `json:"kind"`
	Source           string            `json:"source"`
	MissionID        string            `json:"missionId,omitempty"`
	Provenance       string            `json:"provenance"`
	State            string            `json:"state"`
	EventSeq         int64             `json:"eventSeq,omitempty"`
	Version          int64             `json:"version"`
	Confidence       float64           `json:"confidence"`
	ContentTruncated bool              `json:"contentTruncated"`
}

type ExperienceReviewListRequest struct {
	Cursor string
	State  string
	Limit  int
}

type ExperienceReviewPage struct {
	Items      []ExperienceReviewItem `json:"items"`
	NextCursor string                 `json:"nextCursor"`
}

type ExperienceReviewRelation struct {
	RecordID string `json:"recordId"`
	Type     string `json:"type"`
	Outgoing bool   `json:"outgoing"`
}

type ExperienceReviewMutation struct {
	CreatedAt time.Time `json:"createdAt"`
	Operation string    `json:"operation"`
	Actor     string    `json:"actor"`
	Reason    string    `json:"reason"`
	Version   int64     `json:"version"`
}

type ExperienceReviewDetail struct {
	Item      ExperienceReviewItem       `json:"item"`
	Relations []ExperienceReviewRelation `json:"relations"`
	History   []ExperienceReviewMutation `json:"history"`
}

type ExperienceReviewRequest struct {
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	ContradictsIDs  []string   `json:"contradictsIds,omitempty"`
	IdempotencyKey  string     `json:"idempotencyKey"`
	SupersedesID    string     `json:"supersedesId,omitempty"`
	Decision        string     `json:"decision"`
	Reason          string     `json:"reason"`
	ExpectedVersion int64      `json:"expectedVersion"`
}

type ExperienceReviewResponse struct {
	Item ExperienceReviewItem `json:"item"`
}

func (a *ExperienceAPI) Search(ctx context.Context, req ExperienceSearchRequest) (ExperienceSearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return ExperienceSearchResponse{}, fmt.Errorf("mockarty: experience search: query is required")
	}
	q := url.Values{"query": []string{query}}
	if len(req.Kinds) > 0 {
		q.Set("kinds", strings.Join(req.Kinds, ","))
	}
	if req.MinTrust != "" {
		q.Set("minTrust", req.MinTrust)
	}
	if req.Limit > 0 {
		q.Set("k", intToString(req.Limit))
	}
	var out ExperienceSearchResponse
	if err := a.client.do(ctx, http.MethodGet, "/api/v1/autotester/context/knowledge/search?"+q.Encode(), nil, &out); err != nil {
		return ExperienceSearchResponse{}, err
	}
	if out.Results == nil {
		out.Results = []ExperienceItem{}
	}
	return out, nil
}

func (a *ExperienceAPI) Record(ctx context.Context, req ExperienceRecordRequest) (ExperienceRecordResponse, error) {
	if strings.TrimSpace(req.Text) == "" {
		return ExperienceRecordResponse{}, fmt.Errorf("mockarty: experience record: text is required")
	}
	if strings.TrimSpace(req.Source) == "" {
		return ExperienceRecordResponse{}, fmt.Errorf("mockarty: experience record: source is required")
	}
	var out ExperienceRecordResponse
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/autotester/context/knowledge", req, &out); err != nil {
		return ExperienceRecordResponse{}, err
	}
	return out, nil
}

func (a *ExperienceAPI) ListReview(ctx context.Context, req ExperienceReviewListRequest) (ExperienceReviewPage, error) {
	q := url.Values{}
	state := strings.TrimSpace(req.State)
	if state == "" {
		state = "candidate"
	}
	q.Set("state", state)
	if req.Limit > 0 {
		q.Set("limit", intToString(req.Limit))
	}
	if req.Cursor != "" {
		q.Set("cursor", req.Cursor)
	}
	var out ExperienceReviewPage
	if err := a.client.do(ctx, http.MethodGet, "/api/v1/autotester/context/knowledge/review?"+q.Encode(), nil, &out); err != nil {
		return ExperienceReviewPage{}, err
	}
	if out.Items == nil {
		out.Items = []ExperienceReviewItem{}
	}
	return out, nil
}

func (a *ExperienceAPI) GetReview(ctx context.Context, id string) (ExperienceReviewDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ExperienceReviewDetail{}, fmt.Errorf("mockarty: experience review: id is required")
	}
	var out ExperienceReviewDetail
	if err := a.client.do(ctx, http.MethodGet, "/api/v1/autotester/context/knowledge/review/"+url.PathEscape(id), nil, &out); err != nil {
		return ExperienceReviewDetail{}, err
	}
	if out.Relations == nil {
		out.Relations = []ExperienceReviewRelation{}
	}
	if out.History == nil {
		out.History = []ExperienceReviewMutation{}
	}
	return out, nil
}

func (a *ExperienceAPI) Review(ctx context.Context, id string, req ExperienceReviewRequest) (ExperienceReviewResponse, error) {
	id = strings.TrimSpace(id)
	req.Decision = strings.TrimSpace(req.Decision)
	req.Reason = strings.TrimSpace(req.Reason)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if id == "" || req.ExpectedVersion <= 0 || req.Reason == "" || req.IdempotencyKey == "" {
		return ExperienceReviewResponse{}, fmt.Errorf("mockarty: experience review: id, expected version, reason, and idempotency key are required")
	}
	if req.Decision != "publish" && req.Decision != "reject" {
		return ExperienceReviewResponse{}, fmt.Errorf("mockarty: experience review: decision must be publish or reject")
	}
	if req.Decision == "reject" && (req.ExpiresAt != nil || strings.TrimSpace(req.SupersedesID) != "" || len(req.ContradictsIDs) > 0) {
		return ExperienceReviewResponse{}, fmt.Errorf("mockarty: experience review: expiry and relations apply only to publish")
	}
	var out ExperienceReviewResponse
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/autotester/context/knowledge/review/"+url.PathEscape(id), req, &out); err != nil {
		return ExperienceReviewResponse{}, err
	}
	return out, nil
}
