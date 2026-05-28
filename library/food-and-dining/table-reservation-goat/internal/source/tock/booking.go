// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package tock

// PATCH: cross-network-source-clients (booking) — see .printing-press-patches.json.
//
// Tock booking flow (captured live 2026-05-09 via chrome-MCP):
//
// SSR shape (captured 2026-05-09):
//   Book endpoint:    POST /<venue-slug>/checkout/confirm-purchase
//                     Content-Type: application/x-www-form-urlencoded
//                     (NOT XHR — traditional form-submit page navigation)
//   Cancel endpoint:  POST /<venue-slug>/receipt/cancel
//                     Content-Type: application/x-www-form-urlencoded
//   List endpoint:    GET /profile/upcoming
//                     Parse $REDUX_STATE.patron.purchaseSummaries[]
//
// CRITICAL ARCHITECTURAL NOTE: Tock's book/cancel use traditional form-submit
// page navigation (the browser POSTs and follows redirects to a receipt page).
// Zero XHRs to www.exploretock.com fired during capture. fetch+XHR
// interceptors are bypassed entirely. This means:
//   1. Book() must POST form-encoded body, follow redirects, parse the
//      receipt page's $REDUX_STATE for the result.
//   2. The form body shape was NOT successfully captured (chrome-mcp privacy
//      filter blocked the body content). Implementation deferred to a
//      follow-up capture session OR a chromedp-attach implementation
//      mirroring `internal/source/opentable/chrome_avail.go`.
//
// For v0.2: Book() returns a typed sentinel error directing callers to the
// venue URL. Cancel() and ListUpcomingReservations() are best-effort using
// the URL patterns we did capture.
//
// Confirmation format: TOCK-R-XXXXXXXX (e.g., TOCK-R-CTJO2LDS)
// purchaseId format:    integer (e.g., 362575651)
// Slot-lock TTL:        ~10 minutes (UI shows "Holding reservation for 9:55")
// Cancellation policy:  per-venue; rendered in receipt page text.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Sentinel errors for typed-error handling at the CLI boundary.
var (
	// ErrBookingNotImplemented signals that Tock's book flow requires either
	// a future form-submit replay implementation (with body-capture follow-up)
	// or a chromedp-attach delegation. The CLI maps this to a user-facing
	// "follow this URL to book in your browser" message in v0.2.
	ErrBookingNotImplemented = errors.New("tock: book via CLI not yet implemented in v0.2 — use the venue URL to complete the reservation")

	// ErrPaymentRequired signals a venue requires payment-on-book that the CLI
	// cannot handle (full prepay). Card-required-but-not-prepaid venues
	// surface a different category (handled by the CLI command via CVC prompt).
	ErrPaymentRequired = errors.New("tock: venue requires prepayment (out of v0.2 scope)")

	// ErrPastCancellationWindow signals that cancel was called past the
	// venue's cancellation cutoff (typically 12 hours before the reservation).
	ErrPastCancellationWindow = errors.New("tock: past the cancellation window")

	// ErrCanaryUnrecognizedBody signals shape drift in a captured response.
	ErrCanaryUnrecognizedBody = errors.New("tock: response body shape unrecognized — discriminator may have drifted")

	// ErrUpcomingShapeChanged signals that $REDUX_STATE.patron.purchaseSummaries
	// is missing or wrong shape — Tock SPA-refactor canary.
	ErrUpcomingShapeChanged = errors.New("tock: $REDUX_STATE.patron.purchaseSummaries missing — Tock SPA may have changed")
)

// BookRequest is the user-facing input to Book(). v0.2 returns
// ErrBookingNotImplemented; this struct exists for API symmetry with
// opentable.BookRequest and is the call-site contract for the eventual
// implementation.
type BookRequest struct {
	VenueSlug       string  // e.g., "farzi-cafe-bellevue"
	ExperienceID    int     // numeric experience ID (e.g., 460115)
	ReservationDate string  // "2026-05-14"
	ReservationTime string  // "14:30" (24h)
	PartySize       int     // 1+
	Lat             float64 // metro center
	Lng             float64 // metro center
	// CVC is per-transaction even when card is on file. Empty for non-card-required venues.
	CVC string
}

// BookResponse mirrors what we'd parse from the receipt page after a
// successful book. Confirmation is the human-readable "TOCK-R-XXXXXXXX".
type BookResponse struct {
	ConfirmationNumber string // "TOCK-R-CTJO2LDS"
	PurchaseID         int    // integer, e.g., 362575651
	VenueSlug          string
	VenueName          string
	ReservationDate    string
	ReservationTime    string
	PartySize          int
	CardRequired       bool
	CardLastFour       string // last 4 of card on file, when card-required
	// CancelCutoffDate is parsed from the receipt page text ("up to 12 hours
	// before the time of the reservation"). Best-effort.
	CancelCutoffPolicy string
	// ReceiptURL is the canonical link the user/agent can open to view the
	// reservation in their browser.
	ReceiptURL string
}

// CancelRequest carries the purchaseId + slug needed to cancel a Tock
// reservation. Both values come from BookResponse OR a prior
// ListUpcomingReservations entry.
type CancelRequest struct {
	VenueSlug  string // e.g., "farzi-cafe-bellevue"
	PurchaseID int
}

// CancelResponse is the parsed result of a cancel attempt.
type CancelResponse struct {
	Canceled   bool
	PurchaseID int
	VenueSlug  string
	StatusText string // "Reservation canceled" from the receipt page banner
}

// UpcomingReservation mirrors a Tock $REDUX_STATE.patron.purchaseSummaries[]
// entry. Field set is the v0.2 minimum needed for idempotency pre-flight.
type UpcomingReservation struct {
	PurchaseID         int    `json:"purchaseId"`
	ConfirmationNumber string `json:"confirmationNumber"`
	VenueSlug          string `json:"businessDomainName"` // Tock's slug field
	VenueName          string `json:"businessName"`
	ReservationDate    string `json:"date"` // "2026-05-14"
	ReservationTime    string `json:"time"` // "14:30"
	PartySize          int    `json:"partySize"`
	Status             string `json:"status"` // "CONFIRMED", "CANCELED", etc.
}

// Book places a Tock reservation via chromedp-attach (real Chrome session).
// Tock uses traditional form-submit page navigation with CSRF/Braintree
// integration — too brittle to replay as a raw HTTP POST. ChromeBook drives
// a real Chrome through the click-flow: venue → slot → checkout → fill CVC
// → confirm → receipt page → extract confirmation.
//
// For card-required venues, req.CVC must be set (the CLI prompts the user
// interactively). For free venues, CVC is ignored.
//
// Requires Chrome running with --remote-debugging-port=9222 (the same
// "attach" mode used by `internal/source/opentable/chrome_avail.go`), or
// falls back to a stealth-spawned headless Chrome via
// TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL override.
//
// See docs/research/2026-05-09-booking-flow-discovery-tock.md for the
// architectural rationale (why chromedp instead of HTTP form-replay).
func (c *Client) Book(ctx context.Context, req BookRequest) (*BookResponse, error) {
	return c.ChromeBook(ctx, req)
}

// hiddenInputRE matches `<input type="hidden" name="X" value="Y">` shapes,
// tolerant of attribute ordering and quoting style. Used by the cancel-flow
// CSRF retry to scrape anti-forgery tokens from the receipt page HTML.
var hiddenInputRE = regexp.MustCompile(`(?is)<input[^>]*type=["']hidden["'][^>]*>`)

// hiddenAttrRE pulls the name + value attributes out of a single hidden-input
// tag matched by hiddenInputRE.
var hiddenAttrRE = regexp.MustCompile(`(?is)\b(name|value)=["']([^"']*)["']`)

// csrfNamePatterns names a hidden input is treated as anti-forgery if any of
// these substrings appears (case-insensitive). Drawn from common .NET / Rails
// / Express conventions; covers the field names Tock has historically used
// without us needing a fresh capture.
var csrfNamePatterns = []string{"csrf", "xsrf", "authenticity", "requestverification", "antiforgery"}

// Cancel cancels a Tock reservation by form-submitting to
// /<venue-slug>/receipt/cancel. Two-step: first attempts an empty-body POST.
// On 401/403 (likely CSRF rejection), GETs the receipt page, scrapes hidden
// inputs whose names look like anti-forgery tokens, and retries the POST
// with those fields populated. The actual cancellation is verified by
// checking the post-redirect page state for the confirmation banner.
//
// Status caveat: the CSRF retry has not been live-tested end-to-end (the
// v0.2 testing budget cancelled via the Tock UI). On the next live cancel
// session, capture the receipt page's hidden-input field names and tighten
// csrfNamePatterns if the empty-body POST proves to fail consistently.
func (c *Client) Cancel(ctx context.Context, req CancelRequest) (*CancelResponse, error) {
	if req.VenueSlug == "" || req.PurchaseID == 0 {
		return nil, fmt.Errorf("tock cancel: VenueSlug and PurchaseID are required")
	}
	cancelURL := Origin + "/" + url.PathEscape(req.VenueSlug) + "/receipt/cancel?purchaseId=" + fmt.Sprintf("%d", req.PurchaseID)
	receiptURL := Origin + "/" + url.PathEscape(req.VenueSlug) + "/receipt?purchaseId=" + fmt.Sprintf("%d", req.PurchaseID)

	// First attempt: empty-body POST. Some Tock venues don't enforce CSRF on
	// the cancel form, in which case this succeeds without an extra GET.
	resp, body, err := c.postCancelForm(ctx, cancelURL, receiptURL, url.Values{})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		// Likely CSRF rejection. GET the receipt page, scrape any hidden
		// anti-forgery inputs, and retry once with those fields populated.
		tokens, terr := c.fetchCancelCSRFTokens(ctx, receiptURL)
		if terr != nil {
			return nil, fmt.Errorf("tock cancel: HTTP %d on initial POST and CSRF lookup failed: %w", resp.StatusCode, terr)
		}
		if len(tokens) == 0 {
			return nil, fmt.Errorf("tock cancel: HTTP %d (no anti-forgery tokens found on receipt page; auth may be expired — run `auth login --chrome`)", resp.StatusCode)
		}
		resp, body, err = c.postCancelForm(ctx, cancelURL, receiptURL, tokens)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("tock cancel: HTTP %d after CSRF retry; auth may be expired or token field name has drifted", resp.StatusCode)
		}
	}
	if resp.StatusCode >= 400 && resp.StatusCode != 410 {
		return nil, fmt.Errorf("tock cancel returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode == 410 {
		return nil, fmt.Errorf("%w: HTTP 410", ErrPastCancellationWindow)
	}
	bodyStr := string(body)
	canceled := strings.Contains(bodyStr, "Reservation canceled") || strings.Contains(bodyStr, "Reservation cancelled")
	if !canceled {
		// Some failure modes return a 200 with an error message rendered in
		// the page. Detect a few obvious patterns; surface canary error
		// otherwise so drift surfaces loudly.
		if strings.Contains(bodyStr, "cutoff") || strings.Contains(bodyStr, "12 hours") {
			return nil, fmt.Errorf("%w: receipt page indicates past-cutoff", ErrPastCancellationWindow)
		}
		return nil, fmt.Errorf("%w: cancel response did not contain expected confirmation banner", ErrCanaryUnrecognizedBody)
	}
	return &CancelResponse{
		Canceled:   true,
		PurchaseID: req.PurchaseID,
		VenueSlug:  req.VenueSlug,
		StatusText: "Reservation canceled",
	}, nil
}

// postCancelForm POSTs form-encoded values to the cancel URL and reads the
// full body. Returns the response (caller must NOT close — body is already
// drained), the body bytes, and any transport error.
func (c *Client) postCancelForm(ctx context.Context, cancelURL, referer string, fields url.Values) (*http.Response, []byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cancelURL, strings.NewReader(fields.Encode()))
	if err != nil {
		return nil, nil, fmt.Errorf("building tock cancel request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml")
	httpReq.Header.Set("Origin", Origin)
	httpReq.Header.Set("Referer", referer)
	resp, err := c.do429Aware(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("calling tock cancel: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading tock cancel response: %w", err)
	}
	return resp, body, nil
}

// fetchCancelCSRFTokens GETs the receipt page and scrapes hidden inputs that
// look like anti-forgery tokens. Returns a url.Values populated with all such
// fields (typically zero or one). Caller decides what to do when empty.
func (c *Client) fetchCancelCSRFTokens(ctx context.Context, receiptURL string) (url.Values, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, receiptURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building tock receipt-page request: %w", err)
	}
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.do429Aware(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling tock receipt page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tock receipt page returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tock receipt page: %w", err)
	}
	return extractCSRFTokens(string(body)), nil
}

// extractCSRFTokens parses the HTML for hidden inputs whose name matches one
// of csrfNamePatterns and returns them as url.Values. Exposed for tests.
func extractCSRFTokens(html string) url.Values {
	out := url.Values{}
	for _, tag := range hiddenInputRE.FindAllString(html, -1) {
		var name, value string
		for _, m := range hiddenAttrRE.FindAllStringSubmatch(tag, -1) {
			switch strings.ToLower(m[1]) {
			case "name":
				name = m[2]
			case "value":
				value = m[2]
			}
		}
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		matched := false
		for _, pat := range csrfNamePatterns {
			if strings.Contains(lower, pat) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		out.Set(name, value)
	}
	return out
}

// ListUpcomingReservations fetches the user's upcoming Tock reservations
// from /profile/upcoming SSR. Returns a slice mapped from
// $REDUX_STATE.patron.purchaseSummaries[].
//
// Caveat: during U1 discovery, /profile/upcoming returned an empty
// patron.purchaseSummaries array with `numRequestsInProgress: 0` —
// suggesting either (a) the kooky-imported cookies don't carry the auth
// state for this surface, OR (b) the page hydrates the slice via a
// follow-up XHR rather than at SSR time. v0.2 returns the parsed slice
// (possibly empty); a future follow-up can add an XHR-based path if the
// SSR proves insufficient in U6 dogfood.
func (c *Client) ListUpcomingReservations(ctx context.Context) ([]UpcomingReservation, error) {
	state, err := c.FetchReduxState(ctx, "/profile/upcoming")
	if err != nil {
		return nil, fmt.Errorf("tock list-upcoming: %w", err)
	}
	patron, ok := state["patron"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: state.patron missing", ErrUpcomingShapeChanged)
	}
	rawList, hasList := patron["purchaseSummaries"]
	if !hasList || rawList == nil {
		return nil, fmt.Errorf("%w: state.patron.purchaseSummaries missing", ErrUpcomingShapeChanged)
	}
	listJSON, err := json.Marshal(rawList)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling purchaseSummaries: %w", err)
	}
	var entries []UpcomingReservation
	if err := json.Unmarshal(listJSON, &entries); err != nil {
		return nil, fmt.Errorf("%w: decoding purchaseSummaries: %v", ErrCanaryUnrecognizedBody, err)
	}
	// Filter to upcoming only (status not CANCELED / COMPLETED).
	out := make([]UpcomingReservation, 0, len(entries))
	for _, e := range entries {
		s := strings.ToUpper(e.Status)
		if s == "CANCELED" || s == "CANCELLED" || s == "COMPLETED" {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// buildVenueDeepLinkURL constructs the Tock URL the user/agent can open in
// their browser to complete a booking manually. The URL pre-populates date,
// time, party, and (when known) experience.
func buildVenueDeepLinkURL(slug string, experienceID int, date, time string, party int) string {
	if experienceID > 0 {
		return fmt.Sprintf("%s/%s/experience/%d?date=%s&size=%d&time=%s",
			Origin, url.PathEscape(slug), experienceID, url.QueryEscape(date), party, url.QueryEscape(time))
	}
	return fmt.Sprintf("%s/%s?date=%s&size=%d&time=%s",
		Origin, url.PathEscape(slug), url.QueryEscape(date), party, url.QueryEscape(time))
}
