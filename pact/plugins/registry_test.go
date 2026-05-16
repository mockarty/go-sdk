// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package plugins_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mockarty/mockarty-go/pact/plugins"
)

// stubPlugin is a goroutine-safe Plugin used to drive Registry tests.
// match is consulted in MatchRequest; setting it to nil counts as a
// matched payload.
type stubPlugin struct {
	name         string
	version      string
	contentTypes []string
	match        func(actual []byte) *plugins.MatchError
	respond      func() ([]byte, error)
}

func (s *stubPlugin) Name() string                    { return s.name }
func (s *stubPlugin) Version() string                 { return s.version }
func (s *stubPlugin) SupportedContentTypes() []string { return s.contentTypes }
func (s *stubPlugin) MatchRequest(_ context.Context, _ map[string]any, actual []byte, _ string) *plugins.MatchError {
	if s.match != nil {
		return s.match(actual)
	}
	return nil
}
func (s *stubPlugin) GenerateResponse(_ context.Context, _ map[string]any, _ string) ([]byte, error) {
	if s.respond != nil {
		return s.respond()
	}
	return nil, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	if err := r.Register(&stubPlugin{name: "x", contentTypes: []string{"application/x"}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := r.Get("x"); !ok {
		t.Fatalf("plugin not found after register")
	}
	if !r.Has("x") {
		t.Fatalf("Has should return true")
	}
}

func TestRegistryRejectsNilOrEmptyName(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	if err := r.Register(nil); err == nil {
		t.Fatalf("nil plugin must error")
	}
	if err := r.Register(&stubPlugin{name: " "}); err == nil {
		t.Fatalf("empty name must error")
	}
}

func TestRegistryResolveByContentType(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	exact := &stubPlugin{name: "exact", contentTypes: []string{"application/x-protobuf"}}
	wild := &stubPlugin{name: "wild", contentTypes: []string{"application/grpc-web+*", "application/*"}}
	catchAll := &stubPlugin{name: "fallback", contentTypes: []string{"*/*"}}
	_ = r.Register(exact)
	_ = r.Register(wild)
	_ = r.Register(catchAll)

	cases := []struct {
		mime string
		want string
	}{
		{"application/x-protobuf", "exact"},
		{"application/x-protobuf; charset=utf-8", "exact"},
		{"application/json", "wild"},
		{"text/plain", "fallback"},
		{"", ""},
	}
	for _, c := range cases {
		p, ok := r.ResolveByContentType(c.mime)
		if c.want == "" {
			if ok {
				t.Fatalf("resolve %q: expected miss, got %s", c.mime, p.Name())
			}
			continue
		}
		if !ok {
			t.Fatalf("resolve %q: expected hit", c.mime)
		}
		if p.Name() != c.want {
			t.Fatalf("resolve %q = %q; want %q", c.mime, p.Name(), c.want)
		}
	}
}

func TestRegistryUnregisterIsIdempotent(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	_ = r.Register(&stubPlugin{name: "x", contentTypes: []string{"application/x"}})
	r.Unregister("x")
	r.Unregister("x") // second call must be a no-op
	if _, ok := r.Get("x"); ok {
		t.Fatalf("plugin still present after unregister")
	}
	if _, ok := r.ResolveByContentType("application/x"); ok {
		t.Fatalf("content-type index not pruned")
	}
}

func TestRegistryNamesDeterministicOrder(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	for _, n := range []string{"zeta", "alpha", "mu"} {
		_ = r.Register(&stubPlugin{name: n})
	}
	names := r.Names()
	want := []string{"alpha", "mu", "zeta"}
	if len(names) != len(want) {
		t.Fatalf("names len = %d", len(names))
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v; want %v", names, want)
		}
	}
}

func TestRegistryConcurrentSafe(t *testing.T) {
	t.Parallel()
	r := plugins.NewRegistry()
	const n = 64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := "p" + string(rune('a'+i%26))
			_ = r.Register(&stubPlugin{name: name, contentTypes: []string{"application/" + name}})
			_, _ = r.Get(name)
			_, _ = r.ResolveByContentType("application/" + name)
		}()
	}
	wg.Wait()
	if len(r.Names()) == 0 {
		t.Fatalf("no plugins registered after concurrent burst")
	}
}

func TestMatchErrorFormatting(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  *plugins.MatchError
		want string
	}{
		{&plugins.MatchError{Reason: "bad"}, "bad"},
		{&plugins.MatchError{Reason: "bad", Cause: errors.New("root")}, "bad: root"},
		{&plugins.MatchError{Path: "$.x", Reason: "bad"}, "$.x: bad"},
		{&plugins.MatchError{Path: "$.x", Reason: "bad", Cause: errors.New("root")}, "$.x: bad: root"},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Fatalf("Error() = %q; want %q", got, c.want)
		}
	}
	var nilErr *plugins.MatchError
	if got := nilErr.Error(); got != "" {
		t.Fatalf("nil Error() = %q", got)
	}
	if nilErr.Unwrap() != nil {
		t.Fatalf("nil Unwrap should be nil")
	}
	w := &plugins.MatchError{Cause: errors.New("root")}
	if !errors.Is(w, w.Cause) {
		t.Fatalf("errors.Is should walk Cause")
	}
}

func TestPackageLevelDefaultRegistry(t *testing.T) {
	// NOT t.Parallel() — touches process-global Default.
	plugins.Reset()
	defer plugins.Reset()
	p := &stubPlugin{name: "lvl", contentTypes: []string{"application/lvl"}}
	if err := plugins.Register(p); err != nil {
		t.Fatalf("register: %v", err)
	}
	if got, ok := plugins.Get("lvl"); !ok || got.Name() != "lvl" {
		t.Fatalf("Get miss")
	}
	if got, ok := plugins.ResolveByContentType("application/lvl"); !ok || got.Name() != "lvl" {
		t.Fatalf("ResolveByContentType miss")
	}
	if !errors.Is(plugins.ErrPluginNotFound, plugins.ErrPluginNotFound) {
		t.Fatalf("sentinel must be comparable to itself")
	}
}
