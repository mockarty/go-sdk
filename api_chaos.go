// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ChaosAPI provides methods for chaos engineering experiments.
type ChaosAPI struct {
	client *Client
}

// ---------------------------------------------------------------------------
// Profiles (Cluster Connections)
// ---------------------------------------------------------------------------

// ListProfiles returns all chaos cluster connection profiles.
// GET /api/v1/chaos/profiles
func (a *ChaosAPI) ListProfiles(ctx context.Context) ([]ChaosProfile, error) {
	var resp chaosProfileListResponse
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/profiles", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Profiles, nil
}

// CreateProfile creates a new cluster connection profile.
// POST /api/v1/chaos/profiles
func (a *ChaosAPI) CreateProfile(ctx context.Context, profile *ChaosProfile) (*ChaosProfile, error) {
	var result ChaosProfile
	if err := a.client.do(ctx, "POST", "/api/v1/chaos/profiles", profile, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateProfile updates an existing cluster connection profile.
// PUT /api/v1/chaos/profiles/:id
func (a *ChaosAPI) UpdateProfile(ctx context.Context, id string, profile *ChaosProfile) (*ChaosProfile, error) {
	var result ChaosProfile
	if err := a.client.do(ctx, "PUT", "/api/v1/chaos/profiles/"+url.PathEscape(id), profile, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteProfile deletes a cluster connection profile by ID.
// DELETE /api/v1/chaos/profiles/:id
func (a *ChaosAPI) DeleteProfile(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/chaos/profiles/"+url.PathEscape(id), nil, nil)
}

// TestProfile tests connectivity for an existing cluster profile.
// POST /api/v1/chaos/profiles/:id/test
func (a *ChaosAPI) TestProfile(ctx context.Context, id string) (*ChaosConnectionResult, error) {
	var result ChaosConnectionResult
	if err := a.client.do(ctx, "POST", "/api/v1/chaos/profiles/"+url.PathEscape(id)+"/test", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ConnectProfile connects to a cluster using the given profile.
// POST /api/v1/chaos/profiles/:id/connect
func (a *ChaosAPI) ConnectProfile(ctx context.Context, id string) (*ChaosConnectionResult, error) {
	var result ChaosConnectionResult
	if err := a.client.do(ctx, "POST", "/api/v1/chaos/profiles/"+url.PathEscape(id)+"/connect", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TestInlineKubeconfig tests connectivity using an inline kubeconfig and optional context name.
// POST /api/v1/chaos/profiles-test
func (a *ChaosAPI) TestInlineKubeconfig(ctx context.Context, kubeconfig string, kubectx string) (*ChaosConnectionResult, error) {
	body := struct {
		Kubeconfig string `json:"kubeconfig"`
		Context    string `json:"context,omitempty"`
	}{Kubeconfig: kubeconfig, Context: kubectx}

	var result ChaosConnectionResult
	if err := a.client.do(ctx, "POST", "/api/v1/chaos/profiles-test", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Presets
// ---------------------------------------------------------------------------

// ListPresets returns all available chaos experiment presets.
// GET /api/v1/chaos/presets
func (a *ChaosAPI) ListPresets(ctx context.Context) ([]ChaosPreset, error) {
	var resp chaosPresetListResponse
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/presets", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Presets, nil
}

// ---------------------------------------------------------------------------
// Experiments (CRUD)
// ---------------------------------------------------------------------------

// List returns chaos experiments, optionally filtered by the given options.
// GET /api/v1/chaos/experiments
func (a *ChaosAPI) List(ctx context.Context, opts *ChaosListOptions) ([]ChaosExperiment, int, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Namespace != "" {
			params.Set("namespace", opts.Namespace)
		}
		if opts.Status != "" {
			params.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
	}

	path := "/api/v1/chaos/experiments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp chaosExperimentListResponse
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Experiments, resp.Total, nil
}

// Get retrieves a single chaos experiment by ID.
// GET /api/v1/chaos/experiments/:id
func (a *ChaosAPI) Get(ctx context.Context, id string) (*ChaosExperiment, error) {
	var experiment ChaosExperiment
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id), nil, &experiment); err != nil {
		return nil, err
	}
	return &experiment, nil
}

// Create creates a new chaos experiment.
// POST /api/v1/chaos/experiments
func (a *ChaosAPI) Create(ctx context.Context, experiment *ChaosExperiment) (*ChaosExperiment, error) {
	var result ChaosExperiment
	if err := a.client.do(ctx, "POST", "/api/v1/chaos/experiments", experiment, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update updates an existing chaos experiment.
// PUT /api/v1/chaos/experiments/:id
func (a *ChaosAPI) Update(ctx context.Context, id string, experiment *ChaosExperiment) (*ChaosExperiment, error) {
	var result ChaosExperiment
	if err := a.client.do(ctx, "PUT", "/api/v1/chaos/experiments/"+url.PathEscape(id), experiment, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes a chaos experiment by ID.
// DELETE /api/v1/chaos/experiments/:id
func (a *ChaosAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/chaos/experiments/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Experiment Execution
// ---------------------------------------------------------------------------

// Run starts execution of a chaos experiment.
// POST /api/v1/chaos/experiments/:id/run
func (a *ChaosAPI) Run(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/run", nil, nil)
}

// Abort aborts a running chaos experiment.
// POST /api/v1/chaos/experiments/:id/abort
func (a *ChaosAPI) Abort(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/abort", nil, nil)
}

// ---------------------------------------------------------------------------
// Experiment Results & Metrics
// ---------------------------------------------------------------------------

// GetMetrics retrieves metric snapshots collected during an experiment.
// GET /api/v1/chaos/experiments/:id/metrics
func (a *ChaosAPI) GetMetrics(ctx context.Context, id string) ([]MetricsSnapshot, error) {
	var resp chaosMetricsResponse
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/metrics", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Snapshots, nil
}

// GetEvents retrieves the timeline events for an experiment.
// GET /api/v1/chaos/experiments/:id/events
func (a *ChaosAPI) GetEvents(ctx context.Context, id string) ([]TimelineEvent, error) {
	var resp chaosEventsResponse
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/events", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Events, nil
}

// GetReport retrieves a full experiment report with analytics.
// GET /api/v1/chaos/experiments/:id/report
func (a *ChaosAPI) GetReport(ctx context.Context, id string) (map[string]any, error) {
	var report map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/report", nil, &report); err != nil {
		return nil, err
	}
	return report, nil
}

// DownloadReport downloads an experiment report in the specified format.
// Supported formats: html, json, junit, allure.
// GET /api/v1/chaos/experiments/:id/report/download?format=...
func (a *ChaosAPI) DownloadReport(ctx context.Context, id string, format string) ([]byte, error) {
	params := url.Values{}
	params.Set("format", format)

	data, err := a.client.doJSON(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/report/download?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetSnapshot retrieves a point-in-time resource snapshot for the experiment.
// GET /api/v1/chaos/experiments/:id/snapshot
func (a *ChaosAPI) GetSnapshot(ctx context.Context, id string) (map[string]any, error) {
	var snapshot map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/experiments/"+url.PathEscape(id)+"/snapshot", nil, &snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

// GetQueueStatus returns the experiment queue status for a cluster.
// GET /api/v1/chaos/queue/:clusterId
func (a *ChaosAPI) GetQueueStatus(ctx context.Context, clusterID string) (map[string]any, error) {
	var status map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/queue/"+url.PathEscape(clusterID), nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

// ---------------------------------------------------------------------------
// Cluster Operations (ad-hoc)
// ---------------------------------------------------------------------------

// GetTopology retrieves the cluster topology for a profile and optional namespace.
// GET /api/v1/chaos/clusters/:id/topology?namespace=...
func (a *ChaosAPI) GetTopology(ctx context.Context, profileID string, namespace string) (map[string]any, error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}

	path := "/api/v1/chaos/clusters/" + url.PathEscape(profileID) + "/topology"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var topology map[string]any
	if err := a.client.do(ctx, "GET", path, nil, &topology); err != nil {
		return nil, err
	}
	return topology, nil
}

// KillPod kills a pod in the given namespace. A gracePeriod of 0 means immediate termination.
// DELETE /api/v1/chaos/pods/:namespace/:name?gracePeriod=...
func (a *ChaosAPI) KillPod(ctx context.Context, namespace string, name string, gracePeriod int) error {
	params := url.Values{}
	if gracePeriod > 0 {
		params.Set("gracePeriod", strconv.Itoa(gracePeriod))
	}

	path := fmt.Sprintf("/api/v1/chaos/pods/%s/%s", url.PathEscape(namespace), url.PathEscape(name))
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	return a.client.do(ctx, "DELETE", path, nil, nil)
}

// GetPodDetail retrieves detailed information about a specific pod.
// GET /api/v1/chaos/pods/:namespace/:name
func (a *ChaosAPI) GetPodDetail(ctx context.Context, namespace string, name string) (map[string]any, error) {
	var detail map[string]any
	path := fmt.Sprintf("/api/v1/chaos/pods/%s/%s", url.PathEscape(namespace), url.PathEscape(name))
	if err := a.client.do(ctx, "GET", path, nil, &detail); err != nil {
		return nil, err
	}
	return detail, nil
}

// GetPodLogs retrieves logs from a specific pod container.
// GET /api/v1/chaos/pods/:namespace/:name/logs?container=...&tailLines=...
func (a *ChaosAPI) GetPodLogs(ctx context.Context, namespace string, name string, container string, tailLines int) (string, error) {
	params := url.Values{}
	if container != "" {
		params.Set("container", container)
	}
	if tailLines > 0 {
		params.Set("tailLines", strconv.Itoa(tailLines))
	}

	path := fmt.Sprintf("/api/v1/chaos/pods/%s/%s/logs", url.PathEscape(namespace), url.PathEscape(name))
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	data, err := a.client.doJSON(ctx, "GET", path, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetDeploymentDetail retrieves detailed information about a specific deployment.
// GET /api/v1/chaos/deployments/:namespace/:name
func (a *ChaosAPI) GetDeploymentDetail(ctx context.Context, namespace string, name string) (map[string]any, error) {
	var detail map[string]any
	path := fmt.Sprintf("/api/v1/chaos/deployments/%s/%s", url.PathEscape(namespace), url.PathEscape(name))
	if err := a.client.do(ctx, "GET", path, nil, &detail); err != nil {
		return nil, err
	}
	return detail, nil
}

// ScaleDeployment scales a deployment to the specified number of replicas.
// POST /api/v1/chaos/deployments/:namespace/:name/scale
func (a *ChaosAPI) ScaleDeployment(ctx context.Context, namespace string, name string, replicas int) (map[string]any, error) {
	body := struct {
		Replicas int `json:"replicas"`
	}{Replicas: replicas}

	var result map[string]any
	path := fmt.Sprintf("/api/v1/chaos/deployments/%s/%s/scale", url.PathEscape(namespace), url.PathEscape(name))
	if err := a.client.do(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// RestartDeployment performs a rolling restart of a deployment.
// POST /api/v1/chaos/deployments/:namespace/:name/restart
func (a *ChaosAPI) RestartDeployment(ctx context.Context, namespace string, name string) error {
	path := fmt.Sprintf("/api/v1/chaos/deployments/%s/%s/restart", url.PathEscape(namespace), url.PathEscape(name))
	return a.client.do(ctx, "POST", path, nil, nil)
}

// ---------------------------------------------------------------------------
// ConfigMaps
// ---------------------------------------------------------------------------

// ListConfigMaps returns all ConfigMaps in the given namespace.
// GET /api/v1/chaos/configmaps/:namespace
func (a *ChaosAPI) ListConfigMaps(ctx context.Context, namespace string) ([]map[string]any, error) {
	var resp struct {
		ConfigMaps []map[string]any `json:"configmaps"`
		Count      int              `json:"count"`
	}
	path := fmt.Sprintf("/api/v1/chaos/configmaps/%s", url.PathEscape(namespace))
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ConfigMaps, nil
}

// GetConfigMap returns details of a specific ConfigMap.
// GET /api/v1/chaos/configmaps/:namespace/:name
func (a *ChaosAPI) GetConfigMap(ctx context.Context, namespace string, name string) (map[string]any, error) {
	var cm map[string]any
	path := fmt.Sprintf("/api/v1/chaos/configmaps/%s/%s", url.PathEscape(namespace), url.PathEscape(name))
	if err := a.client.do(ctx, "GET", path, nil, &cm); err != nil {
		return nil, err
	}
	return cm, nil
}

// UpdateConfigMap replaces the data section of an existing ConfigMap.
// PUT /api/v1/chaos/configmaps/:namespace/:name
func (a *ChaosAPI) UpdateConfigMap(ctx context.Context, namespace string, name string, data map[string]string) error {
	body := struct {
		Data map[string]string `json:"data"`
	}{Data: data}

	path := fmt.Sprintf("/api/v1/chaos/configmaps/%s/%s", url.PathEscape(namespace), url.PathEscape(name))
	return a.client.do(ctx, "PUT", path, body, nil)
}

// ---------------------------------------------------------------------------
// Services
// ---------------------------------------------------------------------------

// ListServices returns all services in the given namespace.
// GET /api/v1/chaos/services/:namespace
func (a *ChaosAPI) ListServices(ctx context.Context, namespace string) ([]map[string]any, error) {
	var resp struct {
		Services []map[string]any `json:"services"`
		Count    int              `json:"count"`
	}
	path := fmt.Sprintf("/api/v1/chaos/services/%s", url.PathEscape(namespace))
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Services, nil
}

// ---------------------------------------------------------------------------
// CRDs (Custom Resource Definitions)
// ---------------------------------------------------------------------------

// ListCRDs returns all Custom Resource Definitions in the cluster.
// GET /api/v1/chaos/crds
func (a *ChaosAPI) ListCRDs(ctx context.Context) ([]map[string]any, error) {
	var resp struct {
		CRDs  []map[string]any `json:"crds"`
		Count int              `json:"count"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/chaos/crds", nil, &resp); err != nil {
		return nil, err
	}
	return resp.CRDs, nil
}

// ListCRDResources returns instances of a specific custom resource.
// GET /api/v1/chaos/crds/:group/:version/:resource?namespace=...
func (a *ChaosAPI) ListCRDResources(ctx context.Context, group, version, resource string, namespace string) ([]map[string]any, error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}

	path := fmt.Sprintf("/api/v1/chaos/crds/%s/%s/%s",
		url.PathEscape(group), url.PathEscape(version), url.PathEscape(resource))
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp struct {
		Resources []map[string]any `json:"resources"`
		Count     int              `json:"count"`
	}
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Resources, nil
}

// ---------------------------------------------------------------------------
// Kubernetes Events
// ---------------------------------------------------------------------------

// ListK8sEvents returns recent Kubernetes events in the given namespace.
// GET /api/v1/chaos/events/:namespace?limit=...
func (a *ChaosAPI) ListK8sEvents(ctx context.Context, namespace string, limit int) ([]map[string]any, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := fmt.Sprintf("/api/v1/chaos/events/%s", url.PathEscape(namespace))
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var resp struct {
		Events []map[string]any `json:"events"`
		Count  int              `json:"count"`
	}
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Events, nil
}

// ---------------------------------------------------------------------------
// Operator
// ---------------------------------------------------------------------------

// GetOperatorStatus returns the chaos operator installation status.
// GET /api/v1/chaos/operator/status?namespace=...
func (a *ChaosAPI) GetOperatorStatus(ctx context.Context, namespace string) (*ChaosOperatorStatus, error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}

	path := "/api/v1/chaos/operator/status"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var status ChaosOperatorStatus
	if err := a.client.do(ctx, "GET", path, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// InstallOperator generates the operator manifest for manual application.
// POST /api/v1/chaos/operator/install?namespace=...
func (a *ChaosAPI) InstallOperator(ctx context.Context, namespace string) (map[string]any, error) {
	params := url.Values{}
	if namespace != "" {
		params.Set("namespace", namespace)
	}

	path := "/api/v1/chaos/operator/install"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result map[string]any
	if err := a.client.do(ctx, "POST", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GenerateOperatorManifest generates a YAML manifest for installing the chaos operator.
// POST /api/v1/chaos/operator/manifest
func (a *ChaosAPI) GenerateOperatorManifest(ctx context.Context, adminURL string, image string) (string, error) {
	body := struct {
		AdminURL string `json:"adminUrl,omitempty"`
		Image    string `json:"image,omitempty"`
	}{AdminURL: adminURL, Image: image}

	data, err := a.client.doJSON(ctx, "POST", "/api/v1/chaos/operator/manifest", body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
