// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

// User represents a platform user.
type User struct {
	ID        string `json:"id,omitempty"`
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role,omitempty"` // admin, support, user
	Enabled   bool   `json:"enabled,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// CreateUserRequest defines the payload for creating a new user.
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role,omitempty"`
}

// UpdateUserRequest defines the payload for updating an existing user.
type UpdateUserRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
}

// AdminNamespace represents a namespace in the admin context.
type AdminNamespace struct {
	Name      string `json:"name"`
	MockCount int    `json:"mockCount,omitempty"`
	UserCount int    `json:"userCount,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

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

// BackupConfig represents a backup schedule configuration.
type BackupConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Schedule  string `json:"schedule,omitempty"`  // cron expression
	Retention int    `json:"retention,omitempty"` // days
	Enabled   bool   `json:"enabled,omitempty"`
}

// Backup represents a single backup instance.
type Backup struct {
	ID        string `json:"id,omitempty"`
	ConfigID  string `json:"configId,omitempty"`
	Status    string `json:"status,omitempty"` // completed, failed, in_progress
	Size      int64  `json:"size,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// LicenseStatus represents the current license status.
type LicenseStatus struct {
	Active    bool   `json:"active"`
	Type      string `json:"type,omitempty"` // trial, standard, enterprise
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	MaxUsers  int    `json:"maxUsers,omitempty"`
	MaxMocks  int    `json:"maxMocks,omitempty"`
}

// License represents a license key and its metadata.
type License struct {
	ID        string `json:"id,omitempty"`
	Key       string `json:"key,omitempty"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"` // active, revoked, expired
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// LicenseUsage represents the current resource usage under the license.
type LicenseUsage struct {
	Users      int `json:"users,omitempty"`
	Mocks      int `json:"mocks,omitempty"`
	Namespaces int `json:"namespaces,omitempty"`
}

// LicenseLimits represents the feature limits granted by the active license(s).
type LicenseLimits struct {
	MaxUsers      int  `json:"maxUsers,omitempty"`
	MaxMocks      int  `json:"maxMocks,omitempty"`
	MaxNamespaces int  `json:"maxNamespaces,omitempty"`
	AIEnabled     bool `json:"aiEnabled,omitempty"`
	PerfEnabled   bool `json:"perfEnabled,omitempty"`
	FuzzEnabled   bool `json:"fuzzEnabled,omitempty"`
}

// AdminWebhook represents an admin-configured webhook subscription.
type AdminWebhook struct {
	ID        string   `json:"id,omitempty"`
	URL       string   `json:"url,omitempty"`
	Events    []string `json:"events,omitempty"` // mock.create, mock.update, etc.
	Enabled   bool     `json:"enabled,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
}

// CleanupPolicy defines retention and auto-cleanup settings for a namespace.
type CleanupPolicy struct {
	MockRetentionDays    int  `json:"mockRetentionDays,omitempty"`
	LogRetentionDays     int  `json:"logRetentionDays,omitempty"`
	TestRunRetentionDays int  `json:"testRunRetentionDays,omitempty"`
	AutoCleanup          bool `json:"autoCleanup,omitempty"`
}

// DatabaseHealth represents the health status of the database.
type DatabaseHealth struct {
	Status      string `json:"status,omitempty"` // healthy, degraded, unhealthy
	Size        int64  `json:"size,omitempty"`
	Tables      int    `json:"tables,omitempty"`
	Connections int    `json:"connections,omitempty"`
}
