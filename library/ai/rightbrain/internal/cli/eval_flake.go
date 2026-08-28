// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

// Eval flake detection.
//
// A single eval run answers "did this candidate revision pass?" and nothing
// more: one red test case looks identical whether it is a real defect or an
// LLM judge that flips a coin. The distinction only exists across repeated
// runs, and the API never computes it — each eval run is returned in isolation,
// keyed by ids (result id, candidate run id) that change every run, so the same
// test case cannot even be followed from one run to the next without joining on
// reference_run_id by hand. This command does that join over the locally synced
// eval runs and separates the three shapes a red case can have: it fails and
// passes against the SAME candidate revision (nondeterminism — flake), it fails
// in every run (a real, reproducible defect), or it started failing at a
// revision boundary (a regression). Because the evidence is inherently
// historical, the command reports insufficient history rather than guessing
// when the mirror holds fewer than two eval runs.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rightbrain/internal/store"
)

// evalResultRecord is one EvalResultResponse: the verdict for a single test
// case within a single eval run.
type evalResultRecord struct {
	ID             string `json:"id"`
	EvalRunID      string `json:"eval_run_id"`
	ReferenceRunID string `json:"reference_run_id"`
	CandidateRunID string `json:"candidate_run_id"`
	Verdict        string `json:"verdict"`
	Reasoning      string `json:"reasoning"`
	IsError        bool   `json:"is_error"`
	ErrorMessage   string `json:"error_message"`
	Created        string `json:"created"`
}

// evalRunRecord is the subset of an eval run this command consumes. Results is
// populated only on detail records; list records carry the counters alone.
type evalRunRecord struct {
	ID                  string             `json:"id"`
	TaskID              string             `json:"task_id"`
	EvalSetID           string             `json:"eval_set_id"`
	CandidateRevisionID string             `json:"candidate_revision_id"`
	Status              string             `json:"status"`
	PassCount           int                `json:"pass_count"`
	FailCount           int                `json:"fail_count"`
	ErrorCount          int                `json:"error_count"`
	PassRate            float64            `json:"pass_rate"`
	Created             string             `json:"created"`
	CompletedAt         string             `json:"completed_at"`
	Results             []evalResultRecord `json:"results"`
}

// evalFlakeCase is one test case's history across the examined eval runs.
//
// NonPasses is the failure side of the ledger: a "fail" verdict plus every
// errored observation. EvalVerdict is nullable in the spec and an errored
// result arrives as verdict null with is_error true, so counting only "fail"
// would let a case that errored in every run report a 0.00 fail rate and sort
// to the bottom beside the genuinely healthy ones.
type evalFlakeCase struct {
	ReferenceRunID    string   `json:"reference_run_id"`
	Runs              int      `json:"runs"`
	Fails             int      `json:"fails"`
	Passes            int      `json:"passes"`
	Errors            int      `json:"errors"`
	NonPasses         int      `json:"non_passes"`
	FailRate          float64  `json:"fail_rate"`
	RevisionsSeen     []string `json:"revisions_seen"`
	Flaky             bool     `json:"flaky"`
	ConsistentFailure bool     `json:"consistent_failure"`
	Errored           bool     `json:"errored"`
	Classification    string   `json:"classification"`
	LastVerdict       string   `json:"last_verdict,omitempty"`
	ErrorMessage      string   `json:"error_message,omitempty"`
}

type evalFlakeReport struct {
	TaskID                 string          `json:"task_id"`
	EvalSetID              string          `json:"eval_set_id,omitempty"`
	EvalRunsExamined       int             `json:"eval_runs_examined"`
	Cases                  []evalFlakeCase `json:"cases"`
	FlakyCount             int             `json:"flaky_count"`
	ConsistentFailureCount int             `json:"consistent_failure_count"`
	ErroredCount           int             `json:"errored_count"`
	TotalCases             int             `json:"total_cases"`
	Note                   string          `json:"note,omitempty"`
}

// evalCaseAccum accumulates one test case's observations while walking the
// eval runs. verdictsByRevision is the whole point: flake is only provable
// when the same revision produced both verdicts.
type evalCaseAccum struct {
	view               evalFlakeCase
	runIDs             map[string]struct{}
	revisions          []string
	revisionSeen       map[string]struct{}
	verdictsByRevision map[string]*evalRevisionVerdicts
}

type evalRevisionVerdicts struct {
	passed bool
	failed bool
}

// evalFlakeRank orders classifications for output: the two categories that
// demand different actions (re-run the judge vs. fix the task) come first, and
// "errored" — a case whose every observation blew up before producing a
// verdict — ranks above "stable-pass" so a permanently broken case can never be
// mistaken for a healthy one at the bottom of the table.
var evalFlakeRank = map[string]int{
	"flaky":              0,
	"consistent-failure": 1,
	"errored":            2,
	"intermittent":       3,
	"stable-pass":        4,
}

// fetchEvalRunsLive lists a task's eval runs from the API.
//
// The local mirror cannot be trusted as the complete set here. Rightbrain's
// eval sets and eval runs are two different collections served by two different
// endpoints (.../eval/set and .../eval/run), but the sync walker maps both onto
// the single resource name "project_task_eval", so they share one table and one
// key space and most records are silently dropped — sync still reports success
// with zero errors. Verified against a live project: three records existed
// (one eval set, two eval runs) and exactly one survived a clean sync.
//
// Anything reasoning over eval history therefore has to treat the API as
// authoritative and keep the mirror as a fallback, which is what
// --data-source local selects.
func fetchEvalRunsLive(ctx context.Context, flags *rootFlags, taskID, evalSetID string) ([]evalRunRecord, error) {
	orgID, projectID, err := requireScope(flags)
	if err != nil {
		return nil, err
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/org/%s/project/%s/task/%s/eval/run", orgID, projectID, taskID)
	out := make([]evalRunRecord, 0)
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]string{"page_limit": "100"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := c.Get(ctx, path, params)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Pagination struct {
				NextCursor string `json:"next_cursor"`
				HasNext    bool   `json:"has_next"`
			} `json:"pagination"`
			Results []evalRunRecord `json:"results"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing eval runs: %w", err)
		}
		for _, r := range envelope.Results {
			if evalSetID != "" && r.EvalSetID != evalSetID {
				continue
			}
			out = append(out, r)
		}
		if !envelope.Pagination.HasNext || envelope.Pagination.NextCursor == "" {
			break
		}
		cursor = envelope.Pagination.NextCursor
	}
	return out, nil
}

// mergeEvalRuns unions live and mirrored eval runs, preferring whichever copy
// already carries per-case results so hydration has less to fetch.
func mergeEvalRuns(primary, secondary []evalRunRecord) []evalRunRecord {
	byID := make(map[string]evalRunRecord, len(primary)+len(secondary))
	order := make([]string, 0, len(primary)+len(secondary))
	add := func(r evalRunRecord) {
		if r.ID == "" {
			return
		}
		existing, seen := byID[r.ID]
		if !seen {
			byID[r.ID] = r
			order = append(order, r.ID)
			return
		}
		if len(existing.Results) == 0 && len(r.Results) > 0 {
			byID[r.ID] = r
		}
	}
	for _, r := range primary {
		add(r)
	}
	for _, r := range secondary {
		add(r)
	}
	merged := make([]evalRunRecord, 0, len(order))
	for _, id := range order {
		merged = append(merged, byID[id])
	}
	return merged
}

// sortEvalRunsNewestFirst orders eval runs the way the report examines them, so
// hydration spends its fetch budget on exactly the runs that will be reported.
func sortEvalRunsNewestFirst(runs []evalRunRecord) []evalRunRecord {
	ordered := make([]evalRunRecord, len(runs))
	copy(ordered, runs)
	sort.SliceStable(ordered, func(i, j int) bool {
		ti, iok := parseAPITime(ordered[i].Created)
		tj, jok := parseAPITime(ordered[j].Created)
		if iok && jok {
			return ti.After(tj)
		}
		if iok != jok {
			return iok
		}
		return ordered[i].Created > ordered[j].Created
	})
	return ordered
}

// hydrateEvalRunResults fills in each examined eval run's per-case results.
//
// This exists because of an asymmetry in the Rightbrain API: the eval-run LIST
// endpoint that `sync` mirrors returns aggregate counts only (pass_count,
// fail_count, pass_rate), while the `results` array — the per-test-case
// verdicts this whole command is built on — exists solely on the per-run DETAIL
// endpoint. Case history is therefore impossible to derive from the mirror
// alone, and without this step the command reports zero cases against a
// perfectly healthy, fully synced mirror.
//
// Only the runs the report will actually examine are fetched. A failed fetch
// degrades that run to its aggregate-only mirror row rather than failing the
// command, and every degraded path returns a note explaining what is missing.
func hydrateEvalRunResults(ctx context.Context, flags *rootFlags, taskID string, runs []evalRunRecord, last int) ([]evalRunRecord, string) {
	ordered := sortEvalRunsNewestFirst(runs)
	budget := last
	if budget <= 0 || budget > len(ordered) {
		budget = len(ordered)
	}

	missing := 0
	for i := 0; i < budget; i++ {
		if len(ordered[i].Results) == 0 {
			missing++
		}
	}
	if missing == 0 {
		return ordered, ""
	}

	if flags.dataSource == "local" {
		return ordered, fmt.Sprintf(
			"%d of the %d examined eval run(s) carry no per-case results: the eval-run list endpoint that sync mirrors omits them, and --data-source local forbids fetching the detail records that hold them. Re-run without --data-source local to hydrate case history.",
			missing, budget)
	}

	orgID, projectID, scopeErr := requireScope(flags)
	if scopeErr != nil {
		return ordered, "per-case results are not in the local mirror and no scope is configured to fetch them; set one with 'rightbrain-pp-cli scope use <org_id> <project_id>'."
	}
	c, clientErr := flags.newClient()
	if clientErr != nil {
		return ordered, fmt.Sprintf("per-case results are not in the local mirror and the API client could not be built: %v", clientErr)
	}

	failed := 0
	for i := 0; i < budget; i++ {
		if len(ordered[i].Results) > 0 || ordered[i].ID == "" {
			continue
		}
		path := fmt.Sprintf("/org/%s/project/%s/task/%s/eval/run/%s", orgID, projectID, taskID, ordered[i].ID)
		data, err := c.Get(ctx, path, map[string]string{})
		if err != nil {
			failed++
			continue
		}
		var detail evalRunRecord
		if err := json.Unmarshal(data, &detail); err != nil {
			failed++
			continue
		}
		if len(detail.Results) > 0 {
			ordered[i].Results = detail.Results
		}
	}
	if failed > 0 {
		return ordered, fmt.Sprintf(
			"%d eval-run detail fetch(es) failed; those runs contribute aggregate counts only and their cases are absent from the ranking.",
			failed)
	}
	return ordered, ""
}

// buildEvalFlakeReport aggregates per-test-case verdict history across eval
// runs. Split out from RunE so the classification is unit-testable with no DB
// and no API.
//
// The flaky rule is deliberately conservative: a case is flaky only when it
// both passed and failed against the SAME candidate_revision_id. Observations
// whose revision is unknown are excluded from that check — an unrecorded
// revision cannot prove two verdicts came from identical inputs, and calling
// that flake would let a genuine regression be dismissed as noise.
func buildEvalFlakeReport(runs []evalRunRecord, last int) evalFlakeReport {
	report := evalFlakeReport{Cases: make([]evalFlakeCase, 0)}

	ordered := make([]evalRunRecord, len(runs))
	copy(ordered, runs)
	// Newest first: --last means the most recent N eval runs.
	sort.SliceStable(ordered, func(i, j int) bool {
		ti, iok := parseAPITime(ordered[i].Created)
		tj, jok := parseAPITime(ordered[j].Created)
		if iok && jok {
			return ti.After(tj)
		}
		if iok != jok {
			return iok
		}
		return ordered[i].Created > ordered[j].Created
	})
	if last > 0 && len(ordered) > last {
		ordered = ordered[:last]
	}
	report.EvalRunsExamined = len(ordered)
	for _, run := range ordered {
		if run.TaskID != "" {
			report.TaskID = run.TaskID
			break
		}
	}

	accums := make(map[string]*evalCaseAccum)
	order := make([]string, 0)

	for runIdx, run := range ordered {
		runKey := run.ID
		if runKey == "" {
			runKey = fmt.Sprintf("run#%d", runIdx)
		}
		for resIdx, res := range run.Results {
			// reference_run_id is the only stable identity a test case has
			// across eval runs; id and candidate_run_id are regenerated every
			// run. Records missing it get a per-result key so they are still
			// reported, but they can never accumulate enough history to be
			// called flaky.
			key := res.ReferenceRunID
			if key == "" {
				key = fmt.Sprintf("unidentified:%s:%s:%d", runKey, res.ID, resIdx)
			}
			acc, ok := accums[key]
			if !ok {
				acc = &evalCaseAccum{
					view:               evalFlakeCase{ReferenceRunID: key, RevisionsSeen: make([]string, 0)},
					runIDs:             map[string]struct{}{},
					revisions:          make([]string, 0),
					revisionSeen:       map[string]struct{}{},
					verdictsByRevision: map[string]*evalRevisionVerdicts{},
				}
				accums[key] = acc
				order = append(order, key)
			}

			acc.runIDs[runKey] = struct{}{}
			if rev := run.CandidateRevisionID; rev != "" {
				if _, seen := acc.revisionSeen[rev]; !seen {
					acc.revisionSeen[rev] = struct{}{}
					acc.revisions = append(acc.revisions, rev)
				}
				if acc.verdictsByRevision[rev] == nil {
					acc.verdictsByRevision[rev] = &evalRevisionVerdicts{}
				}
				// Errored observations are deliberately excluded from the
				// flake check: an eval that blew up produced no verdict at
				// all, so pairing it with a pass proves nothing about the
				// judge's determinism.
				if !res.IsError {
					switch res.Verdict {
					case "pass":
						acc.verdictsByRevision[rev].passed = true
					case "fail":
						acc.verdictsByRevision[rev].failed = true
					}
				}
			}

			// is_error wins over the verdict: the spec makes verdict nullable
			// precisely so an errored result can arrive without one, and an
			// errored observation is never a pass.
			switch {
			case res.IsError:
				acc.view.Errors++
			case res.Verdict == "pass":
				acc.view.Passes++
			case res.Verdict == "fail":
				acc.view.Fails++
			}
			// Runs are newest-first, so the first observation of a case is the
			// most recent one.
			if acc.view.LastVerdict == "" {
				if res.IsError {
					acc.view.LastVerdict = "error"
				} else {
					acc.view.LastVerdict = res.Verdict
				}
			}
			if acc.view.ErrorMessage == "" && res.ErrorMessage != "" {
				acc.view.ErrorMessage = res.ErrorMessage
			}
		}
	}

	for _, key := range order {
		acc := accums[key]
		view := acc.view
		view.Runs = len(acc.runIDs)
		view.RevisionsSeen = acc.revisions
		// An errored observation counts on the failure side of the rate: it is
		// certainly not a pass, and reporting 0.00 for a case that never once
		// produced a passing verdict is the worst possible answer.
		view.NonPasses = view.Fails + view.Errors
		if view.Runs > 0 {
			view.FailRate = float64(view.NonPasses) / float64(view.Runs)
		}
		for _, v := range acc.verdictsByRevision {
			if v.passed && v.failed {
				view.Flaky = true
				break
			}
		}
		// Never passed, and at least one observation carried a real "fail"
		// verdict: a reproducible defect.
		view.ConsistentFailure = view.Runs > 1 && view.NonPasses == view.Runs && view.Fails > 0
		// Never passed and never even produced a verdict: the eval itself is
		// broken, which is a different repair from a failing assertion.
		view.Errored = view.Runs > 0 && view.NonPasses == view.Runs && view.Errors > 0 && view.Fails == 0

		switch {
		case view.Flaky:
			view.Classification = "flaky"
		case view.ConsistentFailure:
			view.Classification = "consistent-failure"
		case view.Errored:
			view.Classification = "errored"
		case view.NonPasses > 0:
			// Fails sometimes, but never both ways on one revision: the verdict
			// changed when the revision changed, which is a regression at a
			// revision boundary, not nondeterminism.
			view.Classification = "intermittent"
		default:
			view.Classification = "stable-pass"
		}

		if view.Flaky {
			report.FlakyCount++
		}
		if view.ConsistentFailure {
			report.ConsistentFailureCount++
		}
		if view.Errored {
			report.ErroredCount++
		}
		report.Cases = append(report.Cases, view)
	}
	report.TotalCases = len(report.Cases)

	sort.SliceStable(report.Cases, func(i, j int) bool {
		ri, rj := evalFlakeRank[report.Cases[i].Classification], evalFlakeRank[report.Cases[j].Classification]
		if ri != rj {
			return ri < rj
		}
		if report.Cases[i].FailRate != report.Cases[j].FailRate {
			return report.Cases[i].FailRate > report.Cases[j].FailRate
		}
		return report.Cases[i].ReferenceRunID < report.Cases[j].ReferenceRunID
	})

	switch {
	case report.EvalRunsExamined == 0:
		report.Note = "no eval runs for this task in the local mirror; run: rightbrain-pp-cli sync --resources project_task_eval"
	case report.EvalRunsExamined < 2:
		report.Note = "insufficient history: only 1 eval run is available, and flake cannot be distinguished from a real failure without repeated runs — a single red verdict is equally consistent with a genuine defect and with a nondeterministic judge. Re-run the eval set against the same candidate revision (or sync more eval runs) and try again; nothing is classified flaky here."
	case report.TotalCases == 0:
		report.Note = "eval runs were found but none carry per-case results; the mirror holds list records only. Sync eval run detail records before ranking test cases."
	}
	return report
}

// loadEvalRunsForTask reads eval runs for one task out of the local mirror.
// Every json_extract can be NULL, so each is scanned into a nullable type: a
// bare scan errors on NULL and the row is then silently dropped, which would
// report "no flake" for a task that has plenty.
// The context is the caller's --timeout-bounded one, not cmd.Context(): the
// mirror read is the only work this command does, so ignoring the bound here
// would make --timeout a no-op for eval-flake.
func loadEvalRunsForTask(ctx context.Context, db *store.Store, taskID, evalSetID string) ([]evalRunRecord, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id,
		       json_extract(data, '$.task_id'),
		       json_extract(data, '$.eval_set_id'),
		       data
		FROM resources
		WHERE resource_type = 'project_task_eval'`)
	if err != nil {
		return nil, fmt.Errorf("querying local eval runs: %w", err)
	}

	type evalRow struct {
		id        string
		taskID    string
		evalSetID string
		data      []byte
	}
	// Drain the whole cursor before doing anything else on this connection:
	// the sync hints the caller emits issue their own queries.
	scanned := make([]evalRow, 0)
	for rows.Next() {
		var (
			id           sql.NullString
			rowTaskID    sql.NullString
			rowEvalSetID sql.NullString
			data         []byte
		)
		if err := rows.Scan(&id, &rowTaskID, &rowEvalSetID, &data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("reading local eval runs: %w", err)
		}
		scanned = append(scanned, evalRow{
			id:        id.String,
			taskID:    rowTaskID.String,
			evalSetID: rowEvalSetID.String,
			data:      data,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("reading local eval runs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local eval run cursor: %w", err)
	}

	out := make([]evalRunRecord, 0, len(scanned))
	for _, row := range scanned {
		if row.taskID != taskID {
			continue
		}
		if evalSetID != "" && row.evalSetID != evalSetID {
			continue
		}
		// Eval sets and eval runs share this resource type; only runs carry an
		// eval_set_id or per-case results.
		if row.evalSetID == "" && !strings.Contains(string(row.data), `"results"`) {
			continue
		}
		var rec evalRunRecord
		if err := json.Unmarshal(row.data, &rec); err != nil {
			continue
		}
		if rec.ID == "" {
			rec.ID = row.id
		}
		if rec.TaskID == "" {
			rec.TaskID = row.taskID
		}
		out = append(out, rec)
	}
	return out, nil
}

func newNovelEvalFlakeCmd(flags *rootFlags) *cobra.Command {
	var flagEvalSet string
	var flagLast int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "eval-flake <task_id>",
		Short: "Rank a task's eval test cases by how often they fail, separating genuine flake from consistent failures",
		Long: "Rank a task's eval test cases by failure history across many eval runs.\n\n" +
			"Joins locally synced eval runs on reference_run_id — the only identity a test " +
			"case keeps from one eval run to the next — and classifies each case as flaky " +
			"(it both passed and failed against the SAME candidate revision, so the judge is " +
			"nondeterministic), consistent-failure (it fails in every run, so it is a real " +
			"defect), errored (every observation errored out before producing a verdict, so " +
			"the eval itself is broken), intermittent (its verdict changed only when the " +
			"candidate revision changed, so it is a regression at a revision boundary), or " +
			"stable-pass.\n\n" +
			"An errored result carries no verdict at all, so it counts on the failure side of " +
			"fail_rate and never as a pass.\n\n" +
			"Flake is only provable from repeated observations. With fewer than two eval runs " +
			"in the mirror the command reports insufficient history instead of guessing.\n\n" +
			"Use this command for per-test-case failure history across many eval runs. " +
			"Do NOT use it for a single pass/fail promotion decision on one candidate revision; use 'gate' instead.",
		Example: "  rightbrain-pp-cli eval-flake 0195d1ff-1f05-437a-95ac-6de8969cb47b --last 10 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
			// A task id this command has never seen is indistinguishable from a
			// real task that simply has no eval history: both are "no eval runs
			// for this task". Inventing a not-found error would require asserting
			// the task does not exist, which this command has no way to know and
			// which would be wrong every time someone runs it against a task
			// before its first eval. Both cases return an empty report and an
			// explanatory note instead, so the error-path probe is skipped.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank eval test cases by failure frequency across recent eval runs")
				return nil
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<task_id> is required"))
			}
			taskID := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("rightbrain-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: rightbrain-pp-cli sync --resources project_task_eval --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local mirror %s: %w", dbPath, err)
			}
			defer db.Close()

			runs, err := loadEvalRunsForTask(ctx, db, taskID, flagEvalSet)
			if err != nil {
				return err
			}

			if !hintIfUnsynced(cmd, db, "project_task_eval") {
				hintIfStale(cmd, db, "project_task_eval", flags.maxAge)
			}

			// The mirror is not authoritative for eval history: sync collapses
			// .../eval/set and .../eval/run onto one resource name and silently
			// drops most records. Unless the caller pinned --data-source local,
			// list the runs live and union them with whatever the mirror has.
			var listNote string
			if flags.dataSource != "local" {
				live, liveErr := fetchEvalRunsLive(ctx, flags, taskID, flagEvalSet)
				if liveErr != nil {
					listNote = fmt.Sprintf("could not list eval runs from the API (%v); falling back to the local mirror, which is known to under-report eval history.", liveErr)
				} else {
					if len(live) > len(runs) {
						listNote = fmt.Sprintf("the local mirror held %d eval run(s) but the API reports %d; using the API as the source of truth.", len(runs), len(live))
					}
					runs = mergeEvalRuns(live, runs)
				}
			}

			// The eval-run LIST endpoint carries only aggregate counts; per-case
			// verdicts live on the DETAIL endpoint alone. Without this hydration
			// every case history is empty and the command reports zero cases.
			hydrated, hydrateNote := hydrateEvalRunResults(ctx, flags, taskID, runs, flagLast)
			if listNote != "" {
				hydrateNote = strings.TrimSpace(listNote + " " + hydrateNote)
			}

			report := buildEvalFlakeReport(hydrated, flagLast)
			if hydrateNote != "" {
				if report.Note == "" {
					report.Note = hydrateNote
				} else {
					report.Note = hydrateNote + " " + report.Note
				}
			}
			report.TaskID = taskID
			report.EvalSetID = flagEvalSet

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "task %s — %d eval run(s) examined, %d case(s): %d flaky, %d consistent failure(s), %d errored\n",
					report.TaskID, report.EvalRunsExamined, report.TotalCases,
					report.FlakyCount, report.ConsistentFailureCount, report.ErroredCount)
				if report.EvalSetID != "" {
					fmt.Fprintf(out, "eval set %s\n", report.EvalSetID)
				}
				if report.Note != "" {
					fmt.Fprintf(out, "\n%s\n", report.Note)
				}
				if len(report.Cases) > 0 {
					fmt.Fprintln(out, "\nCLASS\tFAIL RATE\tRUNS\tFAIL/PASS/ERR\tREVISIONS\tCASE")
					for _, c := range report.Cases {
						fmt.Fprintf(out, "%s\t%.2f\t%d\t%d/%d/%d\t%d\t%s\n",
							c.Classification, c.FailRate, c.Runs,
							c.Fails, c.Passes, c.Errors, len(c.RevisionsSeen),
							orDash(c.ReferenceRunID))
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagEvalSet, "eval-set", "",
		"Only consider eval runs belonging to this eval set ID")
	cmd.Flags().IntVar(&flagLast, "last", 10,
		"How many of the most recent eval runs to aggregate (0 means all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}
