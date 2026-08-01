// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// internalLabelPrefix marks SDK-private labels carried on the config label
// slice (e.g. WithIssuePattern). Labels with this prefix are stripped before
// the result is written so they never appear in the Allure report.
const internalLabelPrefix = "_internal."

// scope is the in-memory accumulator for a single test. It collects steps,
// attachments, labels and links and produces a serialisable [Result] on
// finish().
//
// scope is safe for concurrent use: every public mutator takes the embedded
// RWMutex. The mutex sits at the top of the struct for cache-line alignment
// (Mockarty struct-alignment convention).
type scope struct {
	writer     ResultWriter
	now        func() time.Time
	cfg        config
	stepStack  []*AllureStep
	result     Result
	startNS    int64
	finishOnce sync.Once
	mu         sync.Mutex
}

// newScope builds the accumulator from a config; clock falls back to
// time.Now.
func newScope(cfg config, writer ResultWriter) *scope {
	clock := cfg.now
	if clock == nil {
		clock = time.Now
	}
	id := uuid.NewString()
	s := &scope{
		cfg:     cfg,
		now:     clock,
		writer:  writer,
		startNS: clock().UnixMilli(),
	}
	name := cfg.name
	if name == "" {
		name = "unnamed"
	}
	fullName := cfg.fullName
	if fullName == "" {
		fullName = name
	}
	s.result = Result{
		UUID:        id,
		Name:        name,
		FullName:    fullName,
		Status:      StatusPassed,
		Stage:       StageScheduled,
		Start:       s.startNS,
		Labels:      []AllureLabel{},
		Links:       []AllureLink{},
		Parameters:  []AllureParameter{},
		Steps:       []AllureStep{},
		Attachments: []AllureAttachment{},
	}
	// Stable historyId — byte-identical to allure-python's get_history_id:
	// md5(fullName + non-excluded parameter VALUES sorted by name), no
	// separators. Re-runs of the same parameter combination cluster together
	// in Allure's history view; differing parameters fork into distinct
	// history series.
	s.result.HistoryID = computeHistoryID(fullName, cfg.parameters)
	// testCaseId is the parameter-INDEPENDENT identity = md5(fullName),
	// matching allure-pytest (test_result.testCaseId = md5(full_name)). Unlike
	// historyId it stays constant across parameterised iterations, so TCM
	// discovery can match the case by fullName when no explicit id is set.
	s.result.TestCaseID = computeTestCaseID(fullName)

	s.applyConfigLabels()
	if cfg.description != "" {
		s.result.Description = cfg.description
	}
	if cfg.descrHTML != "" {
		s.result.DescriptionHTML = cfg.descrHTML
	}
	return s
}

// computeHistoryID is byte-identical to allure-python's get_history_id:
// md5(fullName + non-excluded parameter VALUES sorted by parameter name),
// concatenated with NO separator. Allure's history-id contract: two runs
// with the same history-id are considered "the same test", so the algorithm
// MUST match across SDKs and against allure-pytest for history/trend/retry
// linking to work in a mixed-language report.
//
// Reference: allure-framework/allure-python allure-python-commons/src/
// allure_commons/utils.py (md5 + get_history_id).
func computeHistoryID(fullName string, params []AllureParameter) string {
	values := make([]string, 0, len(params))
	idx := make([]int, 0, len(params))
	for i, p := range params {
		if p.Excluded {
			continue
		}
		idx = append(idx, i)
	}
	// Sort the surviving parameters by name (stable for equal names), then
	// take their values — names are NOT folded into the hash (Allure parity).
	sort.SliceStable(idx, func(a, b int) bool {
		return params[idx[a]].Name < params[idx[b]].Name
	})
	for _, i := range idx {
		values = append(values, params[i].Value)
	}
	h := md5.New()
	h.Write([]byte(fullName))
	for _, v := range values {
		h.Write([]byte(v))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// computeTestCaseID is the parameter-independent identity = md5(fullName),
// matching allure-pytest's testCaseId. Stays constant across parameterised
// iterations of the same test.
func computeTestCaseID(fullName string) string {
	sum := md5.Sum([]byte(fullName))
	return hex.EncodeToString(sum[:])
}

// goroutineID parses the current goroutine ID out of runtime.Stack output.
// This is the canonical pattern used by allure-java's thread label — the
// goroutine ID is the closest analogue to a thread name in Go.
//
// Returns 0 on parse failure (very defensive — runtime.Stack format has
// been stable across all Go versions).
func goroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Format: "goroutine N [status]:\n..."
	const prefix = "goroutine "
	if n < len(prefix) {
		return 0
	}
	b := buf[len(prefix):n]
	end := bytes.IndexByte(b, ' ')
	if end < 0 {
		return 0
	}
	id, err := strconv.ParseInt(string(b[:end]), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// hostName resolves the host label. We cache it once per process — the
// hostname does not change mid-test-run and the syscall is non-trivial on
// some platforms.
var (
	hostOnce sync.Once
	hostVal  string
)

func hostName() string {
	hostOnce.Do(func() {
		if v, err := os.Hostname(); err == nil && v != "" {
			hostVal = v
			return
		}
		hostVal = "unknown"
	})
	return hostVal
}

func (s *scope) applyConfigLabels() {
	push := func(name, value string) {
		if value == "" {
			return
		}
		s.result.Labels = append(s.result.Labels, AllureLabel{Name: name, Value: value})
	}
	// Framework identity — pinned to product (NOT "mockarty" raw, but the
	// canonical SDK identifier so the Allure report attributes the run
	// correctly when results are mixed across go/python/java).
	push(LabelFramework, FrameworkName)
	push(LabelLanguage, "go")
	push(LabelHost, hostName())
	push(LabelThread, strconv.FormatInt(goroutineID(), 10))
	push(LabelSuite, s.cfg.suite)
	push(LabelParentSuite, s.cfg.parentSuite)
	push(LabelSubSuite, s.cfg.subSuite)
	push(LabelFeature, s.cfg.feature)
	push(LabelStory, s.cfg.story)
	push(LabelEpic, s.cfg.epic)
	push(LabelOwner, s.cfg.owner)
	push(LabelPackage, s.cfg.pkg)
	push(LabelTestClass, s.cfg.testClass)
	push(LabelTestMethod, s.cfg.testMethod)
	if s.cfg.severity != "" {
		push(LabelSeverity, string(s.cfg.severity))
	}
	// AS_ID — the author-pinned Allure id when WithAllureID was used
	// (mirrors @allure.id / @AllureId), else a short stable identifier
	// derived from testClass+testMethod for history navigation. Falls back
	// to a fullName hash so parameterised tests still get an identity.
	if s.cfg.allureID != "" {
		push(LabelAllureID, s.cfg.allureID)
	} else if asID := s.computeAllureStableID(); asID != "" {
		push(LabelAllureID, asID)
	}
	// Append user labels, but NEVER emit internal bookkeeping labels (the
	// `_internal.` prefix is reserved for SDK-private config carried on the
	// label slice, e.g. WithIssuePattern). Leaking them pollutes the Allure
	// report with junk metadata rows.
	for _, l := range s.cfg.labels {
		if strings.HasPrefix(l.Name, internalLabelPrefix) {
			continue
		}
		s.result.Labels = append(s.result.Labels, l)
	}
	s.result.Links = append(s.result.Links, s.cfg.links...)
	s.result.Parameters = append(s.result.Parameters, s.cfg.parameters...)
}

// computeAllureStableID builds a short hex digest of testClass+testMethod
// (or fullName as fallback). 16 hex chars is enough collision-resistance
// for a test catalogue and matches allure-java's AS_ID length.
func (s *scope) computeAllureStableID() string {
	key := s.cfg.testClass + "::" + s.cfg.testMethod
	if s.cfg.testClass == "" && s.cfg.testMethod == "" {
		key = s.cfg.fullName
		if key == "" {
			key = s.cfg.name
		}
	}
	if key == "" {
		return ""
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:8])
}

// FrameworkName is the canonical framework label value emitted by this
// SDK. Stays stable across releases — used by Allure to group runs by
// originating tool.
const FrameworkName = "mockarty-go-sdk"

// scopeKey is the context.Context key for the current scope.
type scopeKey struct{}

// withScope returns a child context carrying the scope.
func withScope(ctx context.Context, s *scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// fromContext extracts the current scope or returns nil if absent.
func fromContext(ctx context.Context) *scope {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(scopeKey{}).(*scope)
	return v
}

// addLabel appends a label under the scope lock.
func (s *scope) addLabel(name, value string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.Labels = append(s.result.Labels, AllureLabel{Name: name, Value: value})
}

func (s *scope) addLink(l AllureLink) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.Links = append(s.result.Links, l)
}

func (s *scope) addParameter(name, value string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Parameters are scoped to the current step if one is open; otherwise
	// they live at the result level (matches Allure semantics).
	if top := s.currentStepLocked(); top != nil {
		top.Parameters = append(top.Parameters, AllureParameter{Name: name, Value: value})
		return
	}
	s.result.Parameters = append(s.result.Parameters, AllureParameter{Name: name, Value: value})
}

func (s *scope) setDescription(desc string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.Description = desc
}

func (s *scope) setTitle(title string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result.Name = title
}

// currentStepLocked returns the innermost open step or nil if we are at the
// test root. Caller must hold s.mu.
func (s *scope) currentStepLocked() *AllureStep {
	if len(s.stepStack) == 0 {
		return nil
	}
	return s.stepStack[len(s.stepStack)-1]
}

// pushStep opens a step and returns the freshly-allocated pointer.
// The pointer is stable until the matching popStep call.
//
// Important: parent step slices are *append-only* until the matching pop —
// we keep pointers into the slice's backing array, so any code that would
// re-slice or shrink an ancestor's Steps slice mid-life would invalidate
// these pointers. The internal API never does that.
func (s *scope) pushStep(name string) *AllureStep {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	step := AllureStep{
		Name:        name,
		Status:      StatusPassed,
		Stage:       StageRunning,
		Start:       s.now().UnixMilli(),
		Parameters:  []AllureParameter{},
		Steps:       []AllureStep{},
		Attachments: []AllureAttachment{},
	}
	// First step push flips the test from scheduled → running.
	if s.result.Stage == StageScheduled {
		s.result.Stage = StageRunning
	}
	parent := s.currentStepLocked()
	if parent != nil {
		parent.Steps = append(parent.Steps, step)
		s.stepStack = append(s.stepStack, &parent.Steps[len(parent.Steps)-1])
	} else {
		s.result.Steps = append(s.result.Steps, step)
		s.stepStack = append(s.stepStack, &s.result.Steps[len(s.result.Steps)-1])
	}
	return s.stepStack[len(s.stepStack)-1]
}

// popStep closes the step opened by the matching pushStep. If status is
// non-empty it overrides the current step status (used to mark failures).
func (s *scope) popStep(status Status, detail *StatusDetail) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stepStack) == 0 {
		return
	}
	idx := len(s.stepStack) - 1
	cur := s.stepStack[idx]
	cur.Stop = s.now().UnixMilli()
	cur.Stage = StageFinished
	if status != "" {
		cur.Status = status
	}
	if detail != nil {
		cur.StatusDetails = detail
	}
	s.stepStack = s.stepStack[:idx]

	// Bubble step failure into the result status. Allure displays the
	// strongest failure across nested steps; we follow this priority:
	// passed < skipped < failed < broken. Higher always wins.
	if statusPriority(cur.Status) > statusPriority(s.result.Status) {
		s.result.Status = cur.Status
		if detail != nil && s.result.StatusDetails == nil {
			s.result.StatusDetails = detail
		}
	}
}

// statusPriority is the ordering used to choose the dominant outcome when
// multiple steps report different statuses.
func statusPriority(s Status) int {
	switch s {
	case StatusBroken:
		return 3
	case StatusFailed:
		return 2
	case StatusSkipped:
		return 1
	default:
		return 0
	}
}

// addAttachment registers an attachment against the current step (or the
// result if no step is open). The bytes are stored alongside the JSON files
// when the scope is flushed.
func (s *scope) addAttachment(name, mime string, content []byte) AllureAttachment {
	if s == nil {
		return AllureAttachment{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	att := AllureAttachment{
		Name:   name,
		Source: uuid.NewString() + "-attachment" + extForMime(mime),
		Type:   mime,
	}
	// Stash the bytes on the writer so flush() writes one file per attachment.
	s.writer.RegisterAttachment(att.Source, content)

	if top := s.currentStepLocked(); top != nil {
		top.Attachments = append(top.Attachments, att)
		return att
	}
	s.result.Attachments = append(s.result.Attachments, att)
	return att
}

// markFailure converts the current scope state to failed/broken.
func (s *scope) markFailure(status Status, message, trace string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if status == "" {
		status = StatusFailed
	}
	s.result.Status = status
	if s.result.StatusDetails == nil {
		s.result.StatusDetails = &StatusDetail{Message: message, Trace: trace}
		return
	}
	if message != "" {
		s.result.StatusDetails.Message = message
	}
	if trace != "" {
		s.result.StatusDetails.Trace = trace
	}
}

// finish flushes the scope to disk. Safe to call multiple times — subsequent
// calls are no-ops.
func (s *scope) finish() error {
	if s == nil {
		return nil
	}
	var flushErr error
	s.finishOnce.Do(func() {
		s.mu.Lock()
		s.result.Stop = s.now().UnixMilli()
		s.result.Stage = StageFinished
		// Any step left open is closed at scope.finish so flush is idempotent.
		for len(s.stepStack) > 0 {
			idx := len(s.stepStack) - 1
			s.stepStack[idx].Stop = s.result.Stop
			s.stepStack[idx].Stage = StageFinished
			s.stepStack = s.stepStack[:idx]
		}
		out := s.result
		writer := s.writer
		s.mu.Unlock()
		flushErr = writer.WriteResult(out)
	})
	return flushErr
}
