// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: clustered congressional buying. Groups local congressional
// equity trades by ticker and flags symbols with >=N distinct members buying in
// a window — distinct-buyer cluster logic the per-row endpoints do not provide.
//
// Scope note: the syncable SEC insider-transactions ("sec") resource is the
// owners view and carries no ticker, so it cannot be clustered by symbol. This
// command therefore clusters congressional disclosures (the ticker-bearing
// source). Congressional trade tracking is a flagship Benzinga data product.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type clusterRow struct {
	Ticker         string   `json:"ticker"`
	DistinctBuyers int      `json:"distinct_buyers"`
	Buyers         []string `json:"buyers"`
	Trades         int      `json:"trades"`
}

func newNovelInsiderClusterCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWindow string
		flagMin    int
		flagLimit  int
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "insider-cluster",
		Short: "Flag tickers where several distinct members of Congress bought within a window",
		Long: strings.Trim(`
Groups synced congressional equity trades by ticker and flags symbols where at
least --min distinct members of Congress filed purchases within the window —
cluster detection beyond a single disclosure.

Note: the syncable SEC insider-transactions resource is the owners view and has
no ticker, so this command clusters congressional disclosures (the ticker-
bearing source), a flagship Benzinga data product.

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources gov --since 90d
`, "\n"),
		Example:     "  benzinga-pp-cli insider-cluster --window 30d --min 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			window, err := novelDur(flagWindow, 30*24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}
			if flagMin < 1 {
				flagMin = 1
			}
			cutoff := time.Now().Add(-window)

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources gov --since 90d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "gov", flags.maxAge)

			rows, err := novelRows(cmd.Context(), db, "gov")
			if err != nil {
				return fmt.Errorf("querying congressional trades: %w", err)
			}

			type agg struct {
				buyers map[string]bool
				trades int
			}
			byTicker := map[string]*agg{}
			for _, m := range rows {
				ticker := normTicker(novelNested(m, "security", "ticker"))
				if ticker == "" {
					continue
				}
				// Purchases only ("P"; sales are "S" / "S (Partial)").
				if !strings.HasPrefix(strings.ToUpper(novelStr(m, "transaction_type")), "P") {
					continue
				}
				// Require a valid transaction_date within the window; dateless or
				// unparseable rows must not silently inflate clusters.
				d, perr := time.Parse("2006-01-02", novelStr(m, "transaction_date"))
				if perr != nil || d.Before(cutoff) {
					continue
				}
				member := novelNested(m, "filer_info", "member_name")
				if member == "" {
					member = novelStr(m, "name")
				}
				a := byTicker[ticker]
				if a == nil {
					a = &agg{buyers: map[string]bool{}}
					byTicker[ticker] = a
				}
				a.trades++
				if member != "" {
					a.buyers[member] = true
				}
			}

			clusters := make([]clusterRow, 0)
			for ticker, a := range byTicker {
				if len(a.buyers) < flagMin {
					continue
				}
				buyers := make([]string, 0, len(a.buyers))
				for b := range a.buyers {
					buyers = append(buyers, b)
				}
				sort.Strings(buyers)
				clusters = append(clusters, clusterRow{
					Ticker: ticker, DistinctBuyers: len(buyers), Buyers: buyers, Trades: a.trades,
				})
			}
			sort.SliceStable(clusters, func(i, j int) bool {
				if clusters[i].DistinctBuyers == clusters[j].DistinctBuyers {
					return clusters[i].Ticker < clusters[j].Ticker
				}
				return clusters[i].DistinctBuyers > clusters[j].DistinctBuyers
			})
			if flagLimit > 0 && len(clusters) > flagLimit {
				clusters = clusters[:flagLimit]
			}

			return novelEmit(cmd, flags, clusters, func() {
				w := cmd.OutOrStdout()
				if len(clusters) == 0 {
					fmt.Fprintf(w, "No tickers with >=%d distinct congressional buyers in the last %s. Widen --window, lower --min, or run sync --resources gov.\n", flagMin, window)
					return
				}
				fmt.Fprintln(w, "TICKER\tBUYERS\tTRADES\tNAMES")
				for _, c := range clusters {
					fmt.Fprintf(w, "%s\t%d\t%d\t%s\n", c.Ticker, c.DistinctBuyers, c.Trades, strings.Join(c.Buyers, ", "))
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Look-back window for congressional purchases (e.g. 30d, 90d)")
	cmd.Flags().IntVar(&flagMin, "min", 3, "Minimum distinct buyers to flag a ticker")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum clusters to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
