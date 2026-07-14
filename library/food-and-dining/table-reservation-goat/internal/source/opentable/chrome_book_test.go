package opentable

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestOpenTableChromeBookPageURL(t *testing.T) {
	tests := []struct {
		name string
		id   int
		slug string
		want string
	}{
		{
			name: "slug route",
			slug: "le-bernardin-new-york",
			want: "https://www.opentable.com/r/le-bernardin-new-york?covers=2&dateTime=2026-07-20T19:00",
		},
		{
			name: "numeric slug",
			slug: "3688",
			want: "https://www.opentable.com/restaurant/profile/3688?covers=2&dateTime=2026-07-20T19:00",
		},
		{
			name: "numeric id",
			id:   100,
			slug: "ignored-when-id-is-known",
			want: "https://www.opentable.com/r/ignored-when-id-is-known?covers=2&dateTime=2026-07-20T19:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openTableChromeBookPageURL(tt.id, tt.slug, 2, "2026-07-20", "19:00")
			if got != tt.want {
				t.Fatalf("openTableChromeBookPageURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateChromeBookRequest(t *testing.T) {
	valid := ChromeBookRequest{RestaurantSlug: "example", ReservationDateTime: "2026-07-20T19:00", PartySize: 2}
	date, hhmm, err := validateChromeBookRequest(valid)
	if err != nil || date != "2026-07-20" || hhmm != "19:00" {
		t.Fatalf("valid request = (%q, %q, %v)", date, hhmm, err)
	}
	for name, req := range map[string]ChromeBookRequest{
		"restaurant": {ReservationDateTime: "2026-07-20T19:00", PartySize: 2},
		"party":      {RestaurantSlug: "example", ReservationDateTime: "2026-07-20T19:00"},
		"datetime":   {RestaurantSlug: "example", ReservationDateTime: "2026-07-20 19:00", PartySize: 2},
		"date":       {RestaurantSlug: "example", ReservationDateTime: "2026-99-20T19:00", PartySize: 2},
		"time":       {RestaurantSlug: "example", ReservationDateTime: "2026-07-20T99:00", PartySize: 2},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := validateChromeBookRequest(req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestOpenTableChromeBookTypedErrors(t *testing.T) {
	typed := []error{ErrAttachUnreachable, ErrNotSignedIn, ErrSelectorDrift, ErrFormValidation, ErrIncompleteConfirmation, ErrSlotTaken}
	for _, kind := range typed {
		err := &ChromeBookError{Kind: kind, Step: "test", PageState: `{"path":"/booking/details"}`}
		if !errors.Is(err, kind) {
			t.Fatalf("errors.Is(%v, %v) = false", err, kind)
		}
		if !strings.Contains(err.Error(), "page_state=") {
			t.Fatalf("typed error omitted page_state: %v", err)
		}
		for _, other := range typed {
			if other != kind && errors.Is(err, other) {
				t.Fatalf("%v unexpectedly matched %v", kind, other)
			}
		}
	}
}

func TestOpenTableFormValidationErrorNamesRedactedRequiredControls(t *testing.T) {
	err := newOpenTableFormValidationError("confirmation_result", `{"path":"/booking/details"}`, []string{
		"I agree to this restaurant's terms",
		"Contact person@example.com at (919) 555-0100",
	})
	if !errors.Is(err, ErrFormValidation) {
		t.Fatalf("error = %v, want ErrFormValidation", err)
	}
	text := err.Error()
	if !strings.Contains(text, "Required agreement") || !strings.Contains(text, "Account identity") {
		t.Fatalf("typed error omitted redacted required labels: %s", text)
	}
	for _, forbidden := range []string{"person@example.com", "555-0100"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("typed error leaked %q: %s", forbidden, text)
		}
	}
}

func TestVenueSignedOutRetryIsBoundedToOneRefresh(t *testing.T) {
	if openTableVenueRefreshLimit != 1 {
		t.Fatalf("openTableVenueRefreshLimit = %d, want exactly one bounded refresh", openTableVenueRefreshLimit)
	}
}

func TestFinalConfirmPatternDoesNotMatchIntermediateControls(t *testing.T) {
	re := regexp.MustCompile(`(?i)` + openTableFinalLabelPattern)
	for _, label := range []string{"Complete reservation", "Confirm reservation", "Make my reservation", "Reserve now"} {
		if !re.MatchString(label) {
			t.Errorf("final pattern did not match %q", label)
		}
	}
	for _, label := range []string{"Continue", "Standard", "Select", "Book a table", "Sign in"} {
		if re.MatchString(label) {
			t.Errorf("final pattern must not match intermediate control %q", label)
		}
	}
}

func TestRequiredAgreementPatternExcludesOptionalMarketing(t *testing.T) {
	agreement := regexp.MustCompile(`(?i)` + openTableRequiredAgreementPattern)
	marketing := regexp.MustCompile(`(?i)` + openTableOptionalMarketingPattern)
	for _, label := range []string{
		"I agree to the restaurant’s terms and conditions",
		"Acknowledge this venue's cancellation policy",
		"Accept the restaurant terms",
	} {
		if !agreement.MatchString(label) || marketing.MatchString(label) {
			t.Errorf("required agreement label was not selected: %q", label)
		}
	}
	for _, label := range []string{
		"Sign me up to receive dining offers and news from this restaurant by email.",
		"Email me promotions and newsletter updates",
		"Send me text message notifications",
	} {
		if agreement.MatchString(label) && !marketing.MatchString(label) {
			t.Errorf("optional marketing label could be selected: %q", label)
		}
	}
}

func TestSlotSelectorCandidatesExcludeFinalConfirmation(t *testing.T) {
	joined := strings.ToLower(strings.Join(openTableSlotSelectorCandidates, " "))
	for _, forbidden := range []string{"complete", "confirm", "make-reservation"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("slot selectors contain final-confirmation term %q: %s", forbidden, joined)
		}
	}
}

func TestPageStateSnapshotRedactsURLQuery(t *testing.T) {
	state := openTablePageState{
		Path:                 "https://www.opentable.com/booking/details?slotLockId=secret&availabilityToken=secret#fragment",
		SignedIn:             true,
		FinalConfirmPresent:  true,
		FinalConfirmEnabled:  true,
		VisibleControlLabels: []string{"Complete reservation"},
	}
	raw := mustMarshalPageState(state)
	if strings.Contains(raw, "slotLockId") || strings.Contains(raw, "availabilityToken") || strings.Contains(raw, "secret") {
		t.Fatalf("page-state snapshot leaked URL query: %s", raw)
	}
	if !strings.Contains(raw, `"path":"/booking/details"`) {
		t.Fatalf("page-state snapshot lost sanitized path: %s", raw)
	}
}

func TestPageStateSnapshotRedactsAccountIdentity(t *testing.T) {
	state := openTablePageState{
		Path:                           "/booking/details",
		VisibleControlLabels:           []string{"Not Test Person?", "person@example.com", "Complete reservation"},
		RequiredCheckboxLabels:         []string{"Agree to terms", "person@example.com"},
		UncheckedRequiredControlLabels: []string{"Agree to terms", "(919) 555-0100"},
	}
	normalized := normalizeOpenTablePageState(state)
	if !normalized.SignedIn {
		t.Fatal("account-switch control on booking details should preserve signed-in evidence")
	}
	raw := mustMarshalPageState(normalized)
	for _, forbidden := range []string{"Test Person", "person@example.com", "555-0100"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("page-state snapshot leaked account identity %q: %s", forbidden, raw)
		}
	}
	for _, want := range []string{"Account switch", "Account identity", "Complete reservation", "Required agreement", "Personal contact"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("page-state snapshot missing %q: %s", want, raw)
		}
	}
}

func TestPageStateSnapshotAllowListsVisibleControlLabels(t *testing.T) {
	raw := mustMarshalPageState(openTablePageState{
		Path:                   "/booking/details",
		VisibleControlLabels:   []string{"Jane Smith", "Complete reservation", "Patio"},
		RequiredCheckboxLabels: []string{"Jane Smith", "Agree to terms"},
	})
	if strings.Contains(raw, "Jane Smith") {
		t.Fatalf("page-state snapshot leaked arbitrary signed-in label: %s", raw)
	}
	for _, want := range []string{"Other control", "Complete reservation", "Patio", "Required checkbox", "Required agreement"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("page-state snapshot missing %q: %s", want, raw)
		}
	}
}

func TestValidateOpenTableConfirmationRejectsIdentifierlessSuccess(t *testing.T) {
	err := validateOpenTableConfirmation(chromeConfirmationState{Success: true}, `{"path":"/booking/confirmation"}`)
	if !errors.Is(err, ErrIncompleteConfirmation) {
		t.Fatalf("error = %v, want ErrIncompleteConfirmation", err)
	}
	for _, partial := range []chromeConfirmationState{
		{Success: true, ConfirmationNumber: 1234},
		{Success: true, ReservationID: 5678},
		{Success: true, SecurityToken: "opaque"},
		{Success: true, ConfirmationNumber: 1234, ReservationID: 5678},
	} {
		if err := validateOpenTableConfirmation(partial, ""); !errors.Is(err, ErrIncompleteConfirmation) {
			t.Fatalf("partial confirmation %#v error = %v, want ErrIncompleteConfirmation", partial, err)
		}
	}
	complete := chromeConfirmationState{
		Success: true, ConfirmationNumber: 1234, ReservationID: 5678, SecurityToken: "opaque",
	}
	if err := validateOpenTableConfirmation(complete, ""); err != nil {
		t.Fatalf("complete confirmation rejected: %v", err)
	}
}

func TestChromeAttachConfiguredRequiresExplicitEnv(t *testing.T) {
	t.Setenv(openTableChromeDebugEnv, "")
	if ChromeAttachConfigured() {
		t.Fatal("empty env must not enable attach booking")
	}
	t.Setenv(openTableChromeDebugEnv, "http://127.0.0.1:9223")
	if !ChromeAttachConfigured() {
		t.Fatal("configured debug endpoint should enable attach booking")
	}
}
