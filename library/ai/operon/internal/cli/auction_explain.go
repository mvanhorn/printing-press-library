// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

type rankingRow struct {
	Rank     int      `json:"rank"`
	Service  string   `json:"service"`
	Score    *float64 `json:"score,omitempty"`
	Bid      *float64 `json:"bid,omitempty"`
	Eligible bool     `json:"eligible"`
	Reason   string   `json:"reason,omitempty"`
}

func newAuctionExplainCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "explain <impression-id>",
		Short: "Render the ranking[] from a logged auction as a sorted table.",
		Long: `Pull a previously logged placement out of the local store and unpack the
stored auction object into a readable ranking table: rank, service, score,
bid, eligible (Y/N), and the per-candidate reason (when the auction layer
emitted one).

Reads from the local store only.`,
		Example: strings.Trim(`
  operon-pp-cli auction explain imp_a1b2c3d4e5f60718
  operon-pp-cli auction explain imp_a1b2c3d4e5f60718 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			impID := strings.TrimSpace(args[0])
			if impID == "" {
				return cmd.Help()
			}

			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would render ranking for: %s\n", impID)
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			p, err := st.GetPlacement(ctx, impID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf(
						"placement %q not found in local store\nhint: only placements logged via this CLI are searchable. Run 'operon-pp-cli sync' or replay a known impression to populate the log.",
						impID,
					))
				}
				return apiErr(err)
			}

			if p.AuctionJSON == "" {
				return notFoundErr(fmt.Errorf("placement %s has no stored auction", impID))
			}

			// --json: emit the raw auction object as the spec demands.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(p.AuctionJSON), flags)
			}

			var auction map[string]any
			if err := json.Unmarshal([]byte(p.AuctionJSON), &auction); err != nil {
				return apiErr(fmt.Errorf("parsing stored auction: %w", err))
			}

			rawRanking, _ := auction["ranking"].([]any)
			rows := make([]rankingRow, 0, len(rawRanking))
			for i, r := range rawRanking {
				entry, ok := r.(map[string]any)
				if !ok {
					continue
				}
				row := rankingRow{Rank: i + 1}
				if v, ok := entry["service"].(string); ok {
					row.Service = v
				}
				if v, ok := entry["score"].(float64); ok {
					f := v
					row.Score = &f
				}
				if v, ok := entry["bid"].(float64); ok {
					f := v
					row.Bid = &f
				}
				if v, ok := entry["eligible"].(bool); ok {
					row.Eligible = v
				}
				if v, ok := entry["reason"].(string); ok {
					row.Reason = v
				}
				rows = append(rows, row)
			}

			if flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"rank", "service", "score", "bid", "eligible", "reason"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				score, bid := "-", "-"
				if r.Score != nil {
					score = fmt.Sprintf("%.2f", *r.Score)
				}
				if r.Bid != nil {
					bid = fmt.Sprintf("%.2f", *r.Bid)
				}
				elig := "N"
				if r.Eligible {
					elig = "Y"
				}
				tableRows = append(tableRows, []string{
					fmt.Sprintf("%d", r.Rank),
					r.Service,
					score,
					bid,
					elig,
					r.Reason,
				})
			}
			if len(tableRows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "auction for %s has no ranking rows\n", impID)
				return nil
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}
