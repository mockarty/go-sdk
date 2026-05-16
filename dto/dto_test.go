// Package dto's tests live next to the generated code (CLAUDE.md "Testing
// Rules": tests co-located with the package). They verify JSON marshalling
// round-trips and that the generator's encoding decisions (omitempty,
// typed enums, $ref linkage) match the wire format the admin server emits.
//
// IMPORTANT: do NOT edit any *.go file in this directory by hand. Add or
// fix assertions here instead. Generation is driven by `make sync-models`.
package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Round-trip: marshal → unmarshal → compare
// ---------------------------------------------------------------------------

func TestLoginRequest_JSONRoundTrip(t *testing.T) {
	in := LoginRequest{Login: "alice", Password: "s3cret"}
	out := unmarshalRoundTrip(t, in, &LoginRequest{}).(*LoginRequest)
	if *out != in {
		t.Errorf("round-trip mismatch: got %+v want %+v", *out, in)
	}
	// Required fields MUST NOT omit on zero — verify the marshalled bytes.
	empty, _ := json.Marshal(LoginRequest{})
	if !strings.Contains(string(empty), `"login":""`) || !strings.Contains(string(empty), `"password":""`) {
		t.Errorf("required fields must be serialized even when empty; got %s", empty)
	}
}

func TestUserInfo_JSONRoundTrip(t *testing.T) {
	in := UserInfo{
		ID:         "u-1",
		Email:      "alice@example.com",
		Login:      "alice",
		SystemRole: "admin",
		Namespaces: []string{"default", "qa"},
	}
	out := unmarshalRoundTrip(t, in, &UserInfo{}).(*UserInfo)
	if out.ID != in.ID || out.Email != in.Email || len(out.Namespaces) != 2 {
		t.Errorf("round-trip mismatch: got %+v want %+v", *out, in)
	}
	// Optional zero-valued fields must be omitted.
	empty, _ := json.Marshal(UserInfo{})
	if string(empty) != "{}" {
		t.Errorf("empty UserInfo should marshal to {} (all optional+omitempty); got %s", empty)
	}
}

func TestAPIKey_JSONRoundTrip(t *testing.T) {
	in := APIKey{
		ID:                 "k-1",
		Name:               "ci-token",
		Description:        "GitHub Actions",
		Namespace:          "default",
		RateLimitPerMinute: 1000,
		RateLimitPerHour:   50000,
		RateLimitPerDay:    1_000_000,
		Enabled:            true,
		AllowedActions:     []string{"GET", "POST"},
		Metadata:           map[string]any{"created_by": "alice"},
	}
	out := unmarshalRoundTrip(t, in, &APIKey{}).(*APIKey)
	if out.ID != in.ID || out.RateLimitPerDay != 1_000_000 || !out.Enabled {
		t.Errorf("round-trip mismatch: %+v", *out)
	}
	if len(out.AllowedActions) != 2 || out.AllowedActions[0] != "GET" {
		t.Errorf("slice round-trip: %v", out.AllowedActions)
	}
	if got, _ := out.Metadata["created_by"].(string); got != "alice" {
		t.Errorf("metadata map round-trip: %v", out.Metadata)
	}
}

func TestAPITesterProtocol_EnumConstantsRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		val  APITesterProtocol
		want string
	}{
		{"http", APITesterProtocolHTTP, `"http"`},
		{"soap", APITesterProtocolSOAP, `"soap"`},
		{"grpc", APITesterProtocolGRPC, `"grpc"`},
		{"mcp", APITesterProtocolMCP, `"mcp"`},
		{"graphql", APITesterProtocolGraphQL, `"graphql"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.val)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != c.want {
				t.Errorf("marshal: got %s want %s", b, c.want)
			}
			var back APITesterProtocol
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if back != c.val {
				t.Errorf("unmarshal: got %v want %v", back, c.val)
			}
		})
	}
}

func TestAssertAction_EnumLiterals(t *testing.T) {
	if AssertActionEquals != "equals" {
		t.Errorf("AssertActionEquals = %q, want %q", AssertActionEquals, "equals")
	}
	if AssertActionNotContains != "not_contains" {
		t.Errorf("AssertActionNotContains = %q, want %q", AssertActionNotContains, "not_contains")
	}
	if AssertActionMatches != "matches" {
		t.Errorf("AssertActionMatches = %q, want %q", AssertActionMatches, "matches")
	}
}

func TestDuration_IntegerEnum(t *testing.T) {
	// time.Duration → Duration int64 alias with named consts.
	if DurationNanosecond != 1 {
		t.Errorf("Nanosecond = %d, want 1", DurationNanosecond)
	}
	if DurationSecond != 1_000_000_000 {
		t.Errorf("Second = %d, want 1e9", DurationSecond)
	}
	// JSON round-trip as a plain int64.
	b, _ := json.Marshal(DurationMillisecond)
	if string(b) != "1000000" {
		t.Errorf("marshal millisecond: %s", b)
	}
}

func TestMStore_FreeFormMapAlias(t *testing.T) {
	// `mockarty_internal_model.MStore` has no properties — it's an
	// additionalProperties:true alias. The generator should emit a typedef
	// to map[string]any, not a struct.
	var m MStore = map[string]any{"k": "v", "n": float64(42)}
	b, _ := json.Marshal(m)
	var back MStore
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("MStore round-trip: %v", err)
	}
	if back["k"] != "v" {
		t.Errorf("MStore lost key 'k': %v", back)
	}
}

func TestMock_NestedRefs(t *testing.T) {
	// Mock embeds many $refs (HTTP, GRPC, ContentResponse, OneOf, Proxy…).
	// Just verify the round-trip works with a partial payload — the JSON
	// shape decides which sub-context is populated.
	payload := `{
		"id": "m-1",
		"namespace": "default",
		"serverName": "api",
		"priority": 100,
		"http": {"path": "/v1/x", "method": "GET"},
		"response": {"code": 200, "body": "{}"},
		"tags": ["a","b"]
	}`
	var m Mock
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.ID != "m-1" || m.ServerName != "api" || m.Priority != 100 {
		t.Errorf("scalar fields: %+v", m)
	}
	if len(m.Tags) != 2 {
		t.Errorf("tags: %v", m.Tags)
	}
}

func TestTestReport_NestedTypes(t *testing.T) {
	payload := `{
		"id": "r-1",
		"protocol": "http",
		"status": "passed",
		"totalTests": 5,
		"passedTests": 4,
		"failedTests": 1,
		"logs": ["ok","fail"]
	}`
	var r TestReport
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Protocol != APITesterProtocolHTTP {
		t.Errorf("protocol typed enum: got %q want http", r.Protocol)
	}
	if r.TotalTests != 5 || r.PassedTests != 4 || r.FailedTests != 1 {
		t.Errorf("test counts: %+v", r)
	}
}

// ---------------------------------------------------------------------------
// Wire-format invariants — these encode contract decisions in tests.
// ---------------------------------------------------------------------------

func TestOmitEmpty_OnOptionalFields(t *testing.T) {
	// UserInfo has zero required fields → empty struct marshals to "{}".
	b, _ := json.Marshal(UserInfo{})
	if string(b) != "{}" {
		t.Errorf("UserInfo{} should marshal to {} (every field is optional with omitempty); got %s", b)
	}
}

func TestRequiredFields_NoOmitEmpty(t *testing.T) {
	// LoginRequest has required login+password → empty struct marshals
	// to a non-empty payload (the zero values are present).
	b, _ := json.Marshal(LoginRequest{})
	got := string(b)
	if !strings.Contains(got, `"login"`) || !strings.Contains(got, `"password"`) {
		t.Errorf("required fields must appear in serialization; got %s", got)
	}
}

// ---------------------------------------------------------------------------
// Field-alignment sanity check — encodes CLAUDE.md mandate. Failing this
// usually means the generator's ordering policy regressed.
// ---------------------------------------------------------------------------

func TestFieldAlignment_HotDTOs(t *testing.T) {
	type spec struct {
		name string
		size uintptr
		// max — fail if the struct grew above this. We don't pin the exact
		// value because adding a property is allowed (it should land
		// idiomatically); we just guard against accidental padding waste.
		max uintptr
	}
	// Measured on darwin/arm64 with the Phase-1 generator. Any future change
	// that bumps these is allowed only if a property was genuinely added
	// (update the constant in the same PR).
	for _, s := range []spec{
		{"APIKey", unsafe.Sizeof(APIKey{}), 280},
		{"LoginRequest", unsafe.Sizeof(LoginRequest{}), 40},
		{"UserInfo", unsafe.Sizeof(UserInfo{}), 120},
		{"Mock", unsafe.Sizeof(Mock{}), 2200},
	} {
		if s.size > s.max {
			t.Errorf("%s padded to %d (cap %d) — generator may have regressed field-alignment ordering",
				s.name, s.size, s.max)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// unmarshalRoundTrip marshals `in`, decodes into `into`, returns the decoded
// value. Fails the test on any I/O error.
func unmarshalRoundTrip(t *testing.T, in any, into any) any {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("unmarshal: %v\npayload: %s", err, b)
	}
	return into
}
