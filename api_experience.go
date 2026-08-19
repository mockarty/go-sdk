// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Provenance string `json:"provenance"`
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
