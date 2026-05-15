// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

type watchEvent struct {
	Timestamp    int64    `json:"timestamp_ms"`
	ImpressionID string   `json:"impression_id"`
	Decision     string   `json:"decision"`
	Winner       string   `json:"winner_advertiser_id,omitempty"`
	ScoutScore   *float64 `json:"scout_score,omitempty"`
}

func newPlacementWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		duration time.Duration
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Tail recently logged placements (compact line per event).",
		Long: `Poll the local placement log every 2 seconds and emit any new rows as
they land. Emits a compact human line (timestamp + decision + winner +
scoutScore) or JSON-Lines when --json is set.

Stops after --duration. Reads from the local store only.`,
		Example: strings.Trim(`
  operon-pp-cli placement watch
  operon-pp-cli placement watch --duration 30s
  operon-pp-cli placement watch --duration 10s --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would tail store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would poll placements every 2s for %s\n", duration)
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), duration)
			defer cancel()

			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			// Start from 5s in the past so a placement logged a beat before
			// the command started doesn't get missed.
			sinceMs := time.Now().Add(-5 * time.Second).UnixMilli()
			seen := map[string]bool{}
			deadline := time.Now().Add(duration)

			for {
				placements, err := st.WatchPlacements(ctx, sinceMs)
				if err != nil {
					return apiErr(err)
				}
				for _, p := range placements {
					if seen[p.ID] {
						continue
					}
					seen[p.ID] = true
					ev := watchEvent{
						Timestamp:    p.CreatedAt,
						ImpressionID: p.ID,
						Decision:     p.ResponseDecision,
						Winner:       p.WinnerAdvertiserID,
						ScoutScore:   p.ScoutScore,
					}
					if err := emitWatchEvent(cmd, ev, flags); err != nil {
						return err
					}
					if p.CreatedAt > sinceMs {
						sinceMs = p.CreatedAt
					}
				}
				if time.Now().After(deadline) {
					return nil
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(2 * time.Second):
				}
			}
		},
	}

	cmd.Flags().DurationVar(&duration, "duration", 60*time.Second, "How long to watch before exiting")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}

func emitWatchEvent(cmd *cobra.Command, ev watchEvent, flags *rootFlags) error {
	w := cmd.OutOrStdout()
	if flags.asJSON {
		// JSONL stream — single-line JSON per event.
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(b))
		return nil
	}
	ts := time.UnixMilli(ev.Timestamp).Format(time.RFC3339)
	score := "-"
	if ev.ScoutScore != nil {
		score = fmt.Sprintf("%.1f", *ev.ScoutScore)
	}
	fmt.Fprintf(w, "%s %-8s %-30s %-30s score=%s\n",
		ts, ev.Decision, ev.ImpressionID, ev.Winner, score)
	return nil
}
