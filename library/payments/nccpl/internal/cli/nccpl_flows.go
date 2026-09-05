package cli

// pp:data-source live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// FIPI/LIPI sector flows via scstrade.
//
// NCCPL's own origin is behind a Cloudflare clearance gate that no non-browser client
// can pass (see proofs/cloudflare-investigation.md). scstrade republishes the same
// FIPI/LIPI sector flows over plain HTTPS with no gate, and the data checks out against
// NCCPL's own arithmetic: on a sampled date every sector row nets to zero across the
// investor classes and FIPI net equals -LIPI net, both to four decimals. `verify` runs
// those same checks on every date fetched here, so a provenance drift shows up as a
// failing invariant rather than a silent wrong number.
//
// This is a DIFFERENT SOURCE, not a drop-in for NCCPL. It is stored under its own
// resource name (`flows`) so it can never be mistaken for NCCPL-sourced rows, and it
// carries only FIPI/LIPI -- the leverage, VAR-margin and settlement datasets remain
// NCCPL-only and arrive through `ingest`.
const nccplFlowsEndpoint = "https://www.scstrade.com/FIPILIPI.aspx/loadfipisector"

// nccplFlowsArchiveStart is the earliest date the upstream archive answers for.
const nccplFlowsArchiveStart = "2016-08-01"

type nccplFlowsResult struct {
	Requested  int      `json:"dates_requested"`
	Fetched    int      `json:"dates_fetched"`
	Skipped    int      `json:"dates_skipped_already_stored"`
	Rows       int      `json:"rows_stored"`
	EmptyDates int      `json:"dates_empty"`
	Failed     int      `json:"dates_failed"`
	Errors     []string `json:"errors,omitempty"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Source     string   `json:"source"`
	DBPath     string   `json:"db_path"`

	// Set when the walk aborted because the upstream throttled us. Unfetched is
	// the exact remainder -- the dates this run owes and did not store. It is
	// never a guess: a rate-limited date is a missing observation, and the only
	// correct fix is to fetch it for real on a later run.
	RateLimited   bool     `json:"rate_limited,omitempty"`
	RetryAfter    string   `json:"retry_after,omitempty"`
	Unfetched     []string `json:"dates_unfetched,omitempty"`
	UnfetchedFrom string   `json:"dates_unfetched_from,omitempty"`
	UnfetchedTo   string   `json:"dates_unfetched_to,omitempty"`
}

func newNCCPLFlowsCmd(flags *rootFlags) *cobra.Command {
	var (
		fromDate string
		toDate   string
		full     bool
		refresh  bool
		maxDates int
		dbPath   string
	)

	cmd := &cobra.Command{
		Use:   "flows",
		Short: "Fetch FIPI/LIPI sector flows into the local store (no browser required)",
		Long: strings.Trim(`
Fetch daily FIPI/LIPI sector flows and store them locally, one settlement date per
request, with the same coverage ledger and vintage stamping the NCCPL paths use.

Source: scstrade, which republishes NCCPL's FIPI/LIPI numbers over plain HTTPS with no
Cloudflare gate. NCCPL's own origin cannot be reached by any non-browser HTTP client --
that is a measured result, documented in the run's proofs. Because this is a different
publisher, rows land under their own resource name (`+"`flows`"+`) and never blend with
NCCPL-sourced data.

Validate what you fetch: `+"`verify`"+` checks the two identities NCCPL's data must satisfy
(each sector nets to zero across investor classes; FIPI net = -LIPI net). A date that
fails is not trustworthy regardless of where it came from.

Archive begins around `+nccplFlowsArchiveStart+`; earlier dates return nothing and are
recorded as fetched-and-empty rather than silently skipped.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli flows --from 2026-08-01 --to 2026-09-04
  nccpl-pp-cli flows --full
  nccpl-pp-cli flows --from 2026-09-01 --to 2026-09-04 --json
`, "\n"),
		Annotations: map[string]string{
			// flows fetches FIPI/LIPI sector flows from scstrade and writes them to the local
			// store. It never writes to the upstream site.
			"mcp:local-write": "true",
			"pp:happy-args":   "--from=2026-09-03;--to=2026-09-04",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "flows")
			}
			if !full && (fromDate == "" || toDate == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --from and --to, or --full"))
			}
			if full {
				fromDate = nccplFlowsArchiveStart
				toDate = time.Now().Format("2006-01-02")
			}
			dates, err := nccplSessionDates(fromDate, toDate)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if cliutil.IsDogfoodEnv() && len(dates) > 1 {
				dates = dates[len(dates)-1:]
			}
			if maxDates > 0 && len(dates) > maxDates {
				dates = dates[len(dates)-maxDates:]
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

			res, _ := nccplResourceByName("flows")
			out := nccplFlowsResult{
				Requested: len(dates), From: fromDate, To: toDate,
				Source: nccplFlowsEndpoint, DBPath: dbPath, Errors: make([]string, 0),
			}

			stored := map[string]bool{}
			if !refresh {
				covered, err := store.NCCPLCoveredDates(ctx, db, "flows")
				if err != nil {
					return err
				}
				for _, c := range covered {
					stored[c.Date] = true
				}
			}

			client := &http.Client{Timeout: 30 * time.Second}
			var throttled *cliutil.RateLimitError
			for i, d := range dates {
				if stored[d] {
					out.Skipped++
					continue
				}
				rows, err := nccplFetchFlows(ctx, client, nccplFlowsEndpoint, res, d)
				if err != nil {
					// A throttle is not a per-date failure -- the same request
					// succeeds later. Continuing would walk the rest of the
					// archive (--full is ~2,600 weekdays) against a host that
					// just asked us to stop, and would report one throttle as
					// thousands of unrelated date failures. Stop here and name
					// the exact remainder instead.
					if errors.As(err, &throttled) {
						out.RateLimited = true
						out.RetryAfter = throttled.RetryAfter.String()
						out.Unfetched = nccplUnfetchedDates(dates[i:], stored)
						if len(out.Unfetched) > 0 {
							out.UnfetchedFrom = out.Unfetched[0]
							out.UnfetchedTo = out.Unfetched[len(out.Unfetched)-1]
						}
						out.Errors = append(out.Errors, err.Error())
						break
					}
					out.Failed++
					if len(out.Errors) < 5 {
						out.Errors = append(out.Errors, err.Error())
					}
					continue
				}
				if err := store.SaveNCCPLDate(ctx, db, "flows", d, rows, time.Now()); err != nil {
					return err
				}
				out.Fetched++
				out.Rows += len(rows)
				if len(rows) == 0 {
					out.EmptyDates++
				}
			}

			// The report is printed on every path, throttled or not: dates
			// already committed must be visible even when the run ends in a
			// non-zero exit.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
				if out.RateLimited {
					return nccplRateLimitAbortErr("flows", out.RetryAfter, len(out.Unfetched))
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"requested=%d fetched=%d skipped=%d empty=%d failed=%d rows=%d\n",
				out.Requested, out.Fetched, out.Skipped, out.EmptyDates, out.Failed, out.Rows)
			for _, e := range out.Errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", e)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nstored %d row(s) into %s under resource 'flows'\n", out.Rows, dbPath)
			if out.RateLimited {
				resume := ""
				if out.UnfetchedFrom != "" {
					resume = fmt.Sprintf("nccpl-pp-cli flows --from %s --to %s", out.UnfetchedFrom, out.UnfetchedTo)
				}
				nccplPrintUnfetched(cmd.ErrOrStderr(), "flows", out.RetryAfter, out.Unfetched,
					out.UnfetchedFrom, out.UnfetchedTo, resume)
				return nccplRateLimitAbortErr("flows", out.RetryAfter, len(out.Unfetched))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run 'nccpl-pp-cli verify' to check the arithmetic identities on what you just fetched\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&full, "full", false, "Backfill the whole archive from "+nccplFlowsArchiveStart+" to today")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch dates already in the coverage ledger")
	cmd.Flags().IntVar(&maxDates, "max-dates", 0, "Cap dates fetched, keeping the most recent (0 = no cap)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// nccplFetchFlows requests one settlement date. The upstream aggregates over a range,
// so from and to are always the same day; a wider window would return a single blended
// figure rather than a daily series.
func nccplFetchFlows(ctx context.Context, client *http.Client, endpoint string, res nccplResource, date string) ([]store.NCCPLRow, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", date, err)
	}
	us := t.Format("01/02/2006")
	body, err := json.Marshal(map[string]string{"date1": us, "date2": us})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flows %s: %w", date, err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Classify 429 before the generic non-200 branch. This path deliberately
	// stays off the shared internal/client (see the endpoint comment above), so
	// the typed classification that client already performs has to happen here
	// -- otherwise a throttle is indistinguishable from a malformed date, and
	// the caller's retry loop cannot tell "wait" from "this will never work".
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("flows %s: %w", date, cliutil.RateLimitFromResponse(resp, endpoint))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flows %s: HTTP %d", date, resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("flows %s: %w", date, err)
	}
	return nccplRowsFromEnvelope(raw, res)
}
