// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
)

// SOAPFacet is the SOAP entry point reached via Tester.SOAP(endpoint).
type SOAPFacet struct {
	t        *Tester
	endpoint string
}

// SOAP returns the SOAP facet bound to a SOAP endpoint URL. Relative
// paths resolve against Tester.BaseURL.
func (t *Tester) SOAP(endpoint string) *SOAPFacet {
	t.flushPending()
	return &SOAPFacet{t: t, endpoint: endpoint}
}

// Call starts a SOAP call. `action` is the SOAPAction header value
// (e.g. "http://service/op"); `body` is the XML payload — usually the
// full <soap:Envelope> document, but a bare <Method/> fragment is
// accepted and wrapped in a minimal envelope.
//
// Both action and body are {{var}}-interpolated.
func (s *SOAPFacet) Call(action, body string) *SOAPStep {
	s.t.flushPending()
	vars := s.t.snapshotVars()
	step := &SOAPStep{
		t:        s.t,
		endpoint: interpolate(s.endpoint, vars),
		action:   interpolate(action, vars),
		body:     wrapSOAPEnvelope(interpolate(body, vars)),
		headers:  map[string]string{},
	}
	s.t.setPending(step)
	return step
}

// wrapSOAPEnvelope passes through inputs that already start with the
// SOAP envelope; otherwise wraps the body fragment in a minimal SOAP
// 1.1 envelope so callers don't have to repeat the boilerplate.
func wrapSOAPEnvelope(body string) string {
	trimmed := strings.TrimSpace(body)
	if strings.HasPrefix(trimmed, "<?xml") || strings.Contains(trimmed, ":Envelope") {
		return body
	}
	return `<?xml version="1.0"?>` +
		`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">` +
		`<soap:Body>` + body + `</soap:Body>` +
		`</soap:Envelope>`
}

// SOAPStep is one SOAP call.
type SOAPStep struct {
	t        *Tester
	endpoint string
	action   string
	body     string
	headers  map[string]string

	resp       *http.Response
	respBody   []byte
	doc        *xmlquery.Node
	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	failures   []string
}

// Header sets a request header (Authorization etc), {{var}}-interpolated.
func (s *SOAPStep) Header(k, v string) *SOAPStep {
	if s.sent {
		s.fail("Header() called after send")
		return s
	}
	s.headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// ExpectStatus asserts the HTTP status (SOAP 1.1 keeps 200 even for
// declared faults; 500 carries protocol-level faults).
func (s *SOAPStep) ExpectStatus(code int) *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	if s.resp != nil && s.resp.StatusCode != code {
		s.fail(fmt.Sprintf("ExpectStatus: want %d, got %d", code, s.resp.StatusCode))
	}
	return s
}

// ExpectXPath asserts that the resolved XPath value (string form)
// equals want. Want is converted with fmt.Sprintf("%v") so the caller
// can pass strings or numbers interchangeably.
func (s *SOAPStep) ExpectXPath(xpathExpr string, want any) *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalXPath(xpathExpr)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectXPath %s: %v", xpathExpr, err))
		return s
	}
	wantStr := fmt.Sprintf("%v", want)
	if got != wantStr {
		s.fail(fmt.Sprintf("ExpectXPath %s: want %q, got %q", xpathExpr, wantStr, got))
	}
	return s
}

// ExpectXPathContains asserts the resolved XPath value (string form)
// contains the substring sub.
func (s *SOAPStep) ExpectXPathContains(xpathExpr, sub string) *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalXPath(xpathExpr)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectXPathContains %s: %v", xpathExpr, err))
		return s
	}
	if !strings.Contains(got, sub) {
		s.fail(fmt.Sprintf("ExpectXPathContains %s: %q not found in %q", xpathExpr, sub, got))
	}
	return s
}

// ExpectNoFault asserts that the response contains no <Fault> element.
func (s *SOAPStep) ExpectNoFault() *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	if s.doc == nil {
		return s
	}
	if n := xmlquery.FindOne(s.doc, "//*[local-name()='Fault']"); n != nil {
		code, _ := s.evalXPath("//*[local-name()='Fault']/*[local-name()='faultcode']/text()")
		msg, _ := s.evalXPath("//*[local-name()='Fault']/*[local-name()='faultstring']/text()")
		s.fail(fmt.Sprintf("ExpectNoFault: %s — %s", code, msg))
	}
	return s
}

// ExpectFault asserts the response contains a <Fault>. Optional
// faultCode (passed as "") skips the code check.
func (s *SOAPStep) ExpectFault(faultCode string) *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	if s.doc == nil {
		s.fail("ExpectFault: no response")
		return s
	}
	if n := xmlquery.FindOne(s.doc, "//*[local-name()='Fault']"); n == nil {
		s.fail("ExpectFault: no <Fault> in response")
		return s
	}
	if faultCode == "" {
		return s
	}
	code, _ := s.evalXPath("//*[local-name()='Fault']/*[local-name()='faultcode']/text()")
	if !strings.Contains(code, faultCode) {
		s.fail(fmt.Sprintf("ExpectFault: want code %q, got %q", faultCode, code))
	}
	return s
}

// Extract resolves an XPath value (string form) and stores it under name.
func (s *SOAPStep) Extract(xpathExpr, name string) *SOAPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalXPath(xpathExpr)
	if err != nil {
		s.fail(fmt.Sprintf("Extract %s: %v", xpathExpr, err))
		return s
	}
	s.t.SetVar(name, got)
	return s
}

// ResponseBody returns the raw response bytes — escape hatch.
func (s *SOAPStep) ResponseBody() []byte {
	s.ensureSent()
	out := make([]byte, len(s.respBody))
	copy(out, s.respBody)
	return out
}

// Done finalises the step.
func (s *SOAPStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *SOAPStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *SOAPStep) evalXPath(expr string) (string, error) {
	if s.doc == nil {
		return "", fmt.Errorf("no document (status=%d)", s.statusCode())
	}
	// FindOne returns the first matching node; for paths ending in /text()
	// xmlquery returns the text node and Node.InnerText() yields its content.
	node := xmlquery.FindOne(s.doc, expr)
	if node == nil {
		return "", fmt.Errorf("no match for %q", expr)
	}
	return strings.TrimSpace(node.InnerText()), nil
}

func (s *SOAPStep) statusCode() int {
	if s.resp != nil {
		return s.resp.StatusCode
	}
	return 0
}

func (s *SOAPStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}

	url := s.endpoint
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		if s.t.baseURL != "" {
			if !strings.HasPrefix(url, "/") {
				url = "/" + url
			}
			url = s.t.baseURL + url
		}
	}

	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(s.body)))
	if err != nil {
		s.fail(fmt.Sprintf("build request: %v", err))
		s.abortChain = true
		return false
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", s.action)
	for k, vs := range s.t.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	s.startedAt = time.Now()
	resp, err := s.t.http.Do(req)
	s.endedAt = time.Now()
	if err != nil {
		s.fail(fmt.Sprintf("soap: %v", err))
		s.abortChain = true
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s.resp = resp
	s.respBody = body

	doc, perr := xmlquery.Parse(bytes.NewReader(body))
	if perr != nil {
		s.fail(fmt.Sprintf("soap: parse XML: %v", perr))
		return true
	}
	s.doc = doc
	return true
}

func (s *SOAPStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:  "soap",
		Method:    "POST",
		Name:      "soap " + s.action,
		URL:       s.endpoint,
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	if s.resp != nil {
		rec.StatusOrCode = s.resp.StatusCode
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
