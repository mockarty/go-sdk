package mockarty

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (a *TCMAPI) listLaunchAttachments(ctx context.Context, path, namespace, parentID string) ([]TCMAttachment, error) {
	out := make([]TCMAttachment, 0)
	seen := make(map[string]bool)
	total := -1
	snapshot := ""
	for page := 0; page < 100; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var resp struct {
			Items    *[]TCMAttachment `json:"items"`
			Total    *int             `json:"total"`
			Offset   *int             `json:"offset"`
			Limit    *int             `json:"limit"`
			Snapshot *string          `json:"snapshot"`
			HasMore  *bool            `json:"hasMore"`
		}
		pagePath := fmt.Sprintf("%s&limit=500&offset=%d", path, len(out))
		if snapshot != "" {
			pagePath += "&snapshot=" + url.QueryEscape(snapshot)
		}
		if err := a.client.do(ctx, http.MethodGet, pagePath, nil, &resp); err != nil {
			return nil, err
		}
		if resp.Items == nil || resp.Total == nil || resp.Offset == nil || resp.Limit == nil || resp.HasMore == nil || *resp.Total < 0 || *resp.Total > 10000 || *resp.Offset != len(out) || *resp.Limit < 1 || *resp.Limit > 500 || len(*resp.Items) > *resp.Limit || (total >= 0 && total != *resp.Total) {
			return nil, fmt.Errorf("tcm: inconsistent launch file pagination; retry the report listing")
		}
		if resp.Snapshot == nil || len(*resp.Snapshot) != 64 || strings.ToLower(*resp.Snapshot) != *resp.Snapshot || (snapshot != "" && snapshot != *resp.Snapshot) {
			return nil, fmt.Errorf("tcm: launch files changed or snapshot is missing; retry the report listing")
		}
		if _, err := hex.DecodeString(*resp.Snapshot); err != nil {
			return nil, fmt.Errorf("tcm: invalid launch file snapshot; retry the report listing")
		}
		snapshot = *resp.Snapshot
		total = *resp.Total
		if len(out)+len(*resp.Items) > total || *resp.HasMore != (len(out)+len(*resp.Items) < total) || (*resp.HasMore && len(*resp.Items) == 0) {
			return nil, fmt.Errorf("tcm: incomplete launch file page; retry the report listing")
		}
		for _, item := range *resp.Items {
			if item.ID == "" || seen[item.ID] || item.Namespace != namespace || item.ParentKind != "test_plan_run" || item.ParentID != parentID {
				return nil, fmt.Errorf("tcm: duplicate or foreign launch file metadata; retry the report listing")
			}
			seen[item.ID] = true
			out = append(out, item)
		}
		if !*resp.HasMore {
			return out, nil
		}
	}
	return nil, fmt.Errorf("tcm: launch file pagination exceeded 100 requests; retry the report listing")
}
