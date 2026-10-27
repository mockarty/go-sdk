// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// These public transport bounds do not replace current server question scope
// validation. Never fetch a different question to complete a missing identity.
func validateMissionAnswer(req MissionAnswerRequest) error {
	fail := func(message string) error { return fmt.Errorf("mockarty: mission answer: %s", message) }
	typed := req.QuestionID != "" || req.Revision != 0 || req.Generation != 0 || req.Text != "" || req.Selected != nil || req.Entity != nil
	if !typed {
		if strings.TrimSpace(req.Answer) == "" {
			return fail("answer or captured typed question answer is required")
		}
		return nil
	}
	if strings.TrimSpace(req.Answer) != "" {
		return fail("do not mix legacy answer with typed fields")
	}
	valid := func(value string, max int, required bool) bool {
		value = strings.TrimSpace(value)
		return (!required || value != "") && utf8.ValidString(value) && len(value) <= max
	}
	if !valid(req.QuestionID, 128, true) || req.Revision <= 0 || req.Generation < 0 {
		return fail("captured questionId, positive revision and non-negative generation are required")
	}
	if strings.TrimSpace(req.Text) == "" && len(req.Selected) == 0 && req.Entity == nil {
		return fail("typed answer is empty")
	}
	if !utf8.ValidString(req.Text) || utf8.RuneCountInString(req.Text) > 4000 {
		return fail("text exceeds 4000 characters or is invalid UTF-8")
	}
	for _, r := range req.Text {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fail("text contains control characters")
		}
	}
	if len(req.Selected) > 32 {
		return fail("at most 32 selections are supported")
	}
	seen := make(map[string]bool, len(req.Selected))
	for _, raw := range req.Selected {
		value := strings.TrimSpace(raw)
		if !valid(value, 128, true) || seen[value] {
			return fail("selection is empty, duplicated or exceeds 128 bytes")
		}
		seen[value] = true
	}
	if e := req.Entity; e != nil && (!valid(e.Kind, 64, true) || !valid(e.ID, 256, true) || !valid(e.Namespace, 256, false)) {
		return fail("entity requires bounded kind/id and optional namespace")
	}
	return nil
}
