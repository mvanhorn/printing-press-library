package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type boardGap struct {
	Keyword      string `json:"keyword"`
	TrendRank    int    `json:"trend_rank"`
	InYourBoards bool   `json:"covered_in_boards"`
	MatchedBoard string `json:"matched_board,omitempty"`
}

func newNovelBoardsGapCmd(flags *rootFlags) *cobra.Command {
	var region string
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "gap",
		Short: "Find trending Pinterest topics you haven't covered in your boards recently.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli boards gap --region US --json
  pinterest-pp-cli boards gap --days 14 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare trending topics in %s against your boards (last %d days)\n", region, days)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'pinterest-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Get trending keywords from local store
			trendRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.keyword'), json_extract(data, '$.rank')
				FROM resources
				WHERE resource_type = 'trending_keywords'
				ORDER BY CAST(json_extract(data, '$.rank') AS INTEGER) ASC
				LIMIT 50`)
			if err != nil {
				return fmt.Errorf("querying trending keywords: %w", err)
			}
			defer trendRows.Close()

			// Get board/pin keywords from local store
			boardRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT LOWER(json_extract(data, '$.name') || ' ' || COALESCE(json_extract(data, '$.description'), ''))
				FROM resources
				WHERE resource_type = 'boards'`)
			if err != nil {
				return fmt.Errorf("querying boards: %w", err)
			}
			defer boardRows.Close()

			var boardTexts []string
			for boardRows.Next() {
				var txt string
				if err := boardRows.Scan(&txt); err == nil {
					boardTexts = append(boardTexts, txt)
				}
			}

			var results []boardGap
			rank := 0
			for trendRows.Next() {
				rank++
				var keyword string
				var trendRank int
				if err := trendRows.Scan(&keyword, &trendRank); err != nil {
					trendRank = rank
					keyword = fmt.Sprintf("keyword_%d", rank)
				}
				kw := strings.ToLower(keyword)
				gap := boardGap{Keyword: keyword, TrendRank: trendRank}
				for _, bt := range boardTexts {
					if strings.Contains(bt, kw) {
						gap.InYourBoards = true
						gap.MatchedBoard = bt
						break
					}
				}
				if !gap.InYourBoards {
					results = append(results, gap)
				}
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No trending topic gaps found, or trending data not synced. Run 'pinterest-pp-cli sync' first.")
				return nil
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&region, "region", "US", "Region for trending topics (e.g. US, GB, AU)")
	cmd.Flags().IntVar(&days, "days", 14, "Look back N days for recent coverage")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
