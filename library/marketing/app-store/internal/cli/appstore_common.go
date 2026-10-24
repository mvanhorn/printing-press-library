// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the hand-built App Store commands: the appstore client
// and store constructors, argument parsing (app refs, platforms, genres,
// storefront lists), dogfood caps, error classification and row printing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// appStoreCLIName names the CLI's data directory.
const appStoreCLIName = "app-store-pp-cli"

// Caps applied under any Printing Press harness (live dogfood or verify), so
// each command finishes inside the harness's per-command budget (30 s for
// the live matrix, 15 s for verify) and a harness pass sends a few real
// requests instead of a full fan-out (contract rule 8). Reads still hit the
// network; only the fan-out width shrinks.
const (
	dogfoodMaxCountries   = 2
	dogfoodMaxReviewPages = 1
	dogfoodMaxChartLimit  = 50
	// How long a harness invocation may wait for send slots, counted from
	// when its client is built. apps.apple.com pages go 20 s apart across
	// runs, more than verify's whole budget, so a slot past the budget exits
	// 7 at once with nothing sent instead of being killed mid-wait; the live
	// matrix still waits for every slot that fits its 30 s.
	verifyMaxPacingWait  = 8 * time.Second
	dogfoodMaxPacingWait = 24 * time.Second
)

// harnessWaitUntil is the App Store client's Options.WaitUntil: the end of
// the active harness's wait budget, counted from now (each command, or each
// MCP tool call in the long-lived server, builds its own client), or the
// zero time outside a harness (waits are then bounded only by --timeout).
func harnessWaitUntil() time.Time {
	switch cliutil.CurrentHarness() {
	case cliutil.HarnessVerify:
		return time.Now().Add(verifyMaxPacingWait)
	case cliutil.HarnessDogfood:
		return time.Now().Add(dogfoodMaxPacingWait)
	}
	return time.Time{}
}

// capForDogfood returns n, or max when running under a Printing Press
// harness and n is larger (n <= 0 means "unbounded" and also becomes max).
func capForDogfood(n, max int) int {
	if !cliutil.IsAnyHarness() {
		return n
	}
	if n <= 0 || n > max {
		return max
	}
	return n
}

// appStoreCountry returns the validated --country storefront code.
func appStoreCountry(flags *rootFlags) (string, error) {
	raw := "us"
	if flags != nil && strings.TrimSpace(flags.country) != "" {
		raw = flags.country
	}
	cc, err := appstore.NormalizeCountry(raw)
	if err != nil {
		return "", usageErr(fmt.Errorf("--country: %w", err))
	}
	return cc, nil
}

// appStoreClientHook is the one test seam for every hand-built App Store
// command: when set, it adjusts the options of each client newAppStoreClient
// builds (after the root flags are applied), so a test can route the real
// commands through a fake Apple transport with a private cache, fast pacing
// and a pinned clock. Production leaves it nil.
var appStoreClientHook func(*appstore.Options)

// newAppStoreClient builds the App Store client from the root flags:
// --country, --lang, --timeout (per request), --no-cache and --rate-limit
// (a ceiling below the per-host defaults, per class and for any two
// requests; it never speeds the client up). The governor keeps its pacing
// slots and throttle holds in <state dir>/appstore-governor.json (inside
// --home when set), so a 429 seen by one invocation holds back the next one
// and pacing carries across runs; pacing waits of 3 s or more are announced
// on stderr unless --quiet. Attach a store with attachCaptureLog to record
// the capture log.
func newAppStoreClient(flags *rootFlags) (*appstore.Client, error) {
	cc, err := appStoreCountry(flags)
	if err != nil {
		return nil, err
	}
	opts := appstore.Options{Country: cc, StatePath: appStoreGovernorPath(), WaitUntil: harnessWaitUntil()}
	if flags != nil {
		opts.Lang = strings.TrimSpace(flags.lang)
		opts.Timeout = flags.timeout
		opts.NoCache = flags.noCache
		if flags.rateLimit > 0 {
			opts.MaxRate = flags.rateLimit
		}
		if !flags.quiet {
			opts.Notice = func(msg string) { _, _ = fmt.Fprintln(os.Stderr, msg) }
		}
	}
	if appStoreClientHook != nil {
		appStoreClientHook(&opts)
	}
	c, err := appstore.New(opts)
	if err != nil {
		return nil, usageErr(err)
	}
	return c, nil
}

// appStoreGovernorPath is the App Store governor's state file in the CLI's
// state directory (resolved per invocation, so --home applies); "" when the
// directory cannot be resolved, which keeps the governor in memory.
func appStoreGovernorPath() string {
	dir, err := cliutil.StateDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	return filepath.Join(dir, appstore.GovernorStateFile)
}

// appStoreDBPath returns the command's --db value when it declares and sets
// one, else the default store path.
func appStoreDBPath(cmd *cobra.Command) string {
	if cmd != nil {
		if f := cmd.Flags().Lookup("db"); f != nil && strings.TrimSpace(f.Value.String()) != "" {
			return f.Value.String()
		}
	}
	return defaultDBPath(appStoreCLIName)
}

// openAppStoreStore opens (creating and migrating if needed) the local
// snapshot store at --db or the default path. Callers must Close it.
func openAppStoreStore(cmd *cobra.Command, flags *rootFlags) (*store.Store, error) {
	_ = flags
	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}
	st, err := store.OpenWithContext(ctx, appStoreDBPath(cmd))
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store: %w", err))
	}
	return st, nil
}

// openAppStoreStoreRead opens the store for a local reader. An existing file
// that is not this CLI's store (another tool's SQLite file, or not SQLite at
// all) is refused before anything touches it, and a store the user made
// read-only is opened read-only: openAppStoreStore would chmod it to 0600,
// switch it to WAL and migrate it. Anything else goes to openAppStoreStore.
func openAppStoreStoreRead(cmd *cobra.Command, flags *rootFlags) (*store.Store, error) {
	path := appStoreDBPath(cmd)
	if _, err := os.Stat(path); err != nil {
		return openAppStoreStore(cmd, flags)
	}
	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}
	ro, err := store.OpenReadOnlyContext(ctx, path)
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store %s read-only: %w", path, err))
	}
	var tables int
	if err := ro.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'captures'`).Scan(&tables); err != nil || tables == 0 {
		_ = ro.Close()
		return nil, usageErr(fmt.Errorf("%s is not an app-store-pp-cli store (no captures table); point --db at a store this CLI wrote", path))
	}
	if f, err := os.OpenFile(path, os.O_WRONLY, 0); err != nil { // #nosec G304 -- the user's own --db store, opened write-only to probe permission and closed unread.
		return ro, nil // read-only file: read it as it is
	} else {
		_ = f.Close()
	}
	_ = ro.Close()
	return openAppStoreStore(cmd, flags)
}

// appStoreMirrorExists reports whether the local store file exists, for the
// local-command missing-mirror guard.
func appStoreMirrorExists(cmd *cobra.Command) (string, bool) {
	path := appStoreDBPath(cmd)
	_, err := os.Stat(path)
	return path, err == nil
}

// attachCaptureLog makes the client append a capture-log row to the store for
// every network request.
func attachCaptureLog(c *appstore.Client, st *store.Store) {
	if c == nil || st == nil {
		return
	}
	c.SetCaptureSink(store.NewCaptureSink(st))
}

// parseAppRef parses an app argument: a numeric id, id123, an apps.apple.com
// URL or a bundle id. Errors are usage errors (exit 2).
func parseAppRef(s string) (appstore.AppRef, error) {
	ref, err := appstore.ParseAppRef(s)
	if err != nil {
		return ref, usageErr(err)
	}
	return ref, nil
}

// refCountry picks the storefront for an app ref: the URL's storefront when
// the ref came from a store URL and --country was not set explicitly,
// otherwise --country.
func refCountry(cmd *cobra.Command, flags *rootFlags, ref appstore.AppRef) (string, error) {
	if ref.Country != "" && !flagChanged(cmd, "country") {
		return ref.Country, nil
	}
	return appStoreCountry(flags)
}

// flagChanged reports whether a local, persistent or inherited flag was set
// on the command line (before or after Cobra merges persistent flags).
func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	for _, fs := range []interface {
		Lookup(string) *pflag.Flag
	}{cmd.Flags(), cmd.PersistentFlags(), cmd.InheritedFlags()} {
		if f := fs.Lookup(name); f != nil && f.Changed {
			return true
		}
	}
	return false
}

// lookupAppRef resolves an app ref to its Lookup result in one storefront.
// A ref with no result is a not-found error (exit 3) that names the
// storefront.
func lookupAppRef(ctx context.Context, c *appstore.Client, ref appstore.AppRef, country string) (appstore.LookupResult, error) {
	q := appstore.LookupQuery{Country: country}
	if ref.ID != "" {
		q.IDs = []string{ref.ID}
	} else {
		q.BundleIDs = []string{ref.BundleID}
	}
	resp, err := c.Lookup(ctx, q)
	if err != nil {
		return appstore.LookupResult{}, classifyAppStoreErr(err)
	}
	if len(resp.Results) == 0 {
		what := ref.ID
		if what == "" {
			what = ref.BundleID
		}
		return appstore.LookupResult{}, notFoundErr(fmt.Errorf("no App Store app %s in storefront %s (check the id, or try another --country)", what, country))
	}
	return resp.Results[0], nil
}

// parsePlatform validates --platform (iphone, ipad, mac, appletv, watch,
// vision; aliases accepted). Errors are usage errors.
func parsePlatform(s string) (string, error) {
	p, err := appstore.NormalizePlatform(s)
	if err != nil {
		return "", usageErr(fmt.Errorf("--platform: %w", err))
	}
	return p, nil
}

// parseChartPlatform is parsePlatform restricted to the platforms with web
// charts (iphone, ipad, mac).
func parseChartPlatform(s string) (string, error) {
	p, err := parsePlatform(s)
	if err != nil {
		return "", err
	}
	for _, ok := range appstore.ChartPlatforms {
		if p == ok {
			return p, nil
		}
	}
	return "", usageErr(fmt.Errorf("--platform: charts exist for iphone, ipad and mac, not %q", s))
}

// resolveGenre resolves --genre (an id such as 6014 or a name/alias such as
// games, puzzle, "developer tools", rpg). Empty means All Apps (36).
func resolveGenre(s string) (appstore.Genre, error) {
	if strings.TrimSpace(s) == "" {
		g, _ := appstore.GenreByID(appstore.AllGenreID)
		return g, nil
	}
	g, err := appstore.ResolveGenre(s)
	if err != nil {
		return appstore.Genre{}, usageErr(fmt.Errorf("--genre: %w", err))
	}
	return g, nil
}

// resolveAgeBand resolves an --age-band flag. An unset or empty flag means
// the overall chart and returns the zero band (ID ""); any other value must
// name a Kids band (usage error otherwise). Only an explicit value such as
// all, kids or 0 selects the All Kids chart (ageBandId 0).
func resolveAgeBand(raw string) (appstore.KidsAgeBand, error) {
	if strings.TrimSpace(raw) == "" {
		return appstore.KidsAgeBand{}, nil
	}
	band, err := appstore.ResolveAgeBand(raw)
	if err != nil {
		return appstore.KidsAgeBand{}, usageErr(fmt.Errorf("--age-band: %w", err))
	}
	return band, nil
}

// checkChartAgeBand rejects a Kids age band on a chart that has none: Kids
// charts are the genre-36 top-free and top-paid charts of the iphone and
// ipad chart pages. A zero band (the overall chart) always passes.
func checkChartAgeBand(band appstore.KidsAgeBand, genre appstore.Genre, platform, chart string) error {
	if band.ID == "" {
		return nil
	}
	switch {
	case chart == "top-grossing":
		return usageErr(fmt.Errorf("--age-band: Kids charts exist for top-free and top-paid only (the grossing RSS feed has no age bands)"))
	case platform == "mac":
		return usageErr(fmt.Errorf("--age-band: Kids charts exist on the iphone and ipad chart pages, not mac"))
	case genre.ID != appstore.AllGenreID:
		return usageErr(fmt.Errorf("--age-band: Kids charts live under --genre 36 (All Apps), not %s %s", genre.ID, genre.Name))
	}
	return nil
}

// ageBandLabel names a stored age band for notes and tables: "" for the
// overall chart, else e.g. "Kids Ages 6-8".
func ageBandLabel(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if b, ok := appstore.KidsAgeBandByID(id); ok {
		if strings.HasPrefix(b.Name, "All Kids") {
			return b.Name
		}
		return "Kids " + b.Name
	}
	return "Kids band " + id
}

// countriesFlag parses a --countries value ("us,gb,pk" or "all"); empty
// means the single --country storefront. Under live dogfood the list is
// capped at dogfoodMaxCountries. Errors are usage errors.
func countriesFlag(flags *rootFlags, raw string) ([]string, error) {
	var list []string
	if strings.TrimSpace(raw) == "" {
		cc, err := appStoreCountry(flags)
		if err != nil {
			return nil, err
		}
		list = []string{cc}
	} else {
		parsed, err := appstore.ParseCountries(raw)
		if err != nil {
			return nil, usageErr(fmt.Errorf("--countries: %w", err))
		}
		list = parsed
	}
	if cliutil.IsAnyHarness() && len(list) > dogfoodMaxCountries {
		list = list[:dogfoodMaxCountries]
	}
	return list, nil
}

// fetchFailure is one failed item of a fan-out command; fan-out commands put
// these in a fetch_failures array and exclude them from aggregates.
type fetchFailure struct {
	ID         string `json:"id,omitempty"`
	Storefront string `json:"storefront,omitempty"`
	Error      string `json:"error"`
}

// warnFetchFailures prints the partial-failure stderr line.
func warnFetchFailures(w io.Writer, failed, total int, what string) {
	if failed <= 0 {
		return
	}
	if what == "" {
		what = "items"
	}
	_, _ = fmt.Fprintf(w, "warning: %d of %d fetches failed; results cover the remaining %d %s\n", failed, total, total-failed, what)
}

// appStoreThrottleError gives a governor stop a clear message while keeping
// *cliutil.RateLimitError reachable through errors.As.
type appStoreThrottleError struct {
	rl *cliutil.RateLimitError
}

func (e *appStoreThrottleError) Error() string {
	body := strings.TrimSpace(e.rl.Body)
	if body == "" {
		body = "Apple throttled the request"
	}
	msg := fmt.Sprintf("rate limited by Apple: %s (url: %s)", body, e.rl.URL)
	if e.rl.RetryAfter > 0 {
		msg += fmt.Sprintf("; retry after %s", e.rl.RetryAfter)
	}
	return msg + "\nhint: wait a few minutes, then retry with fewer storefronts, pages or apps"
}

func (e *appStoreThrottleError) Unwrap() error { return e.rl }

// classifyAppStoreErr maps appstore client errors to typed exit codes:
// throttling (403/429, governor) -> 7, not found -> 3, opt-in surfaces
// without the flag (or a redirect into one) -> 2, everything else -> 5,
// including a 200 whose body is not the surface's documented envelope
// (*appstore.EnvelopeError): that is a failed fetch, never "not found" or
// "verified empty". Already-typed errors pass through unchanged.
func classifyAppStoreErr(err error) error {
	if err == nil {
		return nil
	}
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	if rl, ok := appstore.AsRateLimit(err); ok {
		return rateLimitErr(&appStoreThrottleError{rl: rl})
	}
	// A send slot the run may not wait for is the same "try again later"
	// outcome as a throttle (exit 7), not a failed request: nothing was sent.
	var paced *appstore.PacingError
	if errors.As(err, &paced) {
		return rateLimitErr(fmt.Errorf("%w\nhint: run it again in %s, or raise --timeout so the command can wait for its slot",
			err, paced.Wait.Round(time.Second)))
	}
	if appstore.IsNotFound(err) {
		return notFoundErr(err)
	}
	if errors.Is(err, appstore.ErrOptInRequired) {
		return usageErr(err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apiErr(fmt.Errorf("%w\nhint: the request exceeded --timeout; raise it or narrow the request", err))
	}
	return apiErr(err)
}

// isRateLimited reports whether err is (or wraps) a throttle or a pacing
// refusal; fan-out loops must stop and propagate on it rather than record a
// per-item failure (every later item would be refused the same way).
func isRateLimited(err error) bool {
	if _, ok := appstore.AsRateLimit(err); ok {
		return true
	}
	var paced *appstore.PacingError
	return errors.As(err, &paced)
}

// printRows writes rows (any JSON-marshalable slice or struct) as JSON via
// printJSONFiltered for machine output, or as the generated human table.
func printRows(cmd *cobra.Command, flags *rootFlags, rows any) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, rows, flags)
	}
	items, ok := tableItems(rows)
	if !ok {
		return printJSONFiltered(w, rows, flags)
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "No results.")
		return nil
	}
	return printAutoTable(w, items)
}

// tableItems converts rows to []map[string]any for printAutoTable.
func tableItems(rows any) ([]map[string]any, bool) {
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, false
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, true
	}
	var one map[string]any
	if err := json.Unmarshal(raw, &one); err == nil {
		return []map[string]any{one}, true
	}
	return nil, false
}

// warnStale prints a one-line stderr hint when the newest relevant capture
// a local command read (newest) is older than --max-age at now; --max-age 0
// disables it, and a zero newest (no data) prints nothing (the command's own
// note covers an empty store). Only stderr is written, so JSON output is
// unchanged. refresh names the live command that captures fresh data.
func warnStale(cmd *cobra.Command, flags *rootFlags, newest, now time.Time, what, refresh string) {
	if cmd == nil || flags == nil || flags.maxAge <= 0 || newest.IsZero() {
		return
	}
	age := now.Sub(newest)
	if age <= flags.maxAge {
		return
	}
	msg := fmt.Sprintf("hint: the newest stored %s is %s old (captured %s), older than --max-age %s", what, staleAge(age), appstore.FormatTime(newest), flags.maxAge)
	if refresh != "" {
		msg += "; refresh with: " + refresh
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), msg)
}

// staleAge renders an age compactly: 3d4h, 5h12m or 45m.
func staleAge(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d / (24 * time.Hour))
	hours := int(d % (24 * time.Hour) / time.Hour)
	mins := int(d % time.Hour / time.Minute)
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
