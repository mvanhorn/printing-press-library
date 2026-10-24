// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the slice-3a local history commands (movers,
// rank-history, keyword-history, watch-listing, review-digest and search).
// These commands read only the local snapshot store that the live commands
// write; none of them makes a network request.

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// s3aChartSpec is a resolved chart key plus the genre's display name.
type s3aChartSpec struct {
	Key       store.ChartKey
	GenreName string
}

// label renders the key as storefront/platform/genre[ (Kids band)]/chart (source).
func (s s3aChartSpec) label() string {
	src := s.Key.Source
	if src == "" {
		src = "any source"
	}
	genre := s.Key.GenreID
	if s.GenreName != "" {
		genre += " " + s.GenreName
	}
	if band := ageBandLabel(s.Key.AgeBand); band != "" {
		genre += " (" + band + ")"
	}
	return fmt.Sprintf("%s %s %s %s (%s)", s.Key.Storefront, s.Key.Platform, genre, s.Key.Chart, src)
}

// topHint is the live command that captures this chart.
func (s s3aChartSpec) topHint() string {
	hint := fmt.Sprintf("%s top --chart %s --genre %s --platform %s --country %s", appStoreCLIName, s.Key.Chart, s.Key.GenreID, s.Key.Platform, s.Key.Storefront)
	if s.Key.AgeBand != "" {
		hint += " --age-band " + s.Key.AgeBand
	}
	return hint
}

// s3aResolveChart validates the chart flags shared by movers and
// rank-history: --chart, --genre, --platform, --age-band and --source, plus
// the root --country storefront. An unset --age-band is the overall chart
// (store age band ""); a band selects that Kids chart's own series.
func s3aResolveChart(flags *rootFlags, chart, genre, platform, ageBand, source string) (s3aChartSpec, error) {
	cc, err := appStoreCountry(flags)
	if err != nil {
		return s3aChartSpec{}, err
	}
	ch, err := appstore.NormalizeChart(chart)
	if err != nil {
		return s3aChartSpec{}, usageErr(fmt.Errorf("--chart: %w", err))
	}
	g, err := resolveGenre(genre)
	if err != nil {
		return s3aChartSpec{}, err
	}
	p, err := parseChartPlatform(platform)
	if err != nil {
		return s3aChartSpec{}, err
	}
	src, err := s3aChartSource(source)
	if err != nil {
		return s3aChartSpec{}, err
	}
	band, err := resolveAgeBand(ageBand)
	if err != nil {
		return s3aChartSpec{}, err
	}
	if err := checkChartAgeBand(band, g, p, ch); err != nil {
		return s3aChartSpec{}, err
	}
	return s3aChartSpec{
		Key:       store.ChartKey{Storefront: cc, Platform: p, GenreID: g.ID, AgeBand: band.ID, Chart: ch, Source: src},
		GenreName: g.Name,
	}, nil
}

// s3aChartSource maps --source for chart history: "" (any), web (the chart
// page, 200 ranks) or rss (the legacy feed, 100 ranks).
func s3aChartSource(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "any", "all":
		return "", nil
	case "web", "page":
		return store.SourceWebChart, nil
	case "rss", "feed":
		return store.SourceRSS, nil
	}
	return "", usageErr(fmt.Errorf("--source: unknown chart source %q; use web (chart page, 200 ranks) or rss (legacy feed, 100 ranks)", s))
}

// s3aLocalApp parses the app argument of a local history command: a numeric
// id, id123, a store URL or a bundle id. It returns the ref, the label to
// show until the ref is resolved (the id, or the bundle id as given) and the
// storefront: the URL's storefront when --country was not set, else
// --country. Local history is keyed by the numeric id, so a bundle id is
// resolved against the store once it is open (s3aResolveLocalApp).
func s3aLocalApp(cmd *cobra.Command, flags *rootFlags, arg string) (appstore.AppRef, string, string, error) {
	ref, err := parseAppRef(arg)
	if err != nil {
		return ref, "", "", err
	}
	cc, err := refCountry(cmd, flags, ref)
	if err != nil {
		return ref, "", "", err
	}
	label := ref.ID
	if label == "" {
		label = ref.BundleID
	}
	return ref, label, cc, nil
}

// s3aResolveLocalApp maps an app ref to its numeric App Store id with the
// open store: an id passes through, a bundle id resolves through the stored
// listing snapshots (store.AppIDForBundle). When the store has never seen
// the bundle id the id is "" and note says how to capture it: an empty local
// result, not an error.
func s3aResolveLocalApp(ctx context.Context, st *store.Store, ref appstore.AppRef) (string, string, error) {
	if ref.ID != "" {
		return ref.ID, "", nil
	}
	id, err := store.AppIDForBundle(ctx, st, ref.BundleID)
	if err != nil {
		return "", "", apiErr(fmt.Errorf("reading stored bundle ids: %w", err))
	}
	if id == "" {
		return "", fmt.Sprintf("bundle id %s is not in the local store yet; capture it first with '%s app %s'", ref.BundleID, appStoreCLIName, ref.BundleID), nil
	}
	return id, "", nil
}

// s3aStorefrontFilter parses --countries for local reads: empty means the
// --country storefront, "all" means every stored storefront (a nil filter).
// Unlike countriesFlag it has no dogfood cap: reading the store is free.
func s3aStorefrontFilter(flags *rootFlags, raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		cc, err := appStoreCountry(flags)
		if err != nil {
			return nil, err
		}
		return []string{cc}, nil
	}
	if strings.EqualFold(raw, "all") {
		return nil, nil
	}
	list, err := appstore.ParseCountries(raw)
	if err != nil {
		return nil, usageErr(fmt.Errorf("--countries: %w", err))
	}
	return list, nil
}

// s3aSince parses a --since window: a duration such as 7d, 4w or 36h
// (counted back from now) or a date such as 2026-10-01. Empty or "all" means
// all time, as in s3bParseSince.
func s3aSince(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "all") {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	d, err := cliutil.ParseDurationLoose(raw)
	if err != nil || d <= 0 {
		return time.Time{}, usageErr(fmt.Errorf("--since: want a duration such as 7d, 4w or 36h, or a date such as 2026-10-01, got %q", raw))
	}
	return now.Add(-d).UTC(), nil
}

// s3aMissingMirror implements the missing-mirror guard: when the store file
// does not exist it prints the stderr hint naming the command that fills
// it and reports true. Callers then print their empty result and exit 0.
func s3aMissingMirror(cmd *cobra.Command, run string) bool {
	path, ok := appStoreMirrorExists(cmd)
	if ok {
		return false
	}
	s3aStderr(cmd, "no local mirror at %s\nrun: %s\n", path, run)
	return true
}

// s3aStderr writes a diagnostic line to the command's stderr. A failed
// stderr write has nowhere better to be reported, so it is dropped.
func s3aStderr(cmd *cobra.Command, format string, args ...any) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), format, args...)
}

// s3aEmit prints an object view: JSON (with --select/--compact/--csv/--agent)
// for machine output, the human renderer otherwise. Human output is built in
// a buffer and written once, so a failed write is returned.
func s3aEmit(cmd *cobra.Command, flags *rootFlags, v any, human func(*bytes.Buffer)) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, v, flags)
	}
	var b bytes.Buffer
	human(&b)
	_, err := w.Write(b.Bytes())
	return err
}

// s3aEmitList prints a list view. Machine output is the bare JSON array (so
// an empty result is exactly []) and the note goes to stderr; the human
// renderer prints the note itself.
func s3aEmitList(cmd *cobra.Command, flags *rootFlags, rows any, note string, human func(*bytes.Buffer)) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		if note != "" {
			s3aStderr(cmd, "note: %s\n", note)
		}
		return printJSONFiltered(w, rows, flags)
	}
	var b bytes.Buffer
	human(&b)
	_, err := w.Write(b.Bytes())
	return err
}

// s3aTable renders a human table (bold header, tab-aligned columns) into b.
func s3aTable(b *bytes.Buffer, header []string, rows [][]string) {
	var raw bytes.Buffer
	for i, h := range header {
		if i > 0 {
			raw.WriteByte('\t')
		}
		raw.WriteString(bold(h))
	}
	raw.WriteByte('\n')
	for _, r := range rows {
		raw.WriteString(strings.Join(r, "\t"))
		raw.WriteByte('\n')
	}
	tw := newTabWriter(b)
	_, _ = tw.Write(raw.Bytes())
	_ = tw.Flush()
}

// s3aCloseStore closes a store opened for a read; the read already
// succeeded, so a close error changes nothing for the caller.
func s3aCloseStore(st *store.Store) {
	_ = st.Close()
}

// s3aNoArgs rejects stray positional arguments on commands that take none.
func s3aNoArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	_ = cmd.Usage()
	return usageErr(fmt.Errorf("%s takes no positional arguments (got %q); use its flags", cmd.Name(), strings.Join(args, " ")))
}

// s3aCell scrubs terminal controls from stored text and truncates it for a
// human table cell.
func s3aCell(s string, max int) string {
	return truncate(strings.TrimSpace(cliutil.ScrubTerminal(s)), max)
}

// s3aTime renders a capture time for human output.
func s3aTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02 15:04")
}

// s3aIntPtr returns a pointer to v (nullable JSON numbers).
func s3aIntPtr(v int) *int { return &v }

// s3aRankText renders a nullable rank for human output.
func s3aRankText(r *int) string {
	if r == nil {
		return "-"
	}
	return strconv.Itoa(*r)
}

// s3aRound2 rounds to two decimals.
func s3aRound2(f float64) float64 {
	return math.Round(f*100) / 100
}

// s3aCompareVersions compares App Store version strings segment by segment:
// numeric segments numerically ("9.1.10" > "9.1.9"), others as text; a
// missing segment counts as 0 ("1.0" == "1.0.0"). It returns -1, 0 or 1.
func s3aCompareVersions(a, b string) int {
	split := func(v string) []string {
		return strings.FieldsFunc(strings.TrimSpace(v), func(r rune) bool {
			return r == '.' || r == '-' || r == '_' || r == ' ' || r == '(' || r == ')' || r == '/'
		})
	}
	as, bs := split(a), split(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		x, y := "0", "0"
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		xi, xerr := strconv.ParseInt(x, 10, 64)
		yi, yerr := strconv.ParseInt(y, 10, 64)
		switch {
		case xerr == nil && yerr == nil:
			if xi != yi {
				if xi < yi {
					return -1
				}
				return 1
			}
		default:
			if c := strings.Compare(strings.ToLower(x), strings.ToLower(y)); c != 0 {
				return c
			}
		}
	}
	return 0
}

// s3aStopwords are dropped from review-digest complaint terms: English
// function words, review boilerplate and generic store words (app, game,
// please ...). Terms are lower-cased before the check.
var s3aStopwords = func() map[string]bool {
	words := `a about above after again against all almost along already also although always am among an and
another any anybody anyone anything anyway anywhere app apple apps are aren't around as ask at away back be
became because become been before being below best better between big both but by came can can't cannot
cant come could couldn't did didn't do does doesn't doing don't done down during each either else even ever
every everyone everything few find first for found from game games gave get gets getting give given go goes
going gone good got gotta great had hadn't has hasn't have haven't having he he's her here hers herself him
himself his how however i i'd i'll i'm i've if im in instead into is isn't it it's its itself ive just keep
kind know last least less let let's like lot lots love made make makes many may maybe me might mine more most
much must my myself need needs never new next no none nor not nothing now of off often oh ok okay on once one
ones only or other others our ours out over own people please pls plz pretty put quite rather really right
said same say says see seem seems she should shouldn't since so some someone something sometimes soon still
such sure take than thank thanks that that's the their theirs them themselves then there there's these
they they're thing things think this those though through thru time times to too totally try trying two
under until up upon us use used uses using very want wants was wasn't way we we're well went were weren't
what what's when where whether which while who whole whom whose why will with within without won't would
wouldn't yeah yes yet you you'd you'll you're you've your yours yourself`
	out := map[string]bool{}
	for _, w := range strings.Fields(words) {
		out[w] = true
	}
	return out
}()

// s3aTerms returns the distinct candidate complaint terms of a review text:
// lower-cased runs of letters and digits (apostrophes kept inside words),
// at least three characters, not all digits and not a stopword.
func s3aTerms(text string) []string {
	text = strings.ToLower(strings.ReplaceAll(text, "’", "'"))
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, "'")
		f = strings.TrimSuffix(f, "'s")
		if len([]rune(f)) < 3 || s3aStopwords[f] || seen[f] {
			continue
		}
		allDigits := true
		for _, r := range f {
			if !unicode.IsDigit(r) {
				allDigits = false
				break
			}
		}
		if allDigits {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
