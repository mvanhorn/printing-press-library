// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package tock

import (
	"context"
	"errors"
	"strings"
	"testing"
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
	}
	for _, tc := range wrapped {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.base) {
				t.Errorf("errors.Is(%q, %v) = false; sentinel must be retrievable", tc.err, tc.base)
			}
			others := []error{ErrBookingNotImplemented, ErrPaymentRequired, ErrPastCancellationWindow, ErrCanaryUnrecognizedBody, ErrUpcomingShapeChanged}
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
