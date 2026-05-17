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

func TestNewConsumerDefaults(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("OrderService")
	if c.ConsumerName() != "OrderService" {
		t.Fatalf("consumer name = %q", c.ConsumerName())
	}
	if c.ProviderName() != "UnknownProvider" {
		t.Fatalf("provider default = %q", c.ProviderName())
	}
	if c.SpecVersion() != pact.SpecV4 {
		t.Fatalf("default spec = %q; want V4", c.SpecVersion())
	}
	if c.OutputDir() != "./pacts" {
		t.Fatalf("default output dir = %q", c.OutputDir())
	}
}

func TestConsumerOptions(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithSpecVersion(pact.SpecV3),
		pact.WithOutputDir(out),
	)
	if c.SpecVersion() != pact.SpecV3 {
		t.Fatalf("spec = %q; want V3", c.SpecVersion())
	}
	if c.ProviderName() != "Back" {
		t.Fatalf("provider = %q", c.ProviderName())
	}
	if c.OutputDir() != out {
		t.Fatalf("output dir = %q", c.OutputDir())
	}
}

func TestConsumerInvalidSpecFallsBackToV4(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("X", pact.WithSpecVersion("99.0"))
	if c.SpecVersion() != pact.SpecV4 {
		t.Fatalf("invalid spec did not fall back: got %q", c.SpecVersion())
	}
}

func TestConsumerNilOption(t *testing.T) {
	t.Parallel()
	// Should not panic — defensive against `if cond { opt = ... } else { opt = nil }` patterns.
	_ = pact.NewConsumer("X", nil)
}

func TestAddInteractionBuilderHappyPath(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithProvider("B"))
	c.AddInteraction().
		Given("state-1").
		UponReceiving("hello world").
		WithRequest(http.MethodGet, "/x").
		WithHeader("Accept", "application/json").
		WillRespondWith(200).
		WithJSONBody(map[string]any{"ok": true})

	ixs := c.Interactions()
	if len(ixs) != 1 {
		t.Fatalf("len(interactions) = %d", len(ixs))
	}
	ix := ixs[0]
	if ix.Description != "hello world" {
		t.Fatalf("desc = %q", ix.Description)
	}
	if ix.Request.Method != http.MethodGet || ix.Request.Path != "/x" {
		t.Fatalf("request = %+v", ix.Request)
	}
	if ix.Response.Status != 200 {
		t.Fatalf("status = %d", ix.Response.Status)
	}
	if len(ix.ProviderStates) != 1 || ix.ProviderStates[0].Name != "state-1" {
		t.Fatalf("provider states = %+v", ix.ProviderStates)
	}
	if ix.Type != pact.HTTPInteractionType {
		t.Fatalf("V4 type discriminator missing: %q", ix.Type)
	}
}

func TestAddInteractionV3OmitsTypeDiscriminator(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithSpecVersion(pact.SpecV3))
	c.AddInteraction().
		UponReceiving("v3 has no type").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	ix := c.Interactions()[0]
	if ix.Type != "" {
		t.Fatalf("V3 type should be empty; got %q", ix.Type)
	}
}

func TestStartAndMockServeHappyPath(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(out),
	)
	c.AddInteraction().
		UponReceiving("ping").
		WithRequest(http.MethodGet, "/ping").
		WillRespondWith(200).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"pong": true})

	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		if err := srv.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}()

	resp, err := http.Get(srv.URL() + "/ping")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if err := srv.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyReportsMissedInteractions(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	c := pact.NewConsumer("Front",
		pact.WithProvider("Back"),
		pact.WithOutputDir(out),
	)
	c.AddInteraction().
		UponReceiving("never called").
		WithRequest(http.MethodGet, "/never").
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Close()
	if err := srv.Verify(); err == nil {
		t.Fatalf("verify should fail for uncalled interaction")
	} else if !strings.Contains(err.Error(), "never called") {
		t.Fatalf("verify error missing context: %v", err)
	}
}

func TestMockServerUnmatchedRequestReturns404(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("Front", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("declared").
		WithRequest(http.MethodGet, "/declared").
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Close()
	resp, err := http.Get(srv.URL() + "/wrong-path")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestStartTwiceReusesConsumerState(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv1, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	srv2, err := c.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if srv1.URL() == srv2.URL() {
		t.Fatalf("two mock servers must have distinct URLs")
	}
	_ = srv1.Close()
	if err := srv2.Close(); err == nil {
		// Second close should succeed because finalize is idempotent.
		t.Logf("ok")
	}
}

func TestStartNilContextRejected(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A")
	//nolint:staticcheck // explicit nil to assert defensive return
	if _, err := c.Start(nil); err == nil {
		t.Fatalf("nil context should error")
	}
}

func TestAddInteractionAfterCloseReturnsNil(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	_ = srv.Close()
	if b := c.AddInteraction(); b != nil {
		t.Fatalf("AddInteraction after Close should return nil")
	}
}

func TestAddInteractionAfterStartReturnsNil(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("A", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("x").
		WithRequest(http.MethodGet, "/x").
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = srv.Close() }()
	// Once Start has snapshotted the interactions the mock server is
	// already serving, AddInteraction must be a no-op — otherwise the new
	// interaction silently misses the snapshot the mock is serving.
	if b := c.AddInteraction(); b != nil {
		t.Fatalf("AddInteraction after Start should return nil; got %v", b)
	}
}
