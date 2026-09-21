package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CloudServiceAccountsAPI manages customer-owned machine principals with a
// human Cloud principal. Mutations also require the server's interactive
// service_account step-up proof and a stable idempotency key.
type CloudServiceAccountsAPI struct{ client *Client }

// CloudServiceAccountAPI is the workload API authenticated by an msa_
// credential. It cannot manage service accounts, members, roles, or billing.
type CloudServiceAccountAPI struct{ client *Client }

type CloudServiceAccount struct {
	CreatedAt        time.Time                       `json:"created_at"`
	UpdatedAt        time.Time                       `json:"updated_at"`
	RevokedAt        *time.Time                      `json:"revoked_at,omitempty"`
	Name             string                          `json:"name"`
	Status           string                          `json:"status"`
	ID               string                          `json:"id"`
	BillingAccountID string                          `json:"billing_account_id"`
	CreatedByUserID  string                          `json:"created_by_user_id"`
	Bindings         []CloudServiceAccountBinding    `json:"bindings"`
	Credentials      []CloudServiceAccountCredential `json:"credentials"`
}

type CloudServiceAccountCapacity struct {
	Used      int `json:"used"`
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
}

type CloudServiceAccountBinding struct {
	BoundAt      time.Time `json:"bound_at"`
	SpaceID      string    `json:"space_id"`
	SpaceName    string    `json:"space_name"`
	Capabilities []string  `json:"capabilities"`
}

type CloudServiceAccountCredential struct {
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	ID            string     `json:"id"`
	TokenPrefix   string     `json:"token_prefix"`
	Status        string     `json:"status"`
	RotatedFromID *string    `json:"rotated_from_id,omitempty"`
	NeverUsed     bool       `json:"never_used"`
}

type CloudServiceAccountCredentialIssued struct {
	CloudServiceAccountCredential
	Secret string `json:"secret"`
}

type CloudServiceAccountPage struct {
	ServiceAccounts         []CloudServiceAccount       `json:"service_accounts"`
	Capacity                CloudServiceAccountCapacity `json:"capacity"`
	DelegatableCapabilities []string                    `json:"delegatable_capabilities"`
	AuthorityCapabilities   []string                    `json:"authority_capabilities"`
	RequestID               string                      `json:"request_id"`
}

type CloudServiceAccountCreateRequest struct {
	Name                 string   `json:"name"`
	Capabilities         []string `json:"capabilities"`
	CredentialTTLSeconds int64    `json:"credential_ttl_seconds"`
	IssueCredential      bool     `json:"issue_credential"`
}

type CloudServiceAccountCreateResult struct {
	ServiceAccount CloudServiceAccount                  `json:"service_account"`
	Credential     *CloudServiceAccountCredentialIssued `json:"credential,omitempty"`
}

type CloudServiceAccountSelf struct {
	ServiceAccount struct {
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
		ID               string    `json:"id"`
		Name             string    `json:"name"`
		Status           string    `json:"status"`
		BillingAccountID string    `json:"billing_account_id"`
	} `json:"service_account"`
	Credential              *CloudServiceAccountCredential `json:"credential"`
	Bindings                []CloudServiceAccountBinding   `json:"bindings"`
	Spaces                  []string                       `json:"spaces"`
	Capabilities            map[string][]string            `json:"capabilities"`
	DelegatableCapabilities []string                       `json:"delegatable_capabilities"`
	AuthorityCapabilities   []string                       `json:"authority_capabilities"`
	CredentialKind          string                         `json:"credential_kind"`
	RequestID               string                         `json:"request_id"`
	GeneralToken            bool                           `json:"general_token"`
}

type CloudServiceAccountAuthorization struct {
	SpaceID    string   `json:"space_id"`
	Capability string   `json:"capability"`
	Granted    []string `json:"granted"`
	RequestID  string   `json:"request_id"`
	Allowed    bool     `json:"allowed"`
}

func cloudServiceAccountsPath(spaceID string, segments ...string) (string, error) {
	if strings.TrimSpace(spaceID) == "" {
		return "", fmt.Errorf("mockarty: space id is required")
	}
	path := "/api/v1/cloud/spaces/" + url.PathEscape(spaceID) + "/service-accounts"
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return "", fmt.Errorf("mockarty: service-account path segment is required")
		}
		path += "/" + url.PathEscape(segment)
	}
	return path, nil
}

func (a *CloudServiceAccountsAPI) mutationContext(ctx context.Context, idempotencyKey string) (context.Context, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	return a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": key}), nil
}

func (a *CloudServiceAccountsAPI) List(ctx context.Context, spaceID string) (*CloudServiceAccountPage, error) {
	path, err := cloudServiceAccountsPath(spaceID)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountPage
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudServiceAccountsAPI) Get(ctx context.Context, spaceID, accountID string) (*CloudServiceAccount, error) {
	path, err := cloudServiceAccountsPath(spaceID, accountID)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccount
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudServiceAccountsAPI) Create(ctx context.Context, spaceID string, request CloudServiceAccountCreateRequest, idempotencyKey string) (*CloudServiceAccountCreateResult, error) {
	path, err := cloudServiceAccountsPath(spaceID)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountCreateResult
	err = a.client.do(ctx, http.MethodPost, path, request, &out)
	return &out, err
}

func (a *CloudServiceAccountsAPI) Bind(ctx context.Context, spaceID, accountID, targetSpaceID string, capabilities []string, idempotencyKey string) (*CloudServiceAccountBinding, error) {
	path, err := cloudServiceAccountsPath(spaceID, accountID, "bindings")
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountBinding
	err = a.client.do(ctx, http.MethodPost, path, map[string]any{"space_id": targetSpaceID, "capabilities": capabilities}, &out)
	return &out, err
}

func (a *CloudServiceAccountsAPI) IssueCredential(ctx context.Context, spaceID, accountID string, ttlSeconds int64, idempotencyKey string) (*CloudServiceAccountCredentialIssued, error) {
	return a.issueCredential(ctx, spaceID, accountID, ttlSeconds, idempotencyKey, false)
}

func (a *CloudServiceAccountsAPI) RotateCredential(ctx context.Context, spaceID, accountID string, ttlSeconds int64, idempotencyKey string) (*CloudServiceAccountCredentialIssued, error) {
	return a.issueCredential(ctx, spaceID, accountID, ttlSeconds, idempotencyKey, true)
}

func (a *CloudServiceAccountsAPI) issueCredential(ctx context.Context, spaceID, accountID string, ttlSeconds int64, idempotencyKey string, rotate bool) (*CloudServiceAccountCredentialIssued, error) {
	if ttlSeconds < 0 {
		return nil, fmt.Errorf("mockarty: ttl seconds must not be negative")
	}
	segments := []string{accountID, "credentials"}
	if rotate {
		segments = append(segments, "rotate")
	}
	path, err := cloudServiceAccountsPath(spaceID, segments...)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountCredentialIssued
	err = a.client.do(ctx, http.MethodPost, path, map[string]any{"ttl_seconds": ttlSeconds}, &out)
	return &out, err
}

func (a *CloudServiceAccountsAPI) Revoke(ctx context.Context, spaceID, accountID, idempotencyKey string) (*CloudServiceAccount, error) {
	path, err := cloudServiceAccountsPath(spaceID, accountID)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccount
	err = a.client.do(ctx, http.MethodDelete, path, nil, &out)
	return &out, err
}

func cloudServiceAccountProjectPath(spaceID, projectID string) (string, error) {
	if strings.TrimSpace(spaceID) == "" {
		return "", fmt.Errorf("mockarty: space id is required")
	}
	path := "/api/v1/cloud/service-account/spaces/" + url.PathEscape(spaceID) + "/shared/projects"
	if projectID != "" {
		path += "/" + url.PathEscape(projectID)
	}
	return path, nil
}

func (a *CloudServiceAccountAPI) Self(ctx context.Context) (*CloudServiceAccountSelf, error) {
	var out CloudServiceAccountSelf
	err := a.client.do(a.client.cloudContext(ctx), http.MethodGet, "/api/v1/cloud/service-account/self", nil, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) Authorize(ctx context.Context, spaceID, capability string) (*CloudServiceAccountAuthorization, error) {
	if strings.TrimSpace(spaceID) == "" || strings.TrimSpace(capability) == "" {
		return nil, fmt.Errorf("mockarty: space id and capability are required")
	}
	path := "/api/v1/cloud/service-account/spaces/" + url.PathEscape(spaceID) + "/authorize?capability=" + url.QueryEscape(capability)
	var out CloudServiceAccountAuthorization
	err := a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) ListProjects(ctx context.Context, spaceID, cursor string, limit int) (*CloudSharedProjectPage, error) {
	path, err := cloudServiceAccountProjectPath(spaceID, "")
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	var out CloudSharedProjectPage
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path+"?"+query.Encode(), nil, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) GetProject(ctx context.Context, spaceID, projectID string) (*CloudSharedProject, error) {
	path, err := cloudServiceAccountProjectPath(spaceID, projectID)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) mutationContext(ctx context.Context, idempotencyKey string) (context.Context, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	return a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": key}), nil
}

func (a *CloudServiceAccountAPI) CreateProject(ctx context.Context, spaceID, name string, body json.RawMessage, idempotencyKey string) (*CloudSharedProject, error) {
	path, err := cloudServiceAccountProjectPath(spaceID, "")
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(ctx, http.MethodPost, path, map[string]any{"name": name, "body": body}, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) UpdateProject(ctx context.Context, spaceID string, project CloudSharedProject, idempotencyKey string) (*CloudSharedProject, error) {
	path, err := cloudServiceAccountProjectPath(spaceID, project.ID)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(ctx, http.MethodPut, path, map[string]any{"name": project.Name, "body": project.Body, "revision": project.Revision}, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) DeleteProject(ctx context.Context, spaceID, projectID string, revision int64, idempotencyKey string) error {
	if revision < 1 {
		return fmt.Errorf("mockarty: revision must be positive")
	}
	path, err := cloudServiceAccountProjectPath(spaceID, projectID)
	if err != nil {
		return err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, path+"?revision="+strconv.FormatInt(revision, 10), nil, nil)
}

// CloudServiceAccountRunRequest dispatches one bounded Shared run. OperationID
// is deliberately absent: Cloud derives it from Idempotency-Key and rejects a
// second, conflicting identity in the JSON body.
type CloudServiceAccountRunRequest struct {
	Deadline   time.Time       `json:"deadline"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Capability string          `json:"capability"`
	Class      string          `json:"class"`
	ProjectID  string          `json:"project_id"`
}

type CloudServiceAccountRun struct {
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Deadline    time.Time       `json:"deadline"`
	Result      json.RawMessage `json:"result,omitempty"`
	ID          string          `json:"id"`
	OperationID string          `json:"operation_id"`
	Capability  string          `json:"capability"`
	Class       string          `json:"class"`
	State       string          `json:"state"`
	ErrorCode   string          `json:"error_code,omitempty"`
	Revision    int64           `json:"revision"`
}

func cloudServiceAccountRunPath(spaceID, runID string, cancel bool) (string, error) {
	if strings.TrimSpace(spaceID) == "" {
		return "", fmt.Errorf("mockarty: space id is required")
	}
	path := "/api/v1/cloud/service-account/spaces/" + url.PathEscape(spaceID) + "/shared/runs"
	if runID != "" {
		path += "/" + url.PathEscape(runID)
	}
	if cancel {
		if runID == "" {
			return "", fmt.Errorf("mockarty: run id is required")
		}
		path += "/cancel"
	}
	return path, nil
}

func (a *CloudServiceAccountAPI) DispatchRun(ctx context.Context, spaceID string, request CloudServiceAccountRunRequest, idempotencyKey string) (*CloudServiceAccountRun, error) {
	path, err := cloudServiceAccountRunPath(spaceID, "", false)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountRun
	err = a.client.do(ctx, http.MethodPost, path, request, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) GetRun(ctx context.Context, spaceID, runID string) (*CloudServiceAccountRun, error) {
	path, err := cloudServiceAccountRunPath(spaceID, runID, false)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountRun
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudServiceAccountAPI) CancelRun(ctx context.Context, spaceID, runID string, revision int64, idempotencyKey string) (*CloudServiceAccountRun, error) {
	if revision < 1 {
		return nil, fmt.Errorf("mockarty: revision must be positive")
	}
	path, err := cloudServiceAccountRunPath(spaceID, runID, true)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var out CloudServiceAccountRun
	err = a.client.do(ctx, http.MethodPost, path, map[string]any{"revision": revision}, &out)
	return &out, err
}
