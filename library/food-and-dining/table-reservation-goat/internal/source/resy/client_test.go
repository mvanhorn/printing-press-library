// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package resy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// captureTransport records every outgoing request so tests can assert header
// and body shape without spinning up an httptest.Server for each case.
type captureTransport struct {
	requests []*http.Request
	bodies   []string
	response func(req *http.Request) *http.Response
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
		req.Body = io.NopCloser(strings.NewReader(body))
	}
	c.requests = append(c.requests, req)
	c.bodies = append(c.bodies, body)
	if c.response != nil {
		return c.response(req), nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     http.Header{},
	}, nil
}

func newCaptureClient(t *testing.T, creds Credentials) (*Client, *captureTransport) {
	t.Helper()
	tr := &captureTransport{}
	c := New(creds).WithHTTPClient(&http.Client{Transport: tr})
	return c, tr
}

func TestBuildHeadersAttachesAPIKeyAndToken(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{AuthToken: "tokABCD1234"})
	_, _, _ = c.do(context.Background(), http.MethodGet, "/2/user", requestOpts{})
	if len(tr.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(tr.requests))
	}
	got := tr.requests[0]
	if want := `ResyAPI api_key="` + PublicClientID + `"`; got.Header.Get("Authorization") != want {
		t.Errorf("Authorization = %q, want %q", got.Header.Get("Authorization"), want)
	}
	if got.Header.Get("X-Resy-Auth-Token") != "tokABCD1234" {
		t.Errorf("X-Resy-Auth-Token = %q, want %q", got.Header.Get("X-Resy-Auth-Token"), "tokABCD1234")
	}
	if got.Header.Get("X-Resy-Universal-Auth") != "tokABCD1234" {
		t.Errorf("X-Resy-Universal-Auth = %q, want %q", got.Header.Get("X-Resy-Universal-Auth"), "tokABCD1234")
	}
}

func TestBuildHeadersOmitsAuthHeadersWhenAnonymous(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{})
	_, _, _ = c.do(context.Background(), http.MethodPost, "/3/auth/password", requestOpts{
		body:        &url.Values{"email": []string{"x"}},
		formEncoded: true,
	})
	got := tr.requests[0]
	if got.Header.Get("X-Resy-Auth-Token") != "" {
		t.Errorf("expected no X-Resy-Auth-Token on anonymous request, got %q", got.Header.Get("X-Resy-Auth-Token"))
	}
	// API key should still be present — Resy's gateway rejects calls without one
	// even when authenticated user is absent.
	if !strings.Contains(got.Header.Get("Authorization"), PublicClientID) {
		t.Errorf("expected Authorization with PublicClientID, got %q", got.Header.Get("Authorization"))
	}
}

func TestPostBookingDetailsSendsJSONBody(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{AuthToken: "tok"})
	_, _ = c.rawBookingDetails(context.Background(), "cfg-123", "2026-06-01", 4)
	if got := tr.requests[0].Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	body := tr.bodies[0]
	for _, want := range []string{`"config_id":"cfg-123"`, `"day":"2026-06-01"`, `"party_size":4`} {
		if !strings.Contains(body, want) {
			t.Errorf("details body missing %q\nbody: %s", want, body)
		}
	}
}

func TestConfirmBookingSendsFormEncodedBodyWithJSONPaymentMethod(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{AuthToken: "tok"})
	_, _ = c.rawConfirmBooking(context.Background(), "bt-xyz", "12345", "")
	if got := tr.requests[0].Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", got)
	}
	body := tr.bodies[0]
	form, err := url.ParseQuery(body)
	if err != nil {
		t.Fatalf("body is not parseable form-encoded: %v\nbody: %s", err, body)
	}
	if form.Get("book_token") != "bt-xyz" {
		t.Errorf("book_token = %q, want bt-xyz", form.Get("book_token"))
	}
	// struct_payment_method must be a JSON object with the id UNQUOTED
	// (Resy's web client emits numeric ids as JSON numbers; quoting
	// breaks /3/book). The Go marshaller preserves the int64 shape via
	// the strconv.ParseInt round-trip in rawConfirmBooking.
	if pm := form.Get("struct_payment_method"); pm != `{"id":12345}` {
		t.Errorf("struct_payment_method = %q, want %q (numeric id unquoted)", pm, `{"id":12345}`)
	}
	if form.Get("source_id") != "resy.com-venue-details" {
		t.Errorf("source_id default = %q, want resy.com-venue-details", form.Get("source_id"))
	}
}

func TestVenueIdentityByIDRequiresExactIDMatch(t *testing.T) {
	// /4/find can return rows with empty venue.id (related venues,
	// theoretical edge case). VenueIdentityByID must NOT fall back to
	// returning those — only an exact id match is a valid identity
	// claim for the requested venue. Empty + nil err is the correct
	// "not on Resy" signal.
	cases := []struct {
		name         string
		body         string
		queryVenueID string
		wantID       string
		wantName     string
	}{
		{
			name:         "exact match returns identity",
			body:         `{"results":{"venues":[{"venue":{"id":{"resy":1387},"name":"Le Bernardin"}}]}}`,
			queryVenueID: "1387",
			wantID:       "1387",
			wantName:     "Le Bernardin",
		},
		{
			name:         "mismatched id is not returned",
			body:         `{"results":{"venues":[{"venue":{"id":{"resy":9999},"name":"Other Venue"}}]}}`,
			queryVenueID: "1387",
			wantID:       "",
			wantName:     "",
		},
		{
			name:         "empty id row is not returned",
			body:         `{"results":{"venues":[{"venue":{"name":"Empty ID Row"}}]}}`,
			queryVenueID: "1387",
			wantID:       "",
			wantName:     "",
		},
		{
			name:         "match wins over noise rows",
			body:         `{"results":{"venues":[{"venue":{"name":"Sibling"}},{"venue":{"id":{"resy":1387},"name":"Le Bernardin"}}]}}`,
			queryVenueID: "1387",
			wantID:       "1387",
			wantName:     "Le Bernardin",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &captureTransport{
				response: func(req *http.Request) *http.Response {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(tc.body)),
						Header:     http.Header{},
					}
				},
			}
			c := New(Credentials{AuthToken: "tok"}).WithHTTPClient(&http.Client{Transport: tr})
			got, err := c.VenueIdentityByID(context.Background(), tc.queryVenueID, "2026-06-01", 2)
			if err != nil {
				t.Fatalf("VenueIdentityByID: %v", err)
			}
			if got.ID != tc.wantID || got.Name != tc.wantName {
				t.Errorf("got {ID:%q Name:%q}; want {ID:%q Name:%q}", got.ID, got.Name, tc.wantID, tc.wantName)
			}
		})
	}
}

func TestConfirmBookingFallsBackToStringForNonNumericPaymentID(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{AuthToken: "tok"})
	_, _ = c.rawConfirmBooking(context.Background(), "bt-xyz", "opaque-card-token", "")
	form, _ := url.ParseQuery(tr.bodies[0])
	if pm := form.Get("struct_payment_method"); pm != `{"id":"opaque-card-token"}` {
		t.Errorf("struct_payment_method (non-numeric fallback) = %q, want %q", pm, `{"id":"opaque-card-token"}`)
	}
}

func TestCancelSendsFormEncodedBody(t *testing.T) {
	c, tr := newCaptureClient(t, Credentials{AuthToken: "tok"})
	_, _ = c.rawCancel(context.Background(), "resy-9999")
	if got := tr.requests[0].Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", got)
	}
	form, _ := url.ParseQuery(tr.bodies[0])
	if got := form.Get("resy_token"); got != "resy-9999" {
		t.Errorf("resy_token = %q, want resy-9999", got)
	}
}

func TestDoMaps401ToErrAuthExpired(t *testing.T) {
	tr := &captureTransport{
		response: func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"message":"expired"}`)),
				Header:     http.Header{},
			}
		},
	}
	c := New(Credentials{AuthToken: "tok"}).WithHTTPClient(&http.Client{Transport: tr})
	_, _, err := c.do(context.Background(), http.MethodGet, "/2/user", requestOpts{})
	if err != ErrAuthExpired {
		t.Errorf("expected ErrAuthExpired, got %v", err)
	}
}

func TestDoMaps410ToErrSlotTaken(t *testing.T) {
	tr := &captureTransport{
		response: func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusGone,
				Body:       io.NopCloser(strings.NewReader("gone")),
				Header:     http.Header{},
			}
		},
	}
	c := New(Credentials{AuthToken: "tok"}).WithHTTPClient(&http.Client{Transport: tr})
	_, _, err := c.do(context.Background(), http.MethodPost, "/3/details", requestOpts{})
	if err != ErrSlotTaken {
		t.Errorf("expected ErrSlotTaken, got %v", err)
	}
}

func TestWhoamiReturnsAuthMissingWithoutToken(t *testing.T) {
	c := New(Credentials{})
	_, err := c.Whoami(context.Background())
	if err != ErrAuthMissing {
		t.Errorf("expected ErrAuthMissing, got %v", err)
	}
}

func TestLoginWithPasswordParsesToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/auth/password" {
			t.Errorf("path = %q, want /3/auth/password", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}
		_ = r.ParseForm()
		if r.FormValue("email") != "u@example.com" || r.FormValue("password") != "pw" {
			t.Errorf("form = %v", r.Form)
		}
		_, _ = w.Write([]byte(`{"token":"jwt-12345","id":42,"first_name":"User","em_address":"u@example.com"}`))
	}))
	t.Cleanup(server.Close)

	// Point the client at the test server by replacing the base URL via a
	// custom transport that rewrites the host. Simpler than threading a
	// base URL through the constructor for one test.
	c := New(Credentials{}).WithHTTPClient(&http.Client{Transport: &rewriteHostTransport{target: server.URL}})
	resp, err := c.LoginWithPassword(context.Background(), "u@example.com", "pw")
	if err != nil {
		t.Fatalf("LoginWithPassword: %v", err)
	}
	if resp.Token != "jwt-12345" {
		t.Errorf("Token = %q, want jwt-12345", resp.Token)
	}
	if resp.ID != 42 {
		t.Errorf("ID = %d, want 42", resp.ID)
	}
	if resp.FirstName != "User" {
		t.Errorf("FirstName = %q, want User", resp.FirstName)
	}
}

// rewriteHostTransport replaces every request's scheme+host with `target`
// while preserving the path and query. Used to point a client at httptest
// servers without changing its base URL constant.
type rewriteHostTransport struct {
	target string
}

func (r *rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	parsed, err := url.Parse(r.target)
	if err != nil {
		return nil, err
	}
	req.URL.Scheme = parsed.Scheme
	req.URL.Host = parsed.Host
	req.Host = parsed.Host
	return http.DefaultTransport.RoundTrip(req)
}
