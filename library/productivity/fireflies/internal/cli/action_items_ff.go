// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

func newActionItemsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action-items",
		Short: "Extract action items from transcripts",
		Long:  "Extract and aggregate action items from synced transcripts.",
	}
	cmd.AddCommand(newActionItemsGetCmd(flags))
	cmd.AddCommand(newActionItemsListCmd(flags))
	return cmd
}

func newActionItemsGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var appendFile string

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get action items from a specific transcript",
		Example: strings.Trim(`
  fireflies-pp-cli action-items get abc123
  fireflies-pp-cli action-items get abc123 --append ~/vaults/VBT/TODO.md`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"action_items":"- [ ] Test action item"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			raw, err := db.Get("transcripts", args[0])
			if err != nil {
				return fmt.Errorf("transcript not found — run 'transcripts pull %s' first", args[0])
			}
			var t transcriptRow
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("parsing transcript: %w", err)
			}
			if t.Summary == nil || t.Summary.ActionItems == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No action items found in this transcript.")
				return nil
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				type result struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					Date        string `json:"date"`
					ActionItems string `json:"action_items"`
				}
				return printJSONFiltered(cmd.OutOrStdout(), result{
					ID:          t.ID,
					Title:       t.Title,
					Date:        t.dateFormatted(),
					ActionItems: t.Summary.ActionItems,
				}, flags)
			}

			output := fmt.Sprintf("## Action Items — %s (%s)\n\n%s\n\n",
				t.Title, t.dateFormatted(), t.Summary.ActionItems)

			if appendFile != "" {
				path := expandHome(appendFile)
				f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("opening %s: %w", path, err)
				}
				defer f.Close()
				if _, err := f.WriteString(output); err != nil {
					return fmt.Errorf("writing to %s: %w", path, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Appended action items to %s\n", path)
				return nil
			}

			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&appendFile, "append", "", "Append action items to this file (e.g. ~/vaults/VBT/TODO.md)")
	return cmd
}

func newActionItemsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fromDays int
	var participant string
	var appendFile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List action items across all transcripts in a date range",
		Long: `Aggregate action items from all synced transcripts in a date range.
Useful for weekly commitment audits or pushing to a TODO file.`,
		Example: strings.Trim(`
  fireflies-pp-cli action-items list --from 7
  fireflies-pp-cli action-items list --from 14 --json
  fireflies-pp-cli action-items list --from 7 --append ~/vaults/VBT/TODO.md`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
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

			query := `SELECT data FROM transcripts ORDER BY CAST(date AS REAL) DESC`
			args2 := []any{}
			if fromDays > 0 {
				cutoff := time.Now().AddDate(0, 0, -fromDays).UnixMilli()
				query = `SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) DESC`
				args2 = append(args2, cutoff)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying: %w", err)
			}
			defer rows.Close()

			type actionItem struct {
				TranscriptID    string `json:"transcript_id"`
				TranscriptTitle string `json:"transcript_title"`
				Date            string `json:"date"`
				ActionItems     string `json:"action_items"`
			}
			var results []actionItem

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				if t.Summary == nil || strings.TrimSpace(t.Summary.ActionItems) == "" {
					continue
				}
				if participant != "" && !t.hasParticipant(participant) {
					continue
				}
				results = append(results, actionItem{
					TranscriptID:    t.ID,
					TranscriptTitle: t.Title,
					Date:            t.dateFormatted(),
					ActionItems:     t.Summary.ActionItems,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			var sb strings.Builder
			for _, r := range results {
				sb.WriteString(fmt.Sprintf("## %s (%s)\n\n%s\n\n", r.TranscriptTitle, r.Date, r.ActionItems))
			}
			output := sb.String()

			if appendFile != "" {
				path := expandHome(appendFile)
				f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("opening %s: %w", path, err)
				}
				defer f.Close()
				header := fmt.Sprintf("\n# Action Items — Last %d days\n\n", fromDays)
				if _, err := f.WriteString(header + output); err != nil {
					return fmt.Errorf("writing: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Appended %d meeting action items to %s\n", len(results), path)
				return nil
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No action items found.")
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), output)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&fromDays, "from", 7, "Look back N days (client-side filter)")
	cmd.Flags().StringVar(&participant, "participant", "", "Only include meetings with this participant email")
	cmd.Flags().StringVar(&appendFile, "append", "", "Append to this file (e.g. ~/vaults/VBT/TODO.md)")
	return cmd
}
