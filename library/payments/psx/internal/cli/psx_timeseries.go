// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

// psx_timeseries.go replaces the generated timeseries endpoint commands with
// versions that honour the portal's own error signal. PSX answers an unknown
// symbol with HTTP 200 and {"status":0,"message":"Unknown symbol: X"}, so a
// client that only checks the HTTP status reports success on a failed lookup.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type tsPoint struct {
	Epoch  int64   `json:"epoch"`
	Time   string  `json:"time"`
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
	Open   float64 `json:"open,omitempty"`
}

func newTimeseriesLeaf(flags *rootFlags, kind string) *cobra.Command {
	var limit int
	short := "Intraday tick series for a symbol or index (last active session)"
	long := "Use this command for intraday ticks.\n" +
		"Do NOT use it for daily bars; use 'history' instead.\n" +
		"Tuples are [epoch, price, volume]. PSX serves the last active session, " +
		"so a thinly traded scrip can return stale-dated ticks."
	path := "/timeseries/int/{symbol}"
	if kind == "eod" {
		short = "End-of-day series for a symbol or index"
		long = "Use this command for the end-of-day close series.\n" +
			"Do NOT use it for full OHLC bars; use 'history' instead.\n" +
			"Tuples are [epoch, close, volume, open]."
		path = "/timeseries/eod/{symbol}"
	}
	cmd := &cobra.Command{
		Use:         kind + " <symbol>",
		Short:       short,
		Long:        long,
		Example:     "  psx-pp-cli timeseries " + kind + " OGDC --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "timeseries "+kind)
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC or KSE100"))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			// Substitute into the spec-declared template rather than
			// concatenating, so the path literal names the real endpoint and
			// the symbol is escaped before it reaches the URL.
			reqPath := strings.Replace(path, "{symbol}", url.PathEscape(sym), 1)
			tuples, err := fetchTuples(ctx, psxClient(flags), reqPath)
			if err != nil {
				// The portal reports an unknown symbol in its envelope message.
				if strings.Contains(strings.ToLower(err.Error()), "unknown symbol") {
					return notFoundErr(fmt.Errorf("no series for %q: %w", sym, err))
				}
				return err
			}
			// /timeseries/eod returns {"status":1,"data":[]} for an unknown symbol
			// — no error signal at all — while /timeseries/int reports it in the
			// envelope. Fall back to the instrument master so an unknown code is
			// a not-found rather than a silent empty success.
			if len(tuples) == 0 {
				known, kerr := symbolIsListed(ctx, psxClient(flags), sym)
				if kerr == nil && !known {
					return notFoundErr(fmt.Errorf("no listed instrument %q; check the code with 'psx-pp-cli symbols list'", sym))
				}
			}

			points := make([]tsPoint, 0, len(tuples))
			for _, t := range tuples {
				p := tsPoint{
					Epoch:  t.Epoch,
					Time:   time.Unix(t.Epoch, 0).UTC().Format(time.RFC3339),
					Price:  t.Value,
					Volume: t.Volume,
				}
				if t.HasOpen {
					p.Open = t.Open
				}
				points = append(points, p)
			}
			if limit > 0 && len(points) > limit {
				points = points[:limit]
			}
			view := struct {
				Symbol string    `json:"symbol"`
				Kind   string    `json:"kind"`
				Count  int       `json:"count"`
				Points []tsPoint `json:"points"`
			}{Symbol: sym, Kind: kind, Count: len(points), Points: points}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(points) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No %s points for %s.\n", kind, sym)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-22s %12s %16s\n", "TIME", "PRICE", "VOLUME")
			for _, p := range points {
				fmt.Fprintf(cmd.OutOrStdout(), "%-22s %12.2f %16.0f\n", cliutil.ScrubTerminal(p.Time), p.Price, p.Volume)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum points to return (0 = all)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		ts, _, err := root.Find([]string{"timeseries"})
		if err != nil || ts == nil {
			return
		}
		// Replace the generated leaves so unknown symbols exit non-zero.
		for _, kind := range []string{"intraday", "eod"} {
			for _, existing := range ts.Commands() {
				if existing.Name() == kind {
					ts.RemoveCommand(existing)
					break
				}
			}
			ts.AddCommand(newTimeseriesLeaf(flags, kind))
		}
	})
}
