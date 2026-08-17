// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetShareStatus returns sharing configuration for a notebook.
func (c *Client) GetShareStatus(ctx context.Context, notebookID string) (ShareStatus, error) {
	raw, err := c.Call(ctx, RPCGetShareStatus, notebookPath(notebookID), BuildShareStatusParams(notebookID))
	if err != nil {
		return ShareStatus{}, err
	}
	return parseShareStatus(notebookID, raw), nil
}

func parseShareStatus(notebookID string, raw json.RawMessage) ShareStatus {
	st := ShareStatus{NotebookID: notebookID}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return st
	}
	walkShare(v, &st)
	return st
}

func walkShare(v any, st *ShareStatus) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			walkShare(item, st)
		}
	case float64:
		if t == 1 || t == 2 {
			st.Public = t == 1
		}
	case string:
		if t == "ANYONE_WITH_LINK" || t == "PUBLIC" {
			st.Public = true
		}
	}
}

// GetAccountInfo reads account limits/settings.
func (c *Client) GetAccountInfo(ctx context.Context) (AccountInfo, error) {
	raw, err := c.Call(ctx, RPCGetUserSettings, "/", BuildUserSettingsParams())
	if err != nil {
		if strings.Contains(err.Error(), "null result") {
			return AccountInfo{}, nil
		}
		return AccountInfo{}, err
	}
	return parseAccountInfo(raw), nil
}

func parseAccountInfo(raw json.RawMessage) AccountInfo {
	var info AccountInfo
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return info
	}
	if row, ok := v.([]any); ok && len(row) > 0 {
		if inner, ok := row[0].([]any); ok {
			if len(inner) > 1 {
				if lang, ok := inner[0].(string); ok {
					info.OutputLanguage = lang
				}
			}
			if limits, ok := inner[1].([]any); ok && len(limits) > 4 {
				switch tier := limits[4].(type) {
				case float64:
					info.Tier = tierName(int(tier))
				case int:
					info.Tier = tierName(tier)
				}
			}
		}
	}
	return info
}

func tierName(code int) string {
	switch code {
	case 0:
		return "free"
	case 1:
		return "pro"
	default:
		return fmt.Sprintf("tier_%d", code)
	}
}
