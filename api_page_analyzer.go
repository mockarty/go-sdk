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

type PageAnalyzerAPI struct{ client *Client }

type PageAnalyzerOptions struct {
	UserAgent       string `json:"userAgent,omitempty"`
	Timeout         int    `json:"timeout"`
	MaxRedirects    int    `json:"maxRedirects"`
	MaxResources    int    `json:"maxResources"`
	FollowRedirects bool   `json:"followRedirects"`
	CheckResources  bool   `json:"checkResources"`
}

type PageAnalyzerConfigRequest struct {
	AuthConfig map[string]string    `json:"authConfig,omitempty"`
	Options    *PageAnalyzerOptions `json:"options,omitempty"`
	Name       string               `json:"name"`
	TargetURL  string               `json:"targetUrl"`
	AuthType   string               `json:"authType,omitempty"`
}

// PageAnalyzerConfigUpdateRequest preserves the endpoint's write-only fields:
// nil omits/preserves a value, while a pointer to an empty map explicitly
// clears authConfig. A nil TargetURL preserves the exact stored execution URL.
type PageAnalyzerConfigUpdateRequest struct {
	AuthConfig *map[string]string   `json:"authConfig,omitempty"`
	Options    *PageAnalyzerOptions `json:"options,omitempty"`
	TargetURL  *string              `json:"targetUrl,omitempty"`
	Name       string               `json:"name,omitempty"`
	AuthType   string               `json:"authType,omitempty"`
}

type PageAnalyzerConfig struct {
	CreatedAt  time.Time           `json:"createdAt"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	AuthConfig map[string]string   `json:"authConfig"`
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	TargetURL  string              `json:"targetUrl"`
	Namespace  string              `json:"namespace"`
	UserID     string              `json:"userId"`
	AuthType   string              `json:"authType"`
	Options    PageAnalyzerOptions `json:"options"`
}

type PageAnalyzerConfigList struct {
	Configs []PageAnalyzerConfig `json:"configs"`
}

type PageAnalyzerRunRequest struct {
	AuthConfig map[string]string    `json:"authConfig,omitempty"`
	Options    *PageAnalyzerOptions `json:"options,omitempty"`
	ConfigID   string               `json:"configId,omitempty"`
	TargetURL  string               `json:"targetUrl,omitempty"`
	AuthType   string               `json:"authType,omitempty"`
}

type PageAnalyzerRunResponse struct {
	ResultID string `json:"resultId"`
	Status   string `json:"status"`
	Mode     string `json:"mode"`
	Message  string `json:"message"`
}

type PageAnalyzerResult struct {
	CreatedAt    time.Time              `json:"createdAt"`
	Headers      map[string]interface{} `json:"headers"`
	Resources    map[string]interface{} `json:"resources"`
	DOM          map[string]interface{} `json:"domAnalysis"`
	Timing       map[string]interface{} `json:"timing"`
	ErrorMessage string                 `json:"errorMessage,omitempty"`
	ID           string                 `json:"id"`
	Namespace    string                 `json:"namespace"`
	TargetURL    string                 `json:"targetUrl"`
	Status       string                 `json:"status"`
	Mode         string                 `json:"mode"`
	AIAnalysis   string                 `json:"aiAnalysis,omitempty"`
	UserID       string                 `json:"userId"`
	ConfigID     string                 `json:"configId"`
	Score        float64                `json:"score"`
	DurationMs   int64                  `json:"durationMs"`
}

type PageAnalyzerResultList struct {
	Results []PageAnalyzerResult `json:"results"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
}

type PageAnalyzerAIRequest struct {
	ProfileID     string `json:"profileId,omitempty"`
	SelectedModel string `json:"selectedModel,omitempty"`
}

type PageAnalyzerAIResponse struct {
	Analysis string `json:"analysis"`
}

func (a *PageAnalyzerAPI) path(suffix string, query url.Values) string {
	path := "/api/v1/page-analyzer"
	if suffix != "" {
		path += "/" + suffix
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
	return path
}

func pageAnalyzerID(id, kind string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("mockarty: page analyzer %s id is required", kind)
	}
	return url.PathEscape(id), nil
}

func (a *PageAnalyzerAPI) ListConfigs(ctx context.Context) (*PageAnalyzerConfigList, error) {
	var result PageAnalyzerConfigList
	err := a.client.do(ctx, http.MethodGet, a.path("configs", nil), nil, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) SaveConfig(ctx context.Context, request PageAnalyzerConfigRequest) (*PageAnalyzerConfig, error) {
	var result PageAnalyzerConfig
	err := a.client.do(ctx, http.MethodPost, a.path("configs", nil), request, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) UpdateConfig(ctx context.Context, id string, request PageAnalyzerConfigUpdateRequest) (*PageAnalyzerConfig, error) {
	escaped, err := pageAnalyzerID(id, "config")
	if err != nil {
		return nil, err
	}
	var result PageAnalyzerConfig
	err = a.client.do(ctx, http.MethodPut, a.path("configs/"+escaped, nil), request, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) DeleteConfig(ctx context.Context, id string) error {
	escaped, err := pageAnalyzerID(id, "config")
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, a.path("configs/"+escaped, nil), nil, nil)
}

func (a *PageAnalyzerAPI) Run(ctx context.Context, request PageAnalyzerRunRequest) (*PageAnalyzerRunResponse, error) {
	if strings.TrimSpace(request.ConfigID) == "" && strings.TrimSpace(request.TargetURL) == "" {
		return nil, fmt.Errorf("mockarty: page analyzer target URL or config id is required")
	}
	var result PageAnalyzerRunResponse
	err := a.client.do(ctx, http.MethodPost, a.path("run", nil), request, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) ListResults(ctx context.Context, limit, offset int) (*PageAnalyzerResultList, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		query.Set("offset", strconv.Itoa(offset))
	}
	var result PageAnalyzerResultList
	err := a.client.do(ctx, http.MethodGet, a.path("results", query), nil, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) GetResult(ctx context.Context, id string) (*PageAnalyzerResult, error) {
	escaped, err := pageAnalyzerID(id, "result")
	if err != nil {
		return nil, err
	}
	var result PageAnalyzerResult
	err = a.client.do(ctx, http.MethodGet, a.path("results/"+escaped, nil), nil, &result)
	return &result, err
}

func (a *PageAnalyzerAPI) DeleteResult(ctx context.Context, id string) error {
	escaped, err := pageAnalyzerID(id, "result")
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, a.path("results/"+escaped, nil), nil, nil)
}

func (a *PageAnalyzerAPI) AnalyzeWithAI(ctx context.Context, id string, request PageAnalyzerAIRequest) (*PageAnalyzerAIResponse, error) {
	escaped, err := pageAnalyzerID(id, "result")
	if err != nil {
		return nil, err
	}
	var result PageAnalyzerAIResponse
	err = a.client.do(ctx, http.MethodPost, a.path("results/"+escaped+"/ai-analyze", nil), request, &result)
	return &result, err
}
