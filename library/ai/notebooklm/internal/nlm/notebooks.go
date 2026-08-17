// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ListNotebooks returns recently viewed notebooks for the authenticated account.
func (c *Client) ListNotebooks(ctx context.Context) ([]Notebook, error) {
	// Params from notebooklm-py NotebooksAPI.list: [null, 1, null, [2]]
	params := []any{nil, 1, nil, []int{2}}
	raw, err := c.Call(ctx, RPCListNotebooks, "/", params)
	if err != nil {
		if strings.Contains(err.Error(), "null result") {
			return nil, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	return parseNotebookList(raw)
}

func parseNotebookList(raw json.RawMessage) ([]Notebook, error) {
	var envelope []json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("parse notebook list envelope: %w", err)
	}
	if len(envelope) == 0 {
		return nil, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(envelope[0], &rows); err != nil {
		return nil, fmt.Errorf("parse notebook rows: %w", err)
	}
	var out []Notebook
	for _, row := range rows {
		nb, ok, err := parseNotebookRow(row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, nb)
		}
	}
	return out, nil
}

func parseNotebookRow(raw json.RawMessage) (Notebook, bool, error) {
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return Notebook{}, false, fmt.Errorf("parse notebook row: %w", err)
	}
	if len(arr) < 2 {
		return Notebook{}, false, nil
	}

	// Live API: [title, sources, id, emoji, ...]
	if len(arr) > 2 {
		if id, ok := arr[2].(string); ok && looksLikeUUID(id) {
			title, _ := arr[0].(string)
			nb := Notebook{ID: id, Title: title, SourceCount: countSourceEntries(arr[1])}
			if len(arr) > 3 {
				if emoji, ok := arr[3].(string); ok {
					nb.Emoji = emoji
				}
			}
			return nb, true, nil
		}
	}

	// Legacy/test shape: [id, title, emoji, ...]
	id, _ := arr[0].(string)
	if id == "" || !looksLikeUUID(id) {
		return Notebook{}, false, nil
	}
	title, _ := arr[1].(string)
	nb := Notebook{ID: id, Title: title}
	if len(arr) > 2 {
		if emoji, ok := arr[2].(string); ok {
			nb.Emoji = emoji
		}
	}
	if len(arr) > 5 {
		if meta, ok := arr[5].([]any); ok && len(meta) > 0 {
			switch n := meta[0].(type) {
			case float64:
				nb.SourceCount = int(n)
			case int:
				nb.SourceCount = n
			}
		}
	}
	return nb, true, nil
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
				return false
			}
		}
	}
	return true
}

func countSourceEntries(v any) int {
	rows, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(rows)
}
