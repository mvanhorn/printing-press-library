// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/client"
)

// resolveOwnUserID fetches the authenticated user's user_id by calling
// /subscriptions/page_v2 (which returns subscriptions[].user_id). This is
// required for the draft_bylines field on POST /drafts. The result is
// cached on first call via an env var so repeated invocations don't re-fetch.
func resolveOwnUserID(ctx context.Context, c *client.Client) (int64, error) {
	if cached := os.Getenv("SUBSTACK_OWN_USER_ID"); cached != "" {
		var n int64
		_, err := fmt.Sscanf(cached, "%d", &n)
		if err == nil && n > 0 {
			return n, nil
		}
	}
	raw, err := c.Get(ctx, "/subscriptions/page_v2", nil)
	if err != nil {
		return 0, fmt.Errorf("resolving own user_id via /subscriptions/page_v2: %w", err)
	}
	var resp struct {
		Subscriptions []struct {
			UserID int64 `json:"user_id"`
		} `json:"subscriptions"`
		PublicationUsers []struct {
			UserID int64 `json:"user_id"`
		} `json:"publicationUsers"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("parsing /subscriptions/page_v2 response: %w", err)
	}
	var uid int64
	if len(resp.Subscriptions) > 0 {
		uid = resp.Subscriptions[0].UserID
	}
	// publicationUsers (the creator's role memberships) carries the user's own
	// id even for a fresh creator account that has never subscribed to a
	// newsletter, where subscriptions[] is empty — the common drafts-create path.
	if uid == 0 {
		for _, pu := range resp.PublicationUsers {
			if pu.UserID > 0 {
				uid = pu.UserID
				break
			}
		}
	}
	if uid == 0 {
		return 0, fmt.Errorf("could not infer own user_id from /subscriptions/page_v2 (no subscriptions or publication memberships); set SUBSTACK_OWN_USER_ID or pass --byline")
	}
	_ = os.Setenv("SUBSTACK_OWN_USER_ID", fmt.Sprintf("%d", uid))
	return uid, nil
}

// markdownToProseMirror is a thin wrapper around markdownToProseMirrorExt
// (in prosemirror.go) so existing call sites stay valid. The extended
// converter handles headings, paragraphs, lists, blockquotes, code blocks,
// LaTeX, inline marks (bold/italic/code/link), images, and Substack
// directives ({{button}}, {{embed}}, {{pullquote}}, {{image}}, [paywall]).
func markdownToProseMirror(md string) string {
	return markdownToProseMirrorExt(md)
}

// buildDraftBody converts user-supplied body inputs into the draft_body
// string Substack expects. It supports three modes, in priority order:
// 1. --body-json (raw ProseMirror JSON) — passed through verbatim.
// 2. --body-file (path to a file) — file content is wrapped via markdown converter.
// 3. --body (inline string) — wrapped via markdown converter.
func buildDraftBody(bodyInline, bodyFile, bodyJSON string) (string, error) {
	if bodyJSON != "" {
		// Validate it parses
		var tmp any
		if err := json.Unmarshal([]byte(bodyJSON), &tmp); err != nil {
			return "", fmt.Errorf("--body-json is not valid JSON: %w", err)
		}
		return bodyJSON, nil
	}
	if bodyFile != "" {
		raw, err := os.ReadFile(bodyFile)
		if err != nil {
			return "", fmt.Errorf("reading --body-file %q: %w", bodyFile, err)
		}
		content := string(raw)
		// If the file is already ProseMirror JSON, pass through
		var tmp any
		if json.Unmarshal(raw, &tmp) == nil {
			if m, ok := tmp.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "doc" {
					return content, nil
				}
			}
		}
		return markdownToProseMirror(content), nil
	}
	if bodyInline != "" {
		return markdownToProseMirror(bodyInline), nil
	}
	return "", nil
}
