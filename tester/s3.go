// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mockarty/mockarty-go/protocols/s3"
)

// S3Client is the minimal contract the S3 facet needs. `*s3.Client` from
// `protocols/s3` satisfies it directly; tests pass an in-memory fake so
// no real S3 endpoint is required. Keeping it an interface (mirroring
// SQLConn / KafkaBroker) lets users plug MinIO/AWS via a thin adapter.
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, body []byte, contentType string, metadata map[string]string) (s3.PutResult, error)
	GetObject(ctx context.Context, bucket, key string) (s3.GetResult, error)
	HeadObject(ctx context.Context, bucket, key string) (s3.HeadResult, error)
	ListObjects(ctx context.Context, bucket, prefix string) (s3.ListResult, error)
	DeleteObject(ctx context.Context, bucket, key string) (s3.DeleteResult, error)
}

// S3Facet is the S3 entry point reached via Tester.S3(client).
type S3Facet struct {
	t      *Tester
	client S3Client
}

// S3 returns the S3 facet bound to the supplied client.
//
//	cli := s3.NewClient("http://localhost:18770/s3")
//	t.S3(cli).
//	  Put("reports", "q1.csv").Body("a,b,c").ContentType("text/csv").
//	  Meta("owner", "finance").ExpectOK()
//	t.S3(cli).Get("reports", "q1.csv").ExpectOK().ExpectBodyContains("a,b,c")
func (t *Tester) S3(client S3Client) *S3Facet {
	t.flushPending()
	return &S3Facet{t: t, client: client}
}

// s3Op enumerates the operation a step performs.
type s3Op int

const (
	s3OpPut s3Op = iota
	s3OpGet
	s3OpHead
	s3OpList
	s3OpDelete
)

func (o s3Op) String() string {
	switch o {
	case s3OpPut:
		return "put"
	case s3OpGet:
		return "get"
	case s3OpHead:
		return "head"
	case s3OpList:
		return "list"
	case s3OpDelete:
		return "delete"
	}
	return "?"
}

// Put starts a PutObject chain.
func (f *S3Facet) Put(bucket, key string) *S3Step { return f.newStep(s3OpPut, bucket, key) }

// Get starts a GetObject chain.
func (f *S3Facet) Get(bucket, key string) *S3Step { return f.newStep(s3OpGet, bucket, key) }

// Head starts a HeadObject chain (metadata-only).
func (f *S3Facet) Head(bucket, key string) *S3Step { return f.newStep(s3OpHead, bucket, key) }

// List starts a ListObjects chain. Use Prefix() to scope it.
func (f *S3Facet) List(bucket string) *S3Step { return f.newStep(s3OpList, bucket, "") }

// Delete starts a DeleteObject chain.
func (f *S3Facet) Delete(bucket, key string) *S3Step { return f.newStep(s3OpDelete, bucket, key) }

func (f *S3Facet) newStep(op s3Op, bucket, key string) *S3Step {
	vars := f.t.snapshotVars()
	step := &S3Step{
		t:        f.t,
		client:   f.client,
		op:       op,
		bucket:   interpolate(bucket, vars),
		key:      interpolate(key, vars),
		metadata: map[string]string{},
	}
	f.t.setPending(step)
	return step
}

// S3Step is one S3 operation.
type S3Step struct {
	t        *Tester
	client   S3Client
	metadata map[string]string
	bucket   string
	key      string
	prefix   string
	contentT string
	body     []byte
	op       s3Op

	// results
	put    s3.PutResult
	get    s3.GetResult
	head   s3.HeadResult
	list   s3.ListResult
	delete s3.DeleteResult

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	err        error
	failures   []string
}

// Body sets the object payload for a Put, {{var}}-interpolated.
func (s *S3Step) Body(text string) *S3Step {
	if s.guardSent("Body") {
		return s
	}
	s.body = []byte(interpolate(text, s.t.snapshotVars()))
	return s
}

// Bytes sets a raw (non-interpolated) Put payload.
func (s *S3Step) Bytes(b []byte) *S3Step {
	if s.guardSent("Bytes") {
		return s
	}
	s.body = append([]byte(nil), b...)
	return s
}

// ContentType sets the Content-Type for a Put.
func (s *S3Step) ContentType(ct string) *S3Step {
	if s.guardSent("ContentType") {
		return s
	}
	s.contentT = ct
	return s
}

// Meta adds an x-amz-meta-<k> user-metadata header for a Put,
// {{var}}-interpolated.
func (s *S3Step) Meta(k, v string) *S3Step {
	if s.guardSent("Meta") {
		return s
	}
	s.metadata[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// Prefix scopes a List to keys under the given prefix.
func (s *S3Step) Prefix(p string) *S3Step {
	if s.guardSent("Prefix") {
		return s
	}
	s.prefix = interpolate(p, s.t.snapshotVars())
	return s
}

// ── assertions ────────────────────────────────────────────────────────────

// ExpectOK asserts the operation succeeded (no transport/HTTP error).
func (s *S3Step) ExpectOK() *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectOK: %v", s.err))
	}
	return s
}

// ExpectError asserts the operation returned an error (e.g. 404 on Get).
func (s *S3Step) ExpectError() *S3Step {
	s.ensureSent()
	if s.err == nil {
		s.fail("ExpectError: operation succeeded")
	}
	return s
}

// ExpectStatus asserts the HTTP status of the operation.
func (s *S3Step) ExpectStatus(code int) *S3Step {
	if !s.ensureSent() {
		return s
	}
	got := s.statusCode()
	if got != code {
		s.fail(fmt.Sprintf("ExpectStatus: want %d, got %d", code, got))
	}
	return s
}

// ExpectBodyContains asserts a Get body contains sub.
func (s *S3Step) ExpectBodyContains(sub string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpGet {
		s.fail("ExpectBodyContains only valid after Get()")
		return s
	}
	if !strings.Contains(string(s.get.Body), sub) {
		s.fail(fmt.Sprintf("ExpectBodyContains: %q not found", sub))
	}
	return s
}

// ExpectBodyEquals asserts a Get body equals want exactly.
func (s *S3Step) ExpectBodyEquals(want string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpGet {
		s.fail("ExpectBodyEquals only valid after Get()")
		return s
	}
	if string(s.get.Body) != want {
		s.fail(fmt.Sprintf("ExpectBodyEquals: want %q, got %q", want, string(s.get.Body)))
	}
	return s
}

// ExpectMeta asserts a Get/Head user-metadata value.
func (s *S3Step) ExpectMeta(k, want string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	var meta map[string]string
	switch s.op {
	case s3OpGet:
		meta = s.get.Metadata
	case s3OpHead:
		meta = s.head.Metadata
	default:
		s.fail("ExpectMeta only valid after Get() or Head()")
		return s
	}
	if got := meta[k]; got != want {
		s.fail(fmt.Sprintf("ExpectMeta[%s]: want %q, got %q", k, want, got))
	}
	return s
}

// ExpectContentType asserts a Get/Head Content-Type.
func (s *S3Step) ExpectContentType(want string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	var got string
	switch s.op {
	case s3OpGet:
		got = s.get.ContentType
	case s3OpHead:
		got = s.head.ContentType
	default:
		s.fail("ExpectContentType only valid after Get() or Head()")
		return s
	}
	if got != want {
		s.fail(fmt.Sprintf("ExpectContentType: want %q, got %q", want, got))
	}
	return s
}

// ExpectExists asserts a Head found the object.
func (s *S3Step) ExpectExists() *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpHead {
		s.fail("ExpectExists only valid after Head()")
		return s
	}
	if !s.head.Exists {
		s.fail("ExpectExists: object not found")
	}
	return s
}

// ExpectAbsent asserts a Head did NOT find the object.
func (s *S3Step) ExpectAbsent() *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpHead {
		s.fail("ExpectAbsent only valid after Head()")
		return s
	}
	if s.head.Exists {
		s.fail("ExpectAbsent: object exists")
	}
	return s
}

// ExpectObjectCount asserts a List returned exactly n objects.
func (s *S3Step) ExpectObjectCount(n int) *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpList {
		s.fail("ExpectObjectCount only valid after List()")
		return s
	}
	if len(s.list.Objects) != n {
		s.fail(fmt.Sprintf("ExpectObjectCount: want %d, got %d", n, len(s.list.Objects)))
	}
	return s
}

// ExpectKey asserts a List result contains the given key.
func (s *S3Step) ExpectKey(key string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpList {
		s.fail("ExpectKey only valid after List()")
		return s
	}
	for _, o := range s.list.Objects {
		if o.Key == key {
			return s
		}
	}
	s.fail(fmt.Sprintf("ExpectKey: %q not in listing", key))
	return s
}

// ── extraction / escape hatches ─────────────────────────────────────────

// ExtractETag stores the ETag (Put/Get) under name.
func (s *S3Step) ExtractETag(name string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	var etag string
	switch s.op {
	case s3OpPut:
		etag = s.put.ETag
	case s3OpGet:
		etag = s.get.ETag
	case s3OpHead:
		etag = s.head.ETag
	default:
		s.fail("ExtractETag only valid after Put(), Get(), or Head()")
		return s
	}
	s.t.SetVar(name, etag)
	return s
}

// ExtractBody stores a Get body string under name.
func (s *S3Step) ExtractBody(name string) *S3Step {
	if !s.ensureSent() {
		return s
	}
	if s.op != s3OpGet {
		s.fail("ExtractBody only valid after Get()")
		return s
	}
	s.t.SetVar(name, string(s.get.Body))
	return s
}

// GetBody returns the downloaded body (escape hatch).
func (s *S3Step) GetBody() []byte {
	s.ensureSent()
	return append([]byte(nil), s.get.Body...)
}

// Objects returns the List result objects (escape hatch).
func (s *S3Step) Objects() []s3.ObjectInfo {
	s.ensureSent()
	out := make([]s3.ObjectInfo, len(s.list.Objects))
	copy(out, s.list.Objects)
	return out
}

// Done finalises the step.
func (s *S3Step) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

// ── internals ───────────────────────────────────────────────────────────

func (s *S3Step) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *S3Step) guardSent(method string) bool {
	if s.sent {
		s.fail(method + "() called after send")
		return true
	}
	return false
}

func (s *S3Step) statusCode() int {
	switch s.op {
	case s3OpPut:
		return s.put.StatusCode
	case s3OpGet:
		return s.get.StatusCode
	case s3OpHead:
		return s.head.StatusCode
	case s3OpList:
		return s.list.StatusCode
	case s3OpDelete:
		return s.delete.StatusCode
	}
	return 0
}

func (s *S3Step) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}
	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	s.startedAt = time.Now()
	switch s.op {
	case s3OpPut:
		s.put, s.err = s.client.PutObject(ctx, s.bucket, s.key, s.body, s.contentT, s.metadata)
	case s3OpGet:
		s.get, s.err = s.client.GetObject(ctx, s.bucket, s.key)
	case s3OpHead:
		s.head, s.err = s.client.HeadObject(ctx, s.bucket, s.key)
	case s3OpList:
		s.list, s.err = s.client.ListObjects(ctx, s.bucket, s.prefix)
	case s3OpDelete:
		s.delete, s.err = s.client.DeleteObject(ctx, s.bucket, s.key)
	}
	s.endedAt = time.Now()
	// Don't auto-fail on s.err — ExpectOK / ExpectError / ExpectStatus turn
	// it into a verdict. This keeps negative tests (Get a missing key →
	// ExpectError) ergonomic.
	return true
}

func (s *S3Step) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	target := s.bucket
	if s.key != "" {
		target += "/" + s.key
	}
	rec := StepRecord{
		Protocol:     "s3",
		Method:       s.op.String(),
		Name:         "s3 " + s.op.String() + " " + target,
		URL:          target,
		StatusOrCode: s.statusCode(),
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
