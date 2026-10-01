package mockarty

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const CIContextEnvironment = "MOCKARTY_CI_CONTEXT"

var ErrCIContext = errors.New("invalid or unmatched selected CI context")

// CICallback is private result authorization. Serialize only to the receiver;
// its String representation intentionally omits the capability.
type CICallback struct {
	Token        string `json:"token"`
	LaunchID     string `json:"launchId"`
	ItemUID      string `json:"itemUid"`
	GroupID      string `json:"groupId"`
	AttemptID    string `json:"attemptId"`
	InvocationID string `json:"invocationId"`
}

func (CICallback) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "CICallback([REDACTED])")
}
func (CICallback) String() string { return "CICallback([REDACTED])" }
func validCIUUID(s string) bool {
	u, e := uuid.Parse(s)
	return e == nil && u != uuid.Nil && u.String() == s
}
func (c CICallback) validate(completion bool) error {
	token, e := base64.RawURLEncoding.Strict().DecodeString(c.Token)
	if e != nil || len(token) != 32 || len(c.Token) != 43 || !validCIUUID(c.LaunchID) || !validCIUUID(c.GroupID) || !validCIUUID(c.AttemptID) || !validCIUUID(c.InvocationID) {
		return ErrCIContext
	}
	if completion {
		if c.ItemUID != "" && c.ItemUID != uuid.Nil.String() {
			return ErrCIContext
		}
	} else if !validCIUUID(c.ItemUID) {
		return ErrCIContext
	}
	return nil
}

// CIJobContext contains a frozen selected occurrence set delivered by the job.
// Loading it grants no server authority; the receiver verifies every callback.
type CIJobContext struct {
	EnvelopeFormat string           `json:"format"`
	Namespace      string           `json:"namespace"`
	Token          string           `json:"token"`
	LaunchID       string           `json:"launchId"`
	GroupID        string           `json:"groupId"`
	AttemptID      string           `json:"attemptId"`
	InvocationID   string           `json:"invocationId"`
	Items          []CISelectedItem `json:"items"`
	raw            string
}
type CISelectedItem struct {
	Selector    CISelector `json:"selector"`
	ItemUID     string     `json:"itemUid"`
	CaseID      string     `json:"caseId"`
	VersionID   string     `json:"versionId"`
	JobConfigID string     `json:"jobConfigId"`
	GroupID     string     `json:"groupId"`
	AttemptID   string     `json:"attemptId"`
}
type CISelector struct {
	ID       string `json:"id,omitempty"`
	FullName string `json:"selector,omitempty"`
}

// Optional selector members, when present, must be JSON strings; null is
// not an omitted identity and must not silently erase a frozen selector.
func (selector *CISelector) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil || len(fields) > 2 {
		return ErrCIContext
	}
	*selector = CISelector{}
	for key, value := range fields {
		if len(value) == 0 || value[0] != '"' {
			return ErrCIContext
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			return ErrCIContext
		}
		switch key {
		case "id":
			selector.ID = text
		case "selector":
			selector.FullName = text
		default:
			return ErrCIContext
		}
	}
	return nil
}
func (CIJobContext) Format(state fmt.State, verb rune) {
	_, _ = io.WriteString(state, "CIJobContext([REDACTED])")
}
func (CIJobContext) String() string { return "CIJobContext([REDACTED])" }
func LoadCIJobContext(raw string) (*CIJobContext, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, ErrCIContext
	}
	b, e := base64.RawURLEncoding.Strict().DecodeString(raw)
	if e != nil || !utf8.Valid(b) {
		return nil, ErrCIContext
	}
	if err := uniqueCIJSON(json.NewDecoder(bytes.NewReader(b)), 0); err != nil {
		return nil, ErrCIContext
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var job CIJobContext
	if e = d.Decode(&job); e != nil {
		return nil, ErrCIContext
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return nil, ErrCIContext
	}
	if job.EnvelopeFormat != "mockarty.native-ci-job/v1" || !ciText(job.Namespace, 255) || len(job.Items) == 0 || len(job.Items) > 1000 {
		return nil, ErrCIContext
	}
	if e = job.Callback().validate(true); e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, i := range job.Items {
		if !validCIUUID(i.ItemUID) || !validCIUUID(i.CaseID) || !validCIUUID(i.VersionID) || !validCIUUID(i.JobConfigID) || i.GroupID != job.GroupID || i.AttemptID != job.AttemptID || seen[i.ItemUID] || (i.Selector.ID == "" && i.Selector.FullName == "") || i.Selector.ID != "" && !ciText(i.Selector.ID, 512) || i.Selector.FullName != "" && !ciText(i.Selector.FullName, 2048) {
			return nil, ErrCIContext
		}
		seen[i.ItemUID] = true
	}
	job.raw = raw
	return &job, nil
}
func ciText(s string, max int) bool {
	return s != "" && len(s) <= max && strings.TrimSpace(s) == s && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}
func (j *CIJobContext) Callback() CICallback {
	if j == nil {
		return CICallback{}
	}
	return CICallback{Token: j.Token, LaunchID: j.LaunchID, ItemUID: uuid.Nil.String(), GroupID: j.GroupID, AttemptID: j.AttemptID, InvocationID: j.InvocationID}
}

// matchesReport is a local directory prefilter. Ambiguous frozen occurrences
// are refused; an unmatched source file can be skipped before reading its media.
func (j *CIJobContext) matchesReport(run ExternalRunRequest) (bool, error) {
	if j == nil {
		return false, ErrCIContext
	}
	matches := 0
	for _, i := range j.Items {
		if i.Selector.ID != "" && i.Selector.ID != run.ExternalID && i.Selector.ID != run.TestCaseID {
			continue
		}
		if i.Selector.FullName != "" && i.Selector.FullName != run.FullName {
			continue
		}
		matches++
	}
	if matches > 1 {
		return false, ErrCIContext
	}
	return matches == 1, nil
}

// BindReport requires exactly one frozen occurrence; it never falls back to name
// lookup, creates cases, or joins an ordinary launch.
func (j *CIJobContext) BindReport(run ExternalRunRequest) (ExternalRunRequest, error) {
	if j == nil {
		return run, ErrCIContext
	}
	var item *CISelectedItem
	for _, i := range j.Items {
		if i.Selector.ID != "" && i.Selector.ID != run.ExternalID && i.Selector.ID != run.TestCaseID {
			continue
		}
		if i.Selector.FullName != "" && i.Selector.FullName != run.FullName {
			continue
		}
		if item != nil {
			return run, ErrCIContext
		}
		copy := i
		item = &copy
	}
	if item == nil || run.CaseID != "" && run.CaseID != item.CaseID {
		return run, ErrCIContext
	}
	cb := j.Callback()
	cb.ItemUID = item.ItemUID
	if run.CIContext != nil && *run.CIContext != cb {
		return run, ErrCIContext
	}
	run.CaseID = item.CaseID
	run.CIContext = &cb
	run.PlanID = ""
	run.PlanRunID = ""
	run.AutoCreate = false
	run.ClaimCaseOwnership = false
	return run, nil
}
func prepareCIReport(namespace string, run ExternalRunRequest) (ExternalRunRequest, error) {
	raw := os.Getenv(CIContextEnvironment)
	if raw != "" {
		job, e := LoadCIJobContext(raw)
		if e != nil || job.Namespace != namespace {
			return run, ErrCIContext
		}
		run, e = job.BindReport(run)
		if e != nil {
			return run, e
		}
	}
	if run.CIContext == nil {
		return run, nil
	}
	if e := run.CIContext.validate(false); e != nil {
		return run, e
	}
	// Clone recursively through JSON so all metadata bags and name/value records
	// are covered and caller objects remain untouched. Callback stays separate.
	cb := *run.CIContext
	run.CIContext = nil
	b, e := json.Marshal(run)
	if e != nil {
		return run, errors.New("invalid report payload")
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if e = decoder.Decode(&value); e != nil {
		return run, errors.New("invalid report payload")
	}
	secrets := []string{cb.Token}
	if raw != "" {
		secrets = append(secrets, raw)
	}
	value, e = cleanCIValue(value, secrets, 0)
	if e != nil {
		return run, e
	}
	b, e = json.Marshal(value)
	if e != nil {
		return run, ErrCIContext
	}
	run = ExternalRunRequest{}
	decoder = json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if e = decoder.Decode(&run); e != nil {
		return run, ErrCIContext
	}
	run.CIContext = &cb
	if e := validateSelectedCIReport(run); e != nil {
		return run, e
	}
	return run, nil
}
func cleanCIValue(v any, secrets []string, depth int) (any, error) {
	return cleanCIValueAt(v, secrets, depth, false)
}
func cleanCIValueAt(v any, secrets []string, depth int, attachment bool) (any, error) {
	if depth > 64 {
		return nil, ErrCIContext
	}
	switch x := v.(type) {
	case map[string]any:
		if name, _ := x["name"].(string); strings.EqualFold(name, CIContextEnvironment) {
			return nil, nil
		}
		for k, item := range x {
			if strings.EqualFold(k, CIContextEnvironment) {
				delete(x, k)
				continue
			}
			if k == "bodyB64" && attachment && depth == 2 {
				s, _ := item.(string)
				b, e := base64.StdEncoding.DecodeString(s)
				if e != nil {
					return nil, errors.New("invalid attachment encoding")
				}
				for _, secret := range secrets {
					if bytes.Contains(b, []byte(secret)) {
						return nil, errors.New("attachment contains private CI capability")
					}
				}
				// Only the typed root attachment field contains opaque file bytes.
				continue
			}
			clean, e := cleanCIValueAt(item, secrets, depth+1, depth == 0 && k == "attachments")
			if e != nil {
				return nil, e
			}
			x[k] = clean
		}
		return x, nil
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			clean, e := cleanCIValueAt(item, secrets, depth+1, attachment)
			if e != nil {
				return nil, e
			}
			if clean != nil {
				out = append(out, clean)
			}
		}
		return out, nil
	case string:
		for _, s := range secrets {
			x = strings.ReplaceAll(x, s, "[REDACTED]")
		}
		return x, nil
	}
	return v, nil
}

// CompleteGroup reports the test command outcome after all selected reports have
// been uploaded. Missing items remain a server-side failure; this is not success
// inferred from a single result. Selected requests never automatically retry.
func (a *ExternalRunsAPI) CompleteGroup(ctx context.Context, namespace string, callback CICallback, status string) (json.RawMessage, error) {
	if ctx == nil || a == nil || a.client == nil {
		return nil, ErrCIContext
	}
	if e := callback.validate(true); e != nil {
		return nil, e
	}
	if status != "passed" && status != "failed" && status != "cancelled" {
		return nil, ErrCIContext
	}
	if namespace == "" {
		namespace = a.client.namespace
	}
	if !ciText(namespace, 255) {
		return nil, ErrCIContext
	}
	callback.ItemUID = uuid.Nil.String()
	path := "/api/v1/namespaces/" + url.PathEscape(namespace) + "/tcm/ci-launches/" + callback.LaunchID + "/groups/" + callback.GroupID + "/complete"
	ctx = context.WithValue(ctx, nonReplayableRequestKey{}, true)
	b, e := a.client.doJSON(ctx, "POST", path, map[string]any{"callback": callback, "status": status})
	if e != nil {
		return nil, fmt.Errorf("complete selected CI group: %w", e)
	}
	return b, nil
}

// Decoder parity: duplicate keys and oversized nesting cannot silently replace
// a selected identity or private capability.
func uniqueCIJSON(d *json.Decoder, depth int) error {
	if depth > 64 {
		return ErrCIContext
	}
	token, err := d.Token()
	if err != nil {
		return ErrCIContext
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return ErrCIContext
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrCIContext
			}
			seen[name] = true
			if e = uniqueCIJSON(d, depth+1); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if e := uniqueCIJSON(d, depth+1); e != nil {
				return e
			}
		}
	default:
		return ErrCIContext
	}
	_, err = d.Token()
	return err
}

// Validate the detached selected body before any batch/directory HTTP effect.
func validateSelectedCIReport(run ExternalRunRequest) error {
	if run.SchemaVersion != 0 && run.SchemaVersion != ExternalRunSchemaVersion {
		return ErrCIContext
	}
	switch run.Status {
	case "passed", "failed", "skipped", "cancelled", "broken", "error":
	default:
		return ErrCIContext
	}
	if len(run.Steps) > 1000 || len(run.Attachments) > 32 {
		return ErrCIContext
	}
	for _, step := range run.Steps {
		if strings.TrimSpace(step.Name) == "" {
			return ErrCIContext
		}
		switch step.Status {
		case "passed", "failed", "skipped", "broken", "error":
		default:
			return ErrCIContext
		}
	}
	total := 0
	for _, a := range run.Attachments {
		inline, ref := strings.TrimSpace(a.BodyB64) != "", strings.TrimSpace(a.BlobURI) != ""
		if strings.TrimSpace(a.Name) == "" || inline == ref {
			return ErrCIContext
		}
		if ref {
			continue
		}
		if len(a.BodyB64) > (256<<10)*4/3+4 {
			return ErrCIContext
		}
		body, e := base64.StdEncoding.DecodeString(a.BodyB64)
		if e != nil || len(body) > 256<<10 {
			return ErrCIContext
		}
		total += len(body)
		if total > 1<<20 {
			return ErrCIContext
		}
	}
	return nil
}
