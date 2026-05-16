// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"os"
	"strings"
)

// Link pattern env-vars match allure-pytest / allure-java semantics. When
// set, callers can pass just the short id (e.g. "JIRA-123") and the SDK
// expands "{0}" / "{}" in the pattern to build the full URL.
//
//   ALLURE_ISSUE_LINK_PATTERN — for Issue()
//   ALLURE_TMS_LINK_PATTERN   — for TmsLink()
//   ALLURE_LINK_PATTERN_PREFIX — wildcard pattern for custom link types
//                                (e.g. ALLURE_LINK_PATTERN_DOCS for type=docs).
const (
	IssuePatternEnv = "ALLURE_ISSUE_LINK_PATTERN"
	TmsPatternEnv   = "ALLURE_TMS_LINK_PATTERN"
	LinkPatternPfx  = "ALLURE_LINK_PATTERN_"
)

// expandLinkPattern resolves a short id into a full URL given a pattern.
// Both "{0}" (allure-pytest) and "{}" (allure-java) placeholders are
// honoured so the same env-var works across SDKs.
func expandLinkPattern(pattern, id string) string {
	if pattern == "" {
		return id
	}
	if strings.Contains(pattern, "{0}") {
		return strings.ReplaceAll(pattern, "{0}", id)
	}
	if strings.Contains(pattern, "{}") {
		return strings.ReplaceAll(pattern, "{}", id)
	}
	return pattern + id
}

// resolveLinkURL picks the URL for a link given its (type, name-or-id, url).
// If url is empty it tries to expand from the env-var pattern for the type.
func resolveLinkURL(linkType, idOrName, url string) string {
	if url != "" {
		return url
	}
	switch linkType {
	case LinkTypeIssue:
		return expandLinkPattern(os.Getenv(IssuePatternEnv), idOrName)
	case LinkTypeTMS:
		return expandLinkPattern(os.Getenv(TmsPatternEnv), idOrName)
	default:
		env := LinkPatternPfx + strings.ToUpper(linkType)
		return expandLinkPattern(os.Getenv(env), idOrName)
	}
}

// IssueLink registers an issue link with optional URL inference from the
// ALLURE_ISSUE_LINK_PATTERN env-var. Pass just the short id ("JIRA-123")
// and the SDK builds the full URL.
func IssueLink(ctx context.Context, id string) {
	s := withScopeOrNoop(ctx)
	if s == nil {
		return
	}
	s.addLink(AllureLink{
		Name: id,
		URL:  resolveLinkURL(LinkTypeIssue, id, ""),
		Type: LinkTypeIssue,
	})
}

// TmsLinkID registers a TMS link with URL inference from
// ALLURE_TMS_LINK_PATTERN.
func TmsLinkID(ctx context.Context, id string) {
	s := withScopeOrNoop(ctx)
	if s == nil {
		return
	}
	s.addLink(AllureLink{
		Name: id,
		URL:  resolveLinkURL(LinkTypeTMS, id, ""),
		Type: LinkTypeTMS,
	})
}

// CustomLink registers a link with an arbitrary type and optional URL
// inference from ALLURE_LINK_PATTERN_<TYPE>.
func CustomLink(ctx context.Context, linkType, id string) {
	s := withScopeOrNoop(ctx)
	if s == nil {
		return
	}
	s.addLink(AllureLink{
		Name: id,
		URL:  resolveLinkURL(linkType, id, ""),
		Type: linkType,
	})
}

// LinkTypeRegistry is the catalogue of recognised link types. We seed it
// with the canonical Allure types; callers may register their own via
// RegisterLinkType for custom badges (e.g. "design", "rfc").
var LinkTypeRegistry = map[string]struct{}{
	LinkTypeIssue:   {},
	LinkTypeTMS:     {},
	LinkTypeGeneric: {},
}

// RegisterLinkType adds a custom link type to the registry so consumers
// can validate their categorisation against the active SDK build.
//
// Returns the canonical type string (lower-cased, trimmed).
func RegisterLinkType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t != "" {
		LinkTypeRegistry[t] = struct{}{}
	}
	return t
}
