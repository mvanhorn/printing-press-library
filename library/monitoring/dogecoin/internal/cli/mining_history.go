package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/store"
	"github.com/spf13/cobra"
)

func newMiningHistoryCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Query historical hashrate and difficulty from local SQLite store",
		Long:  "Returns time-series data from local mining_snapshots. Requires 'dogecoin-pp-cli sync' to have run first. Use --since to limit the window.",
		Example: `  dogecoin-pp-cli mining history --since 30d --json
  dogecoin-pp-cli mining history --since 7d --agent
  dogecoin-pp-cli mining history --since 24h --select timestamp,hashrate_net_ths`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"source":"mining_snapshots","since":"`+since+`"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			cutoff, err := store.ParseSince(since)
			if err != nil {
				return err
			}

			snaps, err := s.SnapshotsSince(cmd.Context(), cutoff)
			if err != nil {
				return fmt.Errorf("querying snapshots: %w", err)
			}

			type row struct {
				Timestamp      int64   `json:"timestamp"`
				TimestampISO   string  `json:"timestamp_iso"`
				BlockHeight    int64   `json:"block_height"`
				Difficulty     float64 `json:"difficulty"`
				HashrateNetHPS float64 `json:"hashrate_net_hps"`
				HashrateNetTHS float64 `json:"hashrate_net_ths"`
				PeerCount      int64   `json:"peer_count"`
			}

			rows := make([]row, 0, len(snaps))
			for _, snap := range snaps {
				rows = append(rows, row{
					Timestamp:      snap.TS,
					TimestampISO:   time.Unix(snap.TS, 0).UTC().Format(time.RFC3339),
					BlockHeight:    snap.BlockHeight,
					Difficulty:     snap.Difficulty,
					HashrateNetHPS: snap.HashrateNet,
					HashrateNetTHS: snap.HashrateNet / 1e12,
					PeerCount:      snap.PeerCount,
				})
			}

			// Warn when the store covers significantly less history than requested,
			// so users don't silently receive a thin dataset.
			if len(rows) > 0 && cutoff > 0 {
				oldest := rows[0].Timestamp
				requestedSpanSec := time.Now().Unix() - cutoff
				actualSpanSec := time.Now().Unix() - oldest
				if requestedSpanSec > 0 && actualSpanSec < requestedSpanSec/2 {
					coverage := time.Duration(actualSpanSec) * time.Second
					h := int(coverage.Hours())
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sparse_history","store_coverage_hours":%d,"hint":"run sync more often for longer history"}`+"\n", h)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Time window (e.g. 30d, 7d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newMiningTrendCmd(flags *rootFlags) *cobra.Command {
	var window string
	var thresholdPct float64
	var dbPath string
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Detect difficulty trend — exit 5 on significant change",
		Long:  "Computes difficulty percent change over --window from local SQLite. Exit 0=stable, exit 5=spike or drop beyond threshold.",
		Example: `  dogecoin-pp-cli mining trend --window 24h
  dogecoin-pp-cli mining trend --window 7d --threshold 15 --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"source":"mining_snapshots","exit_5":"difficulty_spike"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			cutoff, err := store.ParseSince(window)
			if err != nil {
				return err
			}

			snaps, err := s.SnapshotsSince(cmd.Context(), cutoff)
			if err != nil {
				return fmt.Errorf("querying snapshots: %w", err)
			}

			if len(snaps) < 2 {
				result := map[string]any{
					"status":  "insufficient_data",
					"message": "need at least 2 snapshots in the window — run 'sync' more often",
					"count":   len(snaps),
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			first := snaps[0].Difficulty
			last := snaps[len(snaps)-1].Difficulty
			changePct := 0.0
			if first > 0 {
				changePct = (last - first) / first * 100
			}

			absChange := changePct
			if absChange < 0 {
				absChange = -absChange
			}

			result := map[string]any{
				"difficulty_start":    first,
				"difficulty_end":      last,
				"change_pct":          changePct,
				"threshold_pct":       thresholdPct,
				"window":              window,
				"snapshots_in_window": len(snaps),
				"alert":               absChange >= thresholdPct,
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}

			if absChange >= thresholdPct {
				fmt.Fprintf(cmd.ErrOrStderr(), "difficulty trend alert: %.1f%% change in %s\n", changePct, window)
				const exitCodeDifficultySpike = 5
				exitWithCode(exitCodeDifficultySpike)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&window, "window", "24h", "Time window to analyze (e.g. 24h, 7d)")
	cmd.Flags().Float64Var(&thresholdPct, "threshold", 10.0, "Percent change that triggers exit 5")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
