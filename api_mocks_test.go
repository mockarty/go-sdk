// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer creates a test server with a simple path-prefix router.
// Handlers are matched by "METHOD /path-prefix" — for example
// "POST /api/v1/mocks" matches POST requests to /api/v1/mocks exactly,
// and "GET /api/v1/mocks/" matches GET requests to any path starting with /api/v1/mocks/.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try exact match first (e.g., "POST /api/v1/mocks")
		key := r.Method + " " + r.URL.Path
		if h, ok := handlers[key]; ok {
			h(w, r)
			return
		}

		// Try prefix match (e.g., "GET /api/v1/mocks/" matches /api/v1/mocks/some-id)
		for pattern, h := range handlers {
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) != 2 {
				continue
			}
			method, pathPrefix := parts[0], parts[1]
			if r.Method == method && strings.HasPrefix(r.URL.Path, pathPrefix) {
				h(w, r)
				return
			}
		}

		// Also try matching with query string stripped
		cleanPath := r.URL.Path
		for pattern, h := range handlers {
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) != 2 {
				continue
			}
			method, pathPrefix := parts[0], parts[1]
			if r.Method == method && strings.HasPrefix(cleanPath, pathPrefix) {
				h(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"test server: no handler for ` + key + `"}`))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.URL, WithAPIKey("test-key"))
	return srv, client
}

func TestMockAPI_Create(t *testing.T) {
	tests := []struct {
		name            string
		mock            *Mock
		serverResp      string
		serverCode      int
		wantErr         bool
		wantOverwritten bool
	}{
		{
			name: "create new mock",
			mock: &Mock{
				ID:        "test-create",
				Namespace: "sandbox",
				HTTP: &HttpRequestContext{
					Route:      "/api/test",
					HttpMethod: "GET",
				},
				Response: &ContentResponse{
					StatusCode: 200,
					Payload:    map[string]any{"ok": true},
				},
			},
			serverResp:      `{"overwritten":false,"mock":{"id":"test-create","namespace":"sandbox"}}`,
			serverCode:      http.StatusOK,
			wantErr:         false,
			wantOverwritten: false,
		},
		{
			name: "overwrite existing mock",
			mock: &Mock{
				ID:        "test-overwrite",
				Namespace: "sandbox",
			},
			serverResp:      `{"overwritten":true,"mock":{"id":"test-overwrite","namespace":"sandbox"}}`,
			serverCode:      http.StatusOK,
			wantErr:         false,
			wantOverwritten: true,
		},
		{
			name:       "server error",
			mock:       &Mock{ID: "bad"},
			serverResp: `{"error":"internal error"}`,
			serverCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := newTestServer(t, map[string]http.HandlerFunc{
				"POST /api/v1/mocks": func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					var mock Mock
					if err := json.Unmarshal(body, &mock); err != nil {
						t.Errorf("failed to decode request body: %v", err)
					}

					if r.Header.Get("X-API-Key") != "test-key" {
						t.Error("expected API key in request")
					}

					w.WriteHeader(tt.serverCode)
					_, _ = w.Write([]byte(tt.serverResp))
				},
			})

			resp, err := client.Mocks().Create(context.Background(), tt.mock)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Overwritten != tt.wantOverwritten {
				t.Errorf("overwritten: got %v, want %v", resp.Overwritten, tt.wantOverwritten)
			}
		})
	}
}

func TestMockAPI_Create_DefaultNamespace(t *testing.T) {
	var gotNamespace string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var mock Mock
			_ = json.Unmarshal(body, &mock)
			gotNamespace = mock.Namespace
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"overwritten":false,"mock":{"id":"test","namespace":"sandbox"}}`))
		},
	})

	_, _ = client.Mocks().Create(context.Background(), &Mock{ID: "test"})
	if gotNamespace != "sandbox" {
		t.Errorf("expected default namespace 'sandbox', got %q", gotNamespace)
	}
}

func TestMockAPI_Get(t *testing.T) {
	tests := []struct {
		name       string
		mockID     string
		serverResp string
		serverCode int
		wantErr    bool
		wantErrIs  error
	}{
		{
			name:       "found",
			mockID:     "existing-mock",
			serverResp: `{"id":"existing-mock","namespace":"sandbox","http":{"route":"/api/test","httpMethod":"GET"}}`,
			serverCode: http.StatusOK,
		},
		{
			name:       "not found",
			mockID:     "missing",
			serverResp: `{"error":"mock not found"}`,
			serverCode: http.StatusNotFound,
			wantErr:    true,
			wantErrIs:  ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := newTestServer(t, map[string]http.HandlerFunc{
				"GET /api/v1/mocks/": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.serverCode)
					_, _ = w.Write([]byte(tt.serverResp))
				},
			})

			mock, err := client.Mocks().Get(context.Background(), tt.mockID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected errors.Is(%v), got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mock.ID != tt.mockID {
				t.Errorf("expected ID %q, got %q", tt.mockID, mock.ID)
			}
		})
	}
}

func TestMockAPI_List(t *testing.T) {
	tests := []struct {
		name       string
		opts       *ListMocksOptions
		serverResp string
	}{
		{
			name:       "no options uses default namespace",
			opts:       nil,
			serverResp: `{"items":[{"id":"mock-1"}],"total":1}`,
		},
		{
			name: "with namespace and tags",
			opts: &ListMocksOptions{
				Namespace: "production",
				Tags:      []string{"users", "v2"},
				Search:    "test",
				Offset:    10,
				Limit:     20,
			},
			serverResp: `{"items":[{"id":"mock-1"},{"id":"mock-2"}],"total":50}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			_, client := newTestServer(t, map[string]http.HandlerFunc{
				"GET /api/v1/mocks": func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.String()
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(tt.serverResp))
				},
			})

			resp, err := client.Mocks().List(context.Background(), tt.opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.Total == 0 && len(resp.Items) > 0 {
				t.Error("unexpected empty total with items")
			}

			if tt.opts != nil {
				if tt.opts.Namespace != "" && !strings.Contains(gotPath, "namespace="+tt.opts.Namespace) {
					t.Errorf("expected namespace in query, got path: %s", gotPath)
				}
				if tt.opts.Search != "" && !strings.Contains(gotPath, "search="+tt.opts.Search) {
					t.Errorf("expected search in query, got path: %s", gotPath)
				}
			}
		})
	}
}

func TestMockAPI_Delete(t *testing.T) {
	tests := []struct {
		name       string
		mockID     string
		serverCode int
		wantErr    bool
	}{
		{
			name:       "success",
			mockID:     "to-delete",
			serverCode: http.StatusOK,
		},
		{
			name:       "not found",
			mockID:     "missing",
			serverCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod string
			_, client := newTestServer(t, map[string]http.HandlerFunc{
				"DELETE /api/v1/mocks/": func(w http.ResponseWriter, r *http.Request) {
					gotMethod = r.Method
					w.WriteHeader(tt.serverCode)
					if tt.serverCode != http.StatusOK {
						_, _ = w.Write([]byte(`{"error":"not found"}`))
					}
				},
			})

			err := client.Mocks().Delete(context.Background(), tt.mockID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != "DELETE" {
				t.Errorf("expected DELETE method, got %s", gotMethod)
			}
		})
	}
}

func TestMockAPI_Restore(t *testing.T) {
	var gotBody struct {
		IDs []string `json:"ids"`
	}
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks/batch/restore": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Mocks().Restore(context.Background(), "restored-mock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotBody.IDs) != 1 || gotBody.IDs[0] != "restored-mock" {
		t.Errorf("expected IDs [restored-mock], got %v", gotBody.IDs)
	}
}

func TestMockAPI_Update(t *testing.T) {
	var gotBody Mock
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"overwritten":true,"mock":{"id":"updated-mock"}}`))
		},
	})

	mock := &Mock{
		Namespace: "sandbox",
		HTTP: &HttpRequestContext{
			Route:      "/api/updated",
			HttpMethod: "PUT",
		},
	}

	result, err := client.Mocks().Update(context.Background(), "updated-mock", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "updated-mock" {
		t.Errorf("expected ID updated-mock, got %q", result.ID)
	}
	if gotBody.ID != "updated-mock" {
		t.Errorf("expected ID in body to be set, got %q", gotBody.ID)
	}
}

func TestMockAPI_BatchCreate(t *testing.T) {
	var gotMocks []*Mock
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks/batch": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotMocks)
			w.WriteHeader(http.StatusOK)
		},
	})

	mocks := []*Mock{
		{ID: "batch-1"},
		{ID: "batch-2"},
		{ID: "batch-3"},
	}

	err := client.Mocks().BatchCreate(context.Background(), mocks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotMocks) != 3 {
		t.Errorf("expected 3 mocks in batch, got %d", len(gotMocks))
	}
}

func TestMockAPI_BatchDelete(t *testing.T) {
	var gotBody struct {
		IDs []string `json:"ids"`
	}
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/mocks/batch": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Mocks().BatchDelete(context.Background(), []string{"d1", "d2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotBody.IDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(gotBody.IDs))
	}
}

func TestMockAPI_BatchRestore(t *testing.T) {
	var gotBody struct {
		IDs []string `json:"ids"`
	}
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks/batch/restore": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Mocks().BatchRestore(context.Background(), []string{"r1", "r2", "r3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotBody.IDs) != 3 {
		t.Errorf("expected 3 IDs, got %d", len(gotBody.IDs))
	}
}

func TestMockAPI_Logs(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/mocks/": func(w http.ResponseWriter, r *http.Request) {
			resp := MockLogs{
				ID: "log-mock",
				Requests: []RequestLog{
					{ID: "log-1", CalledAt: "2024-01-01T00:00:00Z"},
					{ID: "log-2", CalledAt: "2024-01-01T01:00:00Z"},
				},
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		},
	})

	logs, err := client.Mocks().Logs(context.Background(), "log-mock", &LogsOptions{
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs.ID != "log-mock" {
		t.Errorf("expected ID log-mock, got %q", logs.ID)
	}
	if len(logs.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(logs.Requests))
	}
}

func TestMockAPI_Logs_NoOptions(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/mocks/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.String()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"test","requests":[]}`))
		},
	})

	_, err := client.Mocks().Logs(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(gotPath, "limit") || strings.Contains(gotPath, "offset") {
		t.Errorf("expected no query params with nil opts, got %s", gotPath)
	}
}

func TestMockAPI_Versions(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/mocks/chains/": func(w http.ResponseWriter, r *http.Request) {
			mocks := []*Mock{
				{ID: "v1", ChainID: "chain-1"},
				{ID: "v2", ChainID: "chain-1"},
			}
			data, _ := json.Marshal(mocks)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		},
	})

	versions, err := client.Mocks().Versions(context.Background(), "chain-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestMockAPI_BatchCreate_Failure(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks/batch": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid mock in batch"}`))
		},
	})

	mocks := []*Mock{
		{ID: "ok-1"},
		{ID: "fail"},
	}

	err := client.Mocks().BatchCreate(context.Background(), mocks)
	if err == nil {
		t.Fatal("expected error on batch failure")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error should contain server message, got: %v", err)
	}
}

// TestNamespaceAPI tests namespace operations.
func TestNamespaceAPI_Create(t *testing.T) {
	var gotName string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/namespaces": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(body, &req)
			gotName = req.Name
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Namespaces().Create(context.Background(), "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "production" {
		t.Errorf("expected namespace name 'production', got %q", gotName)
	}
}

func TestNamespaceAPI_List(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/namespaces": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`["sandbox","production","staging"]`))
		},
	})

	namespaces, err := client.Namespaces().List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(namespaces) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(namespaces))
	}
	if namespaces[0] != "sandbox" {
		t.Errorf("expected first namespace 'sandbox', got %q", namespaces[0])
	}
}

// TestStoreAPI tests store operations.
func TestStoreAPI_GlobalGet(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/stores/global": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"counter":42,"name":"test"}`))
		},
	})

	store, err := client.Stores().GlobalGet(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store["counter"] != float64(42) {
		t.Errorf("expected counter=42, got %v", store["counter"])
	}
}

func TestStoreAPI_GlobalSet(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/stores/global": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().GlobalSet(context.Background(), "counter", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["key"] != "counter" {
		t.Errorf("expected key=counter in body, got %v", gotBody["key"])
	}
	if gotBody["value"] != float64(42) {
		t.Errorf("expected value=42 in body, got %v", gotBody["value"])
	}
}

func TestStoreAPI_GlobalDelete(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/stores/global/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().GlobalDelete(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "mykey") {
		t.Errorf("expected path to contain key, got %s", gotPath)
	}
}

func TestStoreAPI_ChainGet(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/v1/stores/chain/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"orderStatus":"pending"}`))
		},
	})

	store, err := client.Stores().ChainGet(context.Background(), "chain-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store["orderStatus"] != "pending" {
		t.Errorf("expected orderStatus=pending, got %v", store["orderStatus"])
	}
}

func TestStoreAPI_ChainSet(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/stores/chain/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().ChainSet(context.Background(), "chain-1", "status", "completed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["key"] != "status" {
		t.Errorf("expected key=status in body, got %v", gotBody["key"])
	}
	if gotBody["value"] != "completed" {
		t.Errorf("expected value=completed in body, got %v", gotBody["value"])
	}
}

func TestStoreAPI_ChainDelete(t *testing.T) {
	var gotPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/stores/chain/": func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().ChainDelete(context.Background(), "chain-1", "key1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotPath, "chain-1/key1") {
		t.Errorf("expected path to contain chain-1/key1, got %s", gotPath)
	}
}

// TestStoreAPI_GlobalSet_IncludesNamespace verifies that namespace is sent in the POST body.
func TestStoreAPI_GlobalSet_IncludesNamespace(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/stores/global": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().GlobalSet(context.Background(), "env", "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["namespace"] != "sandbox" {
		t.Errorf("expected namespace=sandbox in body, got %v", gotBody["namespace"])
	}
}

// TestStoreAPI_ChainSet_IncludesNamespace verifies namespace in chain POST body.
func TestStoreAPI_ChainSet_IncludesNamespace(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/stores/chain/": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().ChainSet(context.Background(), "flow-1", "step", "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["namespace"] != "sandbox" {
		t.Errorf("expected namespace=sandbox in body, got %v", gotBody["namespace"])
	}
}

// TestStoreAPI_GlobalDeleteMany verifies batch delete calls individual endpoints.
func TestStoreAPI_GlobalDeleteMany(t *testing.T) {
	var deletedKeys []string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/stores/global/": func(w http.ResponseWriter, r *http.Request) {
			// Extract key from path: /api/v1/stores/global/{key}
			path := r.URL.Path
			key := path[len("/api/v1/stores/global/"):]
			deletedKeys = append(deletedKeys, key)
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().GlobalDeleteMany(context.Background(), "a", "b", "c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deletedKeys) != 3 {
		t.Errorf("expected 3 delete calls, got %d", len(deletedKeys))
	}
}

// TestStoreAPI_GlobalDelete_URLEncoding verifies keys with special chars are URL-encoded.
func TestStoreAPI_GlobalDelete_URLEncoding(t *testing.T) {
	var gotRawPath string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/v1/stores/global/": func(w http.ResponseWriter, r *http.Request) {
			gotRawPath = r.RequestURI
			w.WriteHeader(http.StatusOK)
		},
	})

	err := client.Stores().GlobalDelete(context.Background(), "key with spaces")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(gotRawPath, "key with spaces") {
		t.Errorf("key should be URL-encoded, but raw path was: %s", gotRawPath)
	}
	if !strings.Contains(gotRawPath, "key%20with%20spaces") {
		t.Errorf("expected URL-encoded key in path, got: %s", gotRawPath)
	}
}

// TestHealthAPI tests health check operations.
func TestHealthAPI_Check(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /health": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"pass","releaseId":"1.2.3","checks":{"database":[{"status":"pass","time":"2024-01-01T00:00:00Z"}]}}`))
		},
	})

	resp, err := client.Health().Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != HealthStatusPass {
		t.Errorf("expected status pass, got %q", resp.Status)
	}
	if resp.ReleaseID != "1.2.3" {
		t.Errorf("expected releaseId 1.2.3, got %q", resp.ReleaseID)
	}
	if len(resp.Checks["database"]) != 1 {
		t.Error("expected database check")
	}
}

func TestHealthAPI_Live(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /health/live": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		},
	})

	err := client.Health().Live(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthAPI_Ready(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /health/ready": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		},
	})

	err := client.Health().Ready(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthAPI_Ready_NotReady(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /health/ready": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"not ready"}`))
		},
	})

	err := client.Health().Ready(context.Background())
	if err == nil {
		t.Fatal("expected error when server is not ready")
	}
}

func TestMockAPI_Create_FullRoundtrip(t *testing.T) {
	var gotBody map[string]any
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/v1/mocks": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"overwritten":false,"mock":` + string(body) + `}`))
		},
	})

	mock := NewMockBuilder().
		ID("roundtrip-test").
		Namespace("sandbox").
		Tags("test", "sdk").
		HTTP(func(h *HTTPBuilder) {
			h.Route("/api/users/:id").
				Method("GET").
				HeaderCondition("Authorization", AssertNotEmpty, nil)
		}).
		Response(func(r *ResponseBuilder) {
			r.Status(200).
				Header("Content-Type", "application/json").
				JSONBody(map[string]any{
					"id":   "$.pathParam.id",
					"name": "$.fake.FirstName",
				}).
				Delay(50)
		}).
		TTL(3600).
		Priority(5).
		Build()

	resp, err := client.Mocks().Create(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Mock.ID != "roundtrip-test" {
		t.Errorf("expected ID roundtrip-test, got %q", resp.Mock.ID)
	}

	if gotBody["id"] != "roundtrip-test" {
		t.Error("missing id in serialized body")
	}
	if gotBody["namespace"] != "sandbox" {
		t.Error("missing namespace in serialized body")
	}

	httpCtx, ok := gotBody["http"].(map[string]any)
	if !ok {
		t.Fatal("missing http context in serialized body")
	}
	if httpCtx["route"] != "/api/users/:id" {
		t.Errorf("expected route /api/users/:id, got %v", httpCtx["route"])
	}
	if httpCtx["httpMethod"] != "GET" {
		t.Errorf("expected httpMethod GET, got %v", httpCtx["httpMethod"])
	}

	responseCtx, ok := gotBody["response"].(map[string]any)
	if !ok {
		t.Fatal("missing response in serialized body")
	}
	if responseCtx["statusCode"] != float64(200) {
		t.Errorf("expected statusCode 200, got %v", responseCtx["statusCode"])
	}
	if responseCtx["delay"] != float64(50) {
		t.Errorf("expected delay 50, got %v", responseCtx["delay"])
	}
}
