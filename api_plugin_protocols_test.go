// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"testing"
)

func TestMockAPIListPluginProtocols(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/plugin-protocols": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("namespace"); got != "team a" {
				t.Fatalf("namespace = %q, want team a", got)
			}
			_, _ = w.Write([]byte(`{"protocols":[{"key":"acme-line","name":"ACME Line","transport":"tcp-line","magic":"ACME ","pluginId":"acme.codec","mockProtocol":"socket","serverName":"plugin:acme-line"}],"count":1,"listener":"enabled","usage":"socket mock"}`))
		},
	})
	client.namespace = "team a"

	got, err := client.Mocks().ListPluginProtocols(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || len(got.Protocols) != 1 {
		t.Fatalf("catalogue = %+v", got)
	}
	if protocol := got.Protocols[0]; protocol.ServerName != "plugin:acme-line" || protocol.PluginID != "acme.codec" {
		t.Fatalf("protocol = %+v", protocol)
	}
}
