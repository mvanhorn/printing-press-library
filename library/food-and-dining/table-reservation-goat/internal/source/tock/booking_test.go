// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

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
