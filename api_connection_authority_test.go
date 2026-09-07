package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionAuthorityLifecycleUsesNamespaceAndExactCAS(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.EscapedPath() != "/api/v1/namespaces/team-a/connections/gitlab-prod" && calls != 1 {
			t.Fatalf("path=%s", r.URL.EscapedPath())
		}
		switch calls {
		case 1:
			if r.Method != http.MethodPost || r.URL.EscapedPath() != "/api/v1/namespaces/team-a/connections" {
				t.Fatalf("create %s %s", r.Method, r.URL.EscapedPath())
			}
			var body ConnectionDescriptor
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Namespace != "" || body.Revision != 0 || body.ID != "gitlab-prod" {
				t.Fatalf("create body=%+v", body)
			}
		case 2:
			if r.Method != http.MethodPut || r.URL.Query().Get("expectedRevision") != "1" {
				t.Fatalf("advance %s %s", r.Method, r.URL.String())
			}
		case 3:
			if r.Method != http.MethodGet {
				t.Fatalf("get %s", r.Method)
			}
		case 4:
			if r.Method != http.MethodDelete || r.URL.Query().Get("revision") != "2" {
				t.Fatalf("revoke %s %s", r.Method, r.URL.String())
			}
		}
		if calls != 4 {
			_ = json.NewEncoder(w).Encode(ConnectionSnapshot{Frozen: true, Descriptor: ConnectionDescriptor{ID: "gitlab-prod", Revision: int64(calls)}})
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	api := NewClient(server.URL, WithNamespace("team-a")).Connections()
	d := ConnectionDescriptor{ContractVersion: "mockarty.connection/v1", ID: "gitlab-prod", Kind: "gitlab"}
	if _, err := api.Create(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Advance(context.Background(), d, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GetCurrent(context.Background(), "", d.ID); err != nil {
		t.Fatal(err)
	}
	if err := api.Revoke(context.Background(), "", d.ID, 2); err != nil {
		t.Fatal(err)
	}
}
