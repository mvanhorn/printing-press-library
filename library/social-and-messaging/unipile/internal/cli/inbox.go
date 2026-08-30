// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type inboxEntry struct {
	ChatID      string `json:"chat_id"`
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Unread      int    `json:"unread_count"`
	LastMessage string `json:"last_message,omitempty"`
	LastFrom    string `json:"last_from,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

func newNovelInboxCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		limit    int
		provider string
	)
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "One table of everything unread across every connected provider",
		Long: strings.Trim(`
Unified unread triage across LinkedIn, WhatsApp, Telegram, Instagram, Messenger,
and email, read from the local mirror.

Use this command as the daily triage view. Do NOT use it to read a full
conversation; use 'thread' for that.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli inbox --agent --limit 25
  unipile-pp-cli inbox --provider LINKEDIN
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--limit=5",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inbox")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries := make([]inboxEntry, 0)
			db, ok, err := novelStore(cmd, flags, dbPath, entries)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			chats, err := loadChats(ctx, db, true)
			if err != nil {
				return err
			}
			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			attendees, err := loadAttendees(ctx, db)
			if err != nil {
				return err
			}
			last, _, _, err := loadLastMessages(ctx, db)
			if err != nil {
				return err
			}

			want := strings.ToUpper(strings.TrimSpace(provider))
			for _, c := range chats {
				if want != "" && !strings.EqualFold(c.AccountType, want) {
					continue
				}
				e := inboxEntry{
					ChatID:   c.ID,
					Provider: c.AccountType,
					Name:     counterpartName(c, relations, attendees),
					Unread:   c.UnreadCount,
				}
				if !c.Timestamp.IsZero() {
					e.Timestamp = c.Timestamp.UTC().Format(time.RFC3339)
				}
				if m, hit := last[c.ID]; hit {
					e.LastMessage = truncate(strings.ReplaceAll(m.Text, "\n", " "), 120)
					// Resolve the actual sender. Reusing e.Name here duplicated
					// the chat title and, on sponsored InMail, printed a subject
					// line where a sender name belongs.
					e.LastFrom = senderLabel(m, relations, attendees)
				}
				entries = append(entries, e)
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp > entries[j].Timestamp })
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}

			emptyMirrorHint(ctx, cmd, db, len(entries))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Inbox clear: no unread conversations in the local mirror.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PROVIDER\tFROM\tUNREAD\tLAST MESSAGE\tWHEN")
			for _, e := range entries {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", e.Provider, truncate(e.Name, 24), e.Unread, truncate(e.LastMessage, 60), e.Timestamp)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum unread conversations to return")
	cmd.Flags().StringVar(&provider, "provider", "", "restrict to one provider (LINKEDIN, WHATSAPP, TELEGRAM, INSTAGRAM, MESSENGER, MAIL)")
	return cmd
}
