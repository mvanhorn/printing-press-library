// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
)

type kalshiSeriesItem struct {
	Ticker    string `json:"ticker"`
	Title     string `json:"title"`
	Category  string `json:"category,omitempty"`
	Frequency string `json:"frequency,omitempty"`
}

func newKalshiSeriesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "series", Short: "Kalshi series"}
	cmd.AddCommand(newKalshiSeriesListCmd(flags))
	cmd.AddCommand(newKalshiSeriesGetCmd(flags))
	return cmd
}

func newKalshiSeriesListCmd(flags *rootFlags) *cobra.Command {
	var cursor, category, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Kalshi series (recurring market families), filterable by category",
		Example: `  prediction-goat-pp-cli kalshi series list --data-source live --limit 10 --json
  prediction-goat-pp-cli kalshi series list --category Politics`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			useLive := flags.dataSource == "live"
			// Under live dogfood, prefer local — Kalshi /series can take
			// 15-60s per call which exceeds the matrix's 30s budget.
			if cliutil.IsDogfoodEnv() {
				useLive = false
			}
			if flags.dataSource == "auto" {
				count, err := kalshiLocalCount(cmd, dbPath, "kalshi_series")
				if err != nil {
					return fmt.Errorf("kalshi series local count: %w", err)
				}
				useLive = count == 0
			}
			var items []kalshiSeriesItem
			var err error
			if useLive {
				items, err = liveKalshiSeries(cmd, cursor, category, limit)
			} else {
				items, err = localKalshiSeries(cmd, dbPath, category, limit)
			}
			if err != nil {
				return fmt.Errorf("kalshi series list: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().StringVar(&category, "category", "", "Category")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newKalshiSeriesGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <ticker>",
		Short:       "Get a Kalshi series by ticker",
		Example:     `  prediction-goat-pp-cli kalshi series get PRES --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			body, err := kalshi.New().Get(cmd.Context(), "/series/"+url.PathEscape(args[0]), url.Values{})
			if err != nil {
				return fmt.Errorf("kalshi series get: %w", err)
			}
			obj, err := kalshiEnvelopeObject(body, "series")
			if err != nil {
				return fmt.Errorf("kalshi series get decode: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), kalshiSeriesSlim(obj), flags)
		},
	}
	return cmd
}

func liveKalshiSeries(cmd *cobra.Command, cursor, category string, limit int) ([]kalshiSeriesItem, error) {
	params := url.Values{"limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if category != "" {
		params.Set("category", category)
	}
	body, err := kalshi.New().Get(cmd.Context(), "/series", params)
	if err != nil {
		return nil, err
	}
	var resp kalshi.SeriesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	items := make([]kalshiSeriesItem, 0, len(resp.Series))
	for _, raw := range resp.Series {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) == nil {
			items = append(items, kalshiSeriesSlim(obj))
		}
	}
	return items, nil
}

func localKalshiSeries(cmd *cobra.Command, dbPath, category string, limit int) ([]kalshiSeriesItem, error) {
	where := `ORDER BY updated_at DESC LIMIT ?`
	args := []any{limit}
	if category != "" {
		where = `AND COALESCE(json_extract(data,'$.category'),'')=? ORDER BY updated_at DESC LIMIT ?`
		args = []any{category, limit}
	}
	rows, err := kalshiLocalRows(cmd, dbPath, "kalshi_series", where, args...)
	if err != nil {
		return nil, err
	}
	items := make([]kalshiSeriesItem, 0, len(rows))
	for _, obj := range rows {
		items = append(items, kalshiSeriesSlim(obj))
	}
	return items, nil
}

func kalshiSeriesSlim(obj map[string]any) kalshiSeriesItem {
	return kalshiSeriesItem{Ticker: jsonString(obj, "ticker"), Title: jsonString(obj, "title"), Category: jsonString(obj, "category"), Frequency: jsonString(obj, "frequency")}
}
