// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import "encoding/json"

// UIAction is one recorded UI step in a UITest. It mirrors the server's
// uirecorder.RecordedAction wire shape so a UITest built here (or emitted by
// `mockarty-cli ui export --lang go` from a recording) round-trips through
// POST /api/v1/ui-tests and executes on the platform's browser-runner /
// companion — no Playwright/Appium toolchain in the SDK.
type UIAction struct {
	Type         string            `json:"type"`
	Selector     string            `json:"selector,omitempty"`
	SelectorKind string            `json:"selectorKind,omitempty"`
	Value        string            `json:"value,omitempty"`
	Extras       map[string]string `json:"extras,omitempty"`
}

// UITest is a fluent builder for a recorded UI test. Author a flow in code
// (or generate it from a Chrome-extension / companion recording) and submit it
// with client.UITests().Create — the actions run on the platform runner and the
// result flows to TCM, unified with API/perf tests.
//
//	ui := mockarty.NewUITest("checkout").
//	    Navigate("https://shop.example.com").
//	    Click("[data-testid=cart]").
//	    Fill("#coupon", "SAVE10").Press("#coupon", "Enter").
//	    AssertText(".total", "$90.00").
//	    AssertVisible(".confirmation").
//	    Screenshot()
type UITest struct {
	name     string
	platform string
	startURL string
	actions  []UIAction
}

// NewUITest starts a web UI test with the given name.
func NewUITest(name string) *UITest {
	return &UITest{name: name, platform: "web"}
}

// Platform sets the target platform ("web", "android", "ios"). Default "web".
func (t *UITest) Platform(p string) *UITest { t.platform = p; return t }

// StartURL sets an explicit start URL (otherwise the first Navigate wins).
func (t *UITest) StartURL(u string) *UITest { t.startURL = u; return t }

func (t *UITest) add(a UIAction) *UITest { t.actions = append(t.actions, a); return t }

// SelectorKind overrides the locator strategy of the last-added action
// ("css" (default), "testid", "role", "text", "xpath"). Emitted by codegen when
// a recording resolved a non-CSS locator so the generated test keeps fidelity.
func (t *UITest) SelectorKind(kind string) *UITest {
	if n := len(t.actions); n > 0 {
		t.actions[n-1].SelectorKind = kind
	}
	return t
}

// Extra attaches a key/value to the last-added action (e.g. targetSelector for
// dragAndDrop, attr for assertAttribute, files for upload).
func (t *UITest) Extra(key, value string) *UITest {
	if n := len(t.actions); n > 0 {
		if t.actions[n-1].Extras == nil {
			t.actions[n-1].Extras = map[string]string{}
		}
		t.actions[n-1].Extras[key] = value
	}
	return t
}

// --- navigation ---------------------------------------------------------

func (t *UITest) Navigate(url string) *UITest { return t.add(UIAction{Type: "navigate", Value: url}) }
func (t *UITest) GoBack() *UITest             { return t.add(UIAction{Type: "goBack"}) }
func (t *UITest) GoForward() *UITest          { return t.add(UIAction{Type: "goForward"}) }
func (t *UITest) Reload() *UITest             { return t.add(UIAction{Type: "reload"}) }

// --- interactions -------------------------------------------------------

func (t *UITest) Click(sel string) *UITest { return t.add(UIAction{Type: "click", Selector: sel}) }
func (t *UITest) DoubleClick(sel string) *UITest {
	return t.add(UIAction{Type: "dblclick", Selector: sel})
}
func (t *UITest) RightClick(sel string) *UITest {
	return t.add(UIAction{Type: "rightclick", Selector: sel})
}
func (t *UITest) Hover(sel string) *UITest   { return t.add(UIAction{Type: "hover", Selector: sel}) }
func (t *UITest) Focus(sel string) *UITest   { return t.add(UIAction{Type: "focus", Selector: sel}) }
func (t *UITest) Check(sel string) *UITest   { return t.add(UIAction{Type: "check", Selector: sel}) }
func (t *UITest) Uncheck(sel string) *UITest { return t.add(UIAction{Type: "uncheck", Selector: sel}) }
func (t *UITest) Clear(sel string) *UITest   { return t.add(UIAction{Type: "clear", Selector: sel}) }
func (t *UITest) ScrollIntoView(sel string) *UITest {
	return t.add(UIAction{Type: "scrollIntoView", Selector: sel})
}

// Fill sets an input's value in one shot; Type presses keys one by one.
func (t *UITest) Fill(sel, value string) *UITest {
	return t.add(UIAction{Type: "fill", Selector: sel, Value: value})
}
func (t *UITest) Type(sel, value string) *UITest {
	return t.add(UIAction{Type: "type", Selector: sel, Value: value})
}
func (t *UITest) Press(sel, key string) *UITest {
	return t.add(UIAction{Type: "press", Selector: sel, Value: key})
}
func (t *UITest) Select(sel, value string) *UITest {
	return t.add(UIAction{Type: "select", Selector: sel, Value: value})
}
func (t *UITest) Upload(sel, path string) *UITest {
	return t.add(UIAction{Type: "setInputFiles", Selector: sel, Value: path})
}
func (t *UITest) DragAndDrop(sel, targetSel string) *UITest {
	return t.add(UIAction{Type: "dragAndDrop", Selector: sel, Extras: map[string]string{"targetSelector": targetSel}})
}

// --- assertions ---------------------------------------------------------

func (t *UITest) AssertVisible(sel string) *UITest {
	return t.add(UIAction{Type: "assertVisible", Selector: sel})
}
func (t *UITest) AssertHidden(sel string) *UITest {
	return t.add(UIAction{Type: "assertHidden", Selector: sel})
}
func (t *UITest) AssertEnabled(sel string) *UITest {
	return t.add(UIAction{Type: "assertEnabled", Selector: sel})
}
func (t *UITest) AssertDisabled(sel string) *UITest {
	return t.add(UIAction{Type: "assertDisabled", Selector: sel})
}
func (t *UITest) AssertChecked(sel string) *UITest {
	return t.add(UIAction{Type: "assertChecked", Selector: sel})
}
func (t *UITest) AssertText(sel, text string) *UITest {
	return t.add(UIAction{Type: "assertText", Selector: sel, Value: text})
}
func (t *UITest) AssertValue(sel, value string) *UITest {
	return t.add(UIAction{Type: "assertValue", Selector: sel, Value: value})
}
func (t *UITest) AssertCount(sel string, n int) *UITest {
	return t.add(UIAction{Type: "assertCount", Selector: sel, Value: itoa(n)})
}
func (t *UITest) AssertAttribute(sel, attr, value string) *UITest {
	return t.add(UIAction{Type: "assertAttribute", Selector: sel, Value: value, Extras: map[string]string{"attr": attr}})
}
func (t *UITest) AssertURL(substr string) *UITest {
	return t.add(UIAction{Type: "assertURL", Value: substr})
}
func (t *UITest) AssertTitle(substr string) *UITest {
	return t.add(UIAction{Type: "assertTitle", Value: substr})
}

// --- misc ---------------------------------------------------------------

func (t *UITest) WaitFor(sel string) *UITest { return t.add(UIAction{Type: "waitFor", Selector: sel}) }
func (t *UITest) Screenshot() *UITest        { return t.add(UIAction{Type: "screenshot"}) }
func (t *UITest) VisualCheck(sel string) *UITest {
	return t.add(UIAction{Type: "visualCheck", Selector: sel})
}
func (t *UITest) A11yCheck() *UITest { return t.add(UIAction{Type: "a11yCheck"}) }

// Action appends an arbitrary action type (escape hatch for anything the
// typed helpers don't cover).
func (t *UITest) Action(actionType, selector, value string) *UITest {
	return t.add(UIAction{Type: actionType, Selector: selector, Value: value})
}

// --- output -------------------------------------------------------------

// Name returns the test name.
func (t *UITest) Name() string { return t.name }

// Actions returns the built action list (copy-safe for the caller).
func (t *UITest) Actions() []UIAction {
	out := make([]UIAction, len(t.actions))
	copy(out, t.actions)
	return out
}

// wirePayload builds the create-request wire shape
// ({name, platform, startUrl, actions[]}) for POST /api/v1/ui-tests.
func (t *UITest) wirePayload() map[string]any {
	start := t.startURL
	if start == "" {
		for _, a := range t.actions {
			if a.Type == "navigate" && a.Value != "" {
				start = a.Value
				break
			}
		}
	}
	return map[string]any{
		"name":     t.name,
		"platform": t.platform,
		"startUrl": start,
		"actions":  t.actions,
	}
}

// ToJSON serializes the UITest to the create-request wire shape for
// POST /api/v1/ui-tests.
func (t *UITest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(t.wirePayload(), "", "  ")
}

// itoa is a tiny strconv.Itoa without pulling the import into hot builder code.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
