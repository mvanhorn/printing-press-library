// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsfetch"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsparse"
)

type syncOutcome struct {
	AsOf   string `json:"as_of"`
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	State  string `json:"state"`
	Parser string `json:"parser,omitempty"`
	Rows   int    `json:"rows,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
	Note   string `json:"note,omitempty"`
}

type syncEnvelope struct {
	IndexReleases int           `json:"index_releases"`
	IndexFiles    int           `json:"index_files"`
	Attempted     int           `json:"releases_attempted"`
	Completed     int           `json:"releases_completed"`
	Skipped       int           `json:"releases_skipped_already_present"`
	Remaining     int           `json:"releases_remaining"`
	PriceRows     int           `json:"price_rows_written"`
	WeightRows    int           `json:"weight_rows_written"`
	Outcomes      []syncOutcome `json:"outcomes"`
	Incomplete    bool          `json:"incomplete"`
	ResumeCommand string        `json:"resume_command,omitempty"`
	Note          string        `json:"note,omitempty"`
}

func newPBSSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		kind        string
		from        string
		to          string
		full        bool
		maxReleases int
		refetch     bool
		dbPath      string
		prefer      string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch and parse PBS releases into the local panel",
		Long: strings.Trim(`
Build the local price panel from the Bureau's release files.

Each release is TWO files, not one: the annexure carries the city x item price
appendix, and the executive report carries the item weight vector, the quintile
index table and the section counts. A release synced from only one of them is
missing half the dataset, so both are fetched.

The run is resumable and checkpointed per file. Releases already recorded are
skipped unless --refetch is given, so an interrupted backfill can simply be
re-run. An incomplete run exits non-zero AFTER printing its summary, so a
partial panel is never reported as a whole one.

OPERATIONAL NOTE: a full backfill walks hundreds of files inside one
invocation, which the root --timeout will otherwise cut short. Pass a generous
--timeout for a full run, or bound the work with --max-releases and re-run.
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli sync --full --timeout 6h
  pbs-pp-cli sync --max-releases 5
  pbs-pp-cli sync --from 2026-01-01 --kind spi --timeout 30m
  pbs-pp-cli sync --refetch --from 2026-09-03 --to 2026-09-03
`, "\n"),
		Annotations: map[string]string{
			// sync writes to the local store and fetches upstream, so it is not
			// read-only and must not be hinted as such.
			"pp:typed-exit-codes": "0,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Live dogfood runs every command against the real site inside a
			// flat per-command timeout, so a full backfill cannot fit. Curtail
			// the work rather than substituting mock data: this is a read path
			// and it must still hit the network.
			if cliutil.IsDogfoodEnv() && (full || maxReleases == 0 || maxReleases > 1) {
				full = false
				maxReleases = 1
			}
			if maxReleases == 0 && !full {
				maxReleases = 10
			}

			dbPath = panelDBPath(dbPath)
			db, err := openPanel(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			fc := pbsfetch.New(flags.timeout, 2)
			idx, _, err := fetchIndex(ctx, fc)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			nRel, nFiles, err := upsertReleaseIndex(ctx, db.DB(), idx)
			if err != nil {
				return fmt.Errorf("persist release index: %w", err)
			}
			env := syncEnvelope{IndexReleases: nRel, IndexFiles: nFiles, Outcomes: []syncOutcome{}}

			work := selectReleases(idx, kind, from, to)
			done := map[string]bool{}
			if !refetch {
				done, err = loadCompleted(ctx, db.DB())
				if err != nil {
					return err
				}
			}

			var pending []pbsparse.Release
			for _, r := range work {
				if !refetch && done[r.AsOfKey()+"|"+string(r.Kind)] {
					env.Skipped++
					continue
				}
				pending = append(pending, r)
			}
			if maxReleases > 0 && len(pending) > maxReleases {
				env.Remaining = len(pending) - maxReleases
				pending = pending[:maxReleases]
			}

			var hardErr error
			var coverageErr error
			for _, r := range pending {
				env.Attempted++
				ok := true

				// --- annexure -------------------------------------------------
				if f, found := pickFile(r, pbsparse.RoleAnnexure, prefer); found {
					o := syncOutcome{AsOf: r.AsOfKey(), Kind: string(r.Kind), Role: "annexure"}
					res, ferr := fc.Get(ctx, f.URL)
					switch {
					case ferr == nil:
						var a *pbsparse.Annexure
						var perr error
						switch {
						case res.LooksXLSX():
							o.Parser = "xlsx"
							a, perr = pbsparse.ParseAnnexureXLSX(res.Body, r.AsOfKey())
						case res.LooksPDF():
							o.Parser = "pdf"
							a, perr = pbsparse.ParseAnnexurePDF(res.Body, r.AsOfKey())
						default:
							perr = fmt.Errorf("unrecognised content type %q", res.ContentType)
						}
						if perr != nil {
							o.State, o.Note, ok = covUnparsed, perr.Error(), false
							coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "annexure", f.URL,
								covUnparsed, res.Status, res.SHA256, len(res.Body), 0, pbsparse.StateCensus{}, o.Parser, perr.Error()))
						} else {
							n, werr := persistAnnexure(ctx, db.DB(), a, o.Parser, string(r.Kind))
							if werr != nil {
								return fmt.Errorf("persist annexure %s: %w", r.AsOfKey(), werr)
							}
							env.PriceRows += n
							o.State, o.Rows, o.Bytes = covFetched, n, len(res.Body)
							if n == 0 {
								o.State = covEmpty
							}
							coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "annexure", f.URL,
								o.State, res.Status, res.SHA256, len(res.Body), n, a.Census, o.Parser, ""))
						}
					case errors.Is(ferr, pbsfetch.ErrNotFound):
						o.State, o.Note, ok = covRot, "upstream 404", false
						coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "annexure", f.URL,
							covRot, 404, "", 0, 0, pbsparse.StateCensus{}, "", "upstream 404"))
					default:
						o.State, o.Note, ok = covError, ferr.Error(), false
						hardErr = ferr
						coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "annexure", f.URL,
							covError, 0, "", 0, 0, pbsparse.StateCensus{}, "", ferr.Error()))
					}
					env.Outcomes = append(env.Outcomes, o)
				}

				// --- executive report ----------------------------------------
				if f, found := pickFile(r, pbsparse.RoleReport, "xlsx"); found && strings.EqualFold(f.Ext, "xlsx") {
					o := syncOutcome{AsOf: r.AsOfKey(), Kind: string(r.Kind), Role: "report", Parser: "xlsx"}
					res, ferr := fc.Get(ctx, f.URL)
					switch {
					case ferr == nil:
						rep, perr := pbsparse.ParseReportXLSX(res.Body, r.AsOfKey())
						if perr != nil {
							// A release is TWO files. A report that will not parse
							// means the weight vector and quintile index are absent,
							// so the release is incomplete — previously it was
							// counted complete, and loadCompleted (which keys on the
							// annexure row) would then skip it forever.
							o.State, o.Note, ok = covUnparsed, perr.Error(), false
							coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "report", f.URL,
								covUnparsed, res.Status, res.SHA256, len(res.Body), 0, pbsparse.StateCensus{}, "xlsx", perr.Error()))
						} else {
							n, werr := persistReport(ctx, db.DB(), rep, string(r.Kind))
							if werr != nil {
								return fmt.Errorf("persist report %s: %w", r.AsOfKey(), werr)
							}
							env.WeightRows += n
							o.State, o.Rows, o.Bytes = covFetched, n, len(res.Body)
							coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "report", f.URL,
								covFetched, res.Status, res.SHA256, len(res.Body), n, rep.Census, "xlsx", strings.Join(rep.Notes, "; ")))
						}
					case errors.Is(ferr, pbsfetch.ErrNotFound):
						// Upstream rot is advisory: the file is gone and re-running
						// will not bring it back, so it must not block a clean run.
						o.State, o.Note = covRot, "upstream 404"
						coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "report", f.URL,
							covRot, 404, "", 0, 0, pbsparse.StateCensus{}, "", "upstream 404"))
					default:
						o.State, o.Note, ok = covError, ferr.Error(), false
						hardErr = ferr
						coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "report", f.URL,
							covError, 0, "", 0, 0, pbsparse.StateCensus{}, "", ferr.Error()))
					}
					env.Outcomes = append(env.Outcomes, o)
				} else {
					// A release is TWO files, and only the .xlsx report is
					// parseable today (there is no ParseReportPDF). Falling
					// through this branch silently left `ok` true AND wrote no
					// report row at all, so a release whose report is a PDF was
					// recorded as complete while holding no weight vector, no
					// weight totals and no quintile indices — and `coverage`
					// could not name the hole either, because there was nothing
					// to name it with. Record the hole explicitly.
					// Record a row only when the report FILE EXISTS but cannot
					// be read. When the index lists no report file at all —
					// which is every CPI month, whose review key classifies as
					// unknown rather than report — there is no observation to
					// record, and inventing one would claim an upstream 404 for
					// a file that never existed. loadCompleted consults
					// pbs_release_file for that case instead.
					if found {
						ext := strings.ToLower(f.Ext)
						note := "report published only as ." + ext + "; no non-xlsx report parser"
						// covNotFetched, not covUnparsed: nothing was fetched and
						// no parse was attempted, so claiming a parse failure
						// would overstate what we know. These states are
						// deliberately never collapsed.
						o := syncOutcome{AsOf: r.AsOfKey(), Kind: string(r.Kind), Role: "report", Parser: ext}
						o.State, o.Note = covNotFetched, note
						coverageErr = orFirst(coverageErr, recordCoverage(ctx, db.DB(), r.AsOfKey(), string(r.Kind), "report", f.URL,
							covNotFetched, 0, "", 0, 0, pbsparse.StateCensus{}, ext, note))
						env.Outcomes = append(env.Outcomes, o)
					}
				}

				if ok {
					env.Completed++
				}
			}

			env.Incomplete = env.Remaining > 0 || env.Completed < env.Attempted
			if env.Incomplete {
				env.ResumeCommand = fmt.Sprintf("pbs-pp-cli sync --timeout 6h --db %s", dbPath)
				env.Note = "run is incomplete; re-run the resume command to continue"
			}

			// The summary is printed BEFORE any non-zero exit, so an interrupted
			// backfill always tells the caller how to resume.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), env, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "index: %d releases, %d files\n", env.IndexReleases, env.IndexFiles)
				fmt.Fprintf(cmd.OutOrStdout(), "attempted %d, completed %d, skipped %d already present, %d remaining\n",
					env.Attempted, env.Completed, env.Skipped, env.Remaining)
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %d price rows and %d weight rows\n", env.PriceRows, env.WeightRows)
				for _, o := range env.Outcomes {
					if o.State != covFetched {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s %s %s: %s %s\n", o.AsOf, o.Kind, o.Role, o.State, o.Note)
					}
				}
				if env.Incomplete {
					fmt.Fprintf(cmd.OutOrStdout(), "\nINCOMPLETE. resume with:\n  %s\n", env.ResumeCommand)
				}
			}

			// Gate narrowly. A remaining backlog or a failed fetch is a real
			// failure; upstream rot on a single file is recorded and advisory,
			// so a clean run over a corpus that contains known-dead links does
			// not start failing.
			if coverageErr != nil {
				// Prices were committed but their provenance was not. Silently
				// succeeding here would leave `coverage` reporting the release as
				// never fetched and `revisions` never re-checking it.
				return apiErr(fmt.Errorf("sync wrote data but failed to record coverage, so provenance is incomplete: %w", coverageErr))
			}
			if hardErr != nil {
				return apiErr(fmt.Errorf("sync incomplete: %w", hardErr))
			}
			if env.Remaining > 0 {
				return apiErr(fmt.Errorf("sync incomplete: %d releases remaining; re-run to continue", env.Remaining))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to a series: spi (weekly) or cpi (monthly)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest release date to sync, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "Latest release date to sync, YYYY-MM-DD")
	cmd.Flags().BoolVar(&full, "full", false, "Sync every release in the index (use a generous --timeout)")
	cmd.Flags().IntVar(&maxReleases, "max-releases", 0, "Maximum releases to fetch this run (0 with --full means no cap; otherwise defaults to 10)")
	cmd.Flags().BoolVar(&refetch, "refetch", false, "Re-fetch releases already recorded locally")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	cmd.Flags().StringVar(&prefer, "prefer", "xlsx", "Preferred annexure format when both exist: xlsx or pdf")
	return cmd
}

// orFirst keeps the first non-nil error, so a later success cannot mask an
// earlier failure.
func orFirst(first, next error) error {
	if first != nil {
		return first
	}
	return next
}

// selectReleases applies the kind and date filters, oldest first so a bounded
// run extends the panel backwards deterministically.
func selectReleases(idx *pbsparse.Index, kind, from, to string) []pbsparse.Release {
	var src []pbsparse.Release
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "spi", "weekly", "spi-weekly":
		src = idx.Weekly
	case "cpi", "monthly", "cpi-monthly":
		src = idx.Monthly
	default:
		src = idx.All()
	}
	out := make([]pbsparse.Release, 0, len(src))
	for _, r := range src {
		k := r.AsOfKey()
		if from != "" && k < from {
			continue
		}
		if to != "" && k > to {
			continue
		}
		out = append(out, r)
	}
	// Newest first: the most recent release is the one a caller most often
	// wants, and it is also the one most at risk of disappearing upstream.
	return out
}

// pickFile chooses a release file, preferring an extension when available.
func pickFile(r pbsparse.Release, role pbsparse.FileRole, prefer string) (pbsparse.ReleaseFile, bool) {
	if prefer != "" {
		if f, ok := r.File(role, prefer); ok && strings.EqualFold(f.Ext, prefer) {
			return f, true
		}
	}
	return r.File(role, "")
}

// loadCompleted returns the releases that are already fully recorded, so a
// re-run resumes instead of starting over.
//
// A release is TWO files, so a fetched annexure ALONE is not proof it is done.
// Resume must also see a terminal report row: fetched, gone upstream, or
// present-but-unparseable. Keying on the annexure alone meant a release whose
// annexure committed but whose report then failed to fetch, parse, or persist
// kept its prices and lost its weight vector and quintile indices for good,
// because the next ordinary sync skipped it and only --refetch would revisit
// it.
//
// Only terminal states count: fetched, gone upstream, unparseable, or a
// report we deliberately did not fetch because no parser accepts its format.
// A transport error, an empty read, or no report row at all — the shape left
// behind when a run dies between the two files — stays deliberately eligible
// for another attempt, because treating those as complete is how a partial
// panel becomes permanent.
//
// A release whose INDEX lists no report file needs no report row at all. That
// is every CPI month, and requiring a placeholder row would mean recording an
// observation about a file that does not exist.
func loadCompleted(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT a.as_of, a.kind
		   FROM pbs_coverage a
		   LEFT JOIN pbs_coverage r
		     ON r.as_of = a.as_of AND r.kind = a.kind AND r.role = 'report'
		  WHERE a.role = 'annexure' AND a.state = ?
		    AND (
		          r.state IN (?, ?, ?, ?)
		          OR NOT EXISTS (
		               SELECT 1 FROM pbs_release_file f
		                WHERE f.as_of = a.as_of AND f.kind = a.kind AND f.role = 'report'
		             )
		        )`,
		covFetched, covFetched, covRot, covUnparsed, covNotFetched)
	if err != nil {
		return nil, fmt.Errorf("load coverage: %w", err)
	}
	out := map[string]bool{}
	for rows.Next() {
		var asOf, kind string
		if err := rows.Scan(&asOf, &kind); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan coverage: %w", err)
		}
		out[asOf+"|"+kind] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate coverage: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
