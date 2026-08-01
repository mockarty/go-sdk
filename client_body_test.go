// Copyright (c) 2026 Mockarty. All rights reserved.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDoRaw_RetryReplaysRawByteBody locks the retry fix: a []byte body must be
// replayed verbatim on retry, NOT re-JSON-marshalled (which base64-encoded the
// bytes into a JSON string and corrupted uploads when retries were enabled).
func TestDoRaw_RetryReplaysRawByteBody(t *testing.T) {
	var bodies []string
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 -> retry
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(2, time.Millisecond))
	payload := []byte(`{"raw":"bytes","n":1}`)
	rc, err := c.doRaw(context.Background(), http.MethodPost, "/x", payload)
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	_ = rc.Close()

	if len(bodies) != 2 {
		t.Fatalf("expected 2 attempts (1 fail + 1 retry), got %d", len(bodies))
	}
	for i, b := range bodies {
		if b != string(payload) {
			t.Errorf("attempt %d body = %q, want verbatim %q (retry must replay raw bytes)", i, b, payload)
		}
	}
}

// TestDoRaw_RetryReplaysReaderBody verifies an io.Reader body survives a retry
// (the old code would have JSON-marshalled the already-consumed reader struct).
func TestDoRaw_RetryReplaysReaderBody(t *testing.T) {
	var bodies []string
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(2, time.Millisecond))
	rc, err := c.doRaw(context.Background(), http.MethodPost, "/x", strings.NewReader("STREAMBODY"))
	if err != nil {
		t.Fatalf("doRaw: %v", err)
	}
	_ = rc.Close()
	if len(bodies) != 2 || bodies[0] != "STREAMBODY" || bodies[1] != "STREAMBODY" {
		t.Errorf("reader body not replayed across retries: %#v", bodies)
	}
}

// TestUploadAttachment_MultipartContentType locks the multipart fix: the server
// uses c.FormFile("file"), which requires a multipart/form-data Content-Type
// with a boundary. Previously doRaw forced application/json -> ParseMultipartForm
// failed on the first attempt. This test mirrors the server's parsing exactly.
func TestUploadAttachment_MultipartContentType(t *testing.T) {
	var gotCT, gotFileName, gotFileBody, gotMediaType string
	var parseErr string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			parseErr = err.Error()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"parse multipart"}`))
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			parseErr = err.Error()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"form file"}`))
			return
		}
		defer f.Close()
		data, _ := io.ReadAll(f)
		gotFileName = hdr.Filename
		gotFileBody = string(data)
		gotMediaType = hdr.Header.Get("Content-Type")
		_ = json.NewEncoder(w).Encode(TCMAttachment{ID: "att-1", OriginalName: hdr.Filename})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("ns"))
	att, err := c.TCM().UploadAttachment(context.Background(), "ns", "test_case", "tc-1",
		"shot.png", "image/png", strings.NewReader("PNGDATA"))
	if err != nil {
		t.Fatalf("UploadAttachment: %v (server parse error: %q)", err, parseErr)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %q, want multipart/form-data; boundary=...", gotCT)
	}
	if gotFileName != "shot.png" || gotFileBody != "PNGDATA" || gotMediaType != "image/png" {
		t.Errorf("server parsed file = (name=%q body=%q media=%q)", gotFileName, gotFileBody, gotMediaType)
	}
	if att == nil || att.ID != "att-1" {
		t.Errorf("UploadAttachment returned %+v", att)
	}
}
