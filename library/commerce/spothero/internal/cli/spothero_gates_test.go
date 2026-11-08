package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/mcp/cobratree"
)

type fakeSpotHero struct {
	checkoutPosts, refundPosts int
	checkoutStatus             int
	refundBody                 map[string]any
	originalEligible           bool
	creditEligible             bool
	refunded, refundIgnored    bool
	checkoutBody               string
}

// start serves the account API and Craig from one server, with just enough shape for
// the flows under test. All data is fake.
func (f *fakeSpotHero) start(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := func(v string) { _, _ = w.Write([]byte(`{"data":` + v + `}`)) }
		switch {
		case r.URL.Path == "/search/transient/2175":
			_, _ = w.Write([]byte(`{"tracking":{"search_id":"s","action_id":"a"},"result":{"availability":{"available":true},"facility":{"common":{"title":"Fake Garage","status":"on_sales_allowed","requirements":{"license_plate":false}}},"rates":[{"transient":{"rate_type":"matrix_flex_rates"},"quote":{"meta":{"quote_token":"SECRET-QUOTE","quote_mac":"9","quote_valid_until":"2099-01-15T00:00:00Z"},"total_price":{"value":1800},"order":[{"rate_id":"9","starts":"2099-01-15T09:00:00-06:00","ends":"2099-01-15T12:00:00-06:00"}]}}]}}`))
		case r.URL.Path == "/users/me/":
			data(`{"id":42,"email":"user@example.com","phone_number":"+14155550100"}`)
		case r.URL.Path == "/users/42/credit-cards/":
			data(`{"results":[{"card_id":1,"card_external_id":"card-uuid","card_last4":"4242","is_default":true}]}`)
		case r.URL.Path == "/users/42/vehicles/":
			data(`{"results":[{"id":7,"is_default":true}]}`)
		case r.URL.Path == "/checkout/" && r.Method == http.MethodPost:
			f.checkoutPosts++
			if f.checkoutStatus != 0 {
				w.WriteHeader(f.checkoutStatus)
				return
			}
			if f.checkoutBody != "" {
				_, _ = w.Write([]byte(f.checkoutBody))
				return
			}
			data(`{"order_id":"o1","purchases":[{"rental_id":5}]}`)
		case strings.HasSuffix(r.URL.Path, "/refund-eligibility/"):
			data(`{"original_source_refund":{"is_eligible":` + boolStr(f.originalEligible) + `},"credit_only_refund":{"is_eligible":` + boolStr(f.creditEligible) + `},"cancellation_reasons":[{"id":1,"reason":"No longer need parking","reason_displayed_to_customer":"x"}]}`)
		case strings.HasSuffix(r.URL.Path, "/refund/") && r.Method == http.MethodPost:
			f.refundPosts++
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &f.refundBody)
			f.refunded = true
			if f.refundIgnored {
				f.refunded = false
			}
			data(`{"status":"cancelled"}`)
		case r.URL.Path == "/reservations/5/" || r.URL.Path == "/reservations/6/":
			status := "valid"
			if f.refunded {
				status = "cancelled"
			}
			data(`{"rental_id":5,"facility_id":2175,"reservation_status":"` + status + `","price":1800,"starts":"2099-01-15T09:00:00-06:00"}`)
		case r.URL.Path == "/reservations/":
			data(`{"results":[]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SPOTHERO_BASE_URL", srv.URL)
	t.Setenv("SPOTHERO_CRAIG_BASE_URL", srv.URL)
	t.Setenv("SPOTHERO_COOKIES", "AUTH_TOKEN=fixture")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

var placeBase = []string{"book", "place", "--facility-id", "2175", "--starts", "2099-01-15T09:00:00-06:00", "--ends", "2099-01-15T12:00:00-06:00"}

func args(a []string, more ...string) []string { return append(append([]string{}, a...), more...) }

var allGates = []string{"--enable-live-booking", "--owner-approved", "--confirm", bookConfirmPhrase}

// With no gates the command previews: it reports placed false and posts nothing.
func TestBookPlaceNoGatesPreviewsOnly(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, args(placeBase, "--json")...)
	if code != 0 || f.checkoutPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
	if !strings.Contains(out, `"placed": false`) && !strings.Contains(out, `"placed":false`) {
		t.Fatalf("no placed false: %s", out)
	}
	if strings.Contains(out, "SECRET-QUOTE") {
		t.Fatalf("preview leaked the quote token: %s", out)
	}
}

// Every partial or misspelled gate combination is a refusal, never a preview: someone
// who tried to book must not read success.
func TestBookPlaceRefusesPartialGates(t *testing.T) {
	cases := map[string][]string{
		"confirm only":      {"--confirm", bookConfirmPhrase},
		"no owner":          {"--enable-live-booking", "--confirm", bookConfirmPhrase},
		"no enable":         {"--owner-approved", "--confirm", bookConfirmPhrase},
		"no confirm":        {"--enable-live-booking", "--owner-approved"},
		"lowercase phrase":  {"--enable-live-booking", "--owner-approved", "--confirm", strings.ToLower(bookConfirmPhrase)},
		"yes is not a gate": {"--yes"},
	}
	for name, gate := range cases {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			f := &fakeSpotHero{}
			f.start(t)
			out, code := runCLI(t, args(placeBase, gate...)...)
			if name == "yes is not a gate" {
				// --yes alone leaves no gates set, so it previews; it must never post.
				if f.checkoutPosts != 0 {
					t.Fatalf("--yes posted a checkout: %s", out)
				}
				return
			}
			if code != 2 || f.checkoutPosts != 0 {
				t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
			}
		})
	}
}

func TestBookPlaceAllGatesPostsOnce(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, args(placeBase, args(allGates, "--json")...)...)
	if code != 0 || f.checkoutPosts != 1 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
}

func TestBookPlaceMaxTotalBlocksBeforePosting(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, args(placeBase, args(allGates, "--max-total", "17.99")...)...)
	if code != 2 || f.checkoutPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
	if _, code := runCLI(t, args(placeBase, args(allGates, "--max-total", "18.00")...)...); code != 0 || f.checkoutPosts != 1 {
		t.Fatalf("at the cap: exit %d, posts %d", code, f.checkoutPosts)
	}
}

func TestBookPlaceDryRunNeverPostsAndMasks(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, args(placeBase, args(allGates, "--dry-run", "--json")...)...)
	if code != 0 || f.checkoutPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
	for _, secret := range []string{"SECRET-QUOTE", "user@example.com", "+14155550100"} {
		if strings.Contains(out, secret) {
			t.Fatalf("dry run printed %s", secret)
		}
	}
}

// A second checkout could double charge, so the ambiguous path reads state and stops.
func TestBookPlaceAmbiguousReadsStateAndDoesNotRetry(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{checkoutStatus: http.StatusBadGateway}
	f.start(t)
	out, code := runCLI(t, args(placeBase, args(allGates, "--json")...)...)
	if code != 5 || f.checkoutPosts != 1 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
	if !strings.Contains(out, "unknown") || !strings.Contains(out, "reservation_lookup") {
		t.Fatalf("no state report: %s", out)
	}
}

// The live dogfood matrix sweeps every command. It must be unable to spend money even
// if a fixture ever carried the gates.
func TestHarnessCannotPlaceOrCancel(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{originalEligible: true}
	f.start(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	if out, _ := runCLI(t, args(placeBase, allGates...)...); f.checkoutPosts != 0 {
		t.Fatalf("checkout under harness: %s", out)
	}
	previewOut, code := runCLI(t, "cancel", "preview", "5", "--json")
	if code != 0 {
		t.Fatalf("preview exit %d: %s", code, previewOut)
	}
	if out, _ := runCLI(t, "cancel", "5", "--yes", "--confirm", tokenFrom(t, previewOut)); f.refundPosts != 0 {
		t.Fatalf("refund under harness: %s", out)
	}
}

func tokenFrom(t *testing.T, out string) string {
	t.Helper()
	var got struct {
		ConfirmToken string `json:"confirm_token"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.ConfirmToken == "" {
		t.Fatalf("no confirm token (%v): %s", err, out)
	}
	return got.ConfirmToken
}

// A tokenless --dry-run only prints "Not cancelled", so it must not depend on a session
// being parseable. The verify harness injects a placeholder cookie that fails parsing,
// which used to turn this harmless preview into exit 4.
func TestCancelDryRunWithoutTokenNeedsNoSession(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{originalEligible: true}
	f.start(t)
	t.Setenv("SPOTHERO_COOKIES", "mock-value")
	out, code := runCLI(t, "cancel", "5", "--dry-run", "--json")
	if code != 0 || f.refundPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.refundPosts, out)
	}
	if !strings.Contains(out, `"cancelled": false`) {
		t.Fatalf("must report not cancelled: %s", out)
	}
}

func TestCancelRequiresPreviewTokenAndYes(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{originalEligible: true}
	f.start(t)
	if out, code := runCLI(t, "cancel", "5"); code != 2 || f.refundPosts != 0 {
		t.Fatalf("no flags: exit %d: %s", code, out)
	}
	if out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", "made-up"); code != 2 || f.refundPosts != 0 {
		t.Fatalf("bad token: exit %d: %s", code, out)
	}
	previewOut, code := runCLI(t, "cancel", "preview", "5", "--json")
	if code != 0 || f.refundPosts != 0 {
		t.Fatalf("preview: exit %d, posts %d: %s", code, f.refundPosts, previewOut)
	}
	tok := tokenFrom(t, previewOut)
	if out, code := runCLI(t, "cancel", "5", "--confirm", tok); code != 2 || f.refundPosts != 0 {
		t.Fatalf("token without --yes: exit %d: %s", code, out)
	}
	if out, code := runCLI(t, "cancel", "6", "--yes", "--confirm", tok); code != 2 || f.refundPosts != 0 {
		t.Fatalf("token for another reservation: exit %d: %s", code, out)
	}
	if out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", tok, "--dry-run", "--json"); code != 0 || f.refundPosts != 0 {
		t.Fatalf("dry run: exit %d, posts %d: %s", code, f.refundPosts, out)
	}
	if out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", tok, "--json"); code != 0 || f.refundPosts != 1 {
		t.Fatalf("real: exit %d, posts %d: %s", code, f.refundPosts, out)
	}
	if f.refundBody["reason"] != "No longer need parking" || f.refundBody["refund_as_credit"] != false {
		t.Fatalf("refund body = %v", f.refundBody)
	}
	// Single use.
	if out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", tok); code != 2 || f.refundPosts != 1 {
		t.Fatalf("reused token: exit %d, posts %d: %s", code, f.refundPosts, out)
	}
}

func TestCancelPreviewRefusesNonRefundable(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	if out, code := runCLI(t, "cancel", "preview", "5"); code != 2 {
		t.Fatalf("exit %d: %s", code, out)
	}
}

// Credit-only reservations must be chosen explicitly: quietly switching the destination
// would change where the owner's money goes.
func TestCancelCreditOnlyRequiresExplicitFlagAndIsBound(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{creditEligible: true}
	f.start(t)
	if out, code := runCLI(t, "cancel", "preview", "5"); code != 2 {
		t.Fatalf("exit %d: %s", code, out)
	}
	previewOut, code := runCLI(t, "cancel", "preview", "5", "--refund-as-credit", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, previewOut)
	}
	if out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", tokenFrom(t, previewOut), "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if f.refundBody["refund_as_credit"] != true {
		t.Fatalf("binding lost: %v", f.refundBody)
	}
}

// MCP `confirm` is set by the model, which is not a human approval, so spending and
// refunding commands must not be tools at all.
func TestPlaceAndCancelAreNotMCPTools(t *testing.T) {
	testenv.Isolate(t)
	root := RootCmd()
	for _, path := range [][]string{{"book", "place"}, {"cancel"}} {
		c, _, err := root.Find(path)
		if err != nil || c.Annotations[cobratree.HiddenAnnotation] != "true" {
			t.Fatalf("%v: not hidden from MCP (err %v)", path, err)
		}
	}
	for _, path := range [][]string{{"book", "preview"}, {"find"}} {
		c, _, err := root.Find(path)
		if err != nil || c.Annotations[cobratree.HiddenAnnotation] == "true" {
			t.Fatalf("%v: should stay on MCP (err %v)", path, err)
		}
	}
}

// A 200 with nothing in it must not read as a charge that happened, or did not.
func TestBookPlaceEmptySuccessIsUnknown(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{checkoutBody: `{"data":{}}`}
	f.start(t)
	out, code := runCLI(t, args(placeBase, args(allGates, "--json")...)...)
	if code != 5 || f.checkoutPosts != 1 || !strings.Contains(out, "unknown") {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
}

// The refund response proves nothing, so only the reservation's status can say cancelled.
func TestCancelUnconfirmedIsUnknown(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{originalEligible: true, refundIgnored: true}
	f.start(t)
	previewOut, code := runCLI(t, "cancel", "preview", "5", "--json")
	if code != 0 {
		t.Fatalf("preview exit %d: %s", code, previewOut)
	}
	out, code := runCLI(t, "cancel", "5", "--yes", "--confirm", tokenFrom(t, previewOut), "--json")
	if code != 5 || !strings.Contains(out, "unknown") || f.refundPosts != 1 {
		t.Fatalf("exit %d, posts %d: %s", code, f.refundPosts, out)
	}
}

// --dry-run must show the masked request even with no gates, not just a preview.
func TestBookPlaceDryRunWithoutGatesShowsRequest(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, args(placeBase, "--dry-run", "--json")...)
	if code != 0 || f.checkoutPosts != 0 || !strings.Contains(out, "would_post") || strings.Contains(out, "SECRET-QUOTE") {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
}

// Saved profiles can hold flag values, so a hand-edited one could carry the gates.
// Approval has to be typed, so any live action under a profile is refused.
func TestProfileCannotSupplyApproval(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{originalEligible: true}
	f.start(t)
	if out, code := runCLI(t, "profile", "save", "saved", "--quiet"); code != 0 {
		t.Fatalf("profile save exit %d: %s", code, out)
	}
	out, code := runCLI(t, args(placeBase, args(allGates, "--profile", "saved")...)...)
	if code != 2 || f.checkoutPosts != 0 {
		t.Fatalf("book: exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
	previewOut, code := runCLI(t, "cancel", "preview", "5", "--json")
	if code != 0 {
		t.Fatalf("preview exit %d: %s", code, previewOut)
	}
	out, code = runCLI(t, "cancel", "5", "--yes", "--confirm", tokenFrom(t, previewOut), "--profile", "saved")
	if code != 2 || f.refundPosts != 0 {
		t.Fatalf("cancel: exit %d, posts %d: %s", code, f.refundPosts, out)
	}
}

func TestMaxTotalZeroRefused(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	if out, code := runCLI(t, args(placeBase, args(allGates, "--max-total", "0")...)...); code != 2 || f.checkoutPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
}

// A naive time with no known zone could charge the wrong window, so it is refused.
func TestBookPlaceNaiveTimeWithoutZoneRefused(t *testing.T) {
	testenv.Isolate(t)
	f := &fakeSpotHero{}
	f.start(t)
	out, code := runCLI(t, "book", "place", "--facility-id", "2175", "--starts", "2099-01-15T09:00", "--ends", "2099-01-15T12:00", "--enable-live-booking", "--owner-approved", "--confirm", bookConfirmPhrase)
	if code != 2 || f.checkoutPosts != 0 {
		t.Fatalf("exit %d, posts %d: %s", code, f.checkoutPosts, out)
	}
}

func TestParseDollars(t *testing.T) {
	for in, want := range map[string]int{"30": 3000, "30.1": 3010, "30.10": 3010, "$5.05": 505, "": 0} {
		if got, err := parseDollars(in); err != nil || got != want {
			t.Errorf("%q: %d %v", in, got, err)
		}
	}
	for _, bad := range []string{"abc", "1.234", "-5", "0", "0.00"} {
		if _, err := parseDollars(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
