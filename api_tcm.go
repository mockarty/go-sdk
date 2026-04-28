// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TCMAPI groups the Test Case Management endpoints — folders, cases
// (catalog / versions / rollback), attachments, shared steps. Runtime
// (start / resolve / pause / resume) is layered on top once the case
// runner is wired.
type TCMAPI struct {
	client *Client
}

// TCM returns the TCM API binding.
func (c *Client) TCM() *TCMAPI { return &TCMAPI{client: c} }

// ---------------------------------------------------------------------------
// Folders
// ---------------------------------------------------------------------------

// TCMFolder mirrors internal/tcm/folders.Folder (public JSON shape).
type TCMFolder struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ParentID    *string   `json:"parentId,omitempty"`
	ID          string    `json:"id"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Path        string    `json:"path"`
	Icon        string    `json:"icon,omitempty"`
	Color       string    `json:"color,omitempty"`
	Depth       int32     `json:"depth"`
	SortOrder   int32     `json:"sortOrder"`
	CasesCount  int32     `json:"casesCount"`
}

// TCMFolderTreeNode is the nested tree representation.
type TCMFolderTreeNode struct {
	ParentID   *string             `json:"parentId,omitempty"`
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Path       string              `json:"path"`
	Icon       string              `json:"icon,omitempty"`
	Color      string              `json:"color,omitempty"`
	Children   []TCMFolderTreeNode `json:"children,omitempty"`
	Depth      int32               `json:"depth"`
	SortOrder  int32               `json:"sortOrder"`
	CasesCount int32               `json:"casesCount"`
}

// TCMFolderCreateRequest carries the minimum fields for Create.
type TCMFolderCreateRequest struct {
	ParentID    string `json:"parentId,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
	SortOrder   int32  `json:"sortOrder,omitempty"`
}

// TCMFolderUpdateRequest carries mutable fields (PATCH semantics).
type TCMFolderUpdateRequest struct {
	SortOrder   *int32 `json:"sortOrder,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
}

func tcmFolderPath(ns string) string {
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/tcm/folders"
}

// GetFolderTree returns the full namespace tree snapshot.
func (a *TCMAPI) GetFolderTree(ctx context.Context, namespace string) ([]TCMFolderTreeNode, error) {
	var resp struct {
		Items []TCMFolderTreeNode `json:"items"`
	}
	err := a.client.do(ctx, http.MethodGet, tcmFolderPath(namespace)+"/tree", nil, &resp)
	return resp.Items, err
}

// ListFolders returns folders flattened.
func (a *TCMAPI) ListFolders(ctx context.Context, namespace string) ([]TCMFolder, error) {
	var resp struct {
		Items []TCMFolder `json:"items"`
	}
	err := a.client.do(ctx, http.MethodGet, tcmFolderPath(namespace), nil, &resp)
	return resp.Items, err
}

// CreateFolder inserts a new folder under the given namespace.
func (a *TCMAPI) CreateFolder(ctx context.Context, namespace string, req TCMFolderCreateRequest) (*TCMFolder, error) {
	var out TCMFolder
	err := a.client.do(ctx, http.MethodPost, tcmFolderPath(namespace), req, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFolder PATCHes mutable fields.
func (a *TCMAPI) UpdateFolder(ctx context.Context, namespace, id string, req TCMFolderUpdateRequest) (*TCMFolder, error) {
	var out TCMFolder
	path := tcmFolderPath(namespace) + "/" + url.PathEscape(id)
	if err := a.client.do(ctx, http.MethodPatch, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteFolder soft-deletes the folder (restore via Trash within 7 days).
func (a *TCMAPI) DeleteFolder(ctx context.Context, namespace, id string) error {
	return a.client.do(ctx, http.MethodDelete,
		tcmFolderPath(namespace)+"/"+url.PathEscape(id), nil, nil)
}

// MoveFolder reparents the folder under a new parent.
func (a *TCMAPI) MoveFolder(ctx context.Context, namespace, id, newParentID string) error {
	path := tcmFolderPath(namespace) + "/" + url.PathEscape(id) + "/move"
	return a.client.do(ctx, http.MethodPost, path, map[string]string{"toParentId": newParentID}, nil)
}

// ---------------------------------------------------------------------------
// Attachments
// ---------------------------------------------------------------------------

// TCMAttachment mirrors internal/tcm/attachments.Attachment.
type TCMAttachment struct {
	CreatedAt      time.Time `json:"createdAt"`
	ID             string    `json:"id"`
	Namespace      string    `json:"namespace"`
	ParentKind     string    `json:"parentKind"`
	ParentID       string    `json:"parentId"`
	StorageURI     string    `json:"storageUri,omitempty"`
	ThumbnailURI   string    `json:"thumbnailUri,omitempty"`
	MediaType      string    `json:"mediaType"`
	OriginalName   string    `json:"originalName"`
	ChecksumSHA256 string    `json:"checksumSha256"`
	CreatedBy      string    `json:"createdBy"`
	SizeBytes      int64     `json:"sizeBytes"`
	WidthPx        int32     `json:"widthPx,omitempty"`
	HeightPx       int32     `json:"heightPx,omitempty"`
}

// UploadAttachment streams body into the attachments endpoint and returns
// the persisted metadata. multipart boundary is synthesised in-memory —
// upload sizes are bounded by the server (25 MiB default), so single-shot
// is safe.
func (a *TCMAPI) UploadAttachment(ctx context.Context, namespace, parentKind, parentID, filename, mediaType string, body io.Reader) (*TCMAttachment, error) {
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	boundary := fmt.Sprintf("mockarty-%x", time.Now().UnixNano())
	var mp bytes.Buffer
	mp.WriteString("--" + boundary + "\r\n")
	mp.WriteString(`Content-Disposition: form-data; name="file"; filename="` + filename + "\"\r\n")
	mp.WriteString("Content-Type: " + mediaType + "\r\n\r\n")
	mp.Write(buf)
	mp.WriteString("\r\n--" + boundary + "--\r\n")

	path := fmt.Sprintf("/api/v1/namespaces/%s/tcm/attachments/upload?parentKind=%s&parentId=%s",
		url.PathEscape(namespace), url.QueryEscape(parentKind), url.QueryEscape(parentID))

	reader, err := a.client.doRaw(ctx, http.MethodPost, path, mp.Bytes())
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var out TCMAttachment
	if err := json.NewDecoder(reader).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAttachments returns attachments attached to a (parentKind, parentID).
func (a *TCMAPI) ListAttachments(ctx context.Context, namespace, parentKind, parentID string) ([]TCMAttachment, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/tcm/attachments?parentKind=%s&parentId=%s",
		url.PathEscape(namespace), url.QueryEscape(parentKind), url.QueryEscape(parentID))
	var resp struct {
		Items []TCMAttachment `json:"items"`
	}
	err := a.client.do(ctx, http.MethodGet, path, nil, &resp)
	return resp.Items, err
}

// DeleteAttachment soft-deletes the row and schedules the blob for removal.
func (a *TCMAPI) DeleteAttachment(ctx context.Context, namespace, id string) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/tcm/attachments/%s",
		url.PathEscape(namespace), url.PathEscape(id))
	return a.client.do(ctx, http.MethodDelete, path, nil, nil)
}

// DownloadAttachment streams the raw blob. Caller must Close the returned
// reader.
func (a *TCMAPI) DownloadAttachment(ctx context.Context, namespace, id string) (io.ReadCloser, error) {
	path := fmt.Sprintf("/api/v1/namespaces/%s/tcm/attachments/%s/raw",
		url.PathEscape(namespace), url.PathEscape(id))
	return a.client.doRaw(ctx, http.MethodGet, path, nil)
}
