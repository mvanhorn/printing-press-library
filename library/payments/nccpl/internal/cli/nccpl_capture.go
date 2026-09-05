package cli

// pp:data-source live

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

// Browser-assisted capture.
//
// This is NOT the CLI's transport. Every other command reads the local store and never
// opens a browser. `capture` exists because NCCPL's Cloudflare gate cannot be passed by
// replaying a clearance cookie -- established across matched TLS fingerprints, both HTTP
// versions, and every cookie combination (proofs/cloudflare-investigation.md). A real
// headed Chrome solves the challenge itself, so it is used as an explicit, occasional
// acquisition step, in the same spirit as `auth login --chrome`: throwaway profile,
// launched on demand, killed and deleted afterwards, never the operator's daily profile.
//
// With --headless no window appears: Chrome's own headless User-Agent token is what the
// origin hard-blocks, and pinning the normal token restores the self-solving challenge
// (see nccpl_cdp.go). Without --headless a window appears while the capture runs.

const nccplMarketInfoURL = "https://www.nccpl.com.pk/market-information"

type nccplCaptureRow struct {
	Resource string `json:"resource"`
	Date     string `json:"date"`
	Rows     int    `json:"rows"`
	Status   int    `json:"http_status"`
	Error    string `json:"error,omitempty"`
}

type nccplCaptureView struct {
	Captured  []nccplCaptureRow `json:"captured"`
	TotalRows int               `json:"total_rows"`
	Dates     int               `json:"dates"`
	Resources int               `json:"resources"`
	DBPath    string            `json:"db_path"`
	Note      string            `json:"note,omitempty"`
}

// nccplPageFetch is the shape returned by the in-page fetch helper.
type nccplPageFetch struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
	Error  string          `json:"error"`
}

func newNCCPLCaptureCmd(flags *rootFlags) *cobra.Command {
	var (
		resourcesCSV string
		fromDate     string
		toDate       string
		onDate       string
		latestOnly   bool
		refresh      bool
		keepProfile  string
		solveWait    time.Duration
		dbPath       string
		launch       bool
		stride       int
		headless     bool
	)

	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Fetch gated NCCPL datasets through a controlled browser and store them",
		Long: strings.Trim(`
Acquire the NCCPL datasets that no HTTP client can reach -- leverage positions, VAR
margins and free float, settlement -- by driving a controlled Chrome, then store them
in the same local database every other command reads.

Why a browser: NCCPL's clearance cookie cannot be replayed outside a browser. That is
measured, not assumed -- a byte-exact TLS fingerprint match to the operator's own
Chrome, with valid cookies, over both HTTP/2 and HTTP/3, receives the same challenge as
sending no cookies at all. A real headed Chrome solves the challenge on its own.

What this does NOT do: it is not the CLI's transport. 'panel', 'verify', 'coverage',
'leverage', 'universe' and 'risk-changes' never open a browser; they read the store.
'capture' is an explicit acquisition step, like 'auth login --chrome'.

Mechanics: a throwaway Chrome profile is created, used, then killed and deleted. Your
normal Chrome profile is never touched. With --headless no window appears, which is what
a scheduled capture should use; without it a window appears while the capture runs.

--headless must solve the challenge from a FRESH profile every run, so it cannot be
combined with --profile. Handing a headless Chrome a clearance that was solved earlier --
whether by reusing its profile directory or by injecting the cookies -- is answered with a
hard block ("Sorry, you have been blocked"), which is worse than the ordinary challenge.
Solving fresh costs about 15 seconds and is what --headless is tuned for.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli capture --resources var-margins --latest-only
  nccpl-pp-cli capture --resources mts,mfs,msf,slb --on 2026-09-04 --launch
  nccpl-pp-cli capture --resources var-margins --from 2026-09-01 --to 2026-09-04 --launch
  nccpl-pp-cli capture --resources var-margins --latest-only --launch --headless
`, "\n"),
		Annotations: map[string]string{
			// capture fetches gated NCCPL datasets through a controlled browser and writes them
			// to the local store. It never writes to NCCPL.
			"mcp:local-write": "true",
			"pp:happy-args":   "--resources=var-margins;--latest-only",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "capture")
			}
			// Never drive a browser inside a verification or dogfood harness: it would
			// open a real window and hit the live origin during an automated run.
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "launch a browser for capture")
			}

			selected, err := nccplSelectResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			gated := make([]nccplResource, 0, len(selected))
			for _, r := range selected {
				if !r.External {
					gated = append(gated, r)
				}
			}
			if len(gated) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no gated NCCPL resources selected; 'flows' is fetched by 'nccpl-pp-cli flows'"))
			}
			if onDate != "" {
				fromDate, toDate = onDate, onDate
			}
			if !latestOnly && (fromDate == "" || toDate == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --on, or --from and --to, or --latest-only"))
			}

			// Opening a browser is a visible side effect, so it is opt-in: without
			// --launch this reports what it would capture and changes nothing. That
			// also keeps automated probes from putting a window on someone's screen.
			if !launch {
				names := make([]string, 0, len(gated))
				for _, r := range gated {
					names = append(names, r.Name)
				}
				window := "each resource's latest published date"
				if !latestOnly {
					window = fromDate + " .. " + toDate
				}
				plan := map[string]any{
					"would_launch_browser": true,
					"resources":            names,
					"window":               window,
					"hint":                 "re-run with --launch to open a controlled Chrome window and capture",
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would launch a controlled Chrome and capture:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  resources: %s\n  window:    %s\n", strings.Join(names, ", "), window)
				fmt.Fprintf(cmd.OutOrStdout(), "\nre-run with --launch to actually capture\n")
				return nil
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

			// --headless + --profile is measured-broken, not merely discouraged: a
			// headless Chrome handed a previously-solved clearance (by profile reuse
			// or cookie injection) gets the hard WAF block rather than a challenge.
			// Refusing here stops a scheduled job from failing that way every run.
			if headless && strings.TrimSpace(keepProfile) != "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--headless cannot be combined with --profile: a headless Chrome given a previously-solved clearance is hard-blocked, so it must solve from a fresh profile each run (about 15s). Drop --profile, or drop --headless to reuse a solved profile with a visible window"))
			}
			if headless {
				fmt.Fprintf(cmd.ErrOrStderr(), "launching a controlled headless Chrome (no window)...\n")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "launching a controlled Chrome (throwaway profile; a window will appear)...\n")
			}
			br, err := nccplLaunchChrome(ctx, keepProfile, headless)
			if err != nil {
				return err
			}
			defer func() { _ = br.Close(keepProfile != "") }()

			if err := br.Navigate(ctx, nccplMarketInfoURL); err != nil {
				return fmt.Errorf("navigating: %w", err)
			}
			csrf, err := nccplWaitForClearance(ctx, cmd, br, solveWait)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "challenge cleared; capturing...\n")

			view := nccplCaptureView{Captured: make([]nccplCaptureRow, 0), DBPath: dbPath}
			seenDates := map[string]bool{}
			consecutive := 0
			aborted := false
			observed := time.Now()

			for _, r := range gated {
				if aborted {
					break
				}
				dates := []string{}
				if latestOnly {
					d, err := nccplPageLatestDate(ctx, br, r)
					if err != nil {
						view.Captured = append(view.Captured, nccplCaptureRow{Resource: r.Name, Error: err.Error()})
						continue
					}
					dates = []string{d}
				} else {
					dates, err = nccplSessionDates(fromDate, toDate)
					if err != nil {
						_ = cmd.Usage()
						return usageErr(err)
					}
					// Sample every Nth session. Slow-moving fields do not need every day:
					// free float changes for ~0.2% of symbols between consecutive sessions
					// but ~29% over six months, so a stride localises each change to N
					// sessions at a fraction of the fetches. The final session is always
					// kept so the most recent state is never missed.
					if stride > 1 && len(dates) > 1 {
						sampled := make([]string, 0, len(dates)/stride+1)
						for i := 0; i < len(dates); i += stride {
							sampled = append(sampled, dates[i])
						}
						if sampled[len(sampled)-1] != dates[len(dates)-1] {
							sampled = append(sampled, dates[len(dates)-1])
						}
						dates = sampled
					}
				}
				if !refresh {
					covered, err := store.NCCPLCoveredDates(ctx, db, r.Name)
					if err != nil {
						return err
					}
					have := map[string]bool{}
					for _, c := range covered {
						have[c.Date] = true
					}
					kept := dates[:0]
					for _, d := range dates {
						if !have[d] {
							kept = append(kept, d)
						}
					}
					dates = kept
				}
				for _, d := range dates {
					rows, status, err := nccplPageFetchData(ctx, br, r, d, csrf)
					row := nccplCaptureRow{Resource: r.Name, Date: d, Status: status}
					if err != nil {
						row.Error = err.Error()
						view.Captured = append(view.Captured, row)
						// A transport-level CDP failure means the browser is gone; every
						// subsequent call would fail identically. Stop rather than emit
						// thousands of copies of the same error. Work already stored is
						// kept, and re-running resumes from the coverage ledger.
						if nccplBrowserGone(err) {
							consecutive++
							if consecutive >= 3 {
								view.Note = "browser connection lost after " +
									fmt.Sprint(view.Dates) + " date(s); re-run to resume from the coverage ledger"
								aborted = true
								break
							}
						} else {
							consecutive = 0
						}
						continue
					}
					consecutive = 0
					if err := store.SaveNCCPLDate(ctx, db, r.Name, d, rows, observed); err != nil {
						return err
					}
					row.Rows = len(rows)
					view.TotalRows += len(rows)
					seenDates[d] = true
					view.Captured = append(view.Captured, row)
				}
				view.Resources++
			}
			view.Dates = len(seenDates)
			if view.TotalRows == 0 {
				view.Note = "nothing captured; if every row errored the session may have lapsed -- re-run, and check --from/--to name real settlement dates"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, c := range view.Captured {
				if c.Error != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "%-20s %-12s ERROR %s\n", c.Resource, c.Date, c.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %5d rows (HTTP %d)\n", c.Resource, c.Date, c.Rows, c.Status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\ncaptured %d row(s) across %d date(s) into %s\n",
				view.TotalRows, view.Dates, dbPath)
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated gated resources to capture; empty means all NCCPL resources")
	cmd.Flags().StringVar(&onDate, "on", "", "Capture a single settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&fromDate, "from", "", "First settlement date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Last settlement date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&latestOnly, "latest-only", false, "Capture each resource's most recent published date")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-capture dates already in the coverage ledger")
	cmd.Flags().StringVar(&keepProfile, "profile", "", "Reuse this Chrome profile directory instead of a throwaway one (kept, not deleted)")
	cmd.Flags().DurationVar(&solveWait, "solve-wait", 60*time.Second, "How long to wait for the browser to clear the challenge")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&stride, "stride", 1, "Capture every Nth session in the range (5 = roughly weekly); the last session is always included")
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the controlled browser and capture; without it this only reports what it would do")
	cmd.Flags().BoolVar(&headless, "headless", false, "Run the controlled Chrome without a window (User-Agent pinned to the normal Chrome token); use for scheduled, unattended captures")
	return cmd
}

// nccplWaitForClearance polls until the page is the real site and exposes a CSRF token.
func nccplWaitForClearance(ctx context.Context, cmd *cobra.Command, br *nccplBrowser, limit time.Duration) (string, error) {
	deadline := time.Now().Add(limit)
	warned := false
	for time.Now().Before(deadline) {
		var probe struct {
			Title string `json:"title"`
			CSRF  string `json:"csrf"`
		}
		expr := `(()=>{const m=document.querySelector('meta[name=csrf-token]');` +
			`return {title:document.title||'',csrf:m?m.content:''};})()`
		if err := br.Eval(ctx, expr, &probe); err == nil && probe.CSRF != "" {
			return probe.CSRF, nil
		}
		if !warned && time.Now().After(deadline.Add(-limit/2)) {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"still on the challenge; if the window shows a checkbox, click it\n")
			warned = true
		}
		time.Sleep(time.Second)
	}
	return "", authErr(fmt.Errorf("browser did not clear the challenge within %s; re-run, or raise --solve-wait", limit))
}

// nccplPageLatestDate reads a resource's own latest published date from page context.
func nccplPageLatestDate(ctx context.Context, br *nccplBrowser, r nccplResource) (string, error) {
	expr := fmt.Sprintf(
		`fetch('/api/%s/latest-date',{headers:{'Accept':'*/*'}})`+
			`.then(r=>r.json()).then(j=>j.latest_date||'').catch(()=>'')`, r.Segment)
	var d string
	if err := br.Eval(ctx, expr, &d); err != nil {
		return "", fmt.Errorf("%s latest-date: %w", r.Name, err)
	}
	if strings.TrimSpace(d) == "" {
		return "", fmt.Errorf("%s: no latest-date returned", r.Name)
	}
	return d, nil
}

// nccplPageFetchData POSTs one settlement date from page context and parses the rows.
//
// The request body uses this resource's own date encoding: the API accepts three
// different formats depending on the endpoint family, and sending the wrong one returns
// an empty array with HTTP 200 rather than an error.
func nccplPageFetchData(ctx context.Context, br *nccplBrowser, r nccplResource, date, csrf string) ([]store.NCCPLRow, int, error) {
	body, err := json.Marshal(nccplRequestBody(r, date))
	if err != nil {
		return nil, 0, err
	}
	expr := fmt.Sprintf(
		`(async()=>{try{const r=await fetch('/api/%s/data',{method:'POST',`+
			`headers:{'Content-Type':'application/json','X-CSRF-TOKEN':%q},body:%q});`+
			`const t=await r.text();let b=null;try{b=JSON.parse(t)}catch(e){}`+
			`return {status:r.status,body:b,error:b?'':t.slice(0,120)};`+
			`}catch(e){return {status:0,body:null,error:String(e)}}})()`,
		r.Segment, csrf, string(body))

	var out nccplPageFetch
	if err := br.Eval(ctx, expr, &out); err != nil {
		return nil, 0, fmt.Errorf("%s %s: %w", r.Name, date, err)
	}
	if out.Status != 200 {
		detail := out.Error
		if detail == "" {
			detail = fmt.Sprintf("HTTP %d", out.Status)
		}
		return nil, out.Status, fmt.Errorf("%s %s: %s", r.Name, date, detail)
	}
	if len(out.Body) == 0 {
		return nil, out.Status, fmt.Errorf("%s %s: empty response", r.Name, date)
	}
	rows, err := nccplRowsFromEnvelope(out.Body, r)
	if err != nil {
		return nil, out.Status, fmt.Errorf("%s %s: %w", r.Name, date, err)
	}
	return rows, out.Status, nil
}

// nccplBrowserGone reports whether an error means the controlled browser is no longer
// reachable, as opposed to one date simply failing. Once the CDP socket is broken every
// later call fails the same way, so the caller stops instead of repeating the error for
// every remaining date.
func nccplBrowserGone(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, marker := range []string{"cdp read:", "cdp write:", "use of closed network connection", "broken pipe"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
