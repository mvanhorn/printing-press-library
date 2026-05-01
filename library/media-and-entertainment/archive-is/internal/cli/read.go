// Paywall-reader commands for archive.is.
//
// This file contains the hand-built GOAT features that the generator doesn't
// produce: read (lookup-before-submit), get (markdown extraction), history
// (timemap parsing), save (force fresh capture), and bulk (rate-limited batch).
//
// Archive.is has no official API. Every endpoint here was verified against the
// live service as of April 2026. See the research brief for endpoint details
// and the Feb 2026 Wikipedia blacklist warning.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Mirrors in fallback order. Archive.ph is the current primary; the rest are
// aliases that share the same backend database. Multiple TLDs exist to dodge
// ISP blocks (Australia, NZ).
var archiveIsMirrors = []string{
	"https://archive.ph",
	"https://archive.md",
	"https://archive.is",
	"https://archive.fo",
	"https://archive.li",
	"https://archive.vn",
}

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 archive-is-pp-cli"

// archiveBackend selects where to look up / submit archives.
type archiveBackend string

const (
	backendArchiveIs archiveBackend = "archive-is"
	backendWayback   archiveBackend = "wayback"
)

// memento is the canonical record for a found or created archive snapshot.
type memento struct {
	OriginalURL string    `json:"original_url"`
	MementoURL  string    `json:"memento_url"`
	CapturedAt  time.Time `json:"captured_at"`
	Mirror      string    `json:"mirror"`
	Backend     string    `json:"backend"`
}

// stdinIsTTY reports whether os.Stdin is attached to a terminal (not a pipe/file).
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// newNoRedirectClient returns an HTTP client that does not follow redirects,
// so we can inspect the Location header from timegate responses.
func newNoRedirectClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// timegateLookup queries archive.ph/timegate/<url> across mirrors until one
// returns a 302 with a Location header. Returns the memento or an error.
func timegateLookup(origURL string, timeout time.Duration) (*memento, error) {
	client := newNoRedirectClient(timeout)

	var lastErr error
	for _, mirror := range archiveIsMirrors {
		req, err := http.NewRequest("GET", mirror+"/timegate/"+origURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode == 302 || resp.StatusCode == 301 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				lastErr = fmt.Errorf("%s: 302 with no Location header", mirror)
				continue
			}
			m := &memento{
				OriginalURL: origURL,
				MementoURL:  loc,
				CapturedAt:  parseArchiveTimestamp(loc),
				Mirror:      mirror,
				Backend:     string(backendArchiveIs),
			}
			return m, nil
		}
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("no archive found")
		}
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("%s: rate limited", mirror)
			continue
		}
		lastErr = fmt.Errorf("%s: HTTP %d", mirror, resp.StatusCode)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all mirrors failed")
	}
	return nil, lastErr
}

// runSubmitCapture is the command-facing wrapper around submitCapture. It wires
// the per-invocation budget, Ctrl-C cancellation, a predictive "watch this URL"
// hint, and a progress ticker for interactive terminals.
//
// Commands should call this instead of submitCapture directly so all four
// affordances ship together. Passing budget == 0 runs with no deadline — the
// caller waits until archive.today responds, the user presses Ctrl-C, or a
// per-request sub-context (180s) trips.
//
// Rationale for the predictive URL: archive.today serves the final memento at
// https://archive.ph/<original-url> the moment the capture finishes, regardless
// of how long the submit HTTP call takes. Telling the user "watch this URL"
// gives them a working escape hatch if the CLI hangs — they can see the capture
// land in their browser even if we never return.
func runSubmitCapture(cmd *cobra.Command, flags *rootFlags, origURL string, anyway bool) (*memento, error) {
	budget := flags.submitTimeout

	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}

	// Trap SIGINT so Ctrl-C aborts an in-flight submit cleanly instead of
	// requiring a second Ctrl-C to escape a blocking HTTP call.
	sigCtx, stopSignals := signal.NotifyContext(parent, os.Interrupt)
	defer stopSignals()

	ctx := sigCtx
	var cancelBudget context.CancelFunc = func() {}
	if budget > 0 {
		ctx, cancelBudget = context.WithTimeout(sigCtx, budget)
	}
	defer cancelBudget()

	start := time.Now()
	if isInteractive(flags) && !flags.quiet {
		predicted := "https://archive.ph/" + origURL
		if budget > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Submitting to archive.today (budget %s). Watch %s while you wait.\n",
				formatBudget(budget), predicted)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Submitting to archive.today (no budget — Ctrl-C to cancel). Watch %s while you wait.\n",
				predicted)
		}
	}

	// Progress ticker: nudge the user every 10s so "silent hang vs working"
	// is visible. Only runs in interactive mode to avoid polluting agent/JSON
	// output streams.
	var tickerDone chan struct{}
	if isInteractive(flags) && !flags.quiet {
		tickerDone = make(chan struct{})
		go submitProgressTicker(cmd.ErrOrStderr(), start, budget, ctx, tickerDone)
	}

	m, err := submitCapture(ctx, origURL, anyway, budget)

	if tickerDone != nil {
		close(tickerDone)
	}

	// If ctx was cancelled and submitCapture returned a plain error (not a
	// *SubmitFailureError), wrap it so the caller still gets a structured
	// failure with BudgetExhausted set. This can happen when the first mirror
	// trips the deadline mid-request and submitCapture bubbles up ctx.Err().
	if err != nil && ctxBudgetExhausted(ctx) {
		if _, ok := err.(*SubmitFailureError); !ok {
			err = &SubmitFailureError{
				BudgetExhausted: true,
				Budget:          budget,
			}
		}
	}
	return m, err
}

// submitProgressTicker writes a "still waiting (X:XX elapsed, Y:YY remaining)"
// line to stderr every 10s while a submit is in flight. Exits when ctx is
// cancelled or done is closed (whichever happens first).
func submitProgressTicker(w io.Writer, start time.Time, budget time.Duration, ctx context.Context, done <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			if budget > 0 {
				remaining := (budget - time.Since(start)).Round(time.Second)
				if remaining < 0 {
					remaining = 0
				}
				fmt.Fprintf(w, "  still waiting (%s elapsed, %s remaining)\n",
					formatElapsed(elapsed), formatElapsed(remaining))
			} else {
				fmt.Fprintf(w, "  still waiting (%s elapsed, no budget)\n",
					formatElapsed(elapsed))
			}
		case <-ctx.Done():
			return
		case <-done:
			return
		}
	}
}

// formatElapsed renders a duration as "M:SS" for the progress ticker.
func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// submitCapture triggers a fresh archive via /submit/?url=<url>. Archive.is
// dedupes by default; pass anyway=true to force a new capture.
//
// Pipeline (units 1 + 2 + 3 + budget/progress):
//  1. Cooldown check (unit 1) — short-circuit if we are in a persisted
//     rate-limit window, no HTTP request made
//  2. Prime cookie jar with a GET to archive.ph/ (unit 2) so archive.today's
//     qki= cookie travels with subsequent submit requests
//  3. Iterate mirrors. On 429, do not rotate — all mirrors share the same
//     backend rate limit. Instead, do exponential backoff with jitter:
//     5s, 15s, 60s between retries on the same mirror
//  4. On backoff exhaustion, persist the cooldown window (unit 1) and return
//     a structured SubmitFailureError (unit 3) that formats well for humans
//  5. On non-429 errors (network, DNS, 5xx), fall through to the next mirror
//
// Cancellation: respects ctx. When ctx is cancelled (either because the caller
// hit the --submit-timeout budget or because the user pressed Ctrl-C), any
// in-flight request is aborted and the function returns a SubmitFailureError
// with BudgetExhausted=true. This is the core fix for "the 5-minute timeout
// never fires" — we no longer trust http.Client.Timeout as the stop signal,
// because archive.today holds connections open past the client timeout.
func submitCapture(ctx context.Context, origURL string, anyway bool, budget time.Duration) (*memento, error) {
	// Unit 1: cooldown short-circuit.
	if err := cooldownError(); err != nil {
		return nil, err
	}

	// Per-request HTTP timeout stays short (180s) so individual attempts don't
	// hang forever even when ctx has no deadline. The budget ceiling is
	// enforced by ctx, not by http.Client.Timeout.
	client := newArchiveHTTPClient(180 * time.Second)
	primeCookieJar(client)

	failure := &SubmitFailureError{Budget: budget}
	var any429 bool
	var cookieMaxAge time.Duration

	for _, mirror := range archiveIsMirrors {
		m, attempts, err := tryMirrorWithBackoff(ctx, client, mirror, origURL, anyway)
		if m != nil {
			m.OriginalURL = origURL
			return m, nil
		}

		// Record per-mirror outcome for the structured error.
		result := MirrorResult{
			URL:      mirror,
			Attempts: attempts,
			Err:      err,
		}
		if herr, ok := err.(*mirrorHTTPError); ok {
			result.HTTPCode = herr.code
			if herr.code == 429 {
				any429 = true
				if d := maxAgeFromCookie(herr.cookies, "qki"); d > cookieMaxAge {
					cookieMaxAge = d
				}
			}
		}
		failure.Attempts = append(failure.Attempts, result)

		// Budget exhausted mid-loop: stop trying more mirrors. Mark the
		// failure as budget-exhausted so classifySubmitError routes it to
		// apiErr (exit 5) instead of rateLimitErr (exit 7).
		if ctxBudgetExhausted(ctx) {
			failure.BudgetExhausted = true
			return nil, failure
		}
	}

	// Unit 1: persist cooldown if any mirror 429'd so future invocations
	// skip the HTTP attempts entirely until the window expires.
	if any429 {
		d := cookieMaxAge
		if d <= 0 {
			d = defaultCooldownDuration
		}
		writeCooldown(d)
		until := time.Now().Add(d)
		failure.Cooldown = &d
		failure.CooldownUntil = &until
	}

	return nil, failure
}

// ctxBudgetExhausted reports whether the context was cancelled due to a
// deadline (the --submit-timeout budget) rather than Ctrl-C. We treat
// DeadlineExceeded as budget exhaustion; a user-cancelled Canceled is rare
// here and we let it propagate as-is.
func ctxBudgetExhausted(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.DeadlineExceeded)
}

// tryMirrorWithBackoff attempts the submit GET against a single mirror with
// exponential backoff on 429 responses. Returns the memento on success, the
// number of attempts made, and the final error on failure.
//
// On network errors (DNS, refused, timeout) it returns immediately without
// retrying — those are mirror-specific failures, and the caller should try the
// next mirror. On 429 it retries the SAME mirror per the backoff schedule,
// because rotating mirrors does not help (they share a backend quota).
//
// Backoff waits are cancellable via ctx: a blocking time.Sleep would make the
// budget unreliable, so we select on a timer and ctx.Done().
func tryMirrorWithBackoff(ctx context.Context, client *http.Client, mirror, origURL string, anyway bool) (*memento, int, error) {
	u := mirror + "/submit/?url=" + url.QueryEscape(origURL)
	if anyway {
		u += "&anyway=1"
	}

	delays := backoffSchedule()
	attempts := 0
	var lastErr error

	for attempt := 0; attempt <= len(delays); attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr == nil {
				lastErr = err
			}
			return nil, attempts, lastErr
		}
		attempts++
		m, err := tryMirrorOnce(ctx, client, mirror, u)
		if m != nil {
			return m, attempts, nil
		}
		lastErr = err

		// Non-429 errors (network, DNS, 5xx): stop retrying this mirror, move
		// on. Let the outer loop try the next mirror.
		herr, isHTTP := err.(*mirrorHTTPError)
		if !isHTTP || herr.code != 429 {
			return nil, attempts, err
		}

		// 429: backoff before retry, unless we are out of attempts.
		if attempt < len(delays) {
			select {
			case <-time.After(delays[attempt]):
			case <-ctx.Done():
				return nil, attempts, ctx.Err()
			}
		}
	}
	return nil, attempts, lastErr
}

// tryMirrorOnce performs a single submit GET against the given mirror URL.
// Returns the memento on success. On failure, returns a typed error (either
// *mirrorHTTPError for HTTP-level failures or a plain error for network/DNS).
//
// The per-request sub-context caps individual attempts at 180s even when the
// parent budget is unbounded. This prevents a single stuck connection from
// consuming the entire budget before the next attempt gets a chance.
func tryMirrorOnce(ctx context.Context, client *http.Client, mirror, submitURL string) (*memento, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "GET", submitURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Success path: memento URL returned via Refresh, Location, or Content-Location.
	var mementoURL string
	if ref := resp.Header.Get("Refresh"); ref != "" {
		if idx := strings.Index(strings.ToLower(ref), "url="); idx != -1 {
			mementoURL = strings.TrimSpace(ref[idx+4:])
		}
	}
	if mementoURL == "" {
		mementoURL = resp.Header.Get("Location")
	}
	if mementoURL == "" && resp.StatusCode == 200 {
		mementoURL = resp.Header.Get("Content-Location")
	}
	if mementoURL != "" {
		return &memento{
			OriginalURL: "",
			MementoURL:  mementoURL,
			CapturedAt:  parseArchiveTimestamp(mementoURL),
			Mirror:      mirror,
			Backend:     string(backendArchiveIs),
		}, nil
	}

	// Failure path: return a typed HTTP error with status, cookies, and mirror.
	return nil, &mirrorHTTPError{
		mirror:  mirror,
		code:    resp.StatusCode,
		cookies: resp.Cookies(),
	}
}

// mirrorHTTPError carries the HTTP-level outcome of a mirror attempt so the
// backoff loop and the outer error formatter can both see the status code and
// any cookies (needed to extract the qki Max-Age for cooldown persistence).
type mirrorHTTPError struct {
	mirror  string
	code    int
	cookies []*http.Cookie
}

func (e *mirrorHTTPError) Error() string {
	if e.code == 429 {
		return fmt.Sprintf("%s: rate limited (HTTP 429)", e.mirror)
	}
	return fmt.Sprintf("%s: HTTP %d", e.mirror, e.code)
}

// waybackLookup queries Wayback Machine's CDX index for the most recent 200-status
// snapshot of the given URL. Falls back across canonicalized URL variants because
// Wayback's URL matching is picky: simonwillison.net/ misses but simonwillison.net
// hits. Returns the canonical "no wayback snapshot available" error when all
// variants miss so callers can distinguish "nothing there" from transient failure.
//
// The CDX index is chronological by default, so we parse the last row as "most
// recent." We request statuscode:200 to skip 3xx/4xx captures that would give the
// user a broken click target.
//
// Why CDX instead of the availability API: empirically the
// /wayback/available?url=<x> endpoint returns {"archived_snapshots":{}} for URLs
// that clearly exist in CDX (BBC News since 1999, Wikipedia Python since 2004).
// The two endpoints use different indexes and the availability one is not kept
// in sync. Switching to CDX flips ~14 test URLs from "miss" to "hit" without
// touching anything else in the pipeline.
func waybackLookup(origURL string, timeout time.Duration) (*memento, error) {
	client := &http.Client{Timeout: timeout}

	variants := waybackURLVariants(origURL)
	var lastErr error
	for _, variant := range variants {
		m, err := waybackCDXQuery(client, variant)
		if err == nil {
			// Preserve the caller-supplied original URL so downstream callers
			// that key on OriginalURL (history, get, tldr) see the same value
			// they passed in, not a canonicalized form.
			m.OriginalURL = origURL
			return m, nil
		}
		lastErr = err
	}
	if lastErr != nil && strings.Contains(lastErr.Error(), "no wayback snapshot available") {
		return nil, lastErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no wayback snapshot available")
}

// waybackURLVariants returns canonicalized variants of a URL to try against the
// CDX index, in order. The first variant is the original (unchanged); subsequent
// variants progressively normalize the URL to work around CDX's picky matching.
// Duplicates are suppressed so e.g. a bare-domain input only produces one query.
func waybackURLVariants(origURL string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	add(origURL)

	// Strip trailing slash.
	trimmed := strings.TrimRight(origURL, "/")
	add(trimmed)

	// Strip scheme + www. CDX accepts scheme-less inputs and treats
	// www.example.com and example.com as distinct, so try the bare form.
	stripped := trimmed
	stripped = strings.TrimPrefix(stripped, "https://")
	stripped = strings.TrimPrefix(stripped, "http://")
	add(stripped)

	noWWW := strings.TrimPrefix(stripped, "www.")
	add(noWWW)

	return out
}

// waybackCDXQuery issues a single CDX request for the given URL and returns the
// most recent 200-status snapshot as a memento. Error "no wayback snapshot
// available" is returned when CDX responds OK but the list is empty.
func waybackCDXQuery(client *http.Client, lookupURL string) (*memento, error) {
	endpoint := "https://web.archive.org/cdx/search/cdx" +
		"?url=" + url.QueryEscape(lookupURL) +
		"&output=json" +
		"&limit=-1" +
		"&filter=statuscode:200" +
		"&fl=timestamp,original"

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("wayback cdx HTTP %d", resp.StatusCode)
	}

	var rows [][]string
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, err
	}
	return parseCDXRows(rows)
}

// parseCDXRows turns a decoded CDX JSON response into a memento. The CDX API
// returns a JSON array of string arrays: the first row is the header
// (["timestamp","original"]), subsequent rows are data. An empty result is a
// bare "[]" — no header row at all. limit=-1 with default sort puts the newest
// capture at the end, so we pick rows[len-1].
func parseCDXRows(rows [][]string) (*memento, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("no wayback snapshot available")
	}
	last := rows[len(rows)-1]
	if len(last) < 2 {
		return nil, fmt.Errorf("no wayback snapshot available")
	}
	timestamp := last[0]
	original := last[1]
	if timestamp == "" || original == "" {
		return nil, fmt.Errorf("no wayback snapshot available")
	}

	mementoURL := "https://web.archive.org/web/" + timestamp + "/" + original
	t, _ := time.Parse("20060102150405", timestamp)
	return &memento{
		OriginalURL: original,
		MementoURL:  mementoURL,
		CapturedAt:  t,
		Mirror:      "web.archive.org",
		Backend:     string(backendWayback),
	}, nil
}

// fetchMementoBody fetches the HTML body of a memento URL. Returns the body
// as a byte slice on success. Does not detect CAPTCHAs — the caller should
// check via isCaptchaResponse.
func fetchMementoBody(mementoURL string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", mementoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// isCaptchaResponse returns true if the given HTML body looks like an
// archive.is CAPTCHA challenge page rather than a real archived document.
func isCaptchaResponse(body []byte) bool {
	s := string(body)
	if len(s) == 0 {
		return true
	}
	// archive.is CAPTCHA page markers
	markers := []string{
		"Please complete the security check",
		"Completing the CAPTCHA proves you are a human",
		"One more step",
		"challenge-form",
		"cf-challenge",
	}
	for _, marker := range markers {
		if strings.Contains(s, marker) {
			return true
		}
	}
	// A real archived document should be > 2KB — CAPTCHA pages are small.
	// Skip this heuristic for obvious short pages like example.com.
	return false
}

// parseArchiveTimestamp extracts the 14-digit YYYYMMDDHHMMSS timestamp from
// an archive.ph or web.archive.org memento URL. Returns zero time on no match.
var archiveTimestampRE = regexp.MustCompile(`/(\d{14})/`)

func parseArchiveTimestamp(mementoURL string) time.Time {
	m := archiveTimestampRE.FindStringSubmatch(mementoURL)
	if len(m) < 2 {
		return time.Time{}
	}
	t, err := time.Parse("20060102150405", m[1])
	if err != nil {
		return time.Time{}
	}
	return t
}

// copyToClipboard copies text to the system clipboard. Best-effort — silently
// returns nil on platforms without a clipboard tool.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, then xsel, then wl-copy (wayland).
		for _, try := range []string{"xclip", "xsel", "wl-copy"} {
			if _, err := exec.LookPath(try); err == nil {
				switch try {
				case "xclip":
					cmd = exec.Command("xclip", "-selection", "clipboard")
				case "xsel":
					cmd = exec.Command("xsel", "--clipboard", "--input")
				case "wl-copy":
					cmd = exec.Command("wl-copy")
				}
				break
			}
		}
	case "windows":
		cmd = exec.Command("clip")
	}
	if cmd == nil {
		return nil
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// newReadCmd is the hero command: find existing archive or create one.
// Lookup-before-submit: timegate first (fast, no rate limit), submit only on miss.
func newReadCmd(flags *rootFlags) *cobra.Command {
	var backend string
	var noClipboard bool
	var copyText bool

	cmd := &cobra.Command{
		Use:   "read <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Find or create an archive of a paywalled URL (the hero command)",
		Long: "Find an existing archive.today snapshot for the URL, or create one if none exists.\n\n" +
			"Tries timegate lookup first (fast — usually under 500ms). Only submits a fresh capture\n" +
			"if no recent snapshot is found, which saves 30-120 seconds per hit and avoids rate limits.\n\n" +
			"Copies the resulting memento URL to the clipboard by default. When run in an interactive\n" +
			"terminal, prompts to open the archive in your browser. Use --no-prompt to skip the prompt,\n" +
			"or --copy-text to copy extracted article text instead of the URL.",
		Example: "  archive-is-pp-cli read https://www.nytimes.com/2026/04/10/example-article\n" +
			"  archive-is-pp-cli read https://www.wsj.com/articles/abc --backend archive-is,wayback\n" +
			"  archive-is-pp-cli read https://ft.com/content/xyz --json\n" +
			"  archive-is-pp-cli read https://ft.com/content/xyz --copy-text",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: GET https://archive.ph/timegate/%s\n", origURL)
				return nil
			}
			if !strings.HasPrefix(origURL, "http://") && !strings.HasPrefix(origURL, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https://"))
			}

			backends := parseBackends(backend)
			timeout := 30 * time.Second
			if flags.timeout > 0 {
				timeout = flags.timeout
			}

			var m *memento
			var lookupErr error
			var usedBackend archiveBackend
			for _, b := range backends {
				switch b {
				case backendArchiveIs:
					m, lookupErr = timegateLookup(origURL, timeout)
				case backendWayback:
					m, lookupErr = waybackLookup(origURL, timeout)
				}
				if m != nil {
					usedBackend = b
					break
				}
			}

			// No existing snapshot — submit one (archive.is only; Wayback submit requires auth)
			if m == nil {
				if !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "No existing archive found (%v). Submitting fresh capture — this can take 30-120 seconds...\n", lookupErr)
				}
				var err error
				m, err = runSubmitCapture(cmd, flags, origURL, false)
				if err != nil {
					return classifySubmitError(err)
				}
				usedBackend = backendArchiveIs
			}

			// Warn if we fell back to Wayback for a known hard-paywall domain — those
			// sites typically return teaser-only snapshots on Wayback.
			if usedBackend == backendWayback && isHardPaywallDomain(origURL) && !flags.quiet && !flags.asJSON {
				fmt.Fprint(cmd.ErrOrStderr(), paywallWarning(origURL))
			}

			// Unit 8: detect silent redirects — archive.today snapshots that are
			// actually bot-wall captures being served as redirects to the homepage.
			if usedBackend == backendArchiveIs && !flags.quiet && !flags.asJSON {
				if warning := detectSilentRedirect(origURL, m.MementoURL); warning != "" {
					fmt.Fprint(cmd.ErrOrStderr(), warning)
				}
			}

			// Normalize http → https on the memento URL (archive.is often returns http)
			m.MementoURL = strings.Replace(m.MementoURL, "http://archive.", "https://archive.", 1)

			// Clipboard: either the URL (default) or the extracted article text (--copy-text)
			clipboardPayload := m.MementoURL
			if copyText && !noClipboard && !flags.quiet {
				body, bodyErr := fetchMementoBody(m.MementoURL, 30*time.Second)
				if bodyErr == nil && !isCaptchaResponse(body) {
					clipboardPayload = extractReadableText(string(body))
				} else if !flags.quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "  (--copy-text: body fetch failed, falling back to URL)")
				}
			}
			if !noClipboard && !flags.quiet {
				_ = copyToClipboard(clipboardPayload)
			}

			if err := renderMemento(cmd, flags, m); err != nil {
				return err
			}
			maybeEmitHints(cmd, flags, m)
			return maybePromptOpen(cmd, flags, m)
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "archive-is,wayback", "Backends to try in order (archive-is, wayback)")
	cmd.Flags().BoolVar(&noClipboard, "no-clipboard", false, "Skip copying the result to the clipboard")
	cmd.Flags().BoolVar(&copyText, "copy-text", false, "Copy extracted article text to clipboard instead of the URL")
	return cmd
}

// newGetCmd fetches the archived page and extracts clean text/markdown.
// This is the killer feature for LLM piping.
func newGetCmd(flags *rootFlags) *cobra.Command {
	var backend string
	var rawHTML bool

	cmd := &cobra.Command{
		Use:   "get <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch the full archived article text as clean markdown",
		Long: "Find or create an archive for the URL, then fetch the memento HTML and extract\n" +
			"clean readable text. Perfect for piping into LLMs, notes, or terminal reading.\n\n" +
			"Use --raw to get the untouched HTML instead of extracted markdown.",
		Example: "  archive-is-pp-cli get https://www.nytimes.com/2026/04/10/article | claude\n" +
			"  archive-is-pp-cli get https://ft.com/content/xyz --raw > article.html",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would fetch archive body for %s\n", origURL)
				return nil
			}
			if !strings.HasPrefix(origURL, "http://") && !strings.HasPrefix(origURL, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https://"))
			}

			backends := parseBackends(backend)
			timeout := 30 * time.Second
			if flags.timeout > 0 {
				timeout = flags.timeout
			}

			var m *memento
			var err error
			for _, b := range backends {
				switch b {
				case backendArchiveIs:
					m, err = timegateLookup(origURL, timeout)
				case backendWayback:
					m, err = waybackLookup(origURL, timeout)
				}
				if m != nil {
					break
				}
			}
			if m == nil {
				if !flags.quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "No existing archive. Submitting fresh capture...")
				}
				m, err = runSubmitCapture(cmd, flags, origURL, false)
				if err != nil {
					return classifySubmitError(err)
				}
			}

			// Fetch the memento HTML. archive.is often serves CAPTCHAs for direct body
			// fetches — detect the CAPTCHA page and fall back to Wayback Machine.
			body, fetchErr := fetchMementoBody(m.MementoURL, 30*time.Second)
			if fetchErr != nil || isCaptchaResponse(body) {
				if !flags.quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "archive.is returned a CAPTCHA for body fetch, falling back to Wayback Machine...")
				}
				waybackMemento, waybackErr := waybackLookup(origURL, timeout)
				if waybackErr != nil || waybackMemento == nil {
					return apiErr(fmt.Errorf("archive.is CAPTCHA and Wayback lookup failed: %w", waybackErr))
				}
				body, fetchErr = fetchMementoBody(waybackMemento.MementoURL, 30*time.Second)
				if fetchErr != nil {
					return apiErr(fmt.Errorf("fetching wayback memento: %w", fetchErr))
				}
				m = waybackMemento
				// Warn if this is a known hard-paywall domain — Wayback body is likely teaser-only.
				if isHardPaywallDomain(origURL) && !flags.quiet && !flags.asJSON {
					fmt.Fprint(cmd.ErrOrStderr(), paywallWarning(origURL))
				}
			}

			if rawHTML {
				fmt.Fprint(cmd.OutOrStdout(), string(body))
				return nil
			}

			// Extract clean text. This is a simple heuristic — strip scripts, styles, tags,
			// and collapse whitespace. Good enough for reading paywalled articles.
			text := extractReadableText(string(body))
			fmt.Fprint(cmd.OutOrStdout(), text)
			if !strings.HasSuffix(text, "\n") {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "archive-is,wayback", "Backends to try in order")
	cmd.Flags().BoolVar(&rawHTML, "raw", false, "Output raw HTML instead of extracted text")
	return cmd
}

// newHistoryCmd lists all known snapshots of a URL via /timemap/.
func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "history <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "List all known archive snapshots for a URL, oldest to newest",
		Long:    "Queries the Memento timemap endpoint and returns every snapshot ever taken of the URL, with timestamps.",
		Example: "  archive-is-pp-cli history https://www.nytimes.com/\n  archive-is-pp-cli history https://example.com --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: GET https://archive.ph/timemap/%s\n", origURL)
				return nil
			}

			client := &http.Client{Timeout: 60 * time.Second}
			req, err := http.NewRequest("GET", "https://archive.ph/timemap/"+origURL, nil)
			if err != nil {
				return apiErr(err)
			}
			req.Header.Set("User-Agent", userAgent)

			resp, err := client.Do(req)
			if err != nil {
				return apiErr(fmt.Errorf("timemap request failed: %w", err))
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return apiErr(fmt.Errorf("timemap HTTP %d", resp.StatusCode))
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return apiErr(err)
			}

			snapshots := parseTimeMap(string(body))
			if len(snapshots) == 0 {
				return notFoundErr(fmt.Errorf("no snapshots found for %s", origURL))
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"original_url": origURL,
					"count":        len(snapshots),
					"snapshots":    snapshots,
				})
			}

			for _, s := range snapshots {
				if flags.quiet {
					fmt.Fprintln(cmd.OutOrStdout(), s.MementoURL)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", s.CapturedAt.Format("2006-01-02 15:04"), s.MementoURL)
				}
			}
			return nil
		},
	}
	return cmd
}

// newSaveCmd forces a fresh archive capture via /submit/?url=<url>&anyway=1.
func newSaveCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "save <url>",
		Short: "Archive a URL (submit a new capture to archive.today)",
		Long: "Submits a URL for archiving. Slower than 'read' (30-120 seconds) but guarantees\n" +
			"a fresh capture. Use this when you want to preserve the current version before\n" +
			"it changes or gets taken down.",
		Example: "  archive-is-pp-cli save https://example.com/article\n" +
			"  archive-is-pp-cli save https://example.com --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: GET https://archive.ph/submit/?url=%s&anyway=%t\n", origURL, force)
				return nil
			}
			if !strings.HasPrefix(origURL, "http://") && !strings.HasPrefix(origURL, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https://"))
			}
			m, err := runSubmitCapture(cmd, flags, origURL, force)
			if err != nil {
				return classifySubmitError(err)
			}
			m.MementoURL = strings.Replace(m.MementoURL, "http://archive.", "https://archive.", 1)
			if !flags.quiet {
				_ = copyToClipboard(m.MementoURL)
			}
			if err := renderMemento(cmd, flags, m); err != nil {
				return err
			}
			maybeEmitHints(cmd, flags, m)
			return maybePromptOpen(cmd, flags, m)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force a fresh capture even if a recent snapshot exists")
	return cmd
}

// newBulkCmd archives many URLs with built-in rate limiting.
func newBulkCmd(flags *rootFlags) *cobra.Command {
	var delay time.Duration
	var backend string
	cmd := &cobra.Command{
		Use:   "bulk [file]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Archive a list of URLs with built-in rate limiting",
		Long: "Reads one URL per line from a file (or stdin if no file given) and runs\n" +
			"'read' on each with a delay between requests to avoid rate limits.\n" +
			"Lines starting with # are treated as comments.",
		Example: "  archive-is-pp-cli bulk urls.txt\n" +
			"  cat links.md | archive-is-pp-cli bulk -\n" +
			"  archive-is-pp-cli bulk urls.txt --delay 15s",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// When no file arg and stdin is a TTY (not piped), show help.
			if len(args) == 0 && stdinIsTTY() {
				return cmd.Help()
			}
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN: would archive URLs from input")
				return nil
			}
			var src io.Reader
			if len(args) == 0 || args[0] == "-" {
				src = cmd.InOrStdin()
			} else {
				f, err := os.Open(args[0])
				if err != nil {
					return configErr(err)
				}
				defer f.Close()
				src = f
			}

			scanner := bufio.NewScanner(src)
			var results []*memento
			var failures []string
			timeout := 30 * time.Second

			first := true
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
					failures = append(failures, line+" (not a URL)")
					continue
				}
				if !first {
					time.Sleep(delay)
				}
				first = false

				if !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "→ %s\n", line)
				}

				backends := parseBackends(backend)
				var m *memento
				for _, b := range backends {
					switch b {
					case backendArchiveIs:
						m, _ = timegateLookup(line, timeout)
					case backendWayback:
						m, _ = waybackLookup(line, timeout)
					}
					if m != nil {
						break
					}
				}
				if m == nil {
					// Skip submit for bulk — too slow, would take an hour
					failures = append(failures, line)
					continue
				}
				results = append(results, m)
				if !flags.quiet {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", m.MementoURL)
				}
			}
			if err := scanner.Err(); err != nil {
				return apiErr(err)
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"succeeded":   len(results),
					"failed":      len(failures),
					"results":     results,
					"failed_urls": failures,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nArchived: %d  Failed: %d\n", len(results), len(failures))
			for _, f := range failures {
				fmt.Fprintf(cmd.OutOrStdout(), "  FAIL: %s\n", f)
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&delay, "delay", 10*time.Second, "Delay between requests (default 10s to avoid rate limits)")
	cmd.Flags().StringVar(&backend, "backend", "archive-is,wayback", "Backends to try in order")
	return cmd
}

// newRequestCmd submits an archive request and optionally waits for it to
// become available. This is the "request archive, tell me when it's ready"
// workflow — useful when the URL is not yet archived and you want to fire
// the request, then check back later (or wait in-process).
func newRequestCmd(flags *rootFlags) *cobra.Command {
	var wait bool
	var waitTimeout time.Duration
	var pollInterval time.Duration
	var force bool

	cmd := &cobra.Command{
		Use:   "request <url>",
		Short: "Request a new archive and optionally wait for it to be ready",
		Long: "Submits a URL to archive.today for capture. By default returns immediately\n" +
			"after firing the request — the capture continues on archive.today's side.\n" +
			"Use --wait to poll the timegate endpoint until the snapshot is available.\n\n" +
			"Perfect for LLM workflows: 'archive this URL and tell me when it's ready.'\n" +
			"With --wait and --json, an agent can submit, poll, and know exactly when\n" +
			"the memento URL is live.",
		Example: "  archive-is-pp-cli request https://example.com/article\n" +
			"  archive-is-pp-cli request https://example.com/article --wait\n" +
			"  archive-is-pp-cli request https://example.com/article --wait --wait-timeout 3m --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would submit %s to archive.today\n", origURL)
				return nil
			}
			if !strings.HasPrefix(origURL, "http://") && !strings.HasPrefix(origURL, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https://"))
			}

			// Check first — if it already exists and the user didn't pass --force,
			// return immediately (saves the user's time and avoids hitting rate limits).
			if !force {
				if existing, err := timegateLookup(origURL, 30*time.Second); err == nil && existing != nil {
					if !flags.quiet {
						fmt.Fprintln(cmd.ErrOrStderr(), "Archive already exists — returning existing snapshot.")
						fmt.Fprintln(cmd.ErrOrStderr(), "Use --force to submit a fresh capture anyway.")
					}
					return renderRequestResult(cmd, flags, origURL, existing, "existing", 0)
				}
			}

			// Unit 4: Persist a PENDING record so `request check` knows this URL
			// has a submit in flight. The goroutine updates the record when it
			// completes (success or error).
			recordPending(origURL)

			// Fire the submit in a goroutine so we can immediately report "submitted"
			// and then either return (fire-and-forget) or poll timegate in the foreground.
			//
			// The background submit gets its own context with the flag-configured
			// budget so the goroutine doesn't outlive the caller's deadline. We
			// deliberately skip the progress ticker here: `request` is fire-and-forget
			// and the foreground path prints its own status.
			submitResult := make(chan *memento, 1)
			submitErr := make(chan error, 1)
			bgBudget := flags.submitTimeout
			bgParent := cmd.Context()
			if bgParent == nil {
				bgParent = context.Background()
			}
			bgCtx := bgParent
			var bgCancel context.CancelFunc = func() {}
			if bgBudget > 0 {
				bgCtx, bgCancel = context.WithTimeout(bgParent, bgBudget)
			}
			go func() {
				defer bgCancel()
				m, err := submitCapture(bgCtx, origURL, force, bgBudget)
				if err != nil {
					recordFailed(origURL, err)
					submitErr <- err
					return
				}
				recordReady(origURL, m.MementoURL)
				submitResult <- m
			}()

			if !flags.quiet {
				fmt.Fprintf(cmd.ErrOrStderr(), "Request submitted to archive.today. Typical capture time: 30-120 seconds.\n")
			}

			// Fire-and-forget mode: report pending and exit.
			if !wait {
				if !flags.quiet {
					fmt.Fprintln(cmd.ErrOrStderr(), "Not waiting. Check status with:")
					fmt.Fprintf(cmd.ErrOrStderr(), "  archive-is-pp-cli request check %q\n", origURL)
					fmt.Fprintln(cmd.ErrOrStderr(), "Or wait in-process:")
					fmt.Fprintf(cmd.ErrOrStderr(), "  archive-is-pp-cli request %q --wait\n", origURL)
				}
				pending := &memento{
					OriginalURL: origURL,
					MementoURL:  "",
					Mirror:      "",
					Backend:     string(backendArchiveIs),
				}
				return renderRequestResult(cmd, flags, origURL, pending, "pending", 0)
			}

			// Wait mode: poll timegate until the snapshot appears or we time out.
			start := time.Now()
			deadline := start.Add(waitTimeout)
			attempts := 0

			if !flags.quiet {
				fmt.Fprintf(cmd.ErrOrStderr(), "Waiting up to %s (polling every %s)...\n", waitTimeout, pollInterval)
			}

			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()

			for {
				// Check if the background submit already returned — if it did, use its result.
				select {
				case m := <-submitResult:
					if !flags.quiet {
						fmt.Fprintf(cmd.ErrOrStderr(), "Capture complete in %s\n", time.Since(start).Round(time.Second))
					}
					return renderRequestResult(cmd, flags, origURL, m, "ready", time.Since(start))
				case err := <-submitErr:
					return apiErr(fmt.Errorf("submit failed: %w", err))
				default:
				}

				// Poll timegate to see if the snapshot is live yet (archive.is sometimes
				// publishes before submit returns).
				attempts++
				if m, err := timegateLookup(origURL, 10*time.Second); err == nil && m != nil && !m.CapturedAt.IsZero() && time.Since(m.CapturedAt) < 5*time.Minute {
					if !flags.quiet {
						fmt.Fprintf(cmd.ErrOrStderr(), "Archive ready (poll #%d, elapsed %s)\n", attempts, time.Since(start).Round(time.Second))
					}
					return renderRequestResult(cmd, flags, origURL, m, "ready", time.Since(start))
				}

				if time.Now().After(deadline) {
					if !flags.quiet {
						fmt.Fprintf(cmd.ErrOrStderr(), "Timed out after %s. The capture may still complete on archive.today's side.\n", waitTimeout)
						fmt.Fprintf(cmd.ErrOrStderr(), "Try: archive-is-pp-cli request check %q\n", origURL)
					}
					pending := &memento{OriginalURL: origURL, Backend: string(backendArchiveIs)}
					return renderRequestResult(cmd, flags, origURL, pending, "timeout", time.Since(start))
				}

				if !flags.quiet && attempts%3 == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "  still waiting... (%s elapsed)\n", time.Since(start).Round(time.Second))
				}

				select {
				case <-ticker.C:
				case m := <-submitResult:
					if !flags.quiet {
						fmt.Fprintf(cmd.ErrOrStderr(), "Capture complete in %s\n", time.Since(start).Round(time.Second))
					}
					return renderRequestResult(cmd, flags, origURL, m, "ready", time.Since(start))
				case err := <-submitErr:
					return apiErr(fmt.Errorf("submit failed: %w", err))
				}
			}
		},
	}

	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the archive to be ready before returning")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 3*time.Minute, "Max time to wait when --wait is set")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 10*time.Second, "How often to poll timegate when --wait is set")
	cmd.Flags().BoolVar(&force, "force", false, "Force fresh capture even if a recent snapshot exists")

	// Subcommand: request check — consults the state file first, then polls
	// timegate as a fallback. The state file tells us whether a previous
	// `request <url>` call's goroutine completed with success, failure, or is
	// still running.
	checkCmd := &cobra.Command{
		Use:   "check <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Check whether an archive request has completed",
		Long:  "Consults the local request state file first; if the URL has a known terminal\nstate (ready or failed), reports it immediately. Otherwise falls back to a\ntimegate lookup.\n\nUse after 'request <url>' without --wait to check if the capture finished.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: check state for %s\n", origURL)
				return nil
			}

			// Unit 4: check persisted state first.
			if rec := lookupRequest(origURL); rec != nil && !isStalePending(rec) {
				switch rec.Status {
				case requestStatusReady:
					m := &memento{
						OriginalURL: origURL,
						MementoURL:  rec.MementoURL,
						CapturedAt:  rec.CompletedAt,
						Backend:     string(backendArchiveIs),
					}
					return renderRequestResult(cmd, flags, origURL, m, "ready", 0)
				case requestStatusFailed:
					// Report the failure cleanly and return exit code 5.
					if !flags.asJSON {
						fmt.Fprintf(cmd.OutOrStdout(), "FAILED: request for %s did not complete\n", origURL)
						fmt.Fprintf(cmd.OutOrStdout(), "  reason: %s\n", rec.Error)
						fmt.Fprintln(cmd.OutOrStdout(), "  Use `archive-is-pp-cli request <url>` to retry.")
						return apiErr(fmt.Errorf("%s", rec.Error))
					}
					// JSON path
					failed := &memento{OriginalURL: origURL, Backend: string(backendArchiveIs)}
					if err := renderRequestResult(cmd, flags, origURL, failed, "failed", 0); err != nil {
						return err
					}
					return apiErr(fmt.Errorf("%s", rec.Error))
				case requestStatusPending:
					// Still pending — report the pending state from the file, also poll
					// timegate as a second-chance check in case the server published
					// while our worker was still running.
					if m, err := timegateLookup(origURL, 30*time.Second); err == nil && m != nil && time.Since(m.CapturedAt) < 10*time.Minute {
						recordReady(origURL, m.MementoURL)
						return renderRequestResult(cmd, flags, origURL, m, "ready", 0)
					}
					pending := &memento{OriginalURL: origURL, Backend: string(backendArchiveIs)}
					return renderRequestResult(cmd, flags, origURL, pending, "pending", 0)
				}
			}

			// No state file entry (or stale pending) — fall back to timegate poll.
			m, err := timegateLookup(origURL, 30*time.Second)
			if err != nil || m == nil {
				pending := &memento{OriginalURL: origURL, Backend: string(backendArchiveIs)}
				return renderRequestResult(cmd, flags, origURL, pending, "pending", 0)
			}
			status := "ready"
			if !m.CapturedAt.IsZero() && time.Since(m.CapturedAt) > 1*time.Hour {
				// Old snapshot — this is "existing" not freshly captured.
				status = "existing"
			}
			return renderRequestResult(cmd, flags, origURL, m, status, 0)
		},
	}
	cmd.AddCommand(checkCmd)

	return cmd
}

// renderRequestResult outputs the status of a request in the appropriate format.
// Statuses: "pending" (submitted, not yet captured), "ready" (capture complete),
// "existing" (snapshot already existed), "timeout" (wait gave up).
func renderRequestResult(cmd *cobra.Command, flags *rootFlags, origURL string, m *memento, status string, elapsed time.Duration) error {
	if flags.asJSON {
		out := map[string]any{
			"original_url": origURL,
			"status":       status,
			"memento_url":  m.MementoURL,
			"mirror":       m.Mirror,
			"backend":      m.Backend,
		}
		if !m.CapturedAt.IsZero() {
			out["captured_at"] = m.CapturedAt.Format(time.RFC3339)
		}
		if elapsed > 0 {
			out["elapsed_seconds"] = int(elapsed.Seconds())
		}
		// Unit 3: include next_actions for terminal states (ready/existing)
		// that have a memento URL to act on. Pending/failed/timeout skip
		// because there's no URL to open or summarize yet. --quiet wins.
		if !flags.quiet && (status == "ready" || status == "existing") {
			// Ensure OriginalURL is populated so hints target the source, not the archive.
			if m.OriginalURL == "" {
				m.OriginalURL = origURL
			}
			if actions := agentHintsFor(m); len(actions) > 0 {
				out["next_actions"] = actions
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if flags.quiet {
		if m.MementoURL != "" {
			fmt.Fprintln(cmd.OutOrStdout(), m.MementoURL)
		}
		return nil
	}
	switch status {
	case "ready":
		fmt.Fprintf(cmd.OutOrStdout(), "READY: %s\n", m.MementoURL)
		maybeEmitHints(cmd, flags, m)
		if err := maybePromptOpen(cmd, flags, m); err != nil {
			return err
		}
	case "existing":
		fmt.Fprintf(cmd.OutOrStdout(), "EXISTS: %s\n", m.MementoURL)
		maybeEmitHints(cmd, flags, m)
		if err := maybePromptOpen(cmd, flags, m); err != nil {
			return err
		}
	case "pending":
		fmt.Fprintf(cmd.OutOrStdout(), "PENDING: request sent for %s\n", origURL)
		fmt.Fprintln(cmd.OutOrStdout(), "  Run with --wait to block until ready, or use 'request check <url>' later.")
	case "timeout":
		fmt.Fprintf(cmd.OutOrStdout(), "TIMEOUT: request for %s did not complete in time\n", origURL)
	}
	return nil
}

// maybeEmitHints writes the agent-facing NEXT: hint block to stderr when the
// CLI is running in non-interactive mode (agent caller, piped stdout, etc.)
// and --quiet / --json are not set.
//
// The gating rules:
//   - Interactive TTY            → no hints (the user sees the prompt menu)
//   - --quiet                    → no hints (minimal output is the contract)
//   - --json                     → no hints on stderr (they move into JSON per Unit 3)
//   - Otherwise non-interactive  → emit NEXT: lines on stderr
//
// This is the mirror image of maybePromptOpen: exactly one of the two fires
// for any given completion (or neither, in quiet/json cases).
func maybeEmitHints(cmd *cobra.Command, flags *rootFlags, m *memento) {
	if m == nil || m.MementoURL == "" {
		return
	}
	if isInteractive(flags) {
		return
	}
	if flags.quiet || flags.asJSON {
		return
	}
	writeAgentHints(cmd.ErrOrStderr(), agentHintsFor(m))
}

// maybePromptOpen shows the post-action menu when the CLI is running in an
// interactive terminal. Offers: open in browser (default), tl;dr, read full
// text here, quit. No-op when non-interactive (--json, --quiet, --agent,
// --no-prompt, piped output).
//
// This is the only code path that launches a browser or runs tldr from the
// post-action flow. Nothing else in the CLI auto-opens — it was an explicit
// user correction during dogfooding.
func maybePromptOpen(cmd *cobra.Command, flags *rootFlags, m *memento) error {
	if m == nil || m.MementoURL == "" {
		return nil
	}
	if !isInteractive(flags) {
		return nil
	}

	action := promptMenu(cmd.ErrOrStderr())
	switch action {
	case menuActionOpen:
		if err := openInBrowser(m.MementoURL); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  (could not launch browser: %v)\n", err)
			return nil
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "  opening in browser...")
	case menuActionTldr:
		// Re-use the tldr pipeline. Use the original URL (not the memento URL)
		// so the tldr cache path matches what read produced.
		summary, _, err := runTldr(cmd, flags, m.OriginalURL, "archive-is,wayback")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  tl;dr failed: %v\n", err)
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout())
		if summary.Headline != "" {
			fmt.Fprintln(cmd.OutOrStdout(), summary.Headline)
			fmt.Fprintln(cmd.OutOrStdout())
		}
		for _, b := range summary.Bullets {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", b)
		}
	case menuActionReadHere:
		// Fetch the body and extract clean text — mirrors `get` behavior.
		body, fetchErr := fetchMementoBody(m.MementoURL, 30*time.Second)
		if fetchErr != nil || isCaptchaResponse(body) {
			waybackMemento, waybackErr := waybackLookup(m.OriginalURL, 30*time.Second)
			if waybackErr != nil || waybackMemento == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  read-here failed: could not fetch body\n")
				return nil
			}
			body, fetchErr = fetchMementoBody(waybackMemento.MementoURL, 30*time.Second)
			if fetchErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  read-here failed: %v\n", fetchErr)
				return nil
			}
		}
		text := extractReadableText(string(body))
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), text)
	case menuActionQuit, menuActionNone:
		// No action. Command exits cleanly.
	}
	return nil
}

// renderMemento prints a memento record respecting --json, --quiet, --select, etc.
func renderMemento(cmd *cobra.Command, flags *rootFlags, m *memento) error {
	if flags.asJSON {
		// Unit 3: include next_actions in the JSON payload when we would have
		// emitted stderr hints in non-JSON mode. --quiet still suppresses.
		out := map[string]any{
			"original_url": m.OriginalURL,
			"memento_url":  m.MementoURL,
			"mirror":       m.Mirror,
			"backend":      m.Backend,
		}
		if !m.CapturedAt.IsZero() {
			out["captured_at"] = m.CapturedAt.Format(time.RFC3339)
		}
		if !flags.quiet {
			if actions := agentHintsFor(m); len(actions) > 0 {
				out["next_actions"] = actions
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if flags.quiet {
		fmt.Fprintln(cmd.OutOrStdout(), m.MementoURL)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), m.MementoURL)
	if !m.CapturedAt.IsZero() {
		fmt.Fprintf(cmd.ErrOrStderr(), "  captured: %s\n", m.CapturedAt.Format("2006-01-02 15:04:05"))
	}
	if m.Mirror != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "  mirror:   %s\n", m.Mirror)
	}
	if m.Backend != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "  backend:  %s\n", m.Backend)
	}
	return nil
}

// parseBackends parses a comma-separated backend list. Falls back to archive-is.
func parseBackends(s string) []archiveBackend {
	if s == "" {
		return []archiveBackend{backendArchiveIs}
	}
	var out []archiveBackend
	for _, part := range strings.Split(s, ",") {
		switch strings.TrimSpace(part) {
		case "archive-is", "archive.is", "archive-today", "archive.today":
			out = append(out, backendArchiveIs)
		case "wayback", "web.archive.org":
			out = append(out, backendWayback)
		}
	}
	if len(out) == 0 {
		return []archiveBackend{backendArchiveIs}
	}
	return out
}

// parseTimeMap parses the Memento application/link-format response.
// Lines look like: <http://archive.md/20260409150558/https://example.com/>; rel="memento"; datetime="Thu, 09 Apr 2026 15:05:58 GMT",
var timemapLineRE = regexp.MustCompile(`<([^>]+)>[^,]*rel="([^"]+)"[^,]*datetime="([^"]+)"`)

func parseTimeMap(body string) []*memento {
	var snapshots []*memento
	// The response is one giant line or many lines; split by `,` between entries.
	entries := strings.Split(body, ",\n")
	if len(entries) == 1 {
		entries = strings.Split(body, ",")
	}
	for _, entry := range entries {
		m := timemapLineRE.FindStringSubmatch(entry)
		if len(m) < 4 {
			continue
		}
		rel := m[2]
		if !strings.Contains(rel, "memento") {
			continue
		}
		mementoURL := strings.Replace(m[1], "http://archive.", "https://archive.", 1)
		t, _ := time.Parse(time.RFC1123, m[3])
		snapshots = append(snapshots, &memento{
			MementoURL: mementoURL,
			CapturedAt: t,
			Mirror:     extractMirror(mementoURL),
			Backend:    string(backendArchiveIs),
		})
	}
	return snapshots
}

func extractMirror(mementoURL string) string {
	u, err := url.Parse(mementoURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// extractReadableText strips HTML to produce a readable plain-text version.
// This is deliberately simple — good enough for reading articles, not a full
// readability implementation.
var (
	scriptRE     = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	styleRE      = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	noscriptRE   = regexp.MustCompile(`(?is)<noscript\b[^>]*>.*?</noscript>`)
	tagRE        = regexp.MustCompile(`<[^>]+>`)
	whitespaceRE = regexp.MustCompile(`\n\s*\n\s*\n+`)
	commentRE    = regexp.MustCompile(`(?s)<!--.*?-->`)
)

func extractReadableText(html string) string {
	// Drop archive.is navigation chrome — everything before <body> and after the content
	// (the archived page content is wrapped in archive.is's own shell).
	html = commentRE.ReplaceAllString(html, "")
	html = scriptRE.ReplaceAllString(html, "")
	html = styleRE.ReplaceAllString(html, "")
	html = noscriptRE.ReplaceAllString(html, "")
	// Preserve paragraph breaks by converting block-level closers to newlines
	html = regexp.MustCompile(`(?i)</(p|div|br|h[1-6]|li|tr)>`).ReplaceAllString(html, "\n")
	// Strip remaining tags
	text := tagRE.ReplaceAllString(html, "")
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&rsquo;", "'")
	text = strings.ReplaceAll(text, "&lsquo;", "'")
	text = strings.ReplaceAll(text, "&rdquo;", "\"")
	text = strings.ReplaceAll(text, "&ldquo;", "\"")
	text = strings.ReplaceAll(text, "&mdash;", "-")
	text = strings.ReplaceAll(text, "&ndash;", "-")
	text = strings.ReplaceAll(text, "&hellip;", "...")
	// Collapse excessive blank lines
	text = whitespaceRE.ReplaceAllString(text, "\n\n")
	// Trim leading/trailing whitespace on each line
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(l))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
