// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// Impact metrics, in output order.
const (
	impactMetricRank     = "bestRank"
	impactMetricRatings  = "newRatingsPerDay"
	impactMetricReviews  = "reviewStars"
	impactMetricKeywords = "keywordPosition"
)

// impactMinPoints is the fewest captures a window needs before its value is
// compared; below it the row is insufficient_data and has no delta.
const impactMinPoints = 2

type impactRow struct {
	Storefront       string   `json:"storefront"`
	Metric           string   `json:"metric"`
	Series           string   `json:"series"`
	Before           *float64 `json:"before"`
	After            *float64 `json:"after"`
	Delta            *float64 `json:"delta"`
	PointsBefore     int      `json:"pointsBefore"`
	PointsAfter      int      `json:"pointsAfter"`
	InsufficientData bool     `json:"insufficient_data"`
	Detail           string   `json:"detail"`
}

// settle applies the insufficient-data rule and computes delta = after - before.
func (r *impactRow) settle() {
	r.InsufficientData = r.PointsBefore < impactMinPoints || r.PointsAfter < impactMinPoints
	r.Delta = nil
	if !r.InsufficientData && r.Before != nil && r.After != nil {
		d := s3bRound2(*r.After - *r.Before)
		r.Delta = &d
	}
}

type impactEvent struct {
	Kind          string   `json:"kind"`
	At            string   `json:"at"`
	Version       string   `json:"version,omitempty"`
	Storefront    string   `json:"storefront,omitempty"`
	ChangedFields []string `json:"changed_fields,omitempty"`
	Detail        string   `json:"detail,omitempty"`
	ResolvedFrom  string   `json:"resolved_from"`
}

type impactWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type impactMeta struct {
	App                    string            `json:"app"`
	AppID                  string            `json:"app_id"`
	Title                  string            `json:"title"`
	At                     string            `json:"at"`
	Event                  *impactEvent      `json:"event"`
	Window                 string            `json:"window"`
	WindowDays             float64           `json:"window_days"`
	Before                 *impactWindow     `json:"before"`
	After                  *impactWindow     `json:"after"`
	AfterComplete          *bool             `json:"after_complete"`
	Storefronts            []string          `json:"storefronts"`
	StorefrontsWithoutData []string          `json:"storefronts_without_data"`
	CoMovementOnly         bool              `json:"co_movement_only"`
	Direction              map[string]string `json:"direction"`
	Rule                   string            `json:"insufficient_data_rule"`
	Note                   string            `json:"note"`
}

type impactView struct {
	Meta    impactMeta  `json:"meta"`
	Results []impactRow `json:"results"`
}

const impactCoMovementNote = "Co-movement only: before and after values show what moved around the event, not why it moved."

// impactAtSpec is a parsed --at value.
type impactAtSpec struct {
	Kind    string // latest | version | listing | featured | date
	Version string
	Date    time.Time
	Raw     string
}

func parseImpactAt(raw string) (impactAtSpec, error) {
	v := strings.TrimSpace(raw)
	low := strings.ToLower(v)
	switch {
	case low == "" || low == "latest":
		return impactAtSpec{Kind: "latest", Raw: "latest"}, nil
	case low == "listing":
		return impactAtSpec{Kind: "listing", Raw: "listing"}, nil
	case low == "featured":
		return impactAtSpec{Kind: "featured", Raw: "featured"}, nil
	case strings.HasPrefix(low, "version:"):
		ver := strings.TrimSpace(v[len("version:"):])
		if ver == "" {
			return impactAtSpec{}, usageErr(fmt.Errorf("--at version: needs a version, e.g. --at version:9.1.92"))
		}
		return impactAtSpec{Kind: "version", Version: ver, Raw: v}, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return impactAtSpec{Kind: "date", Date: t.UTC(), Raw: v}, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return impactAtSpec{Kind: "date", Date: t.UTC(), Raw: v}, nil
	}
	return impactAtSpec{}, usageErr(fmt.Errorf("--at %q: use latest, version:<version>, listing, featured or a date such as 2026-10-01", raw))
}

func newNovelImpactCmd(flags *rootFlags) *cobra.Command {
	var flagAt, flagWindow, flagCountries, flagDB string

	cmd := &cobra.Command{
		Use:   "impact <app>",
		Short: "Compare rank, new ratings, review stars and keyword positions in equal windows before and after a release, listing change or date",
		Long:  "Use this to measure how rank, new ratings, review stars and tracked keyword positions moved in equal windows before and after a release, a listing change, an editorial placement or a given date. Do NOT use this to see what changed; use 'watch-listing' for one app's field diff or 'changes' for a feed across apps, and for complaint terms by version use 'review-digest'.",
		Example: strings.Trim(`
  app-store-pp-cli impact 324684580 --at latest --window 7d --agent
  app-store-pp-cli impact 324684580 --at version:9.1.92 --window 14d --countries us,gb --json
  app-store-pp-cli impact 553834731 --at featured --window 7d
  app-store-pp-cli impact 553834731 --at 2026-10-01 --window 7d`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "app=324684580;--at=latest;--window=7d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "impact")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("impact needs one app: a numeric id (324684580), id324684580, an App Store URL or a bundle id"))
			}
			if len(args) > 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("impact takes one app, got %d (%s); run it once per app", len(args), strings.Join(args, " ")))
			}
			refs, err := s3bParseAppArgs(args)
			if err != nil {
				return err
			}
			spec, err := parseImpactAt(flagAt)
			if err != nil {
				return err
			}
			window, err := s3bParseWindow("window", flagWindow)
			if err != nil {
				return err
			}
			scope, err := s3bStorefrontScope(cmd, flags, flagCountries)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			label := s3bRefLabel(refs[0])
			hint := fmt.Sprintf("capture data first, e.g. 'app-store-pp-cli app %s', 'app-store-pp-cli ratings %s', 'app-store-pp-cli top' and 'app-store-pp-cli reviews %s', on days before and after the event", label, label, label)
			st, ok, err := s3bOpenStore(cmd, flags, hint, func(note string) any {
				meta := newImpactMeta(refs[0], spec, window, flagWindow)
				meta.Note = impactCoMovementNote + " " + note
				return impactView{Meta: meta, Results: make([]impactRow, 0)}
			})
			if err != nil || !ok {
				return err
			}
			defer func() { _ = st.Close() }()

			view, err := buildImpact(ctx, st, refs[0], spec, window, flagWindow, scope, s3bNow())
			if err != nil {
				return s3bLocalReadErr(err)
			}
			if view.Meta.AppID != "" {
				s3bWarnStaleApps(ctx, cmd, flags, st, []string{view.Meta.AppID}, scope, fmt.Sprintf("app-store-pp-cli app %s", label))
			}
			return renderImpact(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagAt, "at", "latest", "Event to compare around: latest (newest stored release), version:<v>, listing (latest listing change), featured (latest featuring) or a date (2026-10-01)")
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Length of each window before and after the event (7d, 2w, 48h)")
	cmd.Flags().StringVar(&flagCountries, "countries", "", "Storefronts to compare (us,gb or all); default every storefront with stored data, or --country when set")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local snapshot store path (default: the CLI data directory)")
	return cmd
}

func newImpactMeta(ref appstore.AppRef, spec impactAtSpec, window time.Duration, windowRaw string) impactMeta {
	return impactMeta{
		App:                    s3bRefLabel(ref),
		AppID:                  ref.ID,
		At:                     spec.Raw,
		Window:                 strings.TrimSpace(windowRaw),
		WindowDays:             s3bRound2(window.Hours() / 24),
		Storefronts:            make([]string, 0),
		StorefrontsWithoutData: make([]string, 0),
		CoMovementOnly:         true,
		Direction: map[string]string{
			impactMetricRank:     "lower is better",
			impactMetricRatings:  "new ratings per day between the first and last capture day of each window",
			impactMetricReviews:  "mean stars (1-5) of stored reviews dated in each window",
			impactMetricKeywords: "lower is better; mean position over captures where the app was found",
		},
		Rule: fmt.Sprintf("a window with fewer than %d captures (capture days for ratings, reviews for stars) is insufficient_data and its delta is null", impactMinPoints),
	}
}

func buildImpact(ctx context.Context, st *store.Store, ref appstore.AppRef, spec impactAtSpec, window time.Duration, windowRaw string, scope s3bScope, now time.Time) (impactView, error) {
	view := impactView{Meta: newImpactMeta(ref, spec, window, windowRaw), Results: make([]impactRow, 0)}
	ids, _, err := s3bResolveLocalApps(ctx, st, []appstore.AppRef{ref})
	if err != nil {
		return view, err
	}
	if len(ids) == 0 {
		view.Meta.Note = impactCoMovementNote + fmt.Sprintf(" Bundle id %s is not in the local store; capture it first with 'app-store-pp-cli app %s'.", ref.BundleID, ref.BundleID)
		return view, nil
	}
	appID := ids[0]
	view.Meta.AppID = appID
	titles, err := s3bAppTitles(ctx, st)
	if err != nil {
		return view, err
	}
	view.Meta.Title = titles[appID]

	ev, at, hint, err := resolveImpactEvent(ctx, st, appID, spec, scope)
	if err != nil {
		return view, err
	}
	if ev == nil {
		view.Meta.Note = impactCoMovementNote + " " + hint
		return view, nil
	}
	view.Meta.Event = ev
	view.Meta.Before = &impactWindow{From: s3bFormat(at.Add(-window)), To: s3bFormat(at)}
	view.Meta.After = &impactWindow{From: s3bFormat(at), To: s3bFormat(at.Add(window))}
	complete := !at.Add(window).After(now)
	view.Meta.AfterComplete = &complete

	rows, sfs, err := impactRows(ctx, st, appID, at, window, scope)
	if err != nil {
		return view, err
	}
	view.Results = rows
	view.Meta.Storefronts = sfs
	have := map[string]bool{}
	for _, sf := range sfs {
		have[sf] = true
	}
	for _, sf := range scope.list() {
		if !have[sf] {
			view.Meta.StorefrontsWithoutData = append(view.Meta.StorefrontsWithoutData, sf)
		}
	}
	note := impactCoMovementNote
	if len(rows) == 0 {
		note += fmt.Sprintf(" No stored ratings, chart, review or tracked keyword captures for app %s in these storefronts, so there is nothing to compare.", appID)
	} else {
		note += fmt.Sprintf(" A window with fewer than %d captures is insufficient_data and has no delta.", impactMinPoints)
		if missing := s3bNoDataNote(view.Meta.StorefrontsWithoutData, nil); missing != "" {
			note += " " + missing
		}
	}
	if !complete {
		note += fmt.Sprintf(" The after window is still open (it ends %s); its values cover captures so far.", s3bFormat(at.Add(window)))
	}
	view.Meta.Note = note
	return view, nil
}

// resolveImpactEvent turns --at into one event time. A nil event comes with
// a hint saying what to capture or pass instead (not an error: the local
// store simply has no such event yet).
func resolveImpactEvent(ctx context.Context, st *store.Store, appID string, spec impactAtSpec, scope s3bScope) (*impactEvent, time.Time, string, error) {
	switch spec.Kind {
	case "date":
		return &impactEvent{Kind: "date", At: s3bFormat(spec.Date), ResolvedFrom: spec.Raw}, spec.Date, "", nil
	case "featured":
		return resolveFeaturedEvent(ctx, st, appID, spec, scope)
	}
	versions, snaps, err := s3bKnownVersions(ctx, st, appID)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	switch spec.Kind {
	case "latest":
		for _, v := range versions {
			if !v.ReleasedAt.IsZero() {
				return &impactEvent{Kind: "version", At: s3bFormat(v.ReleasedAt), Version: v.Version, Detail: "newest stored version release", ResolvedFrom: spec.Raw}, v.ReleasedAt, "", nil
			}
		}
		return nil, time.Time{}, fmt.Sprintf("No stored version release for app %s; capture it with 'app-store-pp-cli app %s' (or 'versions %s'), or pass --at with a date such as 2026-10-01.", appID, appID, appID), nil
	case "version":
		for _, v := range versions {
			if v.Version != spec.Version {
				continue
			}
			if !v.ReleasedAt.IsZero() {
				return &impactEvent{Kind: "version", At: s3bFormat(v.ReleasedAt), Version: v.Version, Detail: "stored release date", ResolvedFrom: spec.Raw}, v.ReleasedAt, "", nil
			}
			if !v.FirstSeen.IsZero() {
				return &impactEvent{Kind: "version", At: s3bFormat(v.FirstSeen), Version: v.Version, Detail: "no release date stored; using the first capture that showed this version", ResolvedFrom: spec.Raw}, v.FirstSeen, "", nil
			}
		}
		known := make([]string, 0, 10)
		for i, v := range versions {
			if i == 10 {
				break
			}
			known = append(known, v.Version)
		}
		if len(known) == 0 {
			return nil, time.Time{}, fmt.Sprintf("Version %s is not stored for app %s and no versions are stored yet; capture them with 'app-store-pp-cli versions %s'.", spec.Version, appID, appID), nil
		}
		return nil, time.Time{}, fmt.Sprintf("Version %s is not in the stored history of app %s; stored versions (newest first): %s.", spec.Version, appID, strings.Join(known, ", ")), nil
	case "listing":
		changes := s3bListingChanges(snaps, scope)
		if len(changes) == 0 {
			return nil, time.Time{}, fmt.Sprintf("No listing change stored for app %s: it needs two snapshots of the same kind in one storefront whose content differs; capture again later with 'app-store-pp-cli app %s'.", appID, appID), nil
		}
		last := changes[len(changes)-1]
		return &impactEvent{
			Kind:          "listing",
			At:            s3bFormat(last.Cur.CapturedAt),
			Storefront:    last.Storefront,
			ChangedFields: last.Fields,
			Detail:        fmt.Sprintf("%s listing content changed between captures at %s and %s", last.Source, s3bFormat(last.Prev.CapturedAt), s3bFormat(last.Cur.CapturedAt)),
			ResolvedFrom:  spec.Raw,
		}, last.Cur.CapturedAt, "", nil
	}
	return nil, time.Time{}, "", fmt.Errorf("unknown --at kind %q", spec.Kind)
}

// resolveFeaturedEvent finds the first capture of the app's most recent run
// of consecutive featured captures (the featuring date is the capture date).
func resolveFeaturedEvent(ctx context.Context, st *store.Store, appID string, spec impactAtSpec, scope s3bScope) (*impactEvent, time.Time, string, error) {
	rows, err := store.FeaturedSince(ctx, st, "", time.Time{})
	if err != nil {
		return nil, time.Time{}, "", err
	}
	var best *impactEvent
	var bestAt time.Time
	present := map[string]bool{}
	for _, c := range s3bFeaturedCaptures(rows) {
		if !scope.has(c.Storefront) {
			continue
		}
		key := c.Storefront + "|" + c.Platform
		stories, on := c.Apps[appID]
		if on && !present[key] && (best == nil || c.At.After(bestAt)) {
			best = &impactEvent{Kind: "featured", At: s3bFormat(c.At), Storefront: c.Storefront, Detail: "featured in: " + strings.Join(stories, "; "), ResolvedFrom: spec.Raw}
			bestAt = c.At
		}
		present[key] = on
	}
	if best == nil {
		return nil, time.Time{}, fmt.Sprintf("App %s has no stored featuring; capture the Today tab with 'app-store-pp-cli featured' (the capture date is the featuring date), or pass --at with a date.", appID), nil
	}
	return best, bestAt, "", nil
}

// impactRatingSeries groups the stored rating counts like
// s3bRatingsByStorefront, but picks each storefront's source by coverage
// inside the two windows, the only counts the metric reads: the most capture
// days in the thinner window, then in both windows, then lookup over page. A
// long older series from one source cannot hide the captures made around the
// event through another.
func impactRatingSeries(all []store.RatingsSnapshot, scope s3bScope, inBefore, inAfter func(time.Time) bool) map[string]s3bRatingSeries {
	bySF := map[string]map[string][]store.RatingsSnapshot{}
	for _, r := range all {
		if !scope.has(r.Storefront) || r.ObservedAt().IsZero() {
			continue
		}
		if bySF[r.Storefront] == nil {
			bySF[r.Storefront] = map[string][]store.RatingsSnapshot{}
		}
		bySF[r.Storefront][r.Source] = append(bySF[r.Storefront][r.Source], r)
	}
	out := map[string]s3bRatingSeries{}
	for sf, bySource := range bySF {
		srcs := make([]string, 0, len(bySource))
		for src := range bySource {
			srcs = append(srcs, src)
		}
		sort.Strings(srcs)
		best, bestScore := "", []int(nil)
		for _, src := range srcs {
			before, after := map[time.Time]bool{}, map[time.Time]bool{}
			for _, s := range bySource[src] {
				switch at := s.ObservedAt(); {
				case inBefore(at):
					before[s3bDayStart(at)] = true
				case inAfter(at):
					after[s3bDayStart(at)] = true
				}
			}
			score := []int{min(len(before), len(after)), len(before) + len(after), -s3bRatingSourceRank(src)}
			if bestScore == nil || slices.Compare(score, bestScore) > 0 {
				best, bestScore = src, score
			}
		}
		snaps := bySource[best]
		sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].ObservedAt().Before(snaps[j].ObservedAt()) })
		out[sf] = s3bRatingSeries{Source: best, Snaps: snaps}
	}
	return out
}

// impactRows computes every metric row per storefront (storefront asc,
// metric order, series asc) and the storefronts that had data.
func impactRows(ctx context.Context, st *store.Store, appID string, at time.Time, window time.Duration, scope s3bScope) ([]impactRow, []string, error) {
	bFrom, aTo := at.Add(-window), at.Add(window)
	inBefore := func(t time.Time) bool { return !t.Before(bFrom) && t.Before(at) }
	inAfter := func(t time.Time) bool { return !t.Before(at) && t.Before(aTo) }
	sfs := map[string]bool{}

	ratings, err := store.RatingsSeries(ctx, st, appID, "", "")
	if err != nil {
		return nil, nil, err
	}
	series := impactRatingSeries(ratings, scope, inBefore, inAfter)
	for sf := range series {
		sfs[sf] = true
	}

	type chartSeries struct {
		key   store.ChartKey
		ranks []store.ChartRowRecord
	}
	chartRows, err := store.ChartRows(ctx, st, store.ChartQuery{AppID: appID, Storefronts: scope.list(), AgeBand: store.AnyAgeBand})
	if err != nil {
		return nil, nil, err
	}
	charts := map[string]map[string]*chartSeries{}
	for _, r := range chartRows {
		sfs[r.Storefront] = true
		name := s3bChartSeriesName(r.Platform, r.GenreID, r.AgeBand, r.Chart)
		if charts[r.Storefront] == nil {
			charts[r.Storefront] = map[string]*chartSeries{}
		}
		cs := charts[r.Storefront][name]
		if cs == nil {
			cs = &chartSeries{key: store.ChartKey{Storefront: r.Storefront, Platform: r.Platform, GenreID: r.GenreID, AgeBand: r.AgeBand, Chart: r.Chart}}
			charts[r.Storefront][name] = cs
		}
		cs.ranks = append(cs.ranks, r)
	}

	reviews, err := store.Reviews(ctx, st, store.ReviewFilter{AppID: appID, Storefronts: scope.list(), Since: bFrom, Until: aTo})
	if err != nil {
		return nil, nil, err
	}
	type starSums struct{ sumB, nB, sumA, nA int }
	stars := map[string]*starSums{}
	for _, r := range reviews {
		t, ok := s3bParseStoredTime(r.ReviewedAt)
		if !ok || r.Rating < 1 || r.Rating > 5 || (!inBefore(t) && !inAfter(t)) {
			continue
		}
		sfs[r.Storefront] = true
		s := stars[r.Storefront]
		if s == nil {
			s = &starSums{}
			stars[r.Storefront] = s
		}
		if inBefore(t) {
			s.sumB += r.Rating
			s.nB++
		} else {
			s.sumA += r.Rating
			s.nA++
		}
	}

	kws, err := store.KeywordSeries(ctx, st, store.KeywordFilter{AppID: appID})
	if err != nil {
		return nil, nil, err
	}
	keywords := map[string]map[string][]store.KeywordRank{}
	for _, k := range kws {
		if !scope.has(k.Storefront) {
			continue
		}
		sfs[k.Storefront] = true
		name := impactKeywordSeries(k)
		if keywords[k.Storefront] == nil {
			keywords[k.Storefront] = map[string][]store.KeywordRank{}
		}
		keywords[k.Storefront][name] = append(keywords[k.Storefront][name], k)
	}

	rows := make([]impactRow, 0)
	for _, sf := range s3bSortedKeys(sfs) {
		// Chart rank: one row per chart the app has appeared in.
		if len(charts[sf]) == 0 {
			rows = append(rows, impactPlaceholder(sf, impactMetricRank, "no stored chart capture includes this app in "+sf))
		}
		for _, name := range s3bSortedKeys(charts[sf]) {
			cs := charts[sf][name]
			caps, err := store.ChartCaptures(ctx, st, cs.key)
			if err != nil {
				return nil, nil, err
			}
			row := impactRow{Storefront: sf, Metric: impactMetricRank, Series: name}
			for _, c := range caps {
				if inBefore(c.CapturedAt) {
					row.PointsBefore++
				} else if inAfter(c.CapturedAt) {
					row.PointsAfter++
				}
			}
			bestB, bestA := 0, 0
			chartedB, chartedA := map[time.Time]bool{}, map[time.Time]bool{}
			for _, r := range cs.ranks {
				switch {
				case inBefore(r.CapturedAt):
					chartedB[r.CapturedAt] = true
					if bestB == 0 || r.Rank < bestB {
						bestB = r.Rank
					}
				case inAfter(r.CapturedAt):
					chartedA[r.CapturedAt] = true
					if bestA == 0 || r.Rank < bestA {
						bestA = r.Rank
					}
				}
			}
			if bestB > 0 {
				row.Before = s3bFloatPtr(float64(bestB))
			}
			if bestA > 0 {
				row.After = s3bFloatPtr(float64(bestA))
			}
			row.Detail = fmt.Sprintf("charted in %d of %d captures before and %d of %d after", len(chartedB), row.PointsBefore, len(chartedA), row.PointsAfter)
			row.settle()
			rows = append(rows, row)
		}

		// New ratings per day from the storefront's rating-count series,
		// placed in the windows by when Apple produced the counts (a Lookup
		// CDN copy can be hours older than its capture).
		if s, ok := series[sf]; ok {
			var snapsB, snapsA []store.RatingsSnapshot
			for _, r := range s.Snaps {
				if inBefore(r.ObservedAt()) {
					snapsB = append(snapsB, r)
				} else if inAfter(r.ObservedAt()) {
					snapsA = append(snapsA, r)
				}
			}
			daysB, daysA := s3bDailyCounts(snapsB), s3bDailyCounts(snapsA)
			row := impactRow{Storefront: sf, Metric: impactMetricRatings, Series: s.Source, PointsBefore: len(daysB), PointsAfter: len(daysA)}
			var resetB, resetA bool
			row.Before, resetB = impactRatingRate(daysB)
			row.After, resetA = impactRatingRate(daysA)
			row.Detail = fmt.Sprintf("%s rating counts on %d capture days before and %d after", s.Source, len(daysB), len(daysA))
			if resetB || resetA {
				row.Detail += "; a rating reset (drop of more than 1%) was detected and its post-reset count counted as new"
			}
			row.settle()
			rows = append(rows, row)
		} else {
			rows = append(rows, impactPlaceholder(sf, impactMetricRatings, "no stored rating counts for this app in "+sf))
		}

		// Average stars of stored reviews dated in each window.
		row := impactRow{Storefront: sf, Metric: impactMetricReviews, Series: "stored reviews"}
		if s := stars[sf]; s != nil {
			row.PointsBefore, row.PointsAfter = s.nB, s.nA
			if s.nB > 0 {
				row.Before = s3bFloatPtr(float64(s.sumB) / float64(s.nB))
			}
			if s.nA > 0 {
				row.After = s3bFloatPtr(float64(s.sumA) / float64(s.nA))
			}
		}
		row.Detail = fmt.Sprintf("%d stored reviews dated before and %d after", row.PointsBefore, row.PointsAfter)
		row.settle()
		rows = append(rows, row)

		// Tracked keyword positions (keyword-rank captures targeting the app).
		if len(keywords[sf]) == 0 {
			rows = append(rows, impactPlaceholder(sf, impactMetricKeywords, "no tracked keyword captures for this app in "+sf+" (capture the terms you track with 'keyword-rank <term> --app "+appID+" --country "+sf+"')"))
		}
		for _, name := range s3bSortedKeys(keywords[sf]) {
			row := impactRow{Storefront: sf, Metric: impactMetricKeywords, Series: name}
			var sumB, sumA, foundB, foundA int
			for _, k := range keywords[sf][name] {
				switch {
				case inBefore(k.CapturedAt):
					row.PointsBefore++
					if k.Rank > 0 {
						sumB += k.Rank
						foundB++
					}
				case inAfter(k.CapturedAt):
					row.PointsAfter++
					if k.Rank > 0 {
						sumA += k.Rank
						foundA++
					}
				}
			}
			if foundB > 0 {
				row.Before = s3bFloatPtr(float64(sumB) / float64(foundB))
			}
			if foundA > 0 {
				row.After = s3bFloatPtr(float64(sumA) / float64(foundA))
			}
			row.Detail = fmt.Sprintf("found in %d of %d captures before and %d of %d after", foundB, row.PointsBefore, foundA, row.PointsAfter)
			row.settle()
			rows = append(rows, row)
		}
	}
	return rows, s3bSortedKeys(sfs), nil
}

func impactPlaceholder(sf, metric, detail string) impactRow {
	row := impactRow{Storefront: sf, Metric: metric, Detail: detail}
	row.settle()
	return row
}

// impactRatingRate is new ratings per day between the first and last capture
// day of a window (nil with fewer than two days), and whether a reset was seen.
func impactRatingRate(days []s3bDayCount) (*float64, bool) {
	if len(days) < 2 {
		return nil, false
	}
	var total int64
	reset := false
	for _, inc := range s3bRatingIncrements(days) {
		total += inc.New
		reset = reset || inc.Reset
	}
	span := days[len(days)-1].Day.Sub(days[0].Day).Hours() / 24
	if span <= 0 {
		return nil, reset
	}
	return s3bFloatPtr(float64(total) / span), reset
}

// impactKeywordSeries names a tracked-keyword series: the term, qualified by
// source and platform when stored.
func impactKeywordSeries(k store.KeywordRank) string {
	quals := make([]string, 0, 2)
	for _, q := range []string{k.Source, k.Platform} {
		if strings.TrimSpace(q) != "" {
			quals = append(quals, q)
		}
	}
	if len(quals) == 0 {
		return k.Term
	}
	return k.Term + " (" + strings.Join(quals, ", ") + ")"
}

func renderImpact(cmd *cobra.Command, flags *rootFlags, view impactView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if view.Meta.Event != nil {
		ev := view.Meta.Event
		what := ev.Kind
		if ev.Version != "" {
			what += " " + ev.Version
		}
		_, _ = fmt.Fprintf(w, "Event: %s at %s; windows of %s before and after\n\n", what, ev.At, view.Meta.Window)
	}
	if len(view.Results) == 0 {
		_, _ = fmt.Fprintln(w, view.Meta.Note)
		return nil
	}
	rows := make([][]string, 0, len(view.Results))
	for _, r := range view.Results {
		flag := ""
		if r.InsufficientData {
			flag = "insufficient data"
		}
		rows = append(rows, []string{r.Storefront, r.Metric, r.Series, s3bOptFloat(r.Before), s3bOptFloat(r.After), s3bOptFloat(r.Delta),
			fmt.Sprintf("%d/%d", r.PointsBefore, r.PointsAfter), flag})
	}
	if err := s3bPrintTable(w, []string{"storefront", "metric", "series", "before", "after", "delta", "points", ""}, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(w, "\n"+view.Meta.Note)
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelImpactCmd(flags))
	})
}
