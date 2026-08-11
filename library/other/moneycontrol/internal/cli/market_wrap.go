// Copyright 2026 abhirup-dev and contributors. Licensed under Apache-2.0.
// Hand-authored novel command: market-wrap.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/moneycontrol/internal/client"
)

// newNovelMarketWrapCmd builds the `market-wrap` compound command: SENSEX +
// NIFTY quotes, top gainers, top losers, and the latest market headlines in a
// single envelope. Each leg is fetched concurrently; partial failures are
// preserved per the parallel-fetch rule.
func newNovelMarketWrapCmd(flags *rootFlags) *cobra.Command {
	var gainersLimit int
	var losersLimit int
	var headlinesLimit int

	cmd := &cobra.Command{
		Use:   "market-wrap",
		Short: "End-of-day market snapshot: indices, top gainers, top losers, and latest market headlines.",
		Long:  "End-of-day market snapshot: indices, top gainers, top losers, and latest market headlines.",
		Example: `  moneycontrol-pp-cli market-wrap
  moneycontrol-pp-cli market-wrap --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market-wrap")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			wwwClient, err := flags.newClient()
			if err != nil {
				return err
			}
			priceClient, err := newPriceAPIClient(flags)
			if err != nil {
				return err
			}

			type leg struct {
				key string
				fn  func() (any, error)
			}
			legs := []leg{
				{"indices_sensex", func() (any, error) { return fetchIndexQuote(ctx, priceClient, "in;SEN") }},
				{"indices_nifty", func() (any, error) { return fetchIndexQuote(ctx, priceClient, "in;NSX") }},
				{"top_gainers", func() (any, error) { return fetchScreenRaw(ctx, wwwClient, "topGainers", gainersLimit) }},
				{"top_losers", func() (any, error) { return fetchScreenRaw(ctx, wwwClient, "topLosers", losersLimit) }},
				{"headlines", func() (any, error) {
					return fetchNewsLinks(ctx, wwwClient, "/news/business/markets/", headlinesLimit)
				}},
			}

			type result struct {
				key   string
				value any
				err   error
			}
			results := make(chan result, len(legs))
			var wg sync.WaitGroup
			for _, l := range legs {
				wg.Add(1)
				go func(l leg) {
					defer wg.Done()
					v, err := l.fn()
					results <- result{key: l.key, value: v, err: err}
				}(l)
			}
			wg.Wait()
			close(results)

			view := struct {
				Indices struct {
					Sensex json.RawMessage `json:"sensex,omitempty"`
					Nifty  json.RawMessage `json:"nifty,omitempty"`
				} `json:"indices"`
				TopGainers json.RawMessage `json:"top_gainers,omitempty"`
				TopLosers  json.RawMessage `json:"top_losers,omitempty"`
				Headlines  []articleLink   `json:"headlines"`
				Errors     map[string]string `json:"errors,omitempty"`
			}{}
			errs := map[string]string{}
			hadErr := false
			for r := range results {
				if r.err != nil {
					errs[r.key] = r.err.Error()
					hadErr = true
					continue
				}
				switch r.key {
				case "indices_sensex":
					view.Indices.Sensex = r.value.(json.RawMessage)
				case "indices_nifty":
					view.Indices.Nifty = r.value.(json.RawMessage)
				case "top_gainers":
					view.TopGainers = r.value.(json.RawMessage)
				case "top_losers":
					view.TopLosers = r.value.(json.RawMessage)
				case "headlines":
					if h, ok := r.value.([]articleLink); ok {
						view.Headlines = h
					}
				}
			}
			view.Errors = errs
			if hadErr && len(errs) == len(legs) {
				return fmt.Errorf("all market-wrap legs failed: %v", errs)
			}
			if hadErr {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d market-wrap legs failed; partial results returned\n", len(errs), len(legs))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			// Human output
			printMarketWrapHuman(cmd, view)
			return nil
		},
	}
	cmd.Flags().IntVar(&gainersLimit, "gainers-limit", 5, "max top gainers to return")
	cmd.Flags().IntVar(&losersLimit, "losers-limit", 5, "max top losers to return")
	cmd.Flags().IntVar(&headlinesLimit, "headlines-limit", 8, "max market headlines to return")
	return cmd
}

// fetchIndexQuote returns the raw `data` block of an index pricefeed.
// The index key shape is "in;SEN" (literally containing a semicolon), which
// moneycontrol's edge treats as a path-parameter delimiter — so we percent-encode
// the semicolon to %3B before placing the key in the path. The client does
// string-concat + http.NewRequest, which preserves a pre-encoded %3B on the wire
// (it does not re-encode an already-valid percent-escape).
func fetchIndexQuote(ctx context.Context, c *client.Client, key string) (json.RawMessage, error) {
	encoded := strings.ReplaceAll(key, ";", "%3B")
	path := "/pricefeed/notapplicable/inidicesindia/" + encoded
	var wrap struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := fetchJSONWithClient(ctx, c, path, &wrap); err != nil {
		return nil, err
	}
	if wrap.Code != "200" {
		return nil, fmt.Errorf("index %s: api code %s", key, wrap.Code)
	}
	return wrap.Data, nil
}

// fetchScreenRaw fetches one market-action screen widget and returns its raw
// JSON bytes. The screen widget response shape varies, so we return it
// verbatim for the caller to render.
func fetchScreenRaw(ctx context.Context, c *client.Client, screen string, limit int) (json.RawMessage, error) {
	path := "/mc/widget/stockaction/" + screen + "?classic=true"
	raw, err := c.Get(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching screen %s: %w", screen, err)
	}
	// Best-effort truncate to `limit` rows if the payload is a JSON array.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		if limit > 0 && len(arr) > limit {
			arr = arr[:limit]
		}
		out, _ := json.Marshal(arr)
		return out, nil
	}
	// Otherwise return as-is (may be an object wrapper).
	return raw, nil
}

// printMarketWrapHuman renders the market-wrap for a terminal. It tolerates
// missing legs (empty raw messages) without crashing.
func printMarketWrapHuman(cmd *cobra.Command, view struct {
	Indices struct {
		Sensex json.RawMessage `json:"sensex,omitempty"`
		Nifty  json.RawMessage `json:"nifty,omitempty"`
	} `json:"indices"`
	TopGainers json.RawMessage `json:"top_gainers,omitempty"`
	TopLosers  json.RawMessage `json:"top_losers,omitempty"`
	Headlines  []articleLink   `json:"headlines"`
	Errors     map[string]string `json:"errors,omitempty"`
}) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "MARKET WRAP")
	fmt.Fprintln(out, "-----------")
	if len(view.Indices.Sensex) > 0 && string(view.Indices.Sensex) != "null" {
		fmt.Fprintf(out, "SENSEX: %s\n", compactJSONOneLine(view.Indices.Sensex))
	}
	if len(view.Indices.Nifty) > 0 && string(view.Indices.Nifty) != "null" {
		fmt.Fprintf(out, "NIFTY:  %s\n", compactJSONOneLine(view.Indices.Nifty))
	}
	fmt.Fprintf(out, "\nTop Gainers (%d bytes):\n", len(view.TopGainers))
	fmt.Fprintln(out, "Top Losers:")
	fmt.Fprintf(out, "\nHeadlines (%d):\n", len(view.Headlines))
	for i, h := range view.Headlines {
		fmt.Fprintf(out, "  %d. %s\n     %s\n", i+1, h.Title, h.URL)
	}
}

// compactJSONOneLine returns a compacted one-line JSON representation for
// terse human output. Returns the input string on error.
func compactJSONOneLine(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(out)
}
