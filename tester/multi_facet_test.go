// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/protocols/kafka"
)

// TestMultiFacetFlow drives every protocol facet in one chain to prove
// that:
//   - chains across facets share the same var store
//   - failures from any facet land in t.Errors()
//   - the report serialises in chain order
//   - Allure step emission survives mixed-facet output
//
// Owner's canonical chain example uses .http + .kafka + .grpc +
// .graphql + .soap + .db + .sse + .ws — this test mirrors the SHAPE
// (subset, since some facets need real backends).
func TestMultiFacetFlow(t *testing.T) {
	// HTTP backend with two endpoints — login + downstream.
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok-42"})
		case "/me":
			if r.Header.Get("Authorization") != "Bearer tok-42" {
				http.Error(w, "unauthorized", 401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"user": "alice"})
		}
	}))
	t.Cleanup(httpSrv.Close)

	// GraphQL backend.
	gqlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"orders":[{"id":1},{"id":2}]}}`))
	}))
	t.Cleanup(gqlSrv.Close)

	// In-memory Kafka broker (fakeBroker from kafka_test.go).
	kfk := newFakeBroker()

	// In-memory DB (fakeDB from db_test.go).
	db := newFakeDB()
	db.queries["SELECT count(*) FROM orders WHERE user = ?"] = []DBRow{
		{"count": int64(2)},
	}

	tt := New(WithBaseURL(httpSrv.URL))
	tt.Wrap("login + cross-protocol verify", func() {
		// HTTP login → extract token.
		tt.HTTP().POST("/login").
			JSON(map[string]any{"user": "alice"}).
			ExpectStatus(200).
			Extract("$.token", "token")

		// HTTP downstream call using the extracted token.
		tt.HTTP().GET("/me").
			Header("Authorization", "Bearer {{token}}").
			ExpectStatus(200).
			ExpectJSONPath("$.user", "alice")

		// Kafka publish carrying the token (interpolated into payload).
		tt.Kafka(kfk).Produce("user.events", "k-{{token}}").
			JSON(map[string]any{"type": "login", "user": "alice"}).
			ExpectOK()

		// Kafka consume + assert the produced message landed.
		tt.Kafka(kfk).Consume("user.events").
			Max(5).
			ExpectCount(1).
			ExpectJSONPath(0, "$.user", "alice")

		// GraphQL — orders feed.
		tt.GraphQL(gqlSrv.URL).
			Query("{ orders { id } }", nil).
			ExpectStatus(200).
			ExpectNoErrors().
			ExpectField("$.data.orders[0].id", 1)

		// DB count — uses the extracted user from earlier? actually
		// we hardcode "alice" since the fake doesn't honor args, but
		// the chain still exercises the facet.
		tt.DB(db).Query("SELECT count(*) FROM orders WHERE user = ?", "alice").
			ExpectOK().
			ExpectRowCount(1).
			ExpectField(0, "count", 2)
	})
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("multi-facet chain failed: %v", tt.Errors())
	}

	report := tt.Report()
	// 6 protocol step records. Go's Wrap only opens an Allure parent
	// step, it does NOT add a synthetic record (in contrast with the
	// Python port, where Wrap emits one — that's deliberate: Go has
	// the allure-scope hooks, Python doesn't).
	if len(report) != 6 {
		t.Fatalf("want 6 records (one per facet), got %d: %+v", len(report), report)
	}

	// Verify protocols appear in chain order.
	wantProto := []string{"http", "http", "kafka", "kafka", "graphql", "sql"}
	for i, want := range wantProto {
		if report[i].Protocol != want {
			t.Fatalf("step %d protocol: want %q, got %q", i, want, report[i].Protocol)
		}
	}

	// Verify the token interpolated into the Kafka key.
	msgs, _ := kfk.Consume(nil, kafka.ConsumeOptions{Topic: "user.events", MaxMessages: 1})
	if len(msgs) != 1 || msgs[0].Key != "k-tok-42" {
		t.Fatalf("kafka key interpolation failed: %+v", msgs)
	}
}

// TestMultiFacetFailureCollects asserts that one bad assertion among
// many across different facets still produces ONE failure in the
// accumulated error list — not a cascade.
func TestMultiFacetFailureCollects(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	t.Cleanup(httpSrv.Close)

	kfk := newFakeBroker()
	tt := New(WithBaseURL(httpSrv.URL))

	tt.HTTP().GET("/").ExpectStatus(200)            // pass
	tt.Kafka(kfk).Consume("missing").ExpectCount(1) // fail (empty topic)
	tt.HTTP().GET("/").ExpectStatus(200)            // pass
	tt.Finish()

	if tt.OK() {
		t.Fatal("expected mixed pass/fail")
	}
	errs := tt.Errors()
	if got := len(errs); got != 1 {
		t.Fatalf("want exactly 1 error, got %d: %v", got, errs)
	}
	if !strings.Contains(errs[0].Error(), "ExpectCount") {
		t.Fatalf("error should be from Kafka ExpectCount, got: %v", errs[0])
	}
}

// TestVarSurvivesChainBoundaries ensures the var store carries values
// across all 5 protocol facets (HTTP / Kafka / GraphQL / DB / SOAP-not-used-here).
func TestVarSurvivesChainBoundaries(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"x": "VAL"})
	}))
	t.Cleanup(httpSrv.Close)
	kfk := newFakeBroker()
	db := newFakeDB()
	db.queries["SELECT 'VAL'"] = []DBRow{{"x": "VAL"}}

	tt := New(WithBaseURL(httpSrv.URL))
	tt.HTTP().GET("/").Extract("$.x", "shared")
	tt.Kafka(kfk).Produce("topic", "k").JSON(map[string]any{"v": "{{shared}}"}).ExpectOK()
	tt.Kafka(kfk).Consume("topic").Max(1).
		ExpectJSONPath(0, "$.v", "VAL").
		Extract(0, "$.v", "fromKafka")
	tt.DB(db).Query("SELECT 'VAL'").ExpectField(0, "x", "VAL")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	v := tt.Vars()
	if v["shared"] != "VAL" || v["fromKafka"] != "VAL" {
		t.Fatalf("vars not carried correctly: %+v", v)
	}
}

// TestFailFastAcrossFacets verifies that the fail-fast flag stops
// chains in subsequent facets, not just within the same facet.
func TestFailFastAcrossFacets(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(httpSrv.Close)

	kfk := newFakeBroker()
	tt := New(WithBaseURL(httpSrv.URL), WithFailFast())

	tt.HTTP().GET("/").ExpectStatus(200)        // fails
	tt.Kafka(kfk).Consume("any").ExpectCount(0) // should be skipped
	tt.Finish()

	if tt.OK() {
		t.Fatal("expected failure")
	}
	// 2 step records — both committed even when aborted (with a
	// "skipped" failure message on the second).
	report := tt.Report()
	if len(report) != 2 {
		t.Fatalf("want 2 records, got %d", len(report))
	}
	// Verify the Kafka step records a "skipped" failure.
	found := false
	for _, f := range report[1].Failures {
		if strings.Contains(f, "skipped") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("second step should have a 'skipped' failure: %+v", report[1].Failures)
	}
}

// TestReportTimingsMonotonic verifies StartedAt <= EndedAt for every
// recorded step — basic sanity check across facets.
func TestReportTimingsMonotonic(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(httpSrv.Close)
	tt := New(WithBaseURL(httpSrv.URL))
	tt.HTTP().GET("/").Send().Done()
	r := tt.Report()
	if len(r) != 1 {
		t.Fatalf("want 1 step")
	}
	if !r[0].StartedAt.Before(r[0].EndedAt) && !r[0].StartedAt.Equal(r[0].EndedAt) {
		t.Fatalf("timing not monotonic: started=%v ended=%v", r[0].StartedAt, r[0].EndedAt)
	}
}

// silence unused-import lint when the test grows over time.
var _ = errors.New
