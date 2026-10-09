package mockarty

import (
	"encoding/json"
	"testing"
)

func TestNightMockReadLifecycleMetadata(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int64
	}{
		{`{"id":"deleted","_meta":{"closedAt":1700000000}}`, 1700000000},
		{`{"id":"live","_meta":{}}`, 0},
		{`{"closedAt":12,"_meta":{"closedAt":20}}`, 12},
		{`{"closedAt":0,"_meta":{"closedAt":20}}`, 0},
		{`{"closedAt":null,"_meta":{"closedAt":20}}`, 20},
	} {
		var m Mock
		if err := json.Unmarshal([]byte(tc.body), &m); err != nil {
			t.Fatal(err)
		}
		if m.ClosedAt != tc.want {
			t.Errorf("%s: closedAt=%d want%d", tc.body, m.ClosedAt, tc.want)
		}
	}
}

func TestNightMockLifecycleDecodePreservesOmittedExistingFields(t *testing.T) {
	for _, tc := range []struct {
		body string
		want int64
	}{
		{`{"id":"updated"}`, 5},
		{`{"id":"updated","_meta":{"closedAt":7}}`, 7},
		{`{"id":"updated","closedAt":0,"_meta":{"closedAt":7}}`, 0},
	} {
		m := Mock{ID: "original", Namespace: "existing", ClosedAt: 5}
		if err := json.Unmarshal([]byte(tc.body), &m); err != nil {
			t.Fatal(err)
		}
		if m.ID != "updated" || m.Namespace != "existing" || m.ClosedAt != tc.want {
			t.Fatalf("existing JSON decode compatibility lost for %s: %+v", tc.body, m)
		}
	}
}
