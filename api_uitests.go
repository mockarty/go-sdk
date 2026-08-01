// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// UITestAPI drives recorded UI tests on the platform: save a UITest (authored
// with NewUITest(...) or generated from a recording), run it on a browser-runner
// / companion, and poll the result. The SDK orchestrates — it never embeds a
// browser (that lives in the runner), matching the perf/functional pattern.
type UITestAPI struct {
	client *Client
}

// UITestInfo is a saved UI test as the server lists/returns it.
type UITestInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Platform     string `json:"platform,omitempty"`
	StartURL     string `json:"startUrl,omitempty"`
	SelectorsStr string `json:"selectorsStrategy,omitempty"`
	ActionCount  int    `json:"actionCount,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// UITestRunOptions are optional per-run overrides. A zero value replays the
// recording exactly as captured.
type UITestRunOptions struct {
	Browser        string            `json:"browser,omitempty"`
	Viewport       string            `json:"viewport,omitempty"`
	StorageStateID string            `json:"storageStateId,omitempty"`
	EnvVars        map[string]string `json:"envVars,omitempty"`
	Platform       string            `json:"platform,omitempty"`
	ExistingAppID  string            `json:"existingAppId,omitempty"`
	AppArtifactID  string            `json:"appArtifactId,omitempty"`
	ScreenshotMode string            `json:"screenshotMode,omitempty"`
	RunnerID       string            `json:"runnerId,omitempty"`
	DeviceLeaseID  string            `json:"deviceLeaseId,omitempty"`
}

// UITestRunHandle is returned by Run — the taskId identifies the dispatched
// replay; poll it with RunStatus / WaitForRun.
type UITestRunHandle struct {
	TaskID     string `json:"taskId"`
	UITestID   string `json:"uiTestId"`
	Name       string `json:"name"`
	Actions    int    `json:"actions"`
	StatusPath string `json:"statusPath"`
	SignalPath string `json:"signalPath,omitempty"`
}

// UITestRunStatus is one runner-task row for a UI-test replay. Status is the
// overall verdict; per-step outcomes live under ResultData.extras.steps.
type UITestRunStatus struct {
	ID         string         `json:"id"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	ResultData map[string]any `json:"resultData,omitempty"`
	CreatedAt  string         `json:"createdAt,omitempty"`
	UpdatedAt  string         `json:"updatedAt,omitempty"`
}

// Terminal reports whether the run reached a final state.
func (s *UITestRunStatus) Terminal() bool {
	switch strings.ToLower(s.Status) {
	case "passed", "failed", "broken", "skipped", "cancelled", "error", "completed":
		return true
	}
	return false
}

func (a *UITestAPI) nsQuery() string {
	if a.client.namespace != "" {
		return "?namespace=" + url.QueryEscape(a.client.namespace)
	}
	return ""
}

// Create saves a UITest (POST /api/v1/ui-tests) and returns its stored info.
func (a *UITestAPI) Create(ctx context.Context, t *UITest) (*UITestInfo, error) {
	if t == nil {
		return nil, fmt.Errorf("mockarty: UITests.Create: nil UITest")
	}
	var out UITestInfo
	if err := a.client.do(ctx, "POST", "/api/v1/ui-tests"+a.nsQuery(), t.wirePayload(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the saved UI tests in the client's namespace.
func (a *UITestAPI) List(ctx context.Context) ([]UITestInfo, error) {
	var env struct {
		UITests []UITestInfo `json:"uiTests"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/ui-tests"+a.nsQuery(), nil, &env); err != nil {
		return nil, err
	}
	return env.UITests, nil
}

// Get retrieves a saved UI test by id.
func (a *UITestAPI) Get(ctx context.Context, id string) (*UITestInfo, error) {
	var out UITestInfo
	if err := a.client.do(ctx, "GET", "/api/v1/ui-tests/"+url.PathEscape(id)+a.nsQuery(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Run dispatches a replay on a capability-matched runner and returns a handle.
// opts may be nil (replay as captured).
func (a *UITestAPI) Run(ctx context.Context, id string, opts *UITestRunOptions) (*UITestRunHandle, error) {
	body := opts
	if body == nil {
		body = &UITestRunOptions{}
	}
	var out UITestRunHandle
	if err := a.client.do(ctx, "POST", "/api/v1/ui-tests/"+url.PathEscape(id)+"/run"+a.nsQuery(), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RunStatus reads the current status of a dispatched replay (GET
// /api/v1/runner-tasks/:taskId).
func (a *UITestAPI) RunStatus(ctx context.Context, taskID string) (*UITestRunStatus, error) {
	var out UITestRunStatus
	if err := a.client.do(ctx, "GET", "/api/v1/runner-tasks/"+url.PathEscape(taskID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WaitForRun polls RunStatus until the run is terminal or ctx is done. interval
// defaults to 2s when non-positive.
func (a *UITestAPI) WaitForRun(ctx context.Context, taskID string, interval time.Duration) (*UITestRunStatus, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		st, err := a.RunStatus(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if st.Terminal() {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return st, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// Export returns the recording rendered as source in the given language
// ("go", "python", "java", "playwright", "appium").
func (a *UITestAPI) Export(ctx context.Context, id, lang string) (string, error) {
	q := url.Values{}
	q.Set("format", lang)
	if a.client.namespace != "" {
		q.Set("namespace", a.client.namespace)
	}
	rc, err := a.client.doRaw(ctx, "GET", "/api/v1/ui-tests/"+url.PathEscape(id)+"/export?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, rerr := rc.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return sb.String(), nil
}
