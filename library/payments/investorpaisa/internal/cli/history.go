// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/cliutil"
	"github.com/spf13/cobra"
)

// The per-stock page embeds its data in the Next.js server-component payload
// (self.__next_f.push chunks). There is no JSON endpoint for price history, so
// this command fetches the page with the session cookie and lifts the
// "initialView" object out of that payload.

type ipChartView struct {
	Symbol      string            `json:"symbol"`
	Name        string            `json:"name"`
	Sector      string            `json:"sector"`
	D           []string          `json:"d"`
	O           []float64         `json:"o"`
	H           []float64         `json:"h"`
	L           []float64         `json:"l"`
	C           []float64         `json:"c"`
	V           []float64         `json:"v"`
	Fac         []float64         `json:"fac"`
	Adjustments []json.RawMessage `json:"adjustments"`
	Events      []json.RawMessage `json:"events"`
	AsOf        string            `json:"asOf"`
}

type historyBar struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type historyResult struct {
	Symbol      string            `json:"symbol"`
	Name        string            `json:"name"`
	Sector      string            `json:"sector"`
	AsOf        string            `json:"as_of"`
	Source      string            `json:"source"`
	Adjusted    bool              `json:"adjusted"`
	Bars        int               `json:"bars"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Adjustments []json.RawMessage `json:"adjustments"`
	Events      []json.RawMessage `json:"events"`
	Rows        []historyBar      `json:"rows"`
}

var flightChunkRe = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)

// parseChartHTML extracts the initialView object from a /chart/<SYMBOL> page.
func parseChartHTML(page []byte) (*ipChartView, error) {
	var sb strings.Builder
	for _, m := range flightChunkRe.FindAllSubmatch(page, -1) {
		var chunk string
		if err := json.Unmarshal([]byte(`"`+string(m[1])+`"`), &chunk); err != nil {
			continue
		}
		sb.WriteString(chunk)
	}
	flight := sb.String()
	idx := strings.Index(flight, `"initialView":`)
	if idx < 0 {
		return nil, fmt.Errorf("page has no embedded chart data (signed out or the site layout changed)")
	}
	start := strings.IndexByte(flight[idx:], '{')
	if start < 0 {
		return nil, fmt.Errorf("embedded chart data is malformed")
	}
	obj, ok := balancedObject(flight[idx+start:])
	if !ok {
		return nil, fmt.Errorf("embedded chart data is truncated")
	}
	var v ipChartView
	if err := json.Unmarshal([]byte(obj), &v); err != nil {
		return nil, fmt.Errorf("decoding embedded chart data: %w", err)
	}
	n := len(v.D)
	if n == 0 || len(v.O) != n || len(v.H) != n || len(v.L) != n || len(v.C) != n || len(v.V) != n {
		return nil, fmt.Errorf("embedded chart arrays are inconsistent (dates=%d close=%d)", n, len(v.C))
	}
	return &v, nil
}

// balancedObject returns the leading {...} object of s, honouring JSON strings.
func balancedObject(s string) (string, bool) {
	depth, inStr, esc := 0, false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

// buildHistory turns the columnar view into bars, optionally split/bonus
// adjusted (prices times the factor, volume divided by it) and trimmed to the
// most recent years.
func buildHistory(v *ipChartView, years int, raw bool, source string) historyResult {
	res := historyResult{Symbol: v.Symbol, Name: v.Name, Sector: v.Sector, AsOf: v.AsOf, Source: source, Adjusted: !raw, Adjustments: v.Adjustments, Events: v.Events}
	if res.Adjustments == nil {
		res.Adjustments = []json.RawMessage{}
	}
	if res.Events == nil {
		res.Events = []json.RawMessage{}
	}
	cutoff := ""
	if years > 0 && len(v.D) > 0 {
		if last, err := time.Parse("2006-01-02", v.D[len(v.D)-1]); err == nil {
			cutoff = last.AddDate(-years, 0, 0).Format("2006-01-02")
		}
	}
	res.Rows = make([]historyBar, 0, len(v.D))
	for i, d := range v.D {
		if cutoff != "" && d < cutoff {
			continue
		}
		f := 1.0
		if !raw && i < len(v.Fac) && v.Fac[i] > 0 {
			f = v.Fac[i]
		}
		res.Rows = append(res.Rows, historyBar{
			Date: d, Open: ipRound(v.O[i]*f, 2), High: ipRound(v.H[i]*f, 2), Low: ipRound(v.L[i]*f, 2),
			Close: ipRound(v.C[i]*f, 2), Volume: ipRound(v.V[i]/f, 0),
		})
	}
	res.Bars = len(res.Rows)
	if res.Bars > 0 {
		res.From, res.To = res.Rows[0].Date, res.Rows[res.Bars-1].Date
	}
	return res
}

func fetchChartPage(ctx context.Context, flags *rootFlags, sym string) ([]byte, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.GetWithHeadersNoCache(ctx, "/chart/"+url.PathEscape(sym), nil, map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		return nil, classifyAPIErrorOnly(err)
	}
	return data, nil
}

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var years, tail int
	var raw bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "history [symbol]",
		Short: "Five years of daily OHLCV, split/bonus adjustment factors and corporate events for a stock, cached locally.",
		Long: `Daily OHLCV for a Nifty 200 stock, adjusted for splits and bonuses (use --raw for unadjusted prices), plus the
adjustment events the site applied. The data is read from the stock's chart page and cached in the local store, so
--data-source local replays it offline.

Use this for a stock's 5-year daily OHLCV, split/bonus adjustment factors and corporate events.
Do NOT use it for current metrics or headlines; use 'quote' instead.
Do NOT use it to find stocks matching a pattern; use 'screen' instead.`,
		Example: strings.Trim(`
  investorpaisa-pp-cli history TCS --years 5 --agent
  investorpaisa-pp-cli history RELIANCE --years 1 --raw --agent --select symbol,from,to,rows.date,rows.close`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "symbol=TCS;--years=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "history")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("exactly one symbol is required, e.g. history TCS"))
			}
			if years < 0 {
				return usageErr(fmt.Errorf("--years must not be negative"))
			}
			sym := ipNormalizeSymbol(args[0])
			if !ipValidSymbol(sym) {
				return usageErr(fmt.Errorf("%q is not a valid NSE symbol (letters, digits, & _ . - up to 20 characters)", args[0]))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := ipOpenStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			kind := "history:" + sym
			var view *ipChartView
			source := "live"
			var liveErr error
			if flags.dataSource != "local" {
				page, ferr := fetchChartPage(ctx, flags, sym)
				if ferr == nil {
					v, perr := parseChartHTML(page)
					if perr != nil {
						ferr = notFoundErr(fmt.Errorf("no chart data for %s (%v): it may not be a Nifty 200 symbol, or your session expired; run 'investorpaisa-pp-cli auth login --chrome'", sym, perr))
					} else {
						view = v
						if body, merr := json.Marshal(v); merr == nil {
							if serr := ipSaveSnapshot(ctx, db, kind, "latest", body); serr != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not cache history: %v\n", serr)
							}
						}
					}
				}
				if ferr != nil {
					if flags.dataSource == "live" {
						return ferr
					}
					liveErr = ferr
				}
			}
			if view == nil {
				snaps, serr := ipLatestSnapshots(ctx, db, kind, 1)
				if serr != nil {
					return serr
				}
				if len(snaps) == 0 {
					if liveErr != nil {
						return liveErr
					}
					return notFoundErr(fmt.Errorf("no cached history for %s; fetch it once while signed in", sym))
				}
				if liveErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live fetch failed (%v); using the cached history\n", liveErr)
				}
				var v ipChartView
				if err := json.Unmarshal(snaps[0].Body, &v); err != nil {
					return fmt.Errorf("cached history for %s is unreadable: %w", sym, err)
				}
				view, source = &v, "local"
			}
			res := buildHistory(view, years, raw, source)
			if tail > 0 && len(res.Rows) > tail {
				res.Rows = res.Rows[len(res.Rows)-tail:]
				res.Bars, res.From, res.To = len(res.Rows), res.Rows[0].Date, res.Rows[len(res.Rows)-1].Date
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %d bars %s..%s (%s, adjusted=%v)\n", cliutil.ScrubTerminal(res.Symbol), cliutil.ScrubTerminal(res.Name), res.Bars, res.From, res.To, res.Source, res.Adjusted)
				items := make([]map[string]any, 0)
				start := 0
				if len(res.Rows) > 10 {
					start = len(res.Rows) - 10
				}
				for _, r := range res.Rows[start:] {
					items = append(items, map[string]any{"date": r.Date, "open": r.Open, "high": r.High, "low": r.Low, "close": r.Close, "volume": r.Volume})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().IntVar(&years, "years", 5, "Years of history to return (0 for everything the site has)")
	cmd.Flags().IntVar(&tail, "tail", 0, "Return only the most recent N bars")
	cmd.Flags().BoolVar(&raw, "raw", false, "Return unadjusted prices")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store")
	return cmd
}
