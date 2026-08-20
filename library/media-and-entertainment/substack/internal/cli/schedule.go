// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type scheduledItem struct {
	ScheduledAt string `json:"scheduled_at"`
	Type        string `json:"type"` // post or draft
	ID          string `json:"id"`
	Title       string `json:"title"`
	Publication string `json:"publication_id"`
}

func newScheduleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schedule",
		Short:       "Cross-publication editorial scheduling.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "schedule"},
	}
	cmd.AddCommand(newScheduleBoardCmd(flags))
	return cmd
}

func newScheduleBoardCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		days   int
	)
	cmd := &cobra.Command{
		Use:   "board",
		Short: "ASCII calendar of the next N days of scheduled posts across all your publications.",
		Long: `Reads scheduled_at timestamps from cached posts and drafts and renders an ASCII
calendar covering the next --days days, across every publication you own.`,
		Example:     "  substack-pp-cli schedule board --days 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "schedule board"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			cutoff := time.Now().AddDate(0, 0, days).Format("2006-01-02")
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT scheduled_at, 'post' AS type, id, COALESCE(title, ''), COALESCE(publication_id, '')
				  FROM posts
				 WHERE scheduled_at IS NOT NULL AND scheduled_at != '' AND scheduled_at <= ?
				UNION ALL
				SELECT scheduled_at, 'draft' AS type, id, COALESCE(title, ''), COALESCE(publication_id, '')
				  FROM drafts
				 WHERE scheduled_at IS NOT NULL AND scheduled_at != '' AND scheduled_at <= ?
				ORDER BY scheduled_at ASC
			`, cutoff, cutoff)
			if err != nil {
				return fmt.Errorf("querying schedule: %w", err)
			}
			defer rows.Close()

			out := []scheduledItem{}
			for rows.Next() {
				var item scheduledItem
				if err := rows.Scan(&item.ScheduledAt, &item.Type, &item.ID, &item.Title, &item.Publication); err != nil {
					return err
				}
				out = append(out, item)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", bold(fmt.Sprintf("Editorial schedule — next %d days", days)))
			fmt.Fprintln(w, strings.Repeat("─", 78))
			if len(out) == 0 {
				fmt.Fprintln(w, "Nothing scheduled.")
				return nil
			}
			currentDay := ""
			for _, it := range out {
				day := strings.SplitN(it.ScheduledAt, "T", 2)[0]
				if day != currentDay {
					fmt.Fprintf(w, "\n%s\n", bold(day))
					currentDay = day
				}
				fmt.Fprintf(w, "  [%s] %-30s pub=%s id=%s\n",
					it.Type, truncate(it.Title, 30), truncate(it.Publication, 14), it.ID)
			}
			fmt.Fprintln(w, strings.Repeat("─", 78))
			fmt.Fprintf(w, "%d item(s) scheduled.\n", len(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&days, "days", 30, "Window in days")
	return cmd
}
