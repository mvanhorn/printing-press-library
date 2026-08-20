// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package tock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildVenueDeepLinkURL_WithExperience(t *testing.T) {
	got := buildVenueDeepLinkURL("farzi-cafe-bellevue", 460115, "2026-05-14", "14:30", 2)
	wantContains := []string{
		"https://www.exploretock.com/farzi-cafe-bellevue/experience/460115",
		"date=2026-05-14",
		"size=2",
		"time=14%3A30",
	}
	for _, sub := range wantContains {
		if !strings.Contains(got, sub) {
			t.Errorf("buildVenueDeepLinkURL = %q; missing %q", got, sub)
		}
	}
}

func TestBuildVenueDeepLinkURL_WithoutExperience(t *testing.T) {
	got := buildVenueDeepLinkURL("canlis", 0, "2026-05-14", "19:00", 4)
	wantContains := []string{
		"https://www.exploretock.com/canlis?",
		"date=2026-05-14",
		"size=4",
		"time=19%3A00",
	}
	for _, sub := range wantContains {
		if !strings.Contains(got, sub) {
			t.Errorf("buildVenueDeepLinkURL = %q; missing %q", got, sub)
		}
	}
	// Should NOT contain /experience/ when experienceID == 0
	if strings.Contains(got, "/experience/") {
		t.Errorf("buildVenueDeepLinkURL = %q; should not include /experience/ when experienceID=0", got)
	}
}

func TestBook_DispatchesToChromeBook(t *testing.T) {
	// v0.2 contract: Book() delegates to ChromeBook() which drives a real
	// Chrome session. Without Chrome running on localhost:9222 AND with the
	// stealth-spawned fallback unable to launch in the test environment,
	// ChromeBook returns an error — but it must reach the chromedp layer,
	// proving the dispatch is wired correctly.
	c := &Client{}
	_, err := c.Book(context.Background(), BookRequest{
		VenueSlug: "farzi-cafe-bellevue", ExperienceID: 460115,
		ReservationDate: "2026-05-14", ReservationTime: "14:30", PartySize: 2,
	})
	if err == nil {
		t.Fatal("Book() returned nil error; expected chromedp-layer error in test env")
	}
	// The error must be from chromedp (e.g., "tock chromebook: ..." prefix)
	// or a validation error. It must NOT match ErrBookingNotImplemented since
	// that stub was replaced.
	if errors.Is(err, ErrBookingNotImplemented) {
		t.Errorf("Book() error should not match ErrBookingNotImplemented (stub replaced); got %v", err)
	}
	if !strings.Contains(err.Error(), "tock chromebook") && !strings.Contains(err.Error(), "tock") {
		t.Errorf("Book() error should be from chromedp layer; got %v", err)
	}
}

func TestCancelRequiresIDs(t *testing.T) {
	c := &Client{}
	_, err := c.Cancel(context.Background(), CancelRequest{})
	if err == nil {
		t.Fatal("Cancel with empty request should error")
	}
	if !strings.Contains(err.Error(), "VenueSlug") || !strings.Contains(err.Error(), "PurchaseID") {
		t.Errorf("Cancel error should name missing fields; got %v", err)
	}
}

func TestCancelHTTPFirst_Direct404TriggersUIFallback(t *testing.T) {
	var uiCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/venue/receipt/cancel" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer srv.Close()
	c := newCancelHTTPTestClient(t, srv)
	c.cancelViaUI = func(_ context.Context, req CancelRequest) (*CancelResponse, error) {
		uiCalls++
		return canceledResponse(req), nil
	}
	resp, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.Canceled || uiCalls != 1 {
		t.Fatalf("response=%+v uiCalls=%d, want canceled and one UI fallback", resp, uiCalls)
	}
}

func TestCancelHTTPFirst_Direct404AfterCSRFRetryTriggersUIFallback(t *testing.T) {
	var postCalls, uiCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `<input type="hidden" name="csrf_token" value="fixture-token">`)
		case http.MethodPost:
			postCalls++
			if postCalls == 1 {
				http.Error(w, "csrf required", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.Form.Get("csrf_token"); got != "fixture-token" {
				t.Fatalf("csrf_token=%q, want fixture-token", got)
			}
			http.Error(w, "endpoint missing", http.StatusNotFound)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()
	c := newCancelHTTPTestClient(t, srv)
	c.cancelViaUI = func(_ context.Context, req CancelRequest) (*CancelResponse, error) {
		uiCalls++
		return canceledResponse(req), nil
	}
	resp, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.Canceled || postCalls != 2 || uiCalls != 1 {
		t.Fatalf("response=%+v postCalls=%d uiCalls=%d, want canceled, two POSTs, and one UI fallback", resp, postCalls, uiCalls)
	}
}

func TestCancelHTTPFirst_CSRFRetryPreservesSuccessAndAuthFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		retryStatus int
		retryBody   string
		wantSuccess bool
	}{
		{name: "recognized success", retryStatus: http.StatusOK, retryBody: "Reservation canceled", wantSuccess: true},
		{name: "still unauthorized", retryStatus: http.StatusUnauthorized, retryBody: "still unauthorized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			postCalls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_, _ = io.WriteString(w, `<input type="hidden" name="csrf_token" value="fixture-token">`)
				case http.MethodPost:
					postCalls++
					if postCalls == 1 {
						http.Error(w, "csrf required", http.StatusForbidden)
						return
					}
					if err := r.ParseForm(); err != nil {
						t.Fatalf("ParseForm: %v", err)
					}
					if got := r.Form.Get("csrf_token"); got != "fixture-token" {
						t.Fatalf("csrf_token=%q, want fixture-token", got)
					}
					w.WriteHeader(tc.retryStatus)
					_, _ = io.WriteString(w, tc.retryBody)
				default:
					t.Fatalf("unexpected method %s", r.Method)
				}
			}))
			defer srv.Close()
			c := newCancelHTTPTestClient(t, srv)
			uiCalls := 0
			c.cancelViaUI = func(context.Context, CancelRequest) (*CancelResponse, error) {
				uiCalls++
				return nil, errors.New("must not be called")
			}
			resp, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
			if tc.wantSuccess {
				if err != nil || resp == nil || !resp.Canceled {
					t.Fatalf("Cancel resp=%+v err=%v, want recognized success", resp, err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "HTTP 401 after CSRF retry") {
					t.Fatalf("Cancel err=%v, want preserved post-retry auth failure", err)
				}
			}
			if postCalls != 2 || uiCalls != 0 {
				t.Fatalf("postCalls=%d uiCalls=%d, want two POSTs and no UI fallback", postCalls, uiCalls)
			}
		})
	}
}

func TestCancelHTTPFirst_AmbiguousOutcomesNeverTriggerUI(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, c *Client) func()
	}{
		{
			name: "transport error",
			setup: func(t *testing.T, c *Client) func() {
				c.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("fixture transport failure")
				})}
				return func() {}
			},
		},
		{
			name: "timeout",
			setup: func(t *testing.T, c *Client) func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(100 * time.Millisecond)
					_, _ = io.WriteString(w, "late")
				}))
				c.http = srv.Client()
				c.http.Timeout = 10 * time.Millisecond
				c.origin = srv.URL
				return srv.Close
			},
		},
		{
			name: "redirected final 404",
			setup: func(t *testing.T, c *Client) func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/venue/receipt/cancel" {
						http.Redirect(w, r, "/final-missing", http.StatusFound)
						return
					}
					http.Error(w, "final missing", http.StatusNotFound)
				}))
				c.http = srv.Client()
				c.origin = srv.URL
				return srv.Close
			},
		},
		{
			name: "unrecognized 2xx",
			setup: func(t *testing.T, c *Client) func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = io.WriteString(w, "unexpected success body")
				}))
				c.http = srv.Client()
				c.origin = srv.URL
				return srv.Close
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCancelHTTPTestClient(t, nil)
			cleanup := tc.setup(t, c)
			defer cleanup()
			uiCalls := 0
			c.cancelViaUI = func(context.Context, CancelRequest) (*CancelResponse, error) {
				uiCalls++
				return nil, errors.New("must not be called")
			}
			_, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
			if err == nil {
				t.Fatal("Cancel returned nil error for ambiguous outcome")
			}
			if uiCalls != 0 {
				t.Fatalf("UI fallback calls=%d, want 0", uiCalls)
			}
		})
	}
}

func TestCancelHTTPFirst_PreservesTypedNonFallbackSemantics(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantIs     error
		wantString string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, body: "auth", wantString: "HTTP 401"},
		{name: "forbidden", status: http.StatusForbidden, body: "auth", wantString: "HTTP 403"},
		{name: "past window", status: http.StatusGone, body: "gone", wantIs: ErrPastCancellationWindow},
		{name: "canary", status: http.StatusOK, body: "unknown", wantIs: ErrCanaryUnrecognizedBody},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, "no tokens")
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			c := newCancelHTTPTestClient(t, srv)
			_, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
			if err == nil {
				t.Fatal("Cancel returned nil error")
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("errors.Is(%v, %v)=false", err, tc.wantIs)
			}
			if tc.wantString != "" && !strings.Contains(err.Error(), tc.wantString) {
				t.Fatalf("error %q missing %q", err, tc.wantString)
			}
		})
	}
}

func TestCancelErrorsDoNotExposeURLsProviderCopyOrIdentifiers(t *testing.T) {
	const (
		slug     = "private-venue-slug"
		purchase = 987654321
	)
	secrets := []string{slug, "987654321", "query-token-secret", "provider copy secret", "exploretock.com"}
	tests := []struct {
		name  string
		setup func(t *testing.T, c *Client) func()
	}{
		{
			name: "URL-bearing transport cause through outer wrap",
			setup: func(_ *testing.T, c *Client) func() {
				c.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New(`Post "https://www.exploretock.com/private-venue-slug/receipt/cancel?purchaseId=987654321&token=query-token-secret": provider copy secret`)
				})}
				return func() {}
			},
		},
		{
			name: "HTTP provider body",
			setup: func(_ *testing.T, c *Client) func() {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "provider copy secret query-token-secret", http.StatusInternalServerError)
				}))
				c.http = srv.Client()
				c.origin = srv.URL
				return srv.Close
			},
		},
		{
			name: "malformed request URL",
			setup: func(_ *testing.T, c *Client) func() {
				c.origin = "://query-token-secret"
				return func() {}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCancelHTTPTestClient(t, nil)
			cleanup := tc.setup(t, c)
			defer cleanup()
			_, err := c.Cancel(context.Background(), CancelRequest{VenueSlug: slug, PurchaseID: purchase})
			if err == nil {
				t.Fatal("Cancel returned nil error")
			}
			got := fmt.Errorf("outer cancel wrapper: %w", err).Error()
			for _, secret := range secrets {
				if strings.Contains(got, secret) {
					t.Fatalf("privacy leak %q in %q", secret, got)
				}
			}
			for cause := err; cause != nil; cause = errors.Unwrap(cause) {
				for _, secret := range secrets {
					if strings.Contains(cause.Error(), secret) {
						t.Fatalf("privacy leak %q in unwrapped cause %q", secret, cause.Error())
					}
				}
			}
		})
	}
}

func TestCancelTransportErrorPreservesSafeTypedCauses(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  error
	}{
		{name: "deadline", cause: fmt.Errorf("URL-bearing wrapper: %w", context.DeadlineExceeded), want: context.DeadlineExceeded},
		{name: "canceled", cause: fmt.Errorf("URL-bearing wrapper: %w", context.Canceled), want: context.Canceled},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := newCancelTransportError("transport error", tc.cause)
			if !errors.Is(err, tc.want) {
				t.Fatalf("errors.Is(%v, %v)=false", err, tc.want)
			}
			if strings.Contains(errors.Unwrap(err).Error(), "URL-bearing") {
				t.Fatalf("raw wrapper leaked through safe cause: %v", errors.Unwrap(err))
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newCancelHTTPTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if srv != nil {
		c.http = srv.Client()
		c.origin = srv.URL
	} else {
		c.origin = "http://fixture.invalid"
	}
	return c
}

func TestExtractCSRFTokens(t *testing.T) {
	cases := []struct {
		name string
		html string
		want map[string]string
	}{
		{
			name: "rails-style authenticity_token",
			html: `<form><input type="hidden" name="authenticity_token" value="abc123"></form>`,
			want: map[string]string{"authenticity_token": "abc123"},
		},
		{
			name: "dotnet-style RequestVerificationToken",
			html: `<input type="hidden" name="__RequestVerificationToken" value="xyz789">`,
			want: map[string]string{"__RequestVerificationToken": "xyz789"},
		},
		{
			name: "csrfToken next-style",
			html: `<input type='hidden' name='csrfToken' value='tok'>`,
			want: map[string]string{"csrfToken": "tok"},
		},
		{
			name: "ignores non-csrf hidden inputs",
			html: `<input type="hidden" name="purchaseId" value="362575651">` +
				`<input type="hidden" name="csrf_token" value="abc">` +
				`<input type="hidden" name="venueSlug" value="canlis">`,
			want: map[string]string{"csrf_token": "abc"},
		},
		{
			name: "multiple csrf-shaped tokens",
			html: `<input type="hidden" name="csrf" value="A">` +
				`<input type="hidden" name="xsrfHeader" value="B">`,
			want: map[string]string{"csrf": "A", "xsrfHeader": "B"},
		},
		{
			name: "no hidden inputs",
			html: `<html><body>nothing here</body></html>`,
			want: map[string]string{},
		},
		{
			name: "empty value preserved",
			html: `<input type="hidden" name="csrf_token" value="">`,
			want: map[string]string{"csrf_token": ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCSRFTokens(tc.html)
			if len(got) != len(tc.want) {
				t.Fatalf("extractCSRFTokens len = %d (%v); want %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for k, v := range tc.want {
				if got.Get(k) != v {
					t.Errorf("extractCSRFTokens[%q] = %q; want %q", k, got.Get(k), v)
				}
			}
		})
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	wrapped := []struct {
		name string
		err  error
		base error
	}{
		{"booking-not-impl", errors.Join(ErrBookingNotImplemented, errors.New("v0.2")), ErrBookingNotImplemented},
		{"payment-required", errors.Join(ErrPaymentRequired, errors.New("prepay")), ErrPaymentRequired},
		{"past-window", errors.Join(ErrPastCancellationWindow, errors.New("HTTP 410")), ErrPastCancellationWindow},
		{"canary", errors.Join(ErrCanaryUnrecognizedBody, errors.New("decode fail")), ErrCanaryUnrecognizedBody},
		{"upcoming-shape", errors.Join(ErrUpcomingShapeChanged, errors.New("missing")), ErrUpcomingShapeChanged},
		{"slot-control", errors.Join(ErrSlotControlNotFound, errors.New("page-state")), ErrSlotControlNotFound},
		{"cvc-required", errors.Join(ErrCVCRequired, errors.New("empty")), ErrCVCRequired},
	}
	for _, tc := range wrapped {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.base) {
				t.Errorf("errors.Is(%q, %v) = false; sentinel must be retrievable", tc.err, tc.base)
			}
			others := []error{ErrBookingNotImplemented, ErrPaymentRequired, ErrPastCancellationWindow, ErrCanaryUnrecognizedBody, ErrUpcomingShapeChanged, ErrSlotControlNotFound, ErrCVCRequired}
			for _, o := range others {
				if o == tc.base {
					continue
				}
				if errors.Is(tc.err, o) {
					t.Errorf("errors.Is(%q, %v) = true; sentinels must be distinct", tc.err, o)
				}
			}
		})
	}
}
