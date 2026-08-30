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

type silentEntry struct {
	ChatID      string `json:"chat_id"`
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	LastMessage string `json:"last_message,omitempty"`
	SentAt      string `json:"sent_at,omitempty"`
	DaysSilent  int    `json:"days_silent"`
	TheyReplied bool   `json:"they_ever_replied"`
}

func newNovelSilentCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		days     int
		maxDays  int
		limit    int
		provider string
	)
	cmd := &cobra.Command{
		Use:   "silent",
		Short: "Conversations where you sent the last message and got no reply",
		Long: strings.Trim(`
Finds conversations whose most recent message came from you and has gone
unanswered for at least --days, computed over the local mirror.

Use this command to build a follow-up list. Do NOT use it for people who never
replied to an invitation; use 'funnel' or 'accepted' for that.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli silent --days 7 --agent
  unipile-pp-cli silent --days 3 --provider LINKEDIN --limit 50
  unipile-pp-cli silent --days 30 --max-days 0
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--days=7;--limit=5",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "silent")
			}
			if days < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be zero or positive"))
			}
			if maxDays > 0 && maxDays < days {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-days (%d) must be at least --days (%d)", maxDays, days))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries := make([]silentEntry, 0)
			db, ok, err := novelStore(cmd, flags, dbPath, entries)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			chats, err := loadChats(ctx, db, false)
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
			last, inbound, _, err := loadLastMessages(ctx, db)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			cutoff := now.AddDate(0, 0, -days)
			var floor time.Time
			if maxDays > 0 {
				floor = now.AddDate(0, 0, -maxDays)
			}
			want := strings.ToUpper(strings.TrimSpace(provider))
			for _, c := range chats {
				if want != "" && !strings.EqualFold(c.AccountType, want) {
					continue
				}
				m, hit := last[c.ID]
				if !hit || !m.FromMe || m.Timestamp.After(cutoff) {
					continue
				}
				if !floor.IsZero() && m.Timestamp.Before(floor) {
					continue
				}
				entries = append(entries, silentEntry{
					ChatID:      c.ID,
					Provider:    c.AccountType,
					Name:        counterpartName(c, relations, attendees),
					LastMessage: truncate(strings.ReplaceAll(m.Text, "\n", " "), 160),
					SentAt:      m.Timestamp.UTC().Format(time.RFC3339),
					DaysSilent:  int(now.Sub(m.Timestamp).Hours() / 24),
					TheyReplied: inbound[c.ID] > 0,
				})
			}
			// Ascending: a thread that went quiet three days ago is a live
			// follow-up; one that went quiet three years ago is archaeology.
			sort.Slice(entries, func(i, j int) bool { return entries[i].SentAt > entries[j].SentAt })
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}

			emptyMirrorHint(ctx, cmd, db, len(entries))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				if maxDays > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "No conversations have been silent between %d and %d days.\n", days, maxDays)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No conversations have been silent for %d+ days.\n", days)
				}
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DAYS\tPROVIDER\tPERSON\tEVER REPLIED\tYOUR LAST MESSAGE")
			for _, e := range entries {
				replied := "no"
				if e.TheyReplied {
					replied = "yes"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n", e.DaysSilent, e.Provider, truncate(e.Name, 24), replied, truncate(e.LastMessage, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().IntVar(&days, "days", 7, "minimum days since your unanswered message")
	cmd.Flags().IntVar(&maxDays, "max-days", 90, "ignore threads silent longer than this many days (0 disables the ceiling)")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum conversations to return")
	cmd.Flags().StringVar(&provider, "provider", "", "restrict to one provider (LINKEDIN, WHATSAPP, TELEGRAM, INSTAGRAM, MESSENGER, MAIL)")
	return cmd
}
