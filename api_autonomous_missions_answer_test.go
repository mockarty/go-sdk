// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMissionAnswerCapturedIdentityWire(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v1/missions/M1/answer" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["questionId"] != "question-A" || body["revision"] != float64(2) || body["generation"] != float64(3) || body["text"] != "answer A" || body["idempotencyKey"] != "retry-A" {
			t.Errorf("lost captured identity: %#v", body)
		}
		if _, ok := body["answer"]; ok {
			t.Errorf("typed payload mixed legacy answer: %#v", body)
		}
		if body["entity"].(map[string]any)["namespace"] != "team-A" || body["selected"].([]any)[0] != "staging" {
			t.Errorf("lost choice scope: %#v", body)
		}
		_, _ = w.Write([]byte(`{"mission":{"id":"M1"},"control":{"action":"answer"}}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL).AutonomousMissions().Answer(context.Background(), "M1", MissionAnswerRequest{
		QuestionID: "question-A", Revision: 2, Generation: 3, Text: "answer A", Selected: []string{"staging"},
		Entity: &MissionAnswerEntity{Kind: "environment", ID: "env-A", Namespace: "team-A"}, IdempotencyKey: "retry-A",
	})
	if err != nil || calls != 1 {
		t.Fatalf("answer err=%v calls=%d", err, calls)
	}
}

func TestMissionAnswerCapturedIdentityValidation(t *testing.T) {
	for _, request := range []MissionAnswerRequest{
		{Text: "missing identity"}, {QuestionID: "A", Text: "missing revision"},
		{QuestionID: "A", Revision: 1, Generation: -1, Text: "negative generation"},
		{QuestionID: "A", Revision: 1, Text: "typed", Answer: "legacy"},
		{QuestionID: "A", Revision: 1},
		{QuestionID: "A", Revision: 1, Selected: []string{"one", " one "}},
		{QuestionID: "A", Revision: 1, Selected: make([]string, 33)},
		{QuestionID: "A", Revision: 1, Text: strings.Repeat("я", 4001)},
		{QuestionID: "A", Revision: 1, Entity: &MissionAnswerEntity{Kind: "environment"}},
	} {
		if err := validateMissionAnswer(request); err == nil {
			t.Errorf("accepted invalid answer: %+v", request)
		}
	}
	for _, request := range []MissionAnswerRequest{{Answer: "legacy"}, {QuestionID: "A", Revision: 1, Selected: []string{"one"}}, {QuestionID: "A", Revision: 1, Text: strings.Repeat("я", 4000)}} {
		if err := validateMissionAnswer(request); err != nil {
			t.Errorf("valid answer rejected: %v", err)
		}
	}
}
