// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

// PATCH: cross-network-source-clients (chrome-fallback) — see .printing-press-patches.json.
//
// Akamai's WAF blocks `opname=RestaurantsAvailability` on /dapi/fe/gql for any
// non-real-Chrome client, regardless of cookies, headers, or TLS fingerprint.
// The block is enforced at the edge before Apollo sees the request, and the
// gateway's persisted-query mechanism prevents aliasing the operation under a
// less-blocked name (409 Conflict on operationName mismatch). Workarounds we
// proved don't work: URL encoding tricks, mixed case, header reorder,
// Firefox/iOS/Android TLS impersonation via Surf, mobile-api.opentable.com
// (now Akamai-fronted too).
//
// What does work: a real Chrome instance running Akamai's JS sensor naturally.
// This file uses chromedp to spawn a brief headless Chrome that:
//   1. Navigates to the OT restaurant page with the user's auth cookies
//   2. Lets the page fire its OWN RestaurantsAvailability XHR
//   3. Intercepts that response via CDP Network domain
//   4. Returns parsed slot data identical in shape to the Surf path
//
// Falls back automatically when the Surf path returns *BotDetectionError on
// RestaurantsAvailability — Surf still handles bootstrap, autocomplete, and
// every other non-blocked operation, so Chrome only spawns when needed.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

// chromeAvailPageURL builds the navigation URL chromedp visits to
// trigger the page's own RestaurantsAvailability XHR. When restSlug
// is populated (named-path callers) it uses the canonical /r/<slug>
// route. When restSlug is empty (numeric-ID callers, e.g.
// `availability check 3688` where Autocomplete was skipped) it falls
// back to /restaurant/profile/<id> — Akamai treats both routes as
// legitimate browser traffic, so the fallback URL is equivalent for
// WAF acceptance. Pinned by chrome_avail_url_test.go.
func chromeAvailPageURL(restID int, restSlug string, partySize int, date, hhmm string) string {
	if restSlug == "" {
		return fmt.Sprintf("https://www.opentable.com/restaurant/profile/%d?covers=%d&dateTime=%sT%s",
			restID, partySize, date, hhmm)
	}
	return fmt.Sprintf("https://www.opentable.com/r/%s?covers=%d&dateTime=%sT%s",
		url.PathEscape(strings.TrimPrefix(restSlug, "/")),
		partySize, date, hhmm)
}

// ChromeAvailability spawns a headless Chrome to fetch slots that Akamai
// blocks on the direct Surf path. Returns the same slice shape as
// RestaurantsAvailability so callers can swap fallback in transparently.
//
// `forwardMinutes` and `forwardSlots` are accepted for parity with the Surf
// signature but the Chrome path uses whatever the page's runtime XHR fires
// (which is bound to its own Apollo client config). Date/time/party drive
// the URL parameters that the page reads on hydration.
func (c *Client) ChromeAvailability(
	ctx context.Context,
	restID int,
	restSlug string,
	date string,
	hhmm string,
	partySize int,
	forwardDays int,
) ([]RestaurantAvailability, error) {
	if restID == 0 {
		return nil, fmt.Errorf("opentable chrome: restaurant id required")
	}
	if hhmm == "" {
		hhmm = "19:00"
	}
	if partySize <= 0 {
		partySize = 2
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if forwardDays <= 0 {
		forwardDays = 1
	}

	// Build the target URL the page expects. `?covers=N&dateTime=YYYY-MM-DDTHH:MM`
	// is the page's hydration shape; the SPA reads these params and the
	// embedded availability XHR uses them to populate forwardDays from
	// SSR initial state.
	pageURL := chromeAvailPageURL(restID, restSlug, partySize, date, hhmm)

	// Akamai's JS sensor reliably detects spawned chromedp instances even with
	// stealth shims — `navigator.webdriver`, plugin tweaks, UA override, and
	// the warm-up nav weren't enough on the live OT.com WAF as of 2026-05.
	// What DOES work: attach to a Chrome the user explicitly opened with
	// `--remote-debugging-port=9222`. That's their real browser doing the
	// work; Akamai accepts every request because the JS sensor has been
	// running natively for the full session.
	//
	// Strategy:
	//  1. Try to attach to a CDP endpoint discovered via env or the default
	//     debug port. If found, use it — guaranteed to bypass Akamai because
	//     the request rides on the user's real Chrome session.
	//  2. Fall back to a stealth-spawned headless Chrome. This currently 403s
	//     against OT but the scaffold is here for future stealth wins (or
	//     when Akamai relaxes the WAF rule).
	//
	// `TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL` overrides the default
	// `http://localhost:9222`. Set when the user has Chrome running on a
	// different port (other apps' WebDriver may already use 9222).
	debugURL := os.Getenv("TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL")
	if debugURL == "" {
		debugURL = "http://localhost:9222"
	}
	wsURL, attachErr := discoverChromeWebSocket(ctx, debugURL)

	var allocCtx context.Context
	var cancelAlloc context.CancelFunc
	usedAttach := false
	if wsURL != "" {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(ctx, wsURL)
		usedAttach = true
	} else {
		// Fall back to spawn-with-stealth. Currently 403s on OT but we keep
		// the path live so users without remote-debugging-port still get a
		// clear, actionable error rather than a missing-feature error.
		tmpDir, err := os.MkdirTemp("", "trg-pp-chrome-")
		if err != nil {
			return nil, fmt.Errorf("opentable chrome: temp profile: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		headlessMode := os.Getenv("TABLE_RESERVATION_GOAT_OT_CHROME_HEADLESS")
		if headlessMode == "" {
			headlessMode = "new"
		}
		allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserDataDir(tmpDir),
			chromedp.Flag("headless", headlessMode),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-features", "AutomationControlled,IsolateOrigins,site-per-process"),
			chromedp.Flag("enable-automation", false),
			chromedp.Flag("excludeSwitches", "enable-automation"),
			chromedp.Flag("disable-infobars", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"),
		)
		if headlessMode == "false" {
			allocOpts = append(allocOpts, chromedp.Flag("headless", false))
		}
		if path := findChromeBinary(); path != "" {
			allocOpts = append(allocOpts, chromedp.ExecPath(path))
		}
		allocCtx, cancelAlloc = chromedp.NewExecAllocator(ctx, allocOpts...)
	}
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	_ = attachErr // attachErr is informational; reported on failure below
	// Cap total wall time so a Chrome hang doesn't freeze the CLI. 30s is
	// generous: cold start ~1.5s, navigation ~2s, JS sensor solve + XHR ~1s.
	timed, cancelTimed := context.WithTimeout(browserCtx, 30*time.Second)
	defer cancelTimed()

	// Inject the user's OT cookies before navigation so the authenticated
	// SSR is what hydrates. Akamai cookies (_abck, bm_*) are intentionally
	// NOT injected — Akamai will issue fresh ones for this session and
	// validate them against this Chrome instance's behavior.
	cookies := c.session.HTTPCookies(auth.NetworkOpenTable)

	// Capture state for the response listener
	type slotCapture struct {
		mu     sync.Mutex
		body   []byte
		status int
		err    error
		done   chan struct{}
	}
	cap := &slotCapture{done: make(chan struct{})}
	closed := false
	closeOnce := func() {
		cap.mu.Lock()
		if !closed {
			closed = true
			close(cap.done)
		}
		cap.mu.Unlock()
	}

	chromedp.ListenTarget(timed, func(ev any) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Response == nil {
				return
			}
			if !strings.Contains(e.Response.URL, "/dapi/fe/gql") {
				return
			}
			if !strings.Contains(e.Response.URL, "opname=RestaurantsAvailability") {
				return
			}
			// Defer the body fetch — chromedp wants us to call from the
			// run context, not the listener thread.
			reqID := e.RequestID
			status := int(e.Response.Status)
			go func() {
				body, err := fetchResponseBody(timed, reqID)
				cap.mu.Lock()
				cap.body = body
				cap.status = status
				cap.err = err
				cap.mu.Unlock()
				closeOnce()
			}()
		}
	})

	// Stealth shim — injected before any document loads. Hides the most
	// commonly-fingerprinted automation markers. This is the bare minimum
	// Akamai's sensor checks; rebrowser/puppeteer-extra-plugin-stealth has
	// dozens more, but for OT specifically these have proven sufficient.
	stealth := `
		Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
		window.chrome = window.chrome || { runtime: {}, app: {} };
		Object.defineProperty(navigator, 'plugins', { get: () => [1,2,3,4,5] });
		Object.defineProperty(navigator, 'languages', { get: () => ['en-US','en'] });
		const origQuery = navigator.permissions && navigator.permissions.query;
		if (origQuery) {
			navigator.permissions.query = (p) =>
				p && p.name === 'notifications'
					? Promise.resolve({ state: Notification.permission })
					: origQuery.call(navigator.permissions, p);
		}
	`

	tasks := chromedp.Tasks{
		network.Enable(),
		// Inject stealth before any document, so the very first SSR page
		// sees a clean nav environment.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(stealth).Do(actCtx)
			return err
		}),
		injectCookies(cookies),
		// Warm-up: visit a non-availability page first so Akamai's JS
		// sensor runs and issues fresh _abck/bm_* cookies for this Chrome
		// session. The restaurant profile page is whitelisted by the WAF
		// and gives the sensor enough JS to bless the session before we
		// touch the rate-limited GraphQL endpoint.
		chromedp.Navigate("https://www.opentable.com/restaurant/profile/100"),
		chromedp.Sleep(2 * time.Second),
		chromedp.Navigate(pageURL),
		// Wait until either the listener captures the response or the
		// timeout fires. `chromedp.ActionFunc` lets us yield to the event
		// loop without spinning.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-cap.done:
				return nil
			case <-actCtx.Done():
				return actCtx.Err()
			}
		}),
	}
	if err := chromedp.Run(timed, tasks); err != nil {
		return nil, fmt.Errorf("opentable chrome: navigate/intercept: %w", err)
	}

	cap.mu.Lock()
	body, status, capErr := cap.body, cap.status, cap.err
	cap.mu.Unlock()
	if capErr != nil {
		return nil, fmt.Errorf("opentable chrome: read response body: %w", capErr)
	}
	if status != 200 {
		mode := "spawn"
		hint := ""
		if usedAttach {
			mode = "attach"
		} else {
			// Spawn mode currently 403s against OT's WAF. Tell the user how
			// to switch to attach mode, which uses their actual Chrome.
			hint = ` — to use your real Chrome instead, quit Chrome, then relaunch it with: ` +
				`"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --remote-debugging-port=9222 ` +
				`(or set TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL to a different host:port)`
		}
		return nil, fmt.Errorf("opentable chrome (%s): page-fired RestaurantsAvailability returned HTTP %d%s",
			mode, status, hint)
	}
	return parseAvailabilityResponse(body)
}

// discoverChromeWebSocket queries Chrome's DevTools discovery endpoint and
// returns the browser-level WebSocket URL we can attach to via CDP. Returns
// ("", err) when no debug-enabled Chrome is reachable. The discovery endpoint
// is `<base>/json/version`; the response carries a `webSocketDebuggerUrl`.
func discoverChromeWebSocket(ctx context.Context, baseURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/json/version", nil)
	if err != nil {
		return "", err
	}
	tctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req = req.WithContext(tctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chrome devtools discovery returned HTTP %d", resp.StatusCode)
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	if v.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("chrome devtools discovery: empty webSocketDebuggerUrl")
	}
	return v.WebSocketDebuggerURL, nil
}

// findChromeBinary locates a usable Chrome on the host. macOS, Linux, Windows.
// Returns "" if none found — chromedp will then try its default discovery.
func findChromeBinary() string {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// injectCookies sets the user's session cookies on the headless Chrome's
// CDP cookie store before navigation, so SSR hydrates as the logged-in user.
func injectCookies(cookies []*http.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		expr := time.Now().AddDate(1, 0, 0)
		for _, c := range cookies {
			// Skip Akamai cookies — fresh Chrome session will earn its own.
			lc := c.Name
			if strings.HasPrefix(lc, "bm_") || lc == "_abck" || lc == "ak_bmsc" || lc == "ftc" {
				continue
			}
			expires := c.Expires
			if expires.IsZero() {
				expires = expr
			}
			domain := c.Domain
			if domain == "" {
				domain = ".opentable.com"
			}
			path := c.Path
			if path == "" {
				path = "/"
			}
			expiresEpoch := cdp.TimeSinceEpoch(expires)
			if err := network.SetCookie(c.Name, c.Value).
				WithDomain(domain).
				WithPath(path).
				WithExpires(&expiresEpoch).
				WithSecure(true).
				Do(ctx); err != nil {
				// Best-effort — cookie injection failures shouldn't kill the run.
				continue
			}
		}
		return nil
	})
}

// fetchResponseBody pulls the response body for a captured network request via CDP.
func fetchResponseBody(ctx context.Context, reqID network.RequestID) ([]byte, error) {
	var body []byte
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
		b, err := network.GetResponseBody(reqID).Do(actCtx)
		if err != nil {
			return err
		}
		body = b
		return nil
	}))
	if err != nil {
		return nil, err
	}
	return body, nil
}

// parseAvailabilityResponse converts the GraphQL JSON response into the
// same []RestaurantAvailability shape the Surf path returns. The
// RestaurantsAvailability response from /dapi/fe/gql wraps the data under
// `data.restaurantsAvailability.availabilities[]` per restaurant; each
// availability has `availabilityDays[]` with `slots[]` carrying
// dateTime/timeOffsetMinutes/isAvailable.
func parseAvailabilityResponse(body []byte) ([]RestaurantAvailability, error) {
	var env struct {
		Data struct {
			RestaurantsAvailability struct {
				Availabilities []struct {
					RestaurantID     int `json:"restaurantId"`
					AvailabilityDays []struct {
						Date  string `json:"date"`
						Slots []struct {
							TimeOffsetMinutes int  `json:"timeOffsetMinutes"`
							IsAvailable       bool `json:"isAvailable"`
						} `json:"timeSlots"`
					} `json:"availabilityDays"`
				} `json:"availabilities"`
			} `json:"restaurantsAvailability"`
		} `json:"data"`
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("parsing chrome-captured RestaurantsAvailability: %w", err)
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("opentable: chrome-captured response carried %d GraphQL errors; first: %s",
			len(env.Errors), env.Errors[0].Message)
	}
	out := make([]RestaurantAvailability, 0, len(env.Data.RestaurantsAvailability.Availabilities))
	for _, a := range env.Data.RestaurantsAvailability.Availabilities {
		ra := RestaurantAvailability{RestaurantID: a.RestaurantID}
		for _, d := range a.AvailabilityDays {
			day := AvailabilityDay{Date: d.Date}
			for _, s := range d.Slots {
				day.Slots = append(day.Slots, AvailabilitySlot{
					TimeOffsetMinutes: s.TimeOffsetMinutes,
					IsAvailable:       s.IsAvailable,
				})
			}
			ra.AvailabilityDays = append(ra.AvailabilityDays, day)
		}
		out = append(out, ra)
	}
	return out, nil
}
