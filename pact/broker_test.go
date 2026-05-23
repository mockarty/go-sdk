// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package pact

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const fakePact = `{
  "consumer": {"name": "OrderClient"},
  "provider": {"name": "OrderAPI"},
  "interactions": [],
  "metadata": {"pactSpecification": {"version": "4.0"}}
}`

func TestNewBrokerClient_RequiresURL(t *testing.T) {
	t.Setenv("PACT_BROKER_BASE_URL", "")
	if _, err := NewBrokerClient(); err == nil {
		t.Fatal("expected error when no base URL set")
	}
}

func TestNewBrokerClient_ReadsEnvVars(t *testing.T) {
	t.Setenv("PACT_BROKER_BASE_URL", "https://pact.example.com/")
	t.Setenv("PACT_BROKER_TOKEN", "token-abc")
	c, err := NewBrokerClient()
	if err != nil {
		t.Fatalf("NewBrokerClient: %v", err)
	}
	if c.baseURL != "https://pact.example.com" {
		t.Fatalf("trailing slash not stripped: %q", c.baseURL)
	}
	if c.token != "token-abc" {
		t.Fatalf("token = %q", c.token)
	}
}

func TestBrokerClient_Publish_HappyPath(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	c, err := NewBrokerClient(WithBrokerURL(srv.URL), WithBrokerToken("tok-42"))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if err := c.Publish(context.Background(), []byte(fakePact), "1.0.0", "", nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	wantPath := "/pacts/provider/OrderAPI/consumer/OrderClient/version/1.0.0"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotAuth != "Bearer tok-42" {
		t.Errorf("auth = %q (Bearer not used)", gotAuth)
	}
	// Body round-trips byte-for-byte.
	if string(gotBody) != fakePact {
		t.Errorf("body not forwarded: %d bytes", len(gotBody))
	}
}

func TestBrokerClient_Publish_BasicAuthFallback(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL),
		WithBrokerBasicAuth("ci-bot", "s3cret"))
	if err := c.Publish(context.Background(), []byte(fakePact), "1.0", "", nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("ci-bot:s3cret"))
	if gotAuth != want {
		t.Errorf("auth = %q, want %q", gotAuth, want)
	}
}

func TestBrokerClient_Publish_TokenWinsOverBasic(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL),
		WithBrokerToken("tok-x"),
		WithBrokerBasicAuth("u", "p"))
	_ = c.Publish(context.Background(), []byte(fakePact), "1.0", "", nil)
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("Bearer should win over Basic: %q", gotAuth)
	}
}

func TestBrokerClient_Publish_Tags(t *testing.T) {
	var seenTagPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tags/") {
			seenTagPaths = append(seenTagPaths, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	err := c.Publish(context.Background(), []byte(fakePact), "v2", "", []string{"prod", "stable"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(seenTagPaths) != 2 {
		t.Fatalf("expected 2 tag PUTs, got %d: %v", len(seenTagPaths), seenTagPaths)
	}
	wantContains := []string{"/tags/prod", "/tags/stable"}
	for i, want := range wantContains {
		if !strings.HasSuffix(seenTagPaths[i], want) {
			t.Errorf("tag path[%d] = %q, want suffix %q", i, seenTagPaths[i], want)
		}
	}
}

func TestBrokerClient_Publish_PartialTagFailure(t *testing.T) {
	// Pact PUT succeeds; the 2nd tag PUT returns 500; the 3rd tag MUST
	// still be attempted (no short-circuit). Final error must surface
	// the partial-failure shape, not the first failing tag alone.
	var tagsSeen int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/tags/") {
			mu.Lock()
			tagsSeen++
			n := tagsSeen
			mu.Unlock()
			if n == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	err := c.Publish(context.Background(), []byte(fakePact), "v1", "",
		[]string{"prod", "stable", "ci"})
	if err == nil {
		t.Fatal("expected aggregate error for failing tag")
	}
	if !strings.Contains(err.Error(), "publish succeeded but") {
		t.Errorf("error should mention publish-succeeded-but-tags-failed: %v", err)
	}
	mu.Lock()
	got := tagsSeen
	mu.Unlock()
	if got != 3 {
		t.Errorf("expected 3 tag attempts (no short-circuit); got %d", got)
	}
}

func TestBrokerClient_Publish_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors": ["bad pact"]}`))
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	err := c.Publish(context.Background(), []byte(fakePact), "1.0", "", nil)
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error lacks status: %v", err)
	}
	if !strings.Contains(err.Error(), "bad pact") {
		t.Errorf("error body not surfaced: %v", err)
	}
}

func TestBrokerClient_Publish_MalformedPactRejected(t *testing.T) {
	c, _ := NewBrokerClient(WithBrokerURL("http://x"))
	if err := c.Publish(context.Background(), []byte("not-json"), "1.0", "", nil); err == nil {
		t.Fatal("expected parse error on garbage JSON")
	}
	if err := c.Publish(context.Background(), []byte(`{}`), "1.0", "", nil); err == nil {
		t.Fatal("expected error when consumer.name absent")
	}
}

func TestBrokerClient_Publish_EmptyVersionRejected(t *testing.T) {
	c, _ := NewBrokerClient(WithBrokerURL("http://x"))
	if err := c.Publish(context.Background(), []byte(fakePact), "", "", nil); err == nil {
		t.Fatal("expected error on empty consumer version")
	}
}

func TestBrokerClient_Fetch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakePact))
		_ = r
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	body, err := c.Fetch(context.Background(), "OrderClient", "OrderAPI", "1.0")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !strings.Contains(string(body), "OrderAPI") {
		t.Errorf("body lacks expected content")
	}
}

func TestBrokerClient_FetchLatest_PathConvention(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(fakePact))
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	if _, err := c.FetchLatest(context.Background(), "X", "Y"); err != nil {
		t.Fatalf("fetch latest: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/version/latest") {
		t.Errorf("FetchLatest didn't use /version/latest: %q", gotPath)
	}
}

func TestBrokerClient_Fetch_NotFoundReturnsSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	_, err := c.Fetch(context.Background(), "X", "Y", "1.0")
	if !errors.Is(err, ErrBrokerPactNotFound) {
		t.Fatalf("expected ErrBrokerPactNotFound, got %v", err)
	}
}

func TestBrokerClient_CanIDeploy_Deployable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters.
		q := r.URL.Query()
		if q.Get("pacticipant") != "OrderClient" {
			t.Errorf("pacticipant = %q", q.Get("pacticipant"))
		}
		if q.Get("environment") != "prod" {
			t.Errorf("environment = %q", q.Get("environment"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"summary": map[string]any{
				"deployable": true,
				"reason":     "all verified",
			},
		})
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	res, err := c.CanIDeploy(context.Background(), "OrderClient", "1.0.0", "prod")
	if err != nil {
		t.Fatalf("can-i-deploy: %v", err)
	}
	if !res.Deployable {
		t.Fatal("expected deployable=true")
	}
	if res.Reason != "all verified" {
		t.Errorf("reason = %q", res.Reason)
	}
	if len(res.Raw) == 0 {
		t.Error("raw body not preserved")
	}
}

func TestBrokerClient_CanIDeploy_NotDeployable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"summary": map[string]any{"deployable": false, "reason": "OrderAPI not verified"},
		})
	}))
	t.Cleanup(srv.Close)

	c, _ := NewBrokerClient(WithBrokerURL(srv.URL))
	res, err := c.CanIDeploy(context.Background(), "X", "1.0", "")
	if err != nil {
		t.Fatalf("can-i-deploy: %v", err)
	}
	if res.Deployable {
		t.Fatal("expected deployable=false")
	}
}

func TestBrokerClient_CanIDeploy_Required(t *testing.T) {
	c, _ := NewBrokerClient(WithBrokerURL("http://x"))
	if _, err := c.CanIDeploy(context.Background(), "", "1.0", ""); err == nil {
		t.Fatal("expected error on empty pacticipant")
	}
	if _, err := c.CanIDeploy(context.Background(), "X", "", ""); err == nil {
		t.Fatal("expected error on empty version")
	}
}

func TestExtractConsumerProvider_Errors(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{`{"consumer":{"name":"a"},"provider":{"name":"b"}}`, false},
		{`{"consumer":{},"provider":{"name":"b"}}`, true},
		{`{"consumer":{"name":"a"},"provider":{}}`, true},
		{`not-json`, true},
		{`{}`, true},
	}
	for _, tc := range cases {
		_, _, err := extractConsumerProvider([]byte(tc.in))
		if (err != nil) != tc.wantErr {
			t.Errorf("input=%q: err=%v wantErr=%v", tc.in, err, tc.wantErr)
		}
	}
}

// FuzzBrokerExtractConsumerProvider — never panic on adversarial
// pact JSON. The Publish hot-path calls this on every upload; a
// panic = admin CI outage.
func FuzzBrokerExtractConsumerProvider(f *testing.F) {
	seeds := []string{
		``,
		`{}`,
		`{"consumer":{"name":"a"},"provider":{"name":"b"}}`,
		`{"consumer":null,"provider":null}`,
		`{"consumer":{"name":""},"provider":{"name":""}}`,
		`[{"consumer":1}]`,
		`{"consumer":{"name":"\x00"},"provider":{"name":"\u0000"}}`,
		`{"consumer":{"name":` + strings.Repeat("A", 10*1024) + `},"provider":{"name":"b"}}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("extractConsumerProvider panicked on raw=%q: %v", string(raw), r)
			}
		}()
		_, _, _ = extractConsumerProvider(raw)
	})
}
