// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Package s3 is the Mockarty Go SDK's minimal S3 test client. It speaks
// path-style S3 over plain HTTP so CI/CD test scripts can exercise an
// S3-compatible endpoint (a Mockarty S3 mock, MinIO, or AWS itself) and
// assert on PutObject / GetObject / ListObjects / DeleteObject results.
//
// # Why a minimal client
//
// The full AWS SDK is a heavy dependency to pull into every test binary.
// The operations a test author needs — put/get/head/list/delete an
// object, read its body + metadata — map onto plain HTTP verbs against
// a path-style URL (http://host/<bucket>/<key>). This client implements
// exactly that surface. For SigV4-signed buckets pass a signer via
// WithRequestSigner; against unsigned Mockarty mocks no signer is needed.
//
// # Air-gapped friendly
//
// Pure net/http — no CGO, no external module. Same binary runs in
// distroless.
//
// # Out of scope
//
// Bucket admin (ACL, versioning, lifecycle, multipart), presigned URLs,
// and the SigV4 algorithm itself are NOT implemented — the owner-rule
// for mockarty-go is "expose only the surface useful from CI/CD scripts
// and tests".
package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a goroutine-safe path-style S3 test client bound to a fixed
// endpoint. Create one per test and reuse across operations.
type Client struct {
	http     *http.Client
	signer   RequestSigner
	endpoint string
}

// RequestSigner, when supplied via WithRequestSigner, is called with the
// outbound request just before it is sent so callers can add SigV4
// (or any other) authentication headers. The SDK does not ship a SigV4
// implementation — against Mockarty S3 mocks none is needed.
type RequestSigner func(req *http.Request) error

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the default *http.Client (30s timeout).
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		if c != nil {
			cl.http = c
		}
	}
}

// WithRequestSigner installs a per-request signing hook (e.g. SigV4).
func WithRequestSigner(s RequestSigner) Option {
	return func(cl *Client) { cl.signer = s }
}

// NewClient constructs a client against the supplied endpoint, e.g.
// "http://localhost:18770/s3". The endpoint is the prefix that buckets
// hang off (path-style): an object lives at <endpoint>/<bucket>/<key>.
func NewClient(endpoint string, opts ...Option) *Client {
	cl := &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		http:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(cl)
	}
	return cl
}

// PutResult is what PutObject returns.
type PutResult struct {
	ETag       string
	StatusCode int
}

// GetResult is what GetObject returns.
type GetResult struct {
	Metadata     map[string]string // x-amz-meta-* (key without the prefix)
	ContentType  string
	ETag         string
	Body         []byte
	StatusCode   int
	LastModified time.Time
}

// HeadResult is what HeadObject returns (no body).
type HeadResult struct {
	Metadata      map[string]string
	ContentType   string
	ETag          string
	StatusCode    int
	ContentLength int64
	Exists        bool
}

// ObjectInfo is one entry in a ListObjects result.
type ObjectInfo struct {
	LastModified time.Time
	Key          string
	ETag         string
	Size         int64
}

// ListResult is what ListObjects returns.
type ListResult struct {
	Objects     []ObjectInfo
	StatusCode  int
	IsTruncated bool
}

// DeleteResult is what DeleteObject returns.
type DeleteResult struct {
	StatusCode int
}

// PutObject uploads body to <bucket>/<key>. Optional contentType and
// user metadata (rendered as x-amz-meta-<k> headers) are applied.
func (c *Client) PutObject(ctx context.Context, bucket, key string, body []byte, contentType string, metadata map[string]string) (PutResult, error) {
	req, err := c.newRequest(ctx, http.MethodPut, bucket, key, "", bytes.NewReader(body))
	if err != nil {
		return PutResult{}, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range metadata {
		req.Header.Set("x-amz-meta-"+k, v)
	}
	resp, err := c.do(req)
	if err != nil {
		return PutResult{}, err
	}
	defer drain(resp)
	res := PutResult{StatusCode: resp.StatusCode, ETag: strings.Trim(resp.Header.Get("ETag"), `"`)}
	if resp.StatusCode >= 300 {
		return res, c.httpError("PutObject", resp)
	}
	return res, nil
}

// GetObject downloads <bucket>/<key>.
func (c *Client) GetObject(ctx context.Context, bucket, key string) (GetResult, error) {
	req, err := c.newRequest(ctx, http.MethodGet, bucket, key, "", nil)
	if err != nil {
		return GetResult{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return GetResult{}, err
	}
	defer drain(resp)
	res := GetResult{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		ETag:        strings.Trim(resp.Header.Get("ETag"), `"`),
		Metadata:    extractMeta(resp.Header),
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, perr := http.ParseTime(lm); perr == nil {
			res.LastModified = t
		}
	}
	if resp.StatusCode >= 300 {
		return res, c.httpError("GetObject", resp)
	}
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return res, fmt.Errorf("mockarty s3: read body: %w", rerr)
	}
	res.Body = b
	return res, nil
}

// HeadObject fetches object metadata without the body. A 404 is NOT an
// error — Exists is set false and StatusCode reflects the response so
// negative tests can assert absence.
func (c *Client) HeadObject(ctx context.Context, bucket, key string) (HeadResult, error) {
	req, err := c.newRequest(ctx, http.MethodHead, bucket, key, "", nil)
	if err != nil {
		return HeadResult{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return HeadResult{}, err
	}
	defer drain(resp)
	res := HeadResult{
		StatusCode:    resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		ETag:          strings.Trim(resp.Header.Get("ETag"), `"`),
		ContentLength: resp.ContentLength,
		Metadata:      extractMeta(resp.Header),
		Exists:        resp.StatusCode >= 200 && resp.StatusCode < 300,
	}
	return res, nil
}

// ListObjects lists objects in bucket, optionally filtered by prefix.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) (ListResult, error) {
	query := "list-type=2"
	if prefix != "" {
		query += "&prefix=" + prefix
	}
	req, err := c.newRequest(ctx, http.MethodGet, bucket, "", query, nil)
	if err != nil {
		return ListResult{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return ListResult{}, err
	}
	defer drain(resp)
	res := ListResult{StatusCode: resp.StatusCode}
	if resp.StatusCode >= 300 {
		return res, c.httpError("ListObjects", resp)
	}
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return res, fmt.Errorf("mockarty s3: read list body: %w", rerr)
	}
	var parsed listBucketResult
	if xerr := xml.Unmarshal(b, &parsed); xerr != nil {
		return res, fmt.Errorf("mockarty s3: parse ListBucketResult: %w", xerr)
	}
	res.IsTruncated = parsed.IsTruncated
	for _, o := range parsed.Contents {
		info := ObjectInfo{Key: o.Key, Size: o.Size, ETag: strings.Trim(o.ETag, `"`)}
		if t, perr := time.Parse(time.RFC3339, o.LastModified); perr == nil {
			info.LastModified = t
		}
		res.Objects = append(res.Objects, info)
	}
	return res, nil
}

// DeleteObject removes <bucket>/<key>. A successful delete returns 204.
func (c *Client) DeleteObject(ctx context.Context, bucket, key string) (DeleteResult, error) {
	req, err := c.newRequest(ctx, http.MethodDelete, bucket, key, "", nil)
	if err != nil {
		return DeleteResult{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return DeleteResult{}, err
	}
	defer drain(resp)
	res := DeleteResult{StatusCode: resp.StatusCode}
	if resp.StatusCode >= 300 {
		return res, c.httpError("DeleteObject", resp)
	}
	return res, nil
}

// ── internals ───────────────────────────────────────────────────────────

func (c *Client) newRequest(ctx context.Context, method, bucket, key, query string, body io.Reader) (*http.Request, error) {
	if bucket == "" {
		return nil, fmt.Errorf("mockarty s3: empty bucket")
	}
	u := c.endpoint + "/" + bucket
	if key != "" {
		u += "/" + strings.TrimPrefix(key, "/")
	} else {
		u += "/"
	}
	if query != "" {
		u += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("mockarty s3: build request: %w", err)
	}
	return req, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.signer != nil {
		if err := c.signer(req); err != nil {
			return nil, fmt.Errorf("mockarty s3: sign request: %w", err)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mockarty s3: %s %s: %w", req.Method, req.URL.Path, err)
	}
	return resp, nil
}

// httpError reads the (typically XML) error body and returns a Go error.
func (c *Client) httpError(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var s3err s3ErrorBody
	if xml.Unmarshal(b, &s3err) == nil && s3err.Code != "" {
		return fmt.Errorf("mockarty s3: %s: %s (%d): %s", op, s3err.Code, resp.StatusCode, s3err.Message)
	}
	return fmt.Errorf("mockarty s3: %s: status %d", op, resp.StatusCode)
}

func extractMeta(h http.Header) map[string]string {
	var out map[string]string
	for k, v := range h {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-meta-") && len(v) > 0 {
			if out == nil {
				out = map[string]string{}
			}
			out[strings.TrimPrefix(lk, "x-amz-meta-")] = v[0]
		}
	}
	return out
}

func drain(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		_ = resp.Body.Close()
	}
}

// ── S3 XML shapes ─────────────────────────────────────────────────────────

type listBucketResult struct {
	XMLName     xml.Name        `xml:"ListBucketResult"`
	Contents    []listObjectXML `xml:"Contents"`
	IsTruncated bool            `xml:"IsTruncated"`
}

type listObjectXML struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

type s3ErrorBody struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}
