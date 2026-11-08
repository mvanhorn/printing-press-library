package consumer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/exitcode"
)

// AmbiguousError means a state-changing POST may or may not have taken effect:
// the connection dropped, or the server failed after receiving the request. The
// caller must read state and report; it must never retry, because a second
// checkout could charge the owner twice.
type AmbiguousError struct {
	Operation string
	Detail    string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%s outcome unknown: %s; do not retry, check reservations first", e.Operation, e.Detail)
}

// postOnce sends one POST with no retry. This client has no retry layer, and that
// is deliberate for the two calls that spend or return money.
func (c *Client) postOnce(op string, opts *requestOptions, body any) (json.RawMessage, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	opts.body = bytes.NewReader(b)
	opts.mutating = true
	req, err := opts.build()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return nil, &AmbiguousError{Operation: op, Detail: "request failed before a response arrived"}
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return nil, &AmbiguousError{Operation: op, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	if readErr != nil {
		return nil, &AmbiguousError{Operation: op, Detail: "response body could not be read"}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, exitcode.Transientf("rate limited (HTTP 429); %s was not accepted", op)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode >= 400 {
			return nil, exitcode.APIf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}
		// 2xx with an unreadable body: the server took the request, so what it did
		// is unknown.
		return nil, &AmbiguousError{Operation: op, Detail: "2xx response could not be decoded"}
	}
	if err := checkEnvelope(resp.StatusCode, env); err != nil {
		return nil, err
	}
	data, err := json.Marshal(env.Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Checkout posts one booking. It never retries; see AmbiguousError.
func (c *Client) Checkout(req CheckoutRequest) (map[string]any, error) {
	if len(req.Items) == 0 {
		return nil, exitcode.Usagef("checkout requires at least one item")
	}
	if err := validateCheckoutItem(req.Items[0]); err != nil {
		return nil, exitcode.Usagef("invalid checkout item: %v", err)
	}
	if c.DryRun {
		return nil, exitcode.Usagef("dry-run: would POST %s", PathCheckout)
	}
	data, err := c.postOnce("booking", c.newRequest(http.MethodPost, PathCheckout, nil).withCheckoutHeaders(), req)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &AmbiguousError{Operation: "booking", Detail: "response shape not recognized"}
	}
	return out, nil
}

// CancellationReason is one entry the cancel dialog offers.
type CancellationReason struct {
	ID      int    `json:"id"`
	Reason  string `json:"reason"`
	Display string `json:"reason_displayed_to_customer"`
}

// RefundEligibility is GET /reservations/{id}/refund-eligibility/, the read the
// site's cancel dialog makes before offering a refund.
type RefundEligibility struct {
	OriginalSource bool
	CreditOnly     bool
	Reasons        []CancellationReason
}

// Eligible reports whether any refund route is open.
func (e RefundEligibility) Eligible() bool { return e.OriginalSource || e.CreditOnly }

// ReasonNames lists the wire values accepted as a cancellation reason.
func (e RefundEligibility) ReasonNames() []string {
	out := make([]string, 0, len(e.Reasons))
	for _, r := range e.Reasons {
		out = append(out, r.Reason)
	}
	return out
}

// GetRefundEligibility reads refund status for one reservation.
func (c *Client) GetRefundEligibility(id string) (RefundEligibility, error) {
	path := fmt.Sprintf(PathReservationRefundEligibility, url.PathEscape(id))
	opts := c.newRequest(http.MethodGet, path, nil)
	opts.spotVersion = ConsumerAccountVersion
	req, err := opts.build()
	if err != nil {
		return RefundEligibility{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return RefundEligibility{}, exitcode.Transientf("request failed: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return RefundEligibility{}, err
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return RefundEligibility{}, exitcode.APIf("HTTP %d decoding refund eligibility", resp.StatusCode)
	}
	var generic Envelope
	_ = json.Unmarshal(raw, &generic)
	if err := checkEnvelope(resp.StatusCode, generic); err != nil {
		return RefundEligibility{}, err
	}
	var wire struct {
		Original struct {
			IsEligible bool `json:"is_eligible"`
		} `json:"original_source_refund"`
		Credit struct {
			IsEligible bool `json:"is_eligible"`
		} `json:"credit_only_refund"`
		Reasons []CancellationReason `json:"cancellation_reasons"`
	}
	if err := json.Unmarshal(env.Data, &wire); err != nil {
		return RefundEligibility{}, exitcode.APIf("decode refund eligibility: %v", err)
	}
	return RefundEligibility{OriginalSource: wire.Original.IsEligible, CreditOnly: wire.Credit.IsEligible, Reasons: wire.Reasons}, nil
}

// RefundRequest is POST /reservations/{id}/refund/. The site always sends reason
// and refund_as_credit; access_key only exists for some reservations.
type RefundRequest struct {
	Reason         string `json:"reason"`
	AccessKey      string `json:"access_key,omitempty"`
	RefundAsCredit bool   `json:"refund_as_credit"`
}

// Refund cancels one reservation. It never retries; see AmbiguousError.
func (c *Client) Refund(id string, in RefundRequest) (map[string]any, error) {
	if c.DryRun {
		return nil, exitcode.Usagef("dry-run: would POST %s", fmt.Sprintf(PathReservationRefund, id))
	}
	path := fmt.Sprintf(PathReservationRefund, url.PathEscape(id))
	data, err := c.postOnce("cancellation", c.newRequest(http.MethodPost, path, nil), in)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out, nil
}

// MatchingReservations finds reservations for this facility and start time, used
// to report what actually happened after an ambiguous booking.
func (c *Client) MatchingReservations(facilityID int, startsInstant string) ([]Reservation, error) {
	list, err := c.listReservations("25")
	if err != nil {
		return nil, err
	}
	want, wantErr := time.Parse(time.RFC3339, startsInstant)
	var out []Reservation
	for _, r := range list {
		if r.FacilityID != facilityID {
			continue
		}
		if got, err := time.Parse(time.RFC3339, r.Starts); err == nil && wantErr == nil {
			if got.Equal(want) {
				out = append(out, r)
			}
			continue
		}
		// Some payloads give local wall time with no offset; compare to the minute.
		if len(r.Starts) >= 16 && strings.HasPrefix(startsInstant, r.Starts[:16]) {
			out = append(out, r)
		}
	}
	return out, nil
}

// reservationPageSize stays small because this endpoint is slow: live, 25 rows took
// about 6s and 50 took about 15s, and 100 outlasted a 30s client timeout.
const reservationPageSize = 25

// ListAllReservations walks pages until a short page, so upcoming reservations
// are not missed by accounts with long histories.
func (c *Client) ListAllReservations(maxPages int) ([]Reservation, error) {
	var out []Reservation
	for page := 1; page <= maxPages; page++ {
		raw, err := c.doJSONData(http.MethodGet, fmt.Sprintf("%s?page_size=%d&page=%d", PathReservations, reservationPageSize, page), nil)
		if err != nil {
			return nil, err
		}
		items, err := parseReservationsPayload(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if len(items) < reservationPageSize {
			break
		}
	}
	return out, nil
}

// AirportPoint resolves an IATA code from the public airports list.
func (c *Client) AirportPoint(code string) (float64, float64, error) {
	raw, err := c.doJSONData(http.MethodGet, "/airports/", nil)
	if err != nil {
		return 0, 0, err
	}
	var wire struct {
		Results []struct {
			IATA string  `json:"iata_code"`
			Lat  float64 `json:"latitude"`
			Lng  float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return 0, 0, exitcode.APIf("decode airports: %v", err)
	}
	for _, a := range wire.Results {
		if strings.EqualFold(a.IATA, code) {
			return a.Lat, a.Lng, nil
		}
	}
	return 0, 0, exitcode.NotFoundf("no airport with code %q", code)
}

// ListReservationsPage reads one page, newest first, sized for this slow endpoint.
func (c *Client) ListReservationsPage(page int) ([]Reservation, error) {
	raw, err := c.doJSONData(http.MethodGet, fmt.Sprintf("%s?page_size=%d&page=%d", PathReservations, reservationPageSize, page), nil)
	if err != nil {
		return nil, err
	}
	return parseReservationsPayload(raw)
}

func validateWindowOrder(periods []SearchPeriod) error {
	if len(periods) == 0 {
		return nil
	}
	start, err1 := time.Parse(time.RFC3339, periods[0].Starts)
	end, err2 := time.Parse(time.RFC3339, periods[0].Ends)
	if err1 != nil || err2 != nil {
		return nil
	}
	if !end.After(start) {
		return exitcode.Usagef("--ends must be after --starts")
	}
	return nil
}

// FacilityLocation is the facility's published time zone, or nil when it names none.
func (c *Client) FacilityLocation(facilityID int) (*time.Location, error) {
	tz, err := c.facilityTimeZone(facilityID)
	if err != nil || tz == "" {
		return nil, err
	}
	return time.LoadLocation(tz)
}

// facilityTimeZone reads the zone from the facility's address. The inventory endpoint
// answers without a window, so this needs no prior search. Empty means not published.
func (c *Client) facilityTimeZone(facilityID int) (string, error) {
	var resp struct {
		Result struct {
			Facility struct {
				Common struct {
					Addresses []struct {
						TimeZone string `json:"time_zone"`
					} `json:"addresses"`
				} `json:"common"`
			} `json:"facility"`
		} `json:"result"`
	}
	if err := c.doCraigJSON(http.MethodGet, fmt.Sprintf(PathCraigTransientFacility, facilityID), nil, nil, &resp); err != nil {
		return "", err
	}
	for _, a := range resp.Result.Facility.Common.Addresses {
		if a.TimeZone != "" {
			return a.TimeZone, nil
		}
	}
	return "", nil
}

// CheckoutSucceeded is strict on purpose: a 2xx without a purchase or an order id is
// not proof of a booking, so the caller must report it as unknown rather than booked.
func CheckoutSucceeded(result map[string]any) bool {
	if id, ok := result["order_id"]; ok && id != nil && id != "" {
		return true
	}
	purchases, ok := result["purchases"].([]any)
	return ok && len(purchases) > 0
}

// validateWindow reports bad windows in plain words. The inventory API answers these
// with a raw rpc error that tells the user nothing.
func validateWindow(periods []SearchPeriod) error {
	if err := validateWindowOrder(periods); err != nil || len(periods) == 0 {
		return err
	}
	start, err := time.Parse(time.RFC3339, periods[0].Starts)
	if err != nil {
		return nil
	}
	if !start.After(time.Now()) {
		return exitcode.Usagef("--starts is in the past")
	}
	return nil
}

func firstResultTimeZone(results []json.RawMessage) string {
	for _, raw := range results {
		var r struct {
			Facility struct {
				Common struct {
					Addresses []struct {
						TimeZone string `json:"time_zone"`
					} `json:"addresses"`
				} `json:"common"`
			} `json:"facility"`
		}
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		for _, a := range r.Facility.Common.Addresses {
			if a.TimeZone != "" {
				return a.TimeZone
			}
		}
	}
	return ""
}

// locationWarning is a nudge rather than an error: landmarks legitimately name no city,
// but a search for one place that returns another must not pass silently.
func locationWarning(address, citySlug string) string {
	if address == "" || citySlug == "" {
		return ""
	}
	lower := strings.ToLower(address)
	// SpotHero's slug for New York is an abbreviation no address spells out.
	if citySlug == "nyc" && (strings.Contains(lower, "new york") || strings.Contains(lower, "ny")) {
		return ""
	}
	for _, part := range strings.Split(citySlug, "-") {
		if part != "parking" && strings.Contains(lower, part) {
			return ""
		}
	}
	return fmt.Sprintf("Results are for the SpotHero city %q, which your address does not name. Check this is the right place.", citySlug)
}

// searchNextPage follows @next only on the inventory host, since the URL comes from a
// response and this client would otherwise fetch wherever it pointed.
func (c *Client) searchNextPage(next string) (craigSearchResponse, error) {
	u, err := url.Parse(next)
	if err != nil {
		return craigSearchResponse{}, exitcode.APIf("bad next link")
	}
	base, err := url.Parse(c.CraigBaseURL)
	if err != nil || u.Host != base.Host {
		return craigSearchResponse{}, exitcode.APIf("next link left the inventory host")
	}
	var resp craigSearchResponse
	err = c.doCraigJSON(http.MethodGet, strings.TrimPrefix(u.Path, base.Path), u.Query(), nil, &resp)
	return resp, err
}
