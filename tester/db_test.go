// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"errors"
	"testing"
)

// fakeDB is an in-memory SQLConn. Tests pre-load responses per query
// substring; matching queries return the configured rows / err / result.
type fakeDB struct {
	queries map[string][]DBRow
	errs    map[string]error
	execs   map[string]DBExecResult
	calls   []fakeCall
}

type fakeCall struct {
	args  []any
	query string
	kind  string
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		queries: map[string][]DBRow{},
		errs:    map[string]error{},
		execs:   map[string]DBExecResult{},
	}
}

func (f *fakeDB) Query(ctx context.Context, query string, args ...any) ([]DBRow, error) {
	f.calls = append(f.calls, fakeCall{kind: "query", query: query, args: args})
	if err := f.errs[query]; err != nil {
		return nil, err
	}
	return f.queries[query], nil
}

func (f *fakeDB) Exec(ctx context.Context, query string, args ...any) (DBExecResult, error) {
	f.calls = append(f.calls, fakeCall{kind: "exec", query: query, args: args})
	if err := f.errs[query]; err != nil {
		return DBExecResult{}, err
	}
	return f.execs[query], nil
}

func TestDBQueryHappyPath(t *testing.T) {
	db := newFakeDB()
	db.queries["SELECT id, name FROM users WHERE id = ?"] = []DBRow{
		{"id": int64(42), "name": "Alice"},
	}
	tt := New()
	tt.DB(db).
		Query("SELECT id, name FROM users WHERE id = ?", 42).
		ExpectOK().
		ExpectRowCount(1).
		ExpectColumn("name", "Alice").
		ExpectField(0, "id", 42).
		Extract(0, "name", "user")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	if tt.Vars()["user"] != "Alice" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
}

func TestDBExecHappyPath(t *testing.T) {
	db := newFakeDB()
	db.execs["UPDATE users SET name = ? WHERE id = ?"] = DBExecResult{RowsAffected: 1, LastInsertID: 0}
	tt := New()
	tt.DB(db).
		Exec("UPDATE users SET name = ? WHERE id = ?", "Bob", 42).
		ExpectOK().
		ExpectAffected(1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestDBQueryErrorPropagates(t *testing.T) {
	db := newFakeDB()
	db.errs["SELECT bad"] = errors.New("syntax error")
	tt := New()
	tt.DB(db).Query("SELECT bad").ExpectOK()
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failure")
	}
}

func TestDBExpectError(t *testing.T) {
	db := newFakeDB()
	db.errs["BAD"] = errors.New("nope")
	tt := New()
	tt.DB(db).Query("BAD").ExpectError()
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("ExpectError should pass on err, got: %v", tt.Errors())
	}
}

func TestDBExpectFieldRowOutOfRange(t *testing.T) {
	db := newFakeDB()
	tt := New()
	tt.DB(db).Query("SELECT * FROM nothing").
		ExpectField(0, "id", 1).
		Extract(0, "id", "x")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected row OOB failure")
	}
}

func TestDBExpectFieldColumnMissing(t *testing.T) {
	db := newFakeDB()
	db.queries["X"] = []DBRow{{"a": 1}}
	tt := New()
	tt.DB(db).Query("X").ExpectField(0, "missing", 1)
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected missing-column failure")
	}
}

func TestDBInterpolation(t *testing.T) {
	db := newFakeDB()
	db.queries["SELECT * FROM users WHERE id = 42"] = []DBRow{{"id": int64(42)}}
	tt := New()
	tt.SetVar("id", "42")
	tt.DB(db).Query("SELECT * FROM users WHERE id = {{id}}").ExpectRowCount(1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestDBArgsInterpolation(t *testing.T) {
	db := newFakeDB()
	db.queries["SELECT ?"] = []DBRow{{"x": "alice"}}
	tt := New()
	tt.SetVar("user", "alice")
	tt.DB(db).Query("SELECT ?", "{{user}}").ExpectRowCount(1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
	if len(db.calls) != 1 || db.calls[0].args[0] != "alice" {
		t.Fatalf("arg interpolation failed: %+v", db.calls)
	}
}

func TestDBRowsEscapeHatch(t *testing.T) {
	db := newFakeDB()
	db.queries["X"] = []DBRow{{"a": 1}, {"a": 2}}
	tt := New()
	step := tt.DB(db).Query("X")
	rows := step.Rows()
	step.Done()
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
}

func TestDBAtLeastRows(t *testing.T) {
	db := newFakeDB()
	db.queries["X"] = []DBRow{{"a": 1}, {"a": 2}, {"a": 3}}
	tt := New()
	tt.DB(db).Query("X").ExpectAtLeastRows(2)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestDBExtractTypeShapes(t *testing.T) {
	db := newFakeDB()
	db.queries["X"] = []DBRow{{
		"s": "alice",
		"i": int64(42),
		"f": 3.14,
		"b": true,
		"y": []byte("bytes"),
		"n": nil,
	}}
	tt := New()
	tt.DB(db).Query("X").
		Extract(0, "s", "vs").
		Extract(0, "i", "vi").
		Extract(0, "f", "vf").
		Extract(0, "b", "vb").
		Extract(0, "y", "vy").
		Extract(0, "n", "vn")
	tt.Finish()
	v := tt.Vars()
	if v["vs"] != "alice" || v["vi"] != "42" || v["vb"] != "true" || v["vy"] != "bytes" || v["vn"] != "" {
		t.Fatalf("Extract type shapes wrong: %+v", v)
	}
	// Float formatting should drop trailing zeros.
	if v["vf"] != "3.14" {
		t.Fatalf("Extract float: %q", v["vf"])
	}
}

func TestDBMisuseExpectAffectedAfterQuery(t *testing.T) {
	db := newFakeDB()
	db.queries["X"] = []DBRow{}
	tt := New()
	tt.DB(db).Query("X").ExpectAffected(1)
	tt.Finish()
	if tt.OK() {
		t.Fatal("ExpectAffected after Query() should fail")
	}
}

func TestSqlPreview(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SELECT 1", "SELECT 1"},
		{"  SELECT\t1\n  FROM t  ", "SELECT 1 FROM t"},
		{"SELECT " + repeatStr("a", 200), "SELECT " + repeatStr("a", 70) + "..."},
	}
	for _, c := range cases {
		if got := sqlPreview(c.in); got != c.want {
			t.Fatalf("sqlPreview(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func repeatStr(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
