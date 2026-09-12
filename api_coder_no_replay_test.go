package mockarty

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type coderReplayTransport struct {
	status int
	calls  int
}

func (r *coderReplayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	if r.status == 0 {
		return nil, errors.New("ambiguous transport failure")
	}
	return &http.Response{StatusCode: r.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"unavailable"}`)), Request: req}, nil
}

func TestCoderNonIdempotentOperationsAreNeverAutomaticallyReplayed(t *testing.T) {
	for _, status := range []int{0, 429, 502, 503, 504} {
		for _, upload := range []bool{false, true} {
			transport := &coderReplayTransport{status: status}
			client := NewClient("http://127.0.0.1", WithHTTPClient(&http.Client{Transport: transport}), WithRetry(2, 0))
			var err error
			if upload {
				_, err = client.CoderDelivery().UploadMissionMaterial(context.Background(), "team", "product", "a.txt", "text/plain", strings.NewReader("content"))
			} else {
				_, err = client.CoderDelivery().AddToMission(context.Background(), "mission", CoderMissionAddRequest{Prompts: []string{"test"}})
			}
			if err == nil || transport.calls != 1 {
				t.Errorf("status=%d upload=%v calls=%d err=%v", status, upload, transport.calls, err)
			}
		}
	}
}

func TestCoderNonReplayableOperationsRejectNilContext(t *testing.T) {
	api := NewClient("http://127.0.0.1:1").CoderDelivery()
	if _, err := api.AddToMission(nil, "mission", CoderMissionAddRequest{Prompts: []string{"test"}}); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := api.UploadMissionMaterial(nil, "team", "p", "a.txt", "text/plain", strings.NewReader("content")); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestRequestRetryPolicyRejectsNilContextWithoutPanic(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	if _, err := client.doRaw(nil, http.MethodGet, "/health", nil); err == nil {
		t.Fatal("nil context accepted")
	}
}
