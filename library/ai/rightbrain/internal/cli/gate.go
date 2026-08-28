// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

// Promotion gate for one candidate task revision.
//
// Rightbrain will activate any revision you point it at, and the eval API will
// tell you how many cases a run passed — but nothing in the product answers the
// question a release actually turns on: is this candidate at least as good as
// what shipped last time? An eval run in isolation is a number without a
// reference point; 0.82 is a triumph after 0.60 and a rollback after 0.95. This
// command triggers the eval, waits for it to settle, digs the most recent
// completed run for a *different* revision of the same task out of the local
// mirror to use as the baseline, and turns the comparison into a single
// pass/fail decision with a dedicated exit code so a pipeline can branch on it
// without parsing prose. A first-ever gate has nothing to regress against and
// is never failed for that reason alone.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// gateEvalRun is the subset of EvalRunResponse this command consumes. PassRate
// is a pointer because the API returns null until the run has scored anything —
// a zero float would read as "everything failed".
type gateEvalRun struct {
	ID                  string   `json:"id"`
	TaskID              string   `json:"task_id"`
	CandidateRevisionID string   `json:"candidate_revision_id"`
	EvalSetID           string   `json:"eval_set_id"`
	Status              string   `json:"status"`
	PassCount           int      `json:"pass_count"`
	FailCount           int      `json:"fail_count"`
	ErrorCount          int      `json:"error_count"`
	PassRate            *float64 `json:"pass_rate"`
	StartedAt           string   `json:"started_at"`
	CompletedAt         string   `json:"completed_at"`
	ErrorMessage        string   `json:"error_message"`
}

// gateBaseline is the prior-art side of the comparison: the most recent
// completed eval run recorded locally for the same task but a different
// revision. Found is false when the mirror holds nothing comparable.
type gateBaseline struct {
	Found       bool
	EvalRunID   string
	RevisionID  string
	PassRate    float64
	HasPassRate bool
	CompletedAt string
}

// gateReport is the decision. Nullable numbers are pointers so a missing
// baseline marshals as null instead of a fabricated 0.0.
type gateReport struct {
	TaskID              string   `json:"task_id"`
	EvalRunID           string   `json:"eval_run_id"`
	CandidateRevisionID string   `json:"candidate_revision_id"`
	EvalSetID           string   `json:"eval_set_id,omitempty"`
	Status              string   `json:"status"`
	Passed              bool     `json:"passed"`
	PassRate            *float64 `json:"pass_rate"`
	BaselinePassRate    *float64 `json:"baseline_pass_rate"`
	Delta               *float64 `json:"delta"`
	PassCount           int      `json:"pass_count"`
	FailCount           int      `json:"fail_count"`
	ErrorCount          int      `json:"error_count"`
	MinPassRate         *float64 `json:"min_pass_rate"`
	BaselineFound       bool     `json:"baseline_found"`
	BaselineRevisionID  string   `json:"baseline_revision_id"`
	BaselineEvalRunID   string   `json:"baseline_eval_run_id"`
	Reasons             []string `json:"reasons"`
	ErrorMessage        string   `json:"error_message,omitempty"`
	Note                string   `json:"note,omitempty"`
}

// gateRunTerminal reports whether an eval run has stopped moving.
func gateRunTerminal(status string) bool {
	return status == "completed" || status == "failed"
}

// evaluateGate turns a finished (or stalled) candidate eval run plus whatever
// baseline could be found into the promote/hold decision. Split out from RunE
// so the whole decision surface is unit-testable without a live API or a
// database: every input is a plain value.
func evaluateGate(run gateEvalRun, baseline gateBaseline, minPassRate float64, minPassRateSet bool) gateReport {
	report := gateReport{
		TaskID:              run.TaskID,
		EvalRunID:           run.ID,
		CandidateRevisionID: run.CandidateRevisionID,
		EvalSetID:           run.EvalSetID,
		Status:              run.Status,
		PassCount:           run.PassCount,
		FailCount:           run.FailCount,
		ErrorCount:          run.ErrorCount,
		BaselineFound:       baseline.Found,
		ErrorMessage:        run.ErrorMessage,
		Reasons:             make([]string, 0),
	}
	if run.PassRate != nil {
		rate := *run.PassRate
		report.PassRate = &rate
	}
	if minPassRateSet {
		floor := minPassRate
		report.MinPassRate = &floor
	}
	if baseline.Found {
		report.BaselineRevisionID = baseline.RevisionID
		report.BaselineEvalRunID = baseline.EvalRunID
		if baseline.HasPassRate {
			rate := baseline.PassRate
			report.BaselinePassRate = &rate
			// A delta is only meaningful when both sides are real numbers;
			// inventing one from a missing baseline would read as "no change".
			if report.PassRate != nil {
				delta := *report.PassRate - baseline.PassRate
				report.Delta = &delta
			}
		}
	}

	switch {
	case run.Status == "failed":
		msg := run.ErrorMessage
		if msg == "" {
			msg = "no error message was returned"
		}
		report.Reasons = append(report.Reasons,
			fmt.Sprintf("the eval run failed before scoring finished: %s", msg))
	case !gateRunTerminal(run.Status):
		report.Reasons = append(report.Reasons, fmt.Sprintf(
			"the eval run is still %s, so no verdict exists yet; re-run with --wait (and a larger --timeout-secs)",
			orDash(run.Status)))
	}

	if report.PassRate == nil {
		if gateRunTerminal(run.Status) {
			report.Reasons = append(report.Reasons,
				"the API returned no pass_rate for this eval run, so the candidate cannot be shown to be safe")
		}
	} else {
		if minPassRateSet && *report.PassRate < minPassRate {
			report.Reasons = append(report.Reasons, fmt.Sprintf(
				"pass rate %.4f is below the required --min-pass-rate %.4f", *report.PassRate, minPassRate))
		}
		if baseline.Found && baseline.HasPassRate && *report.PassRate < baseline.PassRate {
			report.Reasons = append(report.Reasons, fmt.Sprintf(
				"pass rate %.4f regressed against baseline %.4f (revision %s, eval run %s)",
				*report.PassRate, baseline.PassRate, orDash(baseline.RevisionID), orDash(baseline.EvalRunID)))
		}
	}

	report.Passed = len(report.Reasons) == 0
	return report
}

// gateExitStatus maps the decision onto the exit-code contract: 0 to promote,
// 3 to hold. Code 3 is its own typed error so CI can branch on the exit status
// instead of grepping output.
func gateExitStatus(report gateReport) error {
	if report.Passed {
		return nil
	}
	reason := strings.Join(report.Reasons, "; ")
	if reason == "" {
		reason = "the candidate revision did not clear the gate"
	}
	return &cliError{code: 3, err: fmt.Errorf("gate failed for revision %s: %s",
		orDash(report.CandidateRevisionID), reason)}
}

// pollGateEvalRun polls fetch until the run reaches a terminal status, the
// deadline passes, or maxPolls is exhausted. maxPolls of 0 means unbounded; the
// dogfood matrix passes 1 so the flat 30s per-command budget is respected. The
// bool reports whether a terminal status was actually observed, so the caller
// can say "timed out" rather than silently reporting a still-running eval as a
// verdict.
func pollGateEvalRun(ctx context.Context, run gateEvalRun, fetch func(context.Context) (gateEvalRun, error),
	deadline time.Time, interval time.Duration, maxPolls int) (gateEvalRun, bool, error) {
	if gateRunTerminal(run.Status) {
		return run, true, nil
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for polls := 0; maxPolls <= 0 || polls < maxPolls; polls++ {
		wait := interval
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return run, false, nil
		}
		if remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return run, false, ctx.Err()
		case <-timer.C:
		}
		next, err := fetch(ctx)
		if err != nil {
			return run, false, err
		}
		run = next
		if gateRunTerminal(run.Status) {
			return run, true, nil
		}
	}
	return run, false, nil
}

// gateStoredRun is one eval-run row drained out of the local mirror. Every
// field comes from json_extract, which yields NULL for an absent key, so each
// is scanned through a Null* type before it reaches this struct.
type gateStoredRun struct {
	id          string
	revisionID  string
	status      string
	passRate    float64
	hasPassRate bool
	completedAt string
	created     string
}

// recency is the timestamp the baseline is ordered by: when the run finished,
// falling back to when it was created for rows the mirror caught mid-flight.
func (r gateStoredRun) recency() string {
	if r.completedAt != "" {
		return r.completedAt
	}
	return r.created
}

// loadGateBaseline finds the most recent completed eval run for the same task
// and a different candidate revision. A missing database, an unsynced mirror,
// and a task with only this one revision are all normal states rather than
// errors: the returned note explains what happened and the gate proceeds
// without a regression check.
func loadGateBaseline(ctx context.Context, cmd *cobra.Command, flags *rootFlags,
	dbPath, taskID, candidateRevisionID, excludeEvalRunID string) (gateBaseline, string) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return gateBaseline{}, fmt.Sprintf(
			"no local mirror at %s, so no baseline was available; run 'rightbrain-pp-cli sync --resources project_task_eval' to enable regression checks", dbPath)
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return gateBaseline{}, fmt.Sprintf("could not open the local mirror (%v); no baseline comparison was made", err)
	}
	defer db.Close()

	var maxAge time.Duration
	if flags != nil {
		maxAge = flags.maxAge
	}
	if !hintIfUnsynced(cmd, db, "project_task_eval") {
		hintIfStale(cmd, db, "project_task_eval", maxAge)
	}

	rows, err := db.DB().QueryContext(ctx, `
		SELECT id,
		       json_extract(data, '$.candidate_revision_id'),
		       json_extract(data, '$.status'),
		       json_extract(data, '$.pass_rate'),
		       json_extract(data, '$.completed_at'),
		       json_extract(data, '$.created')
		FROM resources
		WHERE resource_type = 'project_task_eval'
		  AND json_extract(data, '$.task_id') = ?
		  AND json_extract(data, '$.candidate_revision_id') IS NOT NULL`, taskID)
	if err != nil {
		return gateBaseline{}, fmt.Sprintf("could not read eval runs from the local mirror (%v); no baseline comparison was made", err)
	}

	// Drain fully before doing anything else: the store runs on a single
	// SQLite connection, so an open *sql.Rows blocks every follow-up query.
	stored := make([]gateStoredRun, 0)
	for rows.Next() {
		var (
			id          sql.NullString
			revisionID  sql.NullString
			status      sql.NullString
			passRate    sql.NullFloat64
			completedAt sql.NullString
			created     sql.NullString
		)
		if err := rows.Scan(&id, &revisionID, &status, &passRate, &completedAt, &created); err != nil {
			_ = rows.Close()
			return gateBaseline{}, fmt.Sprintf("could not read an eval run row from the local mirror (%v); no baseline comparison was made", err)
		}
		stored = append(stored, gateStoredRun{
			id:          id.String,
			revisionID:  revisionID.String,
			status:      status.String,
			passRate:    passRate.Float64,
			hasPassRate: passRate.Valid,
			completedAt: completedAt.String,
			created:     created.String,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return gateBaseline{}, fmt.Sprintf("could not read eval runs from the local mirror (%v); no baseline comparison was made", err)
	}
	if err := rows.Close(); err != nil {
		return gateBaseline{}, fmt.Sprintf("could not finish reading eval runs from the local mirror (%v); no baseline comparison was made", err)
	}

	best, ok := pickGateBaseline(stored, candidateRevisionID, excludeEvalRunID)
	if !ok {
		if len(stored) == 0 {
			return gateBaseline{}, "the local mirror holds no eval runs for this task, so this gate has no baseline to compare against; run 'rightbrain-pp-cli sync --resources project_task_eval' after your next eval"
		}
		return gateBaseline{}, "the local mirror holds no completed eval run for a different revision of this task, so there is nothing to regress against"
	}
	return gateBaseline{
		Found:       true,
		EvalRunID:   best.id,
		RevisionID:  best.revisionID,
		PassRate:    best.passRate,
		HasPassRate: best.hasPassRate,
		CompletedAt: best.completedAt,
	}, ""
}

// pickGateBaseline selects the most recent completed run belonging to a
// different revision. Kept separate from the SQL so the selection rule is
// testable on plain values.
func pickGateBaseline(stored []gateStoredRun, candidateRevisionID, excludeEvalRunID string) (gateStoredRun, bool) {
	var best gateStoredRun
	found := false
	for _, r := range stored {
		if r.status != "completed" || !r.hasPassRate {
			continue
		}
		if r.revisionID == "" || r.revisionID == candidateRevisionID {
			continue
		}
		if excludeEvalRunID != "" && r.id == excludeEvalRunID {
			continue
		}
		if !found || apiTimeAfter(r.recency(), best.recency()) {
			best, found = r, true
		}
	}
	return best, found
}

// resolveGateRevisionTag maps a revision tag onto a revision id by reading the
// task's revision list, so a caller can gate "the revision tagged candidate"
// without first looking up its UUID.
func resolveGateRevisionTag(ctx context.Context, c interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}, orgID, projectID, taskID, tag string) (string, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/org/%s/project/%s/task/%s", orgID, projectID, taskID), map[string]string{})
	if err != nil {
		return "", err
	}
	var task struct {
		Revisions []struct {
			ID      string `json:"id"`
			Created string `json:"created"`
			Tags    []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(data, &task); err != nil {
		return "", fmt.Errorf("parsing task revisions: %w", err)
	}
	best, bestCreated := "", ""
	for _, rev := range task.Revisions {
		for _, t := range rev.Tags {
			if !strings.EqualFold(t.Name, tag) {
				continue
			}
			if best == "" || apiTimeAfter(rev.Created, bestCreated) {
				best, bestCreated = rev.ID, rev.Created
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no revision of task %s carries the tag %q", taskID, tag)
	}
	return best, nil
}

func newNovelGateCmd(flags *rootFlags) *cobra.Command {
	var flagRevisionID string
	var flagRevisionTag string
	var flagEvalSet string
	var flagMinPassRate float64
	var flagWait bool
	var flagTimeoutSecs int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "gate <task_id>",
		Short: "Decide whether a candidate task revision is safe to promote",
		Long: "Decide whether a candidate task revision is safe to promote.\n\n" +
			"Triggers an eval run for the candidate revision, optionally waits for it to finish, " +
			"then compares its pass rate against both an absolute floor (--min-pass-rate) and the " +
			"most recent completed eval run recorded locally for a different revision of the same " +
			"task. The first gate for a task has no baseline and is never failed for that reason " +
			"alone; the report says so with baseline_found: false.\n\n" +
			"Exit codes: 0 when the candidate cleared the gate, 2 for a usage error, and 3 when the " +
			"candidate did NOT clear the gate — branch on exit 3 in CI to block a promotion.\n\n" +
			"Use this command to decide whether one candidate revision is safe to promote. " +
			"Do NOT use this command to inspect live traffic split or observed latency of " +
			"already-promoted revisions; use 'rollout' instead. Do NOT use it for per-test-case " +
			"failure history across many eval runs; use 'eval-flake' instead.",
		Example: "  rightbrain-pp-cli gate 0195d1ff-1f05-437a-95ac-6de8969cb47b --min-pass-rate 0.9 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run the eval set against the candidate revision, compare its pass rate to the last completed eval for a different revision, and exit 3 if it regressed")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<task_id> is required"))
			}
			taskID := args[0]
			if flagRevisionID != "" && flagRevisionTag != "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--revision-id and --revision-tag select the same thing; pass only one"))
			}
			// A pass rate is a fraction, but people think in percentages, and
			// --min-pass-rate 90 is silently unsatisfiable: every candidate
			// "regresses" against a floor of 9000%, so the gate exits 3 forever
			// and CI blocks on a typo that reads as a reasonable threshold.
			if cmd.Flags().Changed("min-pass-rate") && (flagMinPassRate < 0 || flagMinPassRate > 1) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf(
					"--min-pass-rate takes a fraction between 0 and 1, got %g (90%% is 0.9, not 90)", flagMinPassRate))
			}

			orgID, projectID, err := requireScope(flags)
			if err != nil {
				return err
			}

			// boundCtx applies the root --timeout (60s by default), which is the
			// right bound for a single request but wrong for --wait: an eval run
			// legitimately takes minutes, so any --timeout-secs larger than the
			// root timeout is unreachable and --wait dies with "context deadline
			// exceeded" instead of polling. When waiting, give the command at
			// least the wait budget plus a margin for the calls either side of
			// the poll loop.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if flagWait {
				if flagTimeoutSecs <= 0 {
					flagTimeoutSecs = 300
				}
				waitBudget := time.Duration(flagTimeoutSecs)*time.Second + 30*time.Second
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < waitBudget {
					cancel()
					ctx, cancel = context.WithTimeout(cmd.Context(), waitBudget)
					defer cancel()
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			revisionID := flagRevisionID
			if flagRevisionTag != "" {
				revisionID, err = resolveGateRevisionTag(ctx, c, orgID, projectID, taskID, flagRevisionTag)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}

			body := map[string]any{}
			if revisionID != "" {
				body["candidate_revision_id"] = revisionID
			}
			if flagEvalSet != "" {
				body["eval_set_id"] = flagEvalSet
			}

			runPath := fmt.Sprintf("/org/%s/project/%s/task/%s/eval/run", orgID, projectID, taskID)
			data, _, err := c.Post(ctx, runPath, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var run gateEvalRun
			if err := json.Unmarshal(data, &run); err != nil {
				return fmt.Errorf("parsing eval run response: %w", err)
			}
			if run.TaskID == "" {
				run.TaskID = taskID
			}
			if run.CandidateRevisionID == "" {
				run.CandidateRevisionID = revisionID
			}

			timedOut := false
			if flagWait && !gateRunTerminal(run.Status) {
				if flagTimeoutSecs <= 0 {
					flagTimeoutSecs = 300
				}
				maxPolls := 0
				if cliutil.IsDogfoodEnv() {
					// The live-dogfood matrix enforces a flat 30s per command;
					// a single poll keeps --wait honest inside that budget.
					maxPolls = 1
				}
				pollPath := fmt.Sprintf("%s/%s", runPath, run.ID)
				fetch := func(ctx context.Context) (gateEvalRun, error) {
					// GetNoCache, not Get: polling asks the same URL repeatedly
					// and the whole point is to observe a status change. A
					// cached first response pins the run at "pending" forever,
					// so --wait spins to its deadline and reports "still
					// running" for a run the server finished seconds in.
					raw, fetchErr := c.GetNoCache(ctx, pollPath, map[string]string{})
					if fetchErr != nil {
						return gateEvalRun{}, fetchErr
					}
					var latest gateEvalRun
					if decodeErr := json.Unmarshal(raw, &latest); decodeErr != nil {
						return gateEvalRun{}, fmt.Errorf("parsing eval run %s: %w", run.ID, decodeErr)
					}
					return latest, nil
				}
				deadline := time.Now().Add(time.Duration(flagTimeoutSecs) * time.Second)
				latest, terminal, pollErr := pollGateEvalRun(ctx, run, fetch, deadline, 5*time.Second, maxPolls)
				if pollErr != nil {
					return classifyAPIError(pollErr, flags)
				}
				run = latest
				timedOut = !terminal
			}

			if dbPath == "" {
				dbPath = defaultDBPath("rightbrain-pp-cli")
			}
			baseline, baselineNote := loadGateBaseline(ctx, cmd, flags, dbPath,
				run.TaskID, run.CandidateRevisionID, run.ID)

			report := evaluateGate(run, baseline, flagMinPassRate, cmd.Flags().Changed("min-pass-rate"))
			report.Note = baselineNote
			if timedOut {
				report.Reasons = append(report.Reasons, fmt.Sprintf(
					"the eval run had not finished after --timeout-secs %d; it is still %s",
					flagTimeoutSecs, orDash(run.Status)))
				report.Passed = false
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				verdict := "HOLD"
				if report.Passed {
					verdict = "PROMOTE"
				}
				fmt.Fprintf(out, "%s  revision %s  eval run %s  (%s)\n",
					verdict, orDash(report.CandidateRevisionID), orDash(report.EvalRunID), orDash(report.Status))
				fmt.Fprintf(out, "pass rate   %s  (%d passed, %d failed, %d errors)\n",
					gateFormatRate(report.PassRate), report.PassCount, report.FailCount, report.ErrorCount)
				if report.BaselineFound {
					fmt.Fprintf(out, "baseline    %s  (revision %s, eval run %s)\n",
						gateFormatRate(report.BaselinePassRate), orDash(report.BaselineRevisionID), orDash(report.BaselineEvalRunID))
					fmt.Fprintf(out, "delta       %s\n", gateFormatDelta(report.Delta))
				} else {
					fmt.Fprintln(out, "baseline    none — nothing comparable is recorded locally, so no regression check was possible")
				}
				if report.MinPassRate != nil {
					fmt.Fprintf(out, "floor       %.4f (--min-pass-rate)\n", *report.MinPassRate)
				}
				if len(report.Reasons) > 0 {
					fmt.Fprintln(out, "reasons:")
					for _, r := range report.Reasons {
						fmt.Fprintf(out, "  - %s\n", r)
					}
				}
				if report.Note != "" {
					fmt.Fprintf(out, "note: %s\n", report.Note)
				}
				return gateExitStatus(report)
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
			return gateExitStatus(report)
		},
	}

	cmd.Flags().StringVar(&flagRevisionID, "revision-id", "",
		"Candidate task revision ID to gate (defaults to whatever the eval API selects)")
	cmd.Flags().StringVar(&flagRevisionTag, "revision-tag", "",
		"Candidate revision selected by tag name instead of ID (e.g. candidate)")
	cmd.Flags().StringVar(&flagEvalSet, "eval-set", "",
		"Eval set ID to run the candidate against")
	cmd.Flags().Float64Var(&flagMinPassRate, "min-pass-rate", 0,
		"Absolute pass-rate floor between 0 and 1; the gate fails below it (unset means baseline comparison only)")
	cmd.Flags().BoolVar(&flagWait, "wait", false,
		"Poll until the eval run completes or fails instead of reporting it as still running")
	cmd.Flags().IntVar(&flagTimeoutSecs, "timeout-secs", 300,
		"How long --wait polls before giving up")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

// gateFormatRate renders a nullable rate without inventing a 0.0 for null.
func gateFormatRate(rate *float64) string {
	if rate == nil {
		return "-"
	}
	return fmt.Sprintf("%.4f", *rate)
}

// gateFormatDelta signs the delta so the direction of the change is readable
// at a glance.
func gateFormatDelta(delta *float64) string {
	if delta == nil {
		return "-"
	}
	return fmt.Sprintf("%+.4f", *delta)
}
