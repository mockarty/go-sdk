// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"
	"unicode"
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
	path, err := a.nsBase(namespace, "/tcm/attachments/upload")
	if err != nil {
		return nil, err
	}
	if body == nil || filename == "" || strings.IndexFunc(filename, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("tcm: attachment requires a body and safe filename")
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	if _, _, err := mime.ParseMediaType(mediaType); err != nil {
		return nil, fmt.Errorf("tcm: invalid attachment media type: %w", err)
	}
	var mp bytes.Buffer
	writer := multipart.NewWriter(&mp)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename})},
		"Content-Type":        {mediaType},
	})
	if err != nil {
		return nil, fmt.Errorf("tcm: create attachment part: %w", err)
	}
	if _, err := io.Copy(part, body); err != nil {
		return nil, fmt.Errorf("tcm: read attachment body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("tcm: close attachment multipart: %w", err)
	}
	path += "?parentKind=" + url.QueryEscape(parentKind) + "&parentId=" + url.QueryEscape(parentID)

	// multipart body needs the matching Content-Type + boundary; the default
	// doRaw forces application/json, which makes the server's c.FormFile fail.
	reader, err := a.client.doRawCT(ctx, http.MethodPost, path, mp.Bytes(), writer.FormDataContentType())
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
	path, err := a.nsBase(namespace, "/tcm/attachments")
	if err != nil {
		return nil, err
	}
	path += "?parentKind=" + url.QueryEscape(parentKind) + "&parentId=" + url.QueryEscape(parentID)
	if parentKind == "test_plan_run" {
		ns := namespace
		if ns == "" {
			ns = a.client.namespace
		}
		return a.listLaunchAttachments(ctx, path, ns, parentID)
	}
	var resp struct {
		Items []TCMAttachment `json:"items"`
	}
	err = a.client.do(ctx, http.MethodGet, path, nil, &resp)
	return resp.Items, err
}

// DeleteAttachment soft-deletes the row and schedules the blob for removal.
func (a *TCMAPI) DeleteAttachment(ctx context.Context, namespace, id string) error {
	path, err := a.nsBase(namespace, "/tcm/attachments/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, path, nil, nil)
}

// DownloadAttachment streams the raw blob. Caller must Close the returned
// reader.
func (a *TCMAPI) DownloadAttachment(ctx context.Context, namespace, id string) (io.ReadCloser, error) {
	path, err := a.nsBase(namespace, "/tcm/attachments/"+url.PathEscape(id)+"/raw")
	if err != nil {
		return nil, err
	}
	return a.client.doRaw(ctx, http.MethodGet, path, nil)
}
