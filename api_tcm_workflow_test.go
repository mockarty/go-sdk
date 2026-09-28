package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTCMWorkflowPreservesFieldsAndPNG(t *testing.T) {
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 1, 2}
	filename := `before "login".png`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/namespaces/qa"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == base+"/test-cases":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload["externalId"] != "testit:100" || payload["customFields"].([]any)[0].(map[string]any)["value"] != "payments" || payload["steps"].([]any)[0].(map[string]any)["action"] != "Pay" {
				t.Errorf("case metadata lost: %+v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"c1","name":"Payment","customFields":[{"name":"Component","value":"payments"}],"steps":[{"action":"Pay"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == base+"/test-cases/c1":
			_, _ = w.Write([]byte(`{"id":"c1","name":"Payment","customFields":[{"name":"Component","value":"payments"}],"steps":[{"action":"Pay"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == base+"/test-cases":
			_, _ = w.Write([]byte(`{"test_cases":[{"id":"c1"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == base+"/tcm/attachments/upload":
			if r.URL.Query().Get("parentKind") != "tcm_case" || r.URL.Query().Get("parentId") != "c1" {
				t.Errorf("attachment owner lost: %s", r.URL.RawQuery)
			}
			file, hdr, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil || hdr.Filename != filename || hdr.Header.Get("Content-Type") != "image/png" || !bytes.Equal(body, png) {
				t.Errorf("PNG changed: name=%q type=%q data=%x err=%v", hdr.Filename, hdr.Header.Get("Content-Type"), body, err)
			}
			_, _ = w.Write([]byte(`{"id":"a1"}`))
		case r.Method == http.MethodGet && r.URL.Path == base+"/tcm/attachments":
			_, _ = w.Write([]byte(`{"items":[{"id":"a1"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == base+"/tcm/attachments/a1/raw":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		case r.Method == http.MethodPost && r.URL.Path == base+"/test-cases/c1/run":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"r1","status":"running"}`))
		case r.Method == http.MethodGet && r.URL.Path == base+"/tcm/case-runs/r1":
			_, _ = w.Write([]byte(`{"id":"r1","status":"running"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	api := NewClient(srv.URL, WithNamespace("qa")).TCM()
	caseBody := TCMObject{"name": "Payment", "externalId": "testit:100", "customFields": []TCMObject{{"name": "Component", "value": "payments"}}, "steps": []TCMObject{{"action": "Pay", "expectedResult": "Accepted"}}}
	created, err := api.CreateCase(ctx, "", caseBody)
	if err != nil || created["id"] != "c1" {
		t.Fatalf("create: %+v %v", created, err)
	}
	got, err := api.GetCase(ctx, "", "c1")
	if err != nil || got["steps"].([]any)[0].(map[string]any)["action"] != "Pay" {
		t.Fatalf("get: %+v %v", got, err)
	}
	listed, err := api.ListCases(ctx, "", nil)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %+v %v", listed, err)
	}
	attached, err := api.UploadAttachment(ctx, "", "tcm_case", "c1", filename, "image/png", bytes.NewReader(png))
	if err != nil || attached.ID != "a1" {
		t.Fatalf("upload: %+v %v", attached, err)
	}
	media, err := api.ListAttachments(ctx, "", "tcm_case", "c1")
	if err != nil || len(media) != 1 {
		t.Fatalf("list media: %+v %v", media, err)
	}
	raw, err := api.DownloadAttachment(ctx, "", "a1")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	data, err := io.ReadAll(raw)
	if err != nil || !bytes.Equal(data, png) {
		t.Fatalf("download changed PNG: %x %v", data, err)
	}
	run, err := api.RunCase(ctx, "", "c1", TCMObject{"mode": "manual"})
	if err != nil || run["id"] != "r1" {
		t.Fatalf("run: %+v %v", run, err)
	}
	readRun, err := api.GetCaseRun(ctx, "", "r1")
	if err != nil || readRun["status"] != "running" {
		t.Fatalf("read run: %+v %v", readRun, err)
	}
}

func TestTCMAttachmentRejectsUnsafeMultipartMetadata(t *testing.T) {
	api := NewClient("http://127.0.0.1:1", WithNamespace("qa")).TCM()
	for _, filename := range []string{"", "bad\r\nname.png", "bad\x00name.png", "bad\tname.png"} {
		if _, err := api.UploadAttachment(context.Background(), "", "tcm_case", "c1", filename, "image/png", bytes.NewReader([]byte("x"))); err == nil {
			t.Fatalf("accepted unsafe filename %q", filename)
		}
	}
	if _, err := api.UploadAttachment(context.Background(), "", "tcm_case", "c1", "good.png", "image/png\r\nInjected: yes", bytes.NewReader([]byte("x"))); err == nil {
		t.Fatal("accepted unsafe media type")
	}
}
