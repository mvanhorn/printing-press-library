// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

type tractionRow struct {
	AppID              string `json:"appId"`
	Title              string `json:"title"`
	Storefront         string `json:"storefront"`
	Period             string `json:"period"`
	NewRatings         *int64 `json:"newRatings"`
	RatingDeltas       int    `json:"ratingDeltas"`
	ResetDetected      bool   `json:"reset_detected"`
	RatingCount        *int64 `json:"ratingCount"`
	RatingsSource      string `json:"ratingsSource"`
	BestRank           *int   `json:"bestRank"`
	BestRankChart      string `json:"bestRankChart"`
	DaysCharted        int    `json:"daysCharted"`
	ChartDaysCaptured  int    `json:"chartDaysCaptured"`
	ReviewVolume       int    `json:"reviewVolume"`
	ReviewCapTruncated bool   `json:"review_cap_truncated"`
}

type tractionMeta struct {
	Proxy                  bool              `json:"proxy"`
	DownloadCounts         bool              `json:"download_counts"`
	Period                 string            `json:"period"`
	Apps                   []string          `json:"apps"`
	AppsWithoutData        []string          `json:"apps_without_data"`
	Since                  *string           `json:"since"`
	FirstCaptureAt         *string           `json:"first_capture_at"`
	StorefrontsWithData    []string          `json:"storefronts_with_data"`
	StorefrontsWithoutData []string          `json:"storefronts_without_data"`
	Metrics                map[string]string `json:"metrics"`
	Note                   string            `json:"note"`
}

type tractionView struct {
	Meta    tractionMeta  `json:"meta"`
	Results []tractionRow `json:"results"`
}

const tractionProxyNote = "Public demand proxies, not downloads: Apple publishes no download or install counts."

func tractionMetricDefs() map[string]string {
	return map[string]string{
		"newRatings":           "sum of day-over-day rises in the stored rating count, credited to the later capture day; a drop of more than 1% is a reset whose post-reset count is counted as new; never negative; null when the period has no day-over-day change to measure",
		"ratingDeltas":         "number of day-over-day changes measured in the period",
		"ratingCount":          "last stored rating count in the period (the count since the last reset when one was detected)",
		"ratingsSource":        "lookup (exact Lookup userRatingCount) when the storefront has lookup captures, otherwise page",
		"bestRank":             "best (lowest) stored chart rank in the period across captured charts (overall and Kids charts); bestRankChart names platform/genre/chart, plus /kids-<ageBandId> for a Kids chart",
		"daysCharted":          "UTC days in the period on which the app appeared in a stored chart capture",
		"chartDaysCaptured":    "UTC days in the period with a stored capture of a chart the app has appeared in",
		"reviewVolume":         "stored reviews dated in the period (captured review volume, not every review)",
		"review_cap_truncated": "the storefront has 500 or more captured reviews in one feed sort, so older reviews may be missing",
	}
}

func newNovelTractionCmd(flags *rootFlags) *cobra.Command {
	var flagCountries, flagPeriod, flagSince, flagDB string

	cmd := &cobra.Command{
		Use:   "traction <app> [app...]",
		Short: "Roll up stored per-country demand proxies by day, month, year or all-time: new ratings, chart rank, review volume (never downloads)",
		Long:  "Use this when asked for downloads, installs or demand by country over time: it returns labelled public proxies per storefront (new ratings, chart rank, captured review volume) rolled up daily, monthly, annual or all-time, never download counts. Do NOT use this for the current star histogram in each storefront; use 'ratings' instead, and for one chart's rank trajectory use 'rank-history'. Real per-country downloads exist only in App Store Connect sales reports for apps you own, which the separate asc CLI reads.",
		Example: strings.Trim(`
  app-store-pp-cli traction 324684580 --countries us,gb,pk --period monthly --agent
  app-store-pp-cli traction 324684580 553834731 --period daily --since 30d --json
  app-store-pp-cli traction com.spotify.client --period all-time`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "app=324684580;--countries=us,gb;--period=monthly",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "traction")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("traction needs at least one app: a numeric id (324684580), id324684580, an App Store URL or a bundle id"))
			}
			refs, err := s3bParseAppArgs(args)
			if err != nil {
				return err
			}
			period, err := s3bParsePeriod(flagPeriod)
			if err != nil {
				return err
			}
			since, err := s3bParseSince("since", flagSince, s3bNow())
			if err != nil {
				return err
			}
			scope, err := s3bStorefrontScope(cmd, flags, flagCountries)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			ratingsCmd, topCmd, reviewsCmd := tractionCaptureCmds(s3bRefLabel(refs[0]), scope)
			hint := fmt.Sprintf("capture data first, e.g. 'app-store-pp-cli %s', 'app-store-pp-cli %s' and 'app-store-pp-cli %s', repeated over days", ratingsCmd, topCmd, reviewsCmd)
			st, ok, err := s3bOpenStore(cmd, flags, hint, func(note string) any {
				meta := newTractionMeta(period, since, refs)
				meta.Note = tractionProxyNote + " " + note
				return tractionView{Meta: meta, Results: make([]tractionRow, 0)}
			})
			if err != nil || !ok {
				return err
			}
			defer func() { _ = st.Close() }()

			view, err := buildTraction(ctx, st, refs, period, since, scope)
			if err != nil {
				return s3bLocalReadErr(err)
			}
			if len(view.Meta.Apps) > 0 {
				s3bWarnStaleApps(ctx, cmd, flags, st, view.Meta.Apps, scope, "app-store-pp-cli "+ratingsCmd)
			}
			return renderTraction(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagCountries, "countries", "", "Storefronts to report (us,gb,pk or all); default every storefront with stored data, or --country when set")
	cmd.Flags().StringVar(&flagPeriod, "period", "monthly", "Rollup period: daily, monthly, annual or all-time")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only use captures and reviews in this window (30d, 12w or a date such as 2026-01-01); default all stored history")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local snapshot store path (default: the CLI data directory)")
	return cmd
}

func newTractionMeta(period string, since time.Time, refs []appstore.AppRef) tractionMeta {
	apps := make([]string, 0, len(refs))
	for _, r := range refs {
		apps = append(apps, s3bRefLabel(r))
	}
	return tractionMeta{
		Proxy:                  true,
		DownloadCounts:         false,
		Period:                 period,
		Apps:                   apps,
		AppsWithoutData:        make([]string, 0),
		Since:                  s3bTimePtr(since),
		StorefrontsWithData:    make([]string, 0),
		StorefrontsWithoutData: make([]string, 0),
		Metrics:                tractionMetricDefs(),
	}
}

// tractionBucket accumulates one storefront's metrics for one period.
type tractionBucket struct {
	newRatings   int64
	deltas       int
	reset        bool
	count        *int64
	countAt      time.Time
	bestRank     int
	bestChart    string
	chartedDays  map[string]bool
	capturedDays map[string]bool
	reviews      int
}

func buildTraction(ctx context.Context, st *store.Store, refs []appstore.AppRef, period string, since time.Time, scope s3bScope) (tractionView, error) {
	view := tractionView{Meta: newTractionMeta(period, since, refs), Results: make([]tractionRow, 0)}
	ids, missing, err := s3bResolveLocalApps(ctx, st, refs)
	if err != nil {
		return view, err
	}
	titles, err := s3bAppTitles(ctx, st)
	if err != nil {
		return view, err
	}
	view.Meta.Apps = ids
	view.Meta.AppsWithoutData = append(view.Meta.AppsWithoutData, missing...)
	withData := map[string]bool{}
	var first time.Time
	for _, id := range ids {
		rows, appFirst, err := tractionForApp(ctx, st, id, titles[id], period, since, scope)
		if err != nil {
			return view, err
		}
		if len(rows) == 0 {
			view.Meta.AppsWithoutData = append(view.Meta.AppsWithoutData, id)
			continue
		}
		first = s3bMinTime(first, appFirst)
		for _, r := range rows {
			withData[r.Storefront] = true
		}
		view.Results = append(view.Results, rows...)
	}
	view.Meta.FirstCaptureAt = s3bTimePtr(first)
	view.Meta.StorefrontsWithData = s3bSortedKeys(withData)
	for _, sf := range scope.list() {
		if !withData[sf] {
			view.Meta.StorefrontsWithoutData = append(view.Meta.StorefrontsWithoutData, sf)
		}
	}
	if len(view.Results) == 0 {
		ratingsCmd, topCmd, reviewsCmd := tractionCaptureCmds(s3bRefLabel(refs[0]), scope)
		view.Meta.Note = tractionProxyNote + fmt.Sprintf(" No stored ratings, chart or review captures match these apps and storefronts yet; capture with '%s', '%s' and '%s', repeated over days to build a series.", ratingsCmd, topCmd, reviewsCmd)
	} else {
		view.Meta.Note = tractionProxyNote + " Values come only from captures stored on this machine; a period with a single capture day has no day-over-day change, so its newRatings is null."
		if missing := s3bNoDataNote(view.Meta.StorefrontsWithoutData, view.Meta.AppsWithoutData); missing != "" {
			view.Meta.Note += " " + missing
		}
	}
	return view, nil
}

// tractionCaptureCmds returns the ratings, top and reviews captures that feed
// traction for app label, in the requested storefronts (us,gb when none was
// named). top reads one storefront per run, so it names the first.
func tractionCaptureCmds(label string, scope s3bScope) (ratings, top, reviews string) {
	countries, first := "us,gb", "us"
	if list := scope.list(); len(list) > 0 {
		countries, first = strings.Join(list, ","), list[0]
	}
	return "ratings " + label + " --countries " + countries,
		"top --genre 36 --country " + first,
		"reviews " + label + " --countries " + countries
}

// tractionForApp builds one app's rows (storefront asc, period asc) and the
// time of its first stored capture in scope.
func tractionForApp(ctx context.Context, st *store.Store, appID, title, period string, since time.Time, scope s3bScope) ([]tractionRow, time.Time, error) {
	var first time.Time
	buckets := map[string]map[string]*tractionBucket{}
	get := func(sf, label string) *tractionBucket {
		if buckets[sf] == nil {
			buckets[sf] = map[string]*tractionBucket{}
		}
		b := buckets[sf][label]
		if b == nil {
			b = &tractionBucket{chartedDays: map[string]bool{}, capturedDays: map[string]bool{}}
			buckets[sf][label] = b
		}
		return b
	}

	// Ratings: one source per storefront, collapsed to one count per UTC day.
	ratings, err := store.RatingsSeries(ctx, st, appID, "", "")
	if err != nil {
		return nil, first, err
	}
	for _, r := range ratings {
		if scope.has(r.Storefront) {
			first = s3bMinTime(first, r.CapturedAt)
		}
	}
	sources := map[string]string{}
	for sf, s := range s3bRatingsByStorefront(ratings, scope, since) {
		sources[sf] = s.Source
		days := s3bDailyCounts(s.Snaps)
		for _, d := range days {
			b := get(sf, s3bBucket(d.Day, period))
			if b.count == nil || !d.At.Before(b.countAt) {
				c := d.Count
				b.count, b.countAt = &c, d.At
			}
		}
		for _, inc := range s3bRatingIncrements(days) {
			b := get(sf, s3bBucket(inc.Day, period))
			b.newRatings += inc.New
			b.deltas++
			if inc.Reset {
				b.reset = true
			}
		}
	}

	// Charts: best rank and charted days from the app's stored chart rows.
	chartRows, err := store.ChartRows(ctx, st, store.ChartQuery{AppID: appID, Storefronts: scope.list(), AgeBand: store.AnyAgeBand})
	if err != nil {
		return nil, first, err
	}
	keys := map[string]map[string]store.ChartKey{}
	for _, r := range chartRows {
		first = s3bMinTime(first, r.CapturedAt)
		name := s3bChartSeriesName(r.Platform, r.GenreID, r.AgeBand, r.Chart)
		if keys[r.Storefront] == nil {
			keys[r.Storefront] = map[string]store.ChartKey{}
		}
		keys[r.Storefront][name] = store.ChartKey{Storefront: r.Storefront, Platform: r.Platform, GenreID: r.GenreID, AgeBand: r.AgeBand, Chart: r.Chart}
		if !since.IsZero() && r.CapturedAt.Before(since) {
			continue
		}
		b := get(r.Storefront, s3bBucket(r.CapturedAt, period))
		b.chartedDays[s3bDayKey(r.CapturedAt)] = true
		if b.bestRank == 0 || r.Rank < b.bestRank || (r.Rank == b.bestRank && name < b.bestChart) {
			b.bestRank, b.bestChart = r.Rank, name
		}
	}
	// Denominator: days with a capture of any chart this app has appeared in.
	for _, sf := range s3bSortedKeys(keys) {
		for _, name := range s3bSortedKeys(keys[sf]) {
			caps, err := store.ChartCaptures(ctx, st, keys[sf][name])
			if err != nil {
				return nil, first, err
			}
			for _, c := range caps {
				if !since.IsZero() && c.CapturedAt.Before(since) {
					continue
				}
				get(sf, s3bBucket(c.CapturedAt, period)).capturedDays[s3bDayKey(c.CapturedAt)] = true
			}
		}
	}

	// Reviews: captured review volume by review date, plus the feed-cap flag.
	reviews, err := store.Reviews(ctx, st, store.ReviewFilter{AppID: appID, Storefronts: scope.list()})
	if err != nil {
		return nil, first, err
	}
	// Apple's reviews RSS stops at 10 pages of 50 per sort. A stored capture
	// whose page 10 came back full means older reviews exist beyond the cap;
	// it is read from the capture log, because last-seen times move forward
	// whenever a later capture sees a review again.
	reviewStorefronts := map[string]bool{}
	for _, r := range reviews {
		first = s3bMinTime(first, r.FirstSeenAt)
		reviewStorefronts[r.Storefront] = true
		t, ok := s3bParseStoredTime(r.ReviewedAt)
		if !ok || (!since.IsZero() && t.Before(since)) {
			continue
		}
		get(r.Storefront, s3bBucket(t, period)).reviews++
	}
	truncated := map[string]bool{}
	for sf := range reviewStorefronts {
		capped, err := store.ReviewFeedCapped(ctx, st, appID, sf)
		if err != nil {
			return nil, first, err
		}
		truncated[sf] = capped
	}

	rows := make([]tractionRow, 0)
	for _, sf := range s3bSortedKeys(buckets) {
		for _, label := range s3bSortedKeys(buckets[sf]) {
			b := buckets[sf][label]
			row := tractionRow{
				AppID:              appID,
				Title:              title,
				Storefront:         sf,
				Period:             label,
				RatingDeltas:       b.deltas,
				ResetDetected:      b.reset,
				RatingCount:        b.count,
				RatingsSource:      sources[sf],
				BestRankChart:      b.bestChart,
				DaysCharted:        len(b.chartedDays),
				ChartDaysCaptured:  len(b.capturedDays),
				ReviewVolume:       b.reviews,
				ReviewCapTruncated: truncated[sf],
			}
			if b.deltas > 0 {
				n := b.newRatings
				row.NewRatings = &n
			}
			if b.bestRank > 0 {
				r := b.bestRank
				row.BestRank = &r
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Storefront != rows[j].Storefront {
			return rows[i].Storefront < rows[j].Storefront
		}
		return rows[i].Period < rows[j].Period
	})
	return rows, first, nil
}

func renderTraction(cmd *cobra.Command, flags *rootFlags, view tractionView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if len(view.Results) == 0 {
		_, _ = fmt.Fprintln(w, view.Meta.Note)
		return nil
	}
	multi := len(view.Meta.Apps) > 1
	headers := []string{"storefront", "period", "new ratings", "reset", "rating count", "best rank", "days charted", "reviews"}
	if multi {
		headers = append([]string{"app"}, headers...)
	}
	rows := make([][]string, 0, len(view.Results))
	for _, r := range view.Results {
		reset := ""
		if r.ResetDetected {
			reset = "reset"
		}
		reviews := fmt.Sprint(r.ReviewVolume)
		if r.ReviewCapTruncated {
			reviews += " (capped)"
		}
		cells := []string{r.Storefront, r.Period, s3bOptInt(r.NewRatings), reset, s3bOptInt(r.RatingCount), s3bOptRank(r.BestRank),
			fmt.Sprintf("%d/%d", r.DaysCharted, r.ChartDaysCaptured), reviews}
		if multi {
			cells = append([]string{r.AppID}, cells...)
		}
		rows = append(rows, cells)
	}
	if err := s3bPrintTable(w, headers, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(w, "\n"+tractionProxyNote)
	if missing := s3bNoDataNote(view.Meta.StorefrontsWithoutData, view.Meta.AppsWithoutData); missing != "" {
		_, _ = fmt.Fprintln(w, missing)
	}
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelTractionCmd(flags))
	})
}
