package cli

// pp:data-source live

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// nccplArchiveStart is the documented first date of the FIPI/LIPI archive.
// Other resources begin later; walking back past their start simply records
// fetched-and-empty dates, which is the honest representation of "no data here".
const nccplArchiveStart = "2015-12-09"

type nccplSyncResourceResult struct {
	Resource   string   `json:"resource"`
	LatestDate string   `json:"latest_date,omitempty"`
	Requested  int      `json:"dates_requested"`
	Fetched    int      `json:"dates_fetched"`
	Skipped    int      `json:"dates_skipped_already_stored"`
	Rows       int      `json:"rows_stored"`
	EmptyDates int      `json:"dates_empty"`
	Failed     int      `json:"dates_failed"`
	Errors     []string `json:"errors,omitempty"`

	// Set when this resource's walk aborted because the upstream throttled us.
	// Unfetched is the exact remainder this run owes and did not store -- never
	// a guess, and never back-filled from a neighbouring date.
	RateLimited   bool     `json:"rate_limited,omitempty"`
	RetryAfter    string   `json:"retry_after,omitempty"`
	Unfetched     []string `json:"dates_unfetched,omitempty"`
	UnfetchedFrom string   `json:"dates_unfetched_from,omitempty"`
	UnfetchedTo   string   `json:"dates_unfetched_to,omitempty"`
}

type nccplSyncView struct {
	Resources []nccplSyncResourceResult `json:"resources"`
	From      string                    `json:"from,omitempty"`
	To        string                    `json:"to,omitempty"`
	TotalRows int                       `json:"total_rows_stored"`
	DBPath    string                    `json:"db_path"`

	// Every non-external resource is served by the same NCCPL host, so a
	// throttle applies to all of them. When one resource is rate limited the run
	// stops and the resources it never got to are listed here rather than being
	// attempted against a host that is already refusing us.
	RateLimited           bool     `json:"rate_limited,omitempty"`
	ResourcesNotAttempted []string `json:"resources_not_attempted,omitempty"`
}

// nccplSyncThrottleErr returns the exit-code-7 error for a run that stopped on an
// upstream throttle, and nil for a run that did not. It is called only after the
// run has printed what it committed, so a throttled sync reports partial coverage
// through both its output and its exit status.
func nccplSyncThrottleErr(view nccplSyncView) error {
	if !view.RateLimited {
		return nil
	}
	unfetched, retryAfter := 0, ""
	for _, r := range view.Resources {
		unfetched += len(r.Unfetched)
		if retryAfter == "" && r.RetryAfter != "" {
			retryAfter = r.RetryAfter
		}
	}
	what := "sync"
	if n := len(view.ResourcesNotAttempted); n > 0 {
		what = fmt.Sprintf("sync (%d resource(s) not attempted)", n)
	}
	return nccplRateLimitAbortErr(what, retryAfter, unfetched)
}

func newNCCPLSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		resourcesCSV string
		fromDate     string
		toDate       string
		full         bool
		latestOnly   bool
		refresh      bool
		maxDates     int
		dbPath       string
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Backfill NCCPL settlement data into the local store",
		Long: strings.Trim(`
Fetch NCCPL data one settlement date at a time and store it locally with a coverage
ledger.

Every date attempted is recorded, including dates that legitimately returned no rows,
so a later gap audit can tell "fetched and empty" apart from "never fetched". Nothing
is interpolated or forward-filled.

Sync always requests a single session per call, even for the endpoints that accept a
date range: the flow rows carry no date of their own, so a wider window returns one
aggregate rather than per-session rows. Use the generated 'fipi data' / 'lipi data'
commands when you want a period aggregate instead.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli sync --resources fipi,lipi --latest-only
  nccpl-pp-cli sync --resources var-margins --from 2026-08-01 --to 2026-09-04
  nccpl-pp-cli sync --resources fipi,lipi --full
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--resources=fipi;--latest-only;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}

			selected, err := nccplSelectResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if !full && !latestOnly && (fromDate == "" || toDate == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --from and --to, or --latest-only, or --full"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureNCCPLSchema(ctx, db); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Live dogfood runs against the real API under a flat per-command
			// timeout, so curtail the walk rather than substituting mock data.
			if cliutil.IsDogfoodEnv() {
				latestOnly = true
				full = false
				if len(selected) > 1 {
					selected = selected[:1]
				}
			}

			view := nccplSyncView{Resources: make([]nccplSyncResourceResult, 0, len(selected)), DBPath: dbPath}
			view.From, view.To = fromDate, toDate

			var throttled *cliutil.RateLimitError
			for _, r := range selected {
				if view.RateLimited {
					// Same host as the resource that was just throttled; do not
					// keep knocking.
					view.ResourcesNotAttempted = append(view.ResourcesNotAttempted, r.Name)
					continue
				}
				res := nccplSyncResourceResult{Resource: r.Name, Errors: make([]string, 0)}
				if r.External {
					// Not an NCCPL origin; it has its own fetch command.
					res.Errors = append(res.Errors, "external resource: use 'nccpl-pp-cli flows' to fetch this")
					view.Resources = append(view.Resources, res)
					continue
				}

				latest, err := nccplLatestDate(ctx, c, r)
				if err != nil {
					if errors.As(err, &throttled) {
						view.RateLimited = true
						res.RateLimited = true
						res.RetryAfter = throttled.RetryAfter.String()
						res.Errors = append(res.Errors, err.Error())
						view.Resources = append(view.Resources, res)
						continue
					}
					res.Errors = append(res.Errors, err.Error())
					res.Failed++
					view.Resources = append(view.Resources, res)
					continue
				}
				res.LatestDate = latest

				var dates []string
				switch {
				case latestOnly:
					dates = []string{latest}
				case full:
					dates, err = nccplSessionDates(nccplArchiveStart, latest)
				default:
					end := toDate
					if end > latest {
						end = latest
					}
					dates, err = nccplSessionDates(fromDate, end)
				}
				if err != nil {
					_ = cmd.Usage()
					return usageErr(err)
				}
				if maxDates > 0 && len(dates) > maxDates {
					dates = dates[len(dates)-maxDates:]
				}
				res.Requested = len(dates)

				stored := map[string]bool{}
				if !refresh {
					covered, err := store.NCCPLCoveredDates(ctx, db, r.Name)
					if err != nil {
						return err
					}
					for _, c := range covered {
						stored[c.Date] = true
					}
				}

				for i, d := range dates {
					if stored[d] {
						res.Skipped++
						continue
					}
					rows, _, err := nccplFetchDate(ctx, c, r, d)
					if err != nil {
						// Throttled: stop this resource's walk rather than
						// spending the rest of the archive on requests the host
						// has already said it will refuse. Dates stored above
						// stay committed -- SaveNCCPLDate is per-date and
						// transactional -- and the remainder is named exactly.
						if errors.As(err, &throttled) {
							view.RateLimited = true
							res.RateLimited = true
							res.RetryAfter = throttled.RetryAfter.String()
							res.Unfetched = nccplUnfetchedDates(dates[i:], stored)
							if len(res.Unfetched) > 0 {
								res.UnfetchedFrom = res.Unfetched[0]
								res.UnfetchedTo = res.Unfetched[len(res.Unfetched)-1]
							}
							res.Errors = append(res.Errors, err.Error())
							break
						}
						res.Failed++
						if len(res.Errors) < 5 {
							res.Errors = append(res.Errors, err.Error())
						}
						continue
					}
					if err := store.SaveNCCPLDate(ctx, db, r.Name, d, rows, time.Now()); err != nil {
						return err
					}
					res.Fetched++
					res.Rows += len(rows)
					if len(rows) == 0 {
						res.EmptyDates++
					}
				}
				view.TotalRows += res.Rows
				view.Resources = append(view.Resources, res)
			}

			// Report on every path, throttled or not: rows already committed
			// must stay visible even when the run ends in a non-zero exit.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return nccplSyncThrottleErr(view)
			}
			for _, r := range view.Resources {
				fmt.Fprintf(cmd.OutOrStdout(),
					"%-20s latest=%-12s requested=%-5d fetched=%-5d skipped=%-5d empty=%-4d failed=%-4d rows=%d\n",
					r.Resource, r.LatestDate, r.Requested, r.Fetched, r.Skipped, r.EmptyDates, r.Failed, r.Rows)
				for _, e := range r.Errors {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", r.Resource, e)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nstored %d rows into %s\n", view.TotalRows, dbPath)
			if view.RateLimited {
				for _, r := range view.Resources {
					if !r.RateLimited {
						continue
					}
					resume := ""
					if r.UnfetchedFrom != "" {
						resume = fmt.Sprintf("nccpl-pp-cli sync --resources %s --from %s --to %s",
							r.Resource, r.UnfetchedFrom, r.UnfetchedTo)
					}
					nccplPrintUnfetched(cmd.ErrOrStderr(), r.Resource, r.RetryAfter, r.Unfetched,
						r.UnfetchedFrom, r.UnfetchedTo, resume)
				}
				if len(view.ResourcesNotAttempted) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"  not attempted (same host, already throttled): %s\n",
						strings.Join(view.ResourcesNotAttempted, ", "))
				}
			}
			return nccplSyncThrottleErr(view)
		},
	}

	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to sync; empty means all ("+strings.Join(nccplResourceNames(), ", ")+")")
	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date to fetch (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date to fetch (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&full, "full", false, "Backfill from the start of the archive ("+nccplArchiveStart+") to each resource's latest published date")
	cmd.Flags().BoolVar(&latestOnly, "latest-only", false, "Fetch only each resource's most recent published date")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch dates already present in the coverage ledger")
	cmd.Flags().IntVar(&maxDates, "max-dates", 0, "Cap the number of dates fetched per resource, keeping the most recent (0 = no cap)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// nccplSelectResources resolves a comma-separated resource list, defaulting to all.
func nccplSelectResources(csv string) ([]nccplResource, error) {
	if strings.TrimSpace(csv) == "" {
		out := make([]nccplResource, len(nccplResources))
		copy(out, nccplResources)
		return out, nil
	}
	out := make([]nccplResource, 0)
	unknown := make([]string, 0)
	for _, name := range strings.Split(csv, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		r, ok := nccplResourceByName(name)
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		out = append(out, r)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown resource(s): %s\nvalid resources: %s",
			strings.Join(unknown, ", "), strings.Join(nccplResourceNames(), ", "))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--resources matched no resources")
	}
	return out, nil
}
