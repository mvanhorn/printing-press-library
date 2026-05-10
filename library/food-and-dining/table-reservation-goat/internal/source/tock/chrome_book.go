// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package tock

// chromedp-attach implementation of Tock book. Mirrors the pattern in
// `internal/source/opentable/chrome_avail.go`: prefer attaching to a Chrome
// session at `localhost:9222` (or `TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL`),
// fall back to a stealth-spawned headless Chrome.
//
// Why chromedp instead of HTTP form-replay: Tock's book uses traditional
// form-submit page navigation (POST /<slug>/checkout/confirm-purchase, no
// XHR). The form body shape was not captured during U1 discovery (chrome-mcp
// privacy filter blocked it). chromedp delegates to a real browser that
// handles all CSRF/Braintree-token complexity natively.
//
// CVC handling: Tock requires per-transaction CVC re-entry even when the
// card is on file. The CLI prompts the user via stdin; the value is passed
// through to ChromeBook(). Per system rules, only CVC (3-4 digits) is asked
// — the full card number stays on the user's Tock profile.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// ChromeBook performs a Tock booking via a real Chrome session. Connects
// to a debug port at localhost:9222 (or TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL),
// or spawns a stealth headless Chrome as fallback. Drives the page through:
// venue → slot click → checkout → CVC fill (if card-required) → confirm →
// receipt page → extract confirmation.
func (c *Client) ChromeBook(ctx context.Context, req BookRequest) (*BookResponse, error) {
	if req.VenueSlug == "" {
		return nil, fmt.Errorf("tock chromebook: VenueSlug required")
	}
	if req.ReservationDate == "" || req.ReservationTime == "" || req.PartySize <= 0 {
		return nil, fmt.Errorf("tock chromebook: Date/Time/PartySize required")
	}

	// Step 1: Establish Chrome connection (attach preferred, spawn fallback).
	debugURL := os.Getenv("TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL")
	if debugURL == "" {
		debugURL = "http://localhost:9222"
	}
	wsURL, _ := discoverTockChromeWebSocket(ctx, debugURL)

	var allocCtx context.Context
	var cancelAlloc context.CancelFunc
	if wsURL != "" {
		allocCtx, cancelAlloc = chromedp.NewRemoteAllocator(ctx, wsURL)
	} else {
		tmpDir, err := os.MkdirTemp("", "trg-pp-chrome-tock-")
		if err != nil {
			return nil, fmt.Errorf("tock chromebook: temp profile: %w", err)
		}
		defer os.RemoveAll(tmpDir)
		headlessMode := os.Getenv("TABLE_RESERVATION_GOAT_TOCK_CHROME_HEADLESS")
		if headlessMode == "" {
			headlessMode = "new"
		}
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserDataDir(tmpDir),
			chromedp.Flag("headless", headlessMode),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"),
		)
		if headlessMode == "false" {
			opts = append(opts, chromedp.Flag("headless", false))
		}
		allocCtx, cancelAlloc = chromedp.NewExecAllocator(ctx, opts...)
	}
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	timed, cancelTimed := context.WithTimeout(browserCtx, 60*time.Second)
	defer cancelTimed()

	// Inject Tock cookies (session auth) before navigation. Nil-safe: if
	// the client was constructed without a session (e.g., unit tests),
	// proceed with no cookies — Chrome will be unauthenticated.
	var cookies []*http.Cookie
	if c.session != nil {
		cookies = c.session.HTTPCookies(auth.NetworkTock)
	}

	// Capture the receipt URL via response listener (set after the form
	// submit redirects to /receipt?purchaseId=NNN).
	type receiptCapture struct {
		mu         sync.Mutex
		receiptURL string
		done       chan struct{}
	}
	rc := &receiptCapture{done: make(chan struct{})}
	closeOnce := func() {
		rc.mu.Lock()
		select {
		case <-rc.done:
		default:
			close(rc.done)
		}
		rc.mu.Unlock()
	}
	chromedp.ListenTarget(timed, func(ev any) {
		if e, ok := ev.(*page.EventFrameNavigated); ok && e.Frame != nil {
			u := e.Frame.URL
			if strings.Contains(u, "/receipt") && strings.Contains(u, "purchaseId=") && !strings.Contains(u, "/cancel") {
				rc.mu.Lock()
				if rc.receiptURL == "" {
					rc.receiptURL = u
				}
				rc.mu.Unlock()
				closeOnce()
			}
		}
	})

	// Build venue URL with date/time/party params (Tock honors these).
	venueURL := buildVenueDeepLinkURL(req.VenueSlug, req.ExperienceID, req.ReservationDate, req.ReservationTime, req.PartySize)

	// Convert ReservationTime "HH:MM" (24h) to display form "H:MM AM/PM" or "HH:MM AM/PM".
	displayTime := convertTo12hDisplay(req.ReservationTime)

	tasks := chromedp.Tasks{
		network.Enable(),
		injectTockCookies(cookies),
		chromedp.Navigate(venueURL),
		chromedp.Sleep(2 * time.Second),
		// Find and click the slot button by visible time text.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickSlotByTimeText(actCtx, displayTime)
		}),
		chromedp.Sleep(2 * time.Second),
		// Wait for the checkout page (URL contains /checkout/confirm-purchase).
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return waitForCheckoutPage(actCtx, 15*time.Second)
		}),
		// If a CVC field is present, fill it. (Free venues skip this.)
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return fillCVCIfPresent(actCtx, req.CVC)
		}),
		// Check the cancellation-policy acknowledgment checkbox if present.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return checkAcknowledgeIfPresent(actCtx)
		}),
		chromedp.Sleep(500 * time.Millisecond),
		// Click "Place reservation" / Confirm button.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickPlaceReservation(actCtx)
		}),
		// Wait for receipt-page navigation OR timeout.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			select {
			case <-rc.done:
				return nil
			case <-actCtx.Done():
				return actCtx.Err()
			}
		}),
	}
	if err := chromedp.Run(timed, tasks); err != nil {
		// If we have a receipt URL captured already, treat as success.
		rc.mu.Lock()
		gotURL := rc.receiptURL
		rc.mu.Unlock()
		if gotURL == "" {
			return nil, fmt.Errorf("tock chromebook: %w", err)
		}
	}

	rc.mu.Lock()
	receiptURL := rc.receiptURL
	rc.mu.Unlock()
	if receiptURL == "" {
		return nil, fmt.Errorf("tock chromebook: never reached /receipt page (slot may have been taken or CVC rejected)")
	}

	// Parse the receipt page's $REDUX_STATE for the booking details.
	resp, err := parseTockReceipt(timed, receiptURL, req)
	if err != nil {
		return nil, fmt.Errorf("tock chromebook: parsing receipt: %w", err)
	}
	resp.ReceiptURL = receiptURL
	return resp, nil
}

// convertTo12hDisplay returns "2:30 PM" from "14:30" so we can match the
// rendered slot button text. Tock's UI shows times in 12h format with PM/AM.
func convertTo12hDisplay(hhmm string) string {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return hhmm
	}
	return t.Format("3:04 PM")
}

// clickSlotByTimeText finds a button whose text contains the slot time and
// "Book", then clicks it.
func clickSlotByTimeText(ctx context.Context, displayTime string) error {
	js := fmt.Sprintf(`
		(() => {
			const target = %q;
			const btns = Array.from(document.querySelectorAll('button, a'));
			for (const b of btns) {
				const t = (b.textContent || '').trim();
				if (t.includes(target) && /book/i.test(t)) {
					b.click();
					return true;
				}
			}
			// Fallback: look for an input/button with the time text alone
			for (const b of btns) {
				const t = (b.textContent || '').trim();
				if (t === target) { b.click(); return true; }
			}
			return false;
		})()
	`, displayTime)
	var clicked bool
	if err := chromedp.Evaluate(js, &clicked).Do(ctx); err != nil {
		return fmt.Errorf("evaluating slot click: %w", err)
	}
	if !clicked {
		return fmt.Errorf("slot button for %q not found", displayTime)
	}
	return nil
}

// waitForCheckoutPage polls for the URL containing /checkout/confirm-purchase.
func waitForCheckoutPage(ctx context.Context, deadline time.Duration) error {
	stop := time.After(deadline)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		var loc string
		if err := chromedp.Location(&loc).Do(ctx); err == nil {
			if strings.Contains(loc, "/checkout/confirm-purchase") {
				return nil
			}
		}
		select {
		case <-tick.C:
		case <-stop:
			return fmt.Errorf("checkout page never reached within %s", deadline)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// fillCVCIfPresent fills the CVC input if found on the page. No-op for
// free venues that don't render a CVC field.
func fillCVCIfPresent(ctx context.Context, cvc string) error {
	if cvc == "" {
		return nil
	}
	js := `
		(() => {
			const inputs = Array.from(document.querySelectorAll('input'));
			for (const i of inputs) {
				const ph = (i.placeholder || '').toLowerCase();
				const name = (i.name || '').toLowerCase();
				const id = (i.id || '').toLowerCase();
				if (ph === 'cvc' || ph === 'cvv' || /cvc|cvv|securityCode/i.test(name) || /cvc|cvv|security/i.test(id)) {
					i.focus();
					i.value = arguments[0];
					i.dispatchEvent(new Event('input', { bubbles: true }));
					i.dispatchEvent(new Event('change', { bubbles: true }));
					return true;
				}
			}
			return false;
		})()
	`
	var filled bool
	if err := chromedp.Evaluate(fmt.Sprintf("(%s)(%q)", js, cvc), &filled).Do(ctx); err != nil {
		return fmt.Errorf("evaluating CVC fill: %w", err)
	}
	// Not finding a CVC field is fine — venue may not require card.
	return nil
}

// checkAcknowledgeIfPresent ticks the cancellation-policy checkbox if present.
func checkAcknowledgeIfPresent(ctx context.Context) error {
	js := `
		(() => {
			const cbs = Array.from(document.querySelectorAll('input[type="checkbox"]'));
			for (const cb of cbs) {
				if (!cb.checked) {
					cb.click();
				}
			}
			return cbs.length;
		})()
	`
	var n int
	_ = chromedp.Evaluate(js, &n).Do(ctx)
	return nil
}

// clickPlaceReservation clicks the confirm button on the checkout page.
func clickPlaceReservation(ctx context.Context) error {
	js := `
		(() => {
			const btns = Array.from(document.querySelectorAll('button'));
			for (const b of btns) {
				const t = (b.textContent || '').trim();
				if (/place reservation|confirm reservation|book now|complete reservation|complete booking/i.test(t)) {
					b.click();
					return t;
				}
			}
			// Fallback: any visible blue/primary submit button at bottom of form
			for (const b of btns) {
				if (b.type === 'submit') { b.click(); return 'submit'; }
			}
			return null;
		})()
	`
	var label any
	if err := chromedp.Evaluate(js, &label).Do(ctx); err != nil {
		return fmt.Errorf("evaluating place-reservation click: %w", err)
	}
	if label == nil {
		return fmt.Errorf("place-reservation button not found")
	}
	return nil
}

// parseTockReceipt navigates to the receipt URL (already there post-redirect),
// extracts $REDUX_STATE, and parses the purchase details.
func parseTockReceipt(ctx context.Context, receiptURL string, req BookRequest) (*BookResponse, error) {
	// Pull $REDUX_STATE from the current page (already on receipt).
	var rawState string
	js := `JSON.stringify(window.$REDUX_STATE || null)`
	if err := chromedp.Evaluate(js, &rawState).Do(ctx); err != nil {
		return nil, fmt.Errorf("evaluating $REDUX_STATE: %w", err)
	}
	resp := &BookResponse{
		VenueSlug:       req.VenueSlug,
		ReservationDate: req.ReservationDate,
		ReservationTime: req.ReservationTime,
		PartySize:       req.PartySize,
		ReceiptURL:      receiptURL,
	}
	// Extract purchaseId from receipt URL.
	if u, err := url.Parse(receiptURL); err == nil {
		if pid := u.Query().Get("purchaseId"); pid != "" {
			fmt.Sscanf(pid, "%d", &resp.PurchaseID)
		}
	}
	if rawState != "" && rawState != "null" {
		var state map[string]any
		if err := json.Unmarshal([]byte(rawState), &state); err == nil {
			if purchase, ok := state["purchase"].(map[string]any); ok {
				if po, ok := purchase["purchasedOrder"].(map[string]any); ok {
					if confNo, ok := po["confirmationNumber"].(string); ok {
						resp.ConfirmationNumber = confNo
					}
				}
			}
		}
	}
	// Best-effort: pull confirmation from page text if state didn't have it.
	if resp.ConfirmationNumber == "" {
		var pageText string
		_ = chromedp.Evaluate(`document.body.innerText || ''`, &pageText).Do(ctx)
		if idx := strings.Index(pageText, "TOCK-R-"); idx >= 0 {
			end := idx + 7
			for end < len(pageText) && (pageText[end] == '-' || (pageText[end] >= 'A' && pageText[end] <= 'Z') || (pageText[end] >= '0' && pageText[end] <= '9')) {
				end++
			}
			resp.ConfirmationNumber = pageText[idx:end]
		}
	}
	return resp, nil
}

// injectTockCookies sets the user's Tock cookies on the Chrome session
// before navigation. Akamai/Cloudflare cookies are skipped — the fresh
// Chrome session will earn its own.
func injectTockCookies(cookies []*http.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		expr := time.Now().AddDate(1, 0, 0)
		for _, c := range cookies {
			if strings.HasPrefix(c.Name, "bm_") || c.Name == "_abck" || c.Name == "ak_bmsc" || strings.HasPrefix(c.Name, "cf_") {
				continue
			}
			expires := c.Expires
			if expires.IsZero() {
				expires = expr
			}
			domain := c.Domain
			if domain == "" {
				domain = ".exploretock.com"
			}
			path := c.Path
			if path == "" {
				path = "/"
			}
			expiresEpoch := cdp.TimeSinceEpoch(expires)
			_ = network.SetCookie(c.Name, c.Value).
				WithDomain(domain).
				WithPath(path).
				WithExpires(&expiresEpoch).
				WithSecure(true).
				Do(ctx)
		}
		return nil
	})
}

// discoverTockChromeWebSocket queries Chrome's DevTools discovery endpoint
// and returns the first usable WebSocket URL. Mirrors the OT-side helper.
func discoverTockChromeWebSocket(ctx context.Context, baseURL string) (string, error) {
	versionURL := strings.TrimRight(baseURL, "/") + "/json/version"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chrome /json/version HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &version); err != nil {
		return "", err
	}
	if version.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("chrome /json/version returned empty webSocketDebuggerUrl")
	}
	return version.WebSocketDebuggerURL, nil
}
