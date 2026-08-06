// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package tock

// PATCH: tock-cancel-ui-flow — see
// .printing-press-patches/tock-cancel-ui-flow.json.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

const (
	cancelPhaseReceipt        = "receipt"
	cancelPhaseQuestion       = "cancel_question"
	cancelPhaseOther          = "other"
	cancelStatusConfirmed     = "confirmed"
	cancelStatusCanceled      = "canceled"
	cancelStatusOther         = "other"
	cancelOutcomeSuccess      = "success"
	cancelOutcomeUnknown      = "outcome_unknown"
	cancelOutcomeDefiniteFail = "definite_failure"
)

// cancelState is the only cancel-page state permitted to cross the Chrome
// boundary. Path is canonical and query-free; it never contains a venue slug.
// All other values are canonical categories, booleans, or bounded counts.
type cancelState struct {
	Path                  string `json:"path"`
	Phase                 string `json:"phase"`
	Status                string `json:"status"`
	CancelCandidateCount  int    `json:"cancel_candidate_count"`
	ConfirmCandidateCount int    `json:"confirm_candidate_count"`
	CancelQuestionPresent bool   `json:"cancel_question_present"`
	ExpectedOrigin        bool   `json:"expected_origin"`
	ExpectedReservation   bool   `json:"expected_reservation"`
	SecondClickDispatched bool   `json:"second_click_dispatched"`
	Outcome               string `json:"outcome"`
	ErrorCategory         string `json:"error_category"`
}

type cancelUIError struct {
	kind  error
	state cancelState
}

func (e *cancelUIError) Error() string {
	b, _ := json.Marshal(e.state)
	return e.kind.Error() + ": cancel_state=" + string(b)
}

func (e *cancelUIError) Unwrap() error { return e.kind }

func newCancelUIError(category string, secondClickDispatched bool, state cancelState) error {
	state.SecondClickDispatched = secondClickDispatched
	state.Outcome = cancelOutcomeDefiniteFail
	state.ErrorCategory = category
	kind := ErrCancelUIFlow
	if secondClickDispatched {
		state.Outcome = cancelOutcomeUnknown
		kind = ErrCancelOutcomeUnknown
	}
	return &cancelUIError{kind: kind, state: state}
}

type cancelPageObservation struct {
	state cancelState
}

type cancelDOMObservation struct {
	Canceled              bool    `json:"canceled"`
	Confirmed             bool    `json:"confirmed"`
	CancelCandidates      int     `json:"cancelCandidates"`
	ConfirmCandidates     int     `json:"confirmCandidates"`
	CancelQuestionPresent bool    `json:"cancelQuestionPresent"`
	Phase                 string  `json:"phase"`
	ExpectedOrigin        bool    `json:"expectedOrigin"`
	ExpectedReservation   bool    `json:"expectedReservation"`
	ClickReady            bool    `json:"clickReady"`
	AlreadyCanceled       bool    `json:"alreadyCanceled"`
	ClickCategory         string  `json:"clickCategory"`
	ClickX                float64 `json:"clickX"`
	ClickY                float64 `json:"clickY"`
}

func (c *Client) chromeCancelReservation(ctx context.Context, req CancelRequest) (*CancelResponse, error) {
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
		tmpDir, err := os.MkdirTemp("", "trg-pp-chrome-tock-cancel-")
		if err != nil {
			return nil, newCancelUIError("chrome_unavailable", false, emptyCancelState())
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
	timed, cancelTimed := context.WithTimeout(browserCtx, 45*time.Second)
	defer cancelTimed()

	var cookies []*http.Cookie
	if wsURL == "" && c.session != nil {
		cookies = c.session.HTTPCookies(auth.NetworkTock)
	}
	receiptURL := c.baseOrigin() + "/" + url.PathEscape(req.VenueSlug) + "/receipt?purchaseId=" + strconv.Itoa(req.PurchaseID)
	if err := chromedp.Run(timed,
		network.Enable(),
		injectTockCookies(cookies),
		chromedp.Navigate(receiptURL),
		chromedp.ActionFunc(func(actCtx context.Context) error { return page.BringToFront().Do(actCtx) }),
	); err != nil {
		return nil, newCancelUIError("receipt_navigation_failed", false, emptyCancelState())
	}

	phaseTimeout := c.cancelPhaseTimeout()
	obs, category := c.waitForCancelObservation(timed, req, cancelPhaseReceipt, phaseTimeout, initialReceiptReady)
	if category != "" {
		return nil, newCancelUIError(category, false, obs.state)
	}
	if obs.state.Status == cancelStatusCanceled {
		return canceledResponse(req), nil
	}
	if obs.state.CancelCandidateCount != 1 {
		return nil, newCancelUIError("cancel_control_count", false, obs.state)
	}
	clickObs, dispatched, alreadyCanceled, clickCategory := c.trustedCancelClick(timed, req, cancelPhaseReceipt)
	if alreadyCanceled {
		return canceledResponse(req), nil
	}
	if !dispatched {
		return nil, newCancelUIError(clickCategory, false, clickObs.state)
	}

	obs, category = c.waitForCancelObservation(timed, req, cancelPhaseQuestion, phaseTimeout, cancelQuestionReady)
	if category != "" {
		return nil, newCancelUIError(category, false, obs.state)
	}
	if !obs.state.CancelQuestionPresent {
		return nil, newCancelUIError("cancel_question_missing", false, obs.state)
	}
	if obs.state.ConfirmCandidateCount != 1 {
		return nil, newCancelUIError("confirm_control_count", false, obs.state)
	}
	clickObs, dispatched, _, clickCategory = c.trustedCancelClick(timed, req, cancelPhaseQuestion)
	if !dispatched {
		// This is the irreversible "Cancel reservation" click: an ambiguous
		// mid-dispatch failure may already have canceled, so it must surface
		// as outcome-unknown rather than a retryable UI failure. (The earlier
		// receipt-phase click only opens the confirmation page, so ambiguity
		// there stays safely retryable.)
		return nil, newCancelUIError(
			clickCategory,
			clickCategory == cancelClickDispatchAmbiguous,
			clickObs.state,
		)
	}

	obs, category = c.waitForCancelObservation(timed, req, cancelPhaseReceipt, phaseTimeout, canceledReceiptReady)
	obs.state.SecondClickDispatched = true
	if category != "" {
		return nil, newCancelUIError("result_not_proved", true, obs.state)
	}
	if obs.state.Status != cancelStatusCanceled {
		return nil, newCancelUIError("result_not_proved", true, obs.state)
	}
	return canceledResponse(req), nil
}

func (c *Client) cancelPhaseTimeout() time.Duration {
	if c.cancelUIPhaseTimeout > 0 {
		return c.cancelUIPhaseTimeout
	}
	return 12 * time.Second
}

func canceledResponse(req CancelRequest) *CancelResponse {
	return &CancelResponse{
		Canceled: true, PurchaseID: req.PurchaseID, VenueSlug: req.VenueSlug,
		StatusText: "Reservation canceled",
	}
}

func emptyCancelState() cancelState {
	return cancelState{Path: "/other", Phase: cancelPhaseOther, Status: cancelStatusOther}
}

func (c *Client) waitForCancelObservation(
	ctx context.Context,
	req CancelRequest,
	wantedPhase string,
	timeout time.Duration,
	ready func(cancelState) bool,
) (cancelPageObservation, string) {
	deadline := time.Now().Add(timeout)
	last := cancelPageObservation{state: emptyCancelState()}
	for {
		obs, err := c.readCancelObservation(ctx, req)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return last, phaseFailureCategory(wantedPhase)
			}
			return last, navigationFailureCategory(wantedPhase)
		}
		last = obs
		if !obs.state.ExpectedOrigin || (wantedPhase == cancelPhaseReceipt && obs.state.Phase != cancelPhaseQuestion && !obs.state.ExpectedReservation) {
			return obs, "page_identity_mismatch"
		}
		if obs.state.Phase == wantedPhase && ready(obs.state) {
			return obs, ""
		}
		if time.Now().After(deadline) {
			// Reaching the right identity/phase but not its rendered readiness
			// condition is still a readable state. Let the caller emit the
			// precise missing-heading/control or result-not-proved category.
			if obs.state.Phase == wantedPhase {
				return obs, ""
			}
			return obs, phaseFailureCategory(wantedPhase)
		}
		select {
		case <-ctx.Done():
			return obs, navigationFailureCategory(wantedPhase)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func initialReceiptReady(state cancelState) bool {
	return state.Status == cancelStatusCanceled || state.CancelCandidateCount > 0
}

func cancelQuestionReady(state cancelState) bool {
	return state.CancelQuestionPresent && state.ConfirmCandidateCount > 0
}

func canceledReceiptReady(state cancelState) bool {
	return state.Status == cancelStatusCanceled
}

func navigationFailureCategory(wantedPhase string) string {
	if wantedPhase == cancelPhaseQuestion {
		return "navigation_before_second_click"
	}
	return "receipt_state_unreadable"
}

func phaseFailureCategory(wantedPhase string) string {
	if wantedPhase == cancelPhaseQuestion {
		return "cancel_question_not_reached"
	}
	return "receipt_not_reached"
}

func (c *Client) readCancelObservation(ctx context.Context, req CancelRequest) (cancelPageObservation, error) {
	var dom cancelDOMObservation
	if err := c.evaluateCancelPage(ctx, req, "", &dom); err != nil {
		return cancelPageObservation{}, err
	}
	return cancelObservationFromDOM(dom), nil
}

func cancelObservationFromDOM(dom cancelDOMObservation) cancelPageObservation {
	path := "/other"
	phase := cancelPhaseOther
	switch dom.Phase {
	case cancelPhaseReceipt:
		path, phase = "/receipt", cancelPhaseReceipt
	case cancelPhaseQuestion:
		path, phase = "/receipt/cancel", cancelPhaseQuestion
	}
	status := cancelStatusOther
	if dom.Canceled {
		status = cancelStatusCanceled
	} else if dom.Confirmed {
		status = cancelStatusConfirmed
	}
	return cancelPageObservation{state: cancelState{
		Path: path, Phase: phase, Status: status,
		CancelCandidateCount:  boundedCancelCount(dom.CancelCandidates),
		ConfirmCandidateCount: boundedCancelCount(dom.ConfirmCandidates),
		CancelQuestionPresent: dom.CancelQuestionPresent,
		ExpectedOrigin:        dom.ExpectedOrigin,
		ExpectedReservation:   dom.ExpectedOrigin && phase == cancelPhaseReceipt && dom.ExpectedReservation,
	}}
}

func (c *Client) evaluateCancelPage(ctx context.Context, req CancelRequest, clickPhase string, dom *cancelDOMObservation) error {
	expected, err := url.Parse(c.baseOrigin())
	if err != nil || expected.Scheme == "" || expected.Host == "" {
		return errors.New("invalid cancel origin")
	}
	expectedOrigin := expected.Scheme + "://" + expected.Host
	receiptPath := "/" + url.PathEscape(req.VenueSlug) + "/receipt"
	questionPath := receiptPath + "/cancel"
	script := fmt.Sprintf(cancelObservationJS,
		strconv.Quote(expectedOrigin),
		strconv.Quote(receiptPath),
		strconv.Quote(questionPath),
		strconv.Quote(strconv.Itoa(req.PurchaseID)),
		strconv.Quote(clickPhase),
	)
	return chromedp.Run(ctx, chromedp.Evaluate(script, dom))
}

func boundedCancelCount(n int) int {
	if n < 0 {
		return 0
	}
	if n > 8 {
		return 8
	}
	return n
}

const cancelObservationJS = `
(() => {
	const expectedOrigin = %s;
	const expectedReceiptPath = %s;
	const expectedQuestionPath = %s;
	const expectedPurchaseID = %s;
	const clickPhase = %s;
	  const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
	  const rendered = (el) => {
	    if (!el || !el.isConnected || el.closest('[aria-hidden="true"]')) return false;
	    for (let node = el; node; node = node.parentElement) {
	      const style = getComputedStyle(node);
	      if (style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse' || Number(style.opacity) === 0) return false;
	    }
	    const r = el.getBoundingClientRect();
	    return r.width > 0 && r.height > 0;
  };
  const name = (el) => {
    const labelledby = (el.getAttribute('aria-labelledby') || '').split(/\s+/).filter(Boolean)
      .map((id) => document.getElementById(id)).filter(Boolean).map((n) => n.textContent).join(' ');
    return clean(el.getAttribute('aria-label') || labelledby || el.value || el.textContent);
  };
  const enabled = (el) => !el.disabled && el.getAttribute('aria-disabled') !== 'true';
  const hit = (el) => {
    el.scrollIntoView({block: 'center'});
    const r = el.getBoundingClientRect();
    const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return !!top && (top === el || el.contains(top));
  };
	  const controls = Array.from(document.querySelectorAll('button,a,input[type="button"],input[type="submit"],[role="button"]'));
	  const eligible = (wanted) => controls.filter((el) => rendered(el) && enabled(el) && name(el) === wanted && hit(el));
	  const first = eligible('Cancel');
	  const confirm = eligible('Cancel reservation');
	  const textNodes = Array.from(document.querySelectorAll('[role="status"],[role="alert"],h1,h2,h3,p,div,span')).filter(rendered);
	  const headings = Array.from(document.querySelectorAll('h1,h2,h3,[role="heading"]')).filter(rendered);
	  const canceled = textNodes.some((el) => /^(reservation canceled|reservation cancelled)$/i.test(clean(el.textContent)));
	  const confirmed = textNodes.some((el) => /^reservation confirmed$/i.test(clean(el.textContent)));
	  const cancelQuestionPresent = headings.some((el) => /^do you want to cancel your reservation\??$/i.test(clean(el.textContent)));
	  const originOK = location.origin.toLowerCase() === expectedOrigin.toLowerCase();
	  const phase = location.pathname === expectedReceiptPath ? 'receipt' :
	    (location.pathname === expectedQuestionPath ? 'cancel_question' : 'other');
	  const reservationOK = originOK && phase === 'receipt' && new URL(location.href).searchParams.get('purchaseId') === expectedPurchaseID;
	  const result = {
	    canceled,
	    confirmed,
	    cancelQuestionPresent,
	    phase,
	    expectedOrigin: originOK,
	    expectedReservation: reservationOK,
	    cancelCandidates: Math.min(first.length, 8),
	    confirmCandidates: Math.min(confirm.length, 8),
	    clickReady: false,
	    alreadyCanceled: false,
	    clickCategory: '',
	    clickX: 0,
	    clickY: 0
	  };
	  if (!clickPhase) return result;
	  if (!originOK) {
	    result.clickCategory = 'page_identity_mismatch';
	    return result;
	  }
	  let target;
	  if (clickPhase === 'receipt') {
	    if (phase !== 'receipt' || !reservationOK) {
	      result.clickCategory = 'page_identity_mismatch';
	      return result;
	    }
	    if (canceled) {
	      result.alreadyCanceled = true;
	      return result;
	    }
	    if (first.length !== 1) {
	      result.clickCategory = 'cancel_control_count';
	      return result;
	    }
	    target = first[0];
	  } else if (clickPhase === 'cancel_question') {
	    if (phase !== 'cancel_question') {
	      result.clickCategory = 'page_identity_mismatch';
	      return result;
	    }
	    if (!cancelQuestionPresent) {
	      result.clickCategory = 'cancel_question_missing';
	      return result;
	    }
	    if (confirm.length !== 1) {
	      result.clickCategory = 'confirm_control_count';
	      return result;
	    }
	    target = confirm[0];
	  } else {
	    result.clickCategory = 'control_state_unreadable';
	    return result;
	  }
	  target.scrollIntoView({block: 'center'});
	  const r = target.getBoundingClientRect();
	  const x = r.left + r.width / 2;
	  const y = r.top + r.height / 2;
	  const top = document.elementFromPoint(x, y);
	  if (!rendered(target) || !enabled(target) || !top || (top !== target && !target.contains(top))) {
	    result.clickCategory = 'control_lost_before_click';
	    return result;
	  }
	  result.clickReady = true;
	  result.clickX = x;
	  result.clickY = y;
	  return result;
})()
`

func (c *Client) trustedCancelClick(ctx context.Context, req CancelRequest, phase string) (cancelPageObservation, bool, bool, string) {
	var dom cancelDOMObservation
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
		return page.BringToFront().Do(actCtx)
	})); err != nil {
		return cancelPageObservation{state: emptyCancelState()}, false, false, "control_state_unreadable"
	}
	if err := c.evaluateCancelPage(ctx, req, phase, &dom); err != nil {
		return cancelPageObservation{state: emptyCancelState()}, false, false, "control_state_unreadable"
	}
	obs := cancelObservationFromDOM(dom)
	if dom.AlreadyCanceled {
		return obs, false, true, ""
	}
	category := canonicalCancelClickCategory(dom.ClickCategory)
	if !dom.ClickReady {
		if category == "" {
			category = "control_lost_before_click"
		}
		return obs, false, false, category
	}
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
		return input.DispatchMouseEvent(input.MouseMoved, dom.ClickX, dom.ClickY).Do(actCtx)
	})); err != nil {
		return obs, false, false, "click_dispatch_failed"
	}
	if err := c.dispatchCancelClick(ctx, dom.ClickX, dom.ClickY); err != nil {
		// MousePressed/MouseReleased may have reached Chrome even though the
		// CDP call errored (e.g. the response was lost), so the click cannot
		// be proved undone. Callers on an irreversible phase must map this
		// category to the outcome-unknown classification, never to a
		// retryable pre-click failure.
		return obs, false, false, cancelClickDispatchAmbiguous
	}
	return obs, true, false, ""
}

// cancelClickDispatchAmbiguous marks a click whose press/release phase failed
// mid-dispatch: the button may or may not have activated.
const cancelClickDispatchAmbiguous = "click_dispatch_ambiguous"

func (c *Client) dispatchCancelClick(ctx context.Context, x, y float64) error {
	if c.cancelClickDispatch != nil {
		return c.cancelClickDispatch(ctx, x, y)
	}
	return c.defaultCancelClickDispatch(ctx, x, y)
}

func (c *Client) defaultCancelClickDispatch(ctx context.Context, x, y float64) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
		if err := input.DispatchMouseEvent(input.MousePressed, x, y).
			WithButton(input.Left).WithButtons(1).WithClickCount(1).Do(actCtx); err != nil {
			return err
		}
		return input.DispatchMouseEvent(input.MouseReleased, x, y).
			WithButton(input.Left).WithClickCount(1).Do(actCtx)
	}))
}

func canonicalCancelClickCategory(category string) string {
	switch category {
	case "page_identity_mismatch", "cancel_question_missing", "cancel_control_count",
		"confirm_control_count", "control_lost_before_click", "control_state_unreadable":
		return category
	default:
		return ""
	}
}
