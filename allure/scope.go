// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

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
		Stage:       StageFinished,
		Start:       s.startNS,
		Labels:      []AllureLabel{},
		Links:       []AllureLink{},
		Parameters:  []AllureParameter{},
		Steps:       []AllureStep{},
		Attachments: []AllureAttachment{},
	}
	// Stable historyId — derived from fullName so re-runs cluster in Allure's
	// history view. uuid.NewSHA1 with a stable namespace gives us 32 hex chars.
	hid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fullName))
	s.result.HistoryID = hid.String()
	s.result.TestCaseID = hid.String()

	s.applyConfigLabels()
	return s
}

func (s *scope) applyConfigLabels() {
	push := func(name, value string) {
		if value == "" {
			return
		}
		s.result.Labels = append(s.result.Labels, AllureLabel{Name: name, Value: value})
	}
	push(LabelFramework, "mockarty")
	push(LabelLanguage, "go")
	push(LabelSuite, s.cfg.suite)
	push(LabelFeature, s.cfg.feature)
	push(LabelStory, s.cfg.story)
	push(LabelEpic, s.cfg.epic)
	push(LabelOwner, s.cfg.owner)
	if s.cfg.severity != "" {
		push(LabelSeverity, string(s.cfg.severity))
	}
	s.result.Labels = append(s.result.Labels, s.cfg.labels...)
	s.result.Links = append(s.result.Links, s.cfg.links...)
	s.result.Parameters = append(s.result.Parameters, s.cfg.parameters...)
}

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
		Stage:       StageFinished,
		Start:       s.now().UnixMilli(),
		Parameters:  []AllureParameter{},
		Steps:       []AllureStep{},
		Attachments: []AllureAttachment{},
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
		// Any step left open is closed at scope.finish so flush is idempotent.
		for len(s.stepStack) > 0 {
			idx := len(s.stepStack) - 1
			s.stepStack[idx].Stop = s.result.Stop
			s.stepStack = s.stepStack[:idx]
		}
		out := s.result
		writer := s.writer
		s.mu.Unlock()
		flushErr = writer.WriteResult(out)
	})
	return flushErr
}
