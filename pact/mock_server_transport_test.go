// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

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
