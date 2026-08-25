package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DeliveryPolicyAPI struct{ client *Client }

type DeliveryPolicySettings struct {
	MaintenanceWindow *DeliveryPolicyMaintenanceWindow `json:"maintenanceWindow,omitempty"`
	ApprovalMode      string                           `json:"approvalMode"`
	RollbackMode      string                           `json:"rollbackMode"`
	MaxConcurrent     int                              `json:"maxConcurrent"`
}

type DeliveryPolicyMaintenanceWindow struct {
	StartMinuteUTC int `json:"startMinuteUtc"`
	EndMinuteUTC   int `json:"endMinuteUtc"`
}

type DeliveryPolicyEnvironmentRequest struct {
	Custom     *DeliveryPolicySettings `json:"custom,omitempty"`
	ID         string                  `json:"id,omitempty"`
	ProjectID  string                  `json:"projectId"`
	Class      string                  `json:"class"`
	Profile    string                  `json:"profile"`
	AuditID    string                  `json:"auditId"`
	EvidenceID string                  `json:"evidenceId"`
}

type DeliveryPolicyEnvironment struct {
	CreatedAt             time.Time `json:"createdAt"`
	ETag                  string    `json:"etag"`
	ID                    string    `json:"id"`
	ProjectID             string    `json:"projectId"`
	BodyDigest            string    `json:"bodyDigest"`
	EffectivePolicyDigest string    `json:"effectivePolicyDigest"`
	Namespace             string    `json:"namespace"`
	AuditID               string    `json:"auditId"`
	EvidenceID            string    `json:"evidenceId"`
	CreatedBy             string    `json:"createdBy"`
	Status                string    `json:"status"`
	Class                 string    `json:"class"`
	Profile               string    `json:"profile"`
	Revision              uint64    `json:"revision"`
	RevocationRevision    uint64    `json:"revocationRevision"`
}

type DeliveryPolicyEnvironmentPage struct {
	NextCursor string                      `json:"nextCursor"`
	Items      []DeliveryPolicyEnvironment `json:"items"`
}

func (a *DeliveryPolicyAPI) path(id string, query url.Values) (string, error) {
	path := "/api/v1/admin/delivery-policy/environments"
	if id != "" {
		if strings.TrimSpace(id) == "" {
			return "", fmt.Errorf("mockarty: delivery-policy environment id is required")
		}
		path += "/" + url.PathEscape(id)
	}
	if query == nil {
		query = url.Values{}
	}
	if namespace := strings.TrimSpace(a.client.namespace); namespace != "" {
		query.Set("namespace", namespace)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path, nil
}

func deliveryPolicyHeaders(ctx context.Context, etag, idempotency string) context.Context {
	headers := map[string]string{}
	if strings.TrimSpace(etag) != "" {
		headers["If-Match"] = etag
	}
	if strings.TrimSpace(idempotency) != "" {
		headers["Idempotency-Key"] = idempotency
	}
	return withRequestHeaders(ctx, headers)
}

func (a *DeliveryPolicyAPI) Create(ctx context.Context, request DeliveryPolicyEnvironmentRequest, idempotencyKey string) (*DeliveryPolicyEnvironment, error) {
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: delivery-policy idempotency key is required")
	}
	path, err := a.path("", nil)
	if err != nil {
		return nil, err
	}
	var result DeliveryPolicyEnvironment
	err = a.client.do(deliveryPolicyHeaders(ctx, "", idempotencyKey), http.MethodPost, path, request, &result)
	return &result, err
}

func (a *DeliveryPolicyAPI) Get(ctx context.Context, id string) (*DeliveryPolicyEnvironment, error) {
	path, err := a.path(id, nil)
	if err != nil {
		return nil, err
	}
	var result DeliveryPolicyEnvironment
	err = a.client.do(ctx, http.MethodGet, path, nil, &result)
	return &result, err
}

func (a *DeliveryPolicyAPI) List(ctx context.Context, status, cursor string, limit int) (*DeliveryPolicyEnvironmentPage, error) {
	query := url.Values{}
	if status != "" {
		query.Set("status", status)
	}
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	path, err := a.path("", query)
	if err != nil {
		return nil, err
	}
	var result DeliveryPolicyEnvironmentPage
	err = a.client.do(ctx, http.MethodGet, path, nil, &result)
	return &result, err
}

func (a *DeliveryPolicyAPI) Advance(ctx context.Context, id string, request DeliveryPolicyEnvironmentRequest, etag, idempotencyKey string) (*DeliveryPolicyEnvironment, error) {
	path, err := a.path(id, nil)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(etag) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: delivery-policy ETag and idempotency key are required")
	}
	request.ID = ""
	var result DeliveryPolicyEnvironment
	err = a.client.do(deliveryPolicyHeaders(ctx, etag, idempotencyKey), http.MethodPut, path, request, &result)
	return &result, err
}

func (a *DeliveryPolicyAPI) Revoke(ctx context.Context, id, etag string) error {
	path, err := a.path(id, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(etag) == "" {
		return fmt.Errorf("mockarty: delivery-policy ETag is required")
	}
	return a.client.do(deliveryPolicyHeaders(ctx, etag, ""), http.MethodDelete, path, nil, nil)
}
