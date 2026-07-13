// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package opentable

// OpenTable attach-mode booking drives the user's existing, signed-in Chrome
// profile instead of replaying /dapi/booking/make-reservation over Surf. The
// latter is routinely blocked by Akamai even when read operations still work.
//
// The final reservation control is deliberately isolated behind
// ChromeBookRequest.Confirm. Callers can run the complete pre-confirm flow with
// Confirm=false to verify selector health without placing a reservation.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var (
	ErrAttachUnreachable      = errors.New("opentable: attach Chrome is unreachable")
	ErrNotSignedIn            = errors.New("opentable: attach Chrome is not signed in")
	ErrSelectorDrift          = errors.New("opentable: booking selector drift")
	ErrFormValidation         = errors.New("opentable: booking form validation failed")
	ErrIncompleteConfirmation = errors.New("opentable: confirmation page omitted reservation identifiers")
)

const (
	openTableChromeDebugEnv           = "TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL"
	openTableFinalLabelPattern        = `^(complete|confirm|make|place) (my )?reservation$|^reserve now$`
	openTableSuccessTextPattern       = `you're all set|you are all set|reservation (is )?confirmed|booking confirmed`
	openTableRequiredAgreementPattern = `\b(agree|acknowledge|accept)\b.*\b(terms|conditions|policy|cancellation)\b|\b(terms|conditions|policy|cancellation)\b.*\b(agree|acknowledge|accept)\b`
	openTableOptionalMarketingPattern = `newsletter|subscrib|promotion|marketing|offer|email updates|dining news|sms|text message|notification|sign me up`
	openTableVenueRefreshLimit        = 1
)

var openTableSlotSelectorCandidates = []string{
	`button[data-test*="slot"]`,
	`button[data-testid*="slot"]`,
	`a[data-test*="slot"]`,
	`a[data-testid*="slot"]`,
	`[role="button"][data-test*="slot"]`,
	`[role="button"][data-testid*="slot"]`,
	`button[aria-label*="AM"]`,
	`button[aria-label*="PM"]`,
	`a[href*="/booking/details"]`,
}

// ChromeBookRequest contains only the inputs needed by the real UI flow.
// RestaurantSlug may also be a numeric restaurant ID.
type ChromeBookRequest struct {
	RestaurantID        int
	RestaurantSlug      string
	ReservationDateTime string
	PartySize           int
	FirstName           string
	LastName            string
	Email               string
	PhoneNumber         string
	Confirm             bool
}

// ChromeBookResult reports either a completed booking or a prepared final
// confirmation control. PageState is sanitized: it contains only a path,
// booleans, and visible control labels; URL queries, cookies, and tokens are
// never included.
type ChromeBookResult struct {
	BookResponse   *BookResponse
	ReadyToConfirm bool
	RestaurantName string
	PageState      string
}

type ChromeBookError struct {
	Kind      error
	Step      string
	PageState string
	Cause     error
}

func (e *ChromeBookError) Error() string {
	parts := []string{e.Kind.Error()}
	if e.Step != "" {
		parts = append(parts, "step="+e.Step)
	}
	if e.Cause != nil {
		parts = append(parts, e.Cause.Error())
	}
	if e.PageState != "" {
		parts = append(parts, "page_state="+e.PageState)
	}
	return strings.Join(parts, ": ")
}

func (e *ChromeBookError) Unwrap() error { return e.Kind }

type openTablePageState struct {
	Path                           string   `json:"path"`
	SignedIn                       bool     `json:"signed_in"`
	LoginWall                      bool     `json:"login_wall"`
	Challenge                      bool     `json:"challenge"`
	SlotUnavailable                bool     `json:"slot_unavailable"`
	FinalConfirmPresent            bool     `json:"final_confirm_present"`
	FinalConfirmEnabled            bool     `json:"final_confirm_enabled"`
	RequiredCheckboxLabels         []string `json:"required_checkbox_labels,omitempty"`
	UncheckedRequiredControlLabels []string `json:"unchecked_required_control_labels,omitempty"`
	VisibleControlLabels           []string `json:"visible_control_labels,omitempty"`
}

type browserControl struct {
	Found    bool    `json:"found"`
	Disabled bool    `json:"disabled"`
	Label    string  `json:"label"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

type requiredCheckboxState struct {
	RequiredLabels  []string `json:"required_labels"`
	CheckedLabels   []string `json:"checked_labels"`
	UncheckedLabels []string `json:"unchecked_labels"`
}

type chromeConfirmationState struct {
	Success            bool   `json:"success"`
	ConfirmationNumber int    `json:"confirmation_number"`
	ReservationID      int    `json:"reservation_id"`
	RestaurantID       int    `json:"restaurant_id"`
	SecurityToken      string `json:"security_token"`
}

// ChromeAttachConfigured is intentionally explicit: booking controls the
// user's real browser only when the runner configured a debug endpoint. The
// availability fallback may still use its historical localhost default.
func ChromeAttachConfigured() bool {
	return strings.TrimSpace(os.Getenv(openTableChromeDebugEnv)) != ""
}

// ChromeBook prepares and, only when Confirm is true, completes an OpenTable
// booking through a fresh tab in the attached Chrome profile.
func (c *Client) ChromeBook(ctx context.Context, req ChromeBookRequest) (*ChromeBookResult, error) {
	date, hhmm, err := validateChromeBookRequest(req)
	if err != nil {
		return nil, err
	}
	debugURL := strings.TrimSpace(os.Getenv(openTableChromeDebugEnv))
	if debugURL == "" {
		return nil, &ChromeBookError{Kind: ErrAttachUnreachable, Step: "discover", Cause: fmt.Errorf("%s is not configured", openTableChromeDebugEnv)}
	}
	wsURL, err := discoverChromeWebSocket(ctx, debugURL)
	if err != nil {
		return nil, &ChromeBookError{Kind: ErrAttachUnreachable, Step: "discover", Cause: err}
	}

	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	timed, cancelTimed := context.WithTimeout(browserCtx, 75*time.Second)
	defer cancelTimed()

	if err := chromedp.Run(timed,
		chromedp.Navigate(Origin+"/user/dining-dashboard"),
		chromedp.ActionFunc(func(actCtx context.Context) error { return page.BringToFront().Do(actCtx) }),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1200*time.Millisecond),
	); err != nil {
		return nil, &ChromeBookError{Kind: ErrAttachUnreachable, Step: "fresh_target", Cause: err}
	}

	state, stateJSON := captureOpenTablePageState(timed)
	if state.LoginWall || !state.SignedIn {
		return nil, &ChromeBookError{Kind: ErrNotSignedIn, Step: "profile_check", PageState: stateJSON}
	}

	pageURL := openTableChromeBookPageURL(req.RestaurantID, req.RestaurantSlug, req.PartySize, date, hhmm)
	if err := chromedp.Run(timed,
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1200*time.Millisecond),
	); err != nil {
		return nil, chromeSelectorError("venue_navigation", timed, err)
	}

	restaurantName := readOpenTableRestaurantName(timed)
	if err := reachOpenTableBookingDetails(timed, hhmm); err != nil {
		return nil, err
	}
	if _, err := waitForOpenTableFinalControlPresent(timed, 12*time.Second); err != nil {
		return nil, chromeSelectorError("booking_form", timed, err)
	}

	if err := fillOpenTableProfileFields(timed, req); err != nil {
		return nil, chromeSelectorError("profile_fields", timed, err)
	}
	required, err := checkOpenTableRequiredPolicies(timed)
	if err != nil {
		return nil, chromeSelectorError("required_policy", timed, err)
	}
	if len(required.UncheckedLabels) > 0 {
		_, pageState := captureOpenTablePageState(timed)
		return nil, newOpenTableFormValidationError("required_policy", pageState, required.UncheckedLabels)
	}
	_, _ = dismissOpenTableOptionalDialog(timed)

	control, err := waitForOpenTableFinalControl(timed, 15*time.Second)
	if err != nil {
		state, _ = captureOpenTablePageState(timed)
		if state.SlotUnavailable {
			return nil, chromeSlotTakenError("final_confirm", timed, fmt.Errorf("slot became unavailable on booking details"))
		}
		return nil, chromeSelectorError("final_confirm", timed, err)
	}
	_, preparedState := captureOpenTablePageState(timed)
	result := &ChromeBookResult{
		ReadyToConfirm: true,
		RestaurantName: restaurantName,
		PageState:      preparedState,
	}
	if !req.Confirm {
		return result, nil
	}

	if err := clickBrowserControl(timed, control); err != nil {
		return nil, chromeSelectorError("final_confirm_click", timed, err)
	}
	confirmation, err := waitForOpenTableConfirmation(timed, control, 35*time.Second)
	if err != nil {
		if errors.Is(err, ErrFormValidation) || errors.Is(err, ErrIncompleteConfirmation) || errors.Is(err, ErrSlotTaken) {
			return nil, err
		}
		state, _ = captureOpenTablePageState(timed)
		if state.SlotUnavailable {
			return nil, chromeSlotTakenError("confirmation_result", timed, fmt.Errorf("slot became unavailable during confirmation"))
		}
		return nil, chromeSelectorError("confirmation_result", timed, err)
	}
	result.ReadyToConfirm = false
	result.BookResponse = &BookResponse{
		ReservationID:       confirmation.ReservationID,
		RestaurantID:        firstNonZero(confirmation.RestaurantID, req.RestaurantID),
		ReservationDateTime: req.ReservationDateTime,
		PartySize:           req.PartySize,
		ConfirmationNumber:  confirmation.ConfirmationNumber,
		SecurityToken:       confirmation.SecurityToken,
		Success:             true,
	}
	return result, nil
}

// reachOpenTableBookingDetails tolerates one transient signed-out venue render.
// OpenTable occasionally paints a logged-out shell in a fresh target even after
// the dashboard proved the shared profile is authenticated. A single reload is
// enough to recover without turning selector failures into an unbounded loop.
func reachOpenTableBookingDetails(ctx context.Context, hhmm string) error {
	var lastErr error
	for attempt := 0; attempt <= openTableVenueRefreshLimit; attempt++ {
		if attempt > 0 {
			if err := chromedp.Run(ctx,
				chromedp.Reload(),
				chromedp.WaitReady("body", chromedp.ByQuery),
				chromedp.Sleep(1200*time.Millisecond),
			); err != nil {
				return chromeSelectorError("venue_refresh", ctx, err)
			}
		}

		if err := clickOpenTableSlot(ctx, hhmm); err != nil {
			lastErr = err
			state, stateJSON := captureOpenTablePageState(ctx)
			switch {
			case errors.Is(err, ErrSlotTaken), state.SlotUnavailable:
				return chromeSlotTakenError("slot_selection", ctx, fmt.Errorf("requested slot %s", hhmm))
			case (state.LoginWall || !state.SignedIn) && attempt < openTableVenueRefreshLimit:
				continue
			case state.LoginWall || !state.SignedIn:
				return &ChromeBookError{Kind: ErrNotSignedIn, Step: "venue_profile_check", PageState: stateJSON, Cause: fmt.Errorf("signed-out venue render persisted after %d refresh", openTableVenueRefreshLimit)}
			default:
				return chromeSelectorError("slot_selection", ctx, err)
			}
		}

		if err := waitForOpenTableBookingDetails(ctx, 18*time.Second); err != nil {
			lastErr = err
			state, stateJSON := captureOpenTablePageState(ctx)
			switch {
			case state.SlotUnavailable:
				return chromeSlotTakenError("booking_details", ctx, fmt.Errorf("requested slot %s", hhmm))
			case (state.LoginWall || !state.SignedIn) && attempt < openTableVenueRefreshLimit:
				continue
			case state.LoginWall || !state.SignedIn:
				return &ChromeBookError{Kind: ErrNotSignedIn, Step: "booking_details", PageState: stateJSON, Cause: fmt.Errorf("signed-out venue render persisted after %d refresh", openTableVenueRefreshLimit)}
			default:
				return chromeSelectorError("booking_details", ctx, err)
			}
		}
		return nil
	}
	return chromeSelectorError("booking_details", ctx, lastErr)
}

func validateChromeBookRequest(req ChromeBookRequest) (date, hhmm string, err error) {
	if req.RestaurantID <= 0 && strings.TrimSpace(req.RestaurantSlug) == "" {
		return "", "", fmt.Errorf("opentable chromebook: restaurant id or slug required")
	}
	if req.PartySize <= 0 {
		return "", "", fmt.Errorf("opentable chromebook: party size must be positive")
	}
	parts := strings.SplitN(req.ReservationDateTime, "T", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("opentable chromebook: ReservationDateTime must be YYYY-MM-DDTHH:MM")
	}
	if _, err := time.Parse("2006-01-02", parts[0]); err != nil {
		return "", "", fmt.Errorf("opentable chromebook: invalid date: %w", err)
	}
	if _, err := time.Parse("15:04", parts[1]); err != nil {
		return "", "", fmt.Errorf("opentable chromebook: invalid time: %w", err)
	}
	return parts[0], parts[1], nil
}

func openTableChromeBookPageURL(restID int, restSlug string, party int, date, hhmm string) string {
	if restID <= 0 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(restSlug)); err == nil {
			restID = parsed
			restSlug = ""
		}
	}
	return chromeAvailPageURL(restID, restSlug, party, date, hhmm)
}

func openTableTimeDisplay(hhmm string) string {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return hhmm
	}
	return t.Format("3:04 PM")
}

func clickOpenTableSlot(ctx context.Context, hhmm string) error {
	selectorsJSON, _ := json.Marshal(openTableSlotSelectorCandidates)
	display := openTableTimeDisplay(hhmm)
	js := fmt.Sprintf(`
		(() => {
			const target = %q;
			const selectors = %s;
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const visible = (el) => {
				if (!el || !el.isConnected) return false;
				const style = getComputedStyle(el), r = el.getBoundingClientRect();
				return style.display !== 'none' && style.visibility !== 'hidden' && r.width > 0 && r.height > 0;
			};
			const label = (el) => clean([el.textContent, el.getAttribute('aria-label'), el.getAttribute('title')].filter(Boolean).join(' '));
			const pool = new Set();
			for (const selector of selectors) for (const el of document.querySelectorAll(selector)) pool.add(el);
			for (const el of document.querySelectorAll('button, a, [role="button"]')) pool.add(el);
			const candidates = Array.from(pool).filter(visible).map((el) => ({el, text: label(el)}))
				.filter((c) => c.text.includes(target))
				.filter((c) => !/next available|notify me|sold out|unavailable/i.test(c.text))
				.sort((a, b) => (a.text === target ? -1 : 0) - (b.text === target ? -1 : 0));
			if (!candidates.length) return {found:false};
			const chosen = candidates[0].el;
			chosen.scrollIntoView({block:'center', inline:'center'});
			const r = chosen.getBoundingClientRect();
			return {found:true, disabled:Boolean(chosen.disabled || chosen.getAttribute('aria-disabled') === 'true'), label:candidates[0].text, x:r.x+r.width/2, y:r.y+r.height/2};
		})()
	`, display, string(selectorsJSON))
	var control browserControl
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &control)); err != nil {
		return err
	}
	if !control.Found {
		return fmt.Errorf("slot control for %q not found", display)
	}
	if control.Disabled {
		return fmt.Errorf("%w: slot control for %q is disabled", ErrSlotTaken, display)
	}
	return clickBrowserControl(ctx, control)
}

func waitForOpenTableBookingDetails(ctx context.Context, deadline time.Duration) error {
	stop := time.Now().Add(deadline)
	for {
		var path string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`location.pathname`, &path)); err == nil && strings.Contains(path, "/booking/details") {
			return nil
		}
		// Some venue layouts insert a seating/experience choice after the time
		// click. Advance only narrowly-labelled non-final controls.
		_, _ = clickOpenTableIntermediateChoice(ctx)
		if time.Now().After(stop) {
			return fmt.Errorf("booking details page not reached within %s", deadline)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

func clickOpenTableIntermediateChoice(ctx context.Context) (bool, error) {
	js := `
		(() => {
			if (/\/authenticate|\/login|\/signin/i.test(location.pathname)) return {found:false};
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const allowed = /^(standard|indoor|outdoor|patio|counter|bar|select|continue)$/i;
			for (const el of document.querySelectorAll('button, [role="button"]')) {
				const text = clean(el.textContent || el.getAttribute('aria-label'));
				const r = el.getBoundingClientRect();
				if (!allowed.test(text) || r.width <= 0 || r.height <= 0 || el.disabled) continue;
				el.scrollIntoView({block:'center'});
				const rr = el.getBoundingClientRect();
				return {found:true, label:text, x:rr.x+rr.width/2, y:rr.y+rr.height/2};
			}
			return {found:false};
		})()
	`
	var control browserControl
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &control)); err != nil {
		return false, err
	}
	if !control.Found {
		return false, nil
	}
	return true, clickBrowserControl(ctx, control)
}

func fillOpenTableProfileFields(ctx context.Context, req ChromeBookRequest) error {
	values, _ := json.Marshal(map[string]string{
		"first": req.FirstName, "last": req.LastName, "email": req.Email, "phone": req.PhoneNumber,
	})
	js := fmt.Sprintf(`
		(() => {
			const values = %s;
			const fields = Array.from(document.querySelectorAll('input'));
			const clean = (s) => (s || '').toLowerCase();
			const set = (input, value) => {
				if (!value || (input.value || '').trim()) return;
				const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
				setter.call(input, value);
				input.dispatchEvent(new Event('input', {bubbles:true}));
				input.dispatchEvent(new Event('change', {bubbles:true}));
			};
			for (const input of fields) {
				const key = clean([input.name,input.id,input.placeholder,input.getAttribute('aria-label')].filter(Boolean).join(' '));
				if (/first/.test(key)) set(input, values.first);
				else if (/last/.test(key)) set(input, values.last);
				else if (/email/.test(key)) set(input, values.email);
				else if (/phone|mobile/.test(key)) set(input, values.phone);
			}
			return true;
		})()
	`, string(values))
	var ok bool
	return chromedp.Run(ctx, chromedp.Evaluate(js, &ok))
}

func checkOpenTableRequiredPolicies(ctx context.Context) (requiredCheckboxState, error) {
	return inspectOpenTableRequiredCheckboxes(ctx, true)
}

func inspectOpenTableRequiredCheckboxes(ctx context.Context, check bool) (requiredCheckboxState, error) {
	js := fmt.Sprintf(`
		(() => {
			const shouldCheck = %t;
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const labelFor = (cb) => {
				if (cb.labels && cb.labels.length) return clean(cb.labels[0].textContent);
				if (cb.id) {
					const explicit = document.querySelector('label[for="' + CSS.escape(cb.id) + '"]');
					if (explicit) return clean(explicit.textContent);
				}
				const ids = clean(cb.getAttribute('aria-labelledby')).split(' ').filter(Boolean);
				const labelled = ids.map((id) => document.getElementById(id)).filter(Boolean).map((el) => el.textContent).join(' ');
				const wrapper = cb.closest('label, [role="group"], fieldset');
				return clean(labelled || cb.getAttribute('aria-label') || (wrapper && wrapper.textContent) || 'Required checkbox').slice(0, 160);
			};
			const finalRE = new RegExp(%q, 'i');
			const final = Array.from(document.querySelectorAll('button, input[type="submit"], [role="button"]'))
				.find((el) => finalRE.test(clean(el.textContent || el.value || el.getAttribute('aria-label'))));
			const form = final && (final.form || final.closest('form'));
			const container = form || (final && final.closest('main'));
			const inBookingForm = (cb) => Boolean(container && (container.contains(cb) || cb.form === form));
			const agreementRE = new RegExp(%q, 'i');
			const marketingRE = new RegExp(%q, 'i');
			const required = (cb) => Boolean(
				cb.required || cb.getAttribute('aria-required') === 'true' ||
				(cb.validity && cb.validity.valueMissing) ||
				(agreementRE.test(labelFor(cb)) && !marketingRE.test(labelFor(cb)))
			);
			const checked = (cb) => cb.matches('input[type="checkbox"]') ? cb.checked : cb.getAttribute('aria-checked') === 'true';
			const controls = Array.from(document.querySelectorAll('input[type="checkbox"], [role="checkbox"]'))
				.filter((cb) => inBookingForm(cb) && !cb.disabled && required(cb));
			const requiredLabels = controls.map(labelFor);
			const checkedLabels = [];
			for (const cb of controls) {
				if (!checked(cb) && shouldCheck) cb.click();
				if (checked(cb)) checkedLabels.push(labelFor(cb));
			}
			return {
				required_labels: requiredLabels,
				checked_labels: checkedLabels,
				unchecked_labels: controls.filter((cb) => !checked(cb)).map(labelFor),
			};
		})()
	`, check, openTableFinalLabelPattern, openTableRequiredAgreementPattern, openTableOptionalMarketingPattern)
	var state requiredCheckboxState
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &state))
	state.RequiredLabels = normalizeOpenTableLabelSlice(state.RequiredLabels)
	state.CheckedLabels = normalizeOpenTableLabelSlice(state.CheckedLabels)
	state.UncheckedLabels = normalizeOpenTableLabelSlice(state.UncheckedLabels)
	return state, err
}

func waitForOpenTableFinalControl(ctx context.Context, deadline time.Duration) (browserControl, error) {
	stop := time.Now().Add(deadline)
	var last browserControl
	for {
		control, err := probeOpenTableFinalControl(ctx)
		if err != nil {
			return browserControl{}, err
		}
		last = control
		if control.Found && !control.Disabled {
			return control, nil
		}
		_, _ = dismissOpenTableOptionalDialog(ctx)
		if time.Now().After(stop) {
			if !last.Found {
				return browserControl{}, fmt.Errorf("final confirmation control not found")
			}
			return browserControl{}, fmt.Errorf("final confirmation control %q remained disabled", last.Label)
		}
		select {
		case <-ctx.Done():
			return browserControl{}, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func waitForOpenTableFinalControlPresent(ctx context.Context, deadline time.Duration) (browserControl, error) {
	stop := time.Now().Add(deadline)
	for {
		control, err := probeOpenTableFinalControl(ctx)
		if err != nil {
			return browserControl{}, err
		}
		if control.Found {
			return control, nil
		}
		if time.Now().After(stop) {
			return browserControl{}, fmt.Errorf("booking form did not render a final confirmation control within %s", deadline)
		}
		select {
		case <-ctx.Done():
			return browserControl{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func probeOpenTableFinalControl(ctx context.Context) (browserControl, error) {
	js := fmt.Sprintf(`
		(() => {
			const re = new RegExp(%q, 'i');
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const controls = Array.from(document.querySelectorAll('button, input[type="submit"], [role="button"]'));
			for (const el of controls) {
				const label = clean(el.textContent || el.value || el.getAttribute('aria-label'));
				const r = el.getBoundingClientRect();
				if (!re.test(label) || r.width <= 0 || r.height <= 0) continue;
				el.scrollIntoView({block:'center'});
				const rr = el.getBoundingClientRect();
				return {found:true, disabled:Boolean(el.disabled || el.getAttribute('aria-disabled') === 'true'), label, x:rr.x+rr.width/2, y:rr.y+rr.height/2};
			}
			return {found:false};
		})()
	`, openTableFinalLabelPattern)
	var control browserControl
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &control))
	return control, err
}

func dismissOpenTableOptionalDialog(ctx context.Context) (bool, error) {
	js := `
		(() => {
			const topic = /sms|text message|text updates|notification|stay in the know|email updates/i;
			const decline = /^(no thanks|not now|skip|maybe later|decline|continue without)$/i;
			const close = /^(close|dismiss)$/i;
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			for (const dialog of document.querySelectorAll('dialog, [role="dialog"], [role="alertdialog"]')) {
				if (!topic.test(clean(dialog.textContent))) continue;
				const controls = Array.from(dialog.querySelectorAll('button, a, [role="button"]'));
				const chosen = controls.find((el) => decline.test(clean(el.textContent || el.getAttribute('aria-label')))) ||
					controls.find((el) => close.test(clean(el.getAttribute('aria-label') || el.getAttribute('title'))));
				if (!chosen) return {found:false};
				const r = chosen.getBoundingClientRect();
				if (r.width <= 0 || r.height <= 0) return {found:false};
				chosen.scrollIntoView({block:'center'});
				const rr = chosen.getBoundingClientRect();
				return {found:true, label:clean(chosen.textContent || chosen.getAttribute('aria-label')), x:rr.x+rr.width/2, y:rr.y+rr.height/2};
			}
			return {found:false};
		})()
	`
	var control browserControl
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &control)); err != nil {
		return false, err
	}
	if !control.Found {
		return false, nil
	}
	return true, clickBrowserControl(ctx, control)
}

func clickBrowserControl(ctx context.Context, control browserControl) error {
	if !control.Found || control.Disabled {
		return fmt.Errorf("browser control is missing or disabled")
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
		if err := page.BringToFront().Do(actCtx); err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MouseMoved, control.X, control.Y).Do(actCtx); err != nil {
			return err
		}
		if err := input.DispatchMouseEvent(input.MousePressed, control.X, control.Y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1).Do(actCtx); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, control.X, control.Y).
			WithButton(input.Left).WithClickCount(1).Do(actCtx)
	}))
}

func waitForOpenTableConfirmation(ctx context.Context, original browserControl, deadline time.Duration) (chromeConfirmationState, error) {
	stop := time.Now().Add(deadline)
	submittedAt := time.Now()
	reclicked := false
	var dismissedAt time.Time
	for {
		confirmation, err := readOpenTableConfirmation(ctx)
		if err == nil && confirmation.Success {
			_, pageState := captureOpenTablePageState(ctx)
			if err := validateOpenTableConfirmation(confirmation, pageState); err != nil {
				return chromeConfirmationState{}, err
			}
			return confirmation, nil
		}
		if dismissedAt.IsZero() {
			if dismissed, dismissErr := dismissOpenTableOptionalDialog(ctx); dismissErr == nil && dismissed {
				dismissedAt = time.Now()
			}
		}
		var path string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`location.pathname`, &path))
		if strings.Contains(path, "/booking/details") && time.Since(submittedAt) > 1200*time.Millisecond {
			required, requiredErr := inspectOpenTableRequiredCheckboxes(ctx, false)
			if requiredErr == nil && len(required.UncheckedLabels) > 0 {
				_, pageState := captureOpenTablePageState(ctx)
				return chromeConfirmationState{}, newOpenTableFormValidationError("confirmation_result", pageState, required.UncheckedLabels)
			}
		}
		if !dismissedAt.IsZero() && !reclicked && time.Since(dismissedAt) > 3*time.Second && strings.Contains(path, "/booking/details") {
			reclicked = true
			if fresh, probeErr := probeOpenTableFinalControl(ctx); probeErr == nil && fresh.Found && !fresh.Disabled {
				_ = clickBrowserControl(ctx, fresh)
			} else if original.Found {
				_ = clickBrowserControl(ctx, original)
			}
		}
		if time.Now().After(stop) {
			if strings.Contains(path, "/booking/details") {
				_, pageState := captureOpenTablePageState(ctx)
				return chromeConfirmationState{}, &ChromeBookError{
					Kind: ErrFormValidation, Step: "confirmation_result", PageState: pageState,
					Cause: fmt.Errorf("final confirmation click produced no navigation within %s; no unchecked required checkbox was detected", deadline),
				}
			}
			return chromeConfirmationState{}, fmt.Errorf("confirmation page not reached within %s", deadline)
		}
		select {
		case <-ctx.Done():
			return chromeConfirmationState{}, ctx.Err()
		case <-time.After(600 * time.Millisecond):
		}
	}
}

func readOpenTableConfirmation(ctx context.Context) (chromeConfirmationState, error) {
	js := fmt.Sprintf(`
		(() => {
			const text = (document.body && document.body.innerText) || '';
			const success = /%s/i.test(text) || /booking\/(confirmation|receipt)/i.test(location.pathname);
			const out = {success, confirmation_number:0, reservation_id:0, restaurant_id:0, security_token:''};
			const seen = new Set();
			const walk = (value, depth) => {
				if (!value || typeof value !== 'object' || depth > 8 || seen.has(value)) return;
				seen.add(value);
				for (const [key, child] of Object.entries(value)) {
					const k = key.toLowerCase();
					if (!out.confirmation_number && k === 'confirmationnumber') out.confirmation_number = Number(child) || 0;
					else if (!out.reservation_id && k === 'reservationid') out.reservation_id = Number(child) || 0;
					else if (!out.restaurant_id && k === 'restaurantid') out.restaurant_id = Number(child) || 0;
					else if (!out.security_token && k === 'securitytoken' && typeof child === 'string') out.security_token = child;
					if (child && typeof child === 'object') walk(child, depth + 1);
				}
			};
			walk(window.__INITIAL_STATE__ || {}, 0);
			if (!out.confirmation_number) {
				const m = text.match(/confirmation(?: number| #|:)\s*#?\s*(\d{4,})/i);
				if (m) out.confirmation_number = Number(m[1]) || 0;
			}
			return out;
		})()
	`, openTableSuccessTextPattern)
	var state chromeConfirmationState
	err := chromedp.Run(ctx, chromedp.Evaluate(js, &state))
	return state, err
}

func validateOpenTableConfirmation(confirmation chromeConfirmationState, pageState string) error {
	if !confirmation.Success {
		return nil
	}
	if confirmation.ConfirmationNumber != 0 || confirmation.ReservationID != 0 || confirmation.SecurityToken != "" {
		return nil
	}
	return &ChromeBookError{
		Kind: ErrIncompleteConfirmation, Step: "confirmation_extract", PageState: pageState,
		Cause: fmt.Errorf("confirmation UI was visible but confirmationNumber, reservationID, and securityToken were all absent"),
	}
}

func captureOpenTablePageState(ctx context.Context) (openTablePageState, string) {
	js := fmt.Sprintf(`
		(() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const body = clean(document.body && document.body.innerText).toLowerCase();
			const initial = window.__INITIAL_STATE__ || {};
			const profile = initial.userProfile || {};
			const dashboard = initial.diningDashboard || {};
			const controls = Array.from(document.querySelectorAll('button, a, [role="button"]'))
				.map((el) => ({el, text:clean(el.textContent || el.getAttribute('aria-label'))}))
				.filter((item) => { const r=item.el.getBoundingClientRect(); return item.text && r.width>0 && r.height>0; })
				.map((item) => item.text.slice(0,80)).slice(0,16);
			const finalRE = new RegExp(%q, 'i');
			const final = Array.from(document.querySelectorAll('button, input[type="submit"], [role="button"]'))
				.find((el) => finalRE.test(clean(el.textContent || el.value || el.getAttribute('aria-label'))));
			const form = final && (final.form || final.closest('form'));
			const container = form || (final && final.closest('main'));
			const labelFor = (cb) => {
				if (cb.labels && cb.labels.length) return clean(cb.labels[0].textContent);
				if (cb.id) {
					const explicit = document.querySelector('label[for="' + CSS.escape(cb.id) + '"]');
					if (explicit) return clean(explicit.textContent);
				}
				const ids = clean(cb.getAttribute('aria-labelledby')).split(' ').filter(Boolean);
				const labelled = ids.map((id) => document.getElementById(id)).filter(Boolean).map((el) => el.textContent).join(' ');
				const wrapper = cb.closest('label, [role="group"], fieldset');
				return clean(labelled || cb.getAttribute('aria-label') || (wrapper && wrapper.textContent) || 'Required checkbox').slice(0, 160);
			};
			const requiredCheckboxes = Array.from(document.querySelectorAll('input[type="checkbox"], [role="checkbox"]'))
				.filter((cb) => container && (container.contains(cb) || cb.form === form))
				.filter((cb) => {
					const agreementRE = new RegExp(%q, 'i');
					const marketingRE = new RegExp(%q, 'i');
					return !cb.disabled && (cb.required || cb.getAttribute('aria-required') === 'true' ||
						(cb.validity && cb.validity.valueMissing) ||
						(agreementRE.test(labelFor(cb)) && !marketingRE.test(labelFor(cb))));
				});
			const isChecked = (cb) => cb.matches('input[type="checkbox"]') ? cb.checked : cb.getAttribute('aria-checked') === 'true';
			const loginWall = /\/authenticate|\/login|\/signin|\/account\/login/i.test(location.pathname) ||
				(/sign in|log in/.test(body) && /email address|continue with google|password/.test(body));
			let initialAuthenticated = false;
			const seen = new Set();
			const findAuth = (value, depth) => {
				if (!value || typeof value !== 'object' || depth > 8 || seen.has(value)) return;
				seen.add(value);
				for (const [key, child] of Object.entries(value)) {
					if (key === 'isAuthenticated' && child === true) initialAuthenticated = true;
					if (child && typeof child === 'object') findAuth(child, depth + 1);
				}
			};
			findAuth(initial, 0);
			const accountControl = document.querySelector('a[href*="/user/dining-dashboard"], a[href*="/my/reservations"], button[aria-label*="profile" i], button[aria-label*="account" i]');
			const signedIn = Boolean(initialAuthenticated || accountControl || profile.email || profile.firstName || profile.lastName ||
				Array.isArray(dashboard.upcomingReservations) ||
				(/\/user\/dining-dashboard/.test(location.pathname) && !loginWall && /upcoming|dining history|reservations/.test(body)));
			return {
				path: location.pathname,
				signed_in: signedIn,
				login_wall: loginWall,
				challenge: /verify you are human|checking your browser|access denied|captcha|challenge/.test(body),
				slot_unavailable: /slot (is )?(no longer )?available|reservation time is no longer available|just booked|sold out/.test(body),
				final_confirm_present: Boolean(final),
				final_confirm_enabled: Boolean(final && !final.disabled && final.getAttribute('aria-disabled') !== 'true'),
				required_checkbox_labels: requiredCheckboxes.map(labelFor),
				unchecked_required_control_labels: requiredCheckboxes.filter((cb) => !isChecked(cb)).map(labelFor),
				visible_control_labels: controls,
			};
		})()
	`, openTableFinalLabelPattern, openTableRequiredAgreementPattern, openTableOptionalMarketingPattern)
	var state openTablePageState
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &state)); err != nil {
		state = openTablePageState{Path: "<unavailable>"}
	}
	state = normalizeOpenTablePageState(state)
	return state, mustMarshalPageState(state)
}

func mustMarshalPageState(state openTablePageState) string {
	state = normalizeOpenTablePageState(state)
	b, err := json.Marshal(state)
	if err != nil {
		return `{"path":"<marshal-error>"}`
	}
	return string(b)
}

func normalizeOpenTablePageState(state openTablePageState) openTablePageState {
	state.Path = sanitizeOpenTablePath(state.Path)
	visible := make([]string, 0, len(state.VisibleControlLabels))
	seen := make(map[string]struct{}, len(state.VisibleControlLabels))
	for _, label := range state.VisibleControlLabels {
		if strings.Contains(state.Path, "/booking/details") && isOpenTableAccountSwitchLabel(label) {
			state.SignedIn = true
		}
		label = redactOpenTableVisibleControlLabel(label)
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		visible = append(visible, label)
	}
	state.VisibleControlLabels = visible
	state.RequiredCheckboxLabels = normalizeOpenTableLabelSlice(state.RequiredCheckboxLabels)
	state.UncheckedRequiredControlLabels = normalizeOpenTableLabelSlice(state.UncheckedRequiredControlLabels)
	return state
}

func normalizeOpenTableLabelSlice(labels []string) []string {
	out := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		label = redactOpenTablePolicyLabel(label)
		if len(label) > 160 {
			label = strings.TrimSpace(label[:160])
		}
		if label == "" {
			label = "Required checkbox"
		}
		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	return out
}

func redactOpenTablePolicyLabel(label string) string {
	redacted := redactOpenTableControlLabel(label)
	if redacted != strings.TrimSpace(label) {
		return redacted
	}
	lower := strings.ToLower(redacted)
	switch lower {
	case "account identity", "personal contact", "required agreement", "marketing choice", "required checkbox":
		return redacted
	}
	agreement := strings.Contains(lower, "agree") || strings.Contains(lower, "acknowledge") || strings.Contains(lower, "accept")
	policy := strings.Contains(lower, "terms") || strings.Contains(lower, "conditions") || strings.Contains(lower, "policy") || strings.Contains(lower, "cancellation")
	if agreement && policy {
		return "Required agreement"
	}
	for _, word := range []string{"newsletter", "marketing", "offers", "email updates", "dining news", "sms", "text message", "notification"} {
		if strings.Contains(lower, word) {
			return "Marketing choice"
		}
	}
	return "Required checkbox"
}

func isOpenTableAccountSwitchLabel(label string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(label))
	return strings.HasPrefix(trimmed, "not ") && strings.HasSuffix(trimmed, "?")
}

func redactOpenTableControlLabel(label string) string {
	trimmed := strings.TrimSpace(label)
	switch {
	case isOpenTableAccountSwitchLabel(trimmed):
		return "Account switch"
	case strings.Contains(trimmed, "@"):
		return "Account identity"
	case looksLikeOpenTablePhoneLabel(trimmed):
		return "Personal contact"
	default:
		return trimmed
	}
}

func redactOpenTableVisibleControlLabel(label string) string {
	redacted := redactOpenTableControlLabel(label)
	if redacted != strings.TrimSpace(label) {
		return redacted
	}
	lower := strings.ToLower(redacted)
	known := map[string]struct{}{
		"complete reservation": {}, "confirm reservation": {}, "make reservation": {},
		"make my reservation": {}, "place reservation": {}, "place my reservation": {},
		"reserve now": {}, "continue": {}, "standard": {}, "indoor": {},
		"outdoor": {}, "patio": {}, "counter": {}, "bar": {}, "select": {},
		"sign in": {}, "log in": {}, "account": {}, "profile": {},
		"dining history": {}, "upcoming reservations": {}, "menu": {},
		"close": {}, "dismiss": {}, "no thanks": {}, "not now": {}, "skip": {},
		"back": {}, "reload": {}, "account switch": {}, "account identity": {},
		"personal contact": {}, "other control": {},
	}
	if _, ok := known[lower]; ok {
		return redacted
	}
	return "Other control"
}

func looksLikeOpenTablePhoneLabel(label string) bool {
	digits := 0
	for _, r := range label {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 7
}

func sanitizeOpenTablePath(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return u.Path
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

func chromeSelectorError(step string, ctx context.Context, cause error) error {
	_, state := captureOpenTablePageState(ctx)
	return &ChromeBookError{Kind: ErrSelectorDrift, Step: step, PageState: state, Cause: cause}
}

func chromeSlotTakenError(step string, ctx context.Context, cause error) error {
	_, state := captureOpenTablePageState(ctx)
	return &ChromeBookError{Kind: ErrSlotTaken, Step: step, PageState: state, Cause: cause}
}

func newOpenTableFormValidationError(step, pageState string, unchecked []string) error {
	labels := normalizeOpenTableLabelSlice(unchecked)
	cause := fmt.Errorf("final confirmation click was blocked by still-unchecked required controls: %s", strings.Join(labels, "; "))
	return &ChromeBookError{Kind: ErrFormValidation, Step: step, PageState: pageState, Cause: cause}
}

func readOpenTableRestaurantName(ctx context.Context) string {
	js := `
		(() => {
			const el = document.querySelector('h1, [data-test*="restaurant-name"], [data-testid*="restaurant-name"]');
			return ((el && el.textContent) || '').replace(/\s+/g, ' ').trim().slice(0, 160);
		})()
	`
	var name string
	_ = chromedp.Run(ctx, chromedp.Evaluate(js, &name))
	return name
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
