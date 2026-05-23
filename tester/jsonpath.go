// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"fmt"
	"strconv"
	"strings"
)

// resolveJSONPath walks a parsed JSON value (any) along a dotted /
// bracketed path and returns the value at the end. Supported shapes:
//
//	$            — the document root
//	$.a.b.c      — nested object keys
//	$.arr[0]     — array index
//	$.arr[-1]    — last element
//	$.arr[*]     — every element (returns []any of the selections)
//	$.arr[0].x   — combined
//
// Anything fancier (filters, slices, recursive descent) is intentionally
// excluded — the SDK keeps its dependency footprint zero. Callers that
// need full JSONPath can plug in their own resolver via a separate API.
func resolveJSONPath(root any, path string) (any, error) {
	if path == "" || path == "$" {
		return root, nil
	}
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("jsonpath must start with $: %q", path)
	}
	segments, err := splitPath(path[1:])
	if err != nil {
		return nil, err
	}
	cur := root
	for i, seg := range segments {
		next, err := stepJSONPath(cur, seg)
		if err != nil {
			return nil, fmt.Errorf("at %s: %w", strings.Join(segments[:i+1], ""), err)
		}
		cur = next
	}
	return cur, nil
}

// splitPath turns ".a.b[0][*].c" into ["a","b","[0]","[*]","c"]. Each
// segment is a key name (no brackets) or an indexer including its
// brackets. Empty segments are rejected.
func splitPath(s string) ([]string, error) {
	var out []string
	for i := 0; i < len(s); {
		switch s[i] {
		case '.':
			i++
			j := i
			for j < len(s) && s[j] != '.' && s[j] != '[' {
				j++
			}
			if j == i {
				return nil, fmt.Errorf("empty segment at offset %d", i)
			}
			out = append(out, s[i:j])
			i = j
		case '[':
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unterminated [ at offset %d", i)
			}
			out = append(out, s[i:i+end+1])
			i += end + 1
		default:
			return nil, fmt.Errorf("unexpected char %q at offset %d", s[i], i)
		}
	}
	return out, nil
}

func stepJSONPath(cur any, seg string) (any, error) {
	if strings.HasPrefix(seg, "[") {
		inner := strings.TrimSuffix(strings.TrimPrefix(seg, "["), "]")
		arr, ok := cur.([]any)
		if !ok {
			return nil, fmt.Errorf("indexer on non-array (%T)", cur)
		}
		if inner == "*" {
			out := make([]any, len(arr))
			copy(out, arr)
			return out, nil
		}
		idx, err := strconv.Atoi(inner)
		if err != nil {
			return nil, fmt.Errorf("invalid index %q", inner)
		}
		if idx < 0 {
			idx += len(arr)
		}
		if idx < 0 || idx >= len(arr) {
			return nil, fmt.Errorf("index %s out of range (len=%d)", inner, len(arr))
		}
		return arr[idx], nil
	}
	obj, ok := cur.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("key access on non-object (%T)", cur)
	}
	v, ok := obj[seg]
	if !ok {
		return nil, fmt.Errorf("key %q not found", seg)
	}
	return v, nil
}

// equalJSONScalar compares the resolved jsonpath value to the expected
// value with sensible loose equality: numbers compare by float64,
// strings/bool by direct ==, nil by both-nil. Arrays/maps must match
// using fmt.Sprintf("%v") representations — callers that want deep
// equality should pass the result through their preferred assertion lib.
func equalJSONScalar(got, want any) bool {
	if got == nil && want == nil {
		return true
	}
	if got == nil || want == nil {
		return false
	}
	switch w := want.(type) {
	case int:
		return floatEqual(toFloat(got), float64(w))
	case int64:
		return floatEqual(toFloat(got), float64(w))
	case float64:
		return floatEqual(toFloat(got), w)
	case string:
		gs, ok := got.(string)
		return ok && gs == w
	case bool:
		gb, ok := got.(bool)
		return ok && gb == w
	default:
		return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f
		}
	}
	return 0
}

func floatEqual(a, b float64) bool { return a == b }
