package mockarty

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestUploadMissionMaterialRejectsUnsupportedNamesAndTypesBeforeHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"reference":{"id":"unexpected"}}`))
	}))
	defer server.Close()
	api := NewClient(server.URL).CoderDelivery()
	for _, tc := range []struct{ name, media string }{
		{"a.svg", "image/svg+xml"}, {"a.bmp", "image/bmp"}, {"a.zip", "application/zip"},
		{"a.txt", "text/plain; broken"}, {"../a.txt", "text/plain"}, {`dir\a.txt`, "text/plain"},
		{" a.txt", "text/plain"}, {"a\x00.txt", "text/plain"}, {"a\u0085.txt", "text/plain"},
		{"..", "text/plain"}, {strings.Repeat("я", 65), "text/plain"}, {string([]byte{0xff}), "text/plain"},
	} {
		t.Run(tc.name+"/"+tc.media, func(t *testing.T) {
			if _, err := api.UploadMissionMaterial(context.Background(), "team", "p", tc.name, tc.media, strings.NewReader("content")); err == nil {
				t.Fatal("unsupported material accepted")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid material caused %d HTTP requests", requests.Load())
	}
}

func TestUploadMissionMaterial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/missions/materials" || r.URL.Query().Get("productId") != "product & one" || r.URL.Query().Get("namespace") != "team" {
			t.Errorf("unexpected request %s", r.URL)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "design" || header.Filename != "design.txt" || !strings.HasPrefix(header.Header.Get("Content-Type"), "text/plain") {
			t.Errorf("multipart corrupted")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"material":{"id":"mat1","sizeBytes":6},"reference":{"kind":"mission_material","id":"mat1","digest":"sha256:abc","revision":1}}`))
	}))
	defer server.Close()
	result, err := NewClient(server.URL, WithNamespace("team")).CoderDelivery().UploadMissionMaterial(context.Background(), "", "product & one", "design.txt", "", strings.NewReader("design"))
	if err != nil || result.Reference.ID != "mat1" || result.Reference.Revision != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestUploadMissionMaterialAcceptsUTF8NameBoundaryAndMediaParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"reference":{"id":"material"}}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL).CoderDelivery().UploadMissionMaterial(context.Background(), "team", "p", strings.Repeat("я", 64), "text/plain; charset=utf-8", strings.NewReader(strings.Repeat("a", 64*1024)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestUploadMissionMaterialRejectsInvalid(t *testing.T) {
	api := NewClient("http://127.0.0.1:1").CoderDelivery()
	if _, err := api.UploadMissionMaterial(context.Background(), "n", "p", "large.txt", "text/plain", strings.NewReader(strings.Repeat("a", 64*1024+1))); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("text budget not enforced: %v", err)
	}
	for _, tc := range []struct{ product, name, media, data string }{{"", "a", "text/plain", "a"}, {"p", "a", "text/plain", ""}, {"p", "a", "text/plain\r\nX: y", "a"}} {
		if _, err := api.UploadMissionMaterial(context.Background(), "n", tc.product, tc.name, tc.media, strings.NewReader(tc.data)); err == nil {
			t.Error("invalid material accepted")
		}
	}
}
