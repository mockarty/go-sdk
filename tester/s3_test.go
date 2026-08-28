// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"errors"
	"testing"

	"github.com/mockarty/mockarty-go/protocols/s3"
)

// fakeS3 is an in-memory S3Client — no real endpoint required.
type fakeS3 struct {
	objects  map[string]fakeObj // key "bucket/key"
	forceErr error
}

type fakeObj struct {
	meta        map[string]string
	contentType string
	etag        string
	body        []byte
}

func newFakeS3() *fakeS3 { return &fakeS3{objects: map[string]fakeObj{}} }

func (f *fakeS3) PutObject(_ context.Context, bucket, key string, body []byte, ct string, meta map[string]string) (s3.PutResult, error) {
	if f.forceErr != nil {
		return s3.PutResult{}, f.forceErr
	}
	f.objects[bucket+"/"+key] = fakeObj{body: append([]byte(nil), body...), contentType: ct, meta: meta, etag: "etag-" + key}
	return s3.PutResult{StatusCode: 200, ETag: "etag-" + key}, nil
}

func (f *fakeS3) GetObject(_ context.Context, bucket, key string) (s3.GetResult, error) {
	o, ok := f.objects[bucket+"/"+key]
	if !ok {
		return s3.GetResult{StatusCode: 404}, errors.New("NoSuchKey")
	}
	return s3.GetResult{StatusCode: 200, Body: o.body, ContentType: o.contentType, ETag: o.etag, Metadata: o.meta}, nil
}

func (f *fakeS3) HeadObject(_ context.Context, bucket, key string) (s3.HeadResult, error) {
	o, ok := f.objects[bucket+"/"+key]
	if !ok {
		return s3.HeadResult{StatusCode: 404, Exists: false}, nil
	}
	return s3.HeadResult{StatusCode: 200, Exists: true, ContentType: o.contentType, ETag: o.etag, Metadata: o.meta, ContentLength: int64(len(o.body))}, nil
}

func (f *fakeS3) ListObjects(_ context.Context, bucket, prefix string) (s3.ListResult, error) {
	res := s3.ListResult{StatusCode: 200}
	for k, o := range f.objects {
		if len(k) <= len(bucket) || k[:len(bucket)+1] != bucket+"/" {
			continue
		}
		key := k[len(bucket)+1:]
		if prefix != "" && (len(key) < len(prefix) || key[:len(prefix)] != prefix) {
			continue
		}
		res.Objects = append(res.Objects, s3.ObjectInfo{Key: key, Size: int64(len(o.body)), ETag: o.etag})
	}
	return res, nil
}

func (f *fakeS3) DeleteObject(_ context.Context, bucket, key string) (s3.DeleteResult, error) {
	delete(f.objects, bucket+"/"+key)
	return s3.DeleteResult{StatusCode: 204}, nil
}

func TestS3PutGetLifecycle(t *testing.T) {
	cli := newFakeS3()
	tst := New()
	tst.S3(cli).Put("reports", "q1.csv").
		Body("a,b,c").ContentType("text/csv").Meta("owner", "finance").
		ExpectOK().ExpectStatus(200).ExtractETag("etag")
	tst.S3(cli).Get("reports", "q1.csv").
		ExpectOK().
		ExpectBodyEquals("a,b,c").
		ExpectBodyContains("b,c").
		ExpectContentType("text/csv").
		ExpectMeta("owner", "finance")
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
	if got := tst.Vars()["etag"]; got != "etag-q1.csv" {
		t.Fatalf("ExtractETag: want etag-q1.csv, got %q", got)
	}
}

func TestS3HeadAndList(t *testing.T) {
	cli := newFakeS3()
	tst := New()
	tst.S3(cli).Put("b", "k1").Body("x").ExpectOK()
	tst.S3(cli).Put("b", "k2").Body("yy").ExpectOK()
	tst.S3(cli).Head("b", "k1").ExpectExists().ExpectStatus(200)
	tst.S3(cli).Head("b", "missing").ExpectAbsent().ExpectStatus(404)
	tst.S3(cli).List("b").ExpectObjectCount(2).ExpectKey("k1").ExpectKey("k2")
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
}

func TestS3DeleteThenGetMissing(t *testing.T) {
	cli := newFakeS3()
	tst := New()
	tst.S3(cli).Put("b", "k").Body("v").ExpectOK()
	tst.S3(cli).Delete("b", "k").ExpectOK().ExpectStatus(204)
	tst.S3(cli).Get("b", "k").ExpectError().ExpectStatus(404)
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
}

func TestS3Negative(t *testing.T) {
	cases := []struct {
		name    string
		run     func(tst *Tester, cli *fakeS3)
		wantErr bool
	}{
		{
			name: "expect-ok-on-missing-fails",
			run: func(tst *Tester, cli *fakeS3) {
				tst.S3(cli).Get("b", "nope").ExpectOK()
			},
			wantErr: true,
		},
		{
			name: "wrong-body-fails",
			run: func(tst *Tester, cli *fakeS3) {
				tst.S3(cli).Put("b", "k").Body("real").ExpectOK()
				tst.S3(cli).Get("b", "k").ExpectBodyEquals("wrong")
			},
			wantErr: true,
		},
		{
			name: "wrong-meta-fails",
			run: func(tst *Tester, cli *fakeS3) {
				tst.S3(cli).Put("b", "k").Body("v").Meta("a", "1").ExpectOK()
				tst.S3(cli).Get("b", "k").ExpectMeta("a", "2")
			},
			wantErr: true,
		},
		{
			name: "list-count-mismatch-fails",
			run: func(tst *Tester, cli *fakeS3) {
				tst.S3(cli).Put("b", "k").Body("v").ExpectOK()
				tst.S3(cli).List("b").ExpectObjectCount(5)
			},
			wantErr: true,
		},
		{
			name: "put-transport-error-fails",
			run: func(tst *Tester, cli *fakeS3) {
				cli.forceErr = errors.New("boom")
				tst.S3(cli).Put("b", "k").Body("v").ExpectOK()
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := newFakeS3()
			tst := New()
			tc.run(tst, cli)
			tst.Finish()
			if tst.OK() == tc.wantErr {
				t.Fatalf("OK()=%v wantErr=%v errs=%v", tst.OK(), tc.wantErr, tst.Errors())
			}
		})
	}
}

func TestS3Interpolation(t *testing.T) {
	cli := newFakeS3()
	tst := New()
	tst.SetVar("bkt", "dyn-bucket")
	tst.SetVar("name", "report.txt")
	tst.S3(cli).Put("{{bkt}}", "{{name}}").Body("payload").ExpectOK()
	tst.S3(cli).Get("{{bkt}}", "{{name}}").ExpectBodyEquals("payload")
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("interpolation failed: %v", tst.Errors())
	}
	if _, ok := cli.objects["dyn-bucket/report.txt"]; !ok {
		t.Fatalf("interpolated key not stored; have %v", cli.objects)
	}
}
