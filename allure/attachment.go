// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

// AttachOption tunes a single attachment.
type AttachOption func(*attachOpts)

type attachOpts struct {
	mime string
}

// WithMime overrides the inferred MIME type for the attachment.
func WithMime(mime string) AttachOption { return func(o *attachOpts) { o.mime = mime } }

// Attachment attaches an in-memory payload to the current step (or the test
// if no step is open). The bytes are copied internally; callers may free /
// mutate the original buffer immediately after the call returns.
//
// MIME defaults to net/http.DetectContentType when empty so common payloads
// (PNG, JSON, plain text) render natively in the Allure UI without extra
// configuration.
func Attachment(ctx context.Context, name string, content []byte, mime string) {
	s := fromContext(ctx)
	if s == nil {
		return
	}
	if mime == "" {
		mime = http.DetectContentType(content)
	}
	s.addAttachment(name, mime, content)
}

// AttachFile reads a file from disk and attaches its contents. MIME is
// inferred from content; pass [WithMime] to override.
//
// Errors are swallowed and recorded as a `.error` attachment so the test
// itself does not fail when an attachment is unavailable (fail-soft).
func AttachFile(ctx context.Context, name, path string, opts ...AttachOption) {
	s := fromContext(ctx)
	if s == nil {
		return
	}
	o := attachOpts{}
	for _, opt := range opts {
		opt(&o)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		s.addAttachment(name+".error", "text/plain",
			[]byte(fmt.Sprintf("AttachFile failed: %s: %v", path, err)))
		return
	}
	if o.mime == "" {
		o.mime = http.DetectContentType(data)
	}
	s.addAttachment(name, o.mime, data)
}
