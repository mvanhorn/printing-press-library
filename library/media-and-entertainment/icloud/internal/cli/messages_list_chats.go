// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newMessagesListChatsCmd(f *rootFlags) *cobra.Command {
	var limit int
	var since string
	var includeEmpty bool

	cmd := &cobra.Command{
		Use:   "list-chats",
		Short: "List chats with most-recent activity",
		Long: `List chats from your Messages database, ordered by the most recent message.

Each row shows the chat GUID, display name (or comma-joined participants
for a DM/group), participant count, message count, and a short preview
of the last message — recovered from attributedBody when message.text is
NULL on modern macOS.`,
		Example: `  # 25 most-recently-active chats
  icloud-pp-cli messages list-chats

  # Show only chats with activity since Jan 1 2026
  icloud-pp-cli messages list-chats --since 2026-01-01

  # Include empty chats (no messages yet)
  icloud-pp-cli messages list-chats --include-empty

  # Pipe to jq
  icloud-pp-cli messages list-chats --agent | jq '[.[] | select(.is_group)]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := ChatListOpts{
				Limit:        limit,
				IncludeEmpty: includeEmpty,
			}
			if since != "" {
				t, err := parseDateFlag(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				opts.Since = &t
			}

			db, err := openMessagesDB(f.messagesDBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			chats, err := listChats(db, opts)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			if err := fillLastPreviews(db, chats, 80); err != nil {
				return fmt.Errorf("preview fetch failed: %w", err)
			}

			if len(chats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No chats found.")
				return nil
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printListChatsJSON(cmd, f, chats)
			}
			return printListChatsTable(cmd, f, chats)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum chats to return (0 = all)")
	cmd.Flags().StringVar(&since, "since", "", "Only chats with activity since this date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().BoolVar(&includeEmpty, "include-empty", false, "Include chats with zero messages")

	return cmd
}

// parseDateFlag accepts YYYY-MM-DD or RFC3339. UTC by default for date-only.
func parseDateFlag(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or RFC3339, got %q", s)
}

type listChatsEntryJSON struct {
	GUID            string  `json:"guid"`
	ChatIdentifier  string  `json:"chat_identifier"`
	DisplayName     string  `json:"display_name,omitempty"`
	Participants    int     `json:"participants"`
	MessageCount    int64   `json:"message_count"`
	LastMessageDate string  `json:"last_message_date,omitempty"`
	LastPreview     string  `json:"last_preview,omitempty"`
	IsGroup         bool    `json:"is_group"`
	Style           int     `json:"style,omitempty"`
	ROWID           int64   `json:"rowid,omitempty"`
}

func printListChatsJSON(cmd *cobra.Command, f *rootFlags, chats []ChatRow) error {
	out := make([]listChatsEntryJSON, len(chats))
	for i, c := range chats {
		row := listChatsEntryJSON{
			GUID:           c.GUID,
			ChatIdentifier: c.ChatIdentifier,
			DisplayName:    c.DisplayName,
			Participants:   c.ParticipantCount,
			MessageCount:   c.MessageCount,
			LastPreview:    c.LastPreview,
			IsGroup:        c.IsGroup,
		}
		if c.LastMessageDate != nil {
			row.LastMessageDate = c.LastMessageDate.Format(time.RFC3339)
		}
		if !f.compact {
			row.Style = c.Style
			row.ROWID = c.ROWID
		}
		out[i] = row
	}
	return printJSON(cmd.OutOrStdout(), out)
}

func printListChatsTable(cmd *cobra.Command, f *rootFlags, chats []ChatRow) error {
	out := cmd.OutOrStdout()
	w := newTabWriter(out)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		bold(f, out, "#"),
		bold(f, out, "Type"),
		bold(f, out, "Participants"),
		bold(f, out, "Messages"),
		bold(f, out, "Last"),
		bold(f, out, "Chat / Preview"),
	)
	for i, c := range chats {
		kind := "DM"
		if c.IsGroup {
			kind = "group"
		}
		last := "-"
		if c.LastMessageDate != nil {
			last = c.LastMessageDate.Format("2006-01-02")
		}
		name := c.DisplayName
		if name == "" {
			name = c.ChatIdentifier
		}
		if c.LastPreview != "" {
			name = name + "  " + yellow(f, out, c.LastPreview)
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%d\t%s\t%s\n",
			i+1, kind, c.ParticipantCount, c.MessageCount, last, name,
		)
	}
	return w.Flush()
}
