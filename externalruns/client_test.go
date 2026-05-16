package externalruns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient wires a Client at the given test server's URL with a
// sensible default namespace + token.
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(srv.URL, "team-alpha", "tok-abc", WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// requireHeaders fails the test if the auth/schema/UA headers are not
// set on the inbound request. Centralised so every endpoint test stays
// honest about the wire contract.
func requireHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get(AuthHeader); got != "tok-abc" {
		t.Errorf("missing/wrong %s header: %q", AuthHeader, got)
	}
	if got := r.Header.Get(SchemaVersionHeader); got != strconv.Itoa(SchemaVersion) {
		t.Errorf("missing/wrong %s header: %q (want %d)", SchemaVersionHeader, got, SchemaVersion)
	}
	if got := r.Header.Get("User-Agent"); got == "" {
		t.Errorf("missing User-Agent header")
	}
}

func TestNewClient_validation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		baseURL, ns, tok  string
		wantErr           error
		wantMessageSubstr string
	}{
		{"empty baseURL", "", "ns", "t", ErrInvalidConfig, "required"},
		{"empty namespace", "https://x.test", "", "t", ErrInvalidConfig, "required"},
		{"empty token", "https://x.test", "ns", "", ErrInvalidConfig, "required"},
		{"bad scheme", "ftp://x.test", "ns", "t", ErrInvalidConfig, "scheme"},
		{"no host", "https://", "ns", "t", ErrInvalidConfig, "host"},
		{"unparseable", "://", "ns", "t", ErrInvalidConfig, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.baseURL, tc.ns, tc.tok)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want is %v", err, tc.wantErr)
			}
			if tc.wantMessageSubstr != "" && !strings.Contains(err.Error(), tc.wantMessageSubstr) {
				t.Fatalf("err message %q lacks substring %q", err.Error(), tc.wantMessageSubstr)
			}
		})
	}
}

func TestNewClient_optionsApply(t *testing.T) {
	t.Parallel()
	custom := &http.Client{Timeout: 9 * time.Second}
	c, err := NewClient("https://x.test", "ns", "tok",
		WithHTTPClient(custom),
		WithUserAgent("test-agent/1.0"),
		WithTimeout(5*time.Second), // ignored because custom client supplied
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.httpClient != custom {
		t.Errorf("WithHTTPClient did not stick")
	}
	if c.userAgent != "test-agent/1.0" {
		t.Errorf("WithUserAgent did not stick: %q", c.userAgent)
	}
	// WithTimeout mutates the now-injected custom client.
	if c.httpClient.Timeout != 5*time.Second {
		t.Errorf("WithTimeout did not apply to injected client: %v", c.httpClient.Timeout)
	}
	if c.Namespace() != "ns" {
		t.Errorf("Namespace() = %q", c.Namespace())
	}
	if !strings.HasPrefix(c.BaseURL(), "https://x.test") {
		t.Errorf("BaseURL = %q", c.BaseURL())
	}
}

func TestNewClient_stripsTrailingSlash(t *testing.T) {
	t.Parallel()
	c, err := NewClient("https://x.test/api///", "ns", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(c.BaseURL(), "/") {
		t.Errorf("BaseURL should not end with slash: %q", c.BaseURL())
	}
}

func TestCreateRun_validates(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")
	if _, err := c.CreateRun(context.Background(), CreateRunRequest{Framework: "go"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for empty name, got %v", err)
	}
	if _, err := c.CreateRun(context.Background(), CreateRunRequest{Name: "x"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest for empty framework, got %v", err)
	}
}

func TestCreateRun_happyPath(t *testing.T) {
	t.Parallel()
	var gotBody CreateRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/namespaces/team-alpha/tcm/external-runs" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Run{
			ID:        "run-1",
			Namespace: "team-alpha",
			Name:      gotBody.Name,
			Framework: gotBody.Framework,
			Status:    StatusRunning,
			SchemaVer: SchemaVersion,
			StartedAt: time.Unix(1700000000, 0).UTC(),
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	run, err := c.CreateRun(context.Background(), CreateRunRequest{
		Name:        "nightly suite",
		Framework:   "go-test",
		Tags:        []string{"smoke", "nightly"},
		Environment: map[string]string{"ci": "github", "sha": "abc123"},
		ExternalID:  "build-42",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.ID != "run-1" {
		t.Errorf("ID = %q", run.ID)
	}
	if gotBody.Name != "nightly suite" || gotBody.Framework != "go-test" {
		t.Errorf("server saw %+v", gotBody)
	}
	if len(gotBody.Tags) != 2 || gotBody.Tags[0] != "smoke" {
		t.Errorf("tags lost: %v", gotBody.Tags)
	}
}

func TestCreateRun_namespaceURLEscaping(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Run{ID: "run-z"})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "team alpha/beta", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateRun(context.Background(), CreateRunRequest{Name: "n", Framework: "f"}); err != nil {
		t.Fatal(err)
	}
	// "team alpha/beta" must be percent-encoded: space -> %20, slash -> %2F.
	if !strings.Contains(gotPath, "team%20alpha%2Fbeta") {
		t.Errorf("namespace not escaped in path: %q", gotPath)
	}
}

func TestAddSteps_emptyIsNoop(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called for empty steps; got %s", r.URL.Path)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.AddSteps(context.Background(), "run-1", nil); err != nil {
		t.Errorf("empty AddSteps should return nil, got %v", err)
	}
}

func TestAddSteps_happyPath(t *testing.T) {
	t.Parallel()
	type stepsBody struct {
		Steps []Step `json:"steps"`
	}
	var got stepsBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		if r.URL.Path != "/api/v1/namespaces/team-alpha/tcm/external-runs/run-1/steps" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	steps := []Step{
		{StepKey: "login", Name: "login", Status: StatusPassed, DurationMS: 12},
		{StepKey: "checkout", Name: "checkout", Status: StatusFailed, Message: "boom"},
	}
	if err := c.AddSteps(context.Background(), "run-1", steps); err != nil {
		t.Fatalf("AddSteps: %v", err)
	}
	if len(got.Steps) != 2 || got.Steps[1].Message != "boom" {
		t.Fatalf("server saw %+v", got)
	}
}

func TestAddSteps_validates(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")

	if err := c.AddSteps(context.Background(), "", []Step{{StepKey: "k"}}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("blank runID should error, got %v", err)
	}
	if err := c.AddSteps(context.Background(), "r", []Step{{Name: "no key"}}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("missing step key should error, got %v", err)
	}
	if err := c.AddSteps(context.Background(), "r", []Step{{StepKey: "k", Status: "weird"}}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("invalid status should error, got %v", err)
	}
}

func TestAttachReport_multipartShape(t *testing.T) {
	t.Parallel()
	var gotFilename, gotMime, gotField string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		if r.URL.Path != "/api/v1/namespaces/team-alpha/tcm/external-runs/run-1/attachments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		files := r.MultipartForm.File["file"]
		if len(files) != 1 {
			t.Fatalf("file count = %d", len(files))
		}
		gotFilename = files[0].Filename
		gotMime = files[0].Header.Get("Content-Type")
		f, err := files[0].Open()
		if err != nil {
			t.Fatalf("open part: %v", err)
		}
		gotBody, _ = io.ReadAll(f)
		_ = f.Close()
		gotField = r.FormValue("name")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	payload := []byte(`{"hello":"world"}`)
	if err := c.AttachReport(context.Background(), "run-1", "report.json", payload, "application/json"); err != nil {
		t.Fatalf("AttachReport: %v", err)
	}
	if gotFilename != "report.json" {
		t.Errorf("filename = %q", gotFilename)
	}
	if gotMime != "application/json" {
		t.Errorf("mime = %q", gotMime)
	}
	if gotField != "report.json" {
		t.Errorf("name field = %q", gotField)
	}
	if !bytes.Equal(gotBody, payload) {
		t.Errorf("body mismatch")
	}
}

func TestAttachReport_defaultMime(t *testing.T) {
	t.Parallel()
	var gotMime string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		files := r.MultipartForm.File["file"]
		gotMime = files[0].Header.Get("Content-Type")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.AttachReport(context.Background(), "run-1", "blob.bin", []byte("data"), ""); err != nil {
		t.Fatal(err)
	}
	if gotMime != "application/octet-stream" {
		t.Errorf("default mime = %q", gotMime)
	}
}

func TestAttachReport_validates(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")
	if err := c.AttachReport(context.Background(), "", "n", []byte("x"), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("blank runID: %v", err)
	}
	if err := c.AttachReport(context.Background(), "r", "", []byte("x"), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("blank name: %v", err)
	}
	if err := c.AttachReport(context.Background(), "r", "n", nil, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("nil content: %v", err)
	}
}

func TestFinishRun_happyPath(t *testing.T) {
	t.Parallel()
	var got FinishRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		if r.URL.Path != "/api/v1/namespaces/team-alpha/tcm/external-runs/run-1/finish" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.FinishRun(context.Background(), "run-1", FinishRunRequest{
		Status:  StatusPassed,
		Summary: "ok",
	})
	if err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	if got.Status != StatusPassed || got.Summary != "ok" {
		t.Errorf("server saw %+v", got)
	}
}

func TestFinishRun_invalidStatus(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")
	err := c.FinishRun(context.Background(), "r", FinishRunRequest{Status: "weird"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("want ErrInvalidRequest, got %v", err)
	}
	err = c.FinishRun(context.Background(), "", FinishRunRequest{Status: StatusPassed})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("want ErrInvalidRequest for blank runID, got %v", err)
	}
}

func TestGetRun_happyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/namespaces/team-alpha/tcm/external-runs/run-7" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Run{ID: "run-7", Status: StatusPassed})
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	run, err := c.GetRun(context.Background(), "run-7")
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "run-7" || run.Status != StatusPassed {
		t.Errorf("got %+v", run)
	}
}

func TestGetRun_blankID(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")
	if _, err := c.GetRun(context.Background(), ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("want ErrInvalidRequest, got %v", err)
	}
}

func TestListRuns_querystring(t *testing.T) {
	t.Parallel()
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireHeaders(t, r)
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RunList{
			Runs:       []Run{{ID: "r1"}},
			Total:      1,
			NextCursor: "cur-2",
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	out, err := c.ListRuns(context.Background(), ListRunsOptions{
		SuiteID:    "suite-x",
		Framework:  "go-test",
		Status:     StatusPassed,
		ExternalID: "ext-7",
		Cursor:     "cur-1",
		Limit:      25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.NextCursor != "cur-2" || len(out.Runs) != 1 {
		t.Errorf("response = %+v", out)
	}
	checks := map[string]string{
		"suite_id":    "suite-x",
		"framework":   "go-test",
		"status":      "passed",
		"external_id": "ext-7",
		"cursor":      "cur-1",
		"limit":       "25",
	}
	for k, want := range checks {
		if got := gotQuery.Get(k); got != want {
			t.Errorf("query %q = %q, want %q", k, got, want)
		}
	}
}

func TestListRuns_invalidStatus(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok")
	if _, err := c.ListRuns(context.Background(), ListRunsOptions{Status: "??"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("want ErrInvalidRequest, got %v", err)
	}
}

func TestAPIError_4xxStructured(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"namespace forbidden","code":"NS_DENIED"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.CreateRun(context.Background(), CreateRunRequest{Name: "n", Framework: "f"})
	apiErr := AsAPIError(err)
	if apiErr == nil {
		t.Fatalf("expected *APIError, got %T %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if apiErr.Message != "namespace forbidden" {
		t.Errorf("message = %q", apiErr.Message)
	}
	if apiErr.Code != "NS_DENIED" {
		t.Errorf("code = %q", apiErr.Code)
	}
	if !strings.Contains(apiErr.Error(), "403") {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}

func TestAPIError_5xxRawBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>upstream</html>"))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.GetRun(context.Background(), "r1")
	apiErr := AsAPIError(err)
	if apiErr == nil {
		t.Fatalf("expected APIError")
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.RawBody, "upstream") {
		t.Errorf("raw body lost: %q", apiErr.RawBody)
	}
	if apiErr.Message != "" {
		t.Errorf("non-json body should leave Message empty: %q", apiErr.Message)
	}
}

func TestAPIError_emptyBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.GetRun(context.Background(), "r1")
	apiErr := AsAPIError(err)
	if apiErr == nil || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 APIError, got %v", err)
	}
	if !strings.Contains(apiErr.Error(), "404") {
		t.Errorf("Error() = %q", apiErr.Error())
	}
}

func TestAPIError_NilSafe(t *testing.T) {
	t.Parallel()
	var e *APIError
	if e.Error() != "<nil>" {
		t.Errorf("nil *APIError.Error() should be safe")
	}
	if AsAPIError(errors.New("plain")) != nil {
		t.Errorf("plain error must not unwrap to APIError")
	}
}

func TestAPIError_RawBodyTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("X", 600)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(long))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.GetRun(context.Background(), "r1")
	apiErr := AsAPIError(err)
	if apiErr == nil {
		t.Fatalf("expected APIError")
	}
	msg := apiErr.Error()
	if !strings.Contains(msg, "...") {
		t.Errorf("expected truncation marker in %q", msg)
	}
	if strings.Count(msg, "X") > 300 {
		t.Errorf("truncation did not shorten output: len=%d", len(msg))
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	_, err := c.GetRun(ctx, "r1")
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestMalformedJSONResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.GetRun(context.Background(), "r1")
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if AsAPIError(err) != nil {
		t.Errorf("decode error should not be APIError")
	}
}

func TestConcurrentUsage_NoRace(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/external-runs"):
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(Run{ID: "r" + r.Header.Get("X-Test-Goroutine"), Status: StatusRunning})
				return
			}
			_ = json.NewEncoder(w).Encode(RunList{Runs: []Run{{ID: "list"}}})
		case strings.HasSuffix(r.URL.Path, "/steps"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/finish"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/attachments"):
			w.WriteHeader(http.StatusNoContent)
		default:
			_ = json.NewEncoder(w).Encode(Run{ID: "any"})
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n*5)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			if _, err := c.CreateRun(ctx, CreateRunRequest{Name: "n", Framework: "f"}); err != nil {
				errCh <- fmt.Errorf("create: %w", err)
				return
			}
			if err := c.AddSteps(ctx, "r"+strconv.Itoa(i), []Step{{StepKey: "k", Name: "n", Status: StatusPassed}}); err != nil {
				errCh <- fmt.Errorf("steps: %w", err)
				return
			}
			if err := c.AttachReport(ctx, "r"+strconv.Itoa(i), "f.txt", []byte("x"), "text/plain"); err != nil {
				errCh <- fmt.Errorf("attach: %w", err)
				return
			}
			if err := c.FinishRun(ctx, "r"+strconv.Itoa(i), FinishRunRequest{Status: StatusPassed}); err != nil {
				errCh <- fmt.Errorf("finish: %w", err)
				return
			}
			if _, err := c.GetRun(ctx, "r"+strconv.Itoa(i)); err != nil {
				errCh <- fmt.Errorf("get: %w", err)
				return
			}
			if _, err := c.ListRuns(ctx, ListRunsOptions{Limit: 5}); err != nil {
				errCh <- fmt.Errorf("list: %w", err)
				return
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent call: %v", err)
	}
	if calls.Load() < int64(n*6) {
		t.Errorf("expected %d calls, got %d", n*6, calls.Load())
	}
}

func TestApplyAuthHeaders_centralised(t *testing.T) {
	// Direct test that BOTH paths (JSON + multipart) call applyAuthHeaders.
	// We check by counting header values across endpoints.
	t.Parallel()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(AuthHeader) == "tok-abc" && r.Header.Get(SchemaVersionHeader) == strconv.Itoa(SchemaVersion) {
			hits++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_ = c.AddSteps(context.Background(), "r", []Step{{StepKey: "k"}})
	_ = c.AttachReport(context.Background(), "r", "f", []byte("x"), "")
	_ = c.FinishRun(context.Background(), "r", FinishRunRequest{Status: StatusPassed})
	if hits != 3 {
		t.Errorf("expected 3 auth-bearing calls, got %d", hits)
	}
}

// roundTripFunc is a tiny adapter to plug a custom transport without
// touching net/http internals — used in TestNetworkError to provoke a
// pre-HTTP error path that httptest can't easily simulate.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNetworkError(t *testing.T) {
	t.Parallel()
	c, _ := NewClient("https://x.test", "ns", "tok",
		WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial: no route to host")
		})}))
	_, err := c.GetRun(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected network error")
	}
	if AsAPIError(err) != nil {
		t.Errorf("network error should not be APIError")
	}
	if !strings.Contains(err.Error(), "no route to host") {
		t.Errorf("err wraps original: %v", err)
	}
}

func TestSchemaVersion_constant(t *testing.T) {
	t.Parallel()
	// Schema bump is a coordinated change. If this trips, update Py + Java
	// clients in the same commit AND bump the server's accepted set.
	if SchemaVersion != 1 {
		t.Errorf("SchemaVersion changed to %d — confirm Py + Java + server in sync", SchemaVersion)
	}
}

// TestMultipartBoundaryUnique guards against the boundary appearing in
// the payload (which would corrupt the multipart frame). Server-side we
// inspect the parsed result.
func TestMultipartBoundaryUnique(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := parseContentType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("ct: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q", mediaType)
		}
		boundary := params["boundary"]
		if boundary == "" {
			t.Errorf("boundary missing")
		}
		mr := multipart.NewReader(r.Body, boundary)
		var parts int
		for {
			p, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("next part: %v", err)
			}
			parts++
			_, _ = io.Copy(io.Discard, p)
			_ = p.Close()
		}
		if parts < 2 {
			t.Errorf("expected file + name field, got %d parts", parts)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.AttachReport(context.Background(), "r", "f.bin", []byte("payload"), "application/x-test"); err != nil {
		t.Fatal(err)
	}
}

func parseContentType(ct string) (string, map[string]string, error) {
	idx := strings.Index(ct, ";")
	if idx < 0 {
		return ct, nil, nil
	}
	media := strings.TrimSpace(ct[:idx])
	params := map[string]string{}
	for _, kv := range strings.Split(ct[idx+1:], ";") {
		kv = strings.TrimSpace(kv)
		eq := strings.Index(kv, "=")
		if eq < 0 {
			continue
		}
		params[strings.TrimSpace(kv[:eq])] = strings.Trim(strings.TrimSpace(kv[eq+1:]), `"`)
	}
	return media, params, nil
}
