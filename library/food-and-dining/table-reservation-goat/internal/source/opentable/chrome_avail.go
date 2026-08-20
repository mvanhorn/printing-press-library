// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
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
	// Session may be nil (anonymous availability read, or a client built with
	// no auth). Availability is anonymous, so skip cookie injection and let
	// the browser earn its own Akamai cookies.
	var cookies []*http.Cookie
	if c.session != nil {
		cookies = c.session.HTTPCookies(auth.NetworkOpenTable)
	}

	// Capture state shared by the event listener and the body-fetch worker.
	type slotCapture struct {
		mu       sync.Mutex
		body     []byte
		status   int
		err      error
		reqHash  string
		hashSeen bool
		hashDone chan struct{}
		done     chan struct{}
	}
	cap := &slotCapture{hashDone: make(chan struct{}), done: make(chan struct{})}
	coordinator := newAvailabilityCaptureCoordinator()
	var coordinatorMu sync.Mutex
	var callbackMu sync.Mutex
	workerCtx, cancelWorker := context.WithCancel(timed)
	var postDataWorkers sync.WaitGroup
	var actionMu sync.Mutex
	var fetchActions []availabilityCaptureAction
	actionReady := make(chan struct{}, 1)
	var resultOnce sync.Once
	publishResult := func(result *availabilityCaptureResult) {
		if result == nil {
			return
		}
		resultOnce.Do(func() {
			cap.mu.Lock()
			cap.body = result.body
			cap.status = result.status
			cap.err = result.err
			cap.mu.Unlock()
			close(cap.done)
		})
	}
	queueAction := func(action *availabilityCaptureAction) {
		if action == nil {
			return
		}
		select {
		case <-workerCtx.Done():
			return
		default:
		}
		actionMu.Lock()
		fetchActions = append(fetchActions, *action)
		actionMu.Unlock()
		select {
		case actionReady <- struct{}{}:
		default:
		}
	}
	dequeueAction := func() (availabilityCaptureAction, bool) {
		actionMu.Lock()
		defer actionMu.Unlock()
		if len(fetchActions) == 0 {
			return availabilityCaptureAction{}, false
		}
		action := fetchActions[0]
		fetchActions = fetchActions[1:]
		return action, true
	}
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-actionReady:
				for {
					action, ok := dequeueAction()
					if !ok {
						break
					}
					// Listener callbacks are synchronous in chromedp. Waiting on
					// this barrier keeps CDP body reads out of the callback path.
					callbackMu.Lock()
					callbackMu.Unlock()
					select {
					case <-workerCtx.Done():
						return
					default:
					}
					body, err := fetchResponseBodyWithRetry(workerCtx, action.requestID, fetchResponseBody, 25*time.Millisecond)
					coordinatorMu.Lock()
					result := coordinator.FetchCompleted(action.requestID, body, err)
					coordinatorMu.Unlock()
					publishResult(result)
				}
			}
		}
	}()
	hashClosed := false
	closeHashOnce := func() {
		cap.mu.Lock()
		if !hashClosed {
			hashClosed = true
			close(cap.hashDone)
		}
		cap.mu.Unlock()
	}

	chromedp.ListenTarget(timed, func(ev any) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			// Harvest the persisted-query hash the page's own JS uses. The
			// hash rides in the outgoing request, so we capture it even when
			// Akamai later 403s the response — this is what seeds the fast
			// direct path after an OpenTable bundle rotation.
			if e.Request == nil || !strings.Contains(e.Request.URL, "opname=RestaurantsAvailability") {
				return
			}
			coordinatorMu.Lock()
			coordinator.RequestWillBeSent(e.RequestID)
			coordinatorMu.Unlock()
			if ids, ok := restaurantIDsFromRequest(e.Request); ok {
				coordinatorMu.Lock()
				action, result := coordinator.BindRequest(e.RequestID, containsRestaurantID(ids, restID))
				coordinatorMu.Unlock()
				queueAction(action)
				publishResult(result)
			} else {
				reqID := e.RequestID
				postDataWorkers.Add(1)
				go func() {
					defer postDataWorkers.Done()
					callbackMu.Lock()
					callbackMu.Unlock()
					postData, err := network.GetRequestPostData(reqID).Do(timed)
					ids, ok := restaurantIDsFromPostData(string(postData))
					coordinatorMu.Lock()
					action, result := coordinator.BindRequest(reqID, err == nil && ok && containsRestaurantID(ids, restID))
					coordinatorMu.Unlock()
					queueAction(action)
					publishResult(result)
				}()
			}
			if h := hashFromRequest(e.Request); h != "" {
				cap.mu.Lock()
				cap.hashSeen = true
				cap.reqHash = h
				cap.mu.Unlock()
				closeHashOnce()
				return
			}
			cap.mu.Lock()
			cap.hashSeen = true
			cap.mu.Unlock()
			reqID := e.RequestID
			postDataWorkers.Add(1)
			go func() {
				defer postDataWorkers.Done()
				callbackMu.Lock()
				callbackMu.Unlock()
				body, err := network.GetRequestPostData(reqID).Do(timed)
				if err == nil {
					if h := hashFromPostData(body); h != "" {
						cap.mu.Lock()
						cap.reqHash = h
						cap.mu.Unlock()
					}
				}
				closeHashOnce()
			}()
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
			coordinatorMu.Lock()
			action := coordinator.ResponseReceived(e.RequestID, int(e.Response.Status))
			coordinatorMu.Unlock()
			queueAction(action)
		case *network.EventLoadingFinished:
			coordinatorMu.Lock()
			action := coordinator.LoadingFinished(e.RequestID)
			coordinatorMu.Unlock()
			queueAction(action)
		case *network.EventLoadingFailed:
			coordinatorMu.Lock()
			result := coordinator.LoadingFailed(e.RequestID, e.ErrorText)
			coordinatorMu.Unlock()
			publishResult(result)
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
		chromedp.ActionFunc(func(actCtx context.Context) error {
			// Arm before navigating: the page can fire its availability
			// request while Navigate is still in flight, and an unarmed
			// coordinator drops that request's events. Warm-up traffic
			// stays excluded by the restaurant-ID binding.
			coordinatorMu.Lock()
			coordinator.Arm()
			coordinatorMu.Unlock()
			_, _, errorText, _, err := page.Navigate(pageURL).Do(actCtx)
			if err != nil {
				return err
			}
			if errorText != "" {
				return fmt.Errorf("page load error %s", errorText)
			}
			return nil
		}),
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
	runErr := chromedp.Run(timed, tasks)
	cancelWorker()

	// Harvest the persisted-query hash from the page's own outgoing request,
	// regardless of whether the availability call returned slots — the hash
	// rides in the request, so even a WAF-403'd or timed-out run yields it.
	// This is what lets the fast direct path self-heal after a bundle rotation.
	cap.mu.Lock()
	hashSeen := cap.hashSeen
	cap.mu.Unlock()
	if hashSeen {
		select {
		case <-cap.hashDone:
		case <-time.After(500 * time.Millisecond):
		}
	}
	// Stop listener-owned post-data work and wait for every worker before
	// reading capture state or returning. This prevents late writes after a
	// successful body capture ends chromedp.Run.
	cancelTimed()
	callbackMu.Lock()
	callbackMu.Unlock()
	postDataWorkers.Wait()
	<-workerDone
	cap.mu.Lock()
	harvested := cap.reqHash
	cap.mu.Unlock()
	if harvested != "" && harvested != currentAvailabilityHash() {
		if err := savePersistedAvailabilityHash(harvested); err != nil {
			// Best-effort: the harvest still helped this call, but a failed
			// persist means the next 409 re-spawns Chrome instead of reusing
			// the value. Surface it so a read-only/mis-configured cache dir is
			// diagnosable rather than silently degrading.
			fmt.Fprintf(os.Stderr, "opentable chrome: could not persist harvested availability hash: %v\n", err)
		}
	}

	if runErr != nil {
		return nil, fmt.Errorf("opentable chrome: navigate/intercept: %w", runErr)
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

func restaurantIDsFromRequest(req *network.Request) ([]int, bool) {
	if req == nil {
		return nil, false
	}
	var postData strings.Builder
	for _, entry := range req.PostDataEntries {
		if entry == nil || entry.Bytes == "" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(entry.Bytes)
		if err != nil {
			return nil, false
		}
		postData.Write(decoded)
	}
	if postData.Len() == 0 {
		return nil, false
	}
	return restaurantIDsFromPostData(postData.String())
}

func restaurantIDsFromPostData(postData string) ([]int, bool) {
	var envelope struct {
		Variables struct {
			RestaurantIDs []json.RawMessage `json:"restaurantIds"`
		} `json:"variables"`
	}
	if err := json.Unmarshal([]byte(postData), &envelope); err != nil || len(envelope.Variables.RestaurantIDs) == 0 {
		return nil, false
	}
	ids := make([]int, 0, len(envelope.Variables.RestaurantIDs))
	for _, raw := range envelope.Variables.RestaurantIDs {
		var id int
		if err := json.Unmarshal(raw, &id); err != nil {
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return nil, false
			}
			parsed, err := strconv.Atoi(text)
			if err != nil {
				return nil, false
			}
			id = parsed
		}
		ids = append(ids, id)
	}
	return ids, true
}

func containsRestaurantID(ids []int, want int) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// hashFromRequest reconstructs a GraphQL request's POST body from the CDP
// PostDataEntries (each base64-encoded) and extracts the persisted-query
// sha256Hash. Returns "" when the request carries no well-formed hash.
func hashFromRequest(req *network.Request) string {
	if req == nil {
		return ""
	}
	var sb strings.Builder
	for _, e := range req.PostDataEntries {
		if e == nil || e.Bytes == "" {
			continue
		}
		if dec, err := base64.StdEncoding.DecodeString(e.Bytes); err == nil {
			sb.Write(dec)
		}
	}
	return extractSha256Hash(sb.String())
}

func hashFromPostData(postData []byte) string {
	return extractSha256Hash(string(postData))
}

// extractSha256Hash pulls the persisted-query hash out of a GraphQL POST body
// (`..."sha256Hash":"<64 hex>"...`). Chrome serializes PostData as compact
// JSON, so the marker is followed immediately by the value. Returns "" when
// the body has no well-formed 64-hex hash.
func extractSha256Hash(postData string) string {
	const marker = `"sha256Hash":"`
	i := strings.Index(postData, marker)
	if i < 0 {
		return ""
	}
	start := i + len(marker)
	if start+64 > len(postData) {
		return ""
	}
	candidate := postData[start : start+64]
	if !availHashPattern.MatchString(candidate) {
		return ""
	}
	return candidate
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
			// Current consumer-frontend shape (observed August 2026). Keep
			// this alongside the legacy nested shape below because OpenTable
			// rolls the two response envelopes independently.
			Availability            []RestaurantAvailability `json:"availability"`
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
	if len(env.Data.Availability) > 0 {
		return env.Data.Availability, nil
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
