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
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 4, 0)
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
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 8, 0)
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

// Regression: production calls clickRequestedTockBookingControl with a BARE
// chromedp context (from NewContext, outside any ActionFunc/executor scope).
// v2026.7.1's first live run failed every probe with chromedp's
// "invalid context" because helpers used raw Action.Do(ctx), which requires
// an executor-wrapped context. Calling the top-level entry here the exact
// way ChromeBook does keeps that wiring honest.
func TestClickRequestedTockBookingControl_WorksOnBareChromedpContext(t *testing.T) {
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
		<button id="submit" onclick="window.submitClicked = true;">Book now</button>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		activeCtx, activeCancel, err := clickRequestedTockBookingControl(ctx, "", "6:15 PM", "2026-07-10", "18:15", 2, 0)
		if activeCancel != nil {
			defer activeCancel()
		}
		if err != nil {
			t.Fatalf("clickRequestedTockBookingControl on bare context: %v", err)
		}
		var selected string
		if err := chromedp.Run(activeCtx, chromedp.Evaluate(`document.querySelector('#time').value`, &selected)); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if selected != "18:15" {
			t.Fatalf("selected time = %q, want 18:15", selected)
		}
	})
}

// Mirrors the live deep-link page state from the 2026-07-08 booking run:
// date/time/party rode the URL so Tock renders NO time picker — just
// experience cards with "Book now" controls. The fallback must click the
// best card directly instead of hunting for a combobox.
func TestClickComboboxExperienceLayout_DeepLinkLayoutWithoutTimePicker(t *testing.T) {
	html := `
		<!doctype html>
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
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, 0)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout on deep-link layout: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedExperience || ''`, &clicked)); err != nil {
			t.Fatalf("read clicked experience: %v", err)
		}
		if clicked != "standard" {
			t.Fatalf("clicked experience = %q, want standard for party of 2", clicked)
		}
	})
}

// The live 2026-07-08 Tock venue route exposes broad experience-card links
// and a separate /search result view. The actual checkout transition is the
// exact-time "Book" row in the search results; clicking the card link first
// resets the query to Tock's default date/time and misses the requested slot.
func TestClickComboboxExperienceLayout_UsesSearchResultSlotBeforeExperienceCard(t *testing.T) {
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Desired reservation time">
			<option value="18:00">6:00 PM</option>
			<option value="18:15">6:15 PM</option>
		</select>
		<a id="search" href="/barcelona-wine-bar-raleigh/search?date=2026-07-10&size=2&time=18%3A15"
			onclick="window.searchClicked = true; document.getElementById('results').style.display = 'block'; return false;">Search</a>
		<section class="experience-card" id="standard">
			<h2>Reservation</h2>
			<a href="/barcelona-wine-bar-raleigh/experience/520126/reservation"
				onclick="window.cardClicked = true; return false;">Book now</a>
		</section>
		<div id="results" style="display:none">
			<div class="SearchModalExperiences-item Consumer-reservation">
				<div class="Consumer-resultsListVertical">
					<div class="MuiPaper-root MuiCard-root">
						<div class="MuiCardHeader-root"><span>6:00 PM</span><button onclick="window.bookedSlot = '6:00 PM';">Book</button></div>
					</div>
					<div class="MuiPaper-root MuiCard-root">
						<div class="MuiCardHeader-root"><span>6:15 PM</span><button onclick="window.bookedSlot = '6:15 PM';">Book</button></div>
					</div>
				</div>
			</div>
		</div>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, 0)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout search result flow: %v", err)
		}
		var selected, bookedSlot string
		var searchClicked, cardClicked bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('#time').value`, &selected),
			chromedp.Evaluate(`window.searchClicked || false`, &searchClicked),
			chromedp.Evaluate(`window.cardClicked || false`, &cardClicked),
			chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if selected != "18:15" {
			t.Fatalf("selected time = %q, want 18:15", selected)
		}
		if !searchClicked {
			t.Fatal("expected search result path to be used")
		}
		if cardClicked {
			t.Fatal("experience card link should not be clicked before exact-time result row")
		}
		if bookedSlot != "6:15 PM" {
			t.Fatalf("bookedSlot = %q, want 6:15 PM", bookedSlot)
		}
	})
}

// Mirrors the live experience modal (confirmed in a real browser
// 2026-07-08): clicking an experience card's "Book now" opens an SPA modal
// whose calendar day buttons are named with ISO dates and whose slot rows
// pair a time label with a "Book" button. The modal defaults to TODAY,
// dropping the deep link's date, so the flow must re-select the day.
func TestClickComboboxExperienceLayout_DrivesExperienceModal(t *testing.T) {
	html := `
		<!doctype html>
		<section class="experience-card" id="standard">
			<h2>Reservation</h2>
			<a href="#" onclick="window.openModal(); return false;">Book now</a>
		</section>
		<div id="modal" style="display:none">
			<button aria-label="2026-07-09">9</button>
			<button aria-label="2026-07-10" onclick="window.pickedDay = '2026-07-10'; document.getElementById('slots').style.display = 'block';">10</button>
			<div id="slots" style="display:none">
				<div class="slot-row"><span>5:45 PM</span><button onclick="window.bookedSlot = '5:45 PM';">Book</button></div>
				<div class="slot-row"><span>6:15 PM</span><button onclick="window.bookedSlot = '6:15 PM';">Book</button></div>
			</div>
		</div>
		<script>
			window.openModal = () => { document.getElementById('modal').style.display = 'block'; };
		</script>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, 0)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout modal flow: %v", err)
		}
		var pickedDay, bookedSlot string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.pickedDay || ''`, &pickedDay),
			chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if pickedDay != "2026-07-10" {
			t.Fatalf("pickedDay = %q, want 2026-07-10", pickedDay)
		}
		if bookedSlot != "6:15 PM" {
			t.Fatalf("bookedSlot = %q, want 6:15 PM", bookedSlot)
		}
	})
}

func TestBuildVenueSearchURL(t *testing.T) {
	got := buildVenueSearchURL("https://www.exploretock.com/barcelona-wine-bar-raleigh?date=2026-07-10&size=2&time=18%3A15", "2026-07-10", "18:15", 2)
	want := "https://www.exploretock.com/barcelona-wine-bar-raleigh/search?date=2026-07-10&size=2&time=18%3A15"
	if got != want {
		t.Fatalf("buildVenueSearchURL = %q, want %q", got, want)
	}
	// Experience deep-link URLs must collapse back to the venue's search page.
	got = buildVenueSearchURL("https://www.exploretock.com/canlis/experience/12345?date=2026-07-10&size=4&time=19%3A00", "2026-07-10", "19:00", 4)
	want = "https://www.exploretock.com/canlis/search?date=2026-07-10&size=4&time=19%3A00"
	if got != want {
		t.Fatalf("buildVenueSearchURL(experience) = %q, want %q", got, want)
	}
	if got := buildVenueSearchURL("", "2026-07-10", "18:15", 2); got != "" {
		t.Fatalf("buildVenueSearchURL(empty venue) = %q, want empty", got)
	}
}

// Mirrors the live /search results page (confirmed in a real browser
// 2026-07-08): each result row pairs a time label with a "Book" button inside
// a small card. The direct search-page path must click the exact-time row.
func TestClickSearchResultsPage_ClicksExactTimeRow(t *testing.T) {
	html := `
		<!doctype html>
		<div class="results">
			<div class="row"><span>6:00 PM</span><button onclick="window.bookedSlot = '6:00 PM';">Book</button></div>
			<div class="row"><span>6:15 PM</span><button onclick="window.bookedSlot = '6:15 PM';">Book</button></div>
		</div>`
	searchURL := "data:text/html;charset=utf-8," + url.PathEscape(html)
	withTockDOMFixture(t, "<!doctype html><p>venue page</p>", func(ctx context.Context) {
		if err := clickSearchResultsPage(ctx, searchURL, "6:15 PM"); err != nil {
			t.Fatalf("clickSearchResultsPage: %v", err)
		}
		var bookedSlot string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot)); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if bookedSlot != "6:15 PM" {
			t.Fatalf("bookedSlot = %q, want 6:15 PM", bookedSlot)
		}
	})
}

// Mirrors the live 2026-07-09 Tock SMS interstitial. Material UI portals the
// role=dialog tree next to #Root, while the matching alert copy and action
// buttons are sibling sections inside the dialog. The old four-hop search
// stopped at the content section and never reached the Skip button.
func TestDismissPostConfirmDialog_ClicksPortalSMSDialogSkip(t *testing.T) {
	html := `
		<!doctype html>
		<div id="Root"><button>Complete reservation</button></div>
		<div class="MuiDialog-root" role="presentation">
			<div class="MuiDialog-container">
				<div role="dialog" aria-label="Enable text alerts from Tock" style="padding:20px">
					<div class="MuiDialogTitle-root">
						<h2>Enable text alerts from Tock</h2>
						<button aria-label="Close" onclick="window.clickedControl = 'close'">×</button>
					</div>
					<div id="sms-confirmation-dialog-content" data-testid="sms-confirmation-dialog-content">
						<div role="alert"><div><div>Stay in the know about your table</div><div>Receive text confirmation and updates for this and future bookings.</div></div></div>
					</div>
					<div class="MuiDialogActions-root">
						<button data-testid="sms-skip-button" onclick="window.clickedControl = 'skip'">Skip</button>
						<button data-testid="sms-agree-button" onclick="window.clickedControl = 'agree'">Agree and Continue</button>
					</div>
				</div>
			</div>
		</div>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		clicked, err := dismissPostConfirmDialog(ctx)
		if err != nil {
			t.Fatalf("dismissPostConfirmDialog: %v", err)
		}
		if !clicked {
			t.Fatal("expected Tock SMS dialog to be dismissed")
		}
		var control string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedControl || ''`, &control)); err != nil {
			t.Fatalf("read clicked control: %v", err)
		}
		if control != "skip" {
			t.Fatalf("clicked control = %q, want skip", control)
		}
	})
}

func TestDismissPostConfirmDialog_GenericFallbackSupportsRoleButton(t *testing.T) {
	html := `
		<!doctype html>
		<section class="custom-modal" style="padding:20px">
			<div><div><div><span>Receive text confirmation and updates for this and future bookings.</span></div></div></div>
			<footer>
				<div role="button" tabindex="0" style="display:inline-block;padding:10px" onclick="window.clickedControl = 'decline'">Not now</div>
				<button onclick="window.clickedControl = 'agree'">Agree and Continue</button>
			</footer>
		</section>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		clicked, err := dismissPostConfirmDialog(ctx)
		if err != nil {
			t.Fatalf("dismissPostConfirmDialog: %v", err)
		}
		if !clicked {
			t.Fatal("expected generic dialog fallback to find Not now")
		}
		var control string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedControl || ''`, &control)); err != nil {
			t.Fatalf("read clicked control: %v", err)
		}
		if control != "decline" {
			t.Fatalf("clicked control = %q, want decline", control)
		}
	})
}

func TestDismissPostConfirmDialog_NeverClicksAgreeOnlyControl(t *testing.T) {
	html := `
		<!doctype html>
		<div role="dialog" aria-label="Enable text alerts from Tock" style="padding:20px">
			<p>Stay in the know about your table</p>
			<p>Receive text confirmation and updates for this and future bookings.</p>
			<button onclick="window.clickedControl = 'agree'">Agree and Continue</button>
		</div>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		clicked, err := dismissPostConfirmDialog(ctx)
		if err != nil {
			t.Fatalf("dismissPostConfirmDialog: %v", err)
		}
		if clicked {
			t.Fatal("agree-only SMS dialog must not be clicked")
		}
		var control string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedControl || ''`, &control)); err != nil {
			t.Fatalf("read clicked control: %v", err)
		}
		if control != "" {
			t.Fatalf("unexpected clicked control %q", control)
		}
	})
}
