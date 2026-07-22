// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
)

type kalshiMarketItem struct {
	Ticker         string  `json:"ticker"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	YesAskDollars  float64 `json:"yes_ask_dollars,omitempty"`
	NoAskDollars   float64 `json:"no_ask_dollars,omitempty"`
	Volume24h      float64 `json:"volume_24h_fp,omitempty"`
	ExpirationTime string  `json:"expiration_time,omitempty"`
}

func newKalshiMarketsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "markets", Short: "Kalshi markets"}
	cmd.AddCommand(newKalshiMarketsListCmd(flags))
	cmd.AddCommand(newKalshiMarketsGetCmd(flags))
	return cmd
}

func newKalshiMarketsListCmd(flags *rootFlags) *cobra.Command {
	var status, cursor, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Kalshi markets with prices, volume, and expiration, filterable by status",
		Example: `  prediction-goat-pp-cli kalshi markets list --data-source live --limit 10 --json
  prediction-goat-pp-cli kalshi markets list --status active`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			useLive := flags.dataSource == "live"
			if flags.dataSource == "auto" {
				count, err := kalshiLocalCount(cmd, dbPath, "kalshi_markets")
				if err != nil {
					return fmt.Errorf("kalshi markets local count: %w", err)
				}
				useLive = count == 0
			}
			var items []kalshiMarketItem
			var err error
			if useLive {
				items, err = liveKalshiMarkets(cmd, status, cursor, limit)
			} else {
				items, err = localKalshiMarkets(cmd, dbPath, status, limit)
			}
			if err != nil {
				return fmt.Errorf("kalshi markets list: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().StringVar(&status, "status", "open", "Market status: open, closed, settled")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newKalshiMarketsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <ticker>",
		Short:       "Get a Kalshi market by ticker",
		Example:     `  prediction-goat-pp-cli kalshi markets get KXMVESPORTSMULTIGAMEEXTENDED-FOO --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			body, err := kalshi.New().Get(cmd.Context(), "/markets/"+url.PathEscape(args[0]), url.Values{})
			if err != nil {
				return fmt.Errorf("kalshi markets get: %w", err)
			}
			obj, err := kalshiEnvelopeObject(body, "market")
			if err != nil {
				return fmt.Errorf("kalshi markets get decode: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), kalshiMarketSlim(obj), flags)
		},
	}
	return cmd
}

func liveKalshiMarkets(cmd *cobra.Command, status, cursor string, limit int) ([]kalshiMarketItem, error) {
	params := url.Values{"limit": {fmt.Sprint(limit)}, "status": {status}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	body, err := kalshi.New().Get(cmd.Context(), "/markets", params)
	if err != nil {
		return nil, err
	}
	var resp kalshi.MarketsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	items := make([]kalshiMarketItem, 0, len(resp.Markets))
	for _, raw := range resp.Markets {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) == nil {
			items = append(items, kalshiMarketSlim(obj))
		}
	}
	return items, nil
}

func localKalshiMarkets(cmd *cobra.Command, dbPath, status string, limit int) ([]kalshiMarketItem, error) {
	rows, err := kalshiLocalRows(cmd, dbPath, "kalshi_markets", `AND COALESCE(json_extract(data,'$.status'),'')=? ORDER BY updated_at DESC LIMIT ?`, status, limit)
	if err != nil {
		return nil, err
	}
	items := make([]kalshiMarketItem, 0, len(rows))
	for _, obj := range rows {
		items = append(items, kalshiMarketSlim(obj))
	}
	return items, nil
}

func kalshiMarketSlim(obj map[string]any) kalshiMarketItem {
	return kalshiMarketItem{Ticker: jsonString(obj, "ticker"), Title: jsonString(obj, "title"), Status: jsonString(obj, "status"), YesAskDollars: jsonFloat(obj, "yes_ask_dollars"), NoAskDollars: jsonFloat(obj, "no_ask_dollars"), Volume24h: jsonFloat(obj, "volume_24h_fp"), ExpirationTime: jsonString(obj, "expiration_time")}
}
