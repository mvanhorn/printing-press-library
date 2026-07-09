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
// CVC handling: Tock requires per-transaction CVC re-entry even when the
// card is on file. The CLI either prompts interactively or, in agent/no-input
// mode, requires TRG_TOCK_CVC before ChromeBook() is called. Per system rules,
// only CVC (3-4 digits) is accepted — the full card number stays on the user's
// Tock profile.

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
	"github.com/chromedp/cdproto/network"
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

	// Inject Tock cookies (session auth) before navigation. Nil-safe: if
	// the client was constructed without a session (e.g., unit tests),
	// proceed with no cookies — Chrome will be unauthenticated.
	var cookies []*http.Cookie
	if c.session != nil {
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
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return nil, fmt.Errorf("tock chromebook: %w", err)
	}

	// Find and click the requested booking control. Legacy Tock pages expose
	// one button per slot time; newer experience-card pages expose a time
	// combobox plus "Book now" controls on experience cards. The helper may
	// reattach to a recovered Tock tab if the original target has gone stale.
	activeCtx, activeCancel, err := clickRequestedTockBookingControl(timed, venueURL, displayTime, req.PartySize, req.ExperienceID)
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
		// Wait for receipt-page navigation.
		chromedp.ActionFunc(func(actCtx context.Context) error {
			u, err := waitForReceiptPage(actCtx, 30*time.Second)
			if err != nil {
				// Distinguish "stalled on a required CVC we don't have" from
				// generic checkout failure so machine callers get a typed,
				// actionable outcome instead of a timeout.
				if req.CVC == "" && emptyCVCFieldPresent(actCtx) {
					return ErrCVCRequired
				}
				return err
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
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &clicked)); err != nil {
		return fmt.Errorf("evaluating slot click: %w", err)
	}
	if !clicked {
		return fmt.Errorf("slot button for %q not found", displayTime)
	}
	return nil
}

func clickRequestedTockBookingControl(ctx context.Context, venueURL, displayTime string, partySize, experienceID int) (context.Context, context.CancelFunc, error) {
	legacyErr := clickSlotByTimeText(ctx, displayTime)
	if legacyErr == nil {
		return ctx, nil, nil
	}

	activeCtx, activeCancel, ensureErr := ensureTockVenuePage(ctx, venueURL)
	comboboxErr := ensureErr
	if comboboxErr == nil {
		var retryCtx context.Context
		var retryCancel context.CancelFunc
		retryCtx, retryCancel, comboboxErr = clickComboboxExperienceLayoutWithRetry(activeCtx, venueURL, displayTime, partySize, experienceID)
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
	return activeCtx, nil, fmt.Errorf("%w: requested_time=%q legacy_slot_error=%v combobox_layout_error=%v page_state=%s",
		ErrSlotControlNotFound, displayTime, legacyErr, comboboxErr, tockBookingPageStateHint(hintCtx, venueURL))
}

type tockComboboxClickResult struct {
	OK     bool   `json:"ok"`
	Step   string `json:"step"`
	Detail string `json:"detail"`
}

func clickComboboxExperienceLayoutWithRetry(ctx context.Context, venueURL, displayTime string, partySize, experienceID int) (context.Context, context.CancelFunc, error) {
	if err := clickComboboxExperienceLayout(ctx, displayTime, partySize, experienceID); err != nil {
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
		if retryErr := clickComboboxExperienceLayout(retryCtx, displayTime, partySize, experienceID); retryErr != nil {
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

// clickComboboxExperienceLayout drives Tock's newer booking layout:
// choose the requested time from a combobox/listbox, pick the best matching
// experience card, then click its "Book now" control.
func clickComboboxExperienceLayout(ctx context.Context, displayTime string, partySize, experienceID int) error {
	js := fmt.Sprintf(`
		(async () => {
			const target = %q;
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

			for (const select of all('select')) {
				const options = Array.from(select.options || []);
				const match = options.find((opt) => clean(opt.textContent || opt.label || opt.value) === target);
				if (match) {
					select.value = match.value;
					fireChange(select);
					await sleep(150);
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
				return await clickBestExperienceBookNow();
			}

			const anyBookNow = all('a, button')
				.some((el) => /book now/i.test(clean(el.textContent || el.getAttribute('aria-label'))));
			const anyTimeCombobox = all('select, [role="combobox"], [aria-haspopup="listbox"], input[list]')
				.some((el) => /time|\b(?:AM|PM)\b/i.test(labelFor(el)));
			if (anyBookNow && !anyTimeCombobox) {
				// Deep-link layout: date/time/party rode the URL, no picker is
				// rendered - the experience card IS the whole flow.
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
				if (experienceID > 0 && href.includes('/experience/' + experienceID)) score += 100;
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
				const controls = all('a, button')
					.filter((el) => /book now/i.test(clean(el.textContent || el.getAttribute('aria-label'))))
					.map(scoreBookControl)
					.sort((a, b) => (b.score - a.score) || (a.top - b.top));
				if (controls.length === 0) {
					return { ok: false, step: 'experience_card', detail: 'no Book now controls found after selecting time' };
				}
				click(controls[0].control);
				await sleep(500);
				if (!/\/checkout\/confirm-purchase/.test(location.href)) {
					const submit = all('button')
						.filter((el) => el !== controls[0].control)
						.filter((el) => /book now/i.test(clean(el.textContent || el.getAttribute('aria-label'))) || el.type === 'submit')
						.sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)[0];
					if (submit) {
						click(submit);
						await sleep(250);
					}
				}
				return { ok: true, step: 'experience_card', detail: controls[0].cardText };
			}
		})()
	`, displayTime, partySize, experienceID)
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
			const summarize = (selector, limit = 12) => Array.from(document.querySelectorAll(selector))
				.filter(visible)
				.map((el) => clean(el.textContent || el.getAttribute('aria-label') || el.getAttribute('placeholder') || el.value))
				.filter(Boolean)
				.slice(0, limit);
			const body = clean(document.body && document.body.innerText).toLowerCase();
			const bookControls = summarize('button, a').filter((t) => /book now|reserve|reservation|am|pm/i.test(t));
			const comboboxes = summarize('select, [role="combobox"], [aria-haspopup="listbox"], input[list], input[aria-autocomplete]');
			const options = summarize('option, [role="option"]', 20);
			return JSON.stringify({
				url: location.href,
				combobox_layout_detected: comboboxes.length > 0 && (options.some((t) => /\b(?:AM|PM)\b/.test(t)) || bookControls.some((t) => /book now/i.test(t))),
				challenge_detected: /verify you are human|checking your browser|access denied|captcha|challenge/.test(body),
				login_wall_detected: /sign in|log in|continue with google|email address/.test(body),
				comboboxes,
				options,
				book_controls: bookControls
			});
		})()
	`
	var state string
	if err := chromedp.Run(hintCtx, chromedp.Evaluate(js, &state)); err != nil {
		if venueURL != "" && isTargetNavigatedOrClosed(err) {
			recoveredCtx, cancel, recoverErr := ensureTockVenuePage(ctx, venueURL)
			if recoverErr == nil {
				if cancel != nil {
					defer cancel()
				}
				if retryErr := chromedp.Run(recoveredCtx, chromedp.Evaluate(js, &state)); retryErr == nil && state != "" {
					return state
				}
			}
		}
		return fmt.Sprintf(`{"hint_error":%q}`, err.Error())
	}
	if state == "" {
		return `{}`
	}
	return state
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

func waitForReceiptPage(ctx context.Context, deadline time.Duration) (string, error) {
	stop := time.After(deadline)
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		var loc string
		if err := chromedp.Location(&loc).Do(ctx); err == nil {
			if strings.Contains(loc, "/receipt") && strings.Contains(loc, "purchaseId=") && !strings.Contains(loc, "/cancel") {
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
