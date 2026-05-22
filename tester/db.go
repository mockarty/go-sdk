// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DBRow is one result row, keyed by column name.
type DBRow map[string]any

// DBExecResult is what Exec returns from an INSERT/UPDATE/DELETE.
type DBExecResult struct {
	RowsAffected int64
	LastInsertID int64
}

// SQLConn is the minimal contract the DB facet needs. *sql.DB and
// *sql.Tx satisfy it via thin adapters — but in this SDK the interface
// is kept dialect-agnostic so users plug in whatever they want
// (driver-less mock in unit tests, real DB in CI).
//
// Query returns all rows materialised as DBRow values (column → value).
// Drivers vary in how they expose decimals / timestamps — the adapter
// is the right place to normalise.
type SQLConn interface {
	Query(ctx context.Context, query string, args ...any) ([]DBRow, error)
	Exec(ctx context.Context, query string, args ...any) (DBExecResult, error)
}

// DBFacet is the SQL entry point reached via Tester.DB(conn).
type DBFacet struct {
	t    *Tester
	conn SQLConn
}

// DB returns the DB facet bound to the supplied connection.
func (t *Tester) DB(conn SQLConn) *DBFacet {
	t.flushPending()
	return &DBFacet{t: t, conn: conn}
}

// Query starts a SELECT chain. Both the SQL text and string-typed args
// pass through {{var}} interpolation.
//
//	t.DB(db).
//	  Query("SELECT id, name FROM users WHERE id = ?", 42).
//	  ExpectRowCount(1).
//	  ExpectField(0, "name", "Alice").
//	  Extract(0, "name", "user")
func (d *DBFacet) Query(query string, args ...any) *DBStep {
	step := &DBStep{
		t:     d.t,
		conn:  d.conn,
		kind:  "query",
		query: interpolate(query, d.t.snapshotVars()),
		args:  interpolateArgs(args, d.t.snapshotVars()),
	}
	d.t.setPending(step)
	return step
}

// Exec starts an INSERT/UPDATE/DELETE chain.
func (d *DBFacet) Exec(query string, args ...any) *DBStep {
	step := &DBStep{
		t:     d.t,
		conn:  d.conn,
		kind:  "exec",
		query: interpolate(query, d.t.snapshotVars()),
		args:  interpolateArgs(args, d.t.snapshotVars()),
	}
	d.t.setPending(step)
	return step
}

func interpolateArgs(args []any, vars map[string]string) []any {
	out := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			out[i] = interpolate(s, vars)
		} else {
			out[i] = a
		}
	}
	return out
}

// DBStep is one Query or Exec call.
type DBStep struct {
	t     *Tester
	conn  SQLConn
	kind  string // "query" | "exec"
	query string
	args  []any

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	rows       []DBRow
	result     DBExecResult
	err        error
	failures   []string
}

// ExpectOK asserts the query/exec ran without error.
func (s *DBStep) ExpectOK() *DBStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectOK: %v", s.err))
	}
	return s
}

// ExpectError asserts the query/exec returned an error.
func (s *DBStep) ExpectError() *DBStep {
	s.ensureSent()
	if s.err == nil {
		s.fail("ExpectError: query succeeded")
	}
	return s
}

// ExpectRowCount asserts the query returned exactly n rows.
func (s *DBStep) ExpectRowCount(n int) *DBStep {
	if !s.ensureSent() {
		return s
	}
	if s.kind != "query" {
		s.fail("ExpectRowCount only valid after Query()")
		return s
	}
	if len(s.rows) != n {
		s.fail(fmt.Sprintf("ExpectRowCount: want %d, got %d", n, len(s.rows)))
	}
	return s
}

// ExpectAtLeastRows asserts at least n rows were returned.
func (s *DBStep) ExpectAtLeastRows(n int) *DBStep {
	if !s.ensureSent() {
		return s
	}
	if s.kind != "query" {
		s.fail("ExpectAtLeastRows only valid after Query()")
		return s
	}
	if len(s.rows) < n {
		s.fail(fmt.Sprintf("ExpectAtLeastRows: want >=%d, got %d", n, len(s.rows)))
	}
	return s
}

// ExpectField asserts the value at (row, col) equals want. Uses the
// loose equality rules from equalJSONScalar so numeric driver types
// (int64 vs float64) compare correctly.
func (s *DBStep) ExpectField(row int, col string, want any) *DBStep {
	if !s.ensureSent() {
		return s
	}
	if row < 0 || row >= len(s.rows) {
		s.fail(fmt.Sprintf("ExpectField[%d.%s]: row out of range (len=%d)", row, col, len(s.rows)))
		return s
	}
	got, ok := s.rows[row][col]
	if !ok {
		s.fail(fmt.Sprintf("ExpectField[%d.%s]: column not in result", row, col))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectField[%d.%s]: want %v, got %v", row, col, want, got))
	}
	return s
}

// ExpectColumn is a convenience for "row 0, column col".
func (s *DBStep) ExpectColumn(col string, want any) *DBStep {
	return s.ExpectField(0, col, want)
}

// ExpectAffected asserts an Exec's RowsAffected.
func (s *DBStep) ExpectAffected(n int64) *DBStep {
	if !s.ensureSent() {
		return s
	}
	if s.kind != "exec" {
		s.fail("ExpectAffected only valid after Exec()")
		return s
	}
	if s.result.RowsAffected != n {
		s.fail(fmt.Sprintf("ExpectAffected: want %d, got %d", n, s.result.RowsAffected))
	}
	return s
}

// Extract stores the value at (row, col) under name.
func (s *DBStep) Extract(row int, col, name string) *DBStep {
	if !s.ensureSent() {
		return s
	}
	if row < 0 || row >= len(s.rows) {
		s.fail(fmt.Sprintf("Extract[%d.%s]: row out of range (len=%d)", row, col, len(s.rows)))
		return s
	}
	v, ok := s.rows[row][col]
	if !ok {
		s.fail(fmt.Sprintf("Extract[%d.%s]: column not in result", row, col))
		return s
	}
	var str string
	switch x := v.(type) {
	case nil:
		str = ""
	case string:
		str = x
	case []byte:
		str = string(x)
	case bool:
		str = fmt.Sprintf("%t", x)
	case int:
		str = fmt.Sprintf("%d", x)
	case int64:
		str = fmt.Sprintf("%d", x)
	case float64:
		str = formatNumber(x)
	default:
		b, _ := json.Marshal(v)
		str = string(b)
	}
	s.t.SetVar(name, str)
	return s
}

// Rows returns a snapshot of the result rows — escape hatch.
func (s *DBStep) Rows() []DBRow {
	s.ensureSent()
	out := make([]DBRow, len(s.rows))
	for i, r := range s.rows {
		c := make(DBRow, len(r))
		for k, v := range r {
			c[k] = v
		}
		out[i] = c
	}
	return out
}

// Result returns the Exec result (RowsAffected / LastInsertID).
func (s *DBStep) Result() DBExecResult {
	s.ensureSent()
	return s.result
}

// Done finalises the step.
func (s *DBStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *DBStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *DBStep) ensureSent() bool {
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
	if s.kind == "exec" {
		s.result, s.err = s.conn.Exec(ctx, s.query, s.args...)
	} else {
		s.rows, s.err = s.conn.Query(ctx, s.query, s.args...)
	}
	s.endedAt = time.Now()
	// Don't auto-record the err as a failure — let ExpectOK / ExpectError
	// turn it into a verdict. Assertions that follow operate on whatever
	// rows / result were returned (empty when err != nil), which still
	// surface as ExpectRowCount mismatches when the caller doesn't use
	// ExpectError.
	return true
}

func (s *DBStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	name := "sql " + s.kind + " " + sqlPreview(s.query)
	rec := StepRecord{
		Protocol:  "sql",
		Method:    s.kind,
		Name:      name,
		URL:       sqlPreview(s.query),
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	if s.kind == "query" {
		rec.StatusOrCode = len(s.rows)
	} else {
		rec.StatusOrCode = int(s.result.RowsAffected)
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}

// sqlPreview returns a compact one-line summary of the SQL — strips
// excess whitespace and clips at 80 chars so reports stay readable.
func sqlPreview(q string) string {
	q = strings.Join(strings.Fields(q), " ")
	if len(q) > 80 {
		return q[:77] + "..."
	}
	return q
}
