// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package appscrape

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
)

type AppPagePayload struct {
	Flows   []map[string]any
	Screens []map[string]any
	AppName string
	Slug    string
}

var nextChunkRE = regexp.MustCompile(`self\.__next_f\.push\(\[1,\s*"((?:\\.|[^"\\])*)"\]\)`)

func Fetch(ctx context.Context, c *client.Client, slug string) (*AppPagePayload, error) {
	raw, err := c.Get(ctx, "https://mobbin.com/apps/"+slug+"/screens", nil)
	if err != nil {
		return nil, err
	}
	html := string(raw)
	var b strings.Builder
	for _, m := range nextChunkRE.FindAllStringSubmatch(html, -1) {
		var s string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &s); err == nil {
			b.WriteString(s)
		}
	}
	stream := b.String()
	if stream == "" {
		stream = html
	}
	arr, err := extractPayloadArray(stream)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal([]byte(arr), &payload); err != nil {
		return nil, fmt.Errorf("parsing app payload: %w", err)
	}
	out := &AppPagePayload{Slug: slug}
	if len(payload) > 0 {
		out.Flows = rawArray(payload[0].Value)
	}
	if len(payload) > 1 {
		out.Screens = rawArray(payload[1].Value)
	}
	out.AppName = findString(out.Screens, "appName", "app_name")
	if out.AppName == "" {
		out.AppName = findString(out.Flows, "appName", "app_name")
	}
	return out, nil
}

func rawArray(raw json.RawMessage) []map[string]any {
	var rows []map[string]any
	_ = json.Unmarshal(raw, &rows)
	return rows
}

func extractPayloadArray(s string) (string, error) {
	idx := strings.Index(s, `[{"value":[`)
	if idx < 0 {
		idx = strings.Index(s, `[{"value":`)
	}
	if idx < 0 {
		idx = strings.Index(s, `[{"value"`)
	}
	if idx < 0 {
		return "", fmt.Errorf("could not find Next.js value payload in app page")
	}
	depth := 0
	inStr := false
	esc := false
	for i := idx; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				// Flight chunks are already unescaped before scanning.
				return s[idx : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated app payload")
}

func findString(rows []map[string]any, keys ...string) string {
	for _, r := range rows {
		for _, k := range keys {
			if s, ok := r[k].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
