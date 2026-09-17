// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type scorecardTarget struct {
	UserID string  `json:"userId"`
	Metric string  `json:"metric"`
	Period string  `json:"period"`
	Value  float64 `json:"value"`
}
type scorecardMetric struct {
	Actual float64 `json:"actual"`
}
type scorecardAgent struct {
	UserID  string                     `json:"userId"`
	Name    string                     `json:"name"`
	Metrics map[string]scorecardMetric `json:"metrics"`
}
type scorecardRow struct {
	UserID        string  `json:"user_id"`
	UserName      string  `json:"user_name"`
	Metric        string  `json:"metric"`
	Period        string  `json:"period"`
	Target        float64 `json:"target"`
	Actual        float64 `json:"actual"`
	AttainmentPct float64 `json:"attainment_pct"`
	PacePct       float64 `json:"pace_pct"`
	Status        string  `json:"status"`
}
type scorecardView struct {
	Rows           []scorecardRow `json:"rows"`
	Timezone       string         `json:"timezone"`
	From           string         `json:"from"`
	To             string         `json:"to"`
	AsOf           string         `json:"as_of"`
	Partial        bool           `json:"partial"`
	Checked        int            `json:"checked"`
	Total          *int           `json:"total,omitempty"`
	Failures       []string       `json:"failures,omitempty"`
	SkippedTargets int            `json:"skipped_targets,omitempty"`
	SkippedPeriods []string       `json:"skipped_periods,omitempty"`
}

func newNovelReportsScorecardCmd(flags *rootFlags) *cobra.Command {
	var flagRange, flagFrom, flagTo, flagUser, flagPeriod, flagAsOf string
	var flagLimit int
	cmd := &cobra.Command{
		Use: "scorecard", Short: "Target versus actual per user and metric with attainment percent and pace to the end of the period.",
		Long:        "Use this command for target-vs-actual attainment per user. Without an explicit window, the window is the UTC calendar day, Monday-based week, month, or quarter containing --as-of (default today). A supplied --range or --from/--to must exactly match one aligned period. For --from/--to, --as-of must be inside that window; when omitted it defaults to today if today is inside the window, otherwise to the nearest window boundary. Do NOT use it for how a report changed versus the prior period; use 'reports compare' instead.",
		Example:     "  conduyt-crm-pp-cli reports scorecard --period month --as-of 2026-09-10 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reports scorecard")
			}
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			now := time.Now().UTC()
			asOf, err := scorecardAsOf(flagAsOf, now)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be zero or positive"))
			}
			from, to, period, asOf, err := scorecardWindow(flagPeriod, flagRange, flagFrom, flagTo, asOf, now, cmd.Flags().Changed("as-of"), cmd.Flags().Changed("range"), cmd.Flags().Changed("from") || cmd.Flags().Changed("to"))
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tp := map[string]string{}
			if flagUser != "" {
				tp["userId"] = flagUser
			}
			view := scorecardView{Rows: []scorecardRow{}, Timezone: "UTC", From: from.Format(time.RFC3339), To: to.Format(time.RFC3339), AsOf: asOf.Format("2006-01-02")}
			targetData, err := c.Get(ctx, "/reports/targets", tp)
			if err != nil {
				return outputScorecardFailure(cmd, flags, view, fmt.Errorf("fetching targets: %w", err))
			}
			var targets []scorecardTarget
			if err := decodeScorecardCollection(targetData, "targets", &targets); err != nil {
				return outputScorecardFailure(cmd, flags, view, fmt.Errorf("decoding targets: %w", err))
			}
			selectedTargets, skipped, skippedPeriods := selectScorecardTargets(targets, period)
			view.SkippedTargets = skipped
			view.SkippedPeriods = skippedPeriods
			params := map[string]string{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339)}
			if flagUser != "" {
				params["userIds"] = flagUser
			}
			perfData, err := c.Get(ctx, "/reports/agent-performance", params)
			if err != nil {
				return outputScorecardFailure(cmd, flags, view, fmt.Errorf("fetching agent performance: %w", err))
			}
			var perf []scorecardAgent
			if err := decodeScorecardCollection(perfData, "rows", &perf); err != nil {
				return outputScorecardFailure(cmd, flags, view, fmt.Errorf("decoding agent performance: %w", err))
			}
			allRows := buildScorecardRows(selectedTargets, perf, period, from, to, asOf.AddDate(0, 0, 1))
			total := len(allRows)
			view.Total = &total
			view.Rows = allRows
			if flagLimit > 0 && len(view.Rows) > flagLimit {
				view.Rows = view.Rows[:flagLimit]
				view.Partial = true
				view.Failures = []string{"--limit capped the scorecard rows"}
			}
			view.Checked = len(view.Rows)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Period: %s to %s (UTC; as of %s)\n", view.From, view.To, view.AsOf)
				if view.Partial {
					fmt.Fprintln(cmd.OutOrStdout(), "WARNING: result is partial because --limit capped the scorecard rows.")
				}
				if view.SkippedTargets > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d target(s) from other periods: %s.\n", view.SkippedTargets, strings.Join(view.SkippedPeriods, ", "))
				}
				if err := printScorecard(cmd, view.Rows); err != nil {
					return err
				}
			}
			if view.Partial {
				return apiErr(fmt.Errorf("reports scorecard is incomplete: --limit capped the rows checked"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagRange, "range", "30d", "Aligned window length as Nd (cannot be a rolling window)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Aligned period start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Aligned period end, exclusive (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "Anchor date for the calendar period and pace (YYYY-MM-DD; default today, UTC)")
	cmd.Flags().StringVar(&flagUser, "user", "", "Limit the scorecard to one user ID")
	cmd.Flags().StringVar(&flagPeriod, "period", "", "Target period: day, week, month, or quarter (required unless the window is exactly one period)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum scorecard rows (0 means no cap)")
	return cmd
}

func scorecardAsOf(raw string, now time.Time) (time.Time, error) {
	if raw == "" {
		raw = now.UTC().Format("2006-01-02")
	}
	asOf, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("--as-of must be YYYY-MM-DD")
	}
	return asOf.UTC(), nil
}

func scorecardWindow(flagPeriod, rawRange, rawFrom, rawTo string, asOf, now time.Time, asOfChanged, rangeChanged, datesChanged bool) (time.Time, time.Time, string, time.Time, error) {
	if datesChanged && (rawFrom == "" || rawTo == "") {
		return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--from and --to must be supplied together")
	}
	if rangeChanged && datesChanged {
		return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--range cannot be combined with --from/--to")
	}

	var from, to time.Time
	if datesChanged {
		var err error
		from, err = time.Parse("2006-01-02", rawFrom)
		if err != nil {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--from must be YYYY-MM-DD")
		}
		to, err = time.Parse("2006-01-02", rawTo)
		if err != nil {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--to must be YYYY-MM-DD")
		}
		if !from.Before(to) {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--from must be before --to")
		}
		if asOfChanged && (asOf.Before(from) || !asOf.Before(to)) {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--as-of must be inside the supplied --from/--to window")
		}
		if !asOfChanged {
			asOf = time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
			if asOf.Before(from) {
				asOf = from
			} else if !asOf.Before(to) {
				asOf = to.AddDate(0, 0, -1)
			}
		}
	} else if rangeChanged {
		days, err := parseDayRange(rawRange)
		if err != nil {
			return time.Time{}, time.Time{}, "", time.Time{}, err
		}
		to = asOf.AddDate(0, 0, 1)
		from = to.AddDate(0, 0, -days)
	}

	period, err := scorecardPeriod(flagPeriod, from, to)
	if err != nil {
		return time.Time{}, time.Time{}, "", time.Time{}, err
	}
	periodFrom, periodTo := scorecardPeriodBounds(period, asOf)
	if rangeChanged || datesChanged {
		alignedFrom, alignedTo := scorecardPeriodBounds(period, from)
		if !from.Equal(alignedFrom) || !to.Equal(alignedTo) {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("the supplied window is not exactly one aligned %s period (%s to %s UTC); omit the window to use the period containing --as-of", scorecardPeriodFlag(period), periodFrom.Format("2006-01-02"), periodTo.Format("2006-01-02"))
		}
		if !from.Equal(periodFrom) || !to.Equal(periodTo) {
			return time.Time{}, time.Time{}, "", time.Time{}, fmt.Errorf("--as-of must select the supplied aligned %s window", scorecardPeriodFlag(period))
		}
		return from.UTC(), to.UTC(), period, asOf, nil
	}
	return periodFrom, periodTo, period, asOf, nil
}

func scorecardPeriodFlag(period string) string {
	return map[string]string{"daily": "day", "weekly": "week", "monthly": "month", "quarterly": "quarter"}[period]
}

func scorecardPeriodBounds(period string, anchor time.Time) (time.Time, time.Time) {
	a := time.Date(anchor.UTC().Year(), anchor.UTC().Month(), anchor.UTC().Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case "daily":
		return a, a.AddDate(0, 0, 1)
	case "weekly":
		from := a.AddDate(0, 0, -((int(a.Weekday()) + 6) % 7))
		return from, from.AddDate(0, 0, 7)
	case "monthly":
		from := time.Date(a.Year(), a.Month(), 1, 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(0, 1, 0)
	default:
		month := time.Month(((int(a.Month())-1)/3)*3 + 1)
		from := time.Date(a.Year(), month, 1, 0, 0, 0, 0, time.UTC)
		return from, from.AddDate(0, 3, 0)
	}
}

func decodeScorecardCollection(raw json.RawMessage, field string, dst any) error {
	if hoisted, ok := hoistPaginatedEnvelope(raw); ok {
		raw = hoisted
	}
	if err := rejectResponseErrorEnvelope(raw); err != nil {
		return err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope["error"] != nil || envelope["message"] != nil {
		return fmt.Errorf("unexpected error envelope")
	}
	dataRaw, ok := envelope["data"]
	if !ok || string(dataRaw) == "null" {
		return fmt.Errorf("response missing data.%s array", field)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &data); err != nil {
		return fmt.Errorf("decoding data: %w", err)
	}
	if data["error"] != nil || data["message"] != nil {
		return fmt.Errorf("unexpected error envelope")
	}
	collection, ok := data[field]
	if !ok || string(collection) == "null" {
		return fmt.Errorf("response missing data.%s array", field)
	}
	if err := json.Unmarshal(collection, dst); err != nil {
		return fmt.Errorf("decoding data.%s: %w", field, err)
	}
	return nil
}

func reportWindow(rawRange, rawFrom, rawTo string, now time.Time) (time.Time, time.Time, error) {
	days, err := parseDayRange(rawRange)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to := now.UTC()
	if rawTo != "" {
		to, err = time.Parse("2006-01-02", rawTo)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--to must be YYYY-MM-DD")
		}
	}
	from := to.Add(-time.Duration(days) * 24 * time.Hour)
	if rawFrom != "" {
		from, err = time.Parse("2006-01-02", rawFrom)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--from must be YYYY-MM-DD")
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("--from must be before --to")
	}
	return from.UTC(), to.UTC(), nil
}

func parseDayRange(v string) (int, error) {
	if !strings.HasSuffix(v, "d") {
		return 0, fmt.Errorf("--range must use Nd, for example 30d")
	}
	d, err := strconv.Atoi(strings.TrimSuffix(v, "d"))
	if err != nil || d < 1 {
		return 0, fmt.Errorf("--range must use a positive Nd value")
	}
	return d, nil
}

func buildScorecardRows(targets []scorecardTarget, agents []scorecardAgent, period string, from, to, now time.Time) []scorecardRow {
	byAgent := make(map[string]scorecardAgent, len(agents))
	for _, a := range agents {
		byAgent[a.UserID] = a
	}
	matched := make(map[string]struct{}, len(targets))
	rows := make([]scorecardRow, 0, len(targets))
	for _, target := range targets {
		key := target.UserID + "\x00" + target.Metric + "\x00" + target.Period
		matched[key] = struct{}{}
		a := byAgent[target.UserID]
		actual := 0.0
		if metric, ok := a.Metrics[target.Metric]; ok {
			actual = metric.Actual
		}
		rows = append(rows, makeScorecardRow(target.UserID, a.Name, target.Metric, target.Period, target.Value, actual, from, to, now))
	}
	for _, a := range agents {
		metrics := make([]string, 0, len(a.Metrics))
		for metric := range a.Metrics {
			metrics = append(metrics, metric)
		}
		sort.Strings(metrics)
		for _, metric := range metrics {
			if _, ok := matched[a.UserID+"\x00"+metric+"\x00"+period]; ok {
				continue
			}
			rows = append(rows, makeScorecardRow(a.UserID, a.Name, metric, period, 0, a.Metrics[metric].Actual, from, to, now))
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].UserID == rows[j].UserID {
			return rows[i].Metric < rows[j].Metric
		}
		return rows[i].UserID < rows[j].UserID
	})
	return rows
}

func outputScorecardFailure(cmd *cobra.Command, flags *rootFlags, view scorecardView, cause error) error {
	view.Partial = true
	view.Failures = []string{cause.Error()}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "WARNING: scorecard is incomplete: %v\n", cause)
	}
	return classifyAPIErrorOnly(cause)
}

func makeScorecardRow(userID, userName, metric, period string, target, actual float64, from, to, now time.Time) scorecardRow {
	attainment, status := 0.0, "no_target"
	if target > 0 {
		attainment, status = actual/target*100, "off_pace"
	}
	elapsed := now.Sub(from).Seconds() / to.Sub(from).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > 1 {
		elapsed = 1
	}
	pace := 0.0
	if elapsed > 0 {
		pace = attainment / elapsed
	}
	if target > 0 && pace >= 100 {
		status = "on_pace"
	}
	return scorecardRow{UserID: userID, UserName: userName, Metric: metric, Period: period, Target: target, Actual: actual, AttainmentPct: attainment, PacePct: pace, Status: status}
}

func printScorecard(cmd *cobra.Command, rows []scorecardRow) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "USER\tNAME\tMETRIC\tPERIOD\tTARGET\tACTUAL\tATTAINMENT\tPACE\tSTATUS")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.2f\t%.2f\t%.1f%%\t%.1f%%\t%s\n", r.UserID, r.UserName, r.Metric, r.Period, r.Target, r.Actual, r.AttainmentPct, r.PacePct, r.Status)
	}
	return tw.Flush()
}

func scorecardPeriod(flag string, from, to time.Time) (string, error) {
	periods := map[string]string{"day": "daily", "week": "weekly", "month": "monthly", "quarter": "quarterly"}
	if flag != "" {
		period, ok := periods[strings.ToLower(flag)]
		if !ok {
			return "", fmt.Errorf("--period must be one of day, week, month, or quarter")
		}
		return period, nil
	}
	if from.IsZero() || to.IsZero() {
		return "", fmt.Errorf("cannot infer a target period without an aligned --range or --from/--to window; pass --period day|week|month|quarter")
	}
	if to.Equal(from.AddDate(0, 0, 1)) {
		return "daily", nil
	}
	if to.Equal(from.AddDate(0, 0, 7)) {
		return "weekly", nil
	}
	if to.Equal(from.AddDate(0, 1, 0)) {
		return "monthly", nil
	}
	if to.Equal(from.AddDate(0, 3, 0)) {
		return "quarterly", nil
	}
	return "", fmt.Errorf("cannot infer a target period from this window; pass --period day|week|month|quarter")
}

func selectScorecardTargets(targets []scorecardTarget, period string) ([]scorecardTarget, int, []string) {
	selected := make([]scorecardTarget, 0, len(targets))
	skipped := make(map[string]struct{})
	skippedCount := 0
	for _, target := range targets {
		if target.Period == period {
			selected = append(selected, target)
			continue
		}
		name := target.Period
		if name == "" {
			name = "(missing)"
		}
		skipped[name] = struct{}{}
		skippedCount++
	}
	periods := make([]string, 0, len(skipped))
	for name := range skipped {
		periods = append(periods, name)
	}
	sort.Strings(periods)
	return selected, skippedCount, periods
}
