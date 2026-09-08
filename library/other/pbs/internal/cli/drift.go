// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// Reads only the accrued local panel; no upstream call. Sync advances the data.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type driftCell struct {
	AsOf  string   `json:"as_of"`
	City  string   `json:"city"`
	Value *float64 `json:"value"`
	State string   `json:"value_state"`
}

type driftEnvelope struct {
	Item     string      `json:"item"`
	Stat     string      `json:"stat"`
	Cities   []string    `json:"cities"`
	Releases []string    `json:"releases"`
	Cells    []driftCell `json:"cells"`
	Census   cellCensus  `json:"census"`
	Note     string      `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		cities []string
		stat   string
		from   string
		to     string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "drift <item>",
		Short: "Follow one item's price over time for named cities",
		Long: strings.Trim(`
Track one essential item's price across releases for one or more cities.

Use this command to follow ONE item's price over TIME for named cities. Do NOT
use it for the cross-city distribution in a single week; use 'spread' instead.
Do NOT use it to find which items moved this week; use 'movers' instead.

The Bureau publishes each week as a standalone file and keeps no series, so
this question cannot be asked of the source at all.

A release with no observation emits a row carrying its value state and no
value. Nothing is interpolated, smoothed or carried forward: a missing week is
reported as missing, which is the correct answer.

Rows are keyed on the item DESCRIPTION, because the source's own Sr column
restarts inside each of three weekly re-ranked sections and its item numbering
shifts whenever the basket changes.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli drift "Wheat Flour Bag" --city lahore --city quetta
  pbs-pp-cli drift "Onions" --city karachi --from 2026-01-01 --agent
  pbs-pp-cli drift "Sugar Refined" --stat max --json
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
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
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
					return printJSONFiltered(cmd.OutOrStdout(), driftEnvelope{
						Item: strings.Join(args, " "), Stat: st,
						Cities: []string{}, Releases: []string{}, Cells: []driftCell{},
						Note: "no local panel yet",
					}, flags)
				}
				return nil
			}
			defer db.Close()
			warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			if panelEmpty(ctx, db.DB()) {
				emptyPanelHint(cmd.ErrOrStderr())
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), driftEnvelope{Item: strings.Join(args, " "), Stat: st, Cities: []string{}, Releases: []string{}, Cells: []driftCell{}, Note: "the local panel is empty"}, flags)
				}
				return nil
			}

			item, err := resolveItem(ctx, db.DB(), strings.Join(args, " "))
			if err != nil {
				return notFoundErr(err)
			}
			out := driftEnvelope{Item: item, Stat: st, Cities: []string{}, Releases: []string{}, Cells: []driftCell{}}

			var want []string
			for _, c := range cities {
				for _, part := range strings.Split(c, ",") {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					resolved, err := resolveCity(ctx, db.DB(), part)
					if err != nil {
						return usageErr(err)
					}
					want = append(want, resolved)
				}
			}
			if len(want) == 0 {
				// No city named: report every city, which is the honest default
				// for "how has this item moved" without a geography filter.
				rows, err := db.DB().QueryContext(ctx,
					`SELECT DISTINCT city FROM pbs_price WHERE surface='appendix-a' AND item_desc = ? ORDER BY city`, item)
				if err != nil {
					return fmt.Errorf("list cities: %w", err)
				}
				for rows.Next() {
					var c string
					if err := rows.Scan(&c); err != nil {
						_ = rows.Close()
						return err
					}
					want = append(want, c)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return err
				}
				if err := rows.Close(); err != nil {
					return err
				}
			}
			sort.Strings(want)
			out.Cities = want

			dates, err := storedAsOfRange(ctx, db.DB(), item, st, from, to)
			if err != nil {
				return err
			}
			out.Releases = dates

			// Load the whole item's series once, then fill the grid, so a
			// missing (release, city) pair is visibly missing rather than absent.
			have := map[string]driftCell{}
			q := `SELECT as_of, city, value, value_state FROM pbs_price
			      WHERE surface='appendix-a' AND item_desc = ? AND stat = ?`
			qa := []any{item, st}
			if from != "" {
				q += ` AND as_of >= ?`
				qa = append(qa, from)
			}
			if to != "" {
				q += ` AND as_of <= ?`
				qa = append(qa, to)
			}
			rows, err := db.DB().QueryContext(ctx, q, qa...)
			if err != nil {
				return fmt.Errorf("query drift: %w", err)
			}
			for rows.Next() {
				var d, c, state string
				var v sql.NullFloat64
				if err := rows.Scan(&d, &c, &v, &state); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan drift: %w", err)
				}
				cell := driftCell{AsOf: d, City: c, State: state}
				if v.Valid && state == "present" {
					f := v.Float64
					cell.Value = &f
				}
				have[d+"|"+c] = cell
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}

			for _, d := range dates {
				for _, c := range want {
					if cell, ok := have[d+"|"+c]; ok {
						out.Cells = append(out.Cells, cell)
						switch cell.State {
						case "present":
							out.Census.Present++
						case "zero":
							out.Census.Zero++
						case "blank":
							out.Census.Blank++
						case "na":
							out.Census.NA++
						default:
							out.Census.Unparseable++
						}
						continue
					}
					// Not published for this city in this release. Recorded as a
					// gap, never filled.
					out.Cells = append(out.Cells, driftCell{AsOf: d, City: c, State: "not_published"})
					out.Census.Blank++
				}
			}

			if len(out.Cells) == 0 {
				out.Note = fmt.Sprintf("no stored observations for %q; sync a wider range first", item)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				if len(out.Cells) == 0 {
					return notFoundErr(fmt.Errorf("no stored observations for %q", item))
				}
				return nil
			}
			if len(out.Cells) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stored observations for %q.\n", item)
				return notFoundErr(fmt.Errorf("no stored observations for %q", item))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  (%s)\n\n", item, st)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "AS OF\t%s\n", strings.ToUpper(strings.Join(out.Cities, "\t")))
			byDate := map[string]map[string]driftCell{}
			for _, c := range out.Cells {
				if byDate[c.AsOf] == nil {
					byDate[c.AsOf] = map[string]driftCell{}
				}
				byDate[c.AsOf][c.City] = c
			}
			for _, d := range out.Releases {
				fmt.Fprintf(tw, "%s", d)
				for _, city := range out.Cities {
					cell := byDate[d][city]
					if cell.Value != nil {
						fmt.Fprintf(tw, "\t%.2f", *cell.Value)
					} else {
						fmt.Fprintf(tw, "\t%s", shortState(cell.State))
					}
				}
				fmt.Fprintln(tw)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if ex := out.Census.Excluded(); ex > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d of %d cells carry no price: %d uncollected zeros, %d blank or not published, %d not-available, %d unparseable. None were filled in.\n",
					ex, len(out.Cells), out.Census.Zero, out.Census.Blank, out.Census.NA, out.Census.Unparseable)
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&cities, "city", nil, "City to include; repeatable, or comma-separated (defaults to every city)")
	cmd.Flags().StringVar(&stat, "stat", "avg", "Which published price to use: min, avg or max")
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date, YYYY-MM-DD")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

// shortState renders a missing cell compactly without pretending it is a value.
func shortState(s string) string {
	switch s {
	case "zero":
		return "uncollected"
	case "na":
		return "n/a"
	case "blank", "not_published":
		return "-"
	case "":
		return "-"
	}
	return s
}
