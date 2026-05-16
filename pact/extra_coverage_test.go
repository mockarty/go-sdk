// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// These tests fill coverage gaps for matcher constructors that have no
// dedicated test elsewhere and exercise the plugin runtime adapter's
// metadata pass-through.

func TestNullAndNumberAndEachLikeBoundedConstructors(t *testing.T) {
	t.Parallel()
	if m := pact.Null(); m.Rule.Match != "null" || m.Example != nil {
		t.Fatalf("Null: %+v", m)
	}
	if m := pact.Number(1.5); m.Rule.Match != "number" || m.Example != 1.5 {
		t.Fatalf("Number: %+v", m)
	}
	// XMLPath round-trip.
	x := pact.XMLPath("//root/elt", pact.Like("v"))
	if x.Rule.Match != "xmlpath" || x.Rule.Value != "//root/elt" {
		t.Fatalf("XMLPath: %+v", x)
	}
	// EachLikeBounded with negative + inverted args clamps.
	b := pact.EachLikeBounded("x", -1, -5)
	if *b.Rule.Min != 0 || *b.Rule.Max != 0 {
		t.Fatalf("EachLikeBounded clamp: %+v / %+v", b.Rule.Min, b.Rule.Max)
	}
}

func TestStrictNumberAndNullMatchers(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("scalars").
		WithRequest(http.MethodPost, "/s").
		WithJSONBody(map[string]any{
			"n": pact.Number(1),
			"z": pact.Null(),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	good := `{"n":3.14,"z":null}`
	resp, _ := http.Post(srv.URL()+"/s", "application/json", strings.NewReader(good))
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("good: %d", resp.StatusCode)
	}
	bad := `{"n":"not number","z":null}`
	resp, _ = http.Post(srv.URL()+"/s", "application/json", strings.NewReader(bad))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-number: %d", resp.StatusCode)
	}
	bad2 := `{"n":1,"z":"oops"}`
	resp, _ = http.Post(srv.URL()+"/s", "application/json", strings.NewReader(bad2))
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("non-null: %d", resp.StatusCode)
	}
}

func TestStrictXMLPathRequiresStringBody(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("xml").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"x": pact.XMLPath("//hello", pact.Like("v")),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"<hello/>"}`); c != http.StatusOK {
		t.Fatalf("xmlpath string body: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":42}`); c != http.StatusNotFound {
		t.Fatalf("xmlpath non-string: %d", c)
	}
}

func TestPluginContentTypeWildcardAndCharset(t *testing.T) {
	t.Parallel()
	// Direct plugin runtime exercise via a malformed protobuf request
	// that the plugin must reject. We rely on the import side-effect to
	// register the protobuf plugin.
	c := pact.NewConsumer("front",
		pact.WithOutputDir(t.TempDir()),
		pact.WithPlugin("protobuf", "1.0.0", map[string]any{
			"fields": map[string]any{"1": map[string]any{"wire": 0}},
		}),
	)
	c.AddInteraction().
		UponReceiving("pb").
		WithRequest(http.MethodPost, "/p").
		WithHeader("Content-Type", "application/x-protobuf").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	// Charset suffix on the request — plugin claim should still match.
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/p", bytes.NewReader([]byte{0x08, 0x2A}))
	req.Header.Set("Content-Type", "application/x-protobuf; charset=binary")
	resp, _ := http.DefaultClient.Do(req)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("charset suffix should not unbind plugin: %d", resp.StatusCode)
	}
	// Truncated body — plugin rejects.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL()+"/p", bytes.NewReader([]byte{0xFF}))
	req2.Header.Set("Content-Type", "application/x-protobuf")
	resp2, _ := http.DefaultClient.Do(req2)
	body, _ := io.ReadAll(resp2.Body)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("truncated body should 404; got %d body=%s", resp2.StatusCode, body)
	}
}

func TestWithLoggerCapturesPluginWarning(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	logger := log.New(buf, "", 0)
	c := pact.NewConsumer("front",
		pact.WithOutputDir(t.TempDir()),
		pact.WithLogger(logger),
		pact.WithPlugin("unknown-future-plugin", "0.0.0", nil),
	)
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if !strings.Contains(buf.String(), "unknown-future-plugin") {
		t.Fatalf("logger should capture warning, got %q", buf.String())
	}
}

func TestRenderEmitsPluginEntries(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	c := pact.NewConsumer("front",
		pact.WithOutputDir(tmp),
		pact.WithPlugin("protobuf", "1.0.0", map[string]any{"k": "v"}),
	)
	c.AddInteraction().
		UponReceiving("p").
		WithRequest(http.MethodGet, "/p").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(tmp)
	if len(files) == 0 {
		t.Fatalf("no pact file written")
	}
	blob, _ := os.ReadFile(tmp + string(os.PathSeparator) + files[0].Name())
	var doc map[string]any
	_ = json.Unmarshal(blob, &doc)
	meta, _ := doc["metadata"].(map[string]any)
	if meta["plugins"] == nil {
		t.Fatalf("plugins missing from metadata: %s", blob)
	}
}

func TestStrictEqualityScalarFallback(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("scalar").
		WithRequest(http.MethodPost, "/v").
		WithJSONBody(map[string]any{"x": 5}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/v", `{"x":5}`); c != http.StatusOK {
		t.Fatalf("literal int match: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/v", `{"x":6}`); c != http.StatusNotFound {
		t.Fatalf("strict literal must reject 6: %d", c)
	}
}

func TestStrictArrayShorterThanDeclared(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("arr").
		WithRequest(http.MethodPost, "/a").
		WithJSONBody(map[string]any{"xs": []any{pact.Like(1), pact.Like(2)}}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/a", `{"xs":[1]}`); c != http.StatusNotFound {
		t.Fatalf("short array should 404: %d", c)
	}
}

func TestStrictDeepEqualNormalised(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("equality-deep").
		WithRequest(http.MethodPost, "/e").
		WithJSONBody(map[string]any{
			"obj": pact.Equality(map[string]any{"k": []any{1, 2}}),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	good := `{"obj":{"k":[1,2]}}`
	bad := `{"obj":{"k":[1,3]}}`
	if c, _ := doPOST(t, srv.URL()+"/e", good); c != http.StatusOK {
		t.Fatalf("deep equality: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/e", bad); c != http.StatusNotFound {
		t.Fatalf("deep equality mismatch: %d", c)
	}
}

func TestStrictInvalidRegexSurfacesMismatch(t *testing.T) {
	t.Parallel()
	// Inject an invalid regex via the public Term DSL — pact servers
	// always tolerate parse failures with a mismatch entry, never panic.
	bogus := pact.Matcher{
		Example: "x",
		Type:    "regex",
		Rule:    pact.MatcherRule{Match: "regex", Regex: "((unclosed"},
	}
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("badregex").
		WithRequest(http.MethodPost, "/r").
		WithJSONBody(map[string]any{"x": bogus}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/r", `{"x":"anything"}`); c != http.StatusNotFound {
		t.Fatalf("invalid regex must mismatch: %d", c)
	}
}

func TestStrictArrayContainsEmptyVariantsRequiresNonEmptyArray(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("ac-empty").
		WithRequest(http.MethodPost, "/ac").
		WithJSONBody(map[string]any{"xs": pact.ArrayContains()}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/ac", `{"xs":[1,2]}`); c != http.StatusOK {
		t.Fatalf("empty variants + non-empty actual: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/ac", `{"xs":[]}`); c != http.StatusNotFound {
		t.Fatalf("empty variants + empty actual: %d", c)
	}
}
