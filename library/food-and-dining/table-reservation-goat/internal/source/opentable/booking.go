// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

// PATCH: cross-network-source-clients (booking) — see .printing-press-patches.json.
//
// OpenTable booking flow (captured live 2026-05-09 via chrome-MCP):
//   - Book   = POST /dapi/booking/make-reservation (REST, application/json)
//   - Cancel = POST /dapi/fe/gql (GraphQL persisted-query "CancelReservation",
//              hash 4ee53a006030f602bdeb1d751fa90ddc4240d9e17d015fb7976f8efcb80a026e)
//   - ListUpcomingReservations = GET /user/dining-dashboard,
//              parse window.__INITIAL_STATE__.diningDashboard.upcomingReservations[]
//   - MyProfile = same SSR fetch, parse __INITIAL_STATE__.userProfile
//
// All write paths require the X-CSRF-Token header (obtained via Bootstrap()).
// `make-reservation` carries `slotAvailabilityToken` and `slotHash` from the
// `RestaurantsAvailability` GraphQL response. `slotLockId` is server-allocated
// at /booking/details page render — for the CLI we GET that page and parse it
// from the SSR-emitted state.
//
// Cancellation deadline is NOT in the book response. To populate it, fetch
// `BookingConfirmationPageInFlow` GraphQL after book and read
// `data.reservation.cancelCutoffDate`.

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
	"time"
)

// Sentinel error categories for typed-error handling at the CLI boundary.
// The CLI maps these to sanitized JSON output; raw error chains stay on stderr.
var (
	ErrSlotTaken              = errors.New("opentable: slot no longer available")
	ErrPaymentRequired        = errors.New("opentable: venue requires payment (out of v0.2 scope)")
	ErrAuthExpired            = errors.New("opentable: auth expired; run `auth login --chrome` to refresh")
	ErrPastCancellationWindow = errors.New("opentable: past the cancellation window")
	ErrCanaryUnrecognizedBody = errors.New("opentable: response body shape unrecognized — discriminator may have drifted")
)

// CancelReservation persisted-query hash, captured live 2026-05-09.
const CancelReservationHash = "4ee53a006030f602bdeb1d751fa90ddc4240d9e17d015fb7976f8efcb80a026e"

// BookingConfirmationPageInFlow persisted-query hash. Used to fetch the
// cancelCutoffDate after a successful book. The captured prefix was truncated
// at capture time; the implementer should refresh from a live capture if 400.
const BookingConfirmationHash = "7c4fe1d7786e25085199bddc46cb1525ebf90ac18621ceb3021074fda52b6000"

// BookRequest is the user-facing input to Book(). All fields are required
// except Points (defaults to 0). The CLI command populates these from
// resolved availability + diner profile.
type BookRequest struct {
	RestaurantID          int    // numeric OT restaurant ID
	ReservationDateTime   string // "2026-05-13T19:00" (ISO local)
	PartySize             int    // 1+
	SlotHash              string // from RestaurantsAvailability slot
	SlotAvailabilityToken string // from RestaurantsAvailability slot
	Points                int    // 0 if no loyalty
	PointsType            string // "Standard" if no loyalty
	DiningAreaID          int    // 1 = Indoor (default)
	// Diner identity — caller fetches via MyProfile() then passes through.
	FirstName            string
	LastName             string
	Email                string
	PhoneNumber          string // 10-digit string
	PhoneNumberCountryID string // "US"
}

// BookResponse is the parsed REST response from /dapi/booking/make-reservation.
// Fields match the OT API verbatim except CancelCutoffDate, which the caller
// populates via a follow-up FetchCancelCutoff() call.
type BookResponse struct {
	ReservationID       int    `json:"reservationId"`
	RestaurantID        int    `json:"restaurantId"`
	ReservationDateTime string `json:"reservationDateTime"`
	PartySize           int    `json:"partySize"`
	ConfirmationNumber  int    `json:"confirmationNumber"`
	Points              int    `json:"points"`
	ReservationStateID  int    `json:"reservationStateId"`
	SecurityToken       string `json:"securityToken"`
	GPID                int64  `json:"gpid"`
	IsRestRef           bool   `json:"isRestRef"`
	ReservationHash     string `json:"reservationHash"`
	ReservationType     string `json:"reservationType"`
	ReservationSource   string `json:"reservationSource"`
	CreditCardLastFour  string `json:"creditCardLastFour,omitempty"`
	UserType            int    `json:"userType"`
	DiningAreaID        int    `json:"diningAreaId"`
	Environment         string `json:"environment"`
	PartnerScaRequired  bool   `json:"partnerScaRequired"`
	Success             bool   `json:"success"`
	// CancelCutoffDate is NOT returned by make-reservation; populated separately.
	CancelCutoffDate string `json:"-"`
}

// CancelRequest carries the triple needed to cancel an OT reservation.
// All three fields come from the BookResponse OR from a prior
// ListUpcomingReservations entry.
type CancelRequest struct {
	RestaurantID       int
	ConfirmationNumber int
	SecurityToken      string
}

// CancelResponse is the parsed GraphQL `cancelReservation.data` payload.
type CancelResponse struct {
	StatusCode         int    `json:"statusCode"`
	RestaurantID       int    `json:"restaurantId"`
	ReservationID      int    `json:"reservationId"`
	ReservationStateID int    `json:"reservationStateId"`
	ReservationState   string `json:"reservationState"`
	ConfirmationNumber int    `json:"confirmationNumber"`
	RefundStatus       string `json:"refundStatus,omitempty"`
}

// UpcomingReservation mirrors __INITIAL_STATE__.diningDashboard.upcomingReservations[]
// entries from /user/dining-dashboard. The CLI uses these for idempotency
// pre-flight (R5) and for surfacing existing reservations to the agent.
type UpcomingReservation struct {
	ConfirmationNumber int    `json:"confirmationNumber"`
	ConfirmationID     int    `json:"confirmationId"`
	SecurityToken      string `json:"securityToken"`
	RestaurantID       int    `json:"restaurantId"`
	RestaurantName     string `json:"restaurantName"`
	DateTime           string `json:"dateTime"` // ISO local "2026-05-10T11:15:00"
	PartySize          int    `json:"partySize"`
	ReservationState   string `json:"reservationState"`
	ReservationType    string `json:"reservationType"`
	IsUpcoming         bool   `json:"isUpcoming"`
	IsForPrimaryDiner  bool   `json:"isForPrimaryDiner"`
	IsPrivateDining    bool   `json:"isPrivateDining"`
	Points             int    `json:"points"`
	DinerFirstName     string `json:"dinerFirstName"`
	DinerLastName      string `json:"dinerLastName"`
}

// MyProfile carries the diner identity needed for Book request bodies.
// Parsed from __INITIAL_STATE__.userProfile on /user/dining-dashboard.
type MyProfile struct {
	FirstName            string
	LastName             string
	Email                string
	PhoneNumber          string
	PhoneNumberCountryID string
}

// makeReservationPath is the REST endpoint for booking.
const makeReservationPath = "/dapi/booking/make-reservation"

// bookingDetailsPath is the SSR page that allocates the slot lock.
const bookingDetailsPath = "/booking/details"

// diningDashboardPath is the SSR page that hydrates upcoming-reservations.
const diningDashboardPath = "/user/dining-dashboard"

// slotLockIDRE matches the `slotLockId=<int>` URL param emitted into
// /booking/details after the server allocates a lock.
var slotLockIDRE = regexp.MustCompile(`slotLockId=(\d+)`)

// Book books a free OpenTable reservation. Two-step internally: first a
// GET to /booking/details to allocate slotLockId, then a POST to
// /dapi/booking/make-reservation. Returns *BookResponse on success.
//
// On typed errors (slot taken, payment required, auth expired, bot
// detection), returns the corresponding sentinel error wrapped with context
// for the CLI to map to sanitized JSON output.
func (c *Client) Book(ctx context.Context, req BookRequest) (*BookResponse, error) {
	if err := c.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("bootstrap before book: %w", err)
	}

	// Step 1: GET /booking/details to allocate slotLockId.
	correlationID := newUUID()
	q := url.Values{}
	q.Set("availabilityToken", req.SlotAvailabilityToken)
	q.Set("correlationId", correlationID)
	q.Set("creditCardRequired", "false")
	q.Set("dateTime", req.ReservationDateTime)
	q.Set("partySize", fmt.Sprintf("%d", req.PartySize))
	q.Set("points", fmt.Sprintf("%d", req.Points))
	q.Set("pointsType", req.PointsType)
	q.Set("resoAttribute", "default")
	q.Set("rid", fmt.Sprintf("%d", req.RestaurantID))
	q.Set("slotHash", req.SlotHash)
	q.Set("isModify", "false")
	q.Set("isMandatory", "false")
	q.Set("cfe", "true")
	q.Set("st", "Standard")

	detailsURL := Origin + bookingDetailsPath + "?" + q.Encode()
	dReq, err := http.NewRequestWithContext(ctx, http.MethodGet, detailsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building /booking/details request: %w", err)
	}
	dReq.Header.Set("Accept", "text/html,application/xhtml+xml")
	dReq.Header.Set("Referer", Origin+"/r/"+fmt.Sprintf("%d", req.RestaurantID))
	dResp, err := c.do429Aware(dReq)
	if err != nil {
		return nil, fmt.Errorf("fetching /booking/details: %w", err)
	}
	defer dResp.Body.Close()
	if dResp.StatusCode >= 400 {
		body, _ := io.ReadAll(dResp.Body)
		return nil, fmt.Errorf("/booking/details returned HTTP %d: %s", dResp.StatusCode, truncate(string(body), 200))
	}
	dBody, err := io.ReadAll(dResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading /booking/details: %w", err)
	}
	// Final URL after redirects carries slotLockId in the URL params; OT's
	// SSR usually rewrites the URL inline. Try both: scrape from response URL
	// AND from the body itself.
	finalURL := ""
	if dResp.Request != nil && dResp.Request.URL != nil {
		finalURL = dResp.Request.URL.String()
	}
	slotLockID := extractSlotLockID(finalURL, dBody)
	if slotLockID == 0 {
		return nil, fmt.Errorf("opentable: slotLockId not found in /booking/details response — slot may already be taken or the SSR shape changed")
	}

	// Step 2: POST /dapi/booking/make-reservation with the full body.
	bodyMap := map[string]any{
		"additionalServiceFees":  []any{},
		"attributionToken":       "",
		"correlationId":          correlationID,
		"country":                "US",
		"diningAreaId":           req.DiningAreaID,
		"email":                  req.Email,
		"fbp":                    "",
		"firstName":              req.FirstName,
		"isModify":               false,
		"katakanaFirstName":      "",
		"katakanaLastName":       "",
		"lastName":               req.LastName,
		"nonBookableExperiences": []any{},
		"optInEmailRestaurant":   false,
		"partySize":              req.PartySize,
		"phoneNumber":            req.PhoneNumber,
		"phoneNumberCountryId":   req.PhoneNumberCountryID,
		"points":                 req.Points,
		"pointsType":             req.PointsType,
		"reservationAttribute":   "default",
		"reservationDateTime":    req.ReservationDateTime,
		"reservationType":        "Standard",
		"restaurantId":           req.RestaurantID,
		"slotAvailabilityToken":  req.SlotAvailabilityToken,
		"slotHash":               req.SlotHash,
		"slotLockId":             slotLockID,
		"tipAmount":              0,
		"tipPercent":             0,
	}
	if req.DiningAreaID == 0 {
		bodyMap["diningAreaId"] = 1 // Indoor default
	}
	if req.PointsType == "" {
		bodyMap["pointsType"] = "Standard"
	}
	bodyJSON, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("marshaling book body: %w", err)
	}

	bookURL := Origin + makeReservationPath
	bReq, err := http.NewRequestWithContext(ctx, http.MethodPost, bookURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, fmt.Errorf("building book request: %w", err)
	}
	bReq.Header.Set("Content-Type", "application/json")
	bReq.Header.Set("Accept", "application/json")
	bReq.Header.Set("Accept-Language", "en-US, en, *")
	bReq.Header.Set("X-CSRF-Token", c.CSRF())
	bReq.Header.Set("Origin", Origin)
	bReq.Header.Set("Referer", finalURL)

	bResp, err := c.do429Aware(bReq)
	if err != nil {
		return nil, fmt.Errorf("calling make-reservation: %w", err)
	}
	defer bResp.Body.Close()
	respBody, err := io.ReadAll(bResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading make-reservation response: %w", err)
	}

	if bResp.StatusCode >= 500 {
		return nil, fmt.Errorf("opentable book returned HTTP %d (server error): %s", bResp.StatusCode, truncate(string(respBody), 200))
	}
	if bResp.StatusCode == 401 || bResp.StatusCode == 403 {
		return nil, fmt.Errorf("%w: HTTP %d", ErrAuthExpired, bResp.StatusCode)
	}
	if bResp.StatusCode == 409 {
		// 409 Conflict typically signals slot was taken between detail-fetch and book.
		return nil, fmt.Errorf("%w: HTTP 409", ErrSlotTaken)
	}
	if bResp.StatusCode == 402 {
		return nil, fmt.Errorf("%w: HTTP 402", ErrPaymentRequired)
	}

	var resp BookResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("%w: cannot decode HTTP %d body: %v", ErrCanaryUnrecognizedBody, bResp.StatusCode, err)
	}
	if !resp.Success {
		// Check for known error patterns in the response body that indicate
		// slot-taken vs payment-required vs other.
		bodyStr := strings.ToLower(string(respBody))
		switch {
		case strings.Contains(bodyStr, "slot") && (strings.Contains(bodyStr, "taken") || strings.Contains(bodyStr, "unavailable")):
			return nil, fmt.Errorf("%w: response body indicates slot unavailable", ErrSlotTaken)
		case strings.Contains(bodyStr, "credit card") || strings.Contains(bodyStr, "payment"):
			return nil, fmt.Errorf("%w: response body indicates payment required", ErrPaymentRequired)
		default:
			return nil, fmt.Errorf("%w: success=false response: %s", ErrCanaryUnrecognizedBody, truncate(string(respBody), 200))
		}
	}
	return &resp, nil
}

// FetchCancelCutoff retrieves the cancellation deadline for a just-booked
// reservation via the BookingConfirmationPageInFlow GraphQL operation.
// Returns ISO-formatted UTC datetime (e.g., "2026-05-10T05:55Z") or empty
// string if the field is absent. Non-fatal if it fails — callers fall back
// to "see venue policy" in the JSON output.
func (c *Client) FetchCancelCutoff(ctx context.Context, restaurantID, confirmationNumber int, securityToken string) (string, error) {
	if err := c.Bootstrap(ctx); err != nil {
		return "", fmt.Errorf("bootstrap before fetch-cutoff: %w", err)
	}
	body := map[string]any{
		"operationName": "BookingConfirmationPageInFlow",
		"variables": map[string]any{
			"gpid":                                 0,
			"diningHistoryLimit":                   4,
			"popularDishesCount":                   3,
			"popularDishesReviewCount":             5,
			"showPopularDishes":                    true,
			"usefallBackCancellationPolicyMessage": false,
			"enableTicketedExperiences":            false,
			"useCBR":                               false,
			"enablePrivateDiningExperiences":       false,
			"rid":                                  restaurantID,
			"tld":                                  "com",
			"confirmationNumber":                   confirmationNumber,
			"databaseRegion":                       "NA",
			"securityToken":                        securityToken,
			"countryId":                            "US",
			"isLoggedIn":                           true,
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": BookingConfirmationHash,
			},
		},
	}
	data, err := c.gqlCall(ctx, "BookingConfirmationPageInFlow", body)
	if err != nil {
		return "", err
	}
	var env struct {
		Data struct {
			Reservation struct {
				CancelCutoffDate string `json:"cancelCutoffDate"`
			} `json:"reservation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("decoding cancel-cutoff response: %w", err)
	}
	return env.Data.Reservation.CancelCutoffDate, nil
}

// Cancel cancels an OpenTable reservation via the CancelReservation
// GraphQL mutation. Requires the {RestaurantID, ConfirmationNumber,
// SecurityToken} triple — all returned by Book() or surfaced by
// ListUpcomingReservations().
func (c *Client) Cancel(ctx context.Context, req CancelRequest) (*CancelResponse, error) {
	if err := c.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("bootstrap before cancel: %w", err)
	}
	body := map[string]any{
		"operationName": "CancelReservation",
		"variables": map[string]any{
			"input": map[string]any{
				"restaurantId":       req.RestaurantID,
				"confirmationNumber": req.ConfirmationNumber,
				"securityToken":      req.SecurityToken,
				"databaseRegion":     "NA",
				"reservationSource":  "Online",
			},
		},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{
				"version":    1,
				"sha256Hash": CancelReservationHash,
			},
		},
	}
	data, err := c.gqlCall(ctx, "CancelReservation", body)
	if err != nil {
		return nil, fmt.Errorf("calling CancelReservation: %w", err)
	}
	var env struct {
		Data struct {
			CancelReservation struct {
				StatusCode int             `json:"statusCode"`
				Errors     []any           `json:"errors"`
				Data       *CancelResponse `json:"data"`
			} `json:"cancelReservation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("%w: cannot decode CancelReservation response: %v", ErrCanaryUnrecognizedBody, err)
	}
	if env.Data.CancelReservation.StatusCode != 200 {
		// 410 / past-window error patterns vary; surface generically with hint.
		errStr := fmt.Sprintf("%v", env.Data.CancelReservation.Errors)
		lowered := strings.ToLower(errStr)
		if strings.Contains(lowered, "cutoff") || strings.Contains(lowered, "deadline") || strings.Contains(lowered, "window") {
			return nil, fmt.Errorf("%w: %s", ErrPastCancellationWindow, errStr)
		}
		return nil, fmt.Errorf("opentable cancel returned statusCode %d: %s", env.Data.CancelReservation.StatusCode, errStr)
	}
	if env.Data.CancelReservation.Data == nil {
		return nil, fmt.Errorf("%w: CancelReservation success but data field empty", ErrCanaryUnrecognizedBody)
	}
	return env.Data.CancelReservation.Data, nil
}

// ListUpcomingReservations fetches the user's upcoming reservations from
// /user/dining-dashboard SSR. Returns a slice mapped from
// __INITIAL_STATE__.diningDashboard.upcomingReservations[].
func (c *Client) ListUpcomingReservations(ctx context.Context) ([]UpcomingReservation, error) {
	state, err := c.fetchDiningDashboardState(ctx)
	if err != nil {
		return nil, err
	}
	dd, ok := state["diningDashboard"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: diningDashboard slice missing from __INITIAL_STATE__", ErrCanaryUnrecognizedBody)
	}
	rawList, ok := dd["upcomingReservations"]
	if !ok || rawList == nil {
		return []UpcomingReservation{}, nil
	}
	listJSON, err := json.Marshal(rawList)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling upcomingReservations: %w", err)
	}
	var entries []UpcomingReservation
	if err := json.Unmarshal(listJSON, &entries); err != nil {
		return nil, fmt.Errorf("%w: decoding upcomingReservations: %v", ErrCanaryUnrecognizedBody, err)
	}
	return entries, nil
}

// MyProfile fetches the diner profile from the same dining-dashboard SSR.
// Returns the fields needed to populate Book request bodies.
func (c *Client) MyProfile(ctx context.Context) (*MyProfile, error) {
	state, err := c.fetchDiningDashboardState(ctx)
	if err != nil {
		return nil, err
	}
	up, ok := state["userProfile"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: userProfile slice missing from __INITIAL_STATE__", ErrCanaryUnrecognizedBody)
	}
	prof := &MyProfile{}
	if v, ok := up["firstName"].(string); ok {
		prof.FirstName = v
	}
	if v, ok := up["lastName"].(string); ok {
		prof.LastName = v
	}
	if v, ok := up["email"].(string); ok {
		prof.Email = v
	}
	// Phone may be nested under phone.number or as plain string
	if phone, ok := up["phone"].(map[string]any); ok {
		if n, ok := phone["number"].(string); ok {
			prof.PhoneNumber = n
		}
		if cc, ok := phone["countryCode"].(string); ok {
			prof.PhoneNumberCountryID = cc
		}
	}
	if prof.PhoneNumberCountryID == "" {
		prof.PhoneNumberCountryID = "US"
	}
	return prof, nil
}

// fetchDiningDashboardState GETs /user/dining-dashboard via the shared
// FetchInitialState helper (which handles the SSR + jseval extraction with
// OT's existing anchor pattern). Shared between ListUpcomingReservations
// and MyProfile.
func (c *Client) fetchDiningDashboardState(ctx context.Context) (map[string]any, error) {
	state, err := c.FetchInitialState(ctx, diningDashboardPath)
	if err != nil {
		// Map auth-related errors to the typed sentinel.
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			return nil, fmt.Errorf("%w: %v", ErrAuthExpired, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrCanaryUnrecognizedBody, err)
	}
	return state, nil
}

// extractSlotLockID pulls slotLockId from either the final URL query string
// or the response body (some OT renders embed it in inline state).
func extractSlotLockID(finalURL string, body []byte) int64 {
	if finalURL != "" {
		if m := slotLockIDRE.FindStringSubmatch(finalURL); m != nil {
			var id int64
			fmt.Sscanf(m[1], "%d", &id)
			if id > 0 {
				return id
			}
		}
	}
	if m := slotLockIDRE.FindSubmatch(body); m != nil {
		var id int64
		fmt.Sscanf(string(m[1]), "%d", &id)
		if id > 0 {
			return id
		}
	}
	return 0
}

// _ keeps time imported when the file evolves to add deadline-parsing helpers.
var _ = time.Now
