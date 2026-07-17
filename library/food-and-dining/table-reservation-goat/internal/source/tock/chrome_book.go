// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

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
// CVC handling: Tock may require per-transaction CVC re-entry even when the
// card is on file. The CLI may prompt interactively, while agent/no-input mode
// either uses TRG_TOCK_CVC or attempts without one and returns typed
// ErrCVCRequired if checkout remains blocked. Per system rules, only CVC (3-4
// digits) is accepted — the full card number stays on the user's Tock profile.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	cdproto "github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
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

	// Inject Tock cookies (session auth) before navigation — but ONLY for
	// the spawned headless fallback. An attached Chrome brings its own live
	// session; injecting the (possibly stale) session.json cookies clobbers
	// it and surfaces login walls mid-flow (observed live 2026-07-08).
	var cookies []*http.Cookie
	if wsURL == "" && c.session != nil {
		cookies = c.session.HTTPCookies(auth.NetworkTock)
	}

	// Build venue URL with date/time/party params (Tock honors these).
	venueURL := buildVenueDeepLinkURL(req.VenueSlug, req.ExperienceID, req.ReservationDate, req.ReservationTime, req.PartySize)

	// Convert ReservationTime "HH:MM" (24h) to display form "H:MM AM/PM" or "HH:MM AM/PM".
	displayTime := convertTo12hDisplay(req.ReservationTime)

	if err := chromedp.Run(timed,
		network.Enable(),
		injectTockCookies(cookies),
		chromedp.Navigate(venueURL),
		// Activate the tab: Chrome throttles hidden/background tabs, which
		// both drops synthesized input and can stall SPA route transitions.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return page.BringToFront().Do(actCtx)
		}),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return nil, fmt.Errorf("tock chromebook: %w", err)
	}

	// Find and click the requested booking control. Legacy Tock pages expose
	// one button per slot time; newer experience-card pages expose a time
	// combobox plus "Book now" controls on experience cards. The helper may
	// reattach to a recovered Tock tab if the original target has gone stale.
	activeCtx, activeCancel, err := clickRequestedTockBookingControl(timed, venueURL, displayTime, req.ReservationDate, req.ReservationTime, req.PartySize, req.ExperienceID)
	if activeCancel != nil {
		defer activeCancel()
	}
	if err != nil {
		return nil, fmt.Errorf("tock chromebook: %w", err)
	}

	var receiptURL string
	if err := chromedp.Run(activeCtx,
		chromedp.Sleep(2*time.Second),
		// Wait for the checkout page (URL contains /checkout/confirm-purchase).
		chromedp.ActionFunc(func(actCtx context.Context) error {
			if err := waitForCheckoutPage(actCtx, 15*time.Second); err == nil {
				return nil
			}
			// Tock's SPA occasionally drops the checkout transition even
			// after the hold locks and prices successfully (observed live
			// 2026-07-09: lock+price 200, checkout never mounts). Re-click
			// once via the search results page before giving up.
			// The search results page is venue-wide (buildVenueSearchURL
			// strips /experience/<id>) and re-clicks by time only, so a
			// pinned experience could silently book a different one at the
			// same time. Fail instead; the caller can retry the exact hold.
			if req.ExperienceID != 0 {
				return fmt.Errorf("checkout page never reached for experience %d; not retrying venue-wide", req.ExperienceID)
			}
			searchURL := buildVenueSearchURL(venueURL, req.ReservationDate, req.ReservationTime, req.PartySize)
			if searchURL == "" {
				return fmt.Errorf("checkout page never reached and no search URL to retry with")
			}
			if err := clickSearchResultsPage(actCtx, searchURL, displayTime); err != nil {
				return fmt.Errorf("checkout transition retry: %w", err)
			}
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
		chromedp.Sleep(500*time.Millisecond),
		// Click "Place reservation" / Confirm button.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickPlaceReservation(actCtx)
		}),
		// Wait for receipt-page navigation, answering any post-confirm
		// interstitial dialogs (e.g. the SMS opt-in) that block submission.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			u, err := waitForReceiptThroughDialogs(actCtx, 30*time.Second)
			if err != nil {
				// Distinguish "stalled on a required CVC we don't have" from
				// generic checkout failure so machine callers get a typed,
				// actionable outcome instead of a timeout.
				if req.CVC == "" && emptyCVCFieldPresent(actCtx) {
					return ErrCVCRequired
				}
				return fmt.Errorf("%w; checkout_state=%s", err, checkoutPageStateHint(actCtx))
			}
			receiptURL = u
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("tock chromebook: %w", err)
	}
	if receiptURL == "" {
		return nil, fmt.Errorf("tock chromebook: never reached /receipt page (slot may have been taken or CVC rejected)")
	}

	// Parse the receipt page's $REDUX_STATE for the booking details.
	resp, err := parseTockReceipt(activeCtx, receiptURL, req)
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
func clickSlotByTimeTextJS(displayTime string) string {
	return fmt.Sprintf(`
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
}

func clickSlotByTimeText(ctx context.Context, displayTime string) error {
	js := clickSlotByTimeTextJS(displayTime)
	var clicked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &clicked)); err != nil {
		return fmt.Errorf("evaluating slot click: %w", err)
	}
	if !clicked {
		return fmt.Errorf("slot button for %q not found", displayTime)
	}
	return nil
}

// buildVenueSearchURL derives the venue's /search results URL from the venue
// (deep-link) URL. Tock's search page honors the full date/size/time query on
// every venue layout and lists exact-time "Book" rows, while the venue page's
// sidebar picker exposes the date through a calendar widget (not a <select>)
// that cannot be driven the way the time control can — so the requested date
// is unreachable from the picker alone (observed live 2026-07-08).
func buildVenueSearchURL(venueURL, isoDate, time24 string, partySize int) string {
	if venueURL == "" || isoDate == "" || time24 == "" {
		return ""
	}
	u, err := url.Parse(venueURL)
	if err != nil || u.Host == "" {
		return ""
	}
	path := u.Path
	if i := strings.Index(path, "/experience/"); i >= 0 {
		path = path[:i]
	}
	u.Path = strings.TrimRight(path, "/") + "/search"
	q := url.Values{}
	q.Set("date", isoDate)
	q.Set("size", fmt.Sprintf("%d", partySize))
	q.Set("time", time24)
	u.RawQuery = q.Encode()
	return u.String()
}

// clickSearchResultsPage navigates to the venue's /search results URL and
// clicks the "Book" row whose nearby time text matches displayTime. Rows pair
// a time label with a Book button inside a small card, so the match walks up
// from each button to the nearest time-bearing ancestor exactly like the
// experience-modal slot matcher.
func clickSearchResultsPage(ctx context.Context, searchURL, displayTime string) error {
	if err := chromedp.Run(ctx,
		chromedp.Navigate(searchURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return fmt.Errorf("navigating to search results: %w", err)
	}
	js := fmt.Sprintf(`
		(() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const target = %q;
			const timePattern = /\d{1,2}:\d{2}\s*(?:AM|PM)/i;
			const times = [];
			for (const btn of Array.from(document.querySelectorAll('button')).filter((el) => /^book$/i.test(clean(el.textContent)))) {
				let node = btn.parentElement;
				for (let hops = 0; node && hops < 5; hops++, node = node.parentElement) {
					const text = clean(node.textContent);
					if (text.length > 200) break;
					const m = text.match(timePattern);
					if (m) {
						times.push(m[0]);
						if (text.includes(target)) {
							btn.scrollIntoView({block: 'center'});
							btn.dispatchEvent(new MouseEvent('mousedown', {bubbles: true}));
							btn.click();
							btn.dispatchEvent(new MouseEvent('mouseup', {bubbles: true}));
							return { ok: true, step: 'search_results_page', detail: target };
						}
						break;
					}
				}
			}
			return { ok: false, step: 'search_results_page', detail: 'visible: ' + (times.length ? times.join(', ') : 'none') };
		})()
	`, displayTime)
	deadline := time.Now().Add(12 * time.Second)
	var last tockComboboxClickResult
	for {
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &last)); err != nil {
			return fmt.Errorf("evaluating search results click: %w", err)
		}
		if last.OK {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("requested time %q not offered on search results page; %s", displayTime, last.Detail)
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond)); err != nil {
			return fmt.Errorf("waiting for search results: %w", err)
		}
	}
}

func clickRequestedTockBookingControl(ctx context.Context, venueURL, displayTime, isoDate, time24 string, partySize, experienceID int) (context.Context, context.CancelFunc, error) {
	var legacyErr error
	if experienceID == 0 {
		legacyErr = clickSlotByTimeText(ctx, displayTime)
	} else {
		legacyErr = clickPinnedSlotByTimeText(ctx, displayTime, experienceID)
	}
	if legacyErr == nil {
		return ctx, nil, nil
	}

	// Prefer the /search results page: it is the one Tock surface that honors
	// the requested date/size/time on every layout. The venue-page flows below
	// remain as fallbacks for layouts where search offers nothing.
	// Never for a pinned experience, though — /search is venue-wide and
	// clicks by time only, so it could book a different experience at the
	// same slot. Pinned requests use only experience-aware paths.
	searchErr := errors.New("no search URL derivable from venue URL")
	if experienceID != 0 {
		searchErr = errors.New("search fallback skipped: experience-specific request")
	} else if searchURL := buildVenueSearchURL(venueURL, isoDate, time24, partySize); searchURL != "" {
		searchErr = clickSearchResultsPage(ctx, searchURL, displayTime)
		if searchErr == nil {
			return ctx, nil, nil
		}
		if isTargetNavigatedOrClosed(searchErr) && onCheckoutPage(ctx) {
			return ctx, nil, nil
		}
	}

	activeCtx, activeCancel, ensureErr := ensureTockVenuePage(ctx, venueURL)
	comboboxErr := ensureErr
	if comboboxErr == nil {
		var retryCtx context.Context
		var retryCancel context.CancelFunc
		retryCtx, retryCancel, comboboxErr = clickComboboxExperienceLayoutWithRetry(activeCtx, venueURL, displayTime, isoDate, partySize, experienceID)
		if retryCancel != nil {
			if activeCancel != nil {
				activeCancel()
			}
			activeCtx = retryCtx
			activeCancel = retryCancel
		}
	}
	if comboboxErr == nil {
		return activeCtx, activeCancel, nil
	}

	hintCtx, hintCancel, hintErr := ensureTockVenuePage(activeCtx, venueURL)
	if hintErr != nil {
		hintCtx = activeCtx
	}
	if hintCancel != nil {
		defer hintCancel()
	}
	if activeCancel != nil {
		defer activeCancel()
	}
	return activeCtx, nil, &ChromeBookError{
		Kind:      ErrSlotControlNotFound,
		Step:      "booking_control",
		PageState: tockBookingPageStateHint(hintCtx, venueURL),
		Cause:     tockBookingControlFailureCause(displayTime, legacyErr, searchErr, comboboxErr),
	}
}

func tockBookingControlFailureCause(displayTime string, legacyErr, searchErr, comboboxErr error) error {
	return fmt.Errorf("requested_time=%q legacy_slot_error=%v search_results_error=%v combobox_layout_error=%v",
		displayTime, legacyErr, searchErr, comboboxErr)
}

type tockComboboxClickResult struct {
	OK     bool   `json:"ok"`
	Step   string `json:"step"`
	Detail string `json:"detail"`
}

type tockBookingPageState struct {
	Path                   string   `json:"path"`
	ComboboxLayoutDetected bool     `json:"combobox_layout_detected"`
	ChallengeDetected      bool     `json:"challenge_detected"`
	LoginWallDetected      bool     `json:"login_wall_detected"`
	LegacySlotPresent      bool     `json:"legacy_slot_present"`
	TimeComboboxPresent    bool     `json:"time_combobox_present"`
	SearchControlPresent   bool     `json:"search_control_present"`
	ExperienceCardPresent  bool     `json:"experience_card_present"`
	BookControlPresent     bool     `json:"book_control_present"`
	TimeOptionLabels       []string `json:"time_option_labels,omitempty"`
	VisibleControlLabels   []string `json:"visible_control_labels,omitempty"`
}

type tockCheckoutPageState struct {
	Path                   string   `json:"path"`
	ConfirmControlPresent  bool     `json:"confirm_control_present"`
	ConfirmControlEnabled  bool     `json:"confirm_control_enabled"`
	HasCVCField            bool     `json:"has_cvc_field"`
	CheckboxCount          int      `json:"checkbox_count"`
	RequiredUncheckedCount int      `json:"required_unchecked_count"`
	SMSDialogPresent       bool     `json:"sms_dialog_present"`
	VisibleControlLabels   []string `json:"visible_control_labels,omitempty"`
}

func clickComboboxExperienceLayoutWithRetry(ctx context.Context, venueURL, displayTime, isoDate string, partySize, experienceID int) (context.Context, context.CancelFunc, error) {
	if err := clickComboboxExperienceLayout(ctx, displayTime, isoDate, partySize, experienceID); err != nil {
		if !isTargetNavigatedOrClosed(err) {
			return ctx, nil, err
		}
		// A destroyed JS context right after our click often means the click
		// WORKED: the page navigated to checkout and took the evaluation with
		// it. Check before "recovering" back to the venue page, which would
		// abandon a checkout already in progress.
		if onCheckoutPage(ctx) {
			return ctx, nil, nil
		}
		retryCtx, retryCancel, ensureErr := ensureTockVenuePage(ctx, venueURL)
		if ensureErr != nil {
			return ctx, nil, fmt.Errorf("%w; recovery failed: %v", err, ensureErr)
		}
		if retryErr := clickComboboxExperienceLayout(retryCtx, displayTime, isoDate, partySize, experienceID); retryErr != nil {
			if isTargetNavigatedOrClosed(retryErr) && onCheckoutPage(retryCtx) {
				return retryCtx, retryCancel, nil
			}
			if retryCancel != nil {
				retryCancel()
			}
			return ctx, nil, fmt.Errorf("%w; retry_after_target_recovery=%v", err, retryErr)
		}
		return retryCtx, retryCancel, nil
	}
	return ctx, nil, nil
}

// onCheckoutPage reports whether the current target already reached Tock's
// checkout (or receipt) page — i.e., a booking click succeeded even if the
// evaluation that clicked it was destroyed by the navigation.
func onCheckoutPage(ctx context.Context) bool {
	var loc string
	if err := chromedp.Run(ctx, chromedp.Location(&loc)); err != nil {
		return false
	}
	return strings.Contains(loc, "/checkout/") || strings.Contains(loc, "/receipt")
}

func ensureTockVenuePage(ctx context.Context, venueURL string) (context.Context, context.CancelFunc, error) {
	if venueURL == "" {
		// No recovery target (fixture/embedded pages) — trust the current page.
		return ctx, nil, nil
	}
	var loc string
	if err := chromedp.Run(ctx, chromedp.Location(&loc)); err == nil && isTockVenuePageURL(loc, venueURL) {
		return ctx, nil, nil
	}
	if targetCtx, cancel, ok := attachExistingTockVenueTarget(ctx, venueURL); ok {
		return targetCtx, cancel, nil
	}
	if err := navigateTockVenuePage(ctx, venueURL); err != nil {
		if targetCtx, cancel, ok := attachExistingTockVenueTarget(ctx, venueURL); ok {
			return targetCtx, cancel, nil
		}
		if freshCtx, freshCancel, freshErr := openFreshTockVenueTarget(ctx, venueURL); freshErr == nil {
			return freshCtx, freshCancel, nil
		}
		return ctx, nil, fmt.Errorf("recovering venue page: %w", err)
	}
	return ctx, nil, nil
}

func attachExistingTockVenueTarget(ctx context.Context, venueURL string) (context.Context, context.CancelFunc, bool) {
	infos, err := chromedp.Targets(ctx)
	if err != nil {
		return nil, nil, false
	}
	for _, info := range infos {
		if info == nil || info.Type != "page" || !isTockVenuePageURL(info.URL, venueURL) {
			continue
		}
		targetCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(info.TargetID))
		safeCancel := detachOnlyCancel(targetCtx, cancel)
		var loc string
		if err := chromedp.Run(targetCtx, chromedp.Location(&loc)); err != nil {
			safeCancel()
			continue
		}
		if isTockVenuePageURL(loc, venueURL) {
			return targetCtx, safeCancel, true
		}
		safeCancel()
	}
	return nil, nil, false
}

func openFreshTockVenueTarget(ctx context.Context, venueURL string) (context.Context, context.CancelFunc, error) {
	targetCtx, cancel := chromedp.NewContext(ctx)
	safeCancel := detachOnlyCancel(targetCtx, cancel)
	if err := navigateTockVenuePage(targetCtx, venueURL); err != nil {
		safeCancel()
		return ctx, nil, err
	}
	return targetCtx, safeCancel, nil
}

func navigateTockVenuePage(ctx context.Context, venueURL string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(venueURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
}

func detachOnlyCancel(ctx context.Context, cancel context.CancelFunc) context.CancelFunc {
	return func() {
		if c := chromedp.FromContext(ctx); c != nil && c.Target != nil {
			c.Target.TargetID = ""
		}
		cancel()
	}
}

func isTargetNavigatedOrClosed(err error) bool {
	var cdpErr *cdproto.Error
	if errors.As(err, &cdpErr) && cdpErr.Code == -32000 {
		msg := strings.ToLower(cdpErr.Message)
		return strings.Contains(msg, "target") && (strings.Contains(msg, "navigated") || strings.Contains(msg, "closed"))
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "-32000") && strings.Contains(msg, "target") &&
		(strings.Contains(msg, "navigated") || strings.Contains(msg, "closed"))
}

// isTransientNavigationError reports whether err is a transient CDP failure
// chromedp surfaces while the page navigates away mid-command: the
// target-navigated/closed cases, plus the destroyed-execution-context family
// raised when an Evaluate's JS context disappears because a navigation
// committed while the call was in flight. Deliberately broader than
// isTargetNavigatedOrClosed, whose narrower semantics other call sites rely on.
func isTransientNavigationError(err error) bool {
	if err == nil {
		return false
	}
	if isTargetNavigatedOrClosed(err) {
		return true
	}
	var cdpErr *cdproto.Error
	if errors.As(err, &cdpErr) && cdpErr.Code == -32000 {
		return isDestroyedContextMessage(cdpErr.Message)
	}
	msg := err.Error()
	return strings.Contains(msg, "-32000") && isDestroyedContextMessage(msg)
}

func isDestroyedContextMessage(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "cannot find context") ||
		strings.Contains(m, "execution context was destroyed") ||
		strings.Contains(m, "context with specified id")
}

func isTockVenuePageURL(rawURL, venueURL string) bool {
	gotPath := tockVenuePagePath(rawURL)
	wantPath := tockVenuePagePath(venueURL)
	return gotPath != "" && gotPath == wantPath
}

func tockVenuePagePath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.Contains(strings.ToLower(u.Host), "exploretock.com") {
		return ""
	}
	parts := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	if len(parts) == 1 {
		return "/" + parts[0]
	}
	if len(parts) >= 3 && parts[1] == "experience" {
		return "/" + parts[0] + "/experience/" + parts[2]
	}
	return ""
}

const tockPinnedExperiencePathJS = `
			function hasPinnedExperiencePath(value) {
				if (experienceID <= 0 || !value) return false;
				let path = String(value);
				try {
					path = new URL(path, location.href).pathname;
				} catch (_) {
					path = path.split(/[?#]/, 1)[0];
				}
				const segments = path.split('/').filter(Boolean);
				const pinnedID = String(experienceID);
				for (let i = 0; i + 1 < segments.length; i++) {
					if (segments[i] === 'experience' && segments[i + 1] === pinnedID) return true;
				}
				return false;
			}
			function hasOtherExperiencePath(value) {
				if (experienceID <= 0 || !value) return false;
				let path = String(value);
				try {
					path = new URL(path, location.href).pathname;
				} catch (_) {
					path = path.split(/[?#]/, 1)[0];
				}
				const segments = path.split('/').filter(Boolean);
				const pinnedID = String(experienceID);
				for (let i = 0; i + 1 < segments.length; i++) {
					if (segments[i] === 'experience' && /^\d+$/.test(segments[i + 1]) && segments[i + 1] !== pinnedID) return true;
				}
				return false;
			}
`

type tockPinnedLegacyClickResult struct {
	PageMatches bool   `json:"page_matches"`
	Clicked     bool   `json:"clicked"`
	Ambiguous   bool   `json:"ambiguous"`
	Detail      string `json:"detail"`
}

func clickPinnedSlotByTimeTextJS(displayTime string, experienceID int) string {
	return fmt.Sprintf(`
		(() => {
			const target = %q;
			const experienceID = %d;
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const all = (selector) => Array.from(document.querySelectorAll(selector));
			%s
			%s
			if (!hasPinnedExperiencePath(location.pathname)) {
				return { page_matches: false, clicked: false, ambiguous: false, detail: '' };
			}

			// A pinned page identifies the page, not every legacy control on it;
			// cross-sell controls still require tight structural attribution.
			const controls = all('button, a');
			const firstPass = controls.filter((control) => {
				const text = (control.textContent || '').trim();
				return text.includes(target) && /book/i.test(text);
			});
			const secondPass = controls.filter((control) => {
				const text = (control.textContent || '').trim();
				return text === target && !firstPass.includes(control);
			});
			// Merge both historical text-match passes before gating so an early
			// untied match cannot bypass a tied or competing later-pass control.
			const candidates = firstPass.concat(secondPass);
			function legacyLinksFor(control, boundary) {
				return [control].concat(
					boundary ? Array.from(boundary.querySelectorAll('a[href]')) : []
				);
			}
			function legacyLinksOnlyOtherExperience(control, boundary) {
				let sawOther = false;
				for (const el of legacyLinksFor(control, boundary)) {
					const href = el.getAttribute && (el.getAttribute('href') || '');
					if (!href) continue;
					if (hasPinnedExperiencePath(href)) return false;
					if (hasOtherExperiencePath(href)) sawOther = true;
				}
				return sawOther;
			}
			function legacyControlOrExplicitCardHasPinnedExperienceLink(control) {
				const card = explicitCardOf(control);
				return legacyLinksFor(control, card).some((el) => {
					const href = el.getAttribute && (el.getAttribute('href') || '');
					return hasPinnedExperiencePath(href);
				});
			}
			function legacyControlOrExplicitCardLinksOnlyOtherExperience(control) {
				return legacyLinksOnlyOtherExperience(control, explicitCardOf(control));
			}
			function legacySoleCandidateFormLinksOnlyOtherExperience(control) {
				// A form is negative evidence only. It must never positively
				// vouch for a legacy control because broad forms can span
				// several experiences.
				if (explicitCardOf(control)) return false;
				const form = formOf(control);
				if (!form) return false;
				const formCandidates = candidates.filter(
					(other) => formOf(other) === form
				);
				if (formCandidates.length !== 1 || formCandidates[0] !== control) {
					return false;
				}
				return legacyLinksOnlyOtherExperience(control, form);
			}

			const surviving = candidates.filter(
				(control) =>
					!legacyControlOrExplicitCardLinksOnlyOtherExperience(control) &&
					!legacySoleCandidateFormLinksOnlyOtherExperience(control)
			);
			const tied = surviving.filter(
				legacyControlOrExplicitCardHasPinnedExperienceLink
			);
			if (tied.length > 0) {
				tied[0].click();
				return { page_matches: true, clicked: true, ambiguous: false, detail: '' };
			}
			if (surviving.length === 1) {
				surviving[0].click();
				return { page_matches: true, clicked: true, ambiguous: false, detail: '' };
			}
			if (surviving.length > 1) {
				return {
					page_matches: true,
					clicked: false,
					ambiguous: true,
					detail: 'pinned experience ' + experienceID +
						' is ambiguous on its deep-link page: ' +
						surviving.length +
						' untied legacy slot controls and none positively tied'
				};
			}
			return {
				page_matches: true,
				clicked: false,
				ambiguous: false,
				detail: candidates.length === 0
					? 'slot button for "' + target + '" not found'
					: 'all requested-time legacy slot controls were tied only to other experiences'
			};
		})()
	`, displayTime, experienceID, tockPinnedExperiencePathJS, tockPinnedExperienceControlAttributionJS)
}

func clickPinnedSlotByTimeText(ctx context.Context, displayTime string, experienceID int) error {
	js := clickPinnedSlotByTimeTextJS(displayTime, experienceID)
	var result tockPinnedLegacyClickResult
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &result)); err != nil {
		return fmt.Errorf("legacy time-only slot path skipped for pinned experience %d: could not atomically verify current page path and click: %w", experienceID, err)
	}
	if !result.PageMatches {
		return fmt.Errorf("legacy time-only slot path skipped for pinned experience %d: current page is not its experience deep link", experienceID)
	}
	if result.Ambiguous {
		return fmt.Errorf(
			"legacy time-only slot path skipped for pinned experience %d: %s",
			experienceID, result.Detail,
		)
	}
	if !result.Clicked {
		return fmt.Errorf(
			"legacy time-only slot path skipped for pinned experience %d: %s",
			experienceID, result.Detail,
		)
	}
	return nil
}

// clickComboboxExperienceLayout drives Tock's newer booking layout:
// choose the requested time from a combobox/listbox, pick the best matching
// experience card, then click its "Book now" control.
const tockPinnedExperienceControlAttributionJS = `
			const explicitCardSelector = '[data-testid*="experience"], [class*="experience"], [class*="card"]';
			function formOf(control) {
				return control.form || (control.closest ? control.closest('form') : null);
			}
			function explicitCardOf(control) {
				return control.closest ? control.closest(explicitCardSelector) : null;
			}
			function isBookNowControl(control) {
				return /book now/i.test(clean(control.textContent || control.getAttribute('aria-label')));
			}
			// Ties resolve through a TIGHT boundary only: an explicitly marked
			// card ancestor, or a form that owns this control as its sole Book now
			// control. Shared forms and cardFor()'s generic fallbacks (section/li/
			// div) can span several experiences and must never vouch for controls
			// inside them. No tight ancestor means only the control's own href
			// counts, and mis-resolution errs toward fail-closed.
			function tightCardOf(control) {
				const card = explicitCardOf(control);
				if (card) return card;
				const form = formOf(control);
				if (!form) return null;
				const formBookNowControls = all('a, button')
					.filter(isBookNowControl)
					.filter((other) => formOf(other) === form);
				return formBookNowControls.length === 1 && formBookNowControls[0] === control ? form : null;
			}
			function controlOrCardHasPinnedExperienceLink(control) {
				const href = control.getAttribute && (control.getAttribute('href') || '');
				if (hasPinnedExperiencePath(href)) return true;
				const card = tightCardOf(control);
				if (!card) return false;
				return Array.from(card.querySelectorAll('a[href]'))
					.some((link) => hasPinnedExperiencePath(link.getAttribute('href') || ''));
			}
			function cardLinksOnlyOtherExperience(control) {
				const card = tightCardOf(control);
				const links = [control].concat(card ? Array.from(card.querySelectorAll('a[href]')) : []);
				let sawOther = false;
				for (const el of links) {
					const href = el.getAttribute && (el.getAttribute('href') || '');
					if (!href) continue;
					if (hasPinnedExperiencePath(href)) return false;
					if (hasOtherExperiencePath(href)) sawOther = true;
				}
				return sawOther;
			}
`

const tockPinnedExperienceEligibilityJS = tockPinnedExperiencePathJS + tockPinnedExperienceControlAttributionJS + `			function eligibleExperienceControls(controls) {
				if (experienceID === 0) return controls;
				// On the pinned experience's own deep-link page the page's own
				// controls carry no experience href, so they stay eligible —
				// but cross-sell cards positively tied to a DIFFERENT
				// experience must not win the sorter.
				if (hasPinnedExperiencePath(location.pathname)) {
					return controls.filter((control) => !cardLinksOnlyOtherExperience(control));
				}
				return controls.filter(controlOrCardHasPinnedExperienceLink);
			}
`

func clickComboboxExperienceLayoutJS(displayTime, isoDate string, partySize, experienceID int) string {
	return fmt.Sprintf(`
		(async () => {
			const target = %q;
			const isoDate = %q;
			const partySize = %d;
			const experienceID = %d;
			const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const labelFor = (el) => {
				if (!el) return '';
				const parts = [
					el.getAttribute && el.getAttribute('aria-label'),
					el.getAttribute && el.getAttribute('aria-labelledby'),
					el.getAttribute && el.getAttribute('placeholder'),
					el.getAttribute && el.getAttribute('name'),
					el.id,
					el.textContent
				].filter(Boolean);
				if (el.id) {
					const lbl = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
					if (lbl) parts.push(lbl.textContent);
				}
				return clean(parts.join(' '));
			};
			const visible = (el) => {
				if (!el || !el.isConnected) return false;
				const style = window.getComputedStyle(el);
				if (style.visibility === 'hidden' || style.display === 'none') return false;
				const rect = el.getBoundingClientRect();
				return rect.width > 0 && rect.height > 0;
			};
			const all = (selector) => Array.from(document.querySelectorAll(selector)).filter(visible);
			const click = (el) => {
				el.scrollIntoView({ block: 'center', inline: 'center' });
				el.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }));
				el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
				el.click();
				el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
			};
			const fireChange = (el) => {
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
			};
			const exactTime = (el) => clean(el.textContent || el.innerText || el.getAttribute('aria-label') || el.value) === target;
			%s

			for (const select of all('select')) {
				const options = Array.from(select.options || []);
				const match = options.find((opt) => clean(opt.textContent || opt.label || opt.value) === target);
				if (match) {
					select.value = match.value;
					fireChange(select);
					await sleep(150);
					const searchResult = await clickSearchResultSlot();
					if (searchResult !== null) return searchResult;
					return await clickBestExperienceBookNow();
				}
			}

			for (const input of all('input[list]')) {
				const list = input.list;
				if (!list) continue;
				const match = Array.from(list.options || []).find((opt) => clean(opt.label || opt.textContent || opt.value) === target);
				if (!match) continue;
				input.focus();
				input.value = match.value || target;
				fireChange(input);
				await sleep(150);
				const searchResult = await clickSearchResultSlot();
				if (searchResult !== null) return searchResult;
				return await clickBestExperienceBookNow();
			}

			const anyBookNow = all('a, button')
				.some((el) => /book now/i.test(clean(el.textContent || el.getAttribute('aria-label'))));
			const anyTimeCombobox = all('select, [role="combobox"], [aria-haspopup="listbox"], input[list]')
				.some((el) => /time|\b(?:AM|PM)\b/i.test(labelFor(el)));
			if (anyBookNow && !anyTimeCombobox) {
				// Deep-link layout: date/time/party rode the URL, no picker is
				// rendered - the experience card IS the whole flow.
				const searchResult = await clickSearchResultSlot();
				if (searchResult !== null) return searchResult;
				return await clickBestExperienceBookNow();
			}

			const comboCandidates = all('[role="combobox"], [aria-haspopup="listbox"], button, input')
				.map((el) => {
					const text = labelFor(el);
					let score = 0;
					if (/time/i.test(text)) score += 8;
					if (text.includes(target)) score += 6;
					if (/\b(?:AM|PM)\b/.test(text)) score += 4;
					if (/date|calendar/i.test(text)) score -= 5;
					if (el.getAttribute('role') === 'combobox') score += 3;
					if (el.getAttribute('aria-haspopup') === 'listbox') score += 2;
					return { el, score, text };
				})
				.filter((c) => c.score > 0 && !/book now|reserve|sign in|log in/i.test(c.text))
				.sort((a, b) => b.score - a.score)
				.slice(0, 6);

			for (const candidate of comboCandidates) {
				click(candidate.el);
				await sleep(150);
				for (let attempt = 0; attempt < 12; attempt++) {
					const options = all('[role="option"], [role="listbox"] [role="option"], [role="listbox"] button, [role="listbox"] li, [role="listbox"] div, li, button')
						.filter((el) => exactTime(el));
					if (options.length > 0) {
						click(options[0]);
						await sleep(200);
						const searchResult = await clickSearchResultSlot();
						if (searchResult !== null) return searchResult;
						return await clickBestExperienceBookNow();
					}
					await sleep(100);
				}
			}
			return { ok: false, step: 'time_combobox', detail: 'requested time option not found' };

			function cardFor(control) {
				const selectors = ['[data-testid*="experience"]', '[class*="experience"]', '[class*="card"]', 'article', 'section', 'li', 'form', 'div'];
				for (const sel of selectors) {
					const found = control.closest(sel);
					if (found && clean(found.textContent).length > clean(control.textContent).length) return found;
				}
				return control;
			}
			function groupRange(text) {
				const m = text.match(/(?:group[s]?\s*)?(\d+)\s*[-–]\s*(\d+)/i);
				if (!m) return null;
				return { min: Number(m[1]), max: Number(m[2]) };
			}
			function scoreBookControl(control) {
				const card = cardFor(control);
				const text = clean(card.textContent || control.textContent);
				const href = control.getAttribute && (control.getAttribute('href') || '');
				let score = 0;
				if (experienceID > 0 && controlOrCardHasPinnedExperienceLink(control)) score += 100;
				const range = groupRange(text);
				if (range) {
					if (partySize >= range.min && partySize <= range.max) score += 45;
					else score -= 45;
				}
				if (/group/i.test(text)) score += partySize >= 7 ? 12 : -25;
				if (/reservation/i.test(text)) score += 8;
				if (!/group/i.test(text) && partySize < 7) score += 20;
				if (!/group/i.test(text) && partySize >= 7) score -= 8;
				const top = control.getBoundingClientRect().top;
				return { control, cardText: text.slice(0, 160), score, top };
			}
			async function clickBestExperienceBookNow() {
				const bookNowControls = all('a, button')
					.filter((el) => /book now/i.test(clean(el.textContent || el.getAttribute('aria-label'))));
				let eligible = experienceID > 0 ? eligibleExperienceControls(bookNowControls) : bookNowControls;
				if (experienceID > 0 && hasPinnedExperiencePath(location.pathname)) {
					// On the pinned deep-link page an untied control is unprovable:
					// it may be the page's own CTA or an href-less sibling card. A
					// positive tie wins outright; a sole survivor is accepted as
					// the page's own control; anything else is ambiguity and the
					// pin guarantee requires failing closed over letting the
					// party-size scorer guess.
					const tied = eligible.filter(controlOrCardHasPinnedExperienceLink);
					if (tied.length > 0) {
						eligible = tied;
					} else if (eligible.length > 1) {
						return {
							ok: false,
							step: 'experience_card',
							detail: 'pinned experience ' + experienceID + ' is ambiguous on its deep-link page: ' +
								eligible.length + ' untied Book now controls and none positively tied',
						};
					}
				}
				const controls = eligible
					.map(scoreBookControl)
					.sort((a, b) => (b.score - a.score) || (a.top - b.top));
				if (controls.length === 0) {
					if (experienceID > 0) {
						return {
							ok: false,
							step: 'experience_card',
							detail: 'pinned experience ' + experienceID + ' could not be positively identified among Book now controls',
						};
					}
					return { ok: false, step: 'experience_card', detail: 'no Book now controls found after selecting time' };
				}
				const selectedControl = controls[0].control;
				const selectedTightCard = experienceID > 0 ? tightCardOf(selectedControl) : null;
				// Once a pinned control is selected, document-wide matching loses
				// experience attribution. Visibility emergence preserves layouts whose
				// follow-on panel has no dialog semantics while excluding old page rows.
				const preSelectionVisible = experienceID > 0
					? new WeakSet(all('*'))
					: null;
				click(selectedControl);
				await sleep(500);
				if (/\/checkout\/confirm-purchase/.test(location.href)) {
					return { ok: true, step: 'experience_card', detail: controls[0].cardText };
				}
				const modalResult = await driveExperienceModal(
					preSelectionVisible,
					selectedTightCard
				);
				if (modalResult !== null) return modalResult;
				// No modal appeared: legacy layout with a separate submit control.
				const isFallbackSubmit = (el) => isBookNowControl(el) || el.type === 'submit';
				const submitCandidates = all('button')
					.filter((el) => el !== selectedControl)
					.filter(isFallbackSubmit);
				// The selection is trustworthy by this point (positively tied or
				// the sole candidate), so the fallback's only job is submitting
				// THAT experience. A form is valid scope only when it owns exactly
				// one fallback candidate and no other Book now control. Even then,
				// a candidate inside a different explicit card cannot inherit either
				// the form's scope or the selected card's containment scope. Explicit-
				// card containment is valid only when the selected card contains no
				// competing Book now control. A fallback
				// outside those boundaries must positively tie itself to the pin;
				// unpinned requests keep the historical ungated behavior.
				const selectedForm = formOf(selectedControl);
				const tightCard = explicitCardOf(selectedControl);
				const tightCardValid = tightCard !== null &&
					!bookNowControls.some((other) => other !== selectedControl && tightCard.contains(other));
				const selectedFormCandidates = selectedForm === null
					? []
					: submitCandidates.filter((el) => formOf(el) === selectedForm);
				const selectedFormValid =
					selectedForm !== null &&
					selectedFormCandidates.length === 1 &&
					!bookNowControls.some((other) => other !== selectedControl && formOf(other) === selectedForm);
				const scopedToSelection = (el) => {
					const candidateCard = explicitCardOf(el);
					const crossesExplicitCard =
						candidateCard !== null && (!tightCardValid || candidateCard !== tightCard);
					const formScoped =
						selectedFormValid && formOf(el) === selectedForm && !crossesExplicitCard;
					const cardScoped =
						tightCardValid && tightCard.contains(el) && !crossesExplicitCard;
					return formScoped || cardScoped;
				};
				const submitEligible = (el) =>
					experienceID === 0 || scopedToSelection(el) || controlOrCardHasPinnedExperienceLink(el);
				const submit = submitCandidates
					.filter(submitEligible)
					.sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)[0];
				if (submit) {
					click(submit);
					await sleep(250);
				}
				return { ok: true, step: 'experience_card', detail: controls[0].cardText };
			}

			function allWithin(root, selector) {
				// querySelectorAll excludes the root itself, but a proven
				// follow-on root can BE the sole revealed control (unhidden
				// beneath a pre-existing visible ancestor) — it must match too.
				const matches = Array.from(root.querySelectorAll(selector));
				if (root.matches && root.matches(selector)) matches.unshift(root);
				return matches.filter(visible);
			}
			function findDayButton(root) {
				if (!isoDate) return null;
				return allWithin(root, 'button').find((el) =>
					clean(el.textContent) === isoDate || clean(el.getAttribute('aria-label')) === isoDate) || null;
			}
			function slotTimeAncestor(btn, root) {
				const timePattern = /\d{1,2}:\d{2}\s*(?:AM|PM)/i;
				let node = btn.parentElement;
				for (let hops = 0; node && hops < 5; hops++, node = node.parentElement) {
					const text = clean(node.textContent);
					if (text.length > 200) break;
					if (timePattern.test(text)) return { node, text };
					if (node === root) break;
				}
				return null;
			}
			function findSlotBookButton(root) {
				for (const btn of allWithin(root, 'button').filter((el) => /^book$/i.test(clean(el.textContent)))) {
					const row = slotTimeAncestor(btn, root);
					if (row) {
						// Nearest time-bearing ancestor IS the row: match or
						// move on — climbing further reaches the whole list.
						if (row.text.includes(target)) return btn;
					}
				}
				return null;
			}
			function findSearchControl() {
				const selectedTime = all('select').map((el) => el.value || '').find(Boolean) || '';
				const controls = all('a, button')
					.map((el) => ({
						el,
						text: clean(el.textContent || el.getAttribute('aria-label')),
						href: el.href || (el.getAttribute && el.getAttribute('href')) || '',
						top: el.getBoundingClientRect().top,
					}))
					.filter((c) => /^search$/i.test(c.text) && c.href.includes('/search'))
					// Hard requirement, not a preference: a search link carrying a
					// different date (the picker's date control is a calendar widget
					// we cannot drive) would surface same-time rows on the WRONG DAY
					// and book them. Better to fall back to the experience-card flow,
					// whose modal calendar selects the ISO date explicitly.
					.filter((c) => !isoDate || c.href.includes('date=' + encodeURIComponent(isoDate)));
				if (controls.length === 0) return null;
				controls.sort((a, b) => {
					const aTime = selectedTime && a.href.includes('time=' + encodeURIComponent(selectedTime)) ? 1 : 0;
					const bTime = selectedTime && b.href.includes('time=' + encodeURIComponent(selectedTime)) ? 1 : 0;
					if (aTime !== bTime) return bTime - aTime;
					return a.top - b.top;
				});
				return controls[0].el;
			}
			async function clickSearchResultSlot() {
				// Never for a pinned experience: /search is venue-wide and
				// clicks by time only, so it could book a different experience
				// at the same slot. Let the experience-aware card flow handle it.
				if (experienceID > 0) return null;
				let slotBtn = findSlotBookButton(document);
				if (slotBtn) {
					click(slotBtn);
					await sleep(400);
					return { ok: true, step: 'search_result_slot', detail: target };
				}
				const search = findSearchControl();
				if (!search) return null;
				click(search);
				const slotDeadline = Date.now() + 12000;
				while (Date.now() < slotDeadline) {
					if (/\/checkout\/confirm-purchase/.test(location.href)) {
						return { ok: true, step: 'search_result_slot', detail: 'checkout reached from search result' };
					}
					slotBtn = findSlotBookButton(document);
					if (slotBtn) {
						click(slotBtn);
						await sleep(400);
						return { ok: true, step: 'search_result_slot', detail: target };
					}
					await sleep(250);
				}
				const seen = visibleSlotTimes(document);
				return {
					ok: false,
					step: 'search_result_slot',
					detail: 'requested time not offered in search results; visible: ' + (seen.length ? seen.join(', ') : 'none'),
				};
			}
			function visibleSlotTimes(root) {
				const times = new Set();
				for (const btn of allWithin(root, 'button').filter((el) => /^book$/i.test(clean(el.textContent)))) {
					const row = slotTimeAncestor(btn, root);
					if (!row) continue;
					const m = row.text.match(/\d{1,2}:\d{2}\s*(?:AM|PM)/i);
					if (m) times.add(m[0]);
				}
				return Array.from(times);
			}
			function followOnSignals(root) {
				const signals = [];
				for (const btn of allWithin(root, 'button')) {
					if (isoDate && (
						clean(btn.textContent) === isoDate ||
						clean(btn.getAttribute('aria-label')) === isoDate
					)) {
						signals.push(btn);
						continue;
					}
					if (/^book$/i.test(clean(btn.textContent)) && slotTimeAncestor(btn, root)) {
						signals.push(btn);
					}
				}
				return signals;
			}
			function normalizeRoots(roots) {
				const unique = roots.filter((root, index) => roots.indexOf(root) === index);
				return unique.filter((root) =>
					!unique.some((other) => other !== root && other.contains(root))
				);
			}
			function highestNewAncestor(signal, preSelectionVisible) {
				let root = signal;
				for (let parent = signal.parentElement; parent; parent = parent.parentElement) {
					if (parent === document.body || parent === document.documentElement) break;
					if (!visible(parent) || preSelectionVisible.has(parent)) break;
					root = parent;
				}
				return root;
			}
			function resolveNewFollowOnRoot(signals, preSelectionVisible, selectedTightCard) {
				const roots = normalizeRoots(signals.map((signal) =>
					highestNewAncestor(signal, preSelectionVisible)
				));
				const cardRoots = selectedTightCard
					? roots.filter((root) => selectedTightCard.contains(root))
					: [];
				if (cardRoots.length === 1) return { root: cardRoots[0], rootCount: roots.length };
				if (cardRoots.length > 1) return { root: null, rootCount: roots.length };

				const semanticRoots = normalizeRoots(signals
					.map((signal) => signal.closest('dialog, [role="dialog"], [aria-modal="true"]'))
					.filter((root) => root && visible(root) && !preSelectionVisible.has(root)));
				if (semanticRoots.length === 1) return { root: semanticRoots[0], rootCount: roots.length };
				if (semanticRoots.length > 1) return { root: null, rootCount: roots.length };
				return { root: roots.length === 1 ? roots[0] : null, rootCount: roots.length };
			}
			function staticDeepLinkScope(preSelectionVisible) {
				if (!hasPinnedExperiencePath(location.pathname)) return { root: null, count: 0 };
				const requested = followOnSignals(document)
					.filter((control) => preSelectionVisible.has(control))
					.filter((control) => {
						const isDay = isoDate && (
							clean(control.textContent) === isoDate ||
							clean(control.getAttribute('aria-label')) === isoDate
						);
						if (isDay) return true;
						const row = slotTimeAncestor(control, document);
						return row && row.text.includes(target);
					})
					.filter((control) => !cardLinksOnlyOtherExperience(control));
				const tied = requested.filter(controlOrCardHasPinnedExperienceLink);
				const candidates = tied.length > 0 ? tied : requested;
				if (candidates.length !== 1) return { root: null, count: candidates.length };
				const control = candidates[0];
				const card = tightCardOf(control);
				if (card) return { root: card, count: 1 };
				const row = /^book$/i.test(clean(control.textContent))
					? slotTimeAncestor(control, document)
					: null;
				const root = row ? row.node : control.parentElement;
				if (!root || root === document.body || root === document.documentElement) {
					return { root: null, count: 1 };
				}
				return { root, count: 1 };
			}
			function pinnedScopeFailure(detail) {
				return { ok: false, step: 'experience_modal_scope', detail };
			}
			// Tock's experience modal (SPA route /experience/<id>/reservation):
			// calendar day buttons named with the ISO date, then slot rows with
			// per-time "Book" buttons. The modal RESETS the deep link's date to
			// today, so the requested day must be re-selected. Returns null when
			// no modal materializes (other layouts).
			async function driveExperienceModal(preSelectionVisible, selectedTightCard) {
				const detectDeadline = Date.now() + 2500;
				let root = document;
				let scopedSignals = [];
				const allowedOutsideSignals = new WeakSet();
				if (experienceID > 0) {
					let sawNewSignal = false;
					let unresolvedRootCount = 0;
					while (Date.now() < detectDeadline) {
						if (/\/checkout\/confirm-purchase/.test(location.href)) {
							return { ok: true, step: 'experience_card', detail: 'checkout reached without modal' };
						}
						const newSignals = followOnSignals(document)
							.filter((signal) => !preSelectionVisible.has(signal));
						if (newSignals.length > 0) {
							sawNewSignal = true;
							const resolved = resolveNewFollowOnRoot(
								newSignals,
								preSelectionVisible,
								selectedTightCard
							);
							unresolvedRootCount = resolved.rootCount;
							if (resolved.root) {
								root = resolved.root;
								scopedSignals = newSignals.filter((signal) => root.contains(signal));
								newSignals
									.filter((signal) => !root.contains(signal))
									.forEach((signal) => allowedOutsideSignals.add(signal));
								break;
							}
						}
						await sleep(200);
					}
					if (root === document) {
						if (sawNewSignal) {
							return pinnedScopeFailure(
								'pinned experience ' + experienceID + ' exposed post-selection date/slot controls in ' +
								unresolvedRootCount + ' newly revealed roots; no unique boundary attributes them to the selected experience'
							);
						}
						const staticScope = staticDeepLinkScope(preSelectionVisible);
						if (staticScope.count > 1) {
							return pinnedScopeFailure(
								'pinned experience ' + experienceID + ' has ' + staticScope.count +
								' surviving pre-existing date/slot controls on its deep-link page; no unique page-owned candidate is provable'
							);
						}
						if (!staticScope.root) return null;
						root = staticScope.root;
						scopedSignals = followOnSignals(root);
					}
				} else {
					while (Date.now() < detectDeadline) {
						if (findDayButton(document) || findSlotBookButton(document)) break;
						if (/\/checkout\/confirm-purchase/.test(location.href)) {
							return { ok: true, step: 'experience_card', detail: 'checkout reached without modal' };
						}
						await sleep(200);
					}
				}
				const scopeFailure = () => {
					if (experienceID === 0) return null;
					if (!root.isConnected || !visible(root)) {
						return pinnedScopeFailure(
							'pinned experience ' + experienceID + ' follow-on boundary became disconnected or hidden'
						);
					}
					if (scopedSignals.some((signal) => signal.isConnected && !root.contains(signal))) {
						return pinnedScopeFailure(
							'pinned experience ' + experienceID + ' follow-on controls moved outside the proven boundary'
						);
					}
					const escapedSignal = followOnSignals(document).some((signal) =>
						!preSelectionVisible.has(signal) &&
						!root.contains(signal) &&
						!allowedOutsideSignals.has(signal)
					);
					if (escapedSignal) {
						return pinnedScopeFailure(
							'pinned experience ' + experienceID + ' exposed follow-on controls outside the proven boundary'
						);
					}
					return null;
				};
				let failedScope = scopeFailure();
				if (failedScope) return failedScope;
				const dayBtn = findDayButton(root);
				if (experienceID === 0 && !dayBtn && !findSlotBookButton(root)) return null;
				if (dayBtn) {
					click(dayBtn);
					await sleep(400);
				}
				const slotDeadline = Date.now() + 12000;
				while (Date.now() < slotDeadline) {
					failedScope = scopeFailure();
					if (failedScope) return failedScope;
					const slotBtn = findSlotBookButton(root);
					if (slotBtn) {
						click(slotBtn);
						await sleep(400);
						return { ok: true, step: 'experience_modal_slot', detail: target };
					}
					await sleep(250);
				}
				failedScope = scopeFailure();
				if (failedScope) return failedScope;
				const seen = visibleSlotTimes(root);
				return {
					ok: false,
					step: 'experience_modal_slot',
					detail: 'requested time not offered in slot list; visible: ' + (seen.length ? seen.join(', ') : 'none'),
				};
			}
		})()
	`, displayTime, isoDate, partySize, experienceID, tockPinnedExperienceEligibilityJS)
}

func clickComboboxExperienceLayout(ctx context.Context, displayTime, isoDate string, partySize, experienceID int) error {
	js := clickComboboxExperienceLayoutJS(displayTime, isoDate, partySize, experienceID)
	var result tockComboboxClickResult
	awaitPromise := func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &result, awaitPromise)); err != nil {
		return fmt.Errorf("evaluating combobox booking layout: %w", err)
	}
	if !result.OK {
		if result.Step == "" {
			result.Step = "unknown"
		}
		if result.Detail == "" {
			result.Detail = "no matching combobox booking controls"
		}
		return fmt.Errorf("%s: %s", result.Step, result.Detail)
	}
	return nil
}

func tockBookingPageStateHint(ctx context.Context, venueURL string) string {
	hintCtx := ctx
	var hintCancel context.CancelFunc
	if venueURL != "" {
		if recoveredCtx, cancel, err := ensureTockVenuePage(ctx, venueURL); err == nil {
			hintCtx = recoveredCtx
			hintCancel = cancel
		}
	}
	if hintCancel != nil {
		defer hintCancel()
	}
	js := `
		(() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const visible = (el) => {
				if (!el || !el.isConnected) return false;
				const style = window.getComputedStyle(el);
				if (style.visibility === 'hidden' || style.display === 'none') return false;
				const rect = el.getBoundingClientRect();
				return rect.width > 0 && rect.height > 0;
			};
			const summarize = (selector, limit = 16) => Array.from(document.querySelectorAll(selector))
				.filter(visible)
				.map((el) => clean(el.textContent || el.getAttribute('aria-label') || el.getAttribute('placeholder') || el.value))
				.filter(Boolean)
				.slice(0, limit);
			const body = clean(document.body && document.body.innerText).toLowerCase();
			const controls = summarize('button, a, [role="button"], [role="combobox"], select, input[list]');
			const bookControls = controls.filter((t) => /book now|^book$|reserve|reservation|\b(?:am|pm)\b/i.test(t));
			const comboboxes = summarize('select, [role="combobox"], [aria-haspopup="listbox"], input[list], input[aria-autocomplete]');
			const options = summarize('option, [role="option"]', 24);
			const timeOptions = options.filter((t) => /^\d{1,2}:\d{2}\s+(?:AM|PM)$/i.test(t));
			return {
				path: location.pathname,
				combobox_layout_detected: comboboxes.length > 0 && (timeOptions.length > 0 || bookControls.some((t) => /book now/i.test(t))),
				challenge_detected: /verify you are human|checking your browser|access denied|captcha|challenge/.test(body),
				login_wall_detected: /sign in|log in|continue with google|email address/.test(body),
				legacy_slot_present: controls.some((t) => /\d{1,2}:\d{2}\s+(?:AM|PM)/i.test(t) && /book/i.test(t)),
				time_combobox_present: comboboxes.length > 0,
				search_control_present: controls.some((t) => /^search$/i.test(t)),
				experience_card_present: bookControls.some((t) => /book now|reservation/i.test(t)),
				book_control_present: bookControls.length > 0,
				time_option_labels: timeOptions,
				visible_control_labels: controls
			};
		})()
	`
	var state tockBookingPageState
	if err := chromedp.Run(hintCtx, chromedp.Evaluate(js, &state)); err != nil {
		if venueURL != "" && isTargetNavigatedOrClosed(err) {
			recoveredCtx, cancel, recoverErr := ensureTockVenuePage(ctx, venueURL)
			if recoverErr == nil {
				if cancel != nil {
					defer cancel()
				}
				if retryErr := chromedp.Run(recoveredCtx, chromedp.Evaluate(js, &state)); retryErr == nil {
					return mustMarshalTockPageState(normalizeTockBookingPageState(state))
				}
			}
		}
		return `{"path":"<unavailable>"}`
	}
	return mustMarshalTockPageState(normalizeTockBookingPageState(state))
}

func normalizeTockBookingPageState(state tockBookingPageState) tockBookingPageState {
	state.Path = sanitizeTockPath(state.Path)
	state.TimeOptionLabels = normalizeTockTimeLabels(state.TimeOptionLabels)
	state.VisibleControlLabels = normalizeTockControlLabels(state.VisibleControlLabels)
	return state
}

func normalizeTockTimeLabels(labels []string) []string {
	out := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		t, err := time.Parse("3:04 PM", strings.ToUpper(strings.TrimSpace(label)))
		if err != nil {
			continue
		}
		label = t.Format("3:04 PM")
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	return out
}

func normalizeTockControlLabels(labels []string) []string {
	out := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		label = redactTockControlLabel(label)
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	return out
}

func redactTockControlLabel(label string) string {
	trimmed := strings.TrimSpace(label)
	lower := strings.ToLower(trimmed)
	if times := normalizeTockTimeLabels([]string{trimmed}); len(times) == 1 {
		return times[0]
	}
	switch {
	case strings.Contains(lower, "fewer guests"):
		return "Fewer guests"
	case strings.Contains(lower, "more guests"):
		return "More guests"
	case strings.Contains(lower, "book now"):
		return "Book now"
	case lower == "book":
		return "Book"
	case lower == "search":
		return "Search"
	case strings.Contains(lower, "group") && strings.Contains(lower, "reservation"):
		return "Group reservation"
	case lower == "reservation":
		return "Reservation"
	case strings.Contains(lower, "time"):
		return "Time"
	case strings.Contains(lower, "sign in"):
		return "Sign in"
	case strings.Contains(lower, "log in"):
		return "Log in"
	case lower == "skip", lower == "not now", lower == "no thanks", lower == "close", lower == "dismiss":
		return strings.ToUpper(lower[:1]) + lower[1:]
	default:
		return "Other control"
	}
}

func sanitizeTockPath(raw string) string {
	if u, err := url.Parse(raw); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Path != "" {
		return u.Path
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	if strings.HasPrefix(raw, "/") {
		return raw
	}
	return "<non-http-page>"
}

func mustMarshalTockPageState(state any) string {
	b, err := json.Marshal(state)
	if err != nil {
		return `{"path":"<marshal-error>"}`
	}
	return string(b)
}

// checkoutPageStateHint summarizes the checkout page when the receipt never
// arrives: a query-free path, allowlisted controls, and boolean/count state for
// confirmation, SMS, checkbox, and CVC inputs.
func checkoutPageStateHint(ctx context.Context) string {
	js := `
		(() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const controls = Array.from(document.querySelectorAll('button, a, [role="button"]'))
				.map((el) => clean(el.textContent || el.getAttribute('aria-label'))).filter(Boolean).slice(0, 16);
			const confirm = Array.from(document.querySelectorAll('button[type="submit"], button'))
				.find((el) => /reservation|book|complete|confirm/i.test(clean(el.textContent || el.getAttribute('aria-label'))));
			const checkboxes = Array.from(document.querySelectorAll('input[type="checkbox"]'));
			return {
				path: location.pathname,
				confirm_control_present: Boolean(confirm),
				confirm_control_enabled: Boolean(confirm && !confirm.disabled && confirm.getAttribute('aria-disabled') !== 'true'),
				has_cvc_field: Boolean(Array.from(document.querySelectorAll('input'))
					.find((i) => /cvc|cvv|security/i.test((i.placeholder || '') + (i.name || '') + (i.id || '')))),
				checkbox_count: checkboxes.length,
				required_unchecked_count: checkboxes.filter((el) => el.required && !el.checked).length,
				sms_dialog_present: Boolean(document.querySelector('[data-testid="sms-confirmation-dialog-content"], #sms-confirmation-dialog-content')),
				visible_control_labels: controls
			};
		})()
	`
	var state tockCheckoutPageState
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &state)); err != nil {
		return `{"path":"<unavailable>"}`
	}
	state.Path = sanitizeTockPath(state.Path)
	state.VisibleControlLabels = normalizeTockControlLabels(state.VisibleControlLabels)
	return mustMarshalTockPageState(state)
}

// waitForCheckoutPage polls for the URL containing /checkout/confirm-purchase.
func waitForCheckoutPage(ctx context.Context, deadline time.Duration) error {
	stop := time.After(deadline)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	lastLoc := ""
	for {
		var loc string
		if err := chromedp.Location(&loc).Do(ctx); err == nil {
			if strings.Contains(loc, "/checkout/confirm-purchase") {
				return nil
			}
			lastLoc = sanitizeTockPath(loc)
		}
		select {
		case <-tick.C:
		case <-stop:
			return fmt.Errorf("checkout page never reached within %s (last url %q)", deadline, lastLoc)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func waitForReceiptPage(ctx context.Context, deadline time.Duration) (string, error) {
	stop := time.After(deadline)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		var loc string
		if err := chromedp.Location(&loc).Do(ctx); err == nil {
			if strings.Contains(loc, "/receipt") && !strings.Contains(loc, "/cancel") {
				return loc, nil
			}
		}
		select {
		case <-tick.C:
		case <-stop:
			return "", fmt.Errorf("receipt page never reached within %s", deadline)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// waitForReceiptThroughDialogs waits for the receipt page like
// waitForReceiptPage, but also answers post-confirm interstitial dialogs
// that block submission. Observed live 2026-07-09: clicking "Complete
// reservation" opens an SMS opt-in ("Stay in the know about your table…")
// that must be answered before the purchase proceeds. The dialog is always
// DECLINED — booking must not opt the user into text marketing. If the
// dialog swallowed the original submission, the confirm control is clicked
// one more time after the dismissal.
func waitForReceiptThroughDialogs(ctx context.Context, deadline time.Duration) (string, error) {
	return waitForReceiptThroughDialogsWith(ctx, deadline, dismissPostConfirmDialog)
}

func waitForReceiptThroughDialogsWith(ctx context.Context, deadline time.Duration,
	dismiss func(context.Context) (bool, error)) (string, error) {
	stop := time.Now().Add(deadline)
	reclicked := false
	var dismissedAt time.Time
	for {
		var loc string
		if err := chromedp.Location(&loc).Do(ctx); err == nil {
			if strings.Contains(loc, "/receipt") && !strings.Contains(loc, "/cancel") {
				return loc, nil
			}
		}
		if dismissedAt.IsZero() {
			clicked, err := dismiss(ctx)
			switch {
			case err == nil:
				if clicked {
					dismissedAt = time.Now()
				}
			case isTransientNavigationError(err):
				// The confirm click can win the race: the page navigates to
				// the receipt while the dialog probe is still in flight, and
				// chromedp fails with a transient navigated/destroyed-context
				// error even though the booking succeeded. Keep polling so the
				// next iteration's receipt-URL check observes the navigation
				// instead of reporting a placed booking as failed.
			default:
				return "", fmt.Errorf("dismissing post-confirm dialog: %w", err)
			}
		}
		// If the dialog intercepted the original submission, the checkout
		// page sits idle after the dismissal — re-arm the confirm click
		// once. clickPlaceReservation only clicks an enabled control, so a
		// submission already in flight (button disabled/gone) is not
		// double-fired.
		if !dismissedAt.IsZero() && !reclicked && time.Since(dismissedAt) > 4*time.Second &&
			strings.Contains(loc, "/checkout/") {
			reclicked = true
			_ = clickPlaceReservation(ctx)
		}
		if time.Now().After(stop) {
			return "", fmt.Errorf("receipt page never reached within %s", deadline)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(700 * time.Millisecond):
		}
	}
}

// dismissPostConfirmDialog finds a blocking post-confirm dialog and clicks
// its decline/close control with a trusted CDP click. Returns whether a
// control was clicked. Only decline-flavored controls or an explicit close
// are used — never an accept/opt-in or ambiguous lone control.
func dismissPostConfirmDialog(ctx context.Context) (bool, error) {
	js := `
		(() => {
			const dlgText = /stay in the know|text confirmation and updates|receive text/i;
			const declRE = /no thanks|not now|skip|maybe later|decline|continue without/i;
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const visible = (el) => {
				const r = el.getBoundingClientRect();
				return r.width > 0 && r.height > 0;
			};
			const label = (el) => clean([
				el.textContent,
				el.getAttribute('aria-label'),
				el.getAttribute('title'),
			].filter(Boolean).join(' '));
			const controls = (host) => Array.from(
				host.querySelectorAll('button, a, [role="button"]')
			).filter(visible);
			const declineIn = (host) => {
				const candidates = controls(host);
				// Proven live Tock DOM (2026-07-09). Prefer the explicit opt-out
				// control before applying the provider-agnostic fallback below.
				const exact = candidates.find((el) => el.getAttribute('data-testid') === 'sms-skip-button');
				if (exact) return exact;
				const decline = candidates.find((el) => declRE.test(label(el)));
				if (decline) return decline;
				const close = candidates.find((el) => /close|dismiss/i.test(
					clean(el.getAttribute('aria-label') || el.getAttribute('title'))
				));
				if (close) return close;
				return null;
			};
			if (!dlgText.test((document.body && document.body.innerText) || '')) return '';

			// Tock portals this Material UI dialog directly under <body>. The
			// matching alert content and the action buttons are siblings inside
			// role="dialog", so searching only the alert's nearby descendants
			// cannot see the opt-out action.
			const hosts = [];
			const addHost = (host) => {
				if (host && !hosts.includes(host)) hosts.push(host);
			};
			const smsContent = document.querySelector(
				'[data-testid="sms-confirmation-dialog-content"], #sms-confirmation-dialog-content'
			);
			addHost(smsContent && smsContent.closest('dialog, [role="dialog"]'));
			for (const dialog of document.querySelectorAll('dialog, [role="dialog"]')) {
				if (dlgText.test(clean(dialog.textContent)) || /text alerts|sms/i.test(label(dialog))) {
					addHost(dialog);
				}
			}

			let best = null;
			for (const el of Array.from(document.querySelectorAll('body *'))) {
				if (el.children.length > 30) continue;
				const t = clean(el.textContent);
				if (!dlgText.test(t) || t.length > 1500) continue;
				if (best === null || t.length < clean(best.textContent).length) best = el;
			}
			if (!best) return '';
			addHost(best.closest('dialog, [role="dialog"]'));

			// Generic fallback for providers/layouts without dialog semantics:
			// walk all the way from the matching copy toward <body>, because
			// actions may live in a sibling section under a higher ancestor.
			for (let host = best; host && host !== document.body; host = host.parentElement) {
				addHost(host);
			}
			for (const host of hosts) {
				const decline = declineIn(host);
				if (!decline) continue;
				decline.scrollIntoView({ block: 'center' });
				const r = decline.getBoundingClientRect();
				return JSON.stringify({
					x: r.x + r.width / 2,
					y: r.y + r.height / 2,
				});
			}
			return '';
		})()
	`
	var raw string
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(actCtx context.Context) error {
			return page.BringToFront().Do(actCtx)
		}),
		chromedp.Evaluate(js, &raw),
	); err != nil {
		return false, err
	}
	if raw == "" {
		return false, nil
	}
	var pt struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(raw), &pt); err != nil {
		return false, nil
	}
	click := chromedp.ActionFunc(func(actCtx context.Context) error {
		if err := input.DispatchMouseEvent(input.MouseMoved, pt.X, pt.Y).Do(actCtx); err != nil {
			return err
		}
		press := input.DispatchMouseEvent(input.MousePressed, pt.X, pt.Y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1)
		if err := press.Do(actCtx); err != nil {
			return err
		}
		release := input.DispatchMouseEvent(input.MouseReleased, pt.X, pt.Y).
			WithButton(input.Left).WithClickCount(1)
		return release.Do(actCtx)
	})
	if err := chromedp.Run(ctx, click); err != nil {
		return false, err
	}
	return true, nil
}

// fillCVCIfPresent fills the CVC input if found on the page. No-op for
// free venues that don't render a CVC field.
func fillCVCIfPresent(ctx context.Context, cvc string) error {
	if cvc == "" {
		return nil
	}
	js := `
		((cvcValue) => {
			const inputs = Array.from(document.querySelectorAll('input'));
			for (const i of inputs) {
				const ph = (i.placeholder || '').toLowerCase();
				const name = (i.name || '').toLowerCase();
				const id = (i.id || '').toLowerCase();
				if (ph === 'cvc' || ph === 'cvv' || /cvc|cvv|securityCode/i.test(name) || /cvc|cvv|security/i.test(id)) {
					i.focus();
					i.value = cvcValue;
					i.dispatchEvent(new Event('input', { bubbles: true }));
					i.dispatchEvent(new Event('change', { bubbles: true }));
					return true;
				}
			}
			return false;
		})
	`
	var filled bool
	if err := chromedp.Evaluate(fmt.Sprintf("(%s)(%q)", js, cvc), &filled).Do(ctx); err != nil {
		return fmt.Errorf("evaluating CVC fill: %w", err)
	}
	// Not finding a CVC field is fine — venue may not require card.
	return nil
}

// emptyCVCFieldPresent reports whether the checkout page shows a CVC/CVV
// input that is still empty — the signature of a venue blocking on
// per-transaction CVC re-entry. Mirrors fillCVCIfPresent's selector.
func emptyCVCFieldPresent(ctx context.Context) bool {
	js := `
		(() => {
			const inputs = Array.from(document.querySelectorAll('input'));
			for (const i of inputs) {
				const ph = (i.placeholder || '').toLowerCase();
				const name = (i.name || '').toLowerCase();
				const id = (i.id || '').toLowerCase();
				if (ph === 'cvc' || ph === 'cvv' || /cvc|cvv|securityCode/i.test(name) || /cvc|cvv|security/i.test(id)) {
					return (i.value || '').trim() === '';
				}
			}
			return false;
		})()
	`
	var present bool
	if err := chromedp.Evaluate(js, &present).Do(ctx); err != nil {
		return false
	}
	return present
}

// checkAcknowledgeIfPresent ticks the cancellation-policy checkbox if present.
// Selector is narrowed to checkboxes whose label/aria-label matches policy
// keywords (cancellation, agree, acknowledge, terms) AND does NOT match
// marketing keywords (newsletter, subscribe, promotional, marketing, offers).
// This prevents the booking flow from silently consenting to data-sharing or
// email opt-in checkboxes that may co-render on the checkout page.
func checkAcknowledgeIfPresent(ctx context.Context) error {
	js := `
		(() => {
			const policyRE  = /cancellation|policy|agree|acknowledg|terms|conditions/i;
			const optInRE   = /newsletter|subscrib|promotion|marketing|offers|(?:promo|marketing|promotional) email|sms|text message/i;
			const labelText = (cb) => {
				const wrap = cb.closest('label');
				if (wrap && wrap.textContent) return wrap.textContent;
				if (cb.id) {
					const lbl = document.querySelector('label[for="' + CSS.escape(cb.id) + '"]');
					if (lbl && lbl.textContent) return lbl.textContent;
				}
				return cb.getAttribute('aria-label') || '';
			};
			const cbs = Array.from(document.querySelectorAll('input[type="checkbox"]'));
			let clicked = 0;
			for (const cb of cbs) {
				if (cb.checked) continue;
				const t = labelText(cb).trim();
				if (!t) continue;
				if (!policyRE.test(t)) continue;
				if (optInRE.test(t)) continue;
				cb.click();
				clicked++;
			}
			return clicked;
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
			// Synthetic JS clicks (isTrusted=false) do not submit this form —
			// confirmed live 2026-07-08: no alert, button present, click ignored.
			// Tag the button; the Go side clicks it via trusted CDP input.
			const tag = (b) => {
				b.scrollIntoView({ block: 'center' });
				b.setAttribute('data-trg-confirm', '1');
			};
			const btns = Array.from(document.querySelectorAll('button'));
			for (const b of btns) {
				const t = (b.textContent || '').trim();
				if (/place reservation|confirm reservation|book now|complete reservation|complete booking/i.test(t)) {
					if (b.disabled) return 'disabled:' + t;
					tag(b);
					return t;
				}
			}
			// Fallback: any visible blue/primary submit button at bottom of form
			for (const b of btns) {
				if (b.type === 'submit') {
					if (b.disabled) return 'disabled:submit';
					tag(b);
					return 'submit';
				}
			}
			return null;
		})()
	`
	// Payment context (Braintree) can take a while to enable the confirm
	// control — poll before concluding it is disabled.
	deadline := time.Now().Add(15 * time.Second)
	var label any
	for {
		label = nil
		if err := chromedp.Run(ctx, chromedp.Evaluate(js, &label)); err != nil {
			return fmt.Errorf("evaluating place-reservation click: %w", err)
		}
		s, isString := label.(string)
		if isString && !strings.HasPrefix(s, "disabled:") {
			break
		}
		if time.Now().After(deadline) {
			if label == nil {
				return fmt.Errorf("place-reservation button not found")
			}
			return fmt.Errorf("place-reservation button is disabled (%s) — payment context likely unavailable in this Chrome session; attach mode (TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL) is required for card-required venues", strings.TrimPrefix(s, "disabled:"))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if label == nil {
		return fmt.Errorf("place-reservation button not found")
	}
	// Trusted browser-level click via CDP input — the page's handlers ignore
	// synthetic JS events on this control. The click is dispatched at explicit
	// CSS-pixel coordinates from getBoundingClientRect rather than through
	// chromedp.Click's node machinery: on an attached browser with
	// devicePixelRatio > 1 (Retina), chromedp.Click computes scaled
	// coordinates that land outside the viewport, and Chrome silently drops
	// out-of-bounds input while chromedp reports success (observed live
	// 2026-07-09: zero page events after a nil-error Click at dpr=2).
	rectJS := `
		(() => {
			const b = document.querySelector('button[data-trg-confirm="1"]');
			if (!b) return null;
			b.scrollIntoView({ block: 'center' });
			const r = b.getBoundingClientRect();
			return JSON.stringify({ x: r.x + r.width / 2, y: r.y + r.height / 2 });
		})()
	`
	var rectJSON string
	if err := chromedp.Run(ctx, chromedp.Evaluate(rectJS, &rectJSON)); err != nil {
		return fmt.Errorf("locating place-reservation control: %w", err)
	}
	if rectJSON == "" {
		return fmt.Errorf("place-reservation button vanished before click")
	}
	var pt struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(rectJSON), &pt); err != nil {
		return fmt.Errorf("parsing place-reservation coordinates: %w", err)
	}
	click := chromedp.ActionFunc(func(actCtx context.Context) error {
		// Chrome does not deliver synthesized input to hidden/background
		// tabs on real sites (observed live 2026-07-09: dispatch reports
		// success, page receives zero events, visibilityState "hidden").
		// chromedp creates its target as a background tab, so activate it
		// before dispatching the trusted click.
		if err := page.BringToFront().Do(actCtx); err != nil {
			return fmt.Errorf("bringing checkout tab to front: %w", err)
		}
		if err := input.DispatchMouseEvent(input.MouseMoved, pt.X, pt.Y).Do(actCtx); err != nil {
			return err
		}
		press := input.DispatchMouseEvent(input.MousePressed, pt.X, pt.Y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1)
		if err := press.Do(actCtx); err != nil {
			return err
		}
		release := input.DispatchMouseEvent(input.MouseReleased, pt.X, pt.Y).
			WithButton(input.Left).WithClickCount(1)
		return release.Do(actCtx)
	})
	if err := chromedp.Run(ctx, click); err != nil {
		return fmt.Errorf("clicking place-reservation control: %w", err)
	}
	return nil
}

// parseTockReceipt navigates to the receipt URL (already there post-redirect),
// extracts $REDUX_STATE, and parses the purchase details.
func parseTockReceipt(ctx context.Context, receiptURL string, req BookRequest) (*BookResponse, error) {
	// Pull $REDUX_STATE from the current page (already on receipt).
	var rawState string
	js := `JSON.stringify(window.$REDUX_STATE || null)`
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &rawState)); err != nil {
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
		_ = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText || ''`, &pageText))
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
