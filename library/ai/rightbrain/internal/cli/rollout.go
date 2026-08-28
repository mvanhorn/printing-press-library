// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

// Revision traffic split, configured versus observed.
//
// A Rightbrain task can serve several revisions at once, each with a weight in
// the task's active_revisions array. Nothing in the API reports whether that
// split actually happened: the task endpoint returns the intended weights and
// the run records carry the revision that served each request, but the two are
// never joined. So the failure mode this command exists for — a canary
// configured at 20% that is receiving 0% because it was never activated, or
// receiving 60% because another revision was retired — is invisible until
// someone counts runs by hand. This command joins the configured weights
// against the runs in the local mirror, and pairs each revision's observed
// share with the failure rate, mean credits, and p50/p95 latency it produced,
// because "the canary got its traffic" and "the canary is fine" are different
// questions and the second one is why anyone splits traffic in the first place.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// rolloutRun is one observed task run, reduced to the fields this command
// aggregates. ChargedCredits stays a string because the API serializes it as
// one ("9.00"); parsing is deferred to the aggregator so an unparseable value
// can be excluded from the mean instead of silently counted as zero.
type rolloutRun struct {
	RevisionID     string
	IsError        bool
	ChargedCredits string
	LatencySecs    float64
	HasLatency     bool
}

// rolloutRevision is one row of the report: what the revision was configured to
// receive, what it actually received, and what that traffic cost.
//
// ConfiguredShare and DriftPct are pointers so a revision whose configured
// weight is null or unreadable serializes them as JSON null. Substituting 0
// there would invent the single most alarming number this report can print —
// "configured 0%, observed 80%" for a revision that is serving exactly the
// traffic it was told to.
type rolloutRevision struct {
	RevisionID      string   `json:"revision_id"`
	Configured      bool     `json:"configured"`
	ConfiguredShare *float64 `json:"configured_share"`
	ObservedShare   float64  `json:"observed_share"`
	DriftPct        *float64 `json:"drift_pct"`
	Runs            int      `json:"runs"`
	FailureRate     float64  `json:"failure_rate"`
	MeanCredits     float64  `json:"mean_credits"`
	CreditSamples   int      `json:"credit_samples"`
	P50LatencySecs  float64  `json:"p50_latency_secs"`
	P95LatencySecs  float64  `json:"p95_latency_secs"`
	LatencySamples  int      `json:"latency_samples"`
}

type rolloutReport struct {
	TaskID          string            `json:"task_id,omitempty"`
	Window          string            `json:"window"`
	TotalRuns       int               `json:"total_runs"`
	ConfiguredFound bool              `json:"configured_found"`
	UndatedRuns     int               `json:"undated_runs,omitempty"`
	Revisions       []rolloutRevision `json:"revisions"`
	Note            string            `json:"note,omitempty"`
}

// rolloutActiveRevision decodes one entry of the task's active_revisions array.
// Weight is decoded as `any` because a weight that arrives as "20" rather than
// 20 must still count; a typed float64 field would fail the whole decode and
// report the split as unconfigured.
type rolloutActiveRevision struct {
	TaskRevisionID string `json:"task_revision_id"`
	Weight         any    `json:"weight"`
}

// rolloutParseFloat accepts the numeric shapes the API mixes for weights and
// credits — a JSON number, or a decimal string like "9.00" — and reports
// whether the value was usable. Callers exclude unusable values from means
// rather than substituting zero, which would drag an average toward 0 and read
// as "this revision got cheaper".
func rolloutParseFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return 0, false
		}
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

// rolloutNormalizeWeights converts raw active_revisions weights into shares of
// total traffic so they are comparable with an observed share. Dividing by the
// sum covers both conventions the API uses — weights that sum to 100 (percent)
// and weights that sum to 1 (fraction) — and also rescues a set that sums to
// anything else, which would otherwise produce a nonsense drift number.
//
// A nil weight means the API sent null (TaskActiveRevision.weight is nullable)
// or sent something unreadable. Such a revision is kept in the output but
// excluded from the denominator and given a nil share, because folding it in as
// 0 would inflate every sibling — with weights {A: 0.2, B: null} A would
// normalize to 100% and be reported as starved at 20% observed, when the 20/80
// split is exactly what was configured.
//
// Excluding it also makes the denominator incomplete, and a share computed from
// an incomplete denominator is the same fabrication one step removed: 0.2 out
// of an unknown total is not 100%. So when a weight is unknown the readable
// weights are normalized only if they already account for a whole split (they
// sum to 1 or to 100), which is the one case where the missing weight can only
// be carrying zero traffic. Otherwise every share is reported as unknown and
// the caller is told why.
func rolloutNormalizeWeights(weights map[string]*float64) map[string]*float64 {
	shares := make(map[string]*float64, len(weights))
	total := 0.0
	anyUnknown := false
	for _, w := range weights {
		if w == nil {
			anyUnknown = true
			continue
		}
		if *w > 0 {
			total += *w
		}
	}
	// Tolerances absorb float noise in sums like 0.2+0.8 and 20+30+50.
	whole := math.Abs(total-1) <= 1e-9 || math.Abs(total-100) <= 1e-7
	derivable := !anyUnknown || whole
	for id, w := range weights {
		if w == nil || !derivable {
			shares[id] = nil
			continue
		}
		share := 0.0
		if total > 0 && *w > 0 {
			share = *w / total
		}
		shares[id] = &share
	}
	return shares
}

// rolloutPercentile returns the nearest-rank percentile of a latency sample.
// Nearest-rank (rather than interpolation) keeps p95 a value that an actual run
// produced, so it can be traced back to a run id.
func rolloutPercentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// rolloutRound trims float noise so shares and drift read as numbers a human
// can compare, without rounding away a real difference.
func rolloutRound(v float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(v*scale) / scale
}

// rolloutShareOrZero reads a nullable share for ordering purposes only. An
// unknown share sorts as if it were zero; it is never printed as one.
func rolloutShareOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// rolloutUnattributed labels runs whose record carries no revision id. They are
// surfaced as their own row rather than dropped, because a task whose runs are
// mostly unattributed means the split cannot be verified at all.
const rolloutUnattributed = "(unattributed)"

// rolloutAccum accumulates one revision's observations.
type rolloutAccum struct {
	runs      int
	errors    int
	creditSum float64
	creditN   int
	latencies []float64
}

// buildRolloutReport joins configured weights against observed runs. Split out
// from RunE so it is unit-testable without an API or a database.
//
// Every configured revision is seeded into the output even when it received no
// runs: "configured 20%, observed 0%" is the single most important signal this
// command produces, and it only exists as a row that survives the join.
func buildRolloutReport(configured map[string]*float64, runs []rolloutRun, window string) rolloutReport {
	report := rolloutReport{
		Window:          window,
		TotalRuns:       len(runs),
		ConfiguredFound: len(configured) > 0,
		Revisions:       make([]rolloutRevision, 0, len(configured)+len(runs)),
	}

	shares := rolloutNormalizeWeights(configured)
	// unknownWeights counts revisions the API listed without a usable weight;
	// suppressedShares records that their absence also cost the readable
	// weights their shares. Both are reported rather than left to be inferred
	// from a column of nulls.
	unknownWeights := 0
	suppressedShares := false
	for id, w := range configured {
		if w == nil {
			unknownWeights++
			continue
		}
		if shares[id] == nil {
			suppressedShares = true
		}
	}

	byRev := make(map[string]*rolloutAccum, len(configured)+len(runs))
	for id := range shares {
		byRev[id] = &rolloutAccum{latencies: make([]float64, 0)}
	}
	for _, run := range runs {
		id := strings.TrimSpace(run.RevisionID)
		if id == "" {
			id = rolloutUnattributed
		}
		acc, ok := byRev[id]
		if !ok {
			acc = &rolloutAccum{latencies: make([]float64, 0)}
			byRev[id] = acc
		}
		acc.runs++
		if run.IsError {
			acc.errors++
		}
		if credits, ok := rolloutParseFloat(run.ChargedCredits); ok {
			acc.creditSum += credits
			acc.creditN++
		}
		if run.HasLatency {
			acc.latencies = append(acc.latencies, run.LatencySecs)
		}
	}

	for id, acc := range byRev {
		share, isConfigured := shares[id]
		row := rolloutRevision{
			RevisionID:     id,
			Configured:     isConfigured,
			Runs:           acc.runs,
			CreditSamples:  acc.creditN,
			LatencySamples: len(acc.latencies),
		}
		switch {
		case !isConfigured:
			// Not in active_revisions at all: its configured share really is
			// zero, and the drift from zero is the finding.
			zero := 0.0
			row.ConfiguredShare = &zero
		case share != nil:
			v := rolloutRound(*share, 4)
			row.ConfiguredShare = &v
		}
		if report.TotalRuns > 0 {
			row.ObservedShare = rolloutRound(float64(acc.runs)/float64(report.TotalRuns), 4)
		}
		if acc.runs > 0 {
			row.FailureRate = rolloutRound(float64(acc.errors)/float64(acc.runs), 4)
		}
		if acc.creditN > 0 {
			row.MeanCredits = rolloutRound(acc.creditSum/float64(acc.creditN), 4)
		}
		row.P50LatencySecs = rolloutRound(rolloutPercentile(acc.latencies, 0.50), 4)
		row.P95LatencySecs = rolloutRound(rolloutPercentile(acc.latencies, 0.95), 4)
		// Drift is only defined against a known configured share; with an
		// unknown weight it stays null rather than being fabricated from 0.
		if row.ConfiguredShare != nil {
			drift := rolloutRound((row.ObservedShare-*row.ConfiguredShare)*100, 2)
			row.DriftPct = &drift
		}
		report.Revisions = append(report.Revisions, row)
	}

	// Busiest revision first; ties broken by configured share then id so the
	// ordering is stable across runs (Go map iteration is not).
	sort.SliceStable(report.Revisions, func(i, j int) bool {
		a, b := report.Revisions[i], report.Revisions[j]
		if a.ObservedShare != b.ObservedShare {
			return a.ObservedShare > b.ObservedShare
		}
		if rolloutShareOrZero(a.ConfiguredShare) != rolloutShareOrZero(b.ConfiguredShare) {
			return rolloutShareOrZero(a.ConfiguredShare) > rolloutShareOrZero(b.ConfiguredShare)
		}
		return a.RevisionID < b.RevisionID
	})

	notes := make([]string, 0, 3)
	if unknownWeights > 0 {
		notes = append(notes, fmt.Sprintf(
			"%d configured revision(s) carry a null or unreadable weight; their configured_share and drift_pct are null rather than 0", unknownWeights))
		if suppressedShares {
			notes = append(notes,
				"the readable weights do not add up to a whole split (1 or 100), so no configured share can be derived from them either; only the observed half of this comparison is trustworthy")
		}
	}
	if !report.ConfiguredFound {
		notes = append(notes, "the API returned no active_revisions for this task, so configured_share is 0 for every row; showing observed traffic only")
	}
	if report.TotalRuns == 0 {
		notes = append(notes, fmt.Sprintf(
			"no local runs for this task in the last %s; sync first (rightbrain-pp-cli sync --resources project_task_run) or widen --since", window))
	}
	report.Note = strings.Join(notes, "; ")
	return report
}

// rolloutQueryRuns reads one task's observed runs out of the local mirror.
//
// Every extracted column is scanned through a sql.Null* type because
// json_extract returns NULL for any absent field, and a bare scan errors on
// NULL — which the surrounding loop would turn into a dropped row, reporting a
// revision as having no traffic when it merely lacked one field. The second
// return value counts runs whose created stamp could not be parsed; those runs
// are kept (nothing proves they fall outside the window) and reported, rather
// than discarded behind the caller's back.
func rolloutQueryRuns(ctx context.Context, db *store.Store, taskID string, cutoff time.Time) ([]rolloutRun, int, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT json_extract(data, '$.task_revision_id'),
		       json_extract(data, '$.created'),
		       json_extract(data, '$.is_error'),
		       json_extract(data, '$.charged_credits'),
		       json_extract(data, '$.llm_call_timing')
		FROM resources
		WHERE resource_type = 'project_task_run'
		  AND json_extract(data, '$.task_id') = ?`, taskID)
	if err != nil {
		return nil, 0, fmt.Errorf("reading local task runs: %w", err)
	}
	// Drain the cursor fully into plain structs before the connection is used
	// for anything else: a follow-up query issued mid-iteration can starve the
	// read connection and abort the scan halfway through.
	defer rows.Close()

	runs := make([]rolloutRun, 0)
	undated := 0
	for rows.Next() {
		var (
			revisionID sql.NullString
			created    sql.NullString
			isError    sql.NullBool
			credits    sql.NullString
			timing     sql.NullFloat64
		)
		if err := rows.Scan(&revisionID, &created, &isError, &credits, &timing); err != nil {
			return nil, 0, fmt.Errorf("scanning local task runs: %w", err)
		}
		if ts, ok := parseAPITime(created.String); ok {
			if ts.Before(cutoff) {
				continue
			}
		} else {
			undated++
		}
		runs = append(runs, rolloutRun{
			RevisionID:     revisionID.String,
			IsError:        isError.Valid && isError.Bool,
			ChargedCredits: credits.String,
			LatencySecs:    timing.Float64,
			HasLatency:     timing.Valid,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading local task runs: %w", err)
	}
	return runs, undated, nil
}

// rolloutConfiguredWeights extracts the configured revision weights from a task
// payload. Decoding is deliberately lenient: an absent, null, or empty
// active_revisions array yields an empty map, so the caller reports "no
// configured weights" and still prints the observed half of the comparison.
func rolloutConfiguredWeights(data []byte) (map[string]*float64, error) {
	configured := map[string]*float64{}
	var taskDoc struct {
		ActiveRevisions []rolloutActiveRevision `json:"active_revisions"`
	}
	if err := json.Unmarshal(data, &taskDoc); err != nil {
		return configured, err
	}
	for _, rev := range taskDoc.ActiveRevisions {
		id := strings.TrimSpace(rev.TaskRevisionID)
		if id == "" {
			continue
		}
		// weight is nullable in the spec, and the map entry is what keeps the
		// revision in the report at all — so an unreadable weight is recorded
		// as unknown (nil), never collapsed to 0.
		weight, ok := rolloutParseFloat(rev.Weight)
		if !ok {
			configured[id] = nil
			continue
		}
		w := weight
		configured[id] = &w
	}
	return configured, nil
}

func newNovelRolloutCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "rollout <task_id>",
		Short: "Show what traffic each task revision actually received versus the weight you configured",
		Long: "Compare a task's configured revision weights against the traffic its revisions actually received.\n\n" +
			"Reads the configured active_revisions weights live, normalizes them to a share of traffic, " +
			"and joins them against the runs in the local mirror to report observed share, drift in " +
			"percentage points, failure rate, mean credits, and p50/p95 latency per revision. A revision " +
			"that is configured but received nothing still appears, with zero runs.\n\n" +
			"Observed share counts every run the mirror holds for a revision, including runs generated by " +
			"eval jobs rather than by production traffic. On a task that is evaluated often, read the number " +
			"as 'where did runs go', not 'where did production traffic go'.\n\n" +
			"Exit codes: 0 on success, 2 for a usage error, and 3 when the task's configured revisions could " +
			"not be read and the mirror holds no runs for it — normally an unknown task id.\n\n" +
			"Use this command to see how a task's configured revision weights compare with the traffic and " +
			"latency actually observed. Do NOT use it to decide whether a revision passes its eval set; use " +
			"'gate' instead. Do NOT use it for a project-wide, cross-task regression sweep; use 'drift' instead.",
		Example: "  rightbrain-pp-cli rollout 0195d1ff-1f05-437a-95ac-6de8969cb47b --since 7d --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// 3 is reachable: an unreadable weights fetch with no local runs is
			// an unknown task id, not a healthy empty rollout.
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare configured revision weights against observed runs")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<task_id> is required"))
			}
			taskID := args[0]

			window := strings.TrimSpace(flagSince)
			if window == "" {
				window = "7d"
			}
			since, err := cliutil.ParseDurationLoose(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if since <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: the window must be positive", flagSince))
			}
			cutoff := time.Now().UTC().Add(-since)

			orgID, projectID, err := requireScope(flags)
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("rightbrain-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: rightbrain-pp-cli sync --resources project_task_run --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			// Configured weights come from the live task. A failure here is
			// reported on stderr and carried into the report's note rather than
			// being fatal: the observed half of the comparison is still worth
			// printing, and it is the half that cannot be reconstructed from
			// anywhere else.
			configured := map[string]*float64{}
			var configErrNote string
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The live fetch gets at most half the command's time budget. An
			// unreachable API would otherwise burn the whole --timeout and take
			// the mirror read down with it, turning a degraded answer into no
			// answer at all.
			fetchCtx := ctx
			if deadline, ok := ctx.Deadline(); ok {
				var fetchCancel context.CancelFunc
				fetchCtx, fetchCancel = context.WithTimeout(ctx, time.Until(deadline)/2)
				defer fetchCancel()
			}
			taskPath := fmt.Sprintf("/org/%s/project/%s/task/%s", orgID, projectID, taskID)
			data, err := c.Get(fetchCtx, taskPath, map[string]string{})
			if err != nil {
				configErrNote = fmt.Sprintf("could not read configured weights: %v", err)
			} else if configured, err = rolloutConfiguredWeights(data); err != nil {
				configErrNote = fmt.Sprintf("could not parse the task's active_revisions: %v", err)
			}
			if configErrNote != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", configErrNote)
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			runs, undated, err := rolloutQueryRuns(ctx, db, taskID, cutoff)
			if err != nil {
				return err
			}

			// Degrading a failed weights fetch to a warning is right while there
			// is still observed traffic to report — the mirror half of the
			// answer stands on its own. It is wrong when there is nothing at
			// all: a task whose revisions cannot be read AND which has no local
			// runs is almost always a wrong task id, and exiting 0 with an empty
			// report says "this rollout looks fine" about a task that does not
			// exist.
			if configErrNote != "" && len(runs) == 0 {
				return notFoundErr(fmt.Errorf(
					"task %s: could not read its configured revisions, and the local mirror holds no runs for it — check the task id, or run 'rightbrain-pp-cli sync' if the task is new (underlying error: %s)",
					taskID, configErrNote))
			}

			report := buildRolloutReport(configured, runs, window)
			report.TaskID = taskID
			report.UndatedRuns = undated
			if configErrNote != "" {
				if report.Note == "" {
					report.Note = configErrNote
				} else {
					report.Note = configErrNote + "; " + report.Note
				}
			}

			if !hintIfUnsynced(cmd, db, "project_task_run") {
				hintIfStale(cmd, db, "project_task_run", flags.maxAge)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "task %s  window %s  %d runs\n", taskID, window, report.TotalRuns)
				if report.Note != "" {
					fmt.Fprintln(out, report.Note)
				}
				if report.UndatedRuns > 0 {
					fmt.Fprintf(out, "%d run(s) had no parseable created timestamp and were counted regardless of --since\n", report.UndatedRuns)
				}
				if len(report.Revisions) == 0 {
					return nil
				}
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "REVISION\tCONFIG\tOBSERVED\tDRIFT\tRUNS\tFAIL\tCREDITS\tP50\tP95")
				for _, rev := range report.Revisions {
					// A revision with no usable credit or timing samples prints
					// "-", not 0.00: a zero there would read as free and instant.
					credits := "-"
					if rev.CreditSamples > 0 {
						credits = fmt.Sprintf("%.2f", rev.MeanCredits)
					}
					p50, p95 := "-", "-"
					if rev.LatencySamples > 0 {
						p50 = fmt.Sprintf("%.2fs", rev.P50LatencySecs)
						p95 = fmt.Sprintf("%.2fs", rev.P95LatencySecs)
					}
					fail := "-"
					if rev.Runs > 0 {
						fail = fmt.Sprintf("%.1f%%", rev.FailureRate*100)
					}
					// An unknown configured weight prints "-" in both the
					// CONFIG and DRIFT columns; 0.0%/+80.0pp there would read
					// as a revision serving traffic it was never given.
					configShare, drift := "-", "-"
					if rev.ConfiguredShare != nil {
						configShare = fmt.Sprintf("%.1f%%", *rev.ConfiguredShare*100)
					}
					if rev.DriftPct != nil {
						drift = fmt.Sprintf("%+.1fpp", *rev.DriftPct)
					}
					fmt.Fprintf(tw, "%s\t%s\t%.1f%%\t%s\t%d\t%s\t%s\t%s\t%s\n",
						orDash(rev.RevisionID),
						configShare,
						rev.ObservedShare*100,
						drift,
						rev.Runs,
						fail,
						credits,
						p50,
						p95)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "7d",
		"Only count runs created within this window (e.g. 24h, 7d, 2w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
