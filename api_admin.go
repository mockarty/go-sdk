// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// AdminAPI provides methods for platform administration.
type AdminAPI struct {
	client *Client
}

// ---------------------------------------------------------------------------
// User Management
// ---------------------------------------------------------------------------

// ListUsers returns all platform users.
func (a *AdminAPI) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := a.client.do(ctx, "GET", "/api/v1/admin/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// CreateUser creates a new platform user.
func (a *AdminAPI) CreateUser(ctx context.Context, user *CreateUserRequest) (*User, error) {
	var result User
	if err := a.client.do(ctx, "POST", "/api/v1/admin/users", user, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateUser updates an existing user by ID.
func (a *AdminAPI) UpdateUser(ctx context.Context, id string, user *UpdateUserRequest) (*User, error) {
	var result User
	if err := a.client.do(ctx, "PUT", "/api/v1/admin/users/"+url.PathEscape(id), user, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteUser deletes a user by ID.
func (a *AdminAPI) DeleteUser(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/admin/users/"+url.PathEscape(id), nil, nil)
}

// SetUserPassword sets a new password for a user.
func (a *AdminAPI) SetUserPassword(ctx context.Context, id string, password string) error {
	body := struct {
		Password string `json:"password"`
	}{Password: password}
	return a.client.do(ctx, "POST", "/api/v1/admin/users/"+url.PathEscape(id)+"/password", body, nil)
}

// DisableUser disables a user account.
func (a *AdminAPI) DisableUser(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/admin/users/"+url.PathEscape(id)+"/disable", nil, nil)
}

// EnableUser enables a previously disabled user account.
func (a *AdminAPI) EnableUser(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/admin/users/"+url.PathEscape(id)+"/enable", nil, nil)
}

// ---------------------------------------------------------------------------
// Namespace Management
// ---------------------------------------------------------------------------

// ListNamespaces returns all namespaces with admin metadata.
func (a *AdminAPI) ListNamespaces(ctx context.Context) ([]AdminNamespace, error) {
	var namespaces []AdminNamespace
	if err := a.client.do(ctx, "GET", "/api/v1/admin/namespaces", nil, &namespaces); err != nil {
		return nil, err
	}
	return namespaces, nil
}

// DeleteNamespace deletes a namespace.
func (a *AdminAPI) DeleteNamespace(ctx context.Context, namespace string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/admin/namespaces/"+url.PathEscape(namespace), nil, nil)
}

// RestoreNamespace restores a previously deleted namespace.
func (a *AdminAPI) RestoreNamespace(ctx context.Context, namespace string) error {
	return a.client.do(ctx, "PUT", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/restore", nil, nil)
}

// ListNamespaceUsers returns all users in a namespace.
func (a *AdminAPI) ListNamespaceUsers(ctx context.Context, namespace string) ([]NamespaceUser, error) {
	var users []NamespaceUser
	if err := a.client.do(ctx, "GET", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// AddNamespaceUser adds a user to a namespace with a specific role.
func (a *AdminAPI) AddNamespaceUser(ctx context.Context, namespace string, req *AddNamespaceUserRequest) error {
	return a.client.do(ctx, "POST", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/users", req, nil)
}

// RemoveNamespaceUser removes a user from a namespace.
func (a *AdminAPI) RemoveNamespaceUser(ctx context.Context, namespace string, userID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/users/"+url.PathEscape(userID), nil, nil)
}

// UpdateNamespaceUserRole updates a user's role within a namespace.
func (a *AdminAPI) UpdateNamespaceUserRole(ctx context.Context, namespace string, userID string, role string) error {
	body := struct {
		Role string `json:"role"`
	}{Role: role}
	return a.client.do(ctx, "PUT", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/users/"+url.PathEscape(userID)+"/role", body, nil)
}

// ---------------------------------------------------------------------------
// Backup Management
// ---------------------------------------------------------------------------

// ListBackupConfigs returns all backup configurations.
func (a *AdminAPI) ListBackupConfigs(ctx context.Context) ([]BackupConfig, error) {
	var configs []BackupConfig
	if err := a.client.do(ctx, "GET", "/api/v1/admin/backups/configs", nil, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// GetBackupConfig retrieves a backup configuration by ID.
func (a *AdminAPI) GetBackupConfig(ctx context.Context, id string) (*BackupConfig, error) {
	var config BackupConfig
	if err := a.client.do(ctx, "GET", "/api/v1/admin/backups/configs/"+url.PathEscape(id), nil, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateBackupConfig creates a new backup configuration.
func (a *AdminAPI) CreateBackupConfig(ctx context.Context, config *BackupConfig) (*BackupConfig, error) {
	var result BackupConfig
	if err := a.client.do(ctx, "POST", "/api/v1/admin/backups/configs", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateBackup triggers a backup using the specified config ID.
func (a *AdminAPI) CreateBackup(ctx context.Context, configID string) (*Backup, error) {
	body := struct {
		ConfigID string `json:"configId"`
	}{ConfigID: configID}
	var result Backup
	if err := a.client.do(ctx, "POST", "/api/v1/admin/backups/create", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBackups returns all backups for a given config ID.
func (a *AdminAPI) ListBackups(ctx context.Context, configID string) ([]Backup, error) {
	var backups []Backup
	if err := a.client.do(ctx, "GET", "/api/v1/admin/backups/configs/"+url.PathEscape(configID)+"/backups", nil, &backups); err != nil {
		return nil, err
	}
	return backups, nil
}

// DownloadBackup downloads a backup by ID as raw bytes.
func (a *AdminAPI) DownloadBackup(ctx context.Context, backupID string) ([]byte, error) {
	params := url.Values{}
	params.Set("id", backupID)
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/admin/backups/download?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// RestoreBackup triggers a restore from a backup by ID.
func (a *AdminAPI) RestoreBackup(ctx context.Context, backupID string) error {
	body := struct {
		BackupID string `json:"backupId"`
	}{BackupID: backupID}
	return a.client.do(ctx, "POST", "/api/v1/admin/backups/restore", body, nil)
}

// DeleteBackup deletes a backup by ID.
func (a *AdminAPI) DeleteBackup(ctx context.Context, backupID string) error {
	return a.client.do(ctx, "POST", "/api/v1/admin/backups/"+url.PathEscape(backupID)+"/delete", nil, nil)
}

// ---------------------------------------------------------------------------
// License Management
// ---------------------------------------------------------------------------

// GetLicenseStatus returns the current license status.
func (a *AdminAPI) GetLicenseStatus(ctx context.Context) (*LicenseStatus, error) {
	var status LicenseStatus
	if err := a.client.do(ctx, "GET", "/api/v1/admin/licenses/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// ListLicenses returns all registered licenses.
func (a *AdminAPI) ListLicenses(ctx context.Context) ([]License, error) {
	var licenses []License
	if err := a.client.do(ctx, "GET", "/api/v1/admin/licenses", nil, &licenses); err != nil {
		return nil, err
	}
	return licenses, nil
}

// ActivateLicense activates a license key.
func (a *AdminAPI) ActivateLicense(ctx context.Context, key string) error {
	body := struct {
		Key string `json:"key"`
	}{Key: key}
	return a.client.do(ctx, "POST", "/api/v1/admin/licenses/activate", body, nil)
}

// GetLicenseUsage returns the current resource usage under the license.
func (a *AdminAPI) GetLicenseUsage(ctx context.Context) (*LicenseUsage, error) {
	var usage LicenseUsage
	if err := a.client.do(ctx, "GET", "/api/v1/admin/licenses/usage", nil, &usage); err != nil {
		return nil, err
	}
	return &usage, nil
}

// GetCombinedLimits returns the combined feature limits from all active licenses.
func (a *AdminAPI) GetCombinedLimits(ctx context.Context) (*LicenseLimits, error) {
	var limits LicenseLimits
	if err := a.client.do(ctx, "GET", "/api/v1/admin/licenses/combined-limits", nil, &limits); err != nil {
		return nil, err
	}
	return &limits, nil
}

// ---------------------------------------------------------------------------
// Webhook Management
// ---------------------------------------------------------------------------

// ListWebhooks returns all admin-configured webhooks.
func (a *AdminAPI) ListWebhooks(ctx context.Context) ([]AdminWebhook, error) {
	var webhooks []AdminWebhook
	if err := a.client.do(ctx, "GET", "/api/v1/admin/webhooks", nil, &webhooks); err != nil {
		return nil, err
	}
	return webhooks, nil
}

// CreateWebhook creates a new admin webhook.
func (a *AdminAPI) CreateWebhook(ctx context.Context, webhook *AdminWebhook) (*AdminWebhook, error) {
	var result AdminWebhook
	if err := a.client.do(ctx, "POST", "/api/v1/admin/webhooks", webhook, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteWebhook deletes an admin webhook by ID.
func (a *AdminAPI) DeleteWebhook(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/admin/webhooks/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Audit
// ---------------------------------------------------------------------------

// ExportAuditLogs exports the audit log as raw bytes.
func (a *AdminAPI) ExportAuditLogs(ctx context.Context) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/admin/audit/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Cleanup Policy
// ---------------------------------------------------------------------------

// GetCleanupPolicy retrieves the cleanup policy for a namespace.
func (a *AdminAPI) GetCleanupPolicy(ctx context.Context, namespace string) (*CleanupPolicy, error) {
	var policy CleanupPolicy
	if err := a.client.do(ctx, "GET", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/cleanup-policy", nil, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpdateCleanupPolicy updates the cleanup policy for a namespace.
func (a *AdminAPI) UpdateCleanupPolicy(ctx context.Context, namespace string, policy *CleanupPolicy) error {
	return a.client.do(ctx, "PUT", "/api/v1/admin/namespaces/"+url.PathEscape(namespace)+"/cleanup-policy", policy, nil)
}

// ---------------------------------------------------------------------------
// Database
// ---------------------------------------------------------------------------

// GetDatabaseHealth returns the database health status.
func (a *AdminAPI) GetDatabaseHealth(ctx context.Context) (*DatabaseHealth, error) {
	var health DatabaseHealth
	if err := a.client.do(ctx, "GET", "/api/v1/admin/database/health", nil, &health); err != nil {
		return nil, err
	}
	return &health, nil
}
