// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// ParameterCase pairs a logical case name with a payload of any type. Used
// by [ParameterizedTest] for the most flexible call shape — when the payload
// is a struct the fields become Allure parameters automatically.
type ParameterCase[T any] struct {
	Name    string
	Payload T
}

// ParameterizedTest runs fn under a fresh subtest for each case. Each
// iteration produces a distinct Allure result with the case's fields
// mirrored as parameters (so Allure renders one row per case in the
// history view, with parameter columns).
//
// Subtest naming follows Go's `testing.T` convention — `t.Run(case.Name)` —
// so the standard `-run` filter works as expected.
//
// Example:
//
//	cases := []allure.ParameterCase[struct{ Input, Want string }]{
//	    {"happy", struct{...}{"a", "b"}},
//	    {"empty", struct{...}{"", ""}},
//	}
//	allure.ParameterizedTest(t, cases, func(t *testing.T, c struct{...}) {
//	    a := allure.T(t)
//	    a.Step("check", func() { ... })
//	})
func ParameterizedTest[T any](t *testing.T, cases []ParameterCase[T], fn func(t *testing.T, p T)) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		name := tc.Name
		if name == "" {
			name = fmt.Sprintf("case-%v", tc.Payload)
		}
		params := derivePayloadParameters(tc.Payload)
		t.Run(name, func(inner *testing.T) {
			inner.Helper()
			// Stash parameters in a deferred config option — fn calls T(inner, ...)
			// itself and picks them up via WithParameter. But since fn owns the
			// AllureT construction, we mirror parameters into a goroutine-local
			// stash that's read by the next T() call in this subtest.
			pushPendingParameters(inner.Name(), params)
			defer popPendingParameters(inner.Name())
			fn(inner, tc.Payload)
		})
	}
}

// ParameterizedRows is a simpler variant for the common case where each
// case is just a string label + a slice of name/value parameters (mirrors
// pytest's @pytest.mark.parametrize(...) shape directly).
func ParameterizedRows(t *testing.T, header []string, rows [][]string, fn func(t *testing.T, params map[string]string)) {
	t.Helper()
	for i, row := range rows {
		name := fmt.Sprintf("row-%d", i)
		params := make([]AllureParameter, 0, len(header))
		paramMap := make(map[string]string, len(header))
		for col := range header {
			val := ""
			if col < len(row) {
				val = row[col]
			}
			params = append(params, AllureParameter{Name: header[col], Value: val})
			paramMap[header[col]] = val
		}
		if len(row) > 0 {
			name = row[0]
		}
		t.Run(name, func(inner *testing.T) {
			inner.Helper()
			pushPendingParameters(inner.Name(), params)
			defer popPendingParameters(inner.Name())
			fn(inner, paramMap)
		})
	}
}

// derivePayloadParameters reflects over a struct payload and returns one
// AllureParameter per exported field. Non-struct payloads produce a single
// "value" parameter with the formatted representation.
func derivePayloadParameters(payload any) []AllureParameter {
	v := reflect.ValueOf(payload)
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return []AllureParameter{{Name: "value", Value: fmt.Sprintf("%v", v.Interface())}}
	}
	t := v.Type()
	out := make([]AllureParameter, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		val := v.Field(i)
		name := f.Name
		if tag, ok := f.Tag.Lookup("allure"); ok {
			tag = strings.TrimSpace(tag)
			if tag == "-" {
				continue
			}
			if tag != "" {
				name = strings.SplitN(tag, ",", 2)[0]
			}
		}
		out = append(out, AllureParameter{Name: name, Value: fmt.Sprintf("%v", val.Interface())})
	}
	return out
}
