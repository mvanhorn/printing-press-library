// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// source_selection.go is the runtime decision seam for "cookie vs bearer"
// on every Happenstance-touching CLI command (coverage, hp people,
// prospect, warm-intro). It implements the decision tree from the
// 2026-04-19 plan's High-Level Technical Design section verbatim:
//
//   1. Explicit --source api  -> SourceAPI  (env var REQUIRED).
//   2. Explicit --source hp   -> SourceCookie (cookies REQUIRED).
//   3. Auto / unset:
//        a. cookie + cached searchesRemaining > 0 -> SourceCookie
//        b. cookie + cached searchesRemaining == 0 + env var set
//                                                    -> SourceAPI
//        c. cookie + cached searchesRemaining == 0 + no env var
//                                                    -> SourceCookie
//                                                       with deferredErr
//        d. no cookie + env var set                  -> SourceAPI
//        e. no cookie + no env var                   -> exit 4
//        f. unknown cached value                     -> SourceCookie
//                                                       (call-site retry
//                                                       wrapper handles
//                                                       any 429 fallback)
//
// The cookie-FIRST default reflects the user's explicit preference: spend
// the free monthly web-app allocation before paid bearer credits. Users
// flip to the bearer surface explicitly via --source api when they want
// the richer schema (deep research, groups, structured usage).
//
// "Try cookie, fall back to bearer on 429" is implemented as
// ExecuteWithSourceFallback at the call site, NOT inside SelectSource.
// That keeps SelectSource a pure function over its inputs and lets the
// retry wrapper own the transient-network concern.
//
// The cookie-quota cache (60s TTL by default; bypass via --no-cache) is
// process-local and lives in this file. The bearer-side /v1/usage is
// NEVER consulted by SelectSource; we do not burn paid network calls in
// the common cookie-first path.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

// Source enumerates the Happenstance auth surfaces the CLI can route a
// search-class call through. The two surfaces have different base URLs,
// different auth (cookie+Clerk vs bearer), different response shapes,
// and different rate-limit models (free monthly allocation vs paid
// credits). SelectSource decides which one to use per command per
// invocation.
type Source string

const (
	// SourceCookie routes through the existing happenstance.ai web-app
	// surface, authenticated via Chrome session cookies. Free quota,
	// monthly renewal. Returns the rich /api/search schema.
	SourceCookie Source = "hp"

	// SourceAPI routes through api.happenstance.ai/v1, authenticated via
	// HAPPENSTANCE_API_KEY bearer token. Paid credits (2 per search, 1
	// per research on completion). Returns a thinner schema; the
	// normalizer in internal/happenstance/api/normalize.go projects it
	// into client.Person.
	SourceAPI Source = "api"
)

// Source flag string values accepted on --source. Kept as named consts so
// the command files reference them by name rather than by string literal.
const (
	SourceFlagAPI    = "api"
	SourceFlagCookie = "hp"
	SourceFlagAuto   = "auto"
	SourceFlagBoth   = "both" // cross-source commands only (coverage)
	SourceFlagLI     = "li"   // cross-source commands only (LinkedIn)
)

// UnknownSearchesRemaining is the sentinel passed to SelectSource when
// the cookie quota cache has no entry yet (and --no-cache was not set,
// or the probe simply has not happened). The decision tree treats it as
// "proceed cookie-first; the call-site retry wrapper will fall back to
// bearer if the server returns 429".
const UnknownSearchesRemaining = -1

// CreditCostPerSearch is the documented cost (in Happenstance credits)
// of one /v1/search call. Surfaced in the cookie-then-bearer fallback
// log so the user sees the credit spend at the moment it happens.
const CreditCostPerSearch = 2

// FallbackNoticeMessage is the canonical wording the call-site fallback
// wrapper writes to stderr when it switches surfaces mid-flight. Tests
// grep for this exact string; rendering layers may also embed it as the
// "fell back from cookie" notice on the response envelope.
const FallbackNoticeMessage = "cookie quota exhausted - falling back to paid bearer surface (cost: 2 credits)"

// SelectSource implements the decision tree documented at the top of
// this file. It is a pure function over its inputs: same inputs always
// produce the same output. All side effects (logging, prompting,
// network calls) live in the surrounding call-site wrappers.
//
// Returns:
//   - chosen Source (SourceCookie or SourceAPI)
//   - deferredErr: non-nil only in the case where cookie was chosen but
//     the user is going to 429 unless they set HAPPENSTANCE_API_KEY.
//     The error wraps an actionable hint; the call-site renders it as
//     a warning before the request even fires, so the user knows the
//     fallback is unavailable in advance.
//   - hardErr: non-nil when no source can be selected and the call must
//     abort. Always carries an authErr exit code (4).
//
// When the explicit source is "" or SourceFlagAuto, the auto-routing
// branch fires. When the explicit source is SourceFlagBoth or
// SourceFlagLI, that's a cross-source flag and the caller should NOT
// have invoked SelectSource for it; we treat them as auto for safety
// (so the cross-source command can still pick a Happenstance side).
func SelectSource(ctx context.Context, source string, cfg *config.Config, cookieAvailable bool, cachedSearchesRemaining int) (chosen Source, deferredErr, hardErr error) {
	hasAPIKey := config.LoadAPIKey(cfg) != ""

	switch source {
	case SourceFlagAPI:
		if !hasAPIKey {
			return "", nil, authErr(fmt.Errorf(
				"%s required for --source api (provision at %s)",
				api.KeyEnvVar, api.RotationURL,
			))
		}
		return SourceAPI, nil, nil

	case SourceFlagCookie:
		if !cookieAvailable {
			return "", nil, authErr(fmt.Errorf(
				"--source hp requires Happenstance cookie auth — run `contact-goat-pp-cli auth login --chrome --service happenstance` first",
			))
		}
		return SourceCookie, nil, nil
	}

	// Auto-routing path. Cookie-first default; bearer is fallback.
	if cookieAvailable {
		switch {
		case cachedSearchesRemaining > 0:
			// Free quota remains: 3a.
			return SourceCookie, nil, nil
		case cachedSearchesRemaining == 0 && hasAPIKey:
			// Quota exhausted, paid surface available: 3b.
			return SourceAPI, nil, nil
		case cachedSearchesRemaining == 0 && !hasAPIKey:
			// Quota exhausted, no paid fallback configured: 3c.
			// Return SourceCookie with a deferred warning; the call
			// will 429 but the user gets an actionable hint.
			return SourceCookie, fmt.Errorf(
				"cookie quota is exhausted — set %s to fall back to the paid bearer surface (provision at %s)",
				api.KeyEnvVar, api.RotationURL,
			), nil
		default:
			// Unknown remaining (3f): proceed cookie-first; the call-site
			// retry wrapper handles 429 fallback if needed.
			return SourceCookie, nil, nil
		}
	}

	// No cookie auth.
	if hasAPIKey {
		return SourceAPI, nil, nil // 3d
	}
	// 3e: nothing configured at all.
	return "", nil, authErr(fmt.Errorf(
		"no Happenstance auth configured — set %s for the bearer surface (provision at %s) OR run `contact-goat-pp-cli auth login --chrome --service happenstance` for the cookie surface",
		api.KeyEnvVar, api.RotationURL,
	))
}

// quotaCacheTTL bounds how long a SearchesRemaining value is reused
// across SelectSource calls within the same process. Long enough to
// cover the natural batch-of-commands-in-a-script case; short enough
// that a renewed quota is picked up quickly. Bypass with --no-cache
// (the rootFlags.noCache field).
const quotaCacheTTL = 60 * time.Second

type quotaCacheEntry struct {
	searchesRemaining int
	asOf              time.Time
}

var (
	quotaCacheMu sync.Mutex
	quotaCache   = map[string]quotaCacheEntry{}
)

// quotaCacheKey scopes the cache by config path so a single process
// servicing two configs (e.g. tests) doesn't cross-contaminate.
func quotaCacheKey(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return cfg.Path
}

// FetchSearchesRemaining probes the cookie surface's /api/user/limits
// endpoint and returns the live searchesRemaining value. The call is
// cached for quotaCacheTTL by default; bypass with bypassCache=true
// (mirroring rootFlags.noCache). Returns UnknownSearchesRemaining and
// no error when the probe itself fails — the SelectSource decision
// tree handles "unknown" as "proceed cookie-first; let the retry
// wrapper fall back".
//
// The cookie client is passed in (rather than constructed) so the
// caller can reuse the same authenticated client for the actual search
// that follows.
func FetchSearchesRemaining(c *client.Client, cfg *config.Config, bypassCache bool) int {
	if c == nil || !c.HasCookieAuth() {
		return UnknownSearchesRemaining
	}
	key := quotaCacheKey(cfg)
	if !bypassCache {
		quotaCacheMu.Lock()
		entry, ok := quotaCache[key]
		quotaCacheMu.Unlock()
		if ok && time.Since(entry.asOf) < quotaCacheTTL {
			return entry.searchesRemaining
		}
	}

	raw, err := c.Get("/api/user/limits", nil)
	if err != nil {
		return UnknownSearchesRemaining
	}
	// /api/user/limits returns {"searchesRemaining":N, ...}.
	var limits struct {
		SearchesRemaining *int `json:"searchesRemaining"`
	}
	if err := json.Unmarshal(raw, &limits); err != nil || limits.SearchesRemaining == nil {
		return UnknownSearchesRemaining
	}

	quotaCacheMu.Lock()
	quotaCache[key] = quotaCacheEntry{
		searchesRemaining: *limits.SearchesRemaining,
		asOf:              time.Now(),
	}
	quotaCacheMu.Unlock()
	return *limits.SearchesRemaining
}

// resetQuotaCache clears the in-memory cache. Test-only; not exported.
func resetQuotaCache() {
	quotaCacheMu.Lock()
	quotaCache = map[string]quotaCacheEntry{}
	quotaCacheMu.Unlock()
}

// IsCookieRateLimitError reports whether err looks like the cookie
// surface's canonical "Rate limit reached" 429. The cookie client
// surfaces this as a free-form error string today; we match on the
// substring rather than a typed sentinel because the underlying
// internal/client/client.go does not export one. If a typed error
// gets added later, prefer errors.Is on it instead.
func IsCookieRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, []string{
		"Rate limit reached",
		"rate limit reached",
		"HTTP 429",
		"returned HTTP 429",
	})
}

// IsBearerRateLimitError reports whether err is the bearer client's
// typed *api.RateLimitError. Symmetric to IsCookieRateLimitError but
// for the paid surface; used by call sites that want to distinguish
// "out of credits" (api 402) from "throttled" (api 429).
func IsBearerRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var rl *api.RateLimitError
	return errors.As(err, &rl)
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if indexOf(s, n) >= 0 {
			return true
		}
	}
	return false
}

// indexOf is a substring search inlined to avoid importing strings just
// for this one call (keeps the file's import surface lean).
func indexOf(s, sub string) int {
	if sub == "" {
		return 0
	}
	n := len(sub)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}

// CookieRunner is the call-site contract for "run a Happenstance
// people-search via the cookie surface and return the canonical
// PeopleSearchResult". Implementations close over the cookie client
// the caller already constructed.
type CookieRunner func() (*client.PeopleSearchResult, error)

// BearerRunner is the call-site contract for "run an equivalent search
// via the bearer surface and return a canonical PeopleSearchResult".
// Implementations close over the bearer api.Client they construct on
// demand. The bearer call must produce a value shape-compatible with
// the cookie call so downstream renderers don't branch on source.
type BearerRunner func() (*client.PeopleSearchResult, error)

// FallbackResult bundles a search result with metadata about how it was
// produced. FellBackFromCookie is true when the cookie call returned a
// 429 mid-flight and the wrapper retried via the bearer surface. The
// renderer surfaces this as an embedded notice on the JSON envelope.
type FallbackResult struct {
	Result             *client.PeopleSearchResult
	UsedSource         Source
	FellBackFromCookie bool
	FallbackNotice     string
}

// ExecuteWithSourceFallback runs the cookie-then-bearer retry pattern
// described at the top of this file. Selected drives the initial
// attempt:
//
//   - Selected == SourceCookie: invoke cookieRun. On 429-class error,
//     if a non-nil bearerRun is provided, log the fallback notice to
//     errOut and invoke bearerRun. Surface either result or both
//     errors.
//   - Selected == SourceAPI: invoke bearerRun directly. No fallback to
//     cookie (we never reach for free quota when the user explicitly
//     picked or auto-routed to the paid surface).
//
// errOut is typically cmd.ErrOrStderr(); pass io.Discard to suppress.
//
// The bearerRun argument may be nil (when no HAPPENSTANCE_API_KEY is
// configured); in that case a cookie 429 surfaces verbatim with an
// actionable hint appended.
func ExecuteWithSourceFallback(
	ctx context.Context,
	selected Source,
	cookieRun CookieRunner,
	bearerRun BearerRunner,
	errOut io.Writer,
) (FallbackResult, error) {
	if errOut == nil {
		errOut = io.Discard
	}
	if selected == SourceAPI {
		if bearerRun == nil {
			return FallbackResult{}, authErr(fmt.Errorf(
				"%s required for --source api (provision at %s)",
				api.KeyEnvVar, api.RotationURL,
			))
		}
		res, err := bearerRun()
		if err != nil {
			return FallbackResult{UsedSource: SourceAPI}, err
		}
		return FallbackResult{Result: res, UsedSource: SourceAPI}, nil
	}

	// SourceCookie path.
	if cookieRun == nil {
		return FallbackResult{}, fmt.Errorf("source_selection: cookieRun is nil but selected source is cookie")
	}
	res, err := cookieRun()
	if err == nil {
		return FallbackResult{Result: res, UsedSource: SourceCookie}, nil
	}
	if !IsCookieRateLimitError(err) {
		// Non-429 cookie error: surface verbatim. We do NOT silently fall
		// back to the bearer surface on arbitrary cookie failures (auth
		// expired, network hiccup, etc.); only documented quota errors
		// trigger the paid retry.
		return FallbackResult{UsedSource: SourceCookie}, err
	}
	// Cookie 429.
	if bearerRun == nil {
		// No paid fallback configured: surface the original 429 with an
		// actionable hint about HAPPENSTANCE_API_KEY appended.
		return FallbackResult{UsedSource: SourceCookie}, fmt.Errorf(
			"%w\nhint: set %s to enable automatic fallback to the paid bearer surface (provision at %s)",
			err, api.KeyEnvVar, api.RotationURL,
		)
	}
	// Paid retry.
	fmt.Fprintln(errOut, FallbackNoticeMessage)
	bres, berr := bearerRun()
	if berr != nil {
		return FallbackResult{UsedSource: SourceAPI, FellBackFromCookie: true}, fmt.Errorf(
			"cookie 429 + bearer fallback failed: %v (cookie error: %v)", berr, err,
		)
	}
	return FallbackResult{
		Result:             bres,
		UsedSource:         SourceAPI,
		FellBackFromCookie: true,
		FallbackNotice:     FallbackNoticeMessage,
	}, nil
}

// LogDeferredHint writes the SelectSource deferredErr to stderr, prefixed
// with "warning:". A nil err is a no-op so call sites can call this
// unconditionally. Stderr matches the existing CLI convention for
// non-fatal hints (used by warm-intro, coverage, prospect).
func LogDeferredHint(errOut io.Writer, err error) {
	if err == nil {
		return
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	fmt.Fprintf(errOut, "warning: %v\n", err)
}

// BearerSearchAdapter is a convenience for call sites: given a bearer
// client and a free-text query, run POST /v1/search + PollSearch and
// project the results into a client.PeopleSearchResult so the
// downstream rendering code does not branch on source.
//
// currentUUID is the current user's Happenstance UUID (fetched by the
// caller from its cookie client). It is used to retag the self-entry
// in the envelope's mutuals list so renderers can distinguish
// 1st-degree-via-friend hits from self-graph hits. Pass "" when the
// caller cannot resolve it (then every bridge is treated as a friend
// bridge, which is harmless but less precise).
//
// The PollSearch loop honors ctx for cancellation; default poll
// timeout / interval mirror the cookie surface (180s / 1s).
//
// Returns a usable result even when the server's terminal status is
// FAILED or FAILED_AMBIGUOUS — the caller decides whether an empty
// People slice with a non-COMPLETED Status is "no results" or "error
// surface". This matches the cookie surface's "Completed=false on
// timeout" convention, where the call site is responsible for
// interpreting partial state.
func BearerSearchAdapter(ctx context.Context, c *api.Client, query string, currentUUID string, opts *api.SearchOptions) (*client.PeopleSearchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("bearer search: nil client")
	}
	// The bearer surface returns HTTP 400 (no-search-source) when no
	// search source is set. The cookie surface's defaultSearchOptions
	// turns on both 1st-degree and 2nd-degree search; mirror that here
	// so auto-routed callers (coverage / hp people / prospect / warm-
	// intro) get parity with the cookie default. Explicit opts from
	// `api hpn search` flags pass through unchanged.
	if opts == nil {
		opts = &api.SearchOptions{
			IncludeMyConnections:      true,
			IncludeFriendsConnections: true,
		}
	} else if !opts.IncludeMyConnections && !opts.IncludeFriendsConnections && len(opts.GroupIDs) == 0 {
		opts.IncludeMyConnections = true
		opts.IncludeFriendsConnections = true
	}
	env, err := c.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	final, err := c.PollSearch(ctx, env.Id, nil)
	if err != nil {
		return nil, err
	}
	people := make([]client.Person, 0, len(final.Results))
	for _, r := range final.Results {
		people = append(people, api.ToClientPersonWithBridges(r, final.Mutuals, currentUUID))
	}
	return &client.PeopleSearchResult{
		RequestID: final.Id,
		Query:     query,
		Status:    final.Status,
		Completed: final.Status == api.StatusCompleted,
		People:    people,
	}, nil
}
