// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// Reads only the accrued local panel; no upstream call. Sync advances the data.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type spreadPoint struct {
	AsOf    string     `json:"as_of"`
	Summary dispersion `json:"summary"`
	Census  cellCensus `json:"census"`
	Cities  []cityObs  `json:"cities,omitempty"`
}

type spreadEnvelope struct {
	Item          string        `json:"item"`
	Stat          string        `json:"stat"`
	Points        []spreadPoint `json:"points"`
	RankStability *rankStab     `json:"rank_stability,omitempty"`
	Note          string        `json:"note,omitempty"`
}

type rankStab struct {
	FirstAsOf string  `json:"first_as_of"`
	LastAsOf  string  `json:"last_as_of"`
	Spearman  float64 `json:"spearman_rho"`
	Cities    int     `json:"cities_compared"`
	Note      string  `json:"note,omitempty"`
}

func newNovelSpreadCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf       string
		series     bool
		from       string
		to         string
		stat       string
		minCities  int
		rankStable bool
		dbPath     string
		showCities bool
	)
	cmd := &cobra.Command{
		Use:   "spread <item>",
		Short: "Compare one item's price across cities, and trend that dispersion",
		Long: strings.Trim(`
Show the cross-city distribution of one essential item's price.

Use this command to compare one item ACROSS CITIES in a single week, or to
trend that cross-city dispersion over time with --series. Do NOT use it to
follow one item's price level over weeks for named cities; use 'drift' instead.
Do NOT use it to rank many items within one week; use 'movers' instead.

The Bureau publishes no dispersion measure at all, and each week as a separate
file, so this answer requires the whole panel held together locally.

Every result states how many cities contributed and how many cells were
excluded, split by reason. That matters here more than anywhere: a numeric zero
in this source means the price was not collected, and averaging those as real
prices moves a measured national figure by about -16.6%.

Outliers are flagged with Tukey fences and never dropped: on this data an
extreme city price is usually a real regional dislocation, which is the signal.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli spread "Onions"
  pbs-pp-cli spread "Wheat Flour Bag" --as-of 2026-09-03 --agent
  pbs-pp-cli spread "Onions" --series --from 2026-01-01 --rank-stability
  pbs-pp-cli spread "Sugar Refined" --stat max --show-cities --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<item>=Onions",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "spread")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an item name is required"))
			}
			st, err := normStat(stat)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = panelDBPath(dbPath)
			db, ok, err := openPanelForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				noMirror(cmd.ErrOrStderr(), dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), spreadEnvelope{
						Item: strings.Join(args, " "), Stat: st,
						Points: []spreadPoint{}, Note: "no local panel yet",
					}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), spreadEnvelope{Item: strings.Join(args, " "), Stat: st, Points: []spreadPoint{}, Note: "the local panel is empty"}, flags)
				}
				return nil
			}

			item, err := resolveItem(ctx, db.DB(), strings.Join(args, " "))
			if err != nil {
				return notFoundErr(err)
			}
			out := spreadEnvelope{Item: item, Stat: st, Points: []spreadPoint{}}

			var dates []string
			if series {
				dates, err = storedAsOfRange(ctx, db.DB(), item, st, from, to)
				if err != nil {
					return err
				}
			} else {
				d := asOf
				if d == "" {
					if d, err = latestAsOf(ctx, db.DB()); err != nil {
						return err
					}
				}
				if d != "" {
					dates = []string{d}
				}
			}

			// Carry raw values, not ranks: ranks must be recomputed over the
			// intersection of the two city sets, which is not known until both
			// releases have been read.
			firstVals := map[string]float64{}
			lastVals := map[string]float64{}
			for _, d := range dates {
				obs, cen, err := cityObsFor(ctx, db.DB(), item, st, d)
				if err != nil {
					return err
				}
				p := spreadPoint{AsOf: d, Summary: summarise(obs), Census: cen}
				if showCities {
					sort.Slice(obs, func(i, j int) bool { return obs[i].Value < obs[j].Value })
					p.Cities = obs
				}
				if minCities > 0 && p.Summary.Cities < minCities {
					p.Summary = dispersion{Cities: p.Summary.Cities}
					out.Note = fmt.Sprintf("one or more releases had fewer than --min-cities %d contributing cities; their summary is suppressed rather than computed over a thin sample", minCities)
				}
				out.Points = append(out.Points, p)

				if rankStable && len(obs) >= 3 {
					vals := make(map[string]float64, len(obs))
					for _, o := range obs {
						vals[o.City] = o.Value
					}
					if len(firstVals) == 0 {
						firstVals = vals
						out.RankStability = &rankStab{FirstAsOf: d}
					}
					lastVals = vals
					if out.RankStability != nil {
						out.RankStability.LastAsOf = d
					}
				}
			}

			if rankStable {
				switch {
				case out.RankStability == nil || len(firstVals) == 0:
					out.Note = strings.TrimSpace(out.Note + " rank stability needs at least one release with three or more contributing cities.")
				case out.RankStability.FirstAsOf == out.RankStability.LastAsOf:
					out.RankStability.Note = "only one release in range; rank stability needs at least two"
				default:
					rho, n := spearmanFromValues(firstVals, lastVals)
					out.RankStability.Spearman = rho
					out.RankStability.Cities = n
					if n < 3 {
						out.RankStability.Note = "fewer than three cities present in BOTH releases, so no rank correlation is defined"
					} else {
						out.RankStability.Note = fmt.Sprintf("ranks recomputed over the %d cities present in both releases", n)
					}
				}
			}

			if len(out.Points) == 0 {
				out.Note = fmt.Sprintf("no stored releases for %q; sync a wider range first", item)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				if len(out.Points) == 0 {
					return notFoundErr(fmt.Errorf("no stored releases for %q", item))
				}
				return nil
			}

			if len(out.Points) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stored releases for %q. Sync a wider range first.\n", item)
				return notFoundErr(fmt.Errorf("no stored releases for %q", item))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  (%s)\n\n", item, st)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "AS OF\tCITIES\tMIN\tCHEAPEST\tMAX\tDEAREST\tRANGE\tCV%\tEXCLUDED")
			for _, p := range out.Points {
				s := p.Summary
				fmt.Fprintf(tw, "%s\t%d\t%.2f\t%s\t%.2f\t%s\t%.2f\t%.1f\t%d\n",
					p.AsOf, s.Cities, s.Min, s.MinCity, s.Max, s.MaxCity, s.Range, s.CV, p.Census.Excluded())
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			last := out.Points[len(out.Points)-1]
			if last.Census.Excluded() > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nexcluded in %s: %d uncollected zeros, %d blank, %d not-available, %d unparseable\n",
					last.AsOf, last.Census.Zero, last.Census.Blank, last.Census.NA, last.Census.Unparseable)
			}
			if len(last.Summary.Outliers) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "tukey-flagged cities in %s: %s\n", last.AsOf, strings.Join(last.Summary.Outliers, ", "))
			}
			if rs := out.RankStability; rs != nil && rs.Cities >= 3 {
				fmt.Fprintf(cmd.OutOrStdout(), "rank stability %s -> %s: spearman %.3f over %d cities\n",
					rs.FirstAsOf, rs.LastAsOf, rs.Spearman, rs.Cities)
			}
			if out.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "Release date to summarise, YYYY-MM-DD (defaults to the newest stored)")
	cmd.Flags().BoolVar(&series, "series", false, "Report dispersion for every stored release in range instead of one")
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date for --series, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date for --series, YYYY-MM-DD")
	cmd.Flags().StringVar(&stat, "stat", "avg", "Which published price to use: min, avg or max")
	cmd.Flags().IntVar(&minCities, "min-cities", 0, "Suppress a release's summary when fewer than this many cities reported")
	cmd.Flags().BoolVar(&rankStable, "rank-stability", false, "Report the Spearman rank correlation between the first and last release in range")
	cmd.Flags().BoolVar(&showCities, "show-cities", false, "Include every contributing city and its price")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}
