// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/pact"
)

type closeIdleSpyTransport struct {
	closed bool
}

func (*closeIdleSpyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected request")
}

func (s *closeIdleSpyTransport) CloseIdleConnections() { s.closed = true }

func TestMockServerCloseDoesNotCloseGlobalHTTPTransport(t *testing.T) {
	original := http.DefaultTransport
	spy := &closeIdleSpyTransport{}
	http.DefaultTransport = spy
	t.Cleanup(func() { http.DefaultTransport = original })

	consumer := pact.NewConsumer("front", pact.WithOutputDir(t.TempDir()))
	consumer.AddInteraction().
		UponReceiving("ping").
		WithRequest(http.MethodGet, "/ping").
		WillRespondWith(http.StatusOK)
	server, err := consumer.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	if spy.closed {
		t.Fatal("closing one pact mock server closed the process-wide HTTP transport")
	}
}

type heldRequestBody struct {
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	releaseOnce sync.Once
}

func (b *heldRequestBody) Read([]byte) (int, error) {
	b.startOnce.Do(func() { close(b.started) })
	<-b.release
	return 0, io.EOF
}

func (b *heldRequestBody) Release() { b.releaseOnce.Do(func() { close(b.release) }) }

func TestMockServerCloseWaitsForInFlightHandlerBeforeWritingPact(t *testing.T) {
	output := t.TempDir()
	consumer := pact.NewConsumer("front", pact.WithProvider("back"), pact.WithOutputDir(output))
	consumer.AddInteraction().
		UponReceiving("held request").
		WithRequest(http.MethodPost, "/held").
		WillRespondWith(http.StatusOK)
	server, err := consumer.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	body := &heldRequestBody{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() {
		body.Release()
		_ = server.Close()
	})
	request, err := http.NewRequest(http.MethodPost, server.URL()+"/held", body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Expect", "100-continue")
	transport := &http.Transport{ExpectContinueTimeout: time.Second}
	t.Cleanup(transport.CloseIdleConnections)
	responseDone := make(chan error, 1)
	go func() {
		response, requestErr := (&http.Client{Transport: transport}).Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		responseDone <- requestErr
	}()
	select {
	case <-body.started:
	case <-time.After(2 * time.Second):
		t.Fatal("request handler did not start reading the held body")
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- server.Close() }()
	select {
	case err = <-closeDone:
		t.Fatalf("Close returned before the active handler completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if files, globErr := filepath.Glob(filepath.Join(output, "*.json")); globErr != nil {
		t.Fatal(globErr)
	} else if len(files) != 0 {
		t.Fatalf("pact artifact was finalized while its handler was active: %v", files)
	}

	body.Release()
	if err = <-responseDone; err != nil {
		t.Fatalf("in-flight request was interrupted by Close: %v", err)
	}
	if err = <-closeDone; err != nil {
		t.Fatal(err)
	}
	if calls := server.Calls(); len(calls) != 1 || calls[0] != 1 {
		t.Fatalf("completed calls = %v, want [1] before artifact finalization", calls)
	}
	files, err := filepath.Glob(filepath.Join(output, "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("pact artifact files = %v, error = %v", files, err)
	}
	if payload, readErr := os.ReadFile(files[0]); readErr != nil || len(payload) == 0 {
		t.Fatalf("final pact artifact is unreadable: bytes=%d error=%v", len(payload), readErr)
	}
}
