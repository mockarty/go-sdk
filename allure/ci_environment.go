package allure

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

// Environment snapshots are public report data; job capabilities stay private.
func ciEnvironment(env map[string]string) map[string]string {
	secrets := []string{}
	raw := os.Getenv("MOCKARTY_CI_CONTEXT")
	for k, v := range env {
		if strings.EqualFold(k, "MOCKARTY_CI_CONTEXT") {
			if v != "" {
				secrets = append(secrets, v)
			}
			if raw == "" {
				raw = v
			}
		}
	}
	if raw != "" {
		secrets = append(secrets, raw)
		if b, e := base64.RawURLEncoding.DecodeString(raw); e == nil {
			var j struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(b, &j) == nil && j.Token != "" {
				secrets = append(secrets, j.Token)
			}
		}
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		if strings.EqualFold(k, "MOCKARTY_CI_CONTEXT") {
			continue
		}
		for _, s := range secrets {
			v = strings.ReplaceAll(v, s, "[REDACTED]")
		}
		out[k] = v
	}
	return out
}
