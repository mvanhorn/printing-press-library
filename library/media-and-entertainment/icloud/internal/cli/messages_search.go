// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newMessagesSearchCmd(f *rootFlags) *cobra.Command {
	var chatFilter, handleFilter string
	var sinceStr, untilStr string
	var fromMe, fromOthers bool
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search message bodies",
		Long: `Full-text search across your iMessage history.

The query is matched case-insensitively against message text. Messages
whose text is NULL on modern macOS are decoded from attributedBody and
re-filtered in process, so post-Big-Sur messages are not silently missed.`,
		Example: `  # Find messages containing "lunch"
  icloud-pp-cli messages search lunch

  # Only messages I sent in 2026
  icloud-pp-cli messages search "thanks" --from-me --since 2026-01-01

  # Restrict to one chat (by GUID or chat_identifier)
  icloud-pp-cli messages search "happy birthday" --chat +15551234567

  # Limit + JSON for downstream tooling
  icloud-pp-cli messages search project --limit 10 --agent | jq .`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if query == "" {
				return usageErr(fmt.Errorf("query may not be empty"))
			}
			if fromMe && fromOthers {
				return usageErr(fmt.Errorf("--from-me and --from-others are mutually exclusive"))
			}

			opts := SearchOpts{
				Query:        query,
				ChatFilter:   chatFilter,
				HandleFilter: handleFilter,
				Limit:        limit,
			}
			if fromMe {
				v := true
				opts.FromMe = &v
			} else if fromOthers {
				v := false
				opts.FromMe = &v
			}
			if sinceStr != "" {
				t, err := parseDateFlag(sinceStr)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				opts.Since = &t
			}
			if untilStr != "" {
				t, err := parseDateFlag(untilStr)
				if err != nil {
					return usageErr(fmt.Errorf("--until: %w", err))
				}
				opts.Until = &t
			}

			db, err := openMessagesDB(f.messagesDBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			results, err := searchMessages(db, opts)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			if len(results) == 0 {
				if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSON(cmd.OutOrStdout(), []searchEntryJSON{})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No matches.")
				return nil
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printSearchJSON(cmd, f, results)
			}
			return printSearchTable(cmd, f, results)
		},
	}

	cmd.Flags().StringVar(&chatFilter, "chat", "", "Restrict to a specific chat (GUID or chat_identifier)")
	cmd.Flags().StringVar(&handleFilter, "handle", "", "Restrict to messages from a specific handle (phone/email)")
	cmd.Flags().BoolVar(&fromMe, "from-me", false, "Only messages sent by you")
	cmd.Flags().BoolVar(&fromOthers, "from-others", false, "Only messages sent by others")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Lower date bound (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&untilStr, "until", "", "Upper date bound (YYYY-MM-DD or RFC3339)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return (max 1000)")

	return cmd
}

type searchEntryJSON struct {
	Chat           string `json:"chat,omitempty"`
	ChatGUID       string `json:"chat_guid,omitempty"`
	Sender         string `json:"sender,omitempty"`
	IsFromMe       bool   `json:"is_from_me"`
	Date           string `json:"date"`
	Text           string `json:"text"`
	TextSource     string `json:"text_source"`
	HasAttachments bool   `json:"has_attachments,omitempty"`
	ROWID          int64  `json:"rowid,omitempty"`
}

func printSearchJSON(cmd *cobra.Command, f *rootFlags, results []MessageRow) error {
	out := make([]searchEntryJSON, len(results))
	for i, m := range results {
		row := searchEntryJSON{
			Chat:           displayChat(m),
			ChatGUID:       m.ChatGUID,
			Sender:         senderLabel(m),
			IsFromMe:       m.IsFromMe,
			Date:           m.Date.Format(time.RFC3339),
			Text:           m.Text,
			TextSource:     m.TextSource,
			HasAttachments: m.HasAttachments,
		}
		if !f.compact {
			row.ROWID = m.ROWID
		}
		out[i] = row
	}
	return printJSON(cmd.OutOrStdout(), out)
}

func printSearchTable(cmd *cobra.Command, f *rootFlags, results []MessageRow) error {
	out := cmd.OutOrStdout()
	w := newTabWriter(out)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
		bold(f, out, "Date"),
		bold(f, out, "Sender"),
		bold(f, out, "Chat"),
		bold(f, out, "Message"),
	)
	for _, m := range results {
		date := m.Date.Format("2006-01-02 15:04")
		text := m.Text
		// PATCH(messages-preview-rune-aware-truncate): slice at rune boundaries
		// so emoji and CJK rows don't display as mojibake at the cell boundary.
		if runes := []rune(text); len(runes) > 100 {
			text = string(runes[:100]) + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			date, senderLabel(m), displayChat(m), text,
		)
	}
	return w.Flush()
}

func displayChat(m MessageRow) string {
	if m.ChatDisplayName != "" {
		return m.ChatDisplayName
	}
	return m.ChatGUID
}

func senderLabel(m MessageRow) string {
	if m.IsFromMe {
		return "me"
	}
	if m.HandleAddress != "" {
		return m.HandleAddress
	}
	return "?"
}
