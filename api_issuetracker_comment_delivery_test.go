package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

const commentOperationID = "33333333-3333-4333-8333-333333333333"
const commentCaptureID = "44444444-4444-4444-8444-444444444444"

func TestAddCommentDeliveryNeverReplaysAmbiguousPOST(t *testing.T) {
	for _, status := range []int{0, 429, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			transport := &coderReplayTransport{status: status}
			client := NewClient("http://127.0.0.1", WithHTTPClient(&http.Client{Transport: transport}), WithRetry(2, time.Millisecond))
			_, err := client.IssueTracker().AddComment(context.Background(), "sandbox", "issue", "Owned body")
			if err == nil || transport.calls != 1 {
				t.Fatalf("ambiguous comment POST replayed: calls=%d err=%v", transport.calls, err)
			}
		})
	}
}

// Reflection keeps the pre-fix fixture compilable: absent additive error
// metadata must be a behavioral failure, not a missing-field build failure.
func commentErrorField(err *APIError, name string) string {
	v := reflect.ValueOf(err).Elem().FieldByName(name)
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return ""
}

func TestAddCommentDeliveryPreservesUnknownHandles(t *testing.T) {
	for _, code := range []string{"", "unavailable"} {
		t.Run(code, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(503)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "collaboration_capture_outcome_unknown", "state": "unknown", "operation_id": commentOperationID, "capture_id": commentCaptureID, "code": code})
			}))
			defer srv.Close()
			_, err := NewClient(srv.URL, WithRetry(0, 0)).IssueTracker().AddComment(context.Background(), "sandbox", "issue", "Owned body")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected existing APIError: %v", err)
			}
			if commentErrorField(apiErr, "OperationID") != commentOperationID || commentErrorField(apiErr, "CaptureID") != commentCaptureID || commentErrorField(apiErr, "State") != "unknown" {
				t.Fatal("unknown comment error silently discarded exact capture handles")
			}
		})
	}
}

func TestAddCommentDeliveryPendingAndLegacyResult(t *testing.T) {
	for _, status := range []int{201, 202} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				if r.Method != "POST" || r.URL.Path != "/api/v1/namespaces/sandbox/issuetracker/issues/issue/comments" {
					t.Error("wrong comment endpoint")
				}
				var body map[string]string
				if json.NewDecoder(r.Body).Decode(&body) != nil || body["body"] != "Owned body" {
					t.Error("wrong comment intent")
				}
				w.WriteHeader(status)
				if status == 201 {
					fmt.Fprint(w, `{"id":"native-comment","body":"Owned body"}`)
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"state": "pending", "operation_id": commentOperationID, "capture_id": commentCaptureID})
				}
			}))
			defer srv.Close()
			got, err := NewClient(srv.URL, WithRetry(2, time.Millisecond)).IssueTracker().AddComment(context.Background(), "sandbox", "issue", "Owned body")
			if err != nil || hits != 1 {
				t.Fatalf("healthy request: hits=%d err=%v", hits, err)
			}
			if status == 202 && (got["state"] != "pending" || got["operation_id"] != commentOperationID || got["capture_id"] != commentCaptureID || got["id"] != nil) {
				t.Fatal("202 misrepresented as saved native comment")
			}
			if status == 201 && got["id"] != "native-comment" {
				t.Fatal("legacy comment result changed")
			}
		})
	}
}

func TestAddCommentDeliveryInvalidUnknownMetadataHasNoAuthorityFields(t *testing.T) {
	for _, patch := range []map[string]any{{"operation_id": 42}, {"operation_id": " "}, {"capture_id": strings.Repeat("x", 257)}, {"capture_id": "44444444-4444-4444-8444-44444444444A"}, {"state": "pending"}, {"error": "transient"}, {"actor_id": "other"}, {"extra": true}, {"code": "conflict"}} {
		body := map[string]any{"error": "collaboration_capture_outcome_unknown", "state": "unknown", "operation_id": commentOperationID, "capture_id": commentCaptureID}
		for key, value := range patch {
			body[key] = value
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503); _ = json.NewEncoder(w).Encode(body) }))
		_, err := NewClient(srv.URL, WithRetry(0, 0)).IssueTracker().AddComment(context.Background(), "sandbox", "issue", "Owned body")
		srv.Close()
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected existing APIError: %v", err)
		}
		if commentErrorField(apiErr, "OperationID") != "" || commentErrorField(apiErr, "CaptureID") != "" || commentErrorField(apiErr, "State") != "" {
			t.Fatalf("partial malformed metadata retained: %v", patch)
		}
	}
}

func TestAddCommentDeliveryDuplicateUnknownMetadataHasNoAuthorityFields(t *testing.T) {
	body := fmt.Sprintf(`{"error":"collaboration_capture_outcome_unknown","state":"unknown","operation_id":%q,"capture_id":%q,"state":"unknown"}`, commentOperationID, commentCaptureID)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, WithRetry(0, 0)).IssueTracker().AddComment(context.Background(), "sandbox", "issue", "Owned body")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected existing APIError: %v", err)
	}
	if commentErrorField(apiErr, "OperationID") != "" || commentErrorField(apiErr, "CaptureID") != "" || commentErrorField(apiErr, "State") != "" {
		t.Fatal("duplicate metadata retained capture handles")
	}
}

func TestAddCommentDeliveryUnknownEnvelopeBounds(t *testing.T) {
	valid := fmt.Sprintf(`{"error":"collaboration_capture_outcome_unknown","state":"unknown","operation_id":%q,"capture_id":%q}`, commentOperationID, commentCaptureID)
	for _, size := range []int{2048, 2049} {
		body := []byte(valid + strings.Repeat(" ", size-len(valid)))
		op, capture, state := commentCaptureMetadata(body, 503)
		if size == 2048 {
			if op != commentOperationID || capture != commentCaptureID || state != "unknown" {
				t.Fatal("exact boundary discarded valid handles")
			}
		} else if op != "" || capture != "" || state != "" {
			t.Fatal("oversized envelope retained metadata")
		}
	}
	for _, status := range []int{400, 500} {
		if op, capture, state := commentCaptureMetadata([]byte(valid), status); op != "" || capture != "" || state != "" {
			t.Fatal("wrong status retained capture metadata")
		}
	}
	if op, capture, state := commentCaptureMetadata([]byte(`{"error":"collaboration_capture_outcome_unknown"}`), 503); op != "" || capture != "" || state != "" {
		t.Fatal("error string alone retained capture metadata")
	}
}

func FuzzCommentCaptureMetadata(f *testing.F) {
	valid := fmt.Sprintf(`{"error":"collaboration_capture_outcome_unknown","state":"unknown","operation_id":%q,"capture_id":%q}`, commentOperationID, commentCaptureID)
	for _, body := range []string{
		valid, "", "null", "{}", "[]", `{"state":null}`,
		valid[:len(valid)-1] + `,"state":"unknown"}`,
		valid[:len(valid)-1] + `,"code":"unavailable"}`,
		valid[:len(valid)-1] + `,"actor_id":"other"}`,
		strings.Replace(valid, commentOperationID, "00000000-0000-0000-0000-000000000000", 1),
		strings.Repeat("[", 1000) + "0" + strings.Repeat("]", 1000),
		valid + strings.Repeat(" ", 2048-len(valid)),
		valid + strings.Repeat(" ", 2049-len(valid)),
		valid + " null", "\xff",
	} {
		f.Add([]byte(body), 503)
	}
	f.Add([]byte(valid), 500)
	idPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	f.Fuzz(func(t *testing.T, body []byte, status int) {
		op, capture, state := commentCaptureMetadata(body, status)
		if op == "" && capture == "" && state == "" {
			return
		}
		if status != 503 || len(body) > 2048 || state != "unknown" ||
			!idPattern.MatchString(op) || !idPattern.MatchString(capture) ||
			op == "00000000-0000-0000-0000-000000000000" || capture == "00000000-0000-0000-0000-000000000000" {
			t.Fatal("partial or noncanonical capture metadata")
		}
		var decoded map[string]any
		if json.Unmarshal(body, &decoded) != nil || (len(decoded) != 4 && len(decoded) != 5) ||
			decoded["error"] != "collaboration_capture_outcome_unknown" || decoded["state"] != state ||
			decoded["operation_id"] != op || decoded["capture_id"] != capture {
			t.Fatal("metadata does not match exact error envelope")
		}
		if code, exists := decoded["code"]; exists && code != "" && code != "unavailable" {
			t.Fatal("contradictory error code retained handles")
		}
	})
}
