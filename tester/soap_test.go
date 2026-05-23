// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// soapEcho returns a fixed XML body with the given status. captures the
// requestBody + SOAPAction for assertion.
func soapEcho(t *testing.T, status int, respBody string, captureAction, captureBody *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captureAction != nil {
			*captureAction = r.Header.Get("SOAPAction")
		}
		if captureBody != nil {
			b, _ := io.ReadAll(r.Body)
			*captureBody = string(b)
		}
		w.Header().Set("Content-Type", "text/xml")
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const userResponse = `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUserResponse xmlns="urn:test">
      <user>
        <id>42</id>
        <name>Alice</name>
        <roles>
          <role>admin</role>
          <role>ops</role>
        </roles>
      </user>
    </GetUserResponse>
  </soap:Body>
</soap:Envelope>`

const faultResponse = `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <soap:Fault>
      <faultcode>soap:Client</faultcode>
      <faultstring>Invalid user id</faultstring>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`

func TestSOAPHappyPath(t *testing.T) {
	var action, body string
	srv := soapEcho(t, 200, userResponse, &action, &body)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").
		Call("urn:test#GetUser", `<GetUser xmlns="urn:test"><id>42</id></GetUser>`).
		ExpectStatus(200).
		ExpectNoFault().
		ExpectXPath("//*[local-name()='name']/text()", "Alice").
		ExpectXPath("//*[local-name()='id']/text()", "42").
		ExpectXPathContains("//*[local-name()='name']/text()", "Ali").
		Extract("//*[local-name()='name']/text()", "user")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	if tt.Vars()["user"] != "Alice" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
	if action != "urn:test#GetUser" {
		t.Fatalf("SOAPAction header: %q", action)
	}
	if !strings.Contains(body, "<soap:Envelope") {
		t.Fatalf("Envelope wrapping not applied: %q", body)
	}
}

func TestSOAPFaultDetected(t *testing.T) {
	srv := soapEcho(t, 500, faultResponse, nil, nil)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").
		Call("op", `<X/>`).
		ExpectFault("Client").
		ExpectXPath("//*[local-name()='faultstring']/text()", "Invalid user id")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestSOAPExpectNoFaultFailsOnFault(t *testing.T) {
	srv := soapEcho(t, 500, faultResponse, nil, nil)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").Call("op", `<X/>`).ExpectNoFault()
	tt.Finish()
	if tt.OK() {
		t.Fatal("ExpectNoFault should fail when a fault is present")
	}
}

func TestSOAPExpectFaultMissing(t *testing.T) {
	srv := soapEcho(t, 200, userResponse, nil, nil)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").Call("op", `<X/>`).ExpectFault("")
	tt.Finish()
	if tt.OK() {
		t.Fatal("ExpectFault should fail when no fault present")
	}
}

func TestSOAPFullEnvelopePassthrough(t *testing.T) {
	var body string
	srv := soapEcho(t, 200, userResponse, nil, &body)
	full := `<?xml version="1.0"?><soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Op/></soap:Body></soap:Envelope>`
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").Call("op", full).ExpectStatus(200)
	tt.Finish()
	if body != full {
		t.Fatalf("full envelope was wrapped again: %q", body)
	}
}

func TestSOAPHeaderInterpolation(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(userResponse))
	}))
	t.Cleanup(srv.Close)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("tok", "abc")
	tt.SOAP("/svc").Call("op", `<X/>`).Header("Authorization", "Bearer {{tok}}").ExpectStatus(200)
	tt.Finish()
	if auth != "Bearer abc" {
		t.Fatalf("interpolation failed: %q", auth)
	}
}

func TestSOAPBodyInterpolation(t *testing.T) {
	var body string
	srv := soapEcho(t, 200, userResponse, nil, &body)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("id", "42")
	tt.SOAP("/svc").Call("op", `<GetUser><id>{{id}}</id></GetUser>`).ExpectStatus(200)
	tt.Finish()
	if !strings.Contains(body, "<id>42</id>") {
		t.Fatalf("body interpolation: %q", body)
	}
}

func TestSOAPXPathMissingFails(t *testing.T) {
	srv := soapEcho(t, 200, userResponse, nil, nil)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").Call("op", `<X/>`).ExpectXPath("//*[local-name()='missing']/text()", "x")
	tt.Finish()
	if tt.OK() {
		t.Fatal("missing XPath should fail")
	}
}

func TestSOAPInvalidXMLResponse(t *testing.T) {
	srv := soapEcho(t, 200, "<broken><", nil, nil)
	tt := New(WithBaseURL(srv.URL))
	tt.SOAP("/svc").Call("op", `<X/>`).ExpectXPath("/x", "y")
	tt.Finish()
	if tt.OK() {
		t.Fatal("invalid XML should produce failures")
	}
}

func TestSOAPResponseBodyEscapeHatch(t *testing.T) {
	srv := soapEcho(t, 200, userResponse, nil, nil)
	tt := New(WithBaseURL(srv.URL))
	step := tt.SOAP("/svc").Call("op", `<X/>`)
	body := step.ResponseBody()
	step.Done()
	if !strings.Contains(string(body), "Alice") {
		t.Fatalf("body: %s", body)
	}
}
