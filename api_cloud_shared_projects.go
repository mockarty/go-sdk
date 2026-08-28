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
	path, err := cloudSharedProjectPath(spaceID, "")
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, path, map[string]any{"name": name, "body": body}, &out)
	return &out, err
}

func (a *CloudSharedProjectsAPI) Update(ctx context.Context, spaceID string, project CloudSharedProject) (*CloudSharedProject, error) {
	path, err := cloudSharedProjectPath(spaceID, project.ID)
	if err != nil {
		return nil, err
	}
	var out CloudSharedProject
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPut, path, map[string]any{"name": project.Name, "body": project.Body, "revision": project.Revision}, &out)
	return &out, err
}

func (a *CloudSharedProjectsAPI) Delete(ctx context.Context, spaceID, projectID string, revision int64) error {
	path, err := cloudSharedProjectPath(spaceID, projectID)
	if err != nil {
		return err
	}
	if revision < 1 {
		return fmt.Errorf("mockarty: revision must be positive")
	}
	return a.client.do(a.client.cloudContext(ctx), http.MethodDelete, path+"?revision="+strconv.FormatInt(revision, 10), nil, nil)
}
