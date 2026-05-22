// Copyright (c) 2026 Mockarty. All rights reserved.

package tester

import "strings"

// interpolate replaces every "{{name}}" token in s with the value from
// vars. Unknown names are left as the literal token so failures surface
// instead of silent empty strings.
//
// Implementation is allocation-conscious: when the input contains no
// "{{" the original string is returned unchanged.
func interpolate(s string, vars map[string]string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			end := strings.Index(s[i+2:], "}}")
			if end < 0 {
				b.WriteString(s[i:])
				break
			}
			name := strings.TrimSpace(s[i+2 : i+2+end])
			if v, ok := vars[name]; ok {
				b.WriteString(v)
			} else {
				b.WriteString(s[i : i+2+end+2])
			}
			i += 2 + end + 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
