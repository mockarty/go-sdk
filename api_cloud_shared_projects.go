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

	"github.com/google/uuid"
)

type CloudSharedProjectsAPI struct{ client *Client }

type CloudSharedProject struct {
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Body      json.RawMessage `json:"body"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Revision  int64           `json:"revision"`
}

type CloudSharedProjectPage struct {
	Projects   []CloudSharedProject `json:"projects"`
	NextCursor string               `json:"next_cursor"`
	HasMore    bool                 `json:"has_more"`
}

func (a *CloudSharedProjectsAPI) mutationContext(ctx context.Context, requestID string) (context.Context, error) {
	parsed, err := uuid.Parse(requestID)
	if err != nil || parsed.String() != requestID {
		return nil, fmt.Errorf("mockarty: request id must be a canonical UUID")
	}
	cloudCtx := a.client.cloudContext(ctx)
	headers := make(map[string]string)
	if existing, ok := cloudCtx.Value(requestHeadersContextKey{}).(map[string]string); ok {
		for name, value := range existing {
			headers[name] = value
		}
	}
	headers[headerRequestID] = requestID
	return withRequestHeaders(cloudCtx, headers), nil
}

func cloudSharedProjectPath(spaceID, projectID string) (string, error) {
	if strings.TrimSpace(spaceID) == "" {
		return "", fmt.Errorf("mockarty: Space id is required")
	}
	path := "/api/v1/cloud/spaces/" + url.PathEscape(spaceID) + "/shared/projects"
	if projectID != "" {
		path += "/" + url.PathEscape(projectID)
	}
	return path, nil
}

func (a *CloudSharedProjectsAPI) List(ctx context.Context, spaceID, cursor string, limit int) (*CloudSharedProjectPage, error) {
	path, err := cloudSharedProjectPath(spaceID, "")
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

func (a *CloudSharedProjectsAPI) Get(ctx context.Context, spaceID, projectID string) (*CloudSharedProject, error) {
	path, err := cloudSharedProjectPath(spaceID, projectID)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return &out, err
}

func (a *CloudSharedProjectsAPI) Create(ctx context.Context, spaceID, name string, body json.RawMessage) (*CloudSharedProject, error) {
	return a.CreateWithRequestID(ctx, spaceID, name, body, uuid.NewString())
}

// CreateWithRequestID creates a project with a stable operation UUID. Reuse
// the same UUID only for the exact same create after an ambiguous response.
func (a *CloudSharedProjectsAPI) CreateWithRequestID(ctx context.Context, spaceID, name string, body json.RawMessage, requestID string) (*CloudSharedProject, error) {
	path, err := cloudSharedProjectPath(spaceID, "")
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, requestID)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(ctx, http.MethodPost, path, map[string]any{"name": name, "body": body}, &out)
	return &out, err
}

func (a *CloudSharedProjectsAPI) Update(ctx context.Context, spaceID string, project CloudSharedProject) (*CloudSharedProject, error) {
	return a.UpdateWithRequestID(ctx, spaceID, project, uuid.NewString())
}

// UpdateWithRequestID updates a project and supplies an audit-correlation UUID.
func (a *CloudSharedProjectsAPI) UpdateWithRequestID(ctx context.Context, spaceID string, project CloudSharedProject, requestID string) (*CloudSharedProject, error) {
	path, err := cloudSharedProjectPath(spaceID, project.ID)
	if err != nil {
		return nil, err
	}
	ctx, err = a.mutationContext(ctx, requestID)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(ctx, http.MethodPut, path, map[string]any{"name": project.Name, "body": project.Body, "revision": project.Revision}, &out)
	return &out, err
}

func (a *CloudSharedProjectsAPI) Delete(ctx context.Context, spaceID, projectID string, revision int64) error {
	return a.DeleteWithRequestID(ctx, spaceID, projectID, revision, uuid.NewString())
}

// DeleteWithRequestID deletes a project and supplies an audit-correlation UUID.
func (a *CloudSharedProjectsAPI) DeleteWithRequestID(ctx context.Context, spaceID, projectID string, revision int64, requestID string) error {
	path, err := cloudSharedProjectPath(spaceID, projectID)
	if err != nil {
		return err
	}
	if revision < 1 {
		return fmt.Errorf("mockarty: revision must be positive")
	}
	ctx, err = a.mutationContext(ctx, requestID)
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, path+"?revision="+strconv.FormatInt(revision, 10), nil, nil)
}
