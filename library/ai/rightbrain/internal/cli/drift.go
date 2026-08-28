// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

// Week-over-week drift.
//
// "Did anything get slower or more expensive this week?" is the question every
// Rightbrain operator asks on Monday, and the API cannot answer it: the usage,
// timing and credit reports are per-task, per-window snapshots with no
// baseline, so answering it live means fetching one report per task per window
// and diffing them by hand. This command does that diff across the whole
// project at once, entirely against the local mirror — it slices the synced
// runs into the current window and the immediately preceding window of equal
// length, aggregates mean credits, tokens, p95 latency and failure rate for
// both, and ranks every task, agent, model or revision by how far it moved.
// Groups too thin to be trustworthy are dropped by --min-runs and counted in
// filtered_out so the cap is never silent, and a group with no history in the
// previous window is reported as new rather than as an infinite regression.

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

// driftGroupByValues are the accepted --group-by dimensions, in the order they
// are listed back to the user when an unknown value is rejected.
var driftGroupByValues = []string{"task", "agent", "model", "revision"}

// driftRun is one locally mirrored run, reduced to the fields the windowed
// comparison consumes. Credits and latency carry an explicit "present" flag:
// a run whose charged_credits is missing or unparseable must be excluded from
// the mean rather than dragged toward zero by counting as 0.00.
type driftRun struct {
	TaskID         string
	TaskName       string
	TaskRevisionID string
	ModelName      string
	TaskAgentID    string
	AgentName      string
	Created        time.Time
	IsError        bool
	TotalTokens    int64
	Credits        float64
	HasCredits     bool
	LatencySecs    float64
	HasLatency     bool
}

// driftRawRun mirrors the run's `data` JSON column. Every numeric field is
// decoded as json.RawMessage because the API renders them inconsistently —
// charged_credits arrives as the string "9.00", total_tokens as a bare number —
// and a typed field would fail the whole row's decode on the other shape.
type driftRawRun struct {
	TaskID         string          `json:"task_id"`
	TaskName       string          `json:"task_name"`
	TaskRevisionID string          `json:"task_revision_id"`
	ModelName      string          `json:"model_name"`
	TaskAgentID    string          `json:"task_agent_id"`
	AgentName      string          `json:"task_agent_name"`
	Created        string          `json:"created"`
	IsError        json.RawMessage `json:"is_error"`
	TotalTokens    json.RawMessage `json:"total_tokens"`
	ChargedCredits json.RawMessage `json:"charged_credits"`
	LLMCallTiming  json.RawMessage `json:"llm_call_timing"`
}

// driftMover is one group's before/after comparison. The four delta fields are
// pointers so an undefined ratio (no previous-window baseline) serializes as
// JSON null instead of +Inf or NaN, which no consumer can sort or threshold.
type driftMover struct {
	Key                 string   `json:"key"`
	Name                string   `json:"name"`
	RunsCurrent         int      `json:"runs_current"`
	RunsPrevious        int      `json:"runs_previous"`
	MeanCreditsCurrent  float64  `json:"mean_credits_current"`
	MeanCreditsPrevious float64  `json:"mean_credits_previous"`
	TotalTokensCurrent  int64    `json:"total_tokens_current"`
	TotalTokensPrev     int64    `json:"total_tokens_previous"`
	P95Current          float64  `json:"p95_current"`
	P95Previous         float64  `json:"p95_previous"`
	FailureRateCurrent  float64  `json:"failure_rate_current"`
	FailureRatePrevious float64  `json:"failure_rate_previous"`
	CreditsDeltaPct     *float64 `json:"credits_delta_pct"`
	P95DeltaPct         *float64 `json:"p95_delta_pct"`
	FailureRateDeltaPct *float64 `json:"failure_rate_delta_pct"`
	RunsDeltaPct        *float64 `json:"runs_delta_pct"`
	New                 bool     `json:"new"`
}

// driftReport is the full JSON contract. Both window boundaries are echoed so
// a caller can tell exactly what "this week" meant without re-deriving it.
type driftReport struct {
	Window              string       `json:"window"`
	GroupBy             string       `json:"group_by"`
	CurrentWindowStart  string       `json:"current_window_start"`
	CurrentWindowEnd    string       `json:"current_window_end"`
	PreviousWindowStart string       `json:"previous_window_start"`
	PreviousWindowEnd   string       `json:"previous_window_end"`
	TotalRunsCurrent    int          `json:"total_runs_current"`
	TotalRunsPrevious   int          `json:"total_runs_previous"`
	UnattributedRuns    int          `json:"unattributed_runs"`
	MinRuns             int          `json:"min_runs"`
	FilteredOut         int          `json:"filtered_out"`
	Movers              []driftMover `json:"movers"`
	Note                string       `json:"note,omitempty"`
}

// driftWindowStats accumulates one group's runs within one window.
type driftWindowStats struct {
	runs        int
	errors      int
	creditSum   float64
	creditCount int
	totalTokens int64
	latencies   []float64
}

func (s *driftWindowStats) add(r driftRun) {
	s.runs++
	if r.IsError {
		s.errors++
	}
	s.totalTokens += r.TotalTokens
	// A missing or unparseable charged_credits is excluded from the mean
	// entirely; folding it in as 0.00 would make a partially-populated group
	// look like a cost win that never happened.
	if r.HasCredits {
		s.creditSum += r.Credits
		s.creditCount++
	}
	if r.HasLatency {
		s.latencies = append(s.latencies, r.LatencySecs)
	}
}

func (s *driftWindowStats) meanCredits() float64 {
	if s.creditCount == 0 {
		return 0
	}
	return driftRound(s.creditSum / float64(s.creditCount))
}

func (s *driftWindowStats) failureRate() float64 {
	if s.runs == 0 {
		return 0
	}
	return driftRound(float64(s.errors) / float64(s.runs))
}

func (s *driftWindowStats) p95() float64 {
	return driftRound(driftP95(s.latencies))
}

// driftP95 is the nearest-rank 95th percentile: sort ascending, take the value
// at ceil(0.95*n). Nearest-rank never interpolates, so the reported p95 is
// always a latency some run actually had.
func driftP95(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	rank := int(math.Ceil(0.95 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// driftRound trims float noise so a delta of 0 reads as 0 and not 1e-17.
func driftRound(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*10000) / 10000
}

// driftPctChange returns the percentage change from prev to cur.
//
// Two cases are deliberately not arithmetic. Identical values return an
// explicit 0 even when both are zero, because "nothing changed" is a real
// answer. A zero baseline against a non-zero current returns nil: the ratio is
// undefined, and emitting +Inf would rank a brand-new group above every genuine
// regression in the project.
func driftPctChange(prev, cur float64) *float64 {
	if prev == cur {
		zero := 0.0
		return &zero
	}
	if prev == 0 {
		return nil
	}
	v := driftRound((cur - prev) / prev * 100)
	return &v
}

// driftGroupKeyFor projects a run onto the requested dimension, returning the
// grouping key and the display name to show beside it.
func driftGroupKeyFor(r driftRun, groupBy string) (string, string) {
	switch groupBy {
	case "agent":
		return r.TaskAgentID, r.AgentName
	case "model":
		return r.ModelName, r.ModelName
	case "revision":
		return r.TaskRevisionID, r.TaskName
	default:
		return r.TaskID, r.TaskName
	}
}

// driftGroup holds both windows for one key while the report is assembled.
type driftGroup struct {
	key      string
	name     string
	current  driftWindowStats
	previous driftWindowStats
}

// buildDriftReport slices rows into the current and previous windows and ranks
// every qualifying group by how far it moved. Split out from RunE and taking
// only plain values so the comparison math is unit-testable without a database.
func buildDriftReport(rows []driftRun, now time.Time, window time.Duration, groupBy string, minRuns int) driftReport {
	if groupBy == "" {
		groupBy = "task"
	}
	if minRuns < 0 {
		minRuns = 0
	}
	now = now.UTC()
	curStart := now.Add(-window)
	prevStart := now.Add(-2 * window)

	report := driftReport{
		Window:              driftFormatWindow(window),
		GroupBy:             groupBy,
		CurrentWindowStart:  curStart.Format(time.RFC3339),
		CurrentWindowEnd:    now.Format(time.RFC3339),
		PreviousWindowStart: prevStart.Format(time.RFC3339),
		PreviousWindowEnd:   curStart.Format(time.RFC3339),
		MinRuns:             minRuns,
		Movers:              make([]driftMover, 0),
	}

	groups := map[string]*driftGroup{}
	order := make([]string, 0)

	for _, r := range rows {
		if r.Created.IsZero() {
			continue
		}
		ts := r.Created.UTC()
		inCurrent := !ts.Before(curStart) && !ts.After(now)
		inPrevious := !ts.Before(prevStart) && ts.Before(curStart)
		if !inCurrent && !inPrevious {
			continue
		}

		key, name := driftGroupKeyFor(r, groupBy)
		if key == "" {
			// A run carrying no value for the requested dimension (a
			// task run with no agent, under --group-by agent) belongs to no
			// group, so no mover can ever account for it. Counting it in the
			// window totals would put 5,040 in the header above a movers table
			// that sums to 40, with filtered_out=0 explaining nothing — so it
			// is counted separately and reported under its own name.
			report.UnattributedRuns++
			continue
		}
		if inCurrent {
			report.TotalRunsCurrent++
		} else {
			report.TotalRunsPrevious++
		}
		g, ok := groups[key]
		if !ok {
			g = &driftGroup{key: key, name: name}
			groups[key] = g
			order = append(order, key)
		}
		if g.name == "" {
			g.name = name
		}
		if inCurrent {
			g.current.add(r)
		} else {
			g.previous.add(r)
		}
	}

	for _, key := range order {
		g := groups[key]
		if g.current.runs < minRuns {
			report.FilteredOut++
			continue
		}
		mover := driftMover{
			Key:                 g.key,
			Name:                g.name,
			RunsCurrent:         g.current.runs,
			RunsPrevious:        g.previous.runs,
			MeanCreditsCurrent:  g.current.meanCredits(),
			MeanCreditsPrevious: g.previous.meanCredits(),
			TotalTokensCurrent:  g.current.totalTokens,
			TotalTokensPrev:     g.previous.totalTokens,
			P95Current:          g.current.p95(),
			P95Previous:         g.previous.p95(),
			FailureRateCurrent:  g.current.failureRate(),
			FailureRatePrevious: g.previous.failureRate(),
			New:                 g.previous.runs == 0,
		}
		mover.CreditsDeltaPct = driftPctChange(mover.MeanCreditsPrevious, mover.MeanCreditsCurrent)
		mover.P95DeltaPct = driftPctChange(mover.P95Previous, mover.P95Current)
		mover.FailureRateDeltaPct = driftPctChange(mover.FailureRatePrevious, mover.FailureRateCurrent)
		mover.RunsDeltaPct = driftPctChange(float64(mover.RunsPrevious), float64(mover.RunsCurrent))
		report.Movers = append(report.Movers, mover)
	}

	// Biggest mover first: the whole point of the report is that the reader
	// stops after the first few rows.
	sort.SliceStable(report.Movers, func(i, j int) bool {
		mi, mj := driftMagnitude(report.Movers[i]), driftMagnitude(report.Movers[j])
		if mi != mj {
			return mi > mj
		}
		if report.Movers[i].RunsCurrent != report.Movers[j].RunsCurrent {
			return report.Movers[i].RunsCurrent > report.Movers[j].RunsCurrent
		}
		return report.Movers[i].Key < report.Movers[j].Key
	})

	if len(report.Movers) == 0 {
		switch {
		case report.FilteredOut > 0:
			report.Note = fmt.Sprintf(
				"no group reached %d runs in the current window; %d group(s) were excluded — lower --min-runs to see them",
				minRuns, report.FilteredOut)
		case report.TotalRunsCurrent == 0 && report.TotalRunsPrevious == 0 && report.UnattributedRuns == 0:
			report.Note = fmt.Sprintf(
				"no locally synced runs fall in the last %s or the %s before it; widen --since, or run 'rightbrain-pp-cli sync' to refresh the mirror",
				report.Window, report.Window)
		default:
			report.Note = fmt.Sprintf(
				"runs were found but none carried a %s to group by; try a different --group-by", groupBy)
		}
	}
	return report
}

// driftMagnitude is the ranking score: the largest absolute percentage move the
// group made on any tracked metric. Undefined deltas contribute nothing, so a
// brand-new group sorts below every group with a real measured regression.
func driftMagnitude(m driftMover) float64 {
	best := 0.0
	for _, d := range []*float64{m.CreditsDeltaPct, m.P95DeltaPct, m.FailureRateDeltaPct, m.RunsDeltaPct} {
		if d == nil {
			continue
		}
		if a := math.Abs(*d); a > best {
			best = a
		}
	}
	return best
}

// driftFormatWindow renders the comparison width the way the user typed it
// ("7d", "12h") and falls back to the shared humanizer for odd values.
func driftFormatWindow(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return humanizeDuration(d)
}

// driftLooseFloat extracts a number from a JSON value the API renders either as
// a bare number or as a quoted decimal string (charged_credits is "9.00").
// Reports false for null, empty, and anything strconv cannot read, so the
// caller can exclude the run instead of scoring it as zero.
func driftLooseFloat(raw json.RawMessage) (float64, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, false
	}
	if strings.HasPrefix(s, `"`) {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err != nil {
			return 0, false
		}
		s = strings.TrimSpace(unquoted)
		if s == "" {
			return 0, false
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// driftLooseBool reads is_error whether it arrives as true/false, 0/1, or the
// string "true".
func driftLooseBool(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return false
	}
	if strings.HasPrefix(s, `"`) {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err != nil {
			return false
		}
		s = strings.TrimSpace(unquoted)
	}
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false", "":
		return false
	}
	v, err := strconv.ParseFloat(s, 64)
	return err == nil && v != 0
}

// driftDecodeRun converts one mirrored `data` blob into a driftRun. Rows whose
// created timestamp is unreadable are dropped by the caller: they cannot be
// placed in either window, and guessing would corrupt both.
func driftDecodeRun(data string) (driftRun, bool) {
	var raw driftRawRun
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return driftRun{}, false
	}
	created, ok := parseAPITime(raw.Created)
	if !ok {
		return driftRun{}, false
	}
	run := driftRun{
		TaskID:         raw.TaskID,
		TaskName:       raw.TaskName,
		TaskRevisionID: raw.TaskRevisionID,
		ModelName:      raw.ModelName,
		TaskAgentID:    raw.TaskAgentID,
		AgentName:      raw.AgentName,
		Created:        created,
		IsError:        driftLooseBool(raw.IsError),
	}
	if tokens, ok := driftLooseFloat(raw.TotalTokens); ok {
		run.TotalTokens = int64(tokens)
	}
	if credits, ok := driftLooseFloat(raw.ChargedCredits); ok {
		run.Credits, run.HasCredits = credits, true
	}
	if timing, ok := driftLooseFloat(raw.LLMCallTiming); ok && timing >= 0 {
		run.LatencySecs, run.HasLatency = timing, true
	}
	return run, true
}

// readDriftRuns loads every mirrored row of one resource type. The parent
// cursor is fully drained into plain strings and closed before anything is
// decoded, so no follow-up work happens while the *sql.Rows is open.
func readDriftRuns(ctx context.Context, db *store.Store, resourceType string) ([]driftRun, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, data FROM resources WHERE resource_type = ?`, resourceType)
	if err != nil {
		return nil, fmt.Errorf("reading %s from the local mirror: %w", resourceType, err)
	}
	blobs := make([]string, 0)
	for rows.Next() {
		// Both columns are scanned NULL-safe: a bare string destination errors
		// on a NULL and the surrounding loop would drop the row silently,
		// reporting "no data" where the real answer is "one bad row".
		var id, data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning %s rows: %w", resourceType, err)
		}
		if !data.Valid || strings.TrimSpace(data.String) == "" {
			continue
		}
		blobs = append(blobs, data.String)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("reading %s rows: %w", resourceType, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing %s cursor: %w", resourceType, err)
	}

	runs := make([]driftRun, 0, len(blobs))
	for _, blob := range blobs {
		if run, ok := driftDecodeRun(blob); ok {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagGroupBy string
	var flagMinRuns int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare this window against the previous one across every task and agent to flag what moved",
		Long: "Compare the current window against the immediately preceding window of equal length.\n\n" +
			"Slices locally synced runs into [now-since, now] and [now-2*since, now-since], aggregates " +
			"mean credits, total tokens, p95 latency and failure rate for both, and ranks every group by " +
			"the largest percentage move it made. Groups with fewer than --min-runs runs in the current " +
			"window are dropped and counted in filtered_out; a group with no runs at all in the previous " +
			"window is reported as new, with null deltas rather than an infinite regression.\n\n" +
			"This command never calls the API — it reads only the local mirror, so run 'sync' first.\n\n" +
			"Use this command for project-wide, week-over-week movement in cost, latency, and failure rate " +
			"across all tasks and agents. Do NOT use it to inspect one task's revision weight split; use " +
			"'rollout' instead.",
		Example: "  rightbrain-pp-cli drift --since 7d --group-by task --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare the current window against the previous window of locally synced runs")
				return nil
			}

			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("drift has no live equivalent: it compares two time windows of locally synced runs; run 'rightbrain-pp-cli sync' first"))
			}

			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if window <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: the comparison window must be positive", flagSince))
			}

			groupBy := strings.ToLower(strings.TrimSpace(flagGroupBy))
			if groupBy == "" {
				groupBy = "task"
			}
			valid := false
			for _, v := range driftGroupByValues {
				if groupBy == v {
					valid = true
					break
				}
			}
			if !valid {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --group-by %q: must be one of %s",
					flagGroupBy, strings.Join(driftGroupByValues, ", ")))
			}

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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening the local mirror at %s: %w", dbPath, err)
			}
			defer db.Close()

			resourceTypes := []string{"project_task_run"}
			if groupBy == "agent" {
				resourceTypes = append(resourceTypes, "project_task_agent_run")
			}
			runs := make([]driftRun, 0)
			for _, rt := range resourceTypes {
				batch, err := readDriftRuns(ctx, db, rt)
				if err != nil {
					return err
				}
				runs = append(runs, batch...)
			}

			report := buildDriftReport(runs, time.Now().UTC(), window, groupBy, flagMinRuns)

			hinted := hintIfUnsynced(cmd, db, "project_task_run")
			if !hinted {
				hinted = hintIfStale(cmd, db, "project_task_run", flags.maxAge)
			}

			// An empty report carries its reason in report.Note, but --select
			// projects that field away: `--select movers.name,...` on an empty
			// window prints a bare {"movers": []} with no explanation. Mirror the
			// note onto stderr — the same channel the sync hints above use — so
			// the "why is this empty" answer survives any projection without
			// polluting the stdout JSON that agents parse. Skipped when a sync
			// hint already fired, since that hint names the same cause.
			if !hinted && report.Note != "" && !wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", report.Note)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "current  %s → %s  (%d runs)\n",
					report.CurrentWindowStart, report.CurrentWindowEnd, report.TotalRunsCurrent)
				fmt.Fprintf(out, "previous %s → %s  (%d runs)\n",
					report.PreviousWindowStart, report.PreviousWindowEnd, report.TotalRunsPrevious)
				fmt.Fprintf(out, "window %s, grouped by %s, at least %d run(s) in the current window\n",
					report.Window, report.GroupBy, report.MinRuns)
				if report.UnattributedRuns > 0 {
					fmt.Fprintf(out, "%d run(s) in these windows carry no %s and are excluded from both totals above\n",
						report.UnattributedRuns, report.GroupBy)
				}
				fmt.Fprintln(out)
				if report.Note != "" {
					fmt.Fprintln(out, report.Note)
					return nil
				}
				fmt.Fprintln(out, "NAME\tRUNS\tΔ RUNS\tCREDITS\tΔ CREDITS\tP95\tΔ P95\tFAIL\tΔ FAIL")
				for _, m := range report.Movers {
					label := m.Name
					if label == "" {
						label = m.Key
					}
					fmt.Fprintf(out, "%s\t%d\t%s\t%.4f\t%s\t%.2fs\t%s\t%.1f%%\t%s\n",
						orDash(label), m.RunsCurrent, driftPctCell(m.RunsDeltaPct, m.New),
						m.MeanCreditsCurrent, driftPctCell(m.CreditsDeltaPct, m.New),
						m.P95Current, driftPctCell(m.P95DeltaPct, m.New),
						m.FailureRateCurrent*100, driftPctCell(m.FailureRateDeltaPct, m.New))
				}
				if report.FilteredOut > 0 {
					fmt.Fprintf(out, "\n%d group(s) below --min-runs=%d were excluded; lower it to see them\n",
						report.FilteredOut, report.MinRuns)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "7d",
		"Width of each comparison window, e.g. 24h, 7d, 1w (the previous window of equal length is the baseline)")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "task",
		"Dimension to compare: task, agent, model, or revision")
	cmd.Flags().IntVar(&flagMinRuns, "min-runs", 20,
		"Minimum runs a group needs in the current window to be reported; the rest are counted in filtered_out")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	return cmd
}

// driftPctCell renders a delta for the human table: "new" when there is no
// baseline at all, "-" when this particular ratio is undefined.
func driftPctCell(p *float64, isNew bool) string {
	if isNew {
		return "new"
	}
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%+.1f%%", *p)
}
