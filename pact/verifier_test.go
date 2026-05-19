// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package pact

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// helper: pact with a single interaction expecting GET /orders/42 → 200 {id:42}.
const simplePact = `{
  "consumer": {"name": "OrderClient"},
  "provider": {"name": "OrderAPI"},
  "interactions": [
    {
      "description": "fetch order 42",
      "providerStates": [{"name": "order 42 exists"}],
      "request":  {"method": "GET", "path": "/orders/42"},
      "response": {"status": 200, "headers": {"Content-Type": "application/json"}, "body": {"id": 42}}
    }
  ],
  "metadata": {"pactSpecification": {"version": "4.0"}}
}`

// pact with V3 singular providerState.
const v3Pact = `{
  "consumer": {"name": "OrderClient"},
  "provider": {"name": "OrderAPI"},
  "interactions": [
    {
      "description": "fetch one",
      "providerState": "data exists",
      "request":  {"method": "GET", "path": "/x"},
      "response": {"status": 204}
    }
  ]
}`

func TestNewVerifier_RequiresProviderURL(t *testing.T) {
	if _, err := NewVerifier(); err == nil {
		t.Fatal("expected error without provider URL")
	}
}

func TestVerifier_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/42" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(srv.Close)

	v, err := NewVerifier(WithProviderURL(srv.URL))
	if err != nil {
		t.Fatalf("verifier: %v", err)
	}
	res, err := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
	if len(res.Interactions) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(res.Interactions))
	}
	if res.Interactions[0].State != "order 42 exists" {
		t.Errorf("state not propagated: %q", res.Interactions[0].State)
	}
}

func TestVerifier_StatusMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(srv.Close)

	v, _ := NewVerifier(WithProviderURL(srv.URL))
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if res.OK() {
		t.Fatal("expected failure")
	}
	ms := res.Interactions[0].Mismatches
	if len(ms) == 0 {
		t.Fatal("expected mismatches")
	}
	if ms[0].Path != "$.status" {
		t.Errorf("first mismatch path = %q, want $.status", ms[0].Path)
	}
}

func TestVerifier_BodyMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 99}`)
	}))
	t.Cleanup(srv.Close)

	v, _ := NewVerifier(WithProviderURL(srv.URL))
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if res.OK() {
		t.Fatal("expected body mismatch")
	}
	found := false
	for _, m := range res.Interactions[0].Mismatches {
		if strings.Contains(m.Path, "id") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected body.id mismatch, got %+v", res.Interactions[0].Mismatches)
	}
}

func TestVerifier_StateHandlerInvoked(t *testing.T) {
	var seen atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(srv.Close)

	v, _ := NewVerifier(
		WithProviderURL(srv.URL),
		WithStateHandler("order 42 exists", func(_ context.Context, _ string, _ map[string]any) error {
			seen.Add(1)
			return nil
		}),
	)
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
	if seen.Load() != 1 {
		t.Errorf("state handler called %d times, want 1", seen.Load())
	}
}

func TestVerifier_StateHandlerError_BlocksInteraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("provider should not have been called after state setup failed")
	}))
	t.Cleanup(srv.Close)

	v, _ := NewVerifier(
		WithProviderURL(srv.URL),
		WithStateHandler("order 42 exists", func(_ context.Context, _ string, _ map[string]any) error {
			return io.ErrUnexpectedEOF
		}),
	)
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if res.OK() {
		t.Fatal("expected failure")
	}
	if !strings.Contains(res.Interactions[0].Error, "state setup") {
		t.Errorf("error not annotated: %q", res.Interactions[0].Error)
	}
}

func TestVerifier_StateSetupURL(t *testing.T) {
	var setupHit atomic.Int32
	var setupBody []byte
	setupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setupHit.Add(1)
		setupBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(setupSrv.Close)

	provSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(provSrv.Close)

	v, _ := NewVerifier(
		WithProviderURL(provSrv.URL),
		WithStateSetupURL(setupSrv.URL),
	)
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
	if setupHit.Load() != 1 {
		t.Errorf("setup URL called %d times", setupHit.Load())
	}
	if !strings.Contains(string(setupBody), "order 42 exists") {
		t.Errorf("setup body missing state: %s", string(setupBody))
	}
}

func TestVerifier_RequestFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Errorf("auth header not stamped: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(srv.Close)

	v, _ := NewVerifier(
		WithProviderURL(srv.URL),
		WithRequestFilter(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer test")
			return nil
		}),
	)
	res, _ := v.VerifyPactBytes(context.Background(), []byte(simplePact))
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
}

func TestVerifier_V3SingularProviderState(t *testing.T) {
	var stateSeen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	v, _ := NewVerifier(
		WithProviderURL(srv.URL),
		WithStateHandler("data exists", func(_ context.Context, state string, _ map[string]any) error {
			stateSeen = state
			return nil
		}),
	)
	res, _ := v.VerifyPactBytes(context.Background(), []byte(v3Pact))
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
	if stateSeen != "data exists" {
		t.Errorf("V3 singular providerState not normalised: %q", stateSeen)
	}
}

func TestVerifier_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pact.json")
	if err := os.WriteFile(path, []byte(simplePact), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(srv.Close)
	v, _ := NewVerifier(WithProviderURL(srv.URL))
	res, err := v.VerifyPactFile(context.Background(), path)
	if err != nil {
		t.Fatalf("verify file: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected pass")
	}
}

func TestVerifier_FromBroker_RequiresBroker(t *testing.T) {
	v, _ := NewVerifier(WithProviderURL("http://x"))
	if _, err := v.VerifyFromBroker(context.Background(), "c", "p", "v"); err == nil {
		t.Fatal("expected error without broker")
	}
}

func TestVerifier_FromBroker_Roundtrip(t *testing.T) {
	provSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id": 42}`)
	}))
	t.Cleanup(provSrv.Close)

	brokerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(simplePact))
	}))
	t.Cleanup(brokerSrv.Close)

	bc, _ := NewBrokerClient(WithBrokerURL(brokerSrv.URL))
	v, _ := NewVerifier(WithProviderURL(provSrv.URL), WithBrokerClient(bc))
	res, err := v.VerifyFromBroker(context.Background(), "OrderClient", "OrderAPI", "1.0")
	if err != nil {
		t.Fatalf("broker verify: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
}

func TestVerifier_PublishResults(t *testing.T) {
	var got map[string]any
	brokerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/verification-results") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(brokerSrv.Close)

	bc, _ := NewBrokerClient(WithBrokerURL(brokerSrv.URL), WithBrokerToken("tok"))
	v, _ := NewVerifier(
		WithProviderURL("http://x"),
		WithProviderName("OrderAPI"),
		WithProviderVersion("1.2.3"),
		WithBrokerClient(bc),
	)
	res := &VerificationResult{
		Interactions: []InteractionResult{
			{Description: "fetch order 42", Passed: true},
		},
	}
	if err := v.PublishResults(context.Background(), "OrderClient", "OrderAPI", "1.0", res); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success not true: %+v", got)
	}
	if got["providerApplicationVersion"] != "1.2.3" {
		t.Errorf("version not stamped: %+v", got)
	}
}

func TestVerifier_PublishResults_RequiresVersion(t *testing.T) {
	bc, _ := NewBrokerClient(WithBrokerURL("http://x"))
	v, _ := NewVerifier(WithProviderURL("http://y"), WithBrokerClient(bc))
	if err := v.PublishResults(context.Background(), "c", "p", "v",
		&VerificationResult{}); err == nil {
		t.Fatal("expected error without provider version")
	}
}

// Decode tests cover the parser's V3 / V4 union without spinning up
// the HTTP layer.
func TestParsePactDoc_V4(t *testing.T) {
	pp, err := parsePactDoc([]byte(simplePact))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(pp.Interactions) != 1 {
		t.Fatalf("expected 1, got %d", len(pp.Interactions))
	}
	in := pp.Interactions[0]
	if in.Description != "fetch order 42" {
		t.Errorf("description: %q", in.Description)
	}
	if len(in.ProviderStates) != 1 || in.ProviderStates[0].Name != "order 42 exists" {
		t.Errorf("states: %+v", in.ProviderStates)
	}
	if in.Request.Method != "GET" {
		t.Errorf("method: %q", in.Request.Method)
	}
	if in.Response.Status != 200 {
		t.Errorf("status: %d", in.Response.Status)
	}
}

func TestParsePactDoc_V3(t *testing.T) {
	pp, err := parsePactDoc([]byte(v3Pact))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	in := pp.Interactions[0]
	if len(in.ProviderStates) != 1 || in.ProviderStates[0].Name != "data exists" {
		t.Errorf("V3 state not normalised: %+v", in.ProviderStates)
	}
}

func TestParsePactDoc_GarbageJSON(t *testing.T) {
	if _, err := parsePactDoc([]byte("<<not json>>")); err == nil {
		t.Fatal("expected parse error")
	}
}

// FuzzParsePactDoc — never panic on adversarial pact JSON.
func FuzzParsePactDoc(f *testing.F) {
	seeds := []string{
		``,
		`{}`,
		simplePact,
		v3Pact,
		`{"interactions": null}`,
		`{"interactions": [{}]}`,
		`{"interactions": [{"request": null, "response": null}]}`,
		`{"interactions": [{"response": {"status": "abc"}}]}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parsePactDoc panicked: %v", r)
			}
		}()
		_, _ = parsePactDoc(raw)
	})
}
