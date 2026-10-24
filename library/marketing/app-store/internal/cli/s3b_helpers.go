// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the slice-3b local analytics commands (traction, impact,
// changes, keyword-share, breakouts). Everything here reads the local
// snapshot store through the typed store accessors; nothing touches the
// network, so none of these commands can be throttled by Apple.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// s3bNow is the clock the local analytics commands read; tests pin it.
var s3bNow = func() time.Time { return time.Now().UTC() }

// s3bOpenStore runs the missing-mirror guard, then opens the local store.
// With no store file it prints a capture hint to stderr and, for machine
// output, the command's empty view (which carries the same note), and
// reports ok=false with a nil error: an empty local store is an ordinary
// exit-0 state, not a failure.
func s3bOpenStore(cmd *cobra.Command, flags *rootFlags, hint string, empty func(note string) any) (*store.Store, bool, error) {
	path, exists := appStoreMirrorExists(cmd)
	if !exists {
		note := fmt.Sprintf("No local App Store snapshot store at %s yet; %s.", path, hint)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s\n", path, hint)
		if w := cmd.OutOrStdout(); !wantsHumanTable(w, flags) {
			return nil, false, printJSONFiltered(w, empty(note), flags)
		}
		return nil, false, nil
	}
	st, err := openAppStoreStoreRead(cmd, flags)
	if err != nil {
		return nil, false, err
	}
	return st, true, nil
}

// s3bParseSince parses a --since style value: a window back from now (7d,
// 4w, 36h), a date (2026-10-01, midnight UTC) or an RFC3339 time. "" and
// "all" mean no lower bound (the zero time). Bad input is a usage error.
func s3bParseSince(flag, raw string, now time.Time) (time.Time, error) {
	v := strings.TrimSpace(raw)
	if v == "" || strings.EqualFold(v, "all") {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC(), nil
	}
	d, err := cliutil.ParseDurationLoose(v)
	if err != nil {
		return time.Time{}, usageErr(fmt.Errorf("--%s %q: use a window such as 7d, 4w or 36h, or a date such as 2026-10-01", flag, raw))
	}
	if d <= 0 {
		return time.Time{}, usageErr(fmt.Errorf("--%s %q: the window must be positive", flag, raw))
	}
	return now.Add(-d).UTC().Truncate(time.Second), nil
}

// s3bParseWindow parses a positive window length such as 7d, 2w or 48h.
func s3bParseWindow(flag, raw string) (time.Duration, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, usageErr(fmt.Errorf("--%s is empty; use a window such as 7d, 2w or 48h", flag))
	}
	d, err := cliutil.ParseDurationLoose(v)
	if err != nil {
		return 0, usageErr(fmt.Errorf("--%s %q: use a window such as 7d, 2w or 48h", flag, raw))
	}
	if d <= 0 {
		return 0, usageErr(fmt.Errorf("--%s %q: the window must be positive", flag, raw))
	}
	if d > 400*24*time.Hour {
		return 0, usageErr(fmt.Errorf("--%s %q: the window must be at most 400d", flag, raw))
	}
	return d, nil
}

// s3bScope is a storefront filter; nil means every storefront with data.
type s3bScope map[string]bool

func (s s3bScope) has(storefront string) bool { return s == nil || s[storefront] }

// list returns the scope as a sorted slice (nil for "every storefront").
func (s s3bScope) list() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// s3bStorefrontScope resolves which storefronts a local command reads:
// --countries when given (a code list), else --country when it was set
// explicitly, else nil (every storefront that has stored data). "--countries
// all" also means every storefront with stored data.
func s3bStorefrontScope(cmd *cobra.Command, flags *rootFlags, countriesRaw string) (s3bScope, error) {
	var codes []string
	switch {
	case strings.EqualFold(strings.TrimSpace(countriesRaw), "all"):
		return nil, nil
	case strings.TrimSpace(countriesRaw) != "":
		list, err := appstore.ParseCountries(countriesRaw)
		if err != nil {
			return nil, usageErr(fmt.Errorf("--countries: %w", err))
		}
		codes = list
	case flagChanged(cmd, "country"):
		cc, err := appStoreCountry(flags)
		if err != nil {
			return nil, err
		}
		codes = []string{cc}
	default:
		return nil, nil
	}
	scope := s3bScope{}
	for _, c := range codes {
		scope[c] = true
	}
	return scope, nil
}

// s3bParseAppArgs validates app arguments (numeric id, id123, store URL or
// bundle id) before any store access and drops duplicates. Bad input is a
// usage error (exit 2).
func s3bParseAppArgs(args []string) ([]appstore.AppRef, error) {
	refs := make([]appstore.AppRef, 0, len(args))
	seen := map[string]bool{}
	for _, a := range args {
		ref, err := parseAppRef(a)
		if err != nil {
			return nil, err
		}
		key := ref.ID
		if key == "" {
			key = "bundle:" + strings.ToLower(ref.BundleID)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, ref)
	}
	return refs, nil
}

// s3bRefLabel renders an app ref as given (id or bundle id) for meta and hints.
func s3bRefLabel(ref appstore.AppRef) string {
	if ref.ID != "" {
		return ref.ID
	}
	return ref.BundleID
}

// s3bNoDataNote names the requested apps and storefronts that have no stored
// data, so a table that leaves them out says so ("" when every one has data).
func s3bNoDataNote(storefronts, apps []string) string {
	parts := make([]string, 0, 2)
	if len(apps) > 0 {
		parts = append(parts, "app(s) "+strings.Join(apps, ", "))
	}
	if len(storefronts) > 0 {
		parts = append(parts, "storefront(s) "+strings.Join(storefronts, ", "))
	}
	if len(parts) == 0 {
		return ""
	}
	return "No stored data for " + strings.Join(parts, "; ") + "."
}

// s3bResolveLocalApps maps app refs to numeric ids using only the store:
// ids pass through; a bundle id resolves through stored listing snapshots
// (store.AppIDForBundle). Bundle ids the store has never seen come back in
// missing (the store just has no data for them; that is not an error).
func s3bResolveLocalApps(ctx context.Context, st *store.Store, refs []appstore.AppRef) (ids, missing []string, err error) {
	ids = make([]string, 0, len(refs))
	missing = make([]string, 0)
	seen := map[string]bool{}
	for _, ref := range refs {
		id := ref.ID
		if id == "" {
			if id, err = store.AppIDForBundle(ctx, st, ref.BundleID); err != nil {
				return nil, nil, err
			}
			if id == "" {
				missing = append(missing, ref.BundleID)
				continue
			}
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, missing, nil
}

// s3bAppTitles maps app ids to their most recently stored listing title.
func s3bAppTitles(ctx context.Context, st *store.Store) (map[string]string, error) {
	tracked, err := store.TrackedApps(ctx, st, time.Time{})
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	latest := map[string]time.Time{}
	for _, t := range tracked {
		if strings.TrimSpace(t.Title) == "" {
			continue
		}
		if prev, ok := latest[t.AppID]; !ok || t.Last.After(prev) {
			latest[t.AppID] = t.Last
			out[t.AppID] = t.Title
		}
	}
	return out, nil
}

// --- ratings ---------------------------------------------------------------

// s3bRatingSeries is one storefront's rating-count series from one source.
type s3bRatingSeries struct {
	Source string
	Snaps  []store.RatingsSnapshot // oldest first
}

// s3bRatingsByStorefront keeps one rating source per storefront so counts
// are compared like with like: "lookup" (Lookup userRatingCount, exact)
// when the storefront has lookup rows, otherwise "page", otherwise the
// storefront's only other source. Rows observed before since (when set) are
// dropped. Each series is ordered by when Apple produced the counts
// (RatingsSnapshot.ObservedAt): a Lookup CDN copy can be up to 24 h older
// than its capture, so two captures minutes apart can hold counts hours
// apart, and a stale copy captured after a fresh one must not read as a
// drop.
func s3bRatingsByStorefront(all []store.RatingsSnapshot, scope s3bScope, since time.Time) map[string]s3bRatingSeries {
	bySF := map[string]map[string][]store.RatingsSnapshot{}
	for _, r := range all {
		if !scope.has(r.Storefront) || r.ObservedAt().IsZero() {
			continue
		}
		if !since.IsZero() && r.ObservedAt().Before(since) {
			continue
		}
		if bySF[r.Storefront] == nil {
			bySF[r.Storefront] = map[string][]store.RatingsSnapshot{}
		}
		bySF[r.Storefront][r.Source] = append(bySF[r.Storefront][r.Source], r)
	}
	out := map[string]s3bRatingSeries{}
	for sf, bySource := range bySF {
		src := s3bPickRatingSource(bySource)
		snaps := bySource[src]
		sort.SliceStable(snaps, func(i, j int) bool { return snaps[i].ObservedAt().Before(snaps[j].ObservedAt()) })
		out[sf] = s3bRatingSeries{Source: src, Snaps: snaps}
	}
	return out
}

// s3bPickRatingSource picks the source series with the most distinct capture
// days (UTC, by observation time), so one stray Lookup row cannot hide a dense
// page series. Ties prefer lookup (exact counts), then page.
func s3bPickRatingSource(bySource map[string][]store.RatingsSnapshot) string {
	keys := make([]string, 0, len(bySource))
	for k := range bySource {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	best, bestDays := "", -1
	for _, k := range keys {
		days := map[time.Time]bool{}
		for _, s := range bySource[k] {
			if at := s.ObservedAt(); !at.IsZero() {
				days[s3bDayStart(at)] = true
			}
		}
		if n := len(days); n > bestDays || (n == bestDays && s3bRatingSourceRank(k) < s3bRatingSourceRank(best)) {
			best, bestDays = k, n
		}
	}
	return best
}

// s3bRatingSourceRank orders rating sources on a tie: lookup (exact
// counts), then page, then anything else.
func s3bRatingSourceRank(src string) int {
	switch src {
	case store.SourceLookup:
		return 0
	case store.SourcePage:
		return 1
	}
	return 2
}

// s3bDayCount is a storefront's rating count at the end of one UTC capture
// day (the last capture of that day).
type s3bDayCount struct {
	Day   time.Time // 00:00 UTC
	At    time.Time // the capture used
	Count int64
}

// s3bDailyCounts collapses a series ordered by observation time to one
// count per UTC day: the count with the newest data that day. Days and
// order follow when Apple produced the counts (ObservedAt: the data time
// when known, else the capture time), since Lookup counts are CDN-cached
// for up to 24 h; a day is the finest granularity the data supports.
func s3bDailyCounts(snaps []store.RatingsSnapshot) []s3bDayCount {
	out := make([]s3bDayCount, 0, len(snaps))
	for _, s := range snaps {
		at := s.ObservedAt()
		if at.IsZero() {
			continue
		}
		day := s3bDayStart(at)
		if n := len(out); n > 0 && out[n-1].Day.Equal(day) {
			if !at.Before(out[n-1].At) {
				out[n-1].At = at
				out[n-1].Count = s.RatingCount
			}
			continue
		}
		out = append(out, s3bDayCount{Day: day, At: at, Count: s.RatingCount})
	}
	return out
}

// s3bResetThreshold: a day-over-day drop larger than this share of the
// previous count is a rating reset, not noise.
const s3bResetThreshold = 0.01

// s3bIncrement is the new-ratings estimate credited to one capture day: the
// change since the previous capture day, never negative.
type s3bIncrement struct {
	Day     time.Time
	PrevDay time.Time
	New     int64
	Reset   bool
}

// s3bRatingIncrements turns daily counts into non-negative increments. A
// rise counts in full. A drop of more than 1% is a reset (Apple restarted
// the count, e.g. for a new version): the post-reset count is all new since
// the reset, so it is counted from the reset day forward. A smaller drop is
// removed ratings and counts as 0.
func s3bRatingIncrements(days []s3bDayCount) []s3bIncrement {
	out := make([]s3bIncrement, 0, len(days))
	for i := 1; i < len(days); i++ {
		prev, cur := days[i-1].Count, days[i].Count
		inc := s3bIncrement{Day: days[i].Day, PrevDay: days[i-1].Day}
		switch {
		case cur >= prev:
			inc.New = cur - prev
		case prev > 0 && float64(prev-cur)/float64(prev) > s3bResetThreshold:
			inc.New = cur
			if inc.New < 0 {
				inc.New = 0
			}
			inc.Reset = true
		default:
			inc.New = 0
		}
		out = append(out, inc)
	}
	return out
}

// --- time and periods ------------------------------------------------------

func s3bDayStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func s3bDayKey(t time.Time) string { return t.UTC().Format("2006-01-02") }

// Rollup periods for traction.
const (
	s3bPeriodDaily   = "daily"
	s3bPeriodMonthly = "monthly"
	s3bPeriodAnnual  = "annual"
	s3bPeriodAll     = "all-time"
)

// s3bParsePeriod validates --period (aliases accepted).
func s3bParsePeriod(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "daily", "day", "days":
		return s3bPeriodDaily, nil
	case "", "monthly", "month", "months":
		return s3bPeriodMonthly, nil
	case "annual", "annually", "yearly", "year", "years":
		return s3bPeriodAnnual, nil
	case "all-time", "alltime", "all", "total":
		return s3bPeriodAll, nil
	}
	return "", usageErr(fmt.Errorf("--period %q: use daily, monthly, annual or all-time", raw))
}

// s3bBucket returns the period label a time falls in.
func s3bBucket(t time.Time, period string) string {
	u := t.UTC()
	switch period {
	case s3bPeriodDaily:
		return u.Format("2006-01-02")
	case s3bPeriodMonthly:
		return u.Format("2006-01")
	case s3bPeriodAnnual:
		return u.Format("2006")
	}
	return s3bPeriodAll
}

// s3bParseStoredTime parses a stored RFC3339 time or a bare date.
func s3bParseStoredTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), true
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func s3bFormat(t time.Time) string { return appstore.FormatTime(t) }

// s3bTimePtr formats t, or returns nil for the zero time (JSON null).
func s3bTimePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := appstore.FormatTime(t)
	return &s
}

// s3bMinTime returns the earlier non-zero time.
func s3bMinTime(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case b.Before(a):
		return b
	}
	return a
}

// --- charts ----------------------------------------------------------------

// s3bChartSeriesName renders a chart key as platform/genre/chart, with a
// fourth segment kids-<ageBandId> for a Kids chart (genre 36 plus an age
// band), which is a different chart from the overall genre-36 one. The first
// three segments always name platform, genre and chart.
func s3bChartSeriesName(platform, genreID, ageBand, chart string) string {
	name := platform + "/" + genreID + "/" + chart
	if ageBand != "" {
		name += "/kids-" + ageBand
	}
	return name
}

// --- featured ----------------------------------------------------------------

// s3bFeaturedCapture is one Today-tab capture of one storefront and platform:
// the apps featured in it and the story titles that featured them.
type s3bFeaturedCapture struct {
	Storefront string
	Platform   string
	At         time.Time
	Apps       map[string][]string
}

// s3bFeaturedCaptures groups stored featured rows into captures, ordered by
// storefront, platform and time. Every stored story row (including stories
// without apps) marks its capture, so captures where an app was absent are
// visible.
func s3bFeaturedCaptures(rows []store.FeaturedRow) []s3bFeaturedCapture {
	type key struct {
		sf, platform string
		at           time.Time
	}
	index := map[key]int{}
	out := make([]s3bFeaturedCapture, 0)
	for _, r := range rows {
		k := key{r.Storefront, r.Platform, r.CapturedAt}
		i, ok := index[k]
		if !ok {
			i = len(out)
			index[k] = i
			out = append(out, s3bFeaturedCapture{Storefront: r.Storefront, Platform: r.Platform, At: r.CapturedAt, Apps: map[string][]string{}})
		}
		if r.AppID != "" {
			out[i].Apps[r.AppID] = append(out[i].Apps[r.AppID], r.StoryTitle)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Storefront != out[j].Storefront {
			return out[i].Storefront < out[j].Storefront
		}
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		return out[i].At.Before(out[j].At)
	})
	return out
}

// --- numbers and text --------------------------------------------------------

func s3bRound2(v float64) float64 { return math.Round(v*100) / 100 }

func s3bFloatPtr(v float64) *float64 {
	r := s3bRound2(v)
	return &r
}

// s3bMedian returns the median of vals (0 for none).
func s3bMedian(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	c := append([]float64(nil), vals...)
	sort.Float64s(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// s3bTruncate shortens s to at most n runes, marking the cut with an ellipsis.
func s3bTruncate(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

func s3bOptInt(p *int64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprint(*p)
}

func s3bOptRank(p *int) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprint(*p)
}

func s3bOptFloat(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%g", *p)
}

// s3bPrintTable writes a small aligned table for human output.
func s3bPrintTable(w io.Writer, headers []string, rows [][]string) error {
	tw := newTabWriter(w)
	hs := make([]string, len(headers))
	for i, h := range headers {
		hs[i] = bold(strings.ToUpper(h))
	}
	_, _ = fmt.Fprintln(tw, strings.Join(hs, "\t"))
	for _, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = cliutil.ScrubTerminal(c)
		}
		_, _ = fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// s3bSortedKeys returns a map's keys sorted.
func s3bSortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// s3bWarnStaleApps prints the --max-age staleness hint for local analytics
// over apps: the newest observation of those apps (every app when ids is
// empty) in the storefront scope.
func s3bWarnStaleApps(ctx context.Context, cmd *cobra.Command, flags *rootFlags, st *store.Store, ids []string, scope s3bScope, refresh string) {
	if flags == nil || flags.maxAge <= 0 {
		return
	}
	newest, err := store.NewestAppObservation(ctx, st, ids, scope.list())
	if err != nil {
		return // the hint is advisory; the result itself already printed or will
	}
	warnStale(cmd, flags, newest, s3bNow(), "capture of these apps", refresh)
}

// s3bNewestOf returns the newest of RFC3339 timestamps (zero for none).
func s3bNewestOf(stamps ...string) time.Time {
	var newest time.Time
	for _, s := range stamps {
		if t, ok := s3bParseStoredTime(s); ok && t.After(newest) {
			newest = t
		}
	}
	return newest
}

// s3bLocalReadErr types a local-store read failure as an API error (exit 5),
// as the s3a readers do, instead of the undocumented exit 1; typed errors
// pass through.
func s3bLocalReadErr(err error) error {
	var typed *cliError
	if err == nil || errors.As(err, &typed) {
		return err
	}
	return apiErr(err)
}
