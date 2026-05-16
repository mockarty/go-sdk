package externalruns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// FuzzRunResponseDecode pumps arbitrary bytes through the JSON response
// decoder for a Run envelope. CLAUDE.md mandates fuzz coverage for any
// external-bytes decoder; ExternalRuns parses untrusted server replies.
//
// The test passes as long as the decoder does not panic. Returning an
// error is fine — that's what malformed input is supposed to do.
func FuzzRunResponseDecode(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"id":"r1","status":"passed","schema_version":1}`),
		[]byte(`{}`),
		[]byte(`{"id":""}`),
		[]byte(`{"environment":{"a":"b"}}`),
		[]byte(`{"steps":[{"step_key":"k","status":"failed"}]}`),
		[]byte(`{"started_at":"2026-05-16T10:00:00Z"}`),
		[]byte("{\"id\":\"\x00\xff\"}"),
		[]byte(`{"attachments":[{"id":"a","size_bytes":9223372036854775807}]}`),
		[]byte(`[]`),
		[]byte(`null`),
		[]byte(`"not an object"`),
		[]byte(``),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		var r Run
		_ = json.Unmarshal(payload, &r)
		var rl RunList
		_ = json.Unmarshal(payload, &rl)
	})
}

// FuzzAPIErrorDecode fuzzes the error-envelope path. The decoder must
// tolerate arbitrary non-2xx payloads without panicking — this includes
// HTML pages from reverse proxies and partial JSON from k8s sidecars.
func FuzzAPIErrorDecode(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"error":"x","code":"X"}`),
		[]byte(`{"message":"x"}`),
		[]byte(`<html>oops</html>`),
		[]byte(``),
		[]byte(`{`),
		[]byte(`{"error":null}`),
		[]byte(`{"error":["array","not","string"]}`),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, payload []byte) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(payload)
		}))
		defer srv.Close()

		c, err := NewClient(srv.URL, "ns", "tok")
		if err != nil {
			t.Skip()
		}
		_, _ = c.GetRun(context.Background(), "r1")
	})
}
