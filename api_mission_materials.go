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
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type MissionMaterialReference struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	Revision int64  `json:"revision"`
}

type MissionMaterial struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	MIMEType  string `json:"mimeType"`
	Digest    string `json:"digest"`
	CreatedBy string `json:"createdBy"`
	Revision  int64  `json:"revision"`
	SizeBytes int64  `json:"sizeBytes"`
}

type MissionMaterialUpload struct {
	Material  MissionMaterial          `json:"material"`
	Reference MissionMaterialReference `json:"reference"`
}

// UploadMissionMaterial preserves an original for a mission's artifacts references.
func (a *CoderDeliveryAPI) UploadMissionMaterial(ctx context.Context, namespace, productID, filename, mediaType string, body io.Reader) (*MissionMaterialUpload, error) {
	if ctx == nil {
		return nil, fmt.Errorf("mockarty: context is required")
	}
	ctx = context.WithValue(ctx, nonReplayableRequestKey{}, true)
	if strings.TrimSpace(namespace) == "" {
		namespace = a.client.namespace
	}
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".md":
			mediaType = "text/markdown"
		case ".yaml", ".yml":
			mediaType = "application/yaml"
		case ".js":
			mediaType = "text/javascript"
		}
		if mediaType == "" {
			return nil, fmt.Errorf("unknown material type; specify mediaType")
		}
	}
	if strings.TrimSpace(productID) == "" || !validMissionMaterialFilename(filename) || body == nil || strings.ContainsAny(mediaType, "\r\n") {
		return nil, fmt.Errorf("product, filename and valid material content are required")
	}
	mediaBase, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return nil, fmt.Errorf("invalid mission material media type")
	}
	binary := false
	switch mediaBase {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "application/pdf":
		binary = true
	case "text/plain", "text/markdown", "text/css", "text/javascript", "application/json", "application/yaml", "application/xml", "text/html":
	default:
		return nil, fmt.Errorf("unsupported mission material media type")
	}
	data, err := io.ReadAll(io.LimitReader(body, 16*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read mission material: %w", err)
	}
	if len(data) == 0 || len(data) > 16*1024*1024 {
		return nil, fmt.Errorf("mission material must be nonempty and at most 16 MiB")
	}
	if !binary && len(data) > 64*1024 {
		return nil, fmt.Errorf("text mission materials must be at most 64 KiB combined")
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
	if mediaType != "" {
		header.Set("Content-Type", mediaType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(data); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	query := url.Values{"namespace": {namespace}, "productId": {productID}}
	response, err := a.client.doRawCT(ctx, http.MethodPost, "/api/v1/missions/materials?"+query.Encode(), buf.Bytes(), writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer response.Close()
	var result MissionMaterialUpload
	if err = json.NewDecoder(response).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func validMissionMaterialFilename(name string) bool {
	return name != "" && len(name) <= 128 && utf8.ValidString(name) &&
		name == strings.TrimSpace(name) && !strings.ContainsAny(name, "/\\") &&
		strings.IndexFunc(name, unicode.IsControl) < 0 && name != "." && name != ".."
}
