// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
)

type kalshiEventItem struct {
	EventTicker       string `json:"event_ticker"`
	SeriesTicker      string `json:"series_ticker,omitempty"`
	Title             string `json:"title"`
	Category          string `json:"category,omitempty"`
	MutuallyExclusive bool   `json:"mutually_exclusive"`
}

func newKalshiEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "events", Short: "Kalshi events"}
	cmd.AddCommand(newKalshiEventsListCmd(flags))
	cmd.AddCommand(newKalshiEventsGetCmd(flags))
	return cmd
}

func newKalshiEventsListCmd(flags *rootFlags) *cobra.Command {
	var cursor, dbPath, series string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Kalshi event groups, filterable by series and synced locally",
		Example: `  prediction-goat-pp-cli kalshi events list --data-source live --limit 10 --json
  prediction-goat-pp-cli kalshi events list --series KXMENWORLDCUP`,
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
				count, err := kalshiLocalCount(cmd, dbPath, "kalshi_events")
				if err != nil {
					return fmt.Errorf("kalshi events local count: %w", err)
				}
				// --series filtering only makes sense against the live API
				// (the local store doesn't index by series). Force-live when
				// the user asked for a series filter so we don't silently
				// return unrelated locally-cached events.
				useLive = count == 0 || series != ""
			}
			var items []kalshiEventItem
			var err error
			if useLive {
				items, err = liveKalshiEvents(cmd, cursor, limit, series)
			} else {
				items, err = localKalshiEvents(cmd, dbPath, limit)
			}
			if err != nil {
				return fmt.Errorf("kalshi events list: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().StringVar(&series, "series", "", "Filter to events under a series ticker (forces --data-source live)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newKalshiEventsGetCmd(flags *rootFlags) *cobra.Command {
	var withMarkets bool
	cmd := &cobra.Command{
		Use:   "get <event_ticker>",
		Short: "Get a Kalshi event by ticker",
		Example: `  prediction-goat-pp-cli kalshi events get KXELONMARS-99 --json
  prediction-goat-pp-cli kalshi events get KXMENWORLDCUP-26 --with-markets --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			params := url.Values{}
			if withMarkets {
				params.Set("with_nested_markets", "true")
			}
			body, err := kalshi.New().Get(cmd.Context(), "/events/"+url.PathEscape(args[0]), params)
			if err != nil {
				return fmt.Errorf("kalshi events get: %w", err)
			}
			obj, err := kalshiEnvelopeObject(body, "event")
			if err != nil {
				return fmt.Errorf("kalshi events get decode: %w", err)
			}
			if withMarkets {
				return printJSONFiltered(cmd.OutOrStdout(), kalshiEventWithMarkets(obj), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), kalshiEventSlim(obj), flags)
		},
	}
	cmd.Flags().BoolVar(&withMarkets, "with-markets", false, "Include nested markets in the response (passes with_nested_markets=true upstream)")
	return cmd
}

// kalshiEventNestedMarket projects each market row from the upstream
// with_nested_markets response into a slim shape. Keeps the same field
// set humans and agents already see from `kalshi markets get`, plus
// the trading fields that make this the one-call answer for "what are
// all the markets under this event."
type kalshiEventNestedMarket struct {
	Ticker         string  `json:"ticker"`
	EventTicker    string  `json:"event_ticker,omitempty"`
	Title          string  `json:"title"`
	YesSubTitle    string  `json:"yes_sub_title,omitempty"`
	Status         string  `json:"status,omitempty"`
	YesAskDollars  float64 `json:"yes_ask_dollars,omitempty"`
	NoAskDollars   float64 `json:"no_ask_dollars,omitempty"`
	Volume24hFP    float64 `json:"volume_24h_fp,omitempty"`
	ExpirationTime string  `json:"expiration_time,omitempty"`
}

type kalshiEventWithMarketsItem struct {
	kalshiEventItem
	Markets []kalshiEventNestedMarket `json:"markets,omitempty"`
}

func kalshiEventWithMarkets(obj map[string]any) kalshiEventWithMarketsItem {
	out := kalshiEventWithMarketsItem{kalshiEventItem: kalshiEventSlim(obj)}
	raw, ok := obj["markets"].([]any)
	if !ok {
		return out
	}
	out.Markets = make([]kalshiEventNestedMarket, 0, len(raw))
	for _, m := range raw {
		mObj, ok := m.(map[string]any)
		if !ok {
			continue
		}
		out.Markets = append(out.Markets, kalshiEventNestedMarket{
			Ticker:         jsonString(mObj, "ticker"),
			EventTicker:    jsonString(mObj, "event_ticker"),
			Title:          jsonString(mObj, "title"),
			YesSubTitle:    jsonString(mObj, "yes_sub_title"),
			Status:         jsonString(mObj, "status"),
			YesAskDollars:  jsonFloat(mObj, "yes_ask_dollars"),
			NoAskDollars:   jsonFloat(mObj, "no_ask_dollars"),
			Volume24hFP:    jsonFloat(mObj, "volume_24h_fp"),
			ExpirationTime: jsonString(mObj, "expiration_time"),
		})
	}
	return out
}

func liveKalshiEvents(cmd *cobra.Command, cursor string, limit int, series string) ([]kalshiEventItem, error) {
	params := url.Values{"limit": {fmt.Sprint(limit)}}
	if cursor != "" {
		params.Set("cursor", cursor)
	}
	if series != "" {
		params.Set("series_ticker", series)
	}
	body, err := kalshi.New().Get(cmd.Context(), "/events", params)
	if err != nil {
		return nil, err
	}
	var resp kalshi.EventsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	items := make([]kalshiEventItem, 0, len(resp.Events))
	for _, raw := range resp.Events {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) == nil {
			items = append(items, kalshiEventSlim(obj))
		}
	}
	return items, nil
}

func localKalshiEvents(cmd *cobra.Command, dbPath string, limit int) ([]kalshiEventItem, error) {
	rows, err := kalshiLocalRows(cmd, dbPath, "kalshi_events", `ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	items := make([]kalshiEventItem, 0, len(rows))
	for _, obj := range rows {
		items = append(items, kalshiEventSlim(obj))
	}
	return items, nil
}

func kalshiEventSlim(obj map[string]any) kalshiEventItem {
	return kalshiEventItem{EventTicker: jsonString(obj, "event_ticker"), SeriesTicker: jsonString(obj, "series_ticker"), Title: jsonString(obj, "title"), Category: jsonString(obj, "category"), MutuallyExclusive: jsonString(obj, "mutually_exclusive") == "true"}
}
