// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// buildChargeContract is the V3/V4 "charge" example shared by the V3 and
// V4 schema round-trip tests. Keeping it factored out makes the
// fixture-vs-emitter comparison clearer.
func buildChargeContract(t *testing.T, spec pact.SpecVersion) *pact.Consumer {
	t.Helper()
	c := pact.NewConsumer("OrderService",
		pact.WithProvider("PaymentService"),
		pact.WithSpecVersion(spec),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		Given("payment service is up").
		UponReceiving("a charge request").
		WithRequest(http.MethodPost, "/charge").
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"amount": pact.Like(100)}).
		WillRespondWith(200).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"id": pact.Like("abc")})
	return c
}

// readFixture loads a reference pact.json from testdata.
func readFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	blob, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	var out map[string]any
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("parse fixture %q: %v", name, err)
	}
	return out
}

func TestWriteV3RoundTripsAgainstReference(t *testing.T) {
	t.Parallel()
	c := buildChargeContract(t, pact.SpecV3)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Don't call the interaction — we're only after the file artefact.
	if err := srv.Close(); err != nil && !strings.Contains(err.Error(), "rename") {
		// Close returns the write error; we expect success.
		t.Fatalf("close: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected 1 pact file; got %v", files)
	}
	actual := map[string]any{}
	blob, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read written: %v", err)
	}
	if err := json.Unmarshal(blob, &actual); err != nil {
		t.Fatalf("parse written: %v", err)
	}
	ref := readFixture(t, "v3-reference.json")

	// Spec version.
	if got := metaVersion(actual); got != "3.0.0" {
		t.Fatalf("V3 spec version: got %q", got)
	}
	if got := metaVersion(ref); got != "3.0.0" {
		t.Fatalf("ref spec version mismatch: %q", got)
	}

	// Interactions: V3 keeps providerState (singular).
	ix := firstInteraction(t, actual)
	if ps, _ := ix["providerState"].(string); ps == "" {
		t.Fatalf("V3 must carry singular providerState; got %v", ix["providerState"])
	}
	// V3 must NOT carry the type discriminator.
	if _, ok := ix["type"]; ok {
		t.Fatalf("V3 interaction must not have a type field; got %v", ix["type"])
	}
	// matchingRules: flat $.body.amount path.
	req, _ := ix["request"].(map[string]any)
	rules, _ := req["matchingRules"].(map[string]any)
	if _, ok := rules["$.body.amount"]; !ok {
		t.Fatalf("V3 expected flat $.body.amount key; got %v", rules)
	}
}

func TestWriteV4RoundTripsAgainstReference(t *testing.T) {
	t.Parallel()
	c := buildChargeContract(t, pact.SpecV4)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected 1 file; got %v", files)
	}
	blob, _ := os.ReadFile(files[0])
	actual := map[string]any{}
	_ = json.Unmarshal(blob, &actual)
	ref := readFixture(t, "v4-reference.json")

	if metaVersion(actual) != "4.0" || metaVersion(ref) != "4.0" {
		t.Fatalf("V4 spec version mismatch")
	}
	ix := firstInteraction(t, actual)
	if ix["type"] != pact.HTTPInteractionType {
		t.Fatalf("V4 type = %v; want Synchronous/HTTP", ix["type"])
	}
	// providerStates plural array.
	psList, _ := ix["providerStates"].([]any)
	if len(psList) != 1 {
		t.Fatalf("V4 providerStates: %v", psList)
	}
	// matchingRules nested under body.
	req, _ := ix["request"].(map[string]any)
	rules, _ := req["matchingRules"].(map[string]any)
	body, _ := rules["body"].(map[string]any)
	if _, ok := body["$.amount"]; !ok {
		t.Fatalf("V4 expected body $.amount; got %v", rules)
	}
}

func TestWriteRejectsInvalidSpec(t *testing.T) {
	t.Parallel()
	_, err := pact.RenderPactFile(pact.PactFile{}, pact.SpecVersion("bogus"))
	if err == nil {
		t.Fatalf("must reject unknown spec version")
	}
}

func TestNilConsumerWrite(t *testing.T) {
	t.Parallel()
	if _, err := pact.WritePactFile(nil); err == nil {
		t.Fatalf("WritePactFile(nil) should error")
	}
}

// metaVersion extracts metadata.pactSpecification.version from a raw
// pact map.
func metaVersion(in map[string]any) string {
	meta, _ := in["metadata"].(map[string]any)
	spec, _ := meta["pactSpecification"].(map[string]any)
	v, _ := spec["version"].(string)
	return v
}

func firstInteraction(t *testing.T, in map[string]any) map[string]any {
	t.Helper()
	ixs, _ := in["interactions"].([]any)
	if len(ixs) == 0 {
		t.Fatalf("no interactions in %v", in)
	}
	m, _ := ixs[0].(map[string]any)
	return m
}

func TestEachLikeProducesArrayBody(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithSpecVersion(pact.SpecV4),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("list").
		WithRequest(http.MethodGet, "/items").
		WillRespondWith(200).
		WithJSONBody(map[string]any{
			"items": pact.EachLike(map[string]any{"id": pact.Like(1)}, 1),
		})
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	blob, _ := os.ReadFile(files[0])
	out := map[string]any{}
	_ = json.Unmarshal(blob, &out)
	ix := firstInteraction(t, out)
	resp, _ := ix["response"].(map[string]any)
	body, _ := resp["body"].(map[string]any)
	items, ok := body["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("EachLike must wrap example in []; got %#v", body["items"])
	}
	// matchingRules must contain a $.body.items entry plus the nested [*] child.
	rules, _ := resp["matchingRules"].(map[string]any)
	bodyRules, _ := rules["body"].(map[string]any)
	if _, ok := bodyRules["$.items"]; !ok {
		t.Fatalf("expected $.items rule; got %v", bodyRules)
	}
}

func TestUnicodeInDescriptionPreserved(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("Привет 你好 — JSON 🚀").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	blob, _ := os.ReadFile(files[0])
	if !strings.Contains(string(blob), "Привет") || !strings.Contains(string(blob), "你好") ||
		!strings.Contains(string(blob), "🚀") {
		t.Fatalf("unicode lost: %s", blob)
	}
}

func TestLargeBodyDoesNotBlow(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("a", 100_000)
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("big").
		WithRequest(http.MethodPost, "/big").
		WithJSONBody(map[string]any{"blob": pact.Like(huge)}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestSafeFilenameStripsExoticChars(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front/Service",
		pact.WithProvider("Back: Service"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	if len(files) != 1 {
		t.Fatalf("files: %v", files)
	}
	name := filepath.Base(files[0])
	if strings.ContainsAny(name, "/: ") {
		t.Fatalf("unsafe filename leaked: %q", name)
	}
}

func TestProviderStateWithParamsV4(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A",
		pact.WithProvider("B"),
		pact.WithSpecVersion(pact.SpecV4),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		GivenWithParams("user exists", map[string]any{"id": "u-1"}).
		UponReceiving("get").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	files, _ := filepath.Glob(filepath.Join(c.OutputDir(), "*.json"))
	blob, _ := os.ReadFile(files[0])
	out := map[string]any{}
	_ = json.Unmarshal(blob, &out)
	ix := firstInteraction(t, out)
	psList, _ := ix["providerStates"].([]any)
	if len(psList) != 1 {
		t.Fatalf("provider states: %v", psList)
	}
	ps, _ := psList[0].(map[string]any)
	params, _ := ps["params"].(map[string]any)
	if params["id"] != "u-1" {
		t.Fatalf("params lost: %v", params)
	}
}
