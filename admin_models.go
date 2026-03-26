// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

// NamespaceUser represents a user's membership in a namespace.
type NamespaceUser struct {
	UserID   string `json:"userId,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"` // owner, editor, viewer
}

// AddNamespaceUserRequest defines the payload for adding a user to a namespace.
type AddNamespaceUserRequest struct {
	UserID string `json:"userId"`
	Role   string `json:"role"` // owner, editor, viewer
}

// CleanupPolicy defines retention and auto-cleanup settings for a namespace.
type CleanupPolicy struct {
	MockRetentionDays    int  `json:"mockRetentionDays,omitempty"`
	LogRetentionDays     int  `json:"logRetentionDays,omitempty"`
	TestRunRetentionDays int  `json:"testRunRetentionDays,omitempty"`
	AutoCleanup          bool `json:"autoCleanup,omitempty"`
}
