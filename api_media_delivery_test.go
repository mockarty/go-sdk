package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMediaDeliveryAPIUsesFencedEndpoints(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		if r.Method == http.MethodPost {
			var body MediaDeliveryReconcileRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RunnerID != "runner-1" || body.Outcome != "not_started" {
				t.Fatalf("body = %+v, err=%v", body, err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"fenced":[],"count":0}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, WithNamespace("ns one"))
	if _, err := client.MediaDelivery().ListFenced(context.Background(), "transcribe"); err != nil {
		t.Fatal(err)
	}
	if err := client.MediaDelivery().Reconcile(context.Background(), "tts", "job/1", "runner-1", "not_started"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"GET /api/v1/transcribe/jobs/fenced?namespace=ns+one",
		"POST /api/v1/tts/jobs/job%2F1/reconcile-delivery?namespace=ns+one",
	}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
