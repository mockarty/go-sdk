package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type MediaDeliveryAPI struct{ client *Client }

type FencedMediaDelivery struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	Namespace string    `json:"namespace"`
	RunnerID  string    `json:"runnerId"`
	Error     string    `json:"error,omitempty"`
	Attempts  int       `json:"attempts"`
}

type FencedMediaDeliveryPage struct {
	Fenced []FencedMediaDelivery `json:"fenced"`
	Count  int                   `json:"count"`
}

type MediaDeliveryReconcileRequest struct {
	RunnerID string `json:"runnerId"`
	Outcome  string `json:"outcome"`
}

func (a *MediaDeliveryAPI) path(engine, jobID string) (string, error) {
	switch engine {
	case "transcribe", "tts":
	default:
		return "", fmt.Errorf("mockarty: media delivery engine must be transcribe or tts")
	}
	path := "/api/v1/" + engine + "/jobs"
	if jobID == "" {
		path += "/fenced"
	} else {
		path += "/" + url.PathEscape(jobID) + "/reconcile-delivery"
	}
	query := url.Values{}
	if namespace := strings.TrimSpace(a.client.namespace); namespace != "" {
		query.Set("namespace", namespace)
	}
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return path, nil
}

func (a *MediaDeliveryAPI) ListFenced(ctx context.Context, engine string) (*FencedMediaDeliveryPage, error) {
	path, err := a.path(engine, "")
	if err != nil {
		return nil, err
	}
	var result FencedMediaDeliveryPage
	err = a.client.do(ctx, http.MethodGet, path, nil, &result)
	return &result, err
}

func (a *MediaDeliveryAPI) Reconcile(ctx context.Context, engine, jobID, runnerID, outcome string) error {
	if strings.TrimSpace(jobID) == "" || strings.TrimSpace(runnerID) == "" {
		return fmt.Errorf("mockarty: media delivery job id and runner id are required")
	}
	if outcome != "not_started" && outcome != "started" {
		return fmt.Errorf("mockarty: media delivery outcome must be not_started or started")
	}
	path, err := a.path(engine, jobID)
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodPost, path, MediaDeliveryReconcileRequest{RunnerID: runnerID, Outcome: outcome}, nil)
}
