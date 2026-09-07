package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type ConnectionAuthorityAPI struct{ client *Client }

type ConnectionSecretRef struct {
	StoreID string `json:"storeId"`
	Key     string `json:"key"`
	Version int32  `json:"version,omitempty"`
}

type ConnectionTargetResource struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

type ConnectionTargetPolicy struct {
	Schemes      []string                   `json:"schemes"`
	Hosts        []string                   `json:"hosts"`
	PathPrefixes []string                   `json:"pathPrefixes,omitempty"`
	Clusters     []string                   `json:"clusters,omitempty"`
	Contexts     []string                   `json:"contexts,omitempty"`
	Spaces       []string                   `json:"spaces,omitempty"`
	Projects     []string                   `json:"projects,omitempty"`
	Resources    []ConnectionTargetResource `json:"resources,omitempty"`
	Methods      []string                   `json:"methods,omitempty"`
	Ports        []uint16                   `json:"ports"`
}

type ConnectionDescriptor struct {
	ConfiguredBindings  map[string]json.RawMessage `json:"configuredBindings,omitempty"`
	Kind                string                     `json:"kind"`
	ContractVersion     string                     `json:"contractVersion"`
	ID                  string                     `json:"id"`
	Namespace           string                     `json:"namespace,omitempty"`
	Endpoint            string                     `json:"endpoint"`
	Residency           string                     `json:"residency,omitempty"`
	TargetPolicy        ConnectionTargetPolicy     `json:"targetPolicy"`
	SecretRefs          []ConnectionSecretRef      `json:"secretRefs,omitempty"`
	AllowedOperationIDs []string                   `json:"allowedOperationIds"`
	DataClasses         []string                   `json:"dataClasses,omitempty"`
	SettingsSchema      json.RawMessage            `json:"settingsSchema,omitempty"`
	Revision            int64                      `json:"revision,omitempty"`
}

type ConnectionSnapshot struct {
	Digest     string               `json:"digest"`
	Descriptor ConnectionDescriptor `json:"descriptor"`
	Frozen     bool                 `json:"frozen"`
}

func (a *ConnectionAuthorityAPI) path(namespace, id string) (string, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = strings.TrimSpace(a.client.namespace)
	}
	if namespace == "" || namespace == "*" {
		return "", fmt.Errorf("mockarty: a concrete namespace is required")
	}
	path := "/api/v1/namespaces/" + url.PathEscape(namespace) + "/connections"
	if id != "" {
		path += "/" + url.PathEscape(id)
	}
	return path, nil
}

func (a *ConnectionAuthorityAPI) Create(ctx context.Context, descriptor ConnectionDescriptor) (*ConnectionSnapshot, error) {
	path, err := a.path(descriptor.Namespace, "")
	if err != nil {
		return nil, err
	}
	descriptor.Namespace, descriptor.Revision = "", 0
	var result ConnectionSnapshot
	err = a.client.do(ctx, http.MethodPost, path, descriptor, &result)
	return &result, err
}

func (a *ConnectionAuthorityAPI) GetCurrent(ctx context.Context, namespace, id string) (*ConnectionSnapshot, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("mockarty: connection id is required")
	}
	path, err := a.path(namespace, id)
	if err != nil {
		return nil, err
	}
	var result ConnectionSnapshot
	err = a.client.do(ctx, http.MethodGet, path, nil, &result)
	return &result, err
}

func (a *ConnectionAuthorityAPI) Advance(ctx context.Context, descriptor ConnectionDescriptor, expectedRevision int64) (*ConnectionSnapshot, error) {
	if expectedRevision <= 0 || strings.TrimSpace(descriptor.ID) == "" {
		return nil, fmt.Errorf("mockarty: connection id and positive expected revision are required")
	}
	path, err := a.path(descriptor.Namespace, descriptor.ID)
	if err != nil {
		return nil, err
	}
	descriptor.Namespace, descriptor.ID, descriptor.Revision = "", "", 0
	path += "?expectedRevision=" + strconv.FormatInt(expectedRevision, 10)
	var result ConnectionSnapshot
	err = a.client.do(ctx, http.MethodPut, path, descriptor, &result)
	return &result, err
}

func (a *ConnectionAuthorityAPI) Revoke(ctx context.Context, namespace, id string, revision int64) error {
	if strings.TrimSpace(id) == "" || revision <= 0 {
		return fmt.Errorf("mockarty: connection id and positive revision are required")
	}
	path, err := a.path(namespace, id)
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, path+"?revision="+strconv.FormatInt(revision, 10), nil, nil)
}
