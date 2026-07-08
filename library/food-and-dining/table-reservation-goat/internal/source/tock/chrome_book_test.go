package tock

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	cdproto "github.com/chromedp/cdproto"
	"github.com/chromedp/chromedp"
)

func withTockDOMFixture(t *testing.T, html string, run func(context.Context)) {
	t.Helper()
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", "new"),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
		)...,
	)
	defer cancelAlloc()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	timed, cancelTimed := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimed()
	dataURL := "data:text/html;charset=utf-8," + url.PathEscape(html)
	if err := chromedp.Run(timed, chromedp.Navigate(dataURL)); err != nil {
		t.Skipf("chromedp unavailable for DOM fixture: %v", err)
	}
	run(timed)
}

func TestClickComboboxExperienceLayout_ChoosesStandardReservationForSmallParty(t *testing.T) {
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:00">6:00 PM</option>
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="standard">
			<h2>Reservation</h2>
			<a href="#" onclick="window.clickedExperience = 'standard'; return false;">Book now</a>
		</section>
		<section class="experience-card" id="group">
			<h2>Reservation: Groups 7-18</h2>
			<a href="#" onclick="window.clickedExperience = 'group'; return false;">Book now</a>
		</section>
		<button id="submit" onclick="window.submitClicked = true;">Book now</button>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", 4, 0)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var selected, clicked string
		var submitClicked bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('#time').value`, &selected),
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submitClicked || false`, &submitClicked),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if selected != "18:15" {
			t.Fatalf("selected time = %q, want 18:15", selected)
		}
		if clicked != "standard" {
			t.Fatalf("clicked experience = %q, want standard", clicked)
		}
		if !submitClicked {
			t.Fatal("expected global submit Book now to be clicked after experience card")
		}
	})
}

func TestClickComboboxExperienceLayout_ChoosesGroupReservationForLargeParty(t *testing.T) {
	html := `
		<!doctype html>
		<select aria-label="Reservation time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="standard">
			<h2>Reservation</h2>
			<a href="#" onclick="window.clickedExperience = 'standard'; return false;">Book now</a>
		</section>
		<section class="experience-card" id="group">
			<h2>Reservation: Groups 7-18</h2>
			<a href="#" onclick="window.clickedExperience = 'group'; return false;">Book now</a>
		</section>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", 8, 0)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedExperience || ''`, &clicked)); err != nil {
			t.Fatalf("read clicked experience: %v", err)
		}
		if clicked != "group" {
			t.Fatalf("clicked experience = %q, want group", clicked)
		}
	})
}

func TestTockBookingPageStateHint_ReportsComboboxControls(t *testing.T) {
	html := `
		<!doctype html>
		<button aria-label="Fewer guests">-</button>
		<button aria-label="More guests">+</button>
		<button role="combobox" aria-label="Time">6:15 PM</button>
		<ul role="listbox"><li role="option">6:15 PM</li></ul>
		<section><h2>Reservation</h2><a href="#">Book now</a></section>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		var hint string
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			hint = tockBookingPageStateHint(actCtx, "")
			return nil
		})); err != nil {
			t.Fatalf("collect page-state hint: %v", err)
		}
		for _, want := range []string{
			`"combobox_layout_detected":true`,
			`"6:15 PM"`,
			`"Book now"`,
		} {
			if !strings.Contains(hint, want) {
				t.Fatalf("page-state hint missing %q: %s", want, hint)
			}
		}
	})
}

func TestTockVenuePageURLMatcherRejectsDeadAndCheckoutTargets(t *testing.T) {
	venueURL := "https://www.exploretock.com/barcelona-wine-bar-raleigh?date=2026-07-10&size=2&time=18%3A15"
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"venue", "https://www.exploretock.com/barcelona-wine-bar-raleigh?date=2026-07-10", true},
		{"experience", "https://www.exploretock.com/barcelona-wine-bar-raleigh/experience/123?date=2026-07-10", false},
		{"about blank", "about:blank", false},
		{"checkout", "https://www.exploretock.com/barcelona-wine-bar-raleigh/checkout/confirm-purchase", false},
		{"receipt", "https://www.exploretock.com/barcelona-wine-bar-raleigh/receipt?purchaseId=1", false},
		{"other venue", "https://www.exploretock.com/other-venue?date=2026-07-10", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTockVenuePageURL(tc.raw, venueURL); got != tc.want {
				t.Fatalf("isTockVenuePageURL(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestTockVenuePageURLMatcherAcceptsMatchingExperienceDeepLink(t *testing.T) {
	venueURL := "https://www.exploretock.com/barcelona-wine-bar-raleigh/experience/123?date=2026-07-10&size=2&time=18%3A15"
	if !isTockVenuePageURL("https://www.exploretock.com/barcelona-wine-bar-raleigh/experience/123?date=2026-07-10", venueURL) {
		t.Fatal("expected matching experience URL to be accepted")
	}
	if isTockVenuePageURL("https://www.exploretock.com/barcelona-wine-bar-raleigh", venueURL) {
		t.Fatal("root venue page must not satisfy an experience-specific recovery target")
	}
}

func TestIsTargetNavigatedOrClosedRecognizesCDPMinus32000(t *testing.T) {
	err := fmt.Errorf("evaluating combobox booking layout: %w", &cdproto.Error{
		Code:    -32000,
		Message: "Inspected target navigated or closed",
	})
	if !isTargetNavigatedOrClosed(err) {
		t.Fatalf("expected CDP -32000 target error to be retryable")
	}
	if isTargetNavigatedOrClosed(fmt.Errorf("evaluating combobox booking layout: ordinary selector miss")) {
		t.Fatal("ordinary selector miss should not be retryable as target loss")
	}
}
