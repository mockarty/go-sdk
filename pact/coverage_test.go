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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// This file deliberately drives the matcher / path-splitter / encoder
// branches that the happy-path tests do not exercise. Coverage here
// matters because pact files round-trip through external verifiers —
// every supported matcher must be exercised at least once so the
// writer's per-matcher quirks are caught locally.

func TestMatcherCoverage_MinMaxAndKeyLike(t *testing.T) {
	t.Parallel()
	// MinType / MaxType / EachKeyLike are V4-leaning matchers that
	// otherwise have no caller in the happy path.
	mn := pact.MinType("x", 2)
	if mn.Rule.Min == nil || *mn.Rule.Min != 2 {
		t.Fatalf("min lost: %+v", mn)
	}
	mx := pact.MaxType("x", 7)
	if mx.Rule.Max == nil || *mx.Rule.Max != 7 {
		t.Fatalf("max lost: %+v", mx)
	}
	// Negative arg clamps to 0.
	if *pact.MinType("x", -1).Rule.Min != 0 {
		t.Fatalf("MinType clamp")
	}
	if *pact.MaxType("x", -1).Rule.Max != 0 {
		t.Fatalf("MaxType clamp")
	}
	ek := pact.EachKeyLike("v")
	if ek.Rule.Match != "values" {
		t.Fatalf("EachKeyLike: %+v", ek.Rule)
	}
	// ArrayContains with many variants triggers the indexSegment fast path
	// AND the itoa fallback (index > 4).
	a := pact.ArrayContains(pact.Like(1), pact.Like(2), pact.Like(3), pact.Like(4), pact.Like(5), pact.Like(6))
	if len(a.Children) != 6 {
		t.Fatalf("array contains children: %d", len(a.Children))
	}
	if a.Children[5].PathSegment != "[5]" {
		t.Fatalf("indexSegment fallback: %q", a.Children[5].PathSegment)
	}
	// indexSegment fast path values 0..4.
	for i, want := range []string{"[0]", "[1]", "[2]", "[3]", "[4]"} {
		if a.Children[i].PathSegment != want {
			t.Fatalf("indexSegment[%d] = %q; want %q", i, a.Children[i].PathSegment, want)
		}
	}
}

// TestV4PathSplittingAllCategories exercises every V4 path category so
// splitV4Path returns each known prefix.
func TestV4PathSplittingAllCategories(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithSpecVersion(pact.SpecV4),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("varied paths").
		WithRequest(http.MethodGet, "/x").
		WithHeader("X-Custom", "v").
		WithQuery("q", "1").
		WithJSONBody(map[string]any{"a": pact.Like(1)}).
		WillRespondWith(200).
		WithHeader("X-Resp", "r").
		WithJSONBody(map[string]any{"b": pact.Like(2)})

	// Inject a synthetic "$.path" matching rule by reaching into the
	// rendered output: simplest way is to add a custom Generators/path
	// rule via the type-level API at the wire layer. Since the public
	// DSL only emits body+header matchers we instead verify splitV4Path
	// indirectly through formatMatchingRules by feeding a PactFile with
	// hand-built rules to RenderPactFile.
	pf := pact.PactFile{
		Consumer: pact.Participant{Name: "A"},
		Provider: pact.Participant{Name: "B"},
		Interactions: []pact.Interaction{{
			Description: "wire",
			Type:        pact.HTTPInteractionType,
			Request: pact.Request{
				Method: http.MethodGet, Path: "/x",
				MatchingRules: map[string]pact.MatchCategory{
					"$.body.id":           {Matchers: []pact.MatcherRule{{Match: "type"}}},
					"$.header.X-A[0]":     {Matchers: []pact.MatcherRule{{Match: "type"}}},
					"$.query.q":           {Matchers: []pact.MatcherRule{{Match: "type"}}},
					"$.path":              {Matchers: []pact.MatcherRule{{Match: "regex", Regex: "/x"}}},
					"$.body":              {Matchers: []pact.MatcherRule{{Match: "type"}}},
					"$.something-strange": {Matchers: []pact.MatcherRule{{Match: "type"}}},
				},
			},
			Response: pact.Response{Status: 200},
		}},
	}
	blob, err := pact.RenderPactFile(pf, pact.SpecV4)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("parse: %v", err)
	}
	ixs, _ := out["interactions"].([]any)
	ix, _ := ixs[0].(map[string]any)
	req, _ := ix["request"].(map[string]any)
	rules, _ := req["matchingRules"].(map[string]any)
	for _, cat := range []string{"body", "header", "query", "path"} {
		if _, ok := rules[cat]; !ok {
			t.Fatalf("category %q missing from %v", cat, rules)
		}
	}
	// Unknown path falls back to body.
	body, _ := rules["body"].(map[string]any)
	if _, ok := body["$.something-strange"]; !ok {
		t.Fatalf("unknown path should bucket into body: %v", body)
	}
}

func TestMatcherAcceptsActualAllKinds(t *testing.T) {
	t.Parallel()
	// Drive each matcher kind through the mock server end-to-end so the
	// strict matcher engine's dispatch table is fully covered.
	tests := []struct {
		body       any
		actual     map[string]any
		name       string
		expectPass bool
	}{
		{name: "integer-pass", body: map[string]any{"x": pact.Integer(1)}, actual: map[string]any{"x": 42}, expectPass: true},
		{name: "integer-fail-float", body: map[string]any{"x": pact.Integer(1)}, actual: map[string]any{"x": 1.5}, expectPass: false},
		{name: "decimal-pass", body: map[string]any{"x": pact.Decimal(1)}, actual: map[string]any{"x": 1.5}, expectPass: true},
		{name: "boolean-pass", body: map[string]any{"x": pact.Boolean(true)}, actual: map[string]any{"x": false}, expectPass: true},
		{name: "boolean-fail-string", body: map[string]any{"x": pact.Boolean(true)}, actual: map[string]any{"x": "no"}, expectPass: false},
		{name: "regex-pass", body: map[string]any{"x": pact.Regex("abc", "[a-z]+")}, actual: map[string]any{"x": "hello"}, expectPass: true},
		{name: "regex-fail-nonstring", body: map[string]any{"x": pact.Regex("abc", "[a-z]+")}, actual: map[string]any{"x": 7}, expectPass: false},
		{name: "equality-pass", body: map[string]any{"x": pact.Equality("v")}, actual: map[string]any{"x": "v"}, expectPass: true},
		{name: "equality-fail", body: map[string]any{"x": pact.Equality("v")}, actual: map[string]any{"x": "other"}, expectPass: false},
		{name: "each-key-pass", body: map[string]any{"x": pact.EachKeyLike("v")}, actual: map[string]any{"x": map[string]any{"k1": 1}}, expectPass: true},
		{name: "each-key-fail-list", body: map[string]any{"x": pact.EachKeyLike("v")}, actual: map[string]any{"x": []any{1}}, expectPass: false},
		{name: "arraycontains-pass", body: map[string]any{"x": pact.ArrayContains(pact.Like("a"))}, actual: map[string]any{"x": []any{"a"}}, expectPass: true},
		{name: "arraycontains-fail-map", body: map[string]any{"x": pact.ArrayContains(pact.Like("a"))}, actual: map[string]any{"x": 1}, expectPass: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := pact.NewConsumer("A",
				pact.WithProvider("B"),
				pact.WithOutputDir(t.TempDir()),
			)
			c.AddInteraction().
				UponReceiving(tc.name).
				WithRequest(http.MethodPost, "/m").
				WithJSONBody(tc.body).
				WillRespondWith(200)
			srv, err := c.Start(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer srv.Close()
			payload, _ := json.Marshal(tc.actual)
			resp, err := http.Post(srv.URL()+"/m", "application/json", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			got := resp.StatusCode == 200
			if got != tc.expectPass {
				t.Fatalf("expect=%v got status=%d", tc.expectPass, resp.StatusCode)
			}
		})
	}
}

func TestStripMatchersForBodyHandlesEverything(t *testing.T) {
	t.Parallel()
	// Exercise every recursion branch via a real consumer flow.
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("varied").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200).
		WithJSONBody(map[string]any{
			"primitive":   pact.Like(1),
			"list":        []any{pact.Like("a"), pact.Like("b")},
			"nestedMap":   map[string]any{"deep": pact.Like(3.14)},
			"matcherList": []pact.Matcher{pact.Like(1), pact.Like(2)},
		})
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, err := http.Get(srv.URL() + "/x")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("response body not JSON: %v\n%s", err, body)
	}
	// All matcher placeholders must be replaced with their examples.
	if out["primitive"] != float64(1) {
		t.Fatalf("primitive: %v", out["primitive"])
	}
	if list, _ := out["list"].([]any); len(list) != 2 || list[0] != "a" {
		t.Fatalf("list: %v", out["list"])
	}
	if ml, _ := out["matcherList"].([]any); len(ml) != 2 || ml[1] != float64(2) {
		t.Fatalf("matcherList: %v", out["matcherList"])
	}
}

func TestWriteRejectsBadOutputDir(t *testing.T) {
	t.Parallel()
	// A path that cannot be created (a file used as a directory) makes
	// the writer fail at mkdir.
	tmp := t.TempDir()
	clash := filepath.Join(tmp, "file")
	if err := os.WriteFile(clash, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithOutputDir(filepath.Join(clash, "subdir")),
	)
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	if _, err := pact.WritePactFile(c); err == nil {
		t.Fatalf("expected write to fail on non-creatable directory")
	}
}

func TestURLAfterCloseShouldPanic(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("URL after Close should panic")
		}
	}()
	_ = srv.URL()
}

func TestEmptyBodyPasses(t *testing.T) {
	t.Parallel()
	// Declared body nil and actual empty body — must succeed.
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("noBody").
		WithRequest(http.MethodPost, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, err := http.Post(srv.URL()+"/x", "application/json", bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestEmptyDeclaredArrayIsWildcard(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("wild").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"items": []any{}}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	body, _ := json.Marshal(map[string]any{"items": []any{1, 2, 3}})
	resp, err := http.Post(srv.URL()+"/x", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestNonJSONActualBodyFailsShape(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("json").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"k": "v"}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	resp, err := http.Post(srv.URL()+"/x", "text/plain", strings.NewReader("not-json-at-all"))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-JSON body should not match JSON-shaped declaration: got %d", resp.StatusCode)
	}
}

func TestPluginMetadataRecordedV4(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithSpecVersion(pact.SpecV4),
		pact.WithOutputDir(t.TempDir()),
		pact.WithPlugin("protobuf", "0.5.0", map[string]any{"foo": "bar"}),
	)
	c.AddInteraction().
		UponReceiving("with plugin").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	// Materialise the file by closing first.
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	if len(files) != 1 {
		t.Fatalf("files: %v", files)
	}
	blob, _ := os.ReadFile(files[0])
	out := map[string]any{}
	_ = json.Unmarshal(blob, &out)
	meta, _ := out["metadata"].(map[string]any)
	plugins, ok := meta["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("plugins missing: %v", meta["plugins"])
	}
}

func TestPluginNotRecordedV3(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithSpecVersion(pact.SpecV3),
		pact.WithOutputDir(t.TempDir()),
		pact.WithPlugin("protobuf", "0.5.0", nil),
	)
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	blob, _ := os.ReadFile(files[0])
	out := map[string]any{}
	_ = json.Unmarshal(blob, &out)
	meta, _ := out["metadata"].(map[string]any)
	if _, has := meta["plugins"]; has {
		t.Fatalf("V3 should not emit plugins; got %v", meta)
	}
}

func TestRequestWithMultiValueQuery(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("multi").
		WithRequest(http.MethodGet, "/x").
		WithQuery("tag", "a", "b").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	u, _ := url.Parse(srv.URL() + "/x")
	q := u.Query()
	q.Add("tag", "a")
	q.Add("tag", "b")
	u.RawQuery = q.Encode()
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("multi-query: status %d", resp.StatusCode)
	}
}

func TestMultiHeaderFlattening(t *testing.T) {
	t.Parallel()
	// Headers with two values must serialise as a list; one-value as a string.
	pf := pact.PactFile{
		Consumer: pact.Participant{Name: "A"},
		Provider: pact.Participant{Name: "B"},
		Interactions: []pact.Interaction{{
			Description: "h",
			Request: pact.Request{
				Method:  http.MethodGet,
				Path:    "/x",
				Headers: map[string][]string{"X-One": {"only"}, "X-Many": {"a", "b"}},
			},
			Response: pact.Response{Status: 200},
		}},
	}
	blob, err := pact.RenderPactFile(pf, pact.SpecV4)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	_ = json.Unmarshal(blob, &out)
	ixs, _ := out["interactions"].([]any)
	ix, _ := ixs[0].(map[string]any)
	req, _ := ix["request"].(map[string]any)
	hdrs, _ := req["headers"].(map[string]any)
	if hdrs["X-One"] != "only" {
		t.Fatalf("single-value header should be a string; got %T %v", hdrs["X-One"], hdrs["X-One"])
	}
	if _, ok := hdrs["X-Many"].([]any); !ok {
		t.Fatalf("multi-value header should be a []any; got %T %v", hdrs["X-Many"], hdrs["X-Many"])
	}
}
