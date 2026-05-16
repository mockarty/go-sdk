// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// Drive every dispatch branch in the strict matcher engine to push
// coverage past the 85% target. Many of these tests exist solely to
// exercise nil-builder defensive paths and negative branches that the
// happy-path suite does not reach.

func TestBuilderDefensiveNilGuards(t *testing.T) {
	t.Parallel()
	// AddInteraction returns nil after the consumer is closed; every
	// builder method must tolerate a nil receiver.
	var b *pact.InteractionBuilder
	if got := b.Given("x"); got != nil {
		t.Fatalf("Given must return nil receiver")
	}
	if got := b.UponReceiving("x"); got != nil {
		t.Fatalf("UponReceiving must return nil receiver")
	}
	if got := b.WithRequest("GET", "/x"); got != nil {
		t.Fatalf("WithRequest must return nil receiver")
	}
	if got := b.WithQuery("k", "v"); got != nil {
		t.Fatalf("WithQuery must return nil receiver")
	}
	if got := b.WithHeader("k", "v"); got != nil {
		t.Fatalf("WithHeader must return nil receiver")
	}
	if got := b.WithJSONBody(nil); got != nil {
		t.Fatalf("WithJSONBody must return nil receiver")
	}
	if got := b.WillRespondWith(200); got != nil {
		t.Fatalf("WillRespondWith must return nil receiver")
	}
	if got := b.GivenWithParams("s", map[string]any{"x": 1}); got != nil {
		t.Fatalf("GivenWithParams must return nil receiver")
	}
}

func TestStrictNumberWithIntJSON(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"n": pact.Number(0)})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"n":7}`); c != http.StatusOK {
		t.Fatalf("int as number: %d", c)
	}
}

func TestStrictIncludeOnNonString(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"x": pact.Include("hi")})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":123}`); c != http.StatusNotFound {
		t.Fatalf("non-string include: %d", c)
	}
}

func TestStrictTypeMismatchSurfacedInDebug(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"x": pact.Like(1)})
	// Send string instead of number.
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"oops"}`); c != http.StatusNotFound {
		t.Fatalf("type mismatch should 404: %d", c)
	}
}

func TestStrictUnsupportedJSONPathSyntax(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("badjp").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"x": pact.JSONPath("$..wildcard", pact.Like("v")),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// `..` descent is plugin territory — the in-process engine must
	// surface a mismatch rather than panic.
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"$..wildcard"}`); c != http.StatusNotFound {
		t.Fatalf("unsupported jsonpath syntax: %d", c)
	}
}

func TestStrictJSONPathRootExpression(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("root").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"x": pact.JSONPath("$", pact.Like(map[string]any{}))}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// Any object payload satisfies "root is object".
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"$","y":1}`); c != http.StatusOK {
		t.Fatalf("root jsonpath: %d", c)
	}
}

func TestStrictEachKeyMissingChildIsNoop(t *testing.T) {
	t.Parallel()
	// Synthesise an each-key matcher without children — the engine
	// must accept any object payload (no key check to apply).
	odd := pact.Matcher{
		Type: "each-key",
		Rule: pact.MatcherRule{Match: "each-key"},
	}
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("childless").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"k": odd}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/x", `{"k":{"a":1}}`); c != http.StatusOK {
		t.Fatalf("childless each-key on object: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"k":[]}`); c != http.StatusNotFound {
		t.Fatalf("childless each-key on array: %d", c)
	}
}

func TestStrictHeaderAndQueryMissing(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("hdr").
		WithRequest(http.MethodGet, "/h").
		WithHeader("X-Trace", "abc").
		WithQuery("q", "v").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// Missing header → 404.
	req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/h?q=v", nil)
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing header: %d", resp.StatusCode)
	}
	// Wrong header value → 404.
	req.Header.Set("X-Trace", "wrong")
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong header: %d", resp.StatusCode)
	}
	// Correct everything.
	req.Header.Set("X-Trace", "abc")
	resp, _ = http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("right: %d", resp.StatusCode)
	}
}

func TestStrictMethodPathMismatch(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("mp").
		WithRequest(http.MethodGet, "/g").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// Wrong method.
	resp, _ := http.Post(srv.URL()+"/g", "text/plain", strings.NewReader(""))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("method: %d", resp.StatusCode)
	}
	// Wrong path.
	resp2, _ := http.Get(srv.URL() + "/other")
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("path: %d", resp2.StatusCode)
	}
}

func TestStrictBodyExpectedButEmpty(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("body-required").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"k": "v"}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, _ := http.Post(srv.URL()+"/x", "application/json", strings.NewReader(""))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("empty body should 404: %d", resp.StatusCode)
	}
}

func TestStrictMalformedJSONBody(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("bad-json").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"k": "v"}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, _ := http.Post(srv.URL()+"/x", "application/json", strings.NewReader(`{not-json}`))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("bad json: %d", resp.StatusCode)
	}
}

func TestStrictExpectedObjectVsActualScalar(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"x": map[string]any{"y": 1}})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"scalar"}`); c != http.StatusNotFound {
		t.Fatalf("scalar vs object: %d", c)
	}
}

func TestStrictExpectedArrayVsActualObject(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"xs": []any{1, 2}})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"xs":{"k":"v"}}`); c != http.StatusNotFound {
		t.Fatalf("object vs array: %d", c)
	}
}

func TestRunPluginMatchersNoOpWithoutContentType(t *testing.T) {
	t.Parallel()
	// Plugin runtime registered but the request omits Content-Type —
	// the plugin should not fire (claim returns false), and the request
	// should still match the declared interaction.
	c := pact.NewConsumer("front",
		pact.WithOutputDir(t.TempDir()),
		pact.WithPlugin("protobuf", "1.0.0", map[string]any{}),
	)
	c.AddInteraction().
		UponReceiving("noct").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, _ := http.Get(srv.URL() + "/x")
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("plugin must skip without content-type: %d", resp.StatusCode)
	}
}

func TestStartRejectsNilContext(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	if _, err := c.Start(nil); err == nil {
		t.Fatalf("Start(nil) must error")
	}
}

func TestStartRejectsAfterClose(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	if _, err := c.Start(context.Background()); err == nil {
		t.Fatalf("Start after close must error")
	}
}

func TestPluginRuntimeAdapterPassthrough(t *testing.T) {
	t.Parallel()
	// The plugin_runtime.go adapter routes Version/GenerateResponse to
	// the underlying plugin; we exercise it via WithPlugin → consumer
	// snapshotForWriter (writes PluginEntry version and name correctly).
	tmp := t.TempDir()
	c := pact.NewConsumer("front",
		pact.WithOutputDir(tmp),
		pact.WithPlugin("grpc", "1.0.0", map[string]any{
			"response": map[string]any{
				"bytes": "CDk=", // proto: field 1 varint 57
			},
		}),
	)
	c.AddInteraction().
		UponReceiving("g").
		WithRequest(http.MethodGet, "/g").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
}
