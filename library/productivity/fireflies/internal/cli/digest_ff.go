// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var processedOnly bool

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Aggregate summary of recent meetings — titles, decisions, and action items",
		Long: `Shows a digest of all meetings in the lookback window:
titles, gist summaries, topics, and action items aggregated in one view.
Designed as the entry point for a morning sync or cron job.`,
		Example: strings.Trim(`
  fireflies-pp-cli digest
  fireflies-pp-cli digest --since 48h
  fireflies-pp-cli digest --since 24h --processed-only --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"meetings":[],"total":0}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			dur, err := parseSinceDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since %q: %w", since, err)
			}
			cutoffMs := nowMinusDuration(dur)

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) DESC`,
				cutoffMs)
			if err != nil {
				return fmt.Errorf("querying: %w", err)
			}
			defer rows.Close()

			type digestEntry struct {
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Date        string   `json:"date"`
				Status      string   `json:"status"`
				Gist        string   `json:"gist,omitempty"`
				Topics      []string `json:"topics,omitempty"`
				ActionItems string   `json:"action_items,omitempty"`
				Participants []string `json:"participants,omitempty"`
			}
			var entries []digestEntry

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				if processedOnly && t.summaryStatus() != "PROCESSED" {
					continue
				}

				entry := digestEntry{
					ID:           t.ID,
					Title:        t.Title,
					Date:         t.dateFormatted(),
					Status:       t.summaryStatus(),
					Participants: t.Participants,
				}
				if t.Summary != nil {
					entry.Gist = t.Summary.Gist
					entry.Topics = t.Summary.Topics
					entry.ActionItems = t.Summary.ActionItems
				}
				entries = append(entries, entry)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				type digestResult struct {
					Since    string        `json:"since"`
					Total    int           `json:"total"`
					Meetings []digestEntry `json:"meetings"`
				}
				return printJSONFiltered(cmd.OutOrStdout(), digestResult{
					Since:    since,
					Total:    len(entries),
					Meetings: entries,
				}, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Digest — last %s (%d meetings)\n\n", since, len(entries))
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No meetings found. Run 'fireflies-pp-cli sync' first.")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "▶ %s  %s  [%s]\n", e.Date, e.Title, e.Status)
				if e.Gist != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", e.Gist)
				}
				if len(e.Topics) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  Topics: %s\n", strings.Join(e.Topics, ", "))
				}
				if e.ActionItems != "" {
					lines := strings.Split(strings.TrimSpace(e.ActionItems), "\n")
					fmt.Fprintf(cmd.OutOrStdout(), "  Actions (%d items)\n", len(lines))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "24h", "Duration to look back (e.g. 24h, 48h, 7d)")
	cmd.Flags().BoolVar(&processedOnly, "processed-only", false, "Only include PROCESSED meetings")
	return cmd
}
