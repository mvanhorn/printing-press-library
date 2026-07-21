// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// watchlists quotes — join a watchlist's members to live quotes in one command.
// The MCP exposes get_watchlist_items and get_equity_quotes separately; this
// does the two-call join a trader does every time they glance at a list.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/client"

	"github.com/spf13/cobra"
)

func newWatchlistsQuotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "quotes <list-id>",
		Short:       "Live quotes for every stock in a watchlist (items + quotes in one call)",
		Example:     "  robinhood-agentic-pp-cli watchlists quotes 11111111-2222-3333-4444-555555555555 --csv",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing <list-id>; usage: %s <list-id>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			listID := args[0]
			symbols, err := watchlistStockSymbols(cmd.Context(), c, listID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(symbols) == 0 {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"list_id": listID, "quotes": []any{}, "note": "watchlist has no stock instruments"}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "watchlist has no stock instruments")
				return nil
			}
			quotesRaw, err := c.Get(cmd.Context(), "/tools/get_equity_quotes", map[string]string{"symbols": strings.Join(symbols, ",")}) // pp:client-call
			if err != nil {
				return classifyAPIError(err, flags)
			}
			quotes := fetchArrayFromRaw(quotesRaw, "results")
			result := map[string]any{"list_id": listID, "symbols": symbols, "quotes": quotes}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// watchlistStockSymbols fetches a watchlist's instrument-type members and
// returns their symbols (crypto pairs and indexes are not equity-quotable).
func watchlistStockSymbols(ctx context.Context, c *client.Client, listID string) ([]string, error) {
	raw, err := c.Get(ctx, "/tools/get_watchlist_items", map[string]string{"list_id": listID}) // pp:client-call
	if err != nil {
		return nil, err
	}
	items := fetchArrayFromRaw(raw, "items")
	var symbols []string
	for _, it := range items {
		objType, _ := it["object_type"].(string)
		sym, _ := it["symbol"].(string)
		if sym != "" && (objType == "" || objType == "instrument") {
			symbols = append(symbols, sym)
		}
	}
	return symbols, nil
}

// fetchArrayFromRaw extracts data.<key> as a slice of objects from a {data:{...}}
// envelope, returning nil on any mismatch.
func fetchArrayFromRaw(raw json.RawMessage, key string) []map[string]any {
	var env map[string]json.RawMessage
	if json.Unmarshal(raw, &env) != nil {
		return nil
	}
	data, ok := env["data"]
	if !ok {
		return nil
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(data, &inner) != nil {
		return nil
	}
	arrRaw, ok := inner[key]
	if !ok {
		return nil
	}
	var arr []map[string]any
	_ = json.Unmarshal(arrRaw, &arr)
	return arr
}
