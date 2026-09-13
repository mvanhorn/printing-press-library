// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

type dropRow struct {
	ID         int64    `json:"id"`
	URL        string   `json:"url"`
	Title      string   `json:"title,omitempty"`
	Locality   string   `json:"locality"`
	Type       string   `json:"type"`
	Deal       string   `json:"deal"`
	Bedrooms   *int     `json:"bedrooms,omitempty"`
	Surface    *float64 `json:"surface_m2,omitempty"`
	DaysListed *int     `json:"days_listed,omitempty"`
	Gone       bool     `json:"gone"`
	cutInfo
}

func newNovelDropsCmd(flags *rootFlags) *cobra.Command {
	var flagCommune, flagType, flagDeal, flagSince, dbPath string
	var minDays, limit int
	var minPct float64
	var includeGone bool

	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Lists listings whose asking price fell, with first and latest price, number of cuts, total % cut and days listed.",
		Long: `Rank every stored listing whose asking price went down, using the price
history the CLI records on each find/watch/pull plus Immoweb's own old-price field.
Filters by commune (postal code or locality name), type, deal, recency of the last
cut and minimum days on market. Works offline on the local store.

Use this command to rank price cuts across all stored listings over a time window.
Do NOT use this command for what changed in one saved search since its last run; use 'watch' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli drops --commune liege --since 30d --agent
  immoweb-pp-cli drops --type house --deal sale --min-days 60 --min-pct 5`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--since=30d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drops")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			f := store.ListingFilter{IncludeGone: includeGone}
			if flagDeal != "" {
				d, err := immo.NormalizeDeal(flagDeal)
				if err != nil {
					return usageErr(err)
				}
				f.Deal = d
			}
			if flagType != "" {
				ts, err := immo.NormalizeTypes(flagType)
				if err != nil {
					return usageErr(err)
				}
				f.Types = ts
			}
			var since time.Time
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q (use e.g. 7d, 30d, 24h)", flagSince))
				}
				since = time.Now().Add(-d)
			}
			rows := make([]dropRow, 0)
			path, ok := localStoreExists(dbPath)
			if !ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: immoweb-pp-cli pull --commune <name> --type <type> --deal <sale|rent>\n", path)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openImmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, store.ImmoResource) {
				hintIfStale(cmd, db, store.ImmoResource, flags.maxAge)
			}
			for _, cmn := range immo.SplitCSV(flagCommune) {
				pcs, err := localPostcodesFor(ctx, db, cmn)
				if err != nil {
					return err
				}
				if len(pcs) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "no stored listings for %s: drops reads the local store only\nrun: immoweb-pp-cli pull --commune %s --type <type> --deal <sale|rent>\n", cmn, cmn)
					pcs = []string{"BE-0000"} // matches nothing; keeps the other communes' filter
				}
				f.PostalCodes = append(f.PostalCodes, pcs...)
			}
			listings, err := db.QueryListings(ctx, f)
			if err != nil {
				return err
			}
			hists, err := db.AllPriceHistories(ctx)
			if err != nil {
				return err
			}
			now := time.Now()
			for _, l := range listings {
				ci, ok := priceCut(l, hists[l.ID])
				if !ok || ci.CutPct < minPct {
					continue
				}
				if !since.IsZero() {
					at, err := time.Parse(time.RFC3339, ci.LastCutAt)
					if err != nil || at.Before(since) {
						continue
					}
				}
				days := daysOf(l, now)
				if minDays > 0 && (days == nil || *days < minDays) {
					continue
				}
				ci.History = nil
				rows = append(rows, dropRow{ID: l.ID, URL: l.URL, Title: l.Title, Locality: locLabel(l.Listing), Type: l.Type, Deal: l.Deal, Bedrooms: l.Bedrooms, Surface: l.Surface, DaysListed: days, Gone: l.GoneAt != "", cutInfo: ci})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].CutPct > rows[j].CutPct })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No price cuts recorded for these filters yet. Cuts appear as find/watch/pull see prices move (and when Immoweb shows an old price).")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ID\tFROM\tTO\tCUT\tCUTS\tDAYS\tLOCALITY\tSOURCE")
			for _, r := range rows {
				days := ""
				if r.DaysListed != nil {
					days = fmt.Sprint(*r.DaysListed)
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t-%.1f%%\t%d\t%s\t%s\t%s\n", r.ID, fmtEUR(r.FirstPrice), fmtEUR(r.LatestPrice), r.CutPct, r.Cuts, days, r.Locality, r.Source)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagCommune, "commune", "", "Commune name(s) or postal code(s), comma-separated")
	cmd.Flags().StringVar(&flagType, "type", "", "Property type(s)")
	cmd.Flags().StringVar(&flagDeal, "deal", "", "sale or rent")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only cuts observed within this window (e.g. 7d, 30d)")
	cmd.Flags().IntVar(&minDays, "min-days", 0, "Only listings online at least this many days")
	cmd.Flags().Float64Var(&minPct, "min-pct", 0, "Only cuts of at least this percentage")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
	cmd.Flags().BoolVar(&includeGone, "include-gone", false, "Include listings that have disappeared")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}
