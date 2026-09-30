// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

const (
	pulseStatusOK        = "ok"
	pulseStatusLowVolume = "low-volume"
	pulseFlagStopped     = "stopped"
	pulseFlagDropped     = "dropped"
	pulseFlagSpiked      = "spiked"

	pulseByServer = "server"
	pulseByStream = "stream"
	pulseByTag    = "tag"

	// Postmark's guidance: investigate bounce rates at 5% and complaint
	// rates at 0.1%.
	pulseBounceFloorPct = 5.0
	pulseSpamFloorPct   = 0.1
	pulseDropRatio      = 0.5
)

type pulseFlag struct {
	Flag   string `json:"flag"`
	Reason string `json:"reason"`
	Next   string `json:"next"`
}

type pulseRow struct {
	Unit                  string      `json:"unit"`
	Server                string      `json:"server"`
	ServerID              int64       `json:"server_id"`
	Stream                string      `json:"stream"`
	Tag                   string      `json:"tag"`
	Status                string      `json:"status"`
	WindowSent            int         `json:"window_sent"`
	WindowBounced         int         `json:"window_bounced"`
	WindowSpam            int         `json:"window_spam"`
	WindowBounceRatePct   float64     `json:"window_bounce_rate_pct"`
	WindowSpamRatePct     float64     `json:"window_spam_rate_pct"`
	BaselineSent          int         `json:"baseline_sent"`
	BaselineBounced       int         `json:"baseline_bounced"`
	BaselineSpam          int         `json:"baseline_spam"`
	BaselineDailyAvg      float64     `json:"baseline_daily_avg"`
	BaselineBounceRatePct float64     `json:"baseline_bounce_rate_pct"`
	BaselineSpamRatePct   float64     `json:"baseline_spam_rate_pct"`
	ExpectedWindowSent    float64     `json:"expected_window_sent"`
	ChangePct             *float64    `json:"change_pct"`
	Flags                 []pulseFlag `json:"flags"`
}

type pulseResult struct {
	By            string                 `json:"by"`
	Source        string                 `json:"source"`
	WindowDays    int                    `json:"window_days"`
	BaselineDays  int                    `json:"baseline_days"`
	BaselineStart string                 `json:"baseline_start"`
	WindowStart   string                 `json:"window_start"`
	WindowEnd     string                 `json:"window_end"`
	Timezone      string                 `json:"timezone"`
	MinBaseline   float64                `json:"min_baseline_per_day"`
	Units         int                    `json:"units"`
	Flagged       int                    `json:"flagged"`
	Rows          []pulseRow             `json:"rows"`
	FetchFailures []postmarkFetchFailure `json:"fetch_failures"`
	Note          string                 `json:"note,omitempty"`
}

// pulseWindow holds the resolved Eastern-time day range. The window ends
// today (inclusive) and the baseline is the span immediately before it.
type pulseWindow struct {
	windowDays, baselineDays int
	baselineStart            time.Time
	windowStart              time.Time
	end                      time.Time
}

func newPulseWindow(now time.Time, windowDays, baselineDays int) pulseWindow {
	today := postmarkDayStart(now)
	windowStart := today.AddDate(0, 0, -(windowDays - 1))
	return pulseWindow{
		windowDays:    windowDays,
		baselineDays:  baselineDays,
		baselineStart: windowStart.AddDate(0, 0, -baselineDays),
		windowStart:   windowStart,
		end:           today,
	}
}

func (w pulseWindow) totalDays() int { return w.windowDays + w.baselineDays }

// pulseEvaluate compares the last windowDays of a zero-filled daily series
// against the baselineDays before it. series must hold baseline days first,
// then window days (len == baselineDays + windowDays).
func pulseEvaluate(series []postmarkDay, windowDays, baselineDays int, minBaseline float64) pulseRow {
	var row pulseRow
	split := len(series) - windowDays
	if split < 0 {
		split = 0
	}
	for i, d := range series {
		if i < split {
			row.BaselineSent += d.Sent
			row.BaselineBounced += d.Bounced
			row.BaselineSpam += d.Spam
			continue
		}
		row.WindowSent += d.Sent
		row.WindowBounced += d.Bounced
		row.WindowSpam += d.Spam
	}
	if baselineDays > 0 {
		row.BaselineDailyAvg = roundTo(float64(row.BaselineSent)/float64(baselineDays), 3)
	}
	expected := row.BaselineDailyAvg * float64(windowDays)
	row.ExpectedWindowSent = roundTo(expected, 2)
	if expected > 0 {
		change := roundTo((float64(row.WindowSent)-expected)/expected*100, 1)
		row.ChangePct = &change
	}
	row.WindowBounceRatePct = ratePct(row.WindowBounced, row.WindowSent)
	row.WindowSpamRatePct = ratePct(row.WindowSpam, row.WindowSent)
	row.BaselineBounceRatePct = ratePct(row.BaselineBounced, row.BaselineSent)
	row.BaselineSpamRatePct = ratePct(row.BaselineSpam, row.BaselineSent)

	row.Flags = make([]pulseFlag, 0)
	established := row.BaselineDailyAvg >= minBaseline && row.BaselineDailyAvg > 0
	switch {
	case established && row.WindowSent == 0:
		row.Flags = append(row.Flags, pulseFlag{Flag: pulseFlagStopped, Reason: fmt.Sprintf("0 sends in the last %d days; baseline averaged %.1f/day", windowDays, row.BaselineDailyAvg)})
	case established && float64(row.WindowSent) < pulseDropRatio*expected:
		row.Flags = append(row.Flags, pulseFlag{Flag: pulseFlagDropped, Reason: fmt.Sprintf("%d sends vs %.0f expected from the baseline (%.1f/day)", row.WindowSent, expected, row.BaselineDailyAvg)})
	}
	if row.WindowSent > 0 {
		bounceSpike := row.WindowBounceRatePct >= pulseBounceFloorPct && row.WindowBounceRatePct >= 2*row.BaselineBounceRatePct
		spamSpike := row.WindowSpamRatePct >= pulseSpamFloorPct
		if bounceSpike {
			row.Flags = append(row.Flags, pulseFlag{Flag: pulseFlagSpiked, Reason: fmt.Sprintf("bounce rate %.2f%% vs %.2f%% baseline", row.WindowBounceRatePct, row.BaselineBounceRatePct)})
		}
		if spamSpike {
			row.Flags = append(row.Flags, pulseFlag{Flag: pulseFlagSpiked, Reason: fmt.Sprintf("spam complaint rate %.3f%% (threshold %.1f%%)", row.WindowSpamRatePct, pulseSpamFloorPct)})
		}
	}
	switch {
	case len(row.Flags) > 0:
		row.Status = row.Flags[0].Flag
	case !established:
		row.Status = pulseStatusLowVolume
	default:
		row.Status = pulseStatusOK
	}
	return row
}

// pulseAttachNext fills each flag's next command for the row's scope.
func pulseAttachNext(row *pulseRow, w pulseWindow) {
	scope := postmarkServerArg(row.Server)
	if row.Stream != "" {
		scope += " --messagestream " + shellQuoteWord(row.Stream)
	}
	if row.Tag != "" {
		scope += " --tag " + shellQuoteWord(row.Tag)
	}
	windowStart := w.windowStart.Format(postmarkDateLayout)
	for i := range row.Flags {
		f := &row.Flags[i]
		switch {
		case f.Flag == pulseFlagStopped:
			f.Next = "postmark-pp-cli messages list" + scope + " --count 5 --json"
		case f.Flag == pulseFlagDropped:
			f.Next = fmt.Sprintf("postmark-pp-cli stats sends%s --fromdate %s --todate %s --json", scope, w.baselineStart.Format(postmarkDateLayout), w.end.Format(postmarkDateLayout))
		case strings.HasPrefix(f.Reason, "spam"):
			f.Next = fmt.Sprintf("postmark-pp-cli bounces list%s --type %s --fromdate %s --json", scope, postmarkSpamComplaint, windowStart)
		default:
			f.Next = fmt.Sprintf("postmark-pp-cli bounces list%s --fromdate %s --count 50 --json", scope, windowStart)
		}
	}
}

func pulseStatusRank(status string) int {
	switch status {
	case pulseFlagStopped:
		return 0
	case pulseFlagSpiked:
		return 1
	case pulseFlagDropped:
		return 2
	case pulseStatusOK:
		return 3
	default:
		return 4
	}
}

func sortPulseRows(rows []pulseRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := pulseStatusRank(rows[i].Status), pulseStatusRank(rows[j].Status)
		if ri != rj {
			return ri < rj
		}
		if rows[i].WindowSent != rows[j].WindowSent {
			return rows[i].WindowSent > rows[j].WindowSent
		}
		return rows[i].Unit < rows[j].Unit
	})
}

func newNovelPulseCmd(flags *rootFlags) *cobra.Command {
	var flagWindow, flagBaseline, flagBy, dbPath string
	var minBaseline float64

	cmd := &cobra.Command{
		Use:   "pulse",
		Short: "Flag servers, streams, or tags whose sends or bounce/spam rates changed vs their own baseline",
		Long: strings.Trim(`
Use this command to find servers, streams, or tags whose send volume, bounce rate, or spam-complaint rate changed against their own recent history, for example a product that stopped sending after a deploy. Do NOT use this command for a current snapshot of every server; use 'overview' instead. Do NOT use it for absolute bounce and spam threshold checks per stream; use 'streams health' instead.

Flags per row: stopped (no sends in the window while the baseline averaged at
least --min-baseline sends/day), dropped (under half the sends the baseline
predicts), spiked (bounce rate at least 2x baseline and 5% or more, or spam
complaint rate 0.1% or more). Each flag carries a next command.

--by server and --by stream read Postmark's daily stats live for every server
the account token lists (or only --server). --by tag reads the local archive
populated by 'sync'. Days are Eastern Time, the zone Postmark stats use.`, "\n"),
		Example: strings.Trim(`
  postmark-pp-cli pulse --window 7d --baseline 28d --agent
  postmark-pp-cli pulse --by stream --server "Main App" --json
  postmark-pp-cli pulse --by tag --window 14d --min-baseline 1 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pulse")
			}
			windowDays, err := postmarkParseDays(flagWindow, "window")
			if err != nil {
				return err
			}
			baselineDays, err := postmarkParseDays(flagBaseline, "baseline")
			if err != nil {
				return err
			}
			if minBaseline < 0 {
				return usageErr(errors.New("--min-baseline must be zero or more"))
			}
			by := strings.ToLower(strings.TrimSpace(flagBy))
			switch by {
			case pulseByServer, pulseByStream:
				if flags.dataSource == "local" {
					return usageErr(fmt.Errorf("--by %s reads Postmark's live stats; use --by tag for the local archive", by))
				}
			case pulseByTag:
				if flags.dataSource == "live" {
					return usageErr(errors.New("--by tag reads the local archive populated by sync; drop --data-source live"))
				}
				if _, explicit := postmarkSelectedServer(); explicit {
					return usageErr(errors.New("--by tag reads the whole local archive, and archived messages carry no server ID, so it cannot be limited to one server; drop --server (and POSTMARK_SERVER) or use --by server"))
				}
			default:
				return usageErr(fmt.Errorf("--by must be server, stream, or tag (got %q)", flagBy))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			w := newPulseWindow(time.Now(), windowDays, baselineDays)
			result := pulseResult{
				By:            by,
				Source:        postmarkSourceLive,
				WindowDays:    windowDays,
				BaselineDays:  baselineDays,
				BaselineStart: w.baselineStart.Format(postmarkDateLayout),
				WindowStart:   w.windowStart.Format(postmarkDateLayout),
				WindowEnd:     w.end.Format(postmarkDateLayout),
				Timezone:      postmarkEasternZone,
				MinBaseline:   minBaseline,
				Rows:          make([]pulseRow, 0),
				FetchFailures: make([]postmarkFetchFailure, 0),
			}
			if by == pulseByTag {
				result.Source = postmarkSourceLocal
				if dbPath == "" {
					dbPath = defaultDBPath("postmark-pp-cli")
				}
				if localMirrorMissing(cmd.ErrOrStderr(), dbPath, "messages,bounces") {
					result.Note = "no local archive; run sync first"
					return pulseOutput(cmd, flags, result)
				}
				rows, err := pulseLocalTagRows(ctx, cmd, dbPath, w, minBaseline)
				if err != nil {
					return err
				}
				result.Rows = rows
			} else {
				targets, err := resolvePostmarkTargets(ctx, flags, targetScope{allServers: true, dogfoodCap: true})
				if err != nil {
					return err
				}
				perServer, failures := fanoutTargets(ctx, targets, func(ctx context.Context, t postmarkTarget) ([]pulseRow, error) {
					return pulseServerRows(ctx, t, by, w, minBaseline)
				})
				for _, rows := range perServer {
					result.Rows = append(result.Rows, rows...)
				}
				result.FetchFailures = failures
				warnFanoutFailures(cmd.ErrOrStderr(), len(failures), len(targets), "server")
				if err := postmarkAllFailed(failures, len(targets)); err != nil {
					return err
				}
			}
			for i := range result.Rows {
				pulseAttachNext(&result.Rows[i], w)
				if len(result.Rows[i].Flags) > 0 {
					result.Flagged++
				}
			}
			sortPulseRows(result.Rows)
			result.Units = len(result.Rows)
			return pulseOutput(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Recent span to check, ending today (e.g. 7d, 2w)")
	cmd.Flags().StringVar(&flagBaseline, "baseline", "28d", "Span immediately before the window used as each unit's normal (e.g. 28d, 4w)")
	cmd.Flags().StringVar(&flagBy, "by", pulseByServer, "Group by server, stream, or tag (tag reads the local archive)")
	cmd.Flags().Float64Var(&minBaseline, "min-baseline", 5, "Minimum baseline sends/day before stopped or dropped can fire")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local archive path for --by tag (default: the CLI's data.db)")
	return cmd
}

// pulseServerRows builds one row for the server, or one per outbound stream.
func pulseServerRows(ctx context.Context, t postmarkTarget, by string, w pulseWindow, minBaseline float64) ([]pulseRow, error) {
	type unit struct{ stream string }
	units := []unit{{}}
	if by == pulseByStream {
		streams, err := listPostmarkStreams(ctx, t.client, true)
		if err != nil {
			return nil, err
		}
		units = units[:0]
		for _, s := range streams {
			if !s.active() {
				continue
			}
			units = append(units, unit{stream: s.ID})
		}
	}
	rows := make([]pulseRow, 0, len(units))
	for _, u := range units {
		days, err := fetchPostmarkDailyStats(ctx, t.client, w.baselineStart, w.end, u.stream, "")
		if err != nil {
			return nil, err
		}
		row := pulseEvaluate(zeroFillDays(days, w.baselineStart, w.totalDays()), w.windowDays, w.baselineDays, minBaseline)
		row.Server, row.ServerID, row.Stream = t.Name, t.ID, u.stream
		row.Unit = t.Name
		if u.stream != "" {
			row.Unit = t.Name + "/" + u.stream
		}
		rows = append(rows, row)
	}
	return rows, nil
}

const pulseUntagged = "(untagged)"

// pulseLocalTagRows groups synced messages and bounces by Tag into daily
// series and evaluates each tag.
func pulseLocalTagRows(ctx context.Context, cmd *cobra.Command, dbPath string, w pulseWindow, minBaseline float64) ([]pulseRow, error) {
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local archive: %w", err)
	}
	defer db.Close()
	hintIfUnsynced(cmd, db, "messages")
	hintIfStale(cmd, db, "messages", 24*time.Hour)

	byTag := map[string]map[string]*postmarkDay{}
	bump := func(tag, date string, apply func(*postmarkDay)) {
		if tag == "" {
			tag = pulseUntagged
		}
		days, ok := byTag[tag]
		if !ok {
			days = map[string]*postmarkDay{}
			byTag[tag] = days
		}
		d, ok := days[date]
		if !ok {
			d = &postmarkDay{Date: date}
			days[date] = d
		}
		apply(d)
	}
	inRange := func(ts string) (string, bool) {
		t, ok := postmarkParseTime(ts)
		if !ok {
			return "", false
		}
		day := postmarkDayStart(t)
		if day.Before(w.baselineStart) || day.After(w.end) {
			return "", false
		}
		return day.Format(postmarkDateLayout), true
	}

	err = queryArchive(ctx, db, "local archive", `SELECT COALESCE(json_extract(data,'$.Tag'),''), COALESCE(json_extract(data,'$.ReceivedAt'),'') FROM resources WHERE resource_type = 'messages'`, 2, func(c []string) error {
		if date, ok := inRange(c[1]); ok {
			bump(c[0], date, func(d *postmarkDay) { d.Sent++ })
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = queryArchive(ctx, db, "local archive", `SELECT COALESCE(json_extract(data,'$.Tag'),''), COALESCE(json_extract(data,'$.BouncedAt'),''), COALESCE(json_extract(data,'$.Type'),'') FROM resources WHERE resource_type = 'bounces'`, 3, func(c []string) error {
		date, ok := inRange(c[1])
		if !ok {
			return nil
		}
		if isSpamComplaint(c[2]) {
			bump(c[0], date, func(d *postmarkDay) { d.Spam++ })
			return nil
		}
		bump(c[0], date, func(d *postmarkDay) { d.Bounced++ })
		return nil
	})
	if err != nil {
		return nil, err
	}
	rows := make([]pulseRow, 0, len(byTag))
	for tag, days := range byTag {
		row := pulseEvaluate(zeroFillDays(days, w.baselineStart, w.totalDays()), w.windowDays, w.baselineDays, minBaseline)
		row.Unit = tag
		if tag != pulseUntagged {
			row.Tag = tag
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func pulseOutput(cmd *cobra.Command, flags *rootFlags, result pulseResult) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Pulse by %s: window %s..%s vs baseline from %s (Eastern)\n", result.By, result.WindowStart, result.WindowEnd, result.BaselineStart)
	if len(result.Rows) == 0 {
		fmt.Fprintln(out, "No units to compare.")
		if result.Note != "" {
			fmt.Fprintln(out, result.Note)
		}
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "UNIT\tSTATUS\tWINDOW\tEXPECTED\tCHANGE\tBOUNCE%\tSPAM%")
	for _, r := range result.Rows {
		change := "-"
		if r.ChangePct != nil {
			change = fmt.Sprintf("%+.0f%%", *r.ChangePct)
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%.0f\t%s\t%.2f\t%.3f\n", r.Unit, r.Status, r.WindowSent, r.ExpectedWindowSent, change, r.WindowBounceRatePct, r.WindowSpamRatePct)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for _, r := range result.Rows {
		for _, f := range r.Flags {
			fmt.Fprintf(out, "\n%s %s: %s\n  next: %s\n", r.Unit, f.Flag, f.Reason, f.Next)
		}
	}
	if len(result.FetchFailures) > 0 {
		fmt.Fprintf(out, "\npartial results: %d server fetches failed\n", len(result.FetchFailures))
	}
	return nil
}
