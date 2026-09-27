// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const allureInlineFileLimit = 256 << 10
const allureInlineResultLimit = 1 << 20
const allureInlineBatchLimit = 32 << 20

// Source files are read once during preflight. No reference may escape the
// selected result directory or silently disappear from the uploaded run.
func readAllureSource(directory, source string, limit int64) ([]byte, error) {
	if source == "" || !filepath.IsLocal(source) || strings.ContainsAny(source, "\\:") {
		return nil, fmt.Errorf("allure: invalid source path %q", source)
	}
	path := directory
	for _, part := range strings.Split(filepath.ToSlash(source), "/") {
		if part == ".." || part == "." || part == "" {
			return nil, fmt.Errorf("allure: invalid source path %q", source)
		}
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("allure: source %q: %w", source, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("allure: source %q is a symlink", source)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("allure: source %q is not a regular file", source)
	}
	body, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || int64(len(body)) > limit {
		return nil, fmt.Errorf("allure: source %q is empty or exceeds %d bytes", source, limit)
	}
	return body, nil
}

func collectAllureMedia(doc map[string]any, directory string) ([]ExternalStep, []ExternalAttachment, map[string][]string, int, error) {
	steps := []ExternalStep{}
	atts := []ExternalAttachment{}
	paths := map[string][]string{}
	total := 0
	var visit func(map[string]any, string, string, int) error
	visit = func(node map[string]any, path, prefix string, depth int) error {
		if depth > 64 {
			return fmt.Errorf("allure: step depth exceeds 64")
		}
		if raw, exists := node["attachments"]; exists && raw != nil {
			rows, ok := raw.([]any)
			if !ok {
				return fmt.Errorf("allure: invalid attachments at %s", path)
			}
			for _, rawAtt := range rows {
				att, ok := rawAtt.(map[string]any)
				if !ok {
					return fmt.Errorf("allure: invalid attachment at %s", path)
				}
				source := allureStr(att, "source")
				if _, seen := paths[source]; !seen {
					if len(atts) >= 32 {
						return fmt.Errorf("allure: more than 32 attachments per result")
					}
					body, err := readAllureSource(directory, source, allureInlineFileLimit)
					if err != nil {
						return err
					}
					total += len(body)
					if total > allureInlineResultLimit {
						return fmt.Errorf("allure: result attachments exceed 1 MiB")
					}
					name := allureStr(att, "name")
					if name == "" {
						name = source
					}
					kind := allureStr(att, "type")
					if kind == "" {
						kind = "application/octet-stream"
					}
					atts = append(atts, ExternalAttachment{Name: name, ContentType: kind, BodyB64: base64.StdEncoding.EncodeToString(body)})
				}
				paths[source] = append(paths[source], path)
			}
		}
		if raw, exists := node["steps"]; exists && raw != nil {
			rows, ok := raw.([]any)
			if !ok {
				return fmt.Errorf("allure: invalid steps at %s", path)
			}
			for i, rawStep := range rows {
				step, ok := rawStep.(map[string]any)
				if !ok {
					return fmt.Errorf("allure: invalid step at %s", path)
				}
				if len(steps) >= 5000 {
					return fmt.Errorf("allure: more than 5000 steps")
				}
				childPath := fmt.Sprintf("%s/steps/%d", path, i)
				status := strings.ToLower(strings.TrimSpace(allureStr(step, "status")))
				switch status {
				case "passed", "failed", "skipped", "broken":
				default:
					status = "broken"
				}
				var detail string
				if sd, ok := step["statusDetails"].(map[string]any); ok {
					detail = strings.TrimSpace(allureStr(sd, "message") + "\n" + allureStr(sd, "trace"))
				}
				duration := int64(0)
				start, sOK := allureNum(step["start"])
				stop, eOK := allureNum(step["stop"])
				if sOK && eOK && stop >= start {
					duration = stop - start
				}
				name := allureStr(step, "name")
				if prefix != "" {
					name = prefix + " / " + name
				}
				steps = append(steps, ExternalStep{Name: name, Status: status, Error: detail, DurationMs: duration, Metadata: map[string]any{"allurePath": childPath}})
				if err := visit(step, childPath, name, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if doc == nil {
		return nil, nil, nil, 0, fmt.Errorf("allure: result must be an object")
	}
	err := visit(doc, "result", "", 0)
	return steps, atts, paths, total, err
}
