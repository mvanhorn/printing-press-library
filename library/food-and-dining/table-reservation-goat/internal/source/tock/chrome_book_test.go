package tock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	cdproto "github.com/chromedp/cdproto"
	"github.com/chromedp/chromedp"
	"github.com/dop251/goja"
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

func withTockDOMFixtureAtPath(t *testing.T, path, html string, run func(context.Context)) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
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
	timed, cancelTimed := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimed()
	if err := chromedp.Run(timed, chromedp.Navigate(srv.URL+path)); err != nil {
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

func TestClickComboboxExperienceLayout_MatchesRoleOptionLabelExactly(t *testing.T) {
	html := `
		<!doctype html>
		<button role="combobox" aria-label="Time" aria-haspopup="listbox"
			onclick="document.getElementById('times').style.display = 'block'">Choose a time</button>
		<div id="times" role="listbox" style="display:none">
			<button role="option" onclick="window.selectedTime = '6:00 PM'">6:00 PM</button>
			<button role="option" onclick="window.selectedTime = '6:15 PM'">6:15 PM</button>
		</div>
		<section class="experience-card">
			<h2>Reservation</h2>
			<a href="#" onclick="window.clickedExperience = 'standard'; return false;">Book now</a>
		</section>
		<button type="submit" onclick="window.submitClicked = true">Book now</button>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, 0); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var selectedTime, clickedExperience string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.selectedTime || ''`, &selectedTime),
			chromedp.Evaluate(`window.clickedExperience || ''`, &clickedExperience),
		); err != nil {
			t.Fatalf("read role-combobox fixture state: %v", err)
		}
		if selectedTime != "6:15 PM" {
			t.Fatalf("selected time = %q, want exact 6:15 PM option", selectedTime)
		}
		if clickedExperience != "standard" {
			t.Fatalf("clicked experience = %q, want standard", clickedExperience)
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
			`"time_combobox_present":true`,
			`"6:15 PM"`,
			`"Book now"`,
		} {
			if !strings.Contains(hint, want) {
				t.Fatalf("page-state hint missing %q: %s", want, hint)
			}
		}
	})
}

func TestClickRequestedTockBookingControl_TypedFallbackIsRedacted(t *testing.T) {
	html := `
		<!doctype html>
		<button aria-label="Profile for Alice Secret">Alice Secret</button>
		<a href="/account?access_token=super-secret">Manage Alice Secret</a>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		_, cancel, err := clickRequestedTockBookingControl(ctx, "", "6:15 PM", "2026-07-10", "18:15", 4, 0)
		if cancel != nil {
			defer cancel()
		}
		if err == nil {
			t.Fatal("expected typed selector-drift error")
		}
		if !errors.Is(err, ErrSlotControlNotFound) {
			t.Fatalf("errors.Is(%v, ErrSlotControlNotFound) = false", err)
		}
		var chromeErr *ChromeBookError
		if !errors.As(err, &chromeErr) {
			t.Fatalf("error type = %T, want *ChromeBookError", err)
		}
		if chromeErr.Step != "booking_control" {
			t.Fatalf("step = %q, want booking_control", chromeErr.Step)
		}
		for _, forbidden := range []string{"Alice", "super-secret", "access_token", "?"} {
			if strings.Contains(chromeErr.PageState, forbidden) || strings.Contains(err.Error(), forbidden) {
				t.Fatalf("typed error leaked %q: %s", forbidden, err)
			}
		}
		var pageState struct {
			Path                 string   `json:"path"`
			VisibleControlLabels []string `json:"visible_control_labels"`
		}
		if err := json.Unmarshal([]byte(chromeErr.PageState), &pageState); err != nil {
			t.Fatalf("page state is not JSON: %v: %s", err, chromeErr.PageState)
		}
		if pageState.Path != "<non-http-page>" {
			t.Fatalf("page-state path = %q, want <non-http-page>", pageState.Path)
		}
		if len(pageState.VisibleControlLabels) != 1 || pageState.VisibleControlLabels[0] != "Other control" {
			t.Fatalf("visible_control_labels = %v, want [Other control]", pageState.VisibleControlLabels)
		}
	})
}

func TestSanitizeTockPathDropsQueryAndFragment(t *testing.T) {
	got := sanitizeTockPath("https://www.exploretock.com/venue/search?access_token=secret#results")
	if got != "/venue/search" {
		t.Fatalf("sanitizeTockPath = %q, want /venue/search", got)
	}
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

// Regression: when the confirm click succeeds, the page navigates to the
// receipt while the dialog probe's Evaluate is still in flight; chromedp then
// surfaces a destroyed-execution-context CDP error. That error must be
// classified as transient so a completed booking is not reported as failed
// (double-booking hazard).
func TestIsTransientNavigationError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			"cdp cannot find context",
			&cdproto.Error{Code: -32000, Message: "Cannot find context with specified id"},
			true,
		},
		{
			"cdp execution context destroyed",
			&cdproto.Error{Code: -32000, Message: "Execution context was destroyed."},
			true,
		},
		{
			"cdp target navigated",
			&cdproto.Error{Code: -32000, Message: "Inspected target navigated or closed"},
			true,
		},
		{
			"wrapped cdp error",
			fmt.Errorf("dismissing post-confirm dialog: %w",
				&cdproto.Error{Code: -32000, Message: "Cannot find context with specified id"}),
			true,
		},
		{
			// cdproto.Error flattened to a plain string (e.g. by %v or an
			// intermediate layer) — the classifier must still recognize it.
			"string form destroyed context",
			errors.New("encountered exception: Execution context was destroyed (-32000)"),
			true,
		},
		{
			"string form cannot find context",
			fmt.Errorf("probe: %s", "Cannot find context with specified id (-32000)"),
			true,
		},
		{
			"wrong cdp code",
			&cdproto.Error{Code: -32602, Message: "Cannot find context with specified id"},
			false,
		},
		{
			"plain network error",
			errors.New("dial tcp 127.0.0.1:9222: connect: connection refused"),
			false,
		},
		{
			"-32000 with unrelated message",
			&cdproto.Error{Code: -32000, Message: "Could not compute box model"},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientNavigationError(tc.err); got != tc.want {
				t.Fatalf("isTransientNavigationError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// Regression for the full loop: the dialog probe fails with a transient
// destroyed-context error while the page is already navigating to the
// receipt. The loop must keep polling and return the receipt URL rather
// than abort a booking that succeeded.
func TestWaitForReceiptThroughDialogs_TransientDismissErrorDoesNotAbort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if strings.HasPrefix(r.URL.Path, "/receipt") {
			fmt.Fprint(w, `<!doctype html><p>Reservation confirmed</p>`)
			return
		}
		// Checkout page that navigates to the receipt shortly after load,
		// mirroring the confirm click winning the race with the dialog probe.
		fmt.Fprint(w, `<!doctype html><p>checkout</p>
			<script>setTimeout(() => { location.href = '/receipt?purchaseId=1'; }, 300);</script>`)
	}))
	defer srv.Close()

	withTockDOMFixture(t, "<!doctype html><p>boot</p>", func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.Navigate(srv.URL+"/checkout/confirm-purchase")); err != nil {
			t.Fatalf("navigate to checkout fixture: %v", err)
		}
		dismissCalls := 0
		dismiss := func(context.Context) (bool, error) {
			dismissCalls++
			return false, &cdproto.Error{Code: -32000, Message: "Execution context was destroyed."}
		}
		var loc string
		err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			var werr error
			loc, werr = waitForReceiptThroughDialogsWith(actCtx, 8*time.Second, dismiss)
			return werr
		}))
		if err != nil {
			t.Fatalf("waitForReceiptThroughDialogsWith: %v", err)
		}
		if !strings.Contains(loc, "/receipt") {
			t.Fatalf("returned location = %q, want receipt URL", loc)
		}
		if dismissCalls == 0 {
			t.Fatal("expected the failing dialog probe to have been invoked")
		}
	})
}

func TestWaitForReceiptThroughDialogs_NonTransientDismissErrorAborts(t *testing.T) {
	withTockDOMFixture(t, "<!doctype html><p>checkout</p>", func(ctx context.Context) {
		dismiss := func(context.Context) (bool, error) {
			return false, errors.New("dialog probe exploded")
		}
		err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			_, werr := waitForReceiptThroughDialogsWith(actCtx, 5*time.Second, dismiss)
			return werr
		}))
		if err == nil {
			t.Fatal("expected non-transient dismiss error to abort the wait")
		}
		if !strings.Contains(err.Error(), "dismissing post-confirm dialog") {
			t.Fatalf("error = %v, want dismissing post-confirm dialog wrap", err)
		}
	})
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

// A pinned request may arrive at a venue-wide legacy page when Tock redirects
// its experience deep link. The first legacy probe matches by visible time
// only, so it is safe for a pinned request only while location.pathname still
// contains the exact /experience/<id> segment pair.
func TestClickRequestedTockBookingControl_LegacyPinnedExperiencePathGate(t *testing.T) {
	const experienceID = 520126
	const html = `
		<!doctype html>
		<button id="first" aria-label="Book now"
			onclick="window.clickedLegacy = 'first';">6:15 PM Book</button>
		<button id="second" aria-label="Book now"
			onclick="window.clickedLegacy = 'second';">6:15 PM Book</button>`

	cases := []struct {
		name            string
		pagePath        string
		experienceID    int
		wantClicked     string
		wantErrContains string
	}{
		{
			name:            "pinned redirect to venue-wide legacy page skips time-only click",
			pagePath:        "/venue",
			experienceID:    experienceID,
			wantErrContains: "legacy time-only slot path skipped for pinned experience 520126",
		},
		{
			name:            "pinned deep-link page with two untied slots fails closed",
			pagePath:        "/venue/experience/520126",
			experienceID:    experienceID,
			wantErrContains: "ambiguous",
		},
		{
			name:        "unpinned legacy behavior is unchanged",
			pagePath:    "/venue",
			wantClicked: "first",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, html)
			}))
			defer srv.Close()

			withTockDOMFixture(t, "<!doctype html><p>boot</p>", func(ctx context.Context) {
				if err := chromedp.Run(ctx, chromedp.Navigate(srv.URL+tc.pagePath)); err != nil {
					t.Fatalf("navigate to legacy fixture: %v", err)
				}
				activeCtx, activeCancel, err := clickRequestedTockBookingControl(
					ctx, "", "6:15 PM", "2026-07-10", "18:15", 2, tc.experienceID,
				)
				if activeCancel != nil {
					defer activeCancel()
				}
				if tc.wantErrContains == "" && err != nil {
					t.Fatalf("clickRequestedTockBookingControl: %v", err)
				}
				if tc.wantErrContains != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrContains)) {
					t.Fatalf("error = %v, want error containing %q", err, tc.wantErrContains)
				}

				var clicked string
				if err := chromedp.Run(activeCtx, chromedp.Evaluate(`window.clickedLegacy || ''`, &clicked)); err != nil {
					t.Fatalf("read legacy fixture state: %v", err)
				}
				if clicked != tc.wantClicked {
					t.Fatalf("clicked legacy control = %q, want %q", clicked, tc.wantClicked)
				}
			})
		})
	}
}

func TestClickPinnedSlotByTimeText_LegacySoleCandidateFormForeignOnlyFailsClosed(t *testing.T) {
	candidateTexts := []string{
		"6:15 PM Book",
		"6:15 PM",
	}
	for _, candidateText := range candidateTexts {
		t.Run(candidateText, func(t *testing.T) {
			html := fmt.Sprintf(`
				<!doctype html>
				<form id="foreign-only">
				  <a href="/venue/experience/111111">Other experience</a>
				  <button onclick="window.clickedForeign = true;">%s</button>
				</form>`, candidateText)
			withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
				err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126)
				if err == nil || !strings.Contains(err.Error(), "all requested-time legacy slot controls were tied only to other experiences") {
					t.Fatalf("error = %v, want foreign-only attribution failure", err)
				}
				var clickedForeign bool
				if evalErr := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedForeign || false`, &clickedForeign)); evalErr != nil {
					t.Fatalf("read legacy fixture state: %v", evalErr)
				}
				if clickedForeign {
					t.Fatal("foreign-only form candidate was clicked")
				}
			})
		})
	}
}

// Form exclusion must require sole candidate ownership: excluding several
// same-form candidates would thin the field down to whatever untied control
// remains outside the form, converting an ambiguous page into a wrong click.
func TestClickPinnedSlotByTimeText_MultiCandidateForeignFormStaysAmbiguous(t *testing.T) {
	const html = `
		<!doctype html>
		<form id="foreign-only">
		  <a href="/venue/experience/111111">Other experience</a>
		  <button onclick="window.clickedForm = 'first';">6:15 PM Book</button>
		  <button onclick="window.clickedForm = 'second';">6:15 PM Book</button>
		</form>
		<button onclick="window.clickedOutside = true;">6:15 PM Book</button>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("error = %v, want ambiguity failure", err)
		}
		var clickedForm string
		var clickedOutside bool
		if evalErr := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedForm || ''`, &clickedForm),
			chromedp.Evaluate(`window.clickedOutside || false`, &clickedOutside),
		); evalErr != nil {
			t.Fatalf("read legacy fixture state: %v", evalErr)
		}
		if clickedForm != "" || clickedOutside {
			t.Fatalf("clickedForm = %q clickedOutside = %v, want no clicks", clickedForm, clickedOutside)
		}
	})
}

func TestClickPinnedSlotByTimeText_LegacyFormForeignCandidateExcludedBeforeOwnSlot(t *testing.T) {
	const html = `
		<!doctype html>
		<form id="foreign-only">
		  <a href="/venue/experience/111111">Other experience</a>
		  <button onclick="window.clickedForeign = true;">6:15 PM Book</button>
		</form>
		<button onclick="window.clickedOwn = true;">6:15 PM Book</button>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedForeign, clickedOwn bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedForeign || false`, &clickedForeign),
			chromedp.Evaluate(`window.clickedOwn || false`, &clickedOwn),
		); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if clickedForeign || !clickedOwn {
			t.Fatalf("clicked foreign/own = %v/%v, want false/true", clickedForeign, clickedOwn)
		}
	})
}

func TestClickPinnedSlotByTimeText_LegacyFormPinnedLinkDoesNotVouch(t *testing.T) {
	const html = `
		<!doctype html>
		<form id="multi-experience">
		  <a href="/venue/experience/520126">Pinned details</a>
		  <a href="/venue/experience/111111">Other experience</a>
		  <button onclick="window.clickedForm = true;">6:15 PM Book</button>
		</form>
		<button onclick="window.clickedUntied = true;">6:15 PM Book</button>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("error = %v, want ambiguity failure", err)
		}
		var clickedForm, clickedUntied bool
		if evalErr := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedForm || false`, &clickedForm),
			chromedp.Evaluate(`window.clickedUntied || false`, &clickedUntied),
		); evalErr != nil {
			t.Fatalf("read legacy fixture state: %v", evalErr)
		}
		if clickedForm || clickedUntied {
			t.Fatalf("clicked form/untied = %v/%v, want false/false", clickedForm, clickedUntied)
		}
	})
}

func TestClickPinnedSlotByTimeText_ExplicitCardInsulatesFromForeignFormLink(t *testing.T) {
	const html = `
		<!doctype html>
		<form>
		  <a href="/venue/experience/111111">Cross-sell</a>
		  <section class="experience-card">
		    <button onclick="window.clickedOwn = true;">6:15 PM Book</button>
		  </section>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedOwn bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedOwn || false`, &clickedOwn)); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if !clickedOwn {
			t.Fatal("explicit-card candidate was not clicked")
		}
	})
}

func TestClickPinnedSlotByTimeText_ForeignCrossSellBeforeOwnSlot(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card" id="foreign">
			<a href="/venue/experience/111111">Other experience</a>
			<button onclick="window.clickedForeign = true;">6:15 PM Book</button>
		</section>
		<section class="experience-card" id="own">
			<button onclick="window.clickedOwn = true;">6:15 PM Book</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedForeign, clickedOwn bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedForeign || false`, &clickedForeign),
			chromedp.Evaluate(`window.clickedOwn || false`, &clickedOwn),
		); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if clickedForeign || !clickedOwn {
			t.Fatalf("clicked foreign/own = %v/%v, want false/true", clickedForeign, clickedOwn)
		}
	})
}

func TestClickPinnedSlotByTimeText_SoleUntiedPageOwnSlot(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card">
			<button onclick="window.clickedOwn = true;">6:15 PM Book</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedOwn bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedOwn || false`, &clickedOwn)); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if !clickedOwn {
			t.Fatal("sole untied page-own slot was not clicked")
		}
	})
}

func TestClickPinnedSlotByTimeText_MixedPassAmbiguityFailsClosed(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card">
			<button onclick="window.clickedFirst = true;">6:15 PM Book</button>
		</section>
		<section class="experience-card">
			<button onclick="window.clickedSecond = true;">6:15 PM</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("error = %v, want ambiguity failure", err)
		}
		var clickedFirst, clickedSecond bool
		if evalErr := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedFirst || false`, &clickedFirst),
			chromedp.Evaluate(`window.clickedSecond || false`, &clickedSecond),
		); evalErr != nil {
			t.Fatalf("read legacy fixture state: %v", evalErr)
		}
		if clickedFirst || clickedSecond {
			t.Fatalf("clicked first/second = %v/%v, want false/false", clickedFirst, clickedSecond)
		}
	})
}

func TestClickPinnedSlotByTimeText_ExactTimeSecondPassForeignExclusion(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card" id="foreign">
			<a href="/venue/experience/111111">Other experience</a>
			<button onclick="window.clickedForeign = true;">6:15 PM</button>
		</section>
		<section class="experience-card" id="own">
			<button onclick="window.clickedOwn = true;">6:15 PM</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedForeign, clickedOwn bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedForeign || false`, &clickedForeign),
			chromedp.Evaluate(`window.clickedOwn || false`, &clickedOwn),
		); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if clickedForeign || !clickedOwn {
			t.Fatalf("clicked foreign/own = %v/%v, want false/true", clickedForeign, clickedOwn)
		}
	})
}

func TestClickPinnedSlotByTimeText_PinnedTieBeatsUntied(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card" id="untied">
			<button onclick="window.clickedUntied = true;">6:15 PM Book</button>
		</section>
		<section class="experience-card" id="tied">
			<a href="/venue/experience/520126" onclick="window.clickedTied = true; return false;">6:15 PM Book</a>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickPinnedSlotByTimeText(ctx, "6:15 PM", 520126); err != nil {
			t.Fatalf("clickPinnedSlotByTimeText: %v", err)
		}
		var clickedUntied, clickedTied bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedUntied || false`, &clickedUntied),
			chromedp.Evaluate(`window.clickedTied || false`, &clickedTied),
		); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if clickedUntied || !clickedTied {
			t.Fatalf("clicked untied/tied = %v/%v, want false/true", clickedUntied, clickedTied)
		}
	})
}

func TestClickPinnedSlotByTimeText_UnpinnedFirstMatchUnchanged(t *testing.T) {
	const html = `
		<!doctype html>
		<section class="experience-card" id="foreign">
			<a href="/venue/experience/111111">Other experience</a>
			<button onclick="window.clicked = 'foreign';">6:15 PM Book</button>
		</section>
		<section class="experience-card" id="own">
			<button onclick="window.clicked = 'own';">6:15 PM Book</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := clickSlotByTimeText(ctx, "6:15 PM"); err != nil {
			t.Fatalf("clickSlotByTimeText: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clicked || ''`, &clicked)); err != nil {
			t.Fatalf("read legacy fixture state: %v", err)
		}
		if clicked != "foreign" {
			t.Fatalf("clicked = %q, want historical first match foreign", clicked)
		}
	})
}

func TestClickPinnedSlotByTimeTextJS_AtomicBoundarySafeGate(t *testing.T) {
	cases := []struct {
		name            string
		pagePath        string
		wantPageMatches bool
		wantClicked     string
		wantQueryCalls  int
	}{
		{
			name:            "exact segment pair clicks",
			pagePath:        "/venue/experience/123",
			wantPageMatches: true,
			wantClicked:     "first",
			wantQueryCalls:  1,
		},
		{
			name:            "exact segment pair with suffix path clicks",
			pagePath:        "/venue/experience/123/reservation",
			wantPageMatches: true,
			wantClicked:     "first",
			wantQueryCalls:  1,
		},
		{name: "numeric prefix collision fails closed", pagePath: "/venue/experience/1234"},
		{name: "numeric suffix collision fails closed", pagePath: "/venue/experience/123-other"},
		{name: "non-segment substring fails closed", pagePath: "/venue/some-experience/123"},
		{name: "venue redirect fails closed", pagePath: "/venue"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := fmt.Sprintf(`
				var location = {href: 'https://www.exploretock.com%s', pathname: %q};
				var clicked = '';
				var queryCalls = 0;
				var controls = [{
					textContent: '6:15 PM Book',
					form: null,
					getAttribute: function() { return ''; },
					closest: function() { return null; },
					click: function() { clicked = 'first'; },
				}];
				var document = {
					querySelectorAll: function() { queryCalls++; return controls; },
				};
				var result = %s;
				JSON.stringify({
					page_matches: result.page_matches,
					clicked: result.clicked,
					clicked_control: clicked,
					query_calls: queryCalls,
				});
			`, tc.pagePath, tc.pagePath, clickPinnedSlotByTimeTextJS("6:15 PM", 123))
			value, err := goja.New().RunString(script)
			if err != nil {
				t.Fatalf("evaluate atomic pinned legacy click: %v", err)
			}
			var got struct {
				PageMatches    bool   `json:"page_matches"`
				Clicked        bool   `json:"clicked"`
				ClickedControl string `json:"clicked_control"`
				QueryCalls     int    `json:"query_calls"`
			}
			if err := json.Unmarshal([]byte(value.String()), &got); err != nil {
				t.Fatalf("decode atomic pinned legacy click result: %v", err)
			}
			if got.PageMatches != tc.wantPageMatches {
				t.Fatalf("page_matches = %v, want %v", got.PageMatches, tc.wantPageMatches)
			}
			if got.Clicked != (tc.wantClicked != "") {
				t.Fatalf("clicked = %v, want %v", got.Clicked, tc.wantClicked != "")
			}
			if got.ClickedControl != tc.wantClicked {
				t.Fatalf("clicked control = %q, want %q", got.ClickedControl, tc.wantClicked)
			}
			if got.QueryCalls != tc.wantQueryCalls {
				t.Fatalf("query calls = %d, want %d", got.QueryCalls, tc.wantQueryCalls)
			}
		})
	}
}

func TestTockBookingControlFailureCause_PreservesLegacySkip(t *testing.T) {
	legacyErr := errors.New("legacy time-only slot path skipped for pinned experience 123: current page is not its experience deep link")
	cause := tockBookingControlFailureCause(
		"6:15 PM",
		legacyErr,
		errors.New("search fallback skipped: experience-specific request"),
		errors.New("experience_card: pinned experience 123 could not be positively identified"),
	)
	combined := (&ChromeBookError{
		Kind:  ErrSlotControlNotFound,
		Step:  "booking_control",
		Cause: cause,
	}).Error()
	for _, want := range []string{
		`requested_time="6:15 PM"`,
		legacyErr.Error(),
		"search fallback skipped: experience-specific request",
		"experience_card: pinned experience 123 could not be positively identified",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("combined error = %q, want it to contain %q", combined, want)
		}
	}
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

// A pinned experience must not use the venue-wide /search flow, whose rows
// are matched by time only. Fall through to the experience-aware card scorer
// so another experience offered at the same time cannot be booked instead.
func TestClickComboboxExperienceLayout_PinnedExperienceSkipsSearchResultSlot(t *testing.T) {
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Desired reservation time">
			<option value="18:00">6:00 PM</option>
			<option value="18:15">6:15 PM</option>
		</select>
		<a id="search" href="/barcelona-wine-bar-raleigh/search?date=2026-07-10&size=2&time=18%3A15"
			onclick="window.searchClicked = true; document.getElementById('results').style.display = 'block'; return false;">Search</a>
		<section class="experience-card" id="other">
			<h2>Other experience</h2>
			<a href="/barcelona-wine-bar-raleigh/experience/111111/reservation"
				onclick="window.clickedExperience = 'other'; return false;">Book now</a>
		</section>
		<section class="experience-card" id="pinned">
			<h2>Pinned experience</h2>
			<a href="/barcelona-wine-bar-raleigh/experience/520126/reservation">Pinned experience details</a>
			<button onclick="window.clickedExperience = 'pinned';">Book now</button>
		</section>
		<div id="results" style="display:none">
			<div><span>6:15 PM</span><button onclick="window.bookedSlot = '6:15 PM';">Book</button></div>
		</div>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, 520126)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout pinned experience flow: %v", err)
		}
		var selected, clickedExperience, bookedSlot string
		var searchClicked bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('#time').value`, &selected),
			chromedp.Evaluate(`window.searchClicked || false`, &searchClicked),
			chromedp.Evaluate(`window.clickedExperience || ''`, &clickedExperience),
			chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot),
		); err != nil {
			t.Fatalf("read pinned-experience fixture state: %v", err)
		}
		if selected != "18:15" {
			t.Fatalf("selected time = %q, want 18:15", selected)
		}
		if searchClicked {
			t.Fatal("pinned experience must not use the venue-wide search path")
		}
		if bookedSlot != "" {
			t.Fatalf("bookedSlot = %q, want no venue-wide result row click", bookedSlot)
		}
		if clickedExperience != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clickedExperience)
		}
	})
}

func TestTockPinnedExperienceEligibilityJS_BoundarySafe(t *testing.T) {
	const experienceID = 520126
	cases := []struct {
		name        string
		controlHref string
		cardHrefs   []string
		pagePath    string
		want        bool
	}{
		{name: "exact control href", controlHref: "/venue/experience/520126", pagePath: "/venue", want: true},
		{name: "exact control href with suffix path", controlHref: "/venue/experience/520126/reservation", pagePath: "/venue", want: true},
		{name: "exact control href with query", controlHref: "/venue/experience/520126?date=2026-07-10", pagePath: "/venue", want: true},
		{name: "different control ID", controlHref: "/venue/experience/111111/reservation", pagePath: "/venue"},
		{name: "control ID prefix collision", controlHref: "/venue/experience/5201264/reservation", pagePath: "/venue"},
		{name: "control ID suffix collision", controlHref: "/venue/experience/520126-other/reservation", pagePath: "/venue"},
		{name: "control query only", controlHref: "/venue?next=/experience/520126", pagePath: "/venue"},
		{name: "exact link in card", cardHrefs: []string{"/venue/experience/520126/reservation"}, pagePath: "/venue", want: true},
		{name: "pinned link in card but no tight ancestor", controlHref: "", cardHrefs: nil, pagePath: "/venue"},
		{name: "card-link ID prefix collision", cardHrefs: []string{"/venue/experience/5201264/reservation"}, pagePath: "/venue"},
		{name: "exact deep-link page", pagePath: "/venue/experience/520126", want: true},
		{name: "deep-link page cross-sell card for another experience", cardHrefs: []string{"/venue/experience/111111"}, pagePath: "/venue/experience/520126"},
		{name: "deep-link page cross-sell control href for another experience", controlHref: "/venue/experience/111111/reservation", pagePath: "/venue/experience/520126"},
		{name: "deep-link page card tied to both pinned and another", cardHrefs: []string{"/venue/experience/111111", "/venue/experience/520126"}, pagePath: "/venue/experience/520126", want: true},
		{name: "deep-link page untied sibling card", cardHrefs: []string{"/venue/menu"}, pagePath: "/venue/experience/520126", want: true},
		{name: "deep-link page ID prefix collision", pagePath: "/venue/experience/5201264"},
		{name: "unrelated page path", pagePath: "/venue/some-experience/520126"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cardHrefs, err := json.Marshal(tc.cardHrefs)
			if err != nil {
				t.Fatalf("marshal card hrefs: %v", err)
			}
			script := fmt.Sprintf(`
				var experienceID = %d;
				var location = {href: 'https://www.exploretock.com%s', pathname: %q};
				function makeControl(href, cardHrefs) {
					const card = cardHrefs === null ? null : {
						querySelectorAll: () => (cardHrefs || []).map((cardHref) => ({
							getAttribute: (name) => name === 'href' ? cardHref : '',
						})),
					};
					return {
						getAttribute: (name) => name === 'href' ? href : '',
						closest: () => card,
					};
				}
				%s
				const control = makeControl(%q, %s);
				eligibleExperienceControls([control]).length === 1;
			`, experienceID, tc.pagePath, tc.pagePath, tockPinnedExperienceEligibilityJS, tc.controlHref, cardHrefs)
			value, err := goja.New().RunString(script)
			if err != nil {
				t.Fatalf("evaluate eligibility helper: %v", err)
			}
			if got := value.ToBoolean(); got != tc.want {
				t.Fatalf("eligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTockPinnedExperienceEligibilityJS_UnpinnedIsNoOp(t *testing.T) {
	script := `
		var experienceID = 0;
		var location = {href: 'https://www.exploretock.com/venue', pathname: '/venue'};
		function cardFor() { throw new Error('unpinned path inspected a card'); }
	` + tockPinnedExperienceEligibilityJS + `
		const controls = [{getAttribute: () => { throw new Error('unpinned path inspected a control'); }}];
		eligibleExperienceControls(controls) === controls;
	`
	value, err := goja.New().RunString(script)
	if err != nil {
		t.Fatalf("evaluate unpinned helper: %v", err)
	}
	if !value.ToBoolean() {
		t.Fatal("unpinned eligibility must return the original controls array")
	}

	generated := clickComboboxExperienceLayoutJS("6:15 PM", "2026-07-10", 8, 0)
	if got := strings.Count(generated, "eligibleExperienceControls("); got != 2 {
		t.Fatalf("eligibility gate definition/call count = %d, want 2 (definition + card pick; the submit fallback uses scoped-or-tied gating instead)", got)
	}
	if got := strings.Count(generated, "submitEligible"); got != 2 {
		t.Fatalf("submit fallback scoped-or-tied gate count = %d, want 2 (definition + filter)", got)
	}
}

// Pinned experience selection must fail closed unless the target ID is tied
// to a Book now control by its href, another link in its card, or the page's
// own deep-link path. Unpinned selection retains the existing heuristic path.
func TestClickComboboxExperienceLayout_PinnedExperienceRequiresPositiveTie(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Desired reservation time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="standard">
			<h2>Reservation</h2>
			<button onclick="window.clickedExperience = 'standard'; history.pushState({}, '', '/checkout/confirm-purchase');">Book now</button>
		</section>
		<section class="experience-card" id="group">
			<h2>Reservation: Groups 7-18</h2>
			<button onclick="window.clickedExperience = 'group'; history.pushState({}, '', '/checkout/confirm-purchase');">Book now</button>
		</section>`

	t.Run("unpinned page fails without a positively tied card", func(t *testing.T) {
		withTockDOMFixture(t, html, func(ctx context.Context) {
			err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID)
			if err == nil {
				t.Fatal("expected pinned experience without a positive tie to fail")
			}
			want := "experience_card: pinned experience 520126 could not be positively identified"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %q, want typed pinned-experience failure containing %q", err, want)
			}
			var clicked string
			if evalErr := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedExperience || ''`, &clicked)); evalErr != nil {
				t.Fatalf("read clicked experience: %v", evalErr)
			}
			if clicked != "" {
				t.Fatalf("clicked experience = %q, want no heuristic click", clicked)
			}
		})
	})

	t.Run("deep-link page fails closed on multiple untied controls", func(t *testing.T) {
		// Historical behavior admitted every href-less control on the pinned
		// deep-link page; that let the party-size scorer book a sibling
		// experience. Multiple untied candidates are now ambiguity: fail
		// closed, click nothing (single-untied and tied cases are covered by
		// the dedicated PinnedDeepLink tests).
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, html)
		}))
		defer srv.Close()

		withTockDOMFixture(t, "<!doctype html><p>boot</p>", func(ctx context.Context) {
			deepLink := fmt.Sprintf("%s/venue/experience/%d", srv.URL, experienceID)
			if err := chromedp.Run(ctx, chromedp.Navigate(deepLink)); err != nil {
				t.Fatalf("navigate to pinned experience fixture: %v", err)
			}
			err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID)
			if err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Fatalf("expected ambiguity fail-closed error on pinned page, got %v", err)
			}
			var clicked string
			if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedExperience || ''`, &clicked)); err != nil {
				t.Fatalf("read clicked experience: %v", err)
			}
			if clicked != "" {
				t.Fatalf("clicked experience = %q, want none on ambiguity", clicked)
			}
		})
	})

	t.Run("unpinned selection keeps party-size heuristics", func(t *testing.T) {
		// Served over HTTP (not a data: URL) so the fixture's pushState-to-
		// checkout works and the flow stops after the first card click.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, html)
		}))
		defer srv.Close()

		withTockDOMFixture(t, "<!doctype html><p>boot</p>", func(ctx context.Context) {
			if err := chromedp.Run(ctx, chromedp.Navigate(srv.URL+"/venue")); err != nil {
				t.Fatalf("navigate to unpinned fixture: %v", err)
			}
			if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 8, 0); err != nil {
				t.Fatalf("clickComboboxExperienceLayout unpinned flow: %v", err)
			}
			var clicked string
			if err := chromedp.Run(ctx, chromedp.Evaluate(`window.clickedExperience || ''`, &clicked)); err != nil {
				t.Fatalf("read clicked experience: %v", err)
			}
			if clicked != "group" {
				t.Fatalf("clicked experience = %q, want group for unpinned party of 8", clicked)
			}
		})
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

func TestClickComboboxExperienceLayout_PinnedModalScopesDayAndSlotAwayFromBackground(t *testing.T) {
	const experienceID = 520126
	t.Run("clicks only the revealed modal controls", func(t *testing.T) {
		html := `
			<!doctype html>
			<label for="time">Time</label>
			<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
			<section id="background">
				<button aria-label="2026-07-10" onclick="window.backgroundDay = true;">10</button>
				<div><span>6:15 PM</span><button onclick="window.backgroundSlot = true;">Book</button></div>
			</section>
			<section class="experience-card" id="pinned-card">
				<a href="/venue/experience/520126">Chef Counter details</a>
				<button onclick="document.getElementById('modal').style.display = 'block';">Book now</button>
			</section>
			<div id="modal" style="display:none">
				<button aria-label="2026-07-10" onclick="window.modalDay = true;">10</button>
				<div><span>6:15 PM</span><button onclick="window.modalSlot = true;">Book</button></div>
			</div>`
		withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
			if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID); err != nil {
				t.Fatalf("clickComboboxExperienceLayout: %v", err)
			}
			var backgroundDay, backgroundSlot, modalDay, modalSlot bool
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(`window.backgroundDay || false`, &backgroundDay),
				chromedp.Evaluate(`window.backgroundSlot || false`, &backgroundSlot),
				chromedp.Evaluate(`window.modalDay || false`, &modalDay),
				chromedp.Evaluate(`window.modalSlot || false`, &modalSlot),
			); err != nil {
				t.Fatalf("read fixture state: %v", err)
			}
			if backgroundDay || backgroundSlot || !modalDay || !modalSlot {
				t.Fatalf("background day/slot and modal day/slot = %v/%v and %v/%v, want false/false and true/true", backgroundDay, backgroundSlot, modalDay, modalSlot)
			}
		})
	})

	t.Run("slot diagnostics remain inside the proven root", func(t *testing.T) {
		html := `
			<!doctype html>
			<label for="time">Time</label>
			<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
			<div><span>5:45 PM</span><button onclick="window.backgroundSlot = true;">Book</button></div>
			<section class="experience-card">
				<a href="/venue/experience/520126">Chef Counter details</a>
				<button onclick="document.getElementById('panel').style.display = 'block';">Book now</button>
			</section>
			<div id="panel" style="display:none">
				<div><span>7:00 PM</span><button onclick="window.panelSlot = true;">Book</button></div>
			</div>`
		withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
			err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID)
			if err == nil {
				t.Fatal("expected unavailable scoped slot to fail")
			}
			if !strings.Contains(err.Error(), "experience_modal_slot: requested time not offered in slot list; visible: 7:00 PM") {
				t.Fatalf("error = %q, want only the panel's 7:00 PM diagnostic", err)
			}
			if strings.Contains(err.Error(), "5:45 PM") {
				t.Fatalf("error = %q, background time leaked into scoped diagnostics", err)
			}
			var backgroundSlot, panelSlot bool
			if runErr := chromedp.Run(ctx,
				chromedp.Evaluate(`window.backgroundSlot || false`, &backgroundSlot),
				chromedp.Evaluate(`window.panelSlot || false`, &panelSlot),
			); runErr != nil {
				t.Fatalf("read fixture state: %v", runErr)
			}
			if backgroundSlot || panelSlot {
				t.Fatalf("background/panel slot clicked = %v/%v, want false/false", backgroundSlot, panelSlot)
			}
		})
	})
}

func TestClickComboboxExperienceLayout_PinnedUnmarkedRevealedPanelRemainsSupported(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card">
			<a href="/venue/experience/520126">Chef Counter details</a>
			<button onclick="document.getElementById('follow-on').style.display = 'block';">Book now</button>
		</section>
		<div id="follow-on" style="display:none">
			<button aria-label="2026-07-10" onclick="window.pickedDay = true;">10</button>
			<div><span>6:15 PM</span><button onclick="window.bookedSlot = true;">Book</button></div>
		</div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var pickedDay, bookedSlot bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.pickedDay || false`, &pickedDay),
			chromedp.Evaluate(`window.bookedSlot || false`, &bookedSlot),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if !pickedDay || !bookedSlot {
			t.Fatalf("picked day/booked slot = %v/%v, want true/true", pickedDay, bookedSlot)
		}
	})
}

func TestClickComboboxExperienceLayout_PinnedAmbiguousFollowOnRootsFailClosed(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card">
			<a href="/venue/experience/520126">Chef Counter details</a>
			<button onclick="document.getElementById('first').style.display = 'block'; document.getElementById('second').style.display = 'block';">Book now</button>
		</section>
		<div id="first" style="display:none">
			<button aria-label="2026-07-10" onclick="window.clickedDay = 'first';">10</button>
			<div><span>6:15 PM</span><button onclick="window.clickedSlot = 'first';">Book</button></div>
		</div>
		<div id="second" style="display:none">
			<button aria-label="2026-07-10" onclick="window.clickedDay = 'second';">10</button>
			<div><span>6:15 PM</span><button onclick="window.clickedSlot = 'second';">Book</button></div>
		</div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID)
		if err == nil || !strings.Contains(err.Error(), "experience_modal_scope:") {
			t.Fatalf("expected experience_modal_scope failure, got %v", err)
		}
		if !strings.Contains(err.Error(), "2 newly revealed roots") {
			t.Fatalf("error = %q, want two-root ambiguity detail", err)
		}
		var clickedDay, clickedSlot string
		if runErr := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedDay || ''`, &clickedDay),
			chromedp.Evaluate(`window.clickedSlot || ''`, &clickedSlot),
		); runErr != nil {
			t.Fatalf("read fixture state: %v", runErr)
		}
		if clickedDay != "" || clickedSlot != "" {
			t.Fatalf("clicked day/slot = %q/%q, want neither", clickedDay, clickedSlot)
		}
	})
}

func TestClickComboboxExperienceLayout_PinnedPreexistingBackgroundDoesNotMaterializeModal(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<div><span>6:15 PM</span><button onclick="window.backgroundSlot = true;">Book</button></div>
		<section class="experience-card">
			<a href="/venue/experience/520126" onclick="window.selectedExperience = true; return false;">Book now</a>
			<button type="submit" onclick="window.fallbackSubmit = true;">Reserve</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var selectedExperience, fallbackSubmit, backgroundSlot bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.selectedExperience || false`, &selectedExperience),
			chromedp.Evaluate(`window.fallbackSubmit || false`, &fallbackSubmit),
			chromedp.Evaluate(`window.backgroundSlot || false`, &backgroundSlot),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if !selectedExperience || !fallbackSubmit || backgroundSlot {
			t.Fatalf("selected/fallback/background = %v/%v/%v, want true/true/false", selectedExperience, fallbackSubmit, backgroundSlot)
		}
	})
}

func TestClickComboboxExperienceLayout_PinnedDeepLinkSoleStaticSlotRemainsSupported(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card" id="foreign">
			<a href="/venue/experience/111111">Patio details</a>
			<div><span>6:15 PM</span><button onclick="window.bookedSlot = 'foreign';">Book</button></div>
		</section>
		<section class="experience-card" id="own">
			<button onclick="window.selectedExperience = true;">Book now</button>
			<div><span>6:15 PM</span><button onclick="window.bookedSlot = 'own';">Book</button></div>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var selectedExperience bool
		var bookedSlot string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.selectedExperience || false`, &selectedExperience),
			chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if !selectedExperience || bookedSlot != "own" {
			t.Fatalf("selected/booked slot = %v/%q, want true/own", selectedExperience, bookedSlot)
		}
	})
}

// The static deep-link exception is sole-candidate only: two pre-existing
// untied slot rows are unattributable and must fail closed as a scope error,
// not fall through to any other click path.
func TestClickComboboxExperienceLayout_PinnedDeepLinkMultipleStaticSlotsFailClosed(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card" id="own">
			<button onclick="window.selectedExperience = true;">Book now</button>
		</section>
		<div><span>6:15 PM</span><button onclick="window.bookedSlot = 'first';">Book</button></div>
		<div><span>6:15 PM</span><button onclick="window.bookedSlot = 'second';">Book</button></div>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID)
		if err == nil || !strings.Contains(err.Error(), "surviving pre-existing date/slot controls") {
			t.Fatalf("error = %v, want static-scope ambiguity failure", err)
		}
		var bookedSlot string
		if evalErr := chromedp.Run(ctx, chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot)); evalErr != nil {
			t.Fatalf("read fixture state: %v", evalErr)
		}
		if bookedSlot != "" {
			t.Fatalf("booked slot = %q, want none for unattributable static rows", bookedSlot)
		}
	})
}

// A follow-on root can be the revealed control itself: unhiding a sole Book
// button beneath a pre-existing visible time row must stay bookable, since
// rooted queries would otherwise never see their own root.
func TestClickComboboxExperienceLayout_PinnedRevealedControlAsRootRemainsBookable(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card" id="own">
			<a href="/venue/experience/520126">Chef Counter</a>
			<button onclick="document.getElementById('slot-btn').style.display = 'inline-block';">Book now</button>
		</section>
		<div><span>6:15 PM</span><button id="slot-btn" style="display:none" onclick="window.bookedSlot = 'revealed';">Book</button></div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, experienceID); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var bookedSlot string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot)); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if bookedSlot != "revealed" {
			t.Fatalf("booked slot = %q, want the revealed sole control", bookedSlot)
		}
	})
}

func TestClickComboboxExperienceLayout_UnpinnedPostSelectionQueriesRemainDocumentWide(t *testing.T) {
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time"><option value="18:15">6:15 PM</option></select>
		<section class="experience-card">
			<button onclick="document.getElementById('first').style.display = 'block'; document.getElementById('second').style.display = 'block';">Book now</button>
		</section>
		<div id="first" style="display:none"><span>6:15 PM</span><button onclick="window.bookedSlot = 'first';">Book</button></div>
		<div id="second" style="display:none"><span>6:15 PM</span><button onclick="window.bookedSlot = 'second';">Book</button></div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := clickComboboxExperienceLayout(ctx, "6:15 PM", "2026-07-10", 2, 0); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var bookedSlot string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.bookedSlot || ''`, &bookedSlot)); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if bookedSlot != "first" {
			t.Fatalf("booked slot = %q, want historical document-order first", bookedSlot)
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

// A type=submit fallback must not book another experience's form: when the
// wrong experience's submit is top-most on the page, a pinned request must
// still submit only within the selected card's scope.
func TestClickComboboxExperienceLayout_PinnedSubmitFallbackScopedToSelectedCard(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="other-card">
			<h2>Patio Tasting</h2>
			<form id="other-form">
				<button type="submit" onclick="window.submittedForm = 'other'; return false;">Reserve</button>
			</form>
		</section>
		<section class="experience-card" id="pinned-card">
			<h2>Chef Counter</h2>
			<a href="/experience/520126">details</a>
			<a href="#" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			<form id="pinned-form">
				<button type="submit" onclick="window.submittedForm = 'pinned'; return false;">Reserve</button>
			</form>
		</section>`
	withTockDOMFixture(t, html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "pinned" {
			t.Fatalf("submitted form = %q, want pinned (the top-most foreign submit must not be clicked)", submitted)
		}
	})
}

// A page-level form is not sufficient scope when it spans explicit experience
// cards and owns more than one possible fallback submit.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsSharedFormForeignSubmit(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="shared-form">
			<section class="experience-card" id="foreign-card">
				<h2>Patio Tasting</h2>
				<button type="submit" onclick="window.submittedForms = (window.submittedForms || []).concat('foreign'); return false;">Reserve</button>
			</section>
			<button type="submit" onclick="window.submittedForms = (window.submittedForms || []).concat('shared'); return false;">Continue</button>
			<section class="experience-card" id="pinned-card">
				<h2>Chef Counter</h2>
				<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
				<button type="submit" onclick="window.submittedForms = (window.submittedForms || []).concat('pinned'); return false;">Reserve</button>
			</section>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		var submitted []string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForms || []`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if len(submitted) != 1 || submitted[0] != "pinned" {
			t.Fatalf("submitted forms = %v, want only pinned", submitted)
		}
	})
}

// A unique same-form fallback still cannot cross from the selected explicit
// card into a different explicit experience card.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsSoleFallbackAcrossExplicitCard(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="shared-form">
			<section class="experience-card" id="foreign-card">
				<h2>Patio Tasting</h2>
				<button type="submit" onclick="window.submittedForm = 'foreign'; return false;">Reserve</button>
			</section>
			<section class="experience-card" id="pinned-card">
				<h2>Chef Counter</h2>
				<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			</section>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none across explicit cards", submitted)
		}
	})
}

// Containment by the selected explicit card cannot vouch for a submit whose
// nearest explicit card is a nested, foreign experience card.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsNestedForeignCardSubmit(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form>
			<section class="experience-card" id="selected">
				<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
				<section class="experience-card" id="foreign">
					<button type="submit" onclick="window.submittedForm = 'foreign'; return false;">Reserve foreign</button>
				</section>
			</section>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none from nested foreign card", submitted)
		}
	})
}

// A second form-owned Book now anchor proves that a form spans more than one
// offering, even though anchors are not fallback submit candidates.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsFormWithSiblingBookNowAnchor(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="shared-form">
			<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			<a href="#other" onclick="window.clickedExperience = 'sibling'; return false;">Book now</a>
			<button type="submit" onclick="window.submittedForm = 'shared'; return false;">Reserve</button>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none from multi-offering form", submitted)
		}
	})
}

// Native form= association makes an external submit form-owned, but it still
// cannot cross from the selected explicit card into a foreign explicit card.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsExternalFormAssociation(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="selected-form">
			<section class="experience-card" id="selected-card">
				<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			</section>
		</form>
		<section class="experience-card" id="foreign-card">
			<button type="submit" form="selected-form" onclick="window.submittedForm = 'foreign'; return false;">Reserve foreign</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none across external form association", submitted)
		}
	})
}

// A foreign Book now button associated into the selected form via the form
// attribute proves the form spans more than one offering: honoring el.form
// must widen the cardinality scan and fail the fallback closed.
func TestClickComboboxExperienceLayout_PinnedSubmitRejectsFormAssociatedForeignBookNow(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="selected-form">
			<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			<button type="submit" onclick="window.submittedForm = 'inside'; return false;">Reserve</button>
		</form>
		<section id="foreign-section">
			<button form="selected-form" onclick="window.submittedForm = 'foreign'; return false;">Book now</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none with a form-associated foreign Book now", submitted)
		}
	})
}

// A form-only legacy layout remains valid when the pinned Book now control has
// exactly one separate fallback submit and no explicit card boundary exists.
func TestClickComboboxExperienceLayout_PinnedSubmitAllowsSoleLegacySameFormSubmit(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="legacy-form">
			<h2>Chef Counter</h2>
			<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			<button type="submit" onclick="window.submittedForm = 'pinned'; return false;">Reserve</button>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "pinned" {
			t.Fatalf("submitted form = %q, want pinned legacy fallback", submitted)
		}
	})
}

// An invalid shared form cannot positively re-tie its bare submits merely
// because the pinned link is elsewhere inside that same form.
func TestClickComboboxExperienceLayout_PinnedSubmitInvalidSharedFormCannotRetieThroughForm(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<form id="shared-form">
			<h2>Chef Counter</h2>
			<a href="/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			<button type="submit" onclick="window.submittedForms = (window.submittedForms || []).concat('first'); return false;">Reserve first</button>
			<button type="submit" onclick="window.submittedForms = (window.submittedForms || []).concat('second'); return false;">Reserve second</button>
		</form>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		var submitted []string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForms || []`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if len(submitted) != 0 {
			t.Fatalf("submitted forms = %v, want none through invalid shared form", submitted)
		}
	})
}

// On the pinned deep-link page, an untied sibling card (plain button with no
// experience href) must not silently compete with the page's own control:
// two unprovable candidates is ambiguity and must fail closed unclicked.
func TestClickComboboxExperienceLayout_PinnedDeepLinkAmbiguityFailsClosed(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="sibling">
			<h2>Reservation: Groups 2-4</h2>
			<button onclick="window.clickedExperience = 'sibling';">Book now</button>
		</section>
		<section class="experience-card" id="own">
			<h2>Chef Counter</h2>
			<button onclick="window.clickedExperience = 'own';">Book now</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		}))
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguity fail-closed error, got %v", err)
		}
		var clicked string
		if runErr := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
		); runErr != nil {
			t.Fatalf("read fixture state: %v", runErr)
		}
		if clicked != "" {
			t.Fatalf("clicked experience = %q, want none on ambiguity", clicked)
		}
	})
}

// A sole surviving untied control on the pinned deep-link page is the page's
// own CTA and must still book (foreign-linked cross-sell cards are excluded
// first, so they do not create ambiguity).
func TestClickComboboxExperienceLayout_PinnedDeepLinkSoleUntiedControlBooks(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="cross-sell">
			<h2>Patio Tasting</h2>
			<a href="/venue/experience/111111">details</a>
			<button onclick="window.clickedExperience = 'cross-sell';">Book now</button>
		</section>
		<section class="experience-card" id="own">
			<h2>Chef Counter</h2>
			<button onclick="window.clickedExperience = 'own';">Book now</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "own" {
			t.Fatalf("clicked experience = %q, want own", clicked)
		}
	})
}

// A control positively tied to the pinned experience beats untied siblings
// outright on the deep-link page — no ambiguity failure.
func TestClickComboboxExperienceLayout_PinnedDeepLinkTiedControlBeatsUntied(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<section class="experience-card" id="sibling">
			<h2>Reservation: Groups 2-4</h2>
			<button onclick="window.clickedExperience = 'sibling';">Book now</button>
		</section>
		<section class="experience-card" id="tied">
			<h2>Chef Counter</h2>
			<a href="/venue/experience/520126/reservation">details</a>
			<button onclick="window.clickedExperience = 'tied';">Book now</button>
		</section>`
	withTockDOMFixtureAtPath(t, "/venue/experience/520126", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "tied" {
			t.Fatalf("clicked experience = %q, want tied", clicked)
		}
	})
}

// cardFor()'s generic div fallback can climb to a wrapper spanning several
// experiences on flat unclassed markup. Containment in such an unproven
// boundary must not scope a foreign form's submit — with no positive tie it
// stays unclicked.
func TestClickComboboxExperienceLayout_PinnedSubmitIgnoresBroadWrapperContainment(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<div id="wrapper">
			<p>Chef Counter
				<a href="/venue/experience/520126" onclick="window.clickedExperience = 'pinned'; return false;">Book now</a>
			</p>
			<p>Patio Tasting</p>
			<form id="foreign-form">
				<button type="submit" onclick="window.submittedForm = 'foreign'; return false;">Reserve</button>
			</form>
		</div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 2, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked, submitted string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
			chromedp.Evaluate(`window.submittedForm || ''`, &submitted),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "pinned" {
			t.Fatalf("clicked experience = %q, want pinned", clicked)
		}
		if submitted != "" {
			t.Fatalf("submitted form = %q, want none (broad wrapper containment must not scope a foreign submit)", submitted)
		}
	})
}

// On a pinned venue page, a broad unclassed wrapper carrying the pinned link
// must not vouch for a foreign Book now control: only the control with its
// own positive tie is eligible, even when party-size scoring favors the
// sibling.
func TestClickComboboxExperienceLayout_PinnedTieRequiresTightBoundary(t *testing.T) {
	const experienceID = 520126
	html := `
		<!doctype html>
		<label for="time">Time</label>
		<select id="time" aria-label="Time">
			<option value="18:15">6:15 PM</option>
		</select>
		<div id="wrapper">
			<p>Chef Counter
				<a href="/venue/experience/520126" onclick="window.clickedExperience = 'own'; return false;">Book now</a>
			</p>
			<p>Patio Tasting: Groups 7-18
				<button onclick="window.clickedExperience = 'sibling';">Book now</button>
			</p>
		</div>`
	withTockDOMFixtureAtPath(t, "/venue", html, func(ctx context.Context) {
		if err := chromedp.Run(ctx, chromedp.ActionFunc(func(actCtx context.Context) error {
			return clickComboboxExperienceLayout(actCtx, "6:15 PM", "2026-07-10", 8, experienceID)
		})); err != nil {
			t.Fatalf("clickComboboxExperienceLayout: %v", err)
		}
		var clicked string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.clickedExperience || ''`, &clicked),
		); err != nil {
			t.Fatalf("read fixture state: %v", err)
		}
		if clicked != "own" {
			t.Fatalf("clicked experience = %q, want own (wrapper must not vouch for the sibling)", clicked)
		}
	})
}
