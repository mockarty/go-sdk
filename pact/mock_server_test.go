// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

func TestMockServerMatchesPOSTWithJSONBody(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("charge").
		WithRequest(http.MethodPost, "/charge").
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"amount": pact.Like(100)}).
		WillRespondWith(200).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"id": pact.Like("ok")})

	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"amount": 999})
	resp, err := http.Post(srv.URL()+"/charge", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		debug, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body=%s", resp.StatusCode, debug)
	}
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["id"] != "ok" {
		t.Fatalf("response body unexpected: %v", out)
	}
	if err := srv.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestMockServerRejectsBodyShapeMismatch(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("expects amount").
		WithRequest(http.MethodPost, "/charge").
		WithJSONBody(map[string]any{"amount": pact.Like(100)}).
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Send a body missing the declared key.
	body, _ := json.Marshal(map[string]any{"wrong": 1})
	resp, err := http.Post(srv.URL()+"/charge", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if err := srv.Verify(); err == nil {
		t.Fatalf("verify must fail when no interaction was matched")
	}
}

func TestMockServerQueryParamsRequired(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("search").
		WithRequest(http.MethodGet, "/search").
		WithQuery("q", "hello").
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Wrong query.
	resp, err := http.Get(srv.URL() + "/search?q=other")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong query; got %d", resp.StatusCode)
	}
	// Right query.
	u, _ := url.Parse(srv.URL() + "/search")
	q := u.Query()
	q.Set("q", "hello")
	u.RawQuery = q.Encode()
	resp2, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("right query got %d", resp2.StatusCode)
	}
}

func TestMockServerContentTypeTolerance(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("strict ct").
		WithRequest(http.MethodPost, "/x").
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"k": "v"}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// Real clients often add `; charset=utf-8`; the server must tolerate this.
	body, _ := json.Marshal(map[string]any{"k": "v"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/x", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("charset header rejected: %d", resp.StatusCode)
	}
}

func TestMockServerParallelRequests(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("ping").
		WithRequest(http.MethodGet, "/ping").
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL() + "/ping")
			if err != nil {
				errs <- err
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode != 200 {
				errs <- nil
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent get: %v", err)
		}
	}
	calls := srv.Calls()
	if calls[0] != n {
		t.Fatalf("calls = %d; want %d", calls[0], n)
	}
}

func TestMockServerVerifyAllSatisfied(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("a").
		WithRequest(http.MethodGet, "/a").
		WillRespondWith(200)
	c.AddInteraction().
		UponReceiving("b").
		WithRequest(http.MethodGet, "/b").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	for _, p := range []string{"/a", "/b"} {
		resp, err := http.Get(srv.URL() + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if err := srv.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestMockServer404IncludesDebugSummary(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("declared").
		WithRequest(http.MethodGet, "/declared").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()

	resp, err := http.Get(srv.URL() + "/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "no interaction matched") {
		t.Fatalf("404 should include debug summary; got %s", body)
	}
}

func TestMockServerCalls(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	calls := srv.Calls()
	if len(calls) != 1 || calls[0] != 0 {
		t.Fatalf("initial calls = %v", calls)
	}
	resp, _ := http.Get(srv.URL() + "/x")
	_ = resp.Body.Close()
	if srv.Calls()[0] != 1 {
		t.Fatalf("call count = %v", srv.Calls())
	}
}

func TestMockServerSliceBodyShape(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("items").
		WithRequest(http.MethodPost, "/items").
		WithJSONBody([]any{map[string]any{"id": pact.Like(1)}}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()

	body, _ := json.Marshal([]any{map[string]any{"id": 42}})
	resp, err := http.Post(srv.URL()+"/items", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestMockServerCloseIdempotent(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
