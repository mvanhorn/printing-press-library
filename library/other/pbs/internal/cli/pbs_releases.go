// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsfetch"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsparse"
)

type releaseView struct {
	AsOf     string   `json:"as_of"`
	Kind     string   `json:"kind"`
	AsOfRaw  string   `json:"as_of_raw"`
	Formats  []string `json:"formats"`
	Annexure string   `json:"annexure_url,omitempty"`
	Report   string   `json:"report_url,omitempty"`
	Mismatch bool     `json:"filename_date_mismatch,omitempty"`
	IndexPos int      `json:"index_pos"`
	Stored   bool     `json:"stored"`
}

type releasesEnvelope struct {
	Source        string              `json:"source"`
	Releases      []releaseView       `json:"releases"`
	WeeklyCount   int                 `json:"weekly_count"`
	MonthlyCount  int                 `json:"monthly_count"`
	Oldest        string              `json:"oldest,omitempty"`
	Newest        string              `json:"newest,omitempty"`
	MissingWeeks  int                 `json:"missing_weeks"`
	DuplicateURLs map[string][]string `json:"duplicate_urls,omitempty"`
	Note          string              `json:"note,omitempty"`
}

func newPBSReleasesCmd(flags *rootFlags) *cobra.Command {
	var (
		kind    string
		from    string
		to      string
		limit   int
		dbPath  string
		offline bool
	)
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "List PBS price releases from the site index, sorted by real date",
		Long: strings.Trim(`
List every Sensitive Price Indicator and CPI release the Bureau indexes.

This is the CLI's enumerator, and it is the only one there can be. Release URLs
cannot be constructed from a date: the filenames use dozens of shapes, including
one with no date at all, plus load-bearing typos, and a guessed URL returns a
clean 404. The index itself is a hand-edited JavaScript array inside the page.

Two properties of that array are corrected here rather than inherited:
the array is NOT sorted, so its last element is not its oldest release; and two
rows carry a filename whose embedded date disagrees with the index date, which
is flagged and never silently reconciled.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli releases --limit 5
  pbs-pp-cli releases --kind cpi --json
  pbs-pp-cli releases --from 2023-07-01 --to 2023-12-31 --agent
  pbs-pp-cli releases --offline --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !isTerminal(cmd.OutOrStdout()) {
				// A bare piped invocation is a legitimate "give me everything".
			} else if len(args) == 0 && cmd.Flags().NFlag() == 0 && isTerminal(cmd.OutOrStdout()) {
				// Interactive bare invocation still lists; there is no required input.
				_ = 0
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "releases")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = panelDBPath(dbPath)
			out := releasesEnvelope{DuplicateURLs: map[string][]string{}}
			var all []pbsparse.Release

			if offline {
				db, ok, err := openPanelForRead(ctx, dbPath)
				if err != nil {
					return err
				}
				if !ok {
					noMirror(cmd.ErrOrStderr(), dbPath)
					out.Source = "local"
					out.Releases = []releaseView{}
					out.Note = "no local panel yet"
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), out, flags)
					}
					return nil
				}
				defer db.Close()
				rows, err := db.DB().QueryContext(ctx,
					`SELECT as_of, kind, as_of_raw, index_pos, filename_mismatch FROM pbs_release ORDER BY as_of DESC`)
				if err != nil {
					return fmt.Errorf("query releases: %w", err)
				}
				var views []releaseView
				for rows.Next() {
					var v releaseView
					var raw *string
					var pos *int
					var mm int
					if err := rows.Scan(&v.AsOf, &v.Kind, &raw, &pos, &mm); err != nil {
						_ = rows.Close()
						return fmt.Errorf("scan release: %w", err)
					}
					if raw != nil {
						v.AsOfRaw = *raw
					}
					if pos != nil {
						v.IndexPos = *pos
					}
					v.Mismatch = mm == 1
					v.Stored = true
					views = append(views, v)
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return fmt.Errorf("iterate releases: %w", err)
				}
				if err := rows.Close(); err != nil {
					return err
				}
				out.Source = "local"
				out.Releases = filterReleaseViews(views, kind, from, to, limit)
			} else {
				fc := pbsfetch.New(flags.timeout, 2)
				idx, _, err := fetchIndex(ctx, fc)
				if err != nil {
					return classifyAPIErrorOnly(err)
				}
				out.Source = pbsBaseURL + pbsIndexPath
				out.WeeklyCount = len(idx.Weekly)
				out.MonthlyCount = len(idx.Monthly)
				out.DuplicateURLs = idx.DuplicateURLs
				for _, g := range idx.WeeklyGaps() {
					out.MissingWeeks += g.MissingWeeks
				}
				all = idx.All()
				views := make([]releaseView, 0, len(all))
				for _, r := range all {
					v := releaseView{
						AsOf: r.AsOfKey(), Kind: string(r.Kind), AsOfRaw: r.AsOfRaw,
						IndexPos: r.IndexPos, Mismatch: r.FilenameDateMismatch,
					}
					seen := map[string]bool{}
					for _, f := range r.Files {
						if f.Ext != "" && !seen[f.Ext] {
							seen[f.Ext] = true
							v.Formats = append(v.Formats, f.Ext)
						}
					}
					sort.Strings(v.Formats)
					if f, ok := r.File(pbsparse.RoleAnnexure, "xlsx"); ok {
						v.Annexure = f.URL
					}
					if f, ok := r.File(pbsparse.RoleReport, "xlsx"); ok {
						v.Report = f.URL
					}
					views = append(views, v)
				}
				out.Releases = filterReleaseViews(views, kind, from, to, limit)
			}

			if len(out.Releases) > 0 {
				out.Newest = out.Releases[0].AsOf
				out.Oldest = out.Releases[len(out.Releases)-1].AsOf
			}
			if out.Releases == nil {
				out.Releases = []releaseView{}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out.Releases) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No releases matched.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "AS OF\tKIND\tFORMATS\tFLAG")
			for _, v := range out.Releases {
				flag := ""
				if v.Mismatch {
					flag = "filename-date-mismatch"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", v.AsOf, v.Kind, strings.Join(v.Formats, ","), flag)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d releases listed (weekly %d, monthly %d)",
				len(out.Releases), out.WeeklyCount, out.MonthlyCount)
			if out.MissingWeeks > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "; %d weeks absent from the upstream index", out.MissingWeeks)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by series: spi (weekly) or cpi (monthly)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date to list, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date to list, YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum releases to list (0 lists all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	cmd.Flags().BoolVar(&offline, "offline", false, "List releases already recorded locally instead of fetching the index")
	return cmd
}

// filterReleaseViews applies the kind and date filters.
func filterReleaseViews(in []releaseView, kind, from, to string, limit int) []releaseView {
	want := ""
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "spi", "weekly", "spi-weekly":
		want = string(pbsparse.KindWeekly)
	case "cpi", "monthly", "cpi-monthly":
		want = string(pbsparse.KindMonthly)
	}
	out := make([]releaseView, 0, len(in))
	for _, v := range in {
		if want != "" && v.Kind != want {
			continue
		}
		if from != "" && v.AsOf < from {
			continue
		}
		if to != "" && v.AsOf > to {
			continue
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].AsOf > out[j].AsOf })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

var _ = time.Now
