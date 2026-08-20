package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newOnairReaperCmd(flags *rootFlags) *cobra.Command {
	var staleDays int
	var apply bool

	cmd := &cobra.Command{
		Use:   "onair-reaper",
		Short: "Find recurring On-Air events with zero attendance over N days; --apply cancels them",
		Long: `Aggregates the local attendance table over the last --stale-days; flags On-Air events
with zero attendance during that window. --apply runs /onair.event.cancel for each.
Without --apply this is a dry-run that prints the candidate list.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()
			if err := ensureMessagesTables(cmd.Context(), db); err != nil {
				return apiErr(err)
			}

			cutoff := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour).UTC().Format(time.RFC3339)

			// Events from local store, excluding any with attendance since cutoff
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT e.id, COALESCE(json_extract(e.data, '$.title'), '') AS title
				FROM onair_event_info e
				WHERE NOT EXISTS (
					SELECT 1 FROM attendance a
					WHERE a.event_id = e.id AND a.joined_at >= ?
				)
			`, cutoff)
			if err != nil {
				return apiErr(fmt.Errorf("query stale events: %w", err))
			}
			defer rows.Close()

			type stale struct {
				ID       string `json:"id"`
				Title    string `json:"title"`
				Canceled bool   `json:"canceled,omitempty"`
				Error    string `json:"error,omitempty"`
			}
			candidates := []stale{}
			for rows.Next() {
				var s stale
				if rows.Scan(&s.ID, &s.Title) == nil {
					candidates = append(candidates, s)
				}
			}

			w := cmd.OutOrStdout()
			if !apply || dryRunOK(flags) {
				body, _ := json.Marshal(map[string]any{"dry_run": true, "stale_days": staleDays, "candidates": candidates})
				fmt.Fprintln(w, string(body))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			canceled := 0
			for i := range candidates {
				_, code, err := c.Post("/onair.event.cancel", map[string]any{"id": candidates[i].ID})
				if err != nil || code >= 400 {
					if err != nil {
						candidates[i].Error = err.Error()
					} else {
						candidates[i].Error = fmt.Sprintf("HTTP %d", code)
					}
					continue
				}
				candidates[i].Canceled = true
				canceled++
			}
			body, _ := json.Marshal(map[string]any{"applied": true, "canceled": canceled, "results": candidates})
			fmt.Fprintln(w, string(body))
			return nil
		},
	}
	cmd.Flags().IntVar(&staleDays, "stale-days", 60, "Window of days with zero attendance to qualify as stale")
	cmd.Flags().BoolVar(&apply, "apply", false, "Cancel matching events (default is dry-run)")
	return cmd
}
