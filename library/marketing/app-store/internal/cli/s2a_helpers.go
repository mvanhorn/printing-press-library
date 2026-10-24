// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the slice-2a app-page commands (app, privacy, versions,
// iap, events, similar, developer, ratings, prices, compare): lookups that
// keep Apple's fetch time, snapshot writes that skip captures already stored,
// the fan-out envelope and small human renderers. The commands build their
// client with newAppStoreClient (tests swap the transport through
// appStoreClientHook).

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// s2aNow is the clock for derived fields such as an in-app event's status.
var s2aNow = time.Now

// s2aAppArgHelp names the accepted <app> forms in usage errors.
const s2aAppArgHelp = "pass a numeric id (324684580), id324684580, an App Store URL or a bundle id (com.spotify.client)"

// s2aOneAppArg validates the single <app> positional of the one-app commands.
func s2aOneAppArg(cmd *cobra.Command, args []string) (appstore.AppRef, error) {
	if len(args) == 0 {
		_ = cmd.Usage()
		return appstore.AppRef{}, usageErr(fmt.Errorf("<app> is required: %s", s2aAppArgHelp))
	}
	if len(args) > 1 {
		_ = cmd.Usage()
		return appstore.AppRef{}, usageErr(fmt.Errorf("%s takes one app, got %d arguments (use 'compare' for several apps)", cmd.Name(), len(args)))
	}
	return parseAppRef(args[0])
}

// s2aAppArgs validates the <app>... positionals of the multi-app commands
// (prices, compare): every argument must parse; duplicates are dropped.
func s2aAppArgs(cmd *cobra.Command, args []string, min int) ([]appstore.AppRef, error) {
	if len(args) < min {
		_ = cmd.Usage()
		if min <= 1 {
			return nil, usageErr(fmt.Errorf("<app> is required: %s", s2aAppArgHelp))
		}
		return nil, usageErr(fmt.Errorf("%s needs at least %d apps, got %d: %s", cmd.Name(), min, len(args), s2aAppArgHelp))
	}
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
	if len(refs) < min {
		_ = cmd.Usage()
		return nil, usageErr(fmt.Errorf("%s needs at least %d different apps, got %d", cmd.Name(), min, len(refs)))
	}
	return refs, nil
}

// s2aStorefronts returns the storefronts a fan-out command queries: the
// --countries list, else the app URL's storefront (unless --country was set)
// or --country.
func s2aStorefronts(cmd *cobra.Command, flags *rootFlags, countriesRaw string, ref *appstore.AppRef) ([]string, error) {
	if strings.TrimSpace(countriesRaw) != "" {
		return countriesFlag(flags, countriesRaw)
	}
	if ref != nil {
		cc, err := refCountry(cmd, flags, *ref)
		if err != nil {
			return nil, err
		}
		return []string{cc}, nil
	}
	return countriesFlag(flags, "")
}

// s2aProductPage fetches an app's product page from its Lookup row's
// canonical URL (one request; the slugless id URL costs an extra redirect).
// platform "" or iphone is the default page.
func s2aProductPage(ctx context.Context, c *appstore.Client, lk appstore.LookupResult, country, platform string) (*appstore.ProductPage, error) {
	q := appstore.ProductPageQuery{URL: lk.CanonicalURL(), Platform: platform}
	if q.URL == "" {
		q = appstore.ProductPageQuery{Country: country, AppID: lk.AppID(), Platform: platform}
	}
	page, err := c.ProductPage(ctx, q)
	if err != nil {
		return nil, classifyAppStoreErr(err)
	}
	return page, nil
}

// s2aLookupIn resolves ref in one storefront. found is false (with a nil
// error) when Apple has no such app there. fetchedAt is when Apple served the
// row; a row from the local cache keeps its original fetch time.
func s2aLookupIn(ctx context.Context, c *appstore.Client, ref appstore.AppRef, country string) (appstore.LookupResult, time.Time, bool, error) {
	q := appstore.LookupQuery{Country: country}
	if ref.ID != "" {
		q.IDs = []string{ref.ID}
	} else {
		q.BundleIDs = []string{ref.BundleID}
	}
	resp, err := c.Lookup(ctx, q)
	if err != nil {
		return appstore.LookupResult{}, time.Time{}, false, classifyAppStoreErr(err)
	}
	at := s2aBatchFetchedAt(resp, 0)
	if len(resp.Results) == 0 {
		return appstore.LookupResult{}, at, false, nil
	}
	return resp.Results[0], at, true, nil
}

// s2aBatchFetchedAt returns when Lookup batch n was served.
func s2aBatchFetchedAt(resp *appstore.LookupResponse, n int) time.Time {
	if resp == nil || len(resp.Meta) == 0 {
		return time.Time{}
	}
	if n < 0 {
		n = 0
	}
	if n >= len(resp.Meta) {
		n = len(resp.Meta) - 1
	}
	return resp.Meta[n].FetchedAt
}

// s2aIDFetchedAt returns when the batch holding id was served, for a Lookup
// run over ids (batches of appstore.LookupBatchSize in input order).
func s2aIDFetchedAt(resp *appstore.LookupResponse, ids []string, id string) time.Time {
	for i, v := range ids {
		if v == id {
			return s2aBatchFetchedAt(resp, i/appstore.LookupBatchSize)
		}
	}
	return s2aBatchFetchedAt(resp, 0)
}

// s2aRefLabel is the identifier a fetch failure or note shows for an app ref.
func s2aRefLabel(ref appstore.AppRef) string {
	if ref.ID != "" {
		return ref.ID
	}
	if ref.BundleID != "" {
		return ref.BundleID
	}
	return ref.Raw
}

// --- snapshot writes ---------------------------------------------------------
//
// A response served from the local cache keeps Apple's original fetch time,
// and the store stamps snapshots with that time. The store's writers are
// idempotent on that observation (app, storefront, source, time), so
// recording a cached response again adds nothing.

// The listing history (snapshots, versions, privacy, IAP prices, events)
// keeps each storefront's default language. A capture in a language the run
// names with --lang is printed but kept out of it, so a translated subtitle or
// purchase name never reads as a listing edit in watch-listing or changes;
// rating counts do not depend on the language and are still recorded.

// s2aLangNoted holds the root commands that already printed the --lang note.
var s2aLangNoted sync.Map

// s2aLangKeepsOutOfHistory reports whether this run names a page language
// (--lang) and, the first time per run, says on stderr that its captures stay
// out of the listing history.
func s2aLangKeepsOutOfHistory(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	f := cmd.Flag("lang")
	if f == nil || strings.TrimSpace(f.Value.String()) == "" {
		return false
	}
	if _, seen := s2aLangNoted.LoadOrStore(cmd.Root(), true); !seen {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: --lang %s captures are printed but not added to the listing history, which keeps each storefront's default language so a translation never reads as a listing edit; rating counts are still recorded\n", strings.TrimSpace(f.Value.String()))
	}
	return true
}

// s2aRecordProductPage persists a product page with store.RecordProductPage
// (listing snapshot when lk is set, versions, privacy, IAP prices, events and
// a page ratings snapshot), dated at the page's fetch time. Under --lang only
// the page ratings snapshot is written.
func s2aRecordProductPage(ctx context.Context, cmd *cobra.Command, st *store.Store, storefront string, page *appstore.ProductPage, lk appstore.LookupResult) error {
	if st == nil || page == nil {
		return nil
	}
	if s2aLangKeepsOutOfHistory(cmd) {
		return store.RecordPageRatings(s2bWriteCtx(ctx), st, storefront, *page, lk)
	}
	return store.RecordProductPage(s2bWriteCtx(ctx), st, storefront, *page, lk)
}

// s2aRecordLookupRatings appends a ratings snapshot with the exact Lookup
// counts (source "lookup"), stamped with the Lookup's fetch time.
func s2aRecordLookupRatings(ctx context.Context, st *store.Store, storefront string, lk appstore.LookupResult, fetchedAt time.Time) error {
	if st == nil || lk.TrackID == 0 {
		return nil
	}
	return store.RecordRatings(s2bWriteCtx(ctx), st, store.RatingsSnapshot{
		CapturedAt:    firstNonZeroTime(fetchedAt, lk.FetchedAt),
		Storefront:    storefront,
		AppID:         lk.AppID(),
		AverageRating: lk.AverageUserRating,
		RatingCount:   lk.UserRatingCount,
		Version:       lk.Version,
		Source:        store.SourceLookup,
		DataAsOf:      lk.DataAsOf,
	})
}

// s2aRecordLookupSnapshot appends a lookup-only listing snapshot (source
// "lookup"), stamped with the Lookup's fetch time. Under --lang nothing is
// written: Lookup localizes the title, release notes and description too.
func s2aRecordLookupSnapshot(ctx context.Context, cmd *cobra.Command, st *store.Store, storefront string, lk appstore.LookupResult, fetchedAt time.Time) error {
	if st == nil || lk.TrackID == 0 || s2aLangKeepsOutOfHistory(cmd) {
		return nil
	}
	return store.RecordLookupSnapshot(s2bWriteCtx(ctx), st, storefront, lk, fetchedAt)
}

// firstNonZeroTime returns the first non-zero time (zero when all are).
func firstNonZeroTime(ts ...time.Time) time.Time {
	for _, t := range ts {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

// s2aWarnStore reports a failed local-store write on stderr. The live answer
// is still printed: the store is history, not the source of the result.
func s2aWarnStore(cmd *cobra.Command, what string, err error) {
	if err != nil {
		s2aPrintf(cmd.ErrOrStderr(), "warning: could not record %s in the local store: %v\n", what, err)
	}
}

// s2aNote prints an explanatory note for an empty or partial result: on
// stdout in human mode, on stderr when stdout carries machine output.
func s2aNote(cmd *cobra.Command, flags *rootFlags, msg string) {
	if msg == "" {
		return
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		s2aPrintln(cmd.OutOrStdout(), msg)
		return
	}
	s2aPrintln(cmd.ErrOrStderr(), "note: "+msg)
}

// --- fan-out envelope -------------------------------------------------------

// s2aEnvelope is the JSON shape of the fan-out commands (iap, ratings,
// prices, compare): rows under items, failed requests under fetch_failures
// (always present, [] when none) and the command's context under meta.
// Failed fetches never become rows.
type s2aEnvelope struct {
	Meta          any            `json:"meta"`
	Items         any            `json:"items"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
}

// s2aFanoutOutcome decides how a fan-out run ends once its loop finished:
// every storefront failing is an error (the first failure's), nothing
// available anywhere is a not-found error, anything else prints.
func s2aFanoutOutcome(total, available int, errs []error, notFound func() error) error {
	if total > 0 && len(errs) == total {
		return errs[0]
	}
	if available == 0 && len(errs) == 0 {
		return notFound()
	}
	return nil
}

// s2aNotAttempted lists the storefronts a fan-out never reached because a
// throttle or a pacing refusal stopped it at stoppedAt (every later request
// would be refused the same way).
func s2aNotAttempted(id string, storefronts []string, stoppedAt string) []fetchFailure {
	out := make([]fetchFailure, 0, len(storefronts))
	for _, cc := range storefronts {
		out = append(out, fetchFailure{ID: id, Storefront: cc, Error: "not attempted: the run stopped at " + stoppedAt + " (rate limited, paced or out of time)"})
	}
	return out
}

// s2aStopped is a fan-out command's result once it printed what it gathered:
// nil when nothing stopped it, else the throttle or pacing error (exit 7), so
// a caller still learns the run is incomplete.
func s2aStopped(stopped error) error {
	if stopped == nil {
		return nil
	}
	return classifyAppStoreErr(stopped)
}

// --- human rendering ---------------------------------------------------------

// s2aPrintf and s2aPrintln write human output and stderr diagnostics. A
// failed write to a terminal or a closed pipe has nowhere better to be
// reported, so the error is dropped here, in one place.
func s2aPrintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func s2aPrintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

// s2aTable writes an aligned plain-text table (cells scrubbed of terminal
// control sequences).
func s2aTable(w io.Writer, headers []string, rows [][]string) error {
	tw := newTabWriter(w)
	up := make([]string, len(headers))
	for i, h := range headers {
		up[i] = strings.ToUpper(h)
	}
	s2aPrintln(tw, strings.Join(up, "\t"))
	for _, r := range rows {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = cliutil.ScrubTerminal(strings.ReplaceAll(c, "\t", " "))
		}
		s2aPrintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// s2aFailuresHuman lists fetch failures under a human table.
func s2aFailuresHuman(w io.Writer, failures []fetchFailure) {
	for _, f := range failures {
		label := strings.TrimSpace(f.Storefront + " " + f.ID)
		s2aPrintf(w, "failed %s: %s\n", label, cliutil.ScrubTerminal(f.Error))
	}
}

// s2aCount formats an integer with thousands separators.
func s2aCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// s2aRating formats an average star rating.
func s2aRating(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// s2aDay shortens an RFC3339 timestamp to its date.
func s2aDay(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// s2aOneLine collapses whitespace and truncates for table cells.
func s2aOneLine(s string, max int) string {
	return truncate(strings.Join(strings.Fields(s), " "), max)
}

// s2aEventStatus classifies an in-app event against now: upcoming, live,
// ended, or unknown when Apple sent no dates.
func s2aEventStatus(start, end string, now time.Time) string {
	st, startOK := s2aParseTime(start)
	en, endOK := s2aParseTime(end)
	switch {
	case endOK && now.After(en):
		return "ended"
	case startOK && now.Before(st):
		return "upcoming"
	case startOK || endOK:
		return "live"
	}
	return "unknown"
}

func s2aParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
