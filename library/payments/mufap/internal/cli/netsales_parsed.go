// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
)

// Real `netsales monthly` and `netsales investor`.
//
// WHY THESE REPLACE THE GENERATED LEAVES INSTEAD OF EDITING THEM. Both
// endpoints are declared `response_format: html`, and the generator's HTML
// handling supports only the `links`, `page` and `embedded-json` modes -- there
// is no table mode. So the generated commands ran the right request and then
// returned `{}` AT EXIT 0. For a flow series that is the worst available
// failure shape: a scripted caller cannot tell it from "MUFAP published nothing
// this month", which is a real and common state here (79 of the 104 months from
// 2018-01 to 2026-08 are genuinely empty).
//
// The generated leaf files carry "DO NOT EDIT", so rather than edit them and
// hope a later `generate --force` preserves it, this file registers a novel
// command hook that removes the generated children and attaches these. Nothing
// generated is touched, and the whole surface survives regeneration.

// netsalesChallengeAttempts is how many times a fetch is retried when
// Cloudflare returns its interstitial.
//
// MUFAP challenges these paths INTERMITTENTLY rather than consistently:
// measured 2026-09-08, 3 of 8 sequential requests from a plain Go client came
// back HTTP 403 with a "Just a moment..." body, and one month needed three
// attempts before it answered. Retrying is therefore the difference between a
// usable backfill and a panel with random holes -- but the retry is bounded and
// paced, and a challenge is never silently converted into an empty month.
const netsalesChallengeAttempts = 4

func netsalesBackoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * time.Second
}

// netsalesValidatePeriod rejects a month/year pair before any request.
//
// MUFAP serves a rendered page for ANY (Month, Year) it is asked for, including
// nonsense, so a bad parameter comes back as an ordinary-looking empty table
// rather than an error. Validating up front is the only way the caller learns
// they mistyped rather than reading a fabricated empty month.
func netsalesValidatePeriod(month, year string) (int, int, error) {
	m, err := strconv.Atoi(month)
	if err != nil || m < 1 || m > 12 {
		return 0, 0, usageErr(fmt.Errorf("--month must be a calendar month 1-12; got %q", month))
	}
	y, err := strconv.Atoi(year)
	if err != nil || y < 2000 || y > 2100 {
		return 0, 0, usageErr(fmt.Errorf("--year must be a four-digit calendar year; got %q", year))
	}
	return m, y, nil
}

// netsalesFetchDoc fetches one net-sales page, retrying the transient challenge.
func netsalesFetchDoc(ctx context.Context, c *client.Client, path string, month, year int) (string, error) {
	params := map[string]string{
		"Month": strconv.Itoa(month),
		"Year":  strconv.Itoa(year),
	}
	var lastErr error
	for attempt := 1; attempt <= netsalesChallengeAttempts; attempt++ {
		doc, err := mufapFetchHTML(ctx, c, path, params)
		if err != nil {
			return "", err
		}
		if !mufap.IsChallengePage(doc) {
			return doc, nil
		}
		lastErr = mufap.ErrNetSalesChallenge
		if attempt == netsalesChallengeAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(netsalesBackoff(attempt)):
		}
	}
	return "", apiErr(fmt.Errorf("%w after %d attempts: the challenge is intermittent, so retrying later usually succeeds. It is reported rather than returned as an empty month, because an empty month is a real observation here",
		lastErr, netsalesChallengeAttempts))
}

func newNetsalesMonthlyParsedCmd(flags *rootFlags) *cobra.Command {
	var flagMonth, flagYear string

	cmd := &cobra.Command{
		Use:   "monthly",
		Short: "Industry net sales for one month, by sector and category, in PKR millions.",
		Long: `Industry sales, redemptions and net sales for one month, by sector and category.

THREE TRAPS, all measured, all handled here:

  1. The last row is SectorId 100 / Sector "Total". It is the INVARIANT CHECK,
     never a data row. It is returned separately as ` + "`total`" + ` and is excluded
     from ` + "`rows`" + `; summing it with the others double-counts the whole month.

  2. MUFAP renders the SAME voluntary-pension figures under BOTH
     "Pension Funds (Open-End Funds)" and "Employer Pension Funds". Summing
     both labels overstates pension flow by about 13% (4,680 rows before
     de-duplication versus 4,072 after). Duplicates are dropped and counted in
     ` + "`vps_duplicates_dropped`" + `.

  3. A dash or an empty cell means the category DID NOT REPORT -- it does not
     mean zero. Those come back as null, never 0. This is the OPPOSITE of the
     CDC convention, where a dash is a real zero. Accounting negatives are
     written in parentheses and are decoded as negative.

The reported rows are reconciled against the sheet's own Total row under a
ROUNDING-AWARE bound. MUFAP renders whole PKR millions, so a sum of n rows
carries up to +/-0.5*(n+1) of rounding error and a fixed tolerance is simply
wrong: over the 25 months that carry a Total row every residual is <= 2.0 while
the bound ranges 3.0-7.5, so all 25 reconcile. Both the sales and the
redemption residual are checked, because 2026-04 reconciles exactly on sales
and is off by 2.0 on redemptions.

MUFAP serves a rendered page for every (Month, Year) whether or not it
published data, so an empty month is a real observation and is reported as one
-- 79 of the 104 months from 2018-01 to 2026-08 are empty this way. A
Cloudflare challenge is never reported as an empty month.`,
		Example: `  mufap-pp-cli netsales monthly --month 5 --year 2026 --json
  mufap-pp-cli netsales monthly --month 4 --year 2026 --agent`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--month=5;--year=2026",
			"pp:typed-exit-codes": "0,2,5",
			"pp:data-source":      "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("netsales monthly takes no positional arguments; got %q", args[0]))
			}
			if !cmd.Flags().Changed("month") && !cmd.Flags().Changed("year") && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "netsales monthly")
			}
			m, y, err := netsalesValidatePeriod(flagMonth, flagYear)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			doc, err := netsalesFetchDoc(ctx, c, "/Industry/WebMonthlyNetAssets", m, y)
			if err != nil {
				return err
			}
			ns, err := mufap.ParseNetSales(doc, mufap.NetSalesPeriod(m, y))
			if err != nil {
				if errors.Is(err, mufap.ErrNetSalesChallenge) {
					return apiErr(err)
				}
				return apiErr(fmt.Errorf("parsing the net-sales table for %s: %w", mufap.NetSalesPeriod(m, y), err))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), ns, flags)
			}
			return renderNetSales(cmd, ns)
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Calendar month 1-12. Zero-padded input is accepted and normalized; MUFAP's wire format is unpadded.")
	cmd.Flags().StringVar(&flagYear, "year", "", "Four-digit calendar year, e.g. 2026.")
	return cmd
}

func renderNetSales(cmd *cobra.Command, ns *mufap.NetSales) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  %s\n", ns.Period, ns.Title)
	fmt.Fprintf(w, "rows %d (reporting %d, non-reporting %d)",
		len(ns.Rows), ns.Reporting, len(ns.Rows)-ns.Reporting)
	if ns.VPSDuplicatesDropped > 0 {
		fmt.Fprintf(w, "  vps duplicates dropped %d", ns.VPSDuplicatesDropped)
	}
	fmt.Fprintln(w)
	if ns.Total != nil {
		fmt.Fprintf(w, "total (the sheet's own, NOT a data row): sales %s  redemptions %s  net %s\n",
			netsalesCell(ns.Total.Sales), netsalesCell(ns.Total.Redemptions), netsalesCell(ns.Total.NetSales))
	}
	if ns.Invariant.Note != "" {
		fmt.Fprintf(w, "invariant: %s\n", ns.Invariant.Note)
	}
	if ns.Note != "" {
		fmt.Fprintf(w, "note: %s\n", ns.Note)
	}
	if ns.Reporting == 0 {
		return nil
	}
	fmt.Fprintf(w, "\n%-34s %-42s %12s %12s %12s\n", "SECTOR", "CATEGORY", "SALES", "REDEMPTIONS", "NET")
	for _, r := range ns.Rows {
		if r.Sales == nil && r.Redemptions == nil && r.NetSales == nil {
			continue
		}
		fmt.Fprintf(w, "%-34s %-42s %12s %12s %12s\n",
			truncNetSales(r.Sector, 34), truncNetSales(r.Category, 42),
			netsalesCell(r.Sales), netsalesCell(r.Redemptions), netsalesCell(r.NetSales))
	}
	return nil
}

// netsalesCell renders a missing figure as "-" rather than 0, so the table
// cannot suggest a category reported zero when it did not report at all.
func netsalesCell(v *float64) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func truncNetSales(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func newNetsalesInvestorParsedCmd(flags *rootFlags) *cobra.Command {
	var flagMonth, flagYear string

	cmd := &cobra.Command{
		Use:   "investor",
		Short: "One month of net sales split across the nine investor classes.",
		Long: `Sales and redemptions for one month split across MUFAP's nine investor classes:
Individuals, Banking & Financial Institutions, Provident fund, Gratuity fund,
Pension fund, Public Limited Companies, Associated Companies, Fund of funds,
and Other. The page is 21 columns -- 3 keys plus one (Sales, Redemptions) pair
per class -- and the class names are read from the spanning group row, so a
class added or removed upstream changes the output rather than silently
shifting every figure one column across.

READ THE LIMITS BEFORE USING IT:

  * There is NO net column. Net must be derived as sales minus redemptions, and
    only where BOTH are present.
  * THIS FEED IS A PARTIAL SLICE. Measured across 25 months, its class totals
    failed to reconcile against the headline month total in 19 of them. It
    therefore CANNOT be used to apportion the industry total. Use it as a
    coverage-matched comparison -- which is how it refuted the mutual-fund tax
    channel on that channel's own predicted incidence, individuals having
    driven 109% of the swing.

Same conventions as ` + "`netsales monthly`" + `: a dash means the class did not report
(returned as null, never 0), parentheses mean negative, and a Cloudflare
challenge is reported as a challenge rather than as an empty month.`,
		Example: `  mufap-pp-cli netsales investor --month 5 --year 2026 --json
  mufap-pp-cli netsales investor --month 4 --year 2026 --agent`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--month=5;--year=2026",
			"pp:typed-exit-codes": "0,2,5",
			"pp:data-source":      "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("netsales investor takes no positional arguments; got %q", args[0]))
			}
			if !cmd.Flags().Changed("month") && !cmd.Flags().Changed("year") && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "netsales investor")
			}
			m, y, err := netsalesValidatePeriod(flagMonth, flagYear)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			doc, err := netsalesFetchDoc(ctx, c, "/Industry/WebMonthlyNetAssetsInvestor", m, y)
			if err != nil {
				return err
			}
			inv, err := mufap.ParseNetSalesInvestor(doc, mufap.NetSalesPeriod(m, y))
			if err != nil {
				if errors.Is(err, mufap.ErrNetSalesChallenge) {
					return apiErr(err)
				}
				return apiErr(fmt.Errorf("parsing the investor net-sales table for %s: %w", mufap.NetSalesPeriod(m, y), err))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), inv, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  %s\n", inv.Period, inv.Title)
			fmt.Fprintf(w, "%d investor classes x %d (sector,category) rows; %d class-rows reporting\n",
				len(inv.Classes), len(inv.Rows)/max1(len(inv.Classes)), inv.Reporting)
			fmt.Fprintf(w, "note: %s\n", inv.Note)
			if len(inv.Total) > 0 {
				fmt.Fprintf(w, "\n%-36s %14s %14s\n", "INVESTOR CLASS (sheet total)", "SALES", "REDEMPTIONS")
				for _, r := range inv.Total {
					fmt.Fprintf(w, "%-36s %14s %14s\n",
						truncNetSales(r.InvestorClass, 36), netsalesCell(r.Sales), netsalesCell(r.Redemptions))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Calendar month 1-12. Zero-padded input is accepted and normalized; MUFAP's wire format is unpadded.")
	cmd.Flags().StringVar(&flagYear, "year", "", "Four-digit calendar year, e.g. 2026.")
	return cmd
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		var parent *cobra.Command
		for _, c := range root.Commands() {
			if c.Name() == "netsales" {
				parent = c
				break
			}
		}
		if parent == nil {
			return
		}
		// Drop the generated leaves, which return {} at exit 0 on these HTML
		// table pages, and attach the parsing implementations in their place.
		for _, child := range parent.Commands() {
			switch child.Name() {
			case "monthly", "investor":
				parent.RemoveCommand(child)
			}
		}
		parent.AddCommand(newNetsalesMonthlyParsedCmd(flags))
		parent.AddCommand(newNetsalesInvestorParsedCmd(flags))
	})
}
