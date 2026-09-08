// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
// Reads the local panel and, unless --offline is given, also consults the live
// index so a stale local copy cannot understate what is missing upstream.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsfetch"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsparse"
)

type covStateRow struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

type covGapRow struct {
	After        string `json:"after"`
	Before       string `json:"before"`
	DaysBetween  int    `json:"days_between"`
	MissingWeeks int    `json:"missing_weeks"`
}

type covFormatRow struct {
	Ext   string `json:"ext"`
	Count int    `json:"count"`
}

type covCellCensus struct {
	AsOf        string `json:"as_of"`
	Present     int    `json:"present"`
	Zero        int    `json:"zero_uncollected"`
	Blank       int    `json:"blank"`
	NA          int    `json:"not_available"`
	Unparseable int    `json:"unparseable"`
}

type coverageEnvelope struct {
	IndexedReleases  int             `json:"indexed_releases"`
	StoredReleases   int             `json:"stored_releases"`
	NotFetched       []string        `json:"not_fetched,omitempty"`
	States           []covStateRow   `json:"file_states"`
	Gaps             []covGapRow     `json:"upstream_gaps,omitempty"`
	MissingWeeks     int             `json:"upstream_missing_weeks"`
	IndexCoveragePct float64         `json:"upstream_index_coverage_pct"`
	Formats          []covFormatRow  `json:"annexure_formats,omitempty"`
	CellCensus       []covCellCensus `json:"cell_census,omitempty"`
	Note             string          `json:"note,omitempty"`
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		from        string
		to          string
		gaps        bool
		stateCensus bool
		formatMix   bool
		offline     bool
		dbPath      string
	)
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report what exists upstream, what is stored, and every hole as a hole",
		Long: strings.Trim(`
Reconcile the local panel against the Bureau's own index.

Use this command to answer what is present and what is MISSING, at release and
cell-state granularity. Do NOT use it to check whether present values parsed
correctly; use 'verify' instead. Do NOT use it to detect changes made upstream;
use 'revisions' instead.

Gaps are reported, never filled.

Gap detection is CADENCE-based with tolerance, deliberately not weekday-based.
Releases land on six different weekdays in this series, so an "expected
Thursday" rule reports about eighteen gaps that do not exist.

Four outcomes are kept distinct and never collapsed: a release not fetched, a
release fetched and genuinely empty, a release whose file has rotted upstream,
and a release whose fetch failed. Merging them is how a coverage map comes to
lie about a short series.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli coverage --gaps
  pbs-pp-cli coverage --gaps --state-census --json
  pbs-pp-cli coverage --format-mix --offline --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = panelDBPath(dbPath)
			// The store is OPTIONAL here. coverage answers "what exists upstream
			// versus what do I hold", and the upstream half is exactly what a
			// caller with no data yet needs. Returning early on a missing store
			// made a fresh install unable to answer its first question.
			db, haveStore, err := openPanelForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if haveStore {
				defer db.Close()
				warnIfStale(ctx, db.DB(), cmd.ErrOrStderr(), flags, cmd)
			} else {
				noMirror(cmd.ErrOrStderr(), dbPath)
			}
			out := coverageEnvelope{States: []covStateRow{}}
			// The store FILE is created by the framework's local-learning loop
			// before any command runs, so "does the file exist" is almost always
			// true and is the wrong question. Whether the panel holds rows is the
			// real signal.
			if !haveStore || (db != nil && panelEmpty(ctx, db.DB())) {
				out.Note = "the local panel holds no releases yet; reporting the upstream side only"
			}

			// Upstream truth: prefer the live index, fall back to what the last
			// sync recorded. Which one was used is stated, because a stale
			// local index would understate the gap.
			var idx *pbsparse.Index
			if !offline {
				fc := pbsfetch.New(flags.timeout, 2)
				idx, _, err = fetchIndex(ctx, fc)
				if err != nil {
					out.Note = "could not reach the live index; reporting against the locally recorded index instead: " + err.Error()
					offline = true
				}
			}
			if idx != nil {
				out.IndexedReleases = len(idx.Weekly) + len(idx.Monthly)
				for _, g := range idx.WeeklyGaps() {
					out.MissingWeeks += g.MissingWeeks
					if gaps {
						out.Gaps = append(out.Gaps, covGapRow{
							After: g.After, Before: g.Before,
							DaysBetween: g.DaysBetween, MissingWeeks: g.MissingWeeks,
						})
					}
				}
				if n := len(idx.Weekly) + out.MissingWeeks; n > 0 {
					out.IndexCoveragePct = float64(len(idx.Weekly)) / float64(n) * 100
				}
			} else {
				if haveStore {
					if err := db.DB().QueryRowContext(ctx,
						`SELECT COUNT(*) FROM pbs_release`).Scan(&out.IndexedReleases); err != nil {
						return fmt.Errorf("count indexed releases: %w", err)
					}
				}
			}

			if haveStore {
				if err := db.DB().QueryRowContext(ctx,
					`SELECT COUNT(DISTINCT as_of) FROM pbs_coverage WHERE state = ?`, covFetched).
					Scan(&out.StoredReleases); err != nil {
					return fmt.Errorf("count stored releases: %w", err)
				}
			}

			// File-state census across every recorded outcome.
			if haveStore {
				sRows, err := db.DB().QueryContext(ctx,
					`SELECT state, COUNT(*) FROM pbs_coverage GROUP BY state ORDER BY COUNT(*) DESC`)
				if err != nil {
					return fmt.Errorf("query file states: %w", err)
				}
				for sRows.Next() {
					var r covStateRow
					if err := sRows.Scan(&r.State, &r.Count); err != nil {
						_ = sRows.Close()
						return err
					}
					out.States = append(out.States, r)
				}
				if err := sRows.Err(); err != nil {
					_ = sRows.Close()
					return err
				}
				if err := sRows.Close(); err != nil {
					return err
				}
			}

			// Releases the index knows about that the store has never fetched.
			if idx != nil {
				stored := map[string]bool{}
				if haveStore {
					iRows, err := db.DB().QueryContext(ctx, `SELECT DISTINCT as_of FROM pbs_coverage`)
					if err != nil {
						return fmt.Errorf("query stored coverage: %w", err)
					}
					for iRows.Next() {
						var d string
						if err := iRows.Scan(&d); err != nil {
							_ = iRows.Close()
							return err
						}
						stored[d] = true
					}
					if err := iRows.Err(); err != nil {
						_ = iRows.Close()
						return err
					}
					if err := iRows.Close(); err != nil {
						return err
					}
				}
				for _, r := range idx.All() {
					k := r.AsOfKey()
					if from != "" && k < from {
						continue
					}
					if to != "" && k > to {
						continue
					}
					if !stored[k] {
						out.NotFetched = append(out.NotFetched, k)
					}
				}
				sort.Sort(sort.Reverse(sort.StringSlice(out.NotFetched)))
			}

			if formatMix && idx != nil {
				counts := map[string]int{}
				for _, r := range idx.Weekly {
					if f, ok := r.File(pbsparse.RoleAnnexure, "xlsx"); ok && f.Ext == "xlsx" {
						counts["xlsx"]++
						continue
					}
					if f, ok := r.File(pbsparse.RoleAnnexure, "pdf"); ok && f.Ext == "pdf" {
						counts["pdf"]++
					}
				}
				for ext, n := range counts {
					out.Formats = append(out.Formats, covFormatRow{Ext: ext, Count: n})
				}
				sort.Slice(out.Formats, func(i, j int) bool { return out.Formats[i].Count > out.Formats[j].Count })
			}

			if stateCensus && haveStore {
				q := `SELECT as_of, COALESCE(SUM(present_cells),0), COALESCE(SUM(zero_cells),0),
				             COALESCE(SUM(blank_cells),0), COALESCE(SUM(na_cells),0),
				             COALESCE(SUM(unparseable_cells),0)
				      FROM pbs_coverage WHERE 1=1`
				var qa []any
				if from != "" {
					q += ` AND as_of >= ?`
					qa = append(qa, from)
				}
				if to != "" {
					q += ` AND as_of <= ?`
					qa = append(qa, to)
				}
				q += ` GROUP BY as_of ORDER BY as_of DESC`
				cRows, err := db.DB().QueryContext(ctx, q, qa...)
				if err != nil {
					return fmt.Errorf("query cell census: %w", err)
				}
				for cRows.Next() {
					var r covCellCensus
					if err := cRows.Scan(&r.AsOf, &r.Present, &r.Zero, &r.Blank, &r.NA, &r.Unparseable); err != nil {
						_ = cRows.Close()
						return err
					}
					out.CellCensus = append(out.CellCensus, r)
				}
				if err := cRows.Err(); err != nil {
					_ = cRows.Close()
					return err
				}
				if err := cRows.Close(); err != nil {
					return err
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			src := "live index"
			if offline {
				src = "locally recorded index"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "coverage against the %s\n\n", src)
			fmt.Fprintf(cmd.OutOrStdout(), "  releases indexed upstream: %d\n", out.IndexedReleases)
			fmt.Fprintf(cmd.OutOrStdout(), "  releases stored locally:   %d\n", out.StoredReleases)
			fmt.Fprintf(cmd.OutOrStdout(), "  releases never fetched:    %d\n", len(out.NotFetched))
			if out.MissingWeeks > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  weeks absent UPSTREAM:     %d (index covers %.1f%% of the weekly cadence)\n",
					out.MissingWeeks, out.IndexCoveragePct)
			}
			if len(out.States) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nfile outcomes:")
				for _, s := range out.States {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-16s %d\n", s.State, s.Count)
				}
			}
			if len(out.Formats) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nweekly annexure formats upstream:")
				for _, f := range out.Formats {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-5s %d\n", f.Ext, f.Count)
				}
			}
			if len(out.Gaps) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nupstream gaps (the Bureau never published these weeks):")
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "  AFTER\tBEFORE\tDAYS\tWEEKS MISSING")
				for _, g := range out.Gaps {
					fmt.Fprintf(tw, "  %s\t%s\t%d\t%d\n", g.After, g.Before, g.DaysBetween, g.MissingWeeks)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}
			if len(out.CellCensus) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\ncell states per release (zero means UNCOLLECTED, not free):")
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "  AS OF\tPRESENT\tZERO\tBLANK\tN/A\tUNPARSEABLE")
				for _, c := range out.CellCensus {
					fmt.Fprintf(tw, "  %s\t%d\t%d\t%d\t%d\t%d\n", c.AsOf, c.Present, c.Zero, c.Blank, c.NA, c.Unparseable)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}
			if out.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date to report, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date to report, YYYY-MM-DD")
	cmd.Flags().BoolVar(&gaps, "gaps", false, "List the weeks the Bureau never published")
	cmd.Flags().BoolVar(&stateCensus, "state-census", false, "Per-release counts of present, zero, blank and not-available cells")
	cmd.Flags().BoolVar(&formatMix, "format-mix", false, "Count how many weekly annexures are xlsx versus pdf upstream")
	cmd.Flags().BoolVar(&offline, "offline", false, "Report against the locally recorded index instead of fetching it")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

var _ = context.Background
var _ = sql.ErrNoRows
