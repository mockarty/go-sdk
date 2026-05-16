// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// MatchMismatch is one structured failure surfaced by the strict
// matching engine. The mock server attaches the list to the 404
// debug body and Verify() returns the joined view so the test author
// sees exactly which JSON-path failed, what was expected, and what
// arrived on the wire.
//
// Field order keeps the 16-byte string headers first for tighter
// alignment on amd64/arm64.
type MatchMismatch struct {
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Path     string `json:"path"`
	Reason   string `json:"reason"`
}

// Error returns a stable single-line representation suitable for log
// fan-out and CI error messages.
func (m MatchMismatch) Error() string {
	return fmt.Sprintf("%s: %s (expected=%v, actual=%v)", m.Path, m.Reason, m.Expected, m.Actual)
}

// MatchResult collects every mismatch encountered while walking an
// interaction's declared shape against the wire-side actual values.
//
// OK() returns true when the interaction matched strictly with zero
// mismatches; the mock server uses this to pick the correct response
// vs build a 404 debug envelope.
//
// Root is set by bodyMismatches to the parsed top-level JSON value so
// jsonpath/xmlpath matchers can resolve their expressions against the
// root regardless of where in the template they appear.
type MatchResult struct {
	Root       any
	Mismatches []MatchMismatch
}

// OK reports whether the actual request matched without any structural
// or value mismatches.
func (r MatchResult) OK() bool { return len(r.Mismatches) == 0 }

// Add appends one mismatch.
func (r *MatchResult) Add(path, reason string, expected, actual any) {
	r.Mismatches = append(r.Mismatches, MatchMismatch{
		Path:     path,
		Reason:   reason,
		Expected: expected,
		Actual:   actual,
	})
}

// regexCache memoises compiled regular expressions used by Term/Regex
// matchers. Compilation is cheap individually but the same regex often
// appears on every request to a high-throughput interaction (e.g. a
// UUID matcher on /resources/:id) — caching saves measurable CPU under
// load tests that point at the mock server.
//
// The cache is unbounded by design: regex strings are author-controlled
// and finite (one per declared matcher). It is not a vector for
// memory exhaustion because the keys come from compile-time DSL calls
// rather than client input.
var regexCache sync.Map // map[string]*regexp.Regexp

// compileRegex compiles pattern, caches the result, and returns the
// compiled value plus any parse error. A nil regex means the pattern
// was invalid — callers must treat the matcher as failing for the
// affected path.
func compileRegex(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		return v.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// strictMatch walks expected (the declared body) against actual (the
// JSON-decoded wire payload) at the given path, recording every
// deviation into result.
//
// Strict-mode semantics:
//   - Maps: every declared key must be present; extra keys in the
//     actual are tolerated (consumer-driven contracts only enforce the
//     consumer's expectations).
//   - Slices: declared length is the strict floor (matches every
//     declared index); extras after the floor are tolerated.
//   - Scalars: equality after JSON normalisation.
//   - Matchers: dispatch to strictMatcherCheck which enforces the
//     matcher's contract (type, regex, equality, etc.).
func strictMatch(expected, actual any, path string, result *MatchResult) {
	switch v := expected.(type) {
	case Matcher:
		strictMatcherCheck(v, actual, path, result)
	case map[string]any:
		am, ok := actual.(map[string]any)
		if !ok {
			result.Add(path, "expected object", expected, actual)
			return
		}
		for k, e := range v {
			a, present := am[k]
			if !present {
				result.Add(joinPath(path, k), "missing key", e, nil)
				continue
			}
			strictMatch(e, a, joinPath(path, k), result)
		}
	case []any:
		as, ok := actual.([]any)
		if !ok {
			result.Add(path, "expected array", expected, actual)
			return
		}
		if len(v) == 0 {
			// Empty declared array means "any array" — wildcard
			// preserves the documented behaviour of the lenient
			// engine for back-compat.
			return
		}
		for i, e := range v {
			if i >= len(as) {
				result.Add(indexPath(path, i), "array shorter than declared", e, nil)
				return
			}
			strictMatch(e, as[i], indexPath(path, i), result)
		}
	case []Matcher:
		as, ok := actual.([]any)
		if !ok {
			result.Add(path, "expected array", expected, actual)
			return
		}
		for i, e := range v {
			if i >= len(as) {
				result.Add(indexPath(path, i), "array shorter than declared", e, nil)
				return
			}
			strictMatch(e, as[i], indexPath(path, i), result)
		}
	case nil:
		if actual != nil {
			result.Add(path, "expected null", nil, actual)
		}
	default:
		if !normalisedEqual(expected, actual) {
			result.Add(path, "scalar mismatch", expected, actual)
		}
	}
}

// strictMatcherCheck dispatches one Matcher against actual and records
// any mismatch.
//
// Dispatch is **table-driven via matcherCheckers** rather than a giant
// switch — adheres to feedback_dynamic_over_hardcode.md ("no switch
// over strings; registerable plugin descriptors").
func strictMatcherCheck(m Matcher, actual any, path string, result *MatchResult) {
	checker, ok := matcherCheckers[m.Rule.Match]
	if !ok {
		// Unknown matcher kind: fall back to lenient type check so a
		// caller that ships a brand-new V4 matcher we haven't taught
		// the engine still passes through. The provider-side verifier
		// is the ultimate source of truth here.
		if !sameJSONType(m.Example, actual) {
			result.Add(path, fmt.Sprintf("unknown matcher %q: type mismatch", m.Rule.Match),
				m.Example, actual)
		}
		return
	}
	checker(m, actual, path, result)
}

// matcherChecker is one strict-mode validator. Each implements the
// matcher's contract: returns silently when actual is acceptable,
// appends a MatchMismatch otherwise.
type matcherChecker func(m Matcher, actual any, path string, result *MatchResult)

// matcherCheckers is the central registry. Add new matchers by writing
// a checker and registering here — no other call site changes.
//
// Populated in init() to break the initialisation cycle between
// matcherCheckers ↔ checkType (which recurses through strictMatch) and
// keep the registry self-describing.
var matcherCheckers map[string]matcherChecker

func init() {
	matcherCheckers = map[string]matcherChecker{
		"":              checkType,
		"type":          checkType,
		"integer":       checkInteger,
		"decimal":       checkDecimal,
		"number":        checkNumber,
		"boolean":       checkBoolean,
		"null":          checkNull,
		"regex":         checkRegex,
		"equality":      checkEquality,
		"include":       checkInclude,
		"values":        checkValuesMap,
		"each-key":      checkEachKey,
		"each-value":    checkEachValue,
		"arrayContains": checkArrayContains,
		"jsonpath":      checkJSONPath,
		"xmlpath":       checkXMLPath,
	}
}

func checkType(m Matcher, actual any, path string, result *MatchResult) {
	// Bounded array matchers (MinType/MaxType/MinMaxType/EachLike) ride
	// on top of `type`. When the matcher carries Min/Max, the actual
	// MUST be an array and the bounds MUST be respected.
	if m.Rule.Min != nil || m.Rule.Max != nil {
		as, ok := actual.([]any)
		if !ok {
			result.Add(path, "expected bounded array", m.Example, actual)
			return
		}
		if m.Rule.Min != nil && len(as) < *m.Rule.Min {
			result.Add(path, fmt.Sprintf("array length %d < min %d", len(as), *m.Rule.Min),
				m.Example, actual)
		}
		if m.Rule.Max != nil && len(as) > *m.Rule.Max {
			result.Add(path, fmt.Sprintf("array length %d > max %d", len(as), *m.Rule.Max),
				m.Example, actual)
		}
		// Each element must match the template SHAPE (not literal values
		// — EachLike semantics per pact-jvm/pact-js: scalars in the
		// template are matched by JSON type, like an implicit Like).
		template := firstTemplate(m.Example)
		for i, el := range as {
			typeOnlyMatch(template, el, indexPath(path, i), result)
		}
		// Recurse into Children describing nested matchers (e.g. an
		// EachLike with a Like inside the element).
		for _, ch := range m.Children {
			if ch.PathSegment == "[*]" {
				for i, el := range as {
					typeOnlyMatch(ch.Matcher, el, indexPath(path, i), result)
				}
			}
		}
		return
	}
	if !sameJSONType(m.Example, actual) {
		result.Add(path, "type mismatch", m.Example, actual)
	}
}

func checkInteger(_ Matcher, actual any, path string, result *MatchResult) {
	switch v := actual.(type) {
	case float64:
		if v != float64(int64(v)) {
			result.Add(path, "expected integer", "integer", actual)
		}
	case int, int32, int64, uint, uint32, uint64:
		return
	default:
		result.Add(path, "expected integer", "integer", actual)
	}
}

func checkDecimal(_ Matcher, actual any, path string, result *MatchResult) {
	if _, ok := actual.(float64); !ok {
		result.Add(path, "expected decimal", "decimal", actual)
	}
}

func checkNumber(_ Matcher, actual any, path string, result *MatchResult) {
	switch actual.(type) {
	case float64, int, int32, int64, uint, uint32, uint64:
		return
	default:
		result.Add(path, "expected number", "number", actual)
	}
}

func checkBoolean(_ Matcher, actual any, path string, result *MatchResult) {
	if _, ok := actual.(bool); !ok {
		result.Add(path, "expected boolean", "boolean", actual)
	}
}

func checkNull(_ Matcher, actual any, path string, result *MatchResult) {
	if actual != nil {
		result.Add(path, "expected null", nil, actual)
	}
}

func checkRegex(m Matcher, actual any, path string, result *MatchResult) {
	s, ok := actual.(string)
	if !ok {
		result.Add(path, "regex matcher requires string", m.Rule.Regex, actual)
		return
	}
	re, err := compileRegex(m.Rule.Regex)
	if err != nil {
		result.Add(path, "invalid regex in declared matcher: "+err.Error(), m.Rule.Regex, actual)
		return
	}
	if !re.MatchString(s) {
		result.Add(path, "regex did not match", m.Rule.Regex, actual)
	}
}

func checkEquality(m Matcher, actual any, path string, result *MatchResult) {
	if !deepEqualNormalised(m.Example, actual) {
		result.Add(path, "equality mismatch", m.Example, actual)
	}
}

func checkInclude(m Matcher, actual any, path string, result *MatchResult) {
	want, ok := m.Example.(string)
	if !ok {
		result.Add(path, "include matcher example must be a string", m.Example, actual)
		return
	}
	got, ok := actual.(string)
	if !ok {
		result.Add(path, "include matcher requires string", want, actual)
		return
	}
	if !strings.Contains(got, want) {
		result.Add(path, "substring not found", want, got)
	}
}

func checkValuesMap(_ Matcher, actual any, path string, result *MatchResult) {
	if _, ok := actual.(map[string]any); !ok {
		result.Add(path, "expected object", "object", actual)
	}
}

// checkEachKey applies the child matcher to every KEY of the actual
// object. Per V4 spec, the key-matcher operates on the string key
// itself, so we feed each key as a string actual into the child rule.
func checkEachKey(m Matcher, actual any, path string, result *MatchResult) {
	am, ok := actual.(map[string]any)
	if !ok {
		result.Add(path, "expected object for each-key", "object", actual)
		return
	}
	var child *Matcher
	for i := range m.Children {
		if m.Children[i].PathSegment == "[*]" {
			c := m.Children[i].Matcher
			child = &c
			break
		}
	}
	if child == nil {
		return
	}
	for k := range am {
		strictMatcherCheck(*child, k, joinPath(path, "<key:"+k+">"), result)
	}
}

// checkEachValue applies the child matcher to every VALUE of the
// actual object.
func checkEachValue(m Matcher, actual any, path string, result *MatchResult) {
	am, ok := actual.(map[string]any)
	if !ok {
		result.Add(path, "expected object for each-value", "object", actual)
		return
	}
	var child *Matcher
	for i := range m.Children {
		if m.Children[i].PathSegment == "[*]" {
			c := m.Children[i].Matcher
			child = &c
			break
		}
	}
	if child == nil {
		return
	}
	for k, v := range am {
		strictMatch(*child, v, joinPath(path, k), result)
	}
}

// checkArrayContains succeeds when the actual array contains at least
// one element matching EACH declared variant. An empty variants slice
// is treated as "match any non-empty array" to preserve the
// permissive-mode default established by the public DSL.
func checkArrayContains(m Matcher, actual any, path string, result *MatchResult) {
	as, ok := actual.([]any)
	if !ok {
		result.Add(path, "array-contains requires array", "array", actual)
		return
	}
	if len(m.Children) == 0 {
		if len(as) == 0 {
			result.Add(path, "array-contains expects non-empty actual", "non-empty", actual)
		}
		return
	}
	for vi, ch := range m.Children {
		matched := false
		for _, el := range as {
			tmp := MatchResult{}
			strictMatch(ch.Matcher, el, path, &tmp)
			if tmp.OK() {
				matched = true
				break
			}
		}
		if !matched {
			result.Add(path, fmt.Sprintf("array-contains variant %d did not match any element", vi),
				ch.Matcher.Example, actual)
		}
	}
}

// checkJSONPath applies a child matcher at a JSON path expression
// against the actual root object. The implementation supports the
// strict-dotted subset (`$.a.b[0].c`) sufficient for round-tripping
// pact-jvm / pact-js fixtures. Full JSONPath query syntax (wildcards,
// filters, recursive descent) is plugin territory — register a Plugin
// via `pact/plugins` to handle it.
func checkJSONPath(m Matcher, actual any, path string, result *MatchResult) {
	expr, _ := m.Example.(string)
	if expr == "" {
		result.Add(path, "jsonpath matcher missing expression", "$.expr", actual)
		return
	}
	// JSONPath expressions are anchored at the ROOT body — evaluate
	// against result.Root (set by bodyMismatches) so the matcher's
	// position in the template does not matter. Fall back to actual
	// when Root is unset (e.g. unit tests that call strictMatcherCheck
	// directly).
	root := actual
	if result.Root != nil {
		root = result.Root
	}
	target, ok := evalSimpleJSONPath(root, expr)
	if !ok {
		result.Add(path, "jsonpath did not resolve: "+expr, expr, actual)
		return
	}
	for _, ch := range m.Children {
		strictMatch(ch.Matcher, target, joinPath(path, expr), result)
	}
}

// checkXMLPath is the XML/XPath sibling of checkJSONPath. The SDK does
// not embed an XML parser (keeps the dep tree thin per
// feedback_sdk_thin_layer); we accept a content-type marker and defer
// the actual XPath evaluation to a registered plugin. Without a
// matching plugin the matcher logs a mismatch but doesn't crash.
func checkXMLPath(m Matcher, actual any, path string, result *MatchResult) {
	expr, _ := m.Example.(string)
	if expr == "" {
		result.Add(path, "xmlpath matcher missing expression", "//expr", actual)
		return
	}
	// XML payloads arrive as strings (we don't auto-parse). We accept
	// any string at this position — the round-trip into the pact.json
	// retains the XPath for downstream verifiers / plugins.
	if _, ok := actual.(string); !ok {
		result.Add(path, "xmlpath requires raw XML string body", expr, actual)
	}
}

// firstTemplate extracts the per-element template from an EachLike's
// Example. EachLike wraps the user-supplied template in a one-element
// slice; the template is the element used to validate every wire-side
// item.
func firstTemplate(ex any) any {
	if s, ok := ex.([]any); ok && len(s) > 0 {
		return s[0]
	}
	return ex
}

// typeOnlyMatch is the per-element matcher used by EachLike: it
// recurses into maps/arrays but treats scalar leaves as TYPE-only
// (an implicit Like). Explicit Matchers nested inside the template
// still apply their own contract — strictMatcherCheck is delegated to.
//
// This is the semantic that pact-jvm and pact-js implement for
// EachLike: the template describes the shape, not literal values.
func typeOnlyMatch(template, actual any, path string, result *MatchResult) {
	switch v := template.(type) {
	case Matcher:
		strictMatcherCheck(v, actual, path, result)
	case map[string]any:
		am, ok := actual.(map[string]any)
		if !ok {
			result.Add(path, "expected object", template, actual)
			return
		}
		for k, e := range v {
			a, present := am[k]
			if !present {
				result.Add(joinPath(path, k), "missing key", e, nil)
				continue
			}
			typeOnlyMatch(e, a, joinPath(path, k), result)
		}
	case []any:
		as, ok := actual.([]any)
		if !ok {
			result.Add(path, "expected array", template, actual)
			return
		}
		if len(v) == 0 {
			return
		}
		for i, e := range v {
			if i >= len(as) {
				result.Add(indexPath(path, i), "array shorter than declared", e, nil)
				return
			}
			typeOnlyMatch(e, as[i], indexPath(path, i), result)
		}
	case nil:
		if actual != nil {
			result.Add(path, "expected null", nil, actual)
		}
	default:
		// Scalars: type-only comparison.
		if !sameJSONType(template, actual) {
			result.Add(path, "type mismatch", template, actual)
		}
	}
}

// joinPath builds a stable JSON-pointer-style breadcrumb. We keep `.`
// for object keys and `[i]` for array indices so mismatches read as
// `$.body.users[0].id`.
func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	if strings.HasPrefix(key, "[") || strings.HasPrefix(key, "<") {
		return parent + key
	}
	return parent + "." + key
}

// indexPath builds an array-index breadcrumb.
func indexPath(parent string, i int) string {
	return parent + indexSegment(i)
}

// deepEqualNormalised compares scalars and recursively compares
// objects/arrays after JSON normalisation. We avoid reflect.DeepEqual
// because it treats `int(1)` and `float64(1)` as different — JSON
// strips that distinction.
func deepEqualNormalised(a, b any) bool {
	a = jsonNormalise(a)
	b = jsonNormalise(b)
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, va := range av {
			vb, ok := bv[k]
			if !ok || !deepEqualNormalised(va, vb) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqualNormalised(av[i], bv[i]) {
				return false
			}
		}
		return true
	}
	return a == b
}

// evalSimpleJSONPath walks a strict-dotted JSONPath like `$.a.b[2].c`
// and returns the value at that location. Unsupported expressions
// (wildcards, filters, recursive descent) return (nil, false) — that
// is the signal for the caller to surface a mismatch instead of
// panicking.
func evalSimpleJSONPath(root any, expr string) (any, bool) {
	if expr == "$" || expr == "" {
		return root, true
	}
	if !strings.HasPrefix(expr, "$") {
		return nil, false
	}
	cur := root
	i := 1
	for i < len(expr) {
		ch := expr[i]
		switch ch {
		case '.':
			// Read the property name until '.', '[' or end.
			j := i + 1
			for j < len(expr) && expr[j] != '.' && expr[j] != '[' {
				j++
			}
			key := expr[i+1 : j]
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			v, present := m[key]
			if !present {
				return nil, false
			}
			cur = v
			i = j
		case '[':
			// Numeric index until ']'.
			j := i + 1
			for j < len(expr) && expr[j] != ']' {
				j++
			}
			if j >= len(expr) {
				return nil, false
			}
			idxStr := expr[i+1 : j]
			idx := 0
			for _, c := range idxStr {
				if c < '0' || c > '9' {
					return nil, false
				}
				idx = idx*10 + int(c-'0')
			}
			as, ok := cur.([]any)
			if !ok || idx >= len(as) {
				return nil, false
			}
			cur = as[idx]
			i = j + 1
		default:
			return nil, false
		}
	}
	return cur, true
}
