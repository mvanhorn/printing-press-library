// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Slice 2b shared helpers (top, search-store, reviews, suggest, keyword-rank,
// featured): the store-then-client opener, Lookup hydration that keeps each batch's
// capture time, snapshot de-duplication for responses served from the local
// cache, and the {meta, results} output envelope.

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// s2bTodayKind is the Today landing-page intent suffix that
// appstore.IsTodayKind matches ("LandingPage_iphone/today_Intent").
const s2bTodayKind = "today_Intent"

// Row limits of the two chart sources: the web chart page ranks 200 apps,
// the legacy RSS feed at most 100. Snapshots carry these so only captures of
// the same depth are compared.
const (
	s2bWebChartRows = 200
	s2bRSSChartRows = appstore.ChartFeedMaxLimit
)

// s2bDBFlag declares the command-local --db flag that appStoreDBPath honours.
func s2bDBFlag(cmd *cobra.Command) {
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI's data directory)")
}

// s2bOpenLive opens the snapshot store first, then builds the client and
// attaches the capture log so every request is logged. Callers close the
// store.
func s2bOpenLive(cmd *cobra.Command, flags *rootFlags) (*appstore.Client, *store.Store, error) {
	st, err := openAppStoreStore(cmd, flags)
	if err != nil {
		return nil, nil, err
	}
	c, err := newAppStoreClient(flags)
	if err != nil {
		_ = st.Close()
		return nil, nil, err
	}
	attachCaptureLog(c, st)
	return c, st, nil
}

// s2bEnvelope is the machine output of the slice-2b list commands: meta
// carries provenance and honest notes, results the rows. The generated
// output pipeline treats it as a {meta, results} envelope (--select reaches
// into results, --csv prints the rows, --agent merges meta).
type s2bEnvelope struct {
	Meta    any `json:"meta"`
	Results any `json:"results"`
}

// s2bPrint writes the envelope for machine output; in human mode it prints
// the rows as a table and the notes on stderr.
func s2bPrint(cmd *cobra.Command, flags *rootFlags, meta, rows any, notes []string) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return s2bPrintEnvelope(w, flags, meta, rows)
	}
	if err := printRows(cmd, flags, rows); err != nil {
		return err
	}
	s2bPrintNotes(cmd, notes)
	return nil
}

// s2bPrintEnvelope writes {meta, results} through the generated output
// pipeline. The rows are already a curated shape, so every key they carry is
// kept under --compact/--agent: the generic compaction would otherwise drop
// sparse fields (editorsChoice on the 25 chart lockups, developerResponse)
// and prose fields such as a review's body.
func s2bPrintEnvelope(w io.Writer, flags *rootFlags, meta, rows any) error {
	keep := make([]string, 0)
	if items, ok := tableItems(rows); ok {
		seen := map[string]bool{}
		for _, item := range items {
			for k := range item {
				if !seen[k] {
					seen[k] = true
					keep = append(keep, k)
				}
			}
		}
		sort.Strings(keep)
	}
	return printJSONFilteredKeep(w, s2bEnvelope{Meta: meta, Results: rows}, flags, keep...)
}

// s2bPrintNotes writes human-mode notes to stderr.
func s2bPrintNotes(cmd *cobra.Command, notes []string) {
	for _, n := range notes {
		if strings.TrimSpace(n) != "" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "note: "+n)
		}
	}
}

// s2bTime formats a capture time (RFC3339 UTC, second precision).
func s2bTime(t time.Time) string { return appstore.FormatTime(t) }

// s2bNowIfZero returns t, or now when t is zero.
func s2bNowIfZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

// s2bWriteCtx is the context for local snapshot writes: they must still
// land when the network context expired (--timeout) mid-run.
func s2bWriteCtx(ctx context.Context) context.Context { return context.WithoutCancel(ctx) }

// --- Lookup hydration -------------------------------------------------------

// s2bLookupBatch is one Lookup request's worth of hydrated apps with the
// time Apple answered it (the original time when served from the cache).
type s2bLookupBatch struct {
	IDs       []string
	Results   []appstore.LookupResult
	FetchedAt time.Time
	FromCache bool
}

// s2bHydration is the merged result of hydrating ranked ids with Lookup.
type s2bHydration struct {
	ByID     map[string]appstore.LookupResult
	Missing  []string
	Batches  []s2bLookupBatch
	Failures []fetchFailure
	Requests int
}

// s2bHydrate looks ids up in one storefront with one request per
// appstore.LookupBatchSize ids. A throttle (or an expired context) stops at
// once and is returned together with the batches that succeeded; any other
// batch error becomes a fetch failure and the remaining batches still run.
func s2bHydrate(ctx context.Context, c *appstore.Client, cc string, ids []string) (*s2bHydration, error) {
	h := &s2bHydration{ByID: map[string]appstore.LookupResult{}, Missing: make([]string, 0), Failures: make([]fetchFailure, 0)}
	for start := 0; start < len(ids); start += appstore.LookupBatchSize {
		end := start + appstore.LookupBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		resp, err := c.Lookup(ctx, appstore.LookupQuery{IDs: chunk, Country: cc})
		if err != nil {
			if isRateLimited(err) || ctx.Err() != nil {
				return h, err
			}
			h.Failures = append(h.Failures, fetchFailure{ID: fmt.Sprintf("lookup ranks %d-%d", start+1, end), Storefront: cc, Error: err.Error()})
			continue
		}
		h.Requests += resp.Requests
		b := s2bLookupBatch{IDs: chunk, Results: resp.Results}
		if len(resp.Meta) > 0 {
			b.FetchedAt = resp.Meta[0].FetchedAt
			b.FromCache = resp.Meta[0].FromCache
		}
		b.FetchedAt = s2bNowIfZero(b.FetchedAt)
		h.Batches = append(h.Batches, b)
		h.Missing = append(h.Missing, resp.Missing...)
		for _, r := range resp.Results {
			h.ByID[r.AppID()] = r
		}
	}
	return h, nil
}

// s2bRecordLookupRatings appends one "lookup" ratings snapshot per hydrated
// app, dated at its batch's fetch time (a batch served from the local cache
// keeps its original time). The store skips observations it already holds,
// so reruns never duplicate points. It returns how many snapshots were
// written.
func s2bRecordLookupRatings(ctx context.Context, st *store.Store, cc string, h *s2bHydration) (int, error) {
	if h == nil {
		return 0, nil
	}
	snaps := make([]store.RatingsSnapshot, 0)
	for _, b := range h.Batches {
		for _, r := range b.Results {
			id := r.AppID()
			if id == "" {
				continue
			}
			snaps = append(snaps, store.RatingsSnapshot{
				CapturedAt:    b.FetchedAt,
				Storefront:    cc,
				AppID:         id,
				AverageRating: r.AverageUserRating,
				RatingCount:   r.UserRatingCount,
				Version:       r.Version,
				Source:        store.SourceLookup,
				DataAsOf:      r.DataAsOf,
			})
		}
	}
	return store.RecordRatingsN(ctx, st, snaps...)
}

// s2bRecordKeywordRank appends a keyword capture dated at the response's
// fetch time; the store skips an observation it already holds (what a rerun
// served from the local cache would be). It reports whether a row was
// written.
func s2bRecordKeywordRank(ctx context.Context, st *store.Store, k store.KeywordRank) (bool, error) {
	k.CapturedAt = s2bNowIfZero(k.CapturedAt)
	return store.RecordKeywordRankOnce(ctx, st, k)
}

// --- search helpers -----------------------------------------------------------

// s2bSearchPlatform validates --platform for store search: the Search API
// has software entities for iphone, ipad and mac only.
func s2bSearchPlatform(raw string) (string, string, error) {
	p, err := appstore.NormalizePlatform(raw)
	if err != nil {
		return "", "", usageErr(fmt.Errorf("--platform: %w", err))
	}
	switch p {
	case "iphone":
		return p, "software", nil
	case "ipad":
		return p, "iPadSoftware", nil
	case "mac":
		return p, "desktopSoftware", nil
	}
	return "", "", usageErr(fmt.Errorf("--platform: store search covers iphone, ipad and mac, not %q", raw))
}

// s2bSearchSource validates --source for search-store and keyword-rank:
// itunes (the documented Search API, default) or web (the robots-disallowed
// web search page, opt-in).
func s2bSearchSource(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "itunes", "api-search", "search-api":
		return store.SourceSearchAPI, nil
	case "web":
		return store.SourceWebSearch, nil
	}
	return "", usageErr(fmt.Errorf("--source: unknown source %q; use itunes (default) or web (opt-in)", raw))
}

// s2bTermWords splits a search term into lower-case words.
func s2bTermWords(term string) []string {
	return strings.FieldsFunc(strings.ToLower(term), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// s2bTitleMatches reports whether a title contains every word of the term
// (case-insensitive word match, any order).
func s2bTitleMatches(title string, words []string) bool {
	if len(words) == 0 {
		return false
	}
	have := map[string]bool{}
	for _, w := range s2bTermWords(title) {
		have[w] = true
	}
	for _, w := range words {
		if !have[w] {
			return false
		}
	}
	return true
}

// s2bFloat and s2bInt return pointers for fields that are absent (nil) when
// unknown rather than a misleading zero.
func s2bFloat(v float64) *float64 { return &v }
func s2bInt(v int64) *int64       { return &v }
func s2bBool(v bool) *bool        { return &v }

// s2bStoreFailed reports a failed local write without failing the command:
// the live result still prints, with a stderr warning and a meta note. The
// time-series point is lost; nothing is wrong with the configuration, so this
// is not exit 10.
func s2bStoreFailed(cmd *cobra.Command, notes *[]string, what string, err error) {
	msg := fmt.Sprintf("could not store %s locally (%v); the live result is still shown", what, err)
	s2aPrintln(cmd.ErrOrStderr(), "warning: "+msg)
	*notes = append(*notes, msg)
}
