package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/store"
	"github.com/spf13/cobra"
)

func newBlocksFoundCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "found",
		Short: "Detect if blocks arrived in time window — exit 0=found, exit 2=none",
		Long:  "Queries block_events SQLite table for blocks in the --since window. Exit 0 when at least one block was found; exit 2 when none. Use in n8n to alert when no blocks arrive in the expected window.",
		Example: `  dogecoin-pp-cli blocks found --since 7d
  dogecoin-pp-cli blocks found --since 24h --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"source":"block_events","exit_0":"blocks_found","exit_2":"none_found"}`)
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

			evs, err := s.BlocksSince(cmd.Context(), cutoff)
			if err != nil {
				return fmt.Errorf("querying block events: %w", err)
			}

			type blockRow struct {
				Timestamp   string  `json:"timestamp"`
				BlockHeight int64   `json:"block_height"`
				BlockHash   string  `json:"block_hash"`
				Difficulty  float64 `json:"difficulty"`
			}
			rows := make([]blockRow, 0, len(evs))
			for _, ev := range evs {
				rows = append(rows, blockRow{
					Timestamp:   time.Unix(ev.TS, 0).UTC().Format(time.RFC3339),
					BlockHeight: ev.BlockHeight,
					BlockHash:   ev.BlockHash,
					Difficulty:  ev.Difficulty,
				})
			}

			result := map[string]any{
				"found":  len(evs) > 0,
				"count":  len(evs),
				"since":  since,
				"blocks": rows,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			if len(evs) == 0 {
				fmt.Fprintf(os.Stderr, "no blocks found in window %s\n", since)
				exitWithCode(2)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window to check (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newBlocksLogCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "log",
		Short: "List recent block arrivals from local store",
		Long:  "Shows block_events from the local SQLite store for the given time window. Requires 'dogecoin-pp-cli sync' to populate the store.",
		Example: `  dogecoin-pp-cli blocks log --since 7d --json
  dogecoin-pp-cli blocks log --since 24h --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"source":"block_events"}`)
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

			evs, err := s.BlocksSince(cmd.Context(), cutoff)
			if err != nil {
				return fmt.Errorf("querying block events: %w", err)
			}

			type blockRow struct {
				Timestamp   string  `json:"timestamp"`
				BlockHeight int64   `json:"block_height"`
				BlockHash   string  `json:"block_hash"`
				TxCount     int64   `json:"tx_count"`
				Difficulty  float64 `json:"difficulty"`
			}
			rows := make([]blockRow, 0, len(evs))
			for _, ev := range evs {
				rows = append(rows, blockRow{
					Timestamp:   time.Unix(ev.TS, 0).UTC().Format(time.RFC3339),
					BlockHeight: ev.BlockHeight,
					BlockHash:   ev.BlockHash,
					TxCount:     ev.TxCount,
					Difficulty:  ev.Difficulty,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window to query (e.g. 7d, 24h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

// exitWithCode exits with a specific code. Defined here to avoid os.Exit calls scattered across files.
func exitWithCode(code int) {
	os.Exit(code)
}
