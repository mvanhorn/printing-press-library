// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Credentials are the per-user fields required to talk to authenticated Resy
// endpoints. APIKey defaults to PublicClientID when empty.
type Credentials struct {
	APIKey    string
	AuthToken string
	Email     string
}

func (c Credentials) effectiveAPIKey() string {
	if c.APIKey == "" {
		return PublicClientID
	}
	return c.APIKey
}

// Client is a minimal Resy HTTP client. It carries the user's credentials and
// a stdlib http.Client; no cookie jar, no fingerprinting — Resy's API gateway
// doesn't gate on TLS like OpenTable's Akamai does.
type Client struct {
	creds      Credentials
	httpClient *http.Client
	userAgent  string
}

// New constructs a Resy client. Pass empty Credentials for anonymous calls
// (only loginWithPassword and a handful of public reads work that way);
// pass real credentials for authenticated endpoints.
func New(creds Credentials) *Client {
	return &Client{
		creds:      creds,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "table-reservation-goat-pp-cli/0.1.0 (+https://github.com/mvanhorn/printing-press-library)",
	}
}

// WithHTTPClient lets tests inject a stub http.Client.
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.httpClient = hc
	return c
}

// buildHeaders returns the headers Resy expects on every authenticated
// request. The same set is sent on anonymous calls too (Resy ignores the
// empty token); only loginWithPassword expects a missing X-Resy-Auth-Token.
func (c *Client) buildHeaders(extra map[string]string) http.Header {
	h := http.Header{}
	h.Set("Authorization", fmt.Sprintf(`ResyAPI api_key="%s"`, c.creds.effectiveAPIKey()))
	if c.creds.AuthToken != "" {
		h.Set("X-Resy-Auth-Token", c.creds.AuthToken)
		h.Set("X-Resy-Universal-Auth", c.creds.AuthToken)
	}
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("User-Agent", c.userAgent)
	for k, v := range extra {
		h.Set(k, v)
	}
	return h
}

// requestOpts is the internal request shape. Body is either nil, a string for
// pre-encoded form bodies, or anything json.Marshal accepts.
type requestOpts struct {
	headers map[string]string
	body    any
	// formEncoded, when true, sends body (which must be a *url.Values or
	// string) as application/x-www-form-urlencoded. Resy's /3/auth/password,
	// /3/book, and /3/cancel still require this even though /3/details
	// migrated to JSON in 2026.
	formEncoded bool
}

// do executes a Resy API request. Returns the raw response bytes plus the
// HTTP status. Callers parse the body themselves so the client stays
// untyped at the wire layer.
func (c *Client) do(ctx context.Context, method, path string, opts requestOpts) ([]byte, int, error) {
	target := path
	if !strings.HasPrefix(target, "http") {
		target = ApiBase + path
	}

	var bodyReader io.Reader
	var contentType string
	switch {
	case opts.body == nil:
		// no body
	case opts.formEncoded:
		switch b := opts.body.(type) {
		case *url.Values:
			bodyReader = strings.NewReader(b.Encode())
		case url.Values:
			bodyReader = strings.NewReader(b.Encode())
		case string:
			bodyReader = strings.NewReader(b)
		default:
			return nil, 0, fmt.Errorf("resy: formEncoded body must be url.Values or string, got %T", opts.body)
		}
		contentType = "application/x-www-form-urlencoded"
	default:
		raw, err := json.Marshal(opts.body)
		if err != nil {
			return nil, 0, fmt.Errorf("resy: marshal body: %w", err)
		}
		bodyReader = strings.NewReader(string(raw))
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("resy: build request: %w", err)
	}
	req.Header = c.buildHeaders(opts.headers)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("resy: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("resy: read body: %w", err)
	}

	switch resp.StatusCode {
	case 401, 419:
		return respBody, resp.StatusCode, ErrAuthExpired
	case 410:
		return respBody, resp.StatusCode, ErrSlotTaken
	}
	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("resy %s %s: HTTP %d: %s",
			method, path, resp.StatusCode, truncateBody(respBody))
	}
	return respBody, resp.StatusCode, nil
}

// LoginWithPassword exchanges email + password for an auth token. The
// response is the full Resy user object; callers extract resp.Token and
// store it as Credentials.AuthToken.
//
// Resy still expects form-encoded bodies on this endpoint (verified 2026).
//
// 401/419 on /3/auth/password means the credentials are wrong, NOT that a
// token has expired — there is no token yet. The generic `do()` mapper
// returns ErrAuthExpired for those statuses to match the authenticated
// endpoints. We intercept that here so the user-facing message is
// actionable ("check email/password") rather than misleading ("token
// expired") for the login flow specifically.
func (c *Client) LoginWithPassword(ctx context.Context, email, password string) (LoginResponse, error) {
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	body, _, err := c.do(ctx, http.MethodPost, "/3/auth/password", requestOpts{
		body:        &form,
		formEncoded: true,
	})
	if err != nil {
		if err == ErrAuthExpired {
			return LoginResponse{}, fmt.Errorf("resy: login rejected — check email/password (verify by signing in at resy.com)")
		}
		return LoginResponse{}, err
	}
	var out LoginResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return LoginResponse{}, fmt.Errorf("resy: parse login response: %w", err)
	}
	if out.Token == "" {
		return out, fmt.Errorf("resy: login returned no token (check email/password)")
	}
	return out, nil
}

// LoginResponse is the subset of /3/auth/password we care about.
type LoginResponse struct {
	Token        string `json:"token"`
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	EmailAddress string `json:"em_address"`
}

// Whoami calls /2/user, the endpoint resy-cli uses to validate that a stored
// token still works. Returns the parsed user object on success.
func (c *Client) Whoami(ctx context.Context) (WhoamiResponse, error) {
	if c.creds.AuthToken == "" {
		return WhoamiResponse{}, ErrAuthMissing
	}
	body, _, err := c.do(ctx, http.MethodGet, "/2/user", requestOpts{})
	if err != nil {
		return WhoamiResponse{}, err
	}
	var out WhoamiResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return WhoamiResponse{}, fmt.Errorf("resy: parse whoami: %w", err)
	}
	return out, nil
}

// WhoamiResponse is the subset of /2/user used by auth status displays.
type WhoamiResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// rawSearch fires Resy's venue search. Returned bytes are passed to the
// search.go parser so types-vs-wire stay separated.
//
// API shift (live-tested 2026-05-11): the gateway used to accept a
// `location` field carrying a two/three-letter city code, but the current
// /3/venuesearch/search rejects it as "Unknown field." Sending only
// {query, per_page, types} is the documented working shape; the cityCode
// argument is accepted for future use (e.g., a different filter field
// Resy adds back) but is ignored on the wire.
func (c *Client) rawSearch(ctx context.Context, query, cityCode string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 20
	}
	body := map[string]any{
		"query":    query,
		"per_page": limit,
		"types":    []string{"venue"},
	}
	_ = cityCode // intentionally ignored — see comment above
	respBody, _, err := c.do(ctx, http.MethodPost, "/3/venuesearch/search", requestOpts{body: body})
	return respBody, err
}

// rawFind fires Resy's /4/find endpoint for a given (venue, date, party).
// Coordinates are passed as 0/0 because the API does not require them for a
// venue-id lookup — they're populated for nearby-search variants we don't
// expose.
func (c *Client) rawFind(ctx context.Context, venueID, date string, partySize int) ([]byte, error) {
	q := url.Values{}
	q.Set("lat", "0")
	q.Set("long", "0")
	q.Set("day", date)
	q.Set("party_size", fmt.Sprintf("%d", partySize))
	q.Set("venue_id", venueID)
	body, _, err := c.do(ctx, http.MethodGet, "/4/find?"+q.Encode(), requestOpts{})
	return body, err
}

// rawBookingDetails fires /3/details (step 1 of the two-step book flow).
// Returns the body; the parser extracts book_token + payment_methods.
//
// LIVE API note (verified 2026): /3/details takes JSON, not form-encoded.
// Sending the parameters as URL query string returns "invalid configuration
// ID" even with a valid token.
func (c *Client) rawBookingDetails(ctx context.Context, configID, day string, partySize int) ([]byte, error) {
	body := map[string]any{
		"config_id":  configID,
		"day":        day,
		"party_size": partySize,
	}
	respBody, _, err := c.do(ctx, http.MethodPost, "/3/details", requestOpts{body: body})
	return respBody, err
}

// rawConfirmBooking fires /3/book (step 2 of the two-step book flow). Unlike
// /3/details, this endpoint is STILL form-encoded; sending JSON returns
// "invalid book token". `struct_payment_method` carries a JSON-stringified
// {id} object as a form field — this matches Resy's own web client.
func (c *Client) rawConfirmBooking(ctx context.Context, bookToken, paymentMethodID, sourceID string) ([]byte, error) {
	if sourceID == "" {
		sourceID = "resy.com-venue-details"
	}
	// Preserve the wire type Resy expects: card IDs come back as
	// JSON numbers (`payment_methods[].id: 12345`) and resy.com's own
	// web client re-emits them unquoted as `{"id":12345}`. Marshaling
	// a Go `string` would emit `{"id":"12345"}` which /3/book rejects
	// even when /3/details succeeded. Re-parse the canonical decimal
	// form as int64 so json.Marshal emits a number; only fall back to
	// the string shape when the id is genuinely non-numeric (e.g.
	// future opaque token format).
	var pmIDValue any
	if n, perr := strconv.ParseInt(paymentMethodID, 10, 64); perr == nil {
		pmIDValue = n
	} else {
		pmIDValue = paymentMethodID
	}
	pmJSON, err := json.Marshal(map[string]any{"id": pmIDValue})
	if err != nil {
		// Discarding this error would silently emit
		// `struct_payment_method=null` in the form body and Resy would
		// reject the book with a generic "invalid book token" message
		// that doesn't surface the encoding failure locally. Returning
		// it eagerly keeps the failure mode honest.
		return nil, fmt.Errorf("resy: marshal payment method: %w", err)
	}
	form := url.Values{}
	form.Set("book_token", bookToken)
	form.Set("struct_payment_method", string(pmJSON))
	form.Set("source_id", sourceID)
	body, _, err := c.do(ctx, http.MethodPost, "/3/book", requestOpts{
		body:        &form,
		formEncoded: true,
	})
	return body, err
}

// rawCancel fires /3/cancel. Form-encoded body with `resy_token` as the only
// required field.
func (c *Client) rawCancel(ctx context.Context, reservationID string) ([]byte, error) {
	form := url.Values{}
	form.Set("resy_token", reservationID)
	body, _, err := c.do(ctx, http.MethodPost, "/3/cancel", requestOpts{
		body:        &form,
		formEncoded: true,
	})
	return body, err
}

// rawReservations fires /3/user/reservations and returns the body for the
// parser. Authenticated-only.
func (c *Client) rawReservations(ctx context.Context) ([]byte, error) {
	if c.creds.AuthToken == "" {
		return nil, ErrAuthMissing
	}
	body, _, err := c.do(ctx, http.MethodGet, "/3/user/reservations", requestOpts{})
	return body, err
}

func truncateBody(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
