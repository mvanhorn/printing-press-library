package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search blocks and mining snapshots in local SQLite store",
		Long:  "Searches block_events and mining_snapshots by block hash prefix or block height. Requires 'dogecoin-pp-cli sync' to populate the store first.",
		Example: `  dogecoin-pp-cli search dfcd7863
  dogecoin-pp-cli search 6209408 --json
  dogecoin-pp-cli search dfcd --since 7d --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"source":"block_events,mining_snapshots"}`)
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

			cutoff := int64(0)
			if since != "" {
				cutoff, err = store.ParseSince(since)
				if err != nil {
					return err
				}
			}

			query := args[0]
			blocks, err := s.BlocksSince(cmd.Context(), cutoff)
			if err != nil {
				return fmt.Errorf("querying blocks: %w", err)
			}

			type blockMatch struct {
				Type        string  `json:"type"`
				Timestamp   string  `json:"timestamp"`
				BlockHeight int64   `json:"block_height"`
				BlockHash   string  `json:"block_hash,omitempty"`
				Difficulty  float64 `json:"difficulty,omitempty"`
			}

			var results []blockMatch
			for _, ev := range blocks {
				hashMatch := strings.HasPrefix(ev.BlockHash, query)
				heightMatch := fmt.Sprintf("%d", ev.BlockHeight) == query
				if hashMatch || heightMatch {
					results = append(results, blockMatch{
						Type:        "block_event",
						Timestamp:   time.Unix(ev.TS, 0).UTC().Format(time.RFC3339),
						BlockHeight: ev.BlockHeight,
						BlockHash:   ev.BlockHash,
						Difficulty:  ev.Difficulty,
					})
				}
			}

			result := map[string]any{
				"query":   query,
				"found":   len(results) > 0,
				"count":   len(results),
				"results": results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().StringVar(&since, "since", "", "Limit search to window (e.g. 7d, 24h)")
	return cmd
}
