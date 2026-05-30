// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package s3

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// inMemoryS3 is a tiny path-style S3 stub for client tests — just enough
// wire behaviour (PUT/GET/HEAD/DELETE/list) to exercise the client.
func inMemoryS3(t *testing.T) (*httptest.Server, map[string][]byte) {
	t.Helper()
	store := map[string][]byte{}
	meta := map[string]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/")
		switch r.Method {
		case http.MethodPut:
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			store[key] = buf
			if m := r.Header.Get("x-amz-meta-owner"); m != "" {
				meta[key] = m
			}
			w.Header().Set("ETag", `"deadbeef"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if strings.HasSuffix(r.URL.Path, "/") || r.URL.Query().Get("list-type") != "" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`<?xml version="1.0"?><ListBucketResult><IsTruncated>false</IsTruncated><Contents><Key>k1</Key><Size>3</Size><ETag>"e1"</ETag><LastModified>2026-01-01T00:00:00Z</LastModified></Contents></ListBucketResult>`))
				return
			}
			b, ok := store[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`<Error><Code>NoSuchKey</Code><Message>not found</Message></Error>`))
				return
			}
			if m := meta[key]; m != "" {
				w.Header().Set("x-amz-meta-owner", m)
			}
			w.Header().Set("ETag", `"deadbeef"`)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(b)
		case http.MethodHead:
			if _, ok := store[key]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("ETag", `"deadbeef"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			delete(store, key)
			w.WriteHeader(http.StatusNoContent)
		}
	})
	return httptest.NewServer(mux), store
}

func TestClientLifecycle(t *testing.T) {
	ts, store := inMemoryS3(t)
	defer ts.Close()
	cli := NewClient(ts.URL)
	ctx := context.Background()

	put, err := cli.PutObject(ctx, "bucket", "k1", []byte("abc"), "text/plain", map[string]string{"owner": "fin"})
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if put.StatusCode != 200 || put.ETag != "deadbeef" {
		t.Fatalf("put result: %+v", put)
	}
	if string(store["bucket/k1"]) != "abc" {
		t.Fatalf("stored body: %q", store["bucket/k1"])
	}

	get, err := cli.GetObject(ctx, "bucket", "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(get.Body) != "abc" || get.ContentType != "text/plain" || get.Metadata["owner"] != "fin" {
		t.Fatalf("get result: %+v", get)
	}

	head, err := cli.HeadObject(ctx, "bucket", "k1")
	if err != nil || !head.Exists {
		t.Fatalf("head: %+v err=%v", head, err)
	}
	missing, _ := cli.HeadObject(ctx, "bucket", "nope")
	if missing.Exists {
		t.Fatalf("head missing should not exist: %+v", missing)
	}

	list, err := cli.ListObjects(ctx, "bucket", "")
	if err != nil || len(list.Objects) != 1 || list.Objects[0].Key != "k1" {
		t.Fatalf("list: %+v err=%v", list, err)
	}

	del, err := cli.DeleteObject(ctx, "bucket", "k1")
	if err != nil || del.StatusCode != 204 {
		t.Fatalf("delete: %+v err=%v", del, err)
	}

	if _, gerr := cli.GetObject(ctx, "bucket", "k1"); gerr == nil {
		t.Fatalf("expected error getting deleted key")
	}
}

func TestClientGetMissingError(t *testing.T) {
	ts, _ := inMemoryS3(t)
	defer ts.Close()
	cli := NewClient(ts.URL)
	_, err := cli.GetObject(context.Background(), "bucket", "ghost")
	if err == nil || !strings.Contains(err.Error(), "NoSuchKey") {
		t.Fatalf("want NoSuchKey error, got %v", err)
	}
}

func TestClientSignerInvoked(t *testing.T) {
	ts, _ := inMemoryS3(t)
	defer ts.Close()
	called := false
	cli := NewClient(ts.URL, WithRequestSigner(func(r *http.Request) error {
		called = true
		r.Header.Set("Authorization", "AWS4-HMAC-SHA256 ...")
		return nil
	}))
	if _, err := cli.PutObject(context.Background(), "b", "k", []byte("x"), "", nil); err != nil {
		t.Fatalf("put: %v", err)
	}
	if !called {
		t.Fatalf("signer was not invoked")
	}
}

func TestClientEmptyBucket(t *testing.T) {
	cli := NewClient("http://example.invalid")
	if _, err := cli.GetObject(context.Background(), "", "k"); err == nil {
		t.Fatalf("expected empty-bucket error")
	}
}
