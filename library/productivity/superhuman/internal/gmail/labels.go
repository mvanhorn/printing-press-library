// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gmail

import (
	"context"
	"sort"
)

// Label is one row from users.labels.list. Type is "system" for the 11
// built-in labels (INBOX, SENT, DRAFT, SPAM, TRASH, IMPORTANT, STARRED,
// UNREAD, CATEGORY_*) and "user" for user-created labels.
type Label struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	MessageListVisibility string `json:"messageListVisibility,omitempty"`
	LabelListVisibility   string `json:"labelListVisibility,omitempty"`
	Color                 *Color `json:"color,omitempty"`
}

// Color is the optional swatch Gmail attaches to user labels.
type Color struct {
	TextColor       string `json:"textColor,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

// ListLabels fetches every label visible to the active account and returns
// them sorted: system labels first (their natural Gmail order), then user
// labels alphabetical. This deterministic ordering means the CLI's labels-list
// output is stable across invocations even when Gmail's backend re-orders.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var raw struct {
		Labels []Label `json:"labels"`
	}
	if err := c.GetJSON(ctx, "/users/me/labels", &raw); err != nil {
		return nil, err
	}

	// Sort: system first (stable order), then user (alphabetical by name).
	sort.SliceStable(raw.Labels, func(i, j int) bool {
		a, b := raw.Labels[i], raw.Labels[j]
		if a.Type != b.Type {
			// "system" < "user"
			return a.Type == "system"
		}
		if a.Type == "user" {
			return a.Name < b.Name
		}
		// System labels: preserve Gmail's order (stable sort = no swap).
		return false
	})

	return raw.Labels, nil
}

// Well-known system label ids referenced by parity commands. Centralized so
// the CLI's threads update / trash / mark-spam / archive verbs all map to
// the same canonical names rather than embedding string literals.
const (
	SystemLabelInbox    = "INBOX"
	SystemLabelUnread   = "UNREAD"
	SystemLabelStarred  = "STARRED"
	SystemLabelSpam     = "SPAM"
	SystemLabelTrash    = "TRASH"
	SystemLabelImportant = "IMPORTANT"
)
