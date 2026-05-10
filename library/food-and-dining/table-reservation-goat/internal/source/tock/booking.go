// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

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

// Cancel cancels a Tock reservation by form-submitting to
// /<venue-slug>/receipt/cancel. The request body shape is best-effort
// (CSRF token if needed); the actual cancellation is verified by checking
// the post-redirect page state.
//
// Status caveat: this implementation hasn't been live-tested against an
// active reservation due to the v0.2 constraint of "no fresh test bookings."
// First U6 dogfood pass should validate against a manually-created Tock
// reservation, then be removed if the form-submit shape proves brittle in
// favor of chromedp-attach (Option B).
func (c *Client) Cancel(ctx context.Context, req CancelRequest) (*CancelResponse, error) {
	if req.VenueSlug == "" || req.PurchaseID == 0 {
		return nil, fmt.Errorf("tock cancel: VenueSlug and PurchaseID are required")
	}
	cancelURL := Origin + "/" + url.PathEscape(req.VenueSlug) + "/receipt/cancel?purchaseId=" + fmt.Sprintf("%d", req.PurchaseID)

	// Form body: empty for now. The page may include an anti-forgery token
	// rendered into the cancel page HTML; if so, GET that page first and
	// extract the token. v0.2 attempts the simple POST and falls back to
	// the typed error on non-200.
	formBody := url.Values{}.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cancelURL, strings.NewReader(formBody))
	if err != nil {
		return nil, fmt.Errorf("building tock cancel request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml")
	httpReq.Header.Set("Origin", Origin)
	httpReq.Header.Set("Referer", Origin+"/"+url.PathEscape(req.VenueSlug)+"/receipt?purchaseId="+fmt.Sprintf("%d", req.PurchaseID))

	resp, err := c.do429Aware(httpReq)
	if err != nil {
		return nil, fmt.Errorf("calling tock cancel: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("tock cancel: HTTP %d (auth required); ensure session cookies are fresh", resp.StatusCode)
	}
	if resp.StatusCode >= 400 && resp.StatusCode != 410 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tock cancel returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode == 410 {
		return nil, fmt.Errorf("%w: HTTP 410", ErrPastCancellationWindow)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading tock cancel response: %w", err)
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
