package tock

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const runTockCancelChromeTestsEnv = "TRG_RUN_TOCK_CANCEL_CHROME_TESTS"

type cancelFixture struct {
	receiptHTML       string
	questionHTML      string
	receiptRedirect   string
	afterConfirmPath  string
	canceledAfterPost bool
	// failDispatchOnCall injects a mid-dispatch CDP failure on the Nth
	// trusted-click press/release (1-based); earlier calls use the real
	// dispatch. Zero disables the seam.
	failDispatchOnCall int32
	dispatchCalls      atomic.Int32
	firstClicks        atomic.Int32
	secondClicks       atomic.Int32
	canceled           atomic.Bool
	receiptRedirected  atomic.Bool
}

func requireTockCancelChrome(t *testing.T) {
	t.Helper()
	if os.Getenv(runTockCancelChromeTestsEnv) != "1" {
		t.Skipf("host-gated real Chrome fixture; set %s=1", runTockCancelChromeTestsEnv)
	}
	t.Setenv("TABLE_RESERVATION_GOAT_TOCK_CHROME_DEBUG_URL", "http://127.0.0.1:1")
	t.Setenv("TABLE_RESERVATION_GOAT_TOCK_CHROME_HEADLESS", "new")
}

func TestCancelObservationReadiness(t *testing.T) {
	if initialReceiptReady(cancelState{}) {
		t.Fatal("empty receipt state must not be ready")
	}
	if !initialReceiptReady(cancelState{CancelCandidateCount: 1}) || !initialReceiptReady(cancelState{Status: cancelStatusCanceled}) {
		t.Fatal("receipt must become ready for one candidate or canonical canceled status")
	}
	if cancelQuestionReady(cancelState{CancelQuestionPresent: true}) || cancelQuestionReady(cancelState{ConfirmCandidateCount: 1}) {
		t.Fatal("question readiness requires both canonical heading and a confirmation candidate")
	}
	if !cancelQuestionReady(cancelState{CancelQuestionPresent: true, ConfirmCandidateCount: 1}) {
		t.Fatal("question with heading and candidate must be ready")
	}
	if canceledReceiptReady(cancelState{Status: cancelStatusConfirmed}) || !canceledReceiptReady(cancelState{Status: cancelStatusCanceled}) {
		t.Fatal("result readiness requires canonical canceled status")
	}
}

func TestCancelUIFlow_ClientCancel_HappyPath(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{canceledAfterPost: true}
	resp, err := runCancelFixture(t, fx)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.Canceled || fx.firstClicks.Load() != 1 || fx.secondClicks.Load() != 1 {
		t.Fatalf("resp=%+v clicks=(%d,%d)", resp, fx.firstClicks.Load(), fx.secondClicks.Load())
	}
}

func TestCancelUIFlow_ClientCancel_AlreadyCanceledIsZeroClickSuccess(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{receiptHTML: `<h1>Reservation canceled</h1>`}
	resp, err := runCancelFixture(t, fx)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.Canceled || fx.firstClicks.Load() != 0 || fx.secondClicks.Load() != 0 {
		t.Fatalf("resp=%+v clicks=(%d,%d), want zero clicks", resp, fx.firstClicks.Load(), fx.secondClicks.Load())
	}
}

func TestCancelUIFlow_ClientCancel_RejectsWrongIdentityAfterRedirect(t *testing.T) {
	requireTockCancelChrome(t)
	for _, tc := range []struct {
		name     string
		redirect string
	}{
		{name: "wrong slug", redirect: "/other-venue/receipt?purchaseId=123"},
		{name: "wrong purchase", redirect: "/venue/receipt?purchaseId=999"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := &cancelFixture{receiptRedirect: tc.redirect}
			_, err := runCancelFixture(t, fx)
			assertCancelCategory(t, err, ErrCancelUIFlow, "page_identity_mismatch")
			if fx.firstClicks.Load() != 0 || fx.secondClicks.Load() != 0 {
				t.Fatal("identity mismatch must fail before clicks")
			}
		})
	}
}

func TestCancelUIFlow_ClientCancel_RejectsWrongPhaseOrHeading(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{questionHTML: `<h1>Review reservation</h1><button>Cancel reservation</button>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_question_missing")
	if fx.secondClicks.Load() != 0 {
		t.Fatal("missing canonical question heading must fail before second click")
	}
}

func TestCancelUIFlow_ClientCancel_MissingCancelControl(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{receiptHTML: `<h1>Reservation confirmed</h1>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_control_count")
}

func TestCancelUIFlow_ClientCancel_IgnoresHiddenAndDisabledDuplicates(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{canceledAfterPost: true, receiptHTML: `<h1>Reservation confirmed</h1>
<button style="display:none">Cancel</button><button disabled>Cancel</button>
<a href="/first-click-nav">Cancel</a>`,
		questionHTML: `<h1>Do you want to cancel your reservation?</h1>
<button style="display:none">Cancel reservation</button><button disabled>Cancel reservation</button>
<form method="post" action="/commit"><button>Cancel reservation</button></form>`,
	}
	resp, err := runCancelFixture(t, fx)
	if err != nil || resp == nil || !resp.Canceled {
		t.Fatalf("Cancel resp=%+v err=%v", resp, err)
	}
}

func TestCancelUIFlow_ClientCancel_TwoEligibleCandidatesFailClosed(t *testing.T) {
	requireTockCancelChrome(t)
	for _, tc := range []struct {
		name         string
		fx           *cancelFixture
		wantCategory string
	}{
		{name: "receipt controls", fx: &cancelFixture{receiptHTML: `<h1>Reservation confirmed</h1><button>Cancel</button><button>Cancel</button>`}, wantCategory: "cancel_control_count"},
		{name: "confirmation controls", fx: &cancelFixture{questionHTML: `<h1>Do you want to cancel your reservation?</h1><button>Cancel reservation</button><button>Cancel reservation</button>`}, wantCategory: "confirm_control_count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runCancelFixture(t, tc.fx)
			assertCancelCategory(t, err, ErrCancelUIFlow, tc.wantCategory)
			if tc.fx.secondClicks.Load() != 0 {
				t.Fatal("multiple candidates must fail before irreversible dispatch")
			}
		})
	}
}

func TestCancelUIFlow_ClientCancel_MissingConfirmationControl(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{questionHTML: `<h1>Do you want to cancel your reservation?</h1><button>Modify reservation</button>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "confirm_control_count")
	if fx.secondClicks.Load() != 0 {
		t.Fatal("missing confirmation control must fail before irreversible dispatch")
	}
}

func TestCancelUIFlow_ClientCancel_OccludedControlFailsClosed(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{receiptHTML: `<h1>Reservation confirmed</h1><button>Cancel</button>
<div style="position:fixed;inset:0;z-index:999;background:white">blocking layer</div>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_control_count")
}

func TestCancelUIFlow_ClientCancel_ModifyOnlyNeverClicks(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{receiptHTML: `<h1>Reservation confirmed</h1><button>Modify</button><button>Modify reservation</button>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_control_count")
	if fx.firstClicks.Load() != 0 || fx.secondClicks.Load() != 0 {
		t.Fatal("Modify controls must never be clicked")
	}
}

func TestCancelUIFlow_ClientCancel_NavigationLossBeforeSecondClickIsDefiniteFailure(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{receiptHTML: `<h1>Reservation confirmed</h1><a href="/lost-before">Cancel</a>`}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_question_not_reached")
	if errors.Is(err, ErrCancelOutcomeUnknown) {
		t.Fatal("pre-second-click loss must be a definite failure")
	}
}

func TestCancelUIFlow_ClientCancel_NavigationLossAfterSecondClickIsOutcomeUnknown(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{afterConfirmPath: "/lost-after"}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelOutcomeUnknown, "result_not_proved")
	if fx.secondClicks.Load() != 1 {
		t.Fatalf("second click requests=%d, want exactly one and no automatic retry", fx.secondClicks.Load())
	}
	if !strings.Contains(err.Error(), `"second_click_dispatched":true`) {
		t.Fatalf("outcome-unknown error lacks dispatched marker: %v", err)
	}
}

func TestCancelUIFlow_ClientCancel_AmbiguousFinalDispatchIsOutcomeUnknown(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{failDispatchOnCall: 2}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelOutcomeUnknown, cancelClickDispatchAmbiguous)
	if errors.Is(err, ErrCancelUIFlow) && !errors.Is(err, ErrCancelOutcomeUnknown) {
		t.Fatal("ambiguous final dispatch must not surface as a retryable UI failure")
	}
	if fx.secondClicks.Load() != 0 {
		t.Fatalf("commit POSTs=%d, want zero (failure injected before activation) and no automatic retry", fx.secondClicks.Load())
	}
}

func TestCancelUIFlow_ClientCancel_AmbiguousFirstDispatchStaysRetryable(t *testing.T) {
	requireTockCancelChrome(t)
	fx := &cancelFixture{failDispatchOnCall: 1}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, cancelClickDispatchAmbiguous)
	if errors.Is(err, ErrCancelOutcomeUnknown) {
		t.Fatal("receipt-phase click only opens the confirmation page; ambiguity there must stay retryable")
	}
}

func TestCancelUIFlow_ClientCancel_PrivacyFixture(t *testing.T) {
	requireTockCancelChrome(t)
	secrets := []string{
		"Ada Lovelace", "ada@example.test", "+1-212-555-0199", "TOCK-R-SECRET99",
		"purchase-372077155", "query-token-supersecret", "4111111111111111",
	}
	raw := html.EscapeString(strings.Join(secrets, " | "))
	fx := &cancelFixture{receiptHTML: fmt.Sprintf(`<h1>Reservation confirmed</h1><div role="alert">%s</div><div class="error">%s</div>`, raw, raw)}
	_, err := runCancelFixture(t, fx)
	assertCancelCategory(t, err, ErrCancelUIFlow, "cancel_control_count")
	for _, secret := range secrets {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("privacy leak %q in error: %v", secret, err)
		}
	}
	if strings.Contains(err.Error(), "venue") || strings.Contains(err.Error(), "123") {
		t.Fatalf("slug or purchase ID leaked in cancel state: %v", err)
	}
}

func runCancelFixture(t *testing.T, fx *cancelFixture) (*CancelResponse, error) {
	t.Helper()
	if fx.receiptHTML == "" {
		fx.receiptHTML = `<h1>Reservation confirmed</h1><a href="/first-click-nav">Cancel</a>`
	}
	if fx.questionHTML == "" {
		action := fx.afterConfirmPath
		if action == "" {
			action = "/commit"
		}
		fx.questionHTML = fmt.Sprintf(`<h1>Do you want to cancel your reservation?</h1><form method="post" action="%s"><button>Cancel reservation</button></form>`, html.EscapeString(action))
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/venue/receipt/cancel":
			http.Error(w, "endpoint missing", http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/venue/receipt":
			// A redirect fixture models one provider redirect followed by a
			// readable terminal page. Redirecting every request makes the
			// wrong-purchase target loop back to itself and tests navigation
			// failure instead of reservation-identity rejection.
			if fx.receiptRedirect != "" && fx.receiptRedirected.CompareAndSwap(false, true) {
				http.Redirect(w, r, fx.receiptRedirect, http.StatusFound)
				return
			}
			if fx.canceled.Load() {
				_, _ = io.WriteString(w, `<h1>Reservation canceled</h1>`)
				return
			}
			_, _ = io.WriteString(w, fx.receiptHTML)
		case r.Method == http.MethodGet && r.URL.Path == "/venue/receipt/cancel":
			_, _ = io.WriteString(w, fx.questionHTML)
		case r.URL.Path == "/first-click-nav":
			fx.firstClicks.Add(1)
			http.Redirect(w, r, "/venue/receipt/cancel", http.StatusFound)
		case r.Method == http.MethodPost && r.URL.Path == "/commit":
			fx.secondClicks.Add(1)
			if fx.canceledAfterPost {
				fx.canceled.Store(true)
			}
			http.Redirect(w, r, "/venue/receipt?purchaseId=123", http.StatusFound)
		case r.URL.Path == "/lost-after":
			fx.secondClicks.Add(1)
			_, _ = io.WriteString(w, `<h1>unavailable</h1>`)
		case r.URL.Path == "/lost-before":
			fx.firstClicks.Add(1)
			_, _ = io.WriteString(w, `<h1>unavailable</h1>`)
		default:
			_, _ = io.WriteString(w, `<h1>Reservation confirmed</h1>`)
		}
	}))
	defer srv.Close()
	c := newCancelHTTPTestClient(t, srv)
	c.cancelUIPhaseTimeout = 750 * time.Millisecond
	if fx.failDispatchOnCall > 0 {
		c.cancelClickDispatch = func(ctx context.Context, x, y float64) error {
			if fx.dispatchCalls.Add(1) == fx.failDispatchOnCall {
				return errors.New("cdp: response lost mid-dispatch")
			}
			return c.defaultCancelClickDispatch(ctx, x, y)
		}
	}
	return c.Cancel(context.Background(), CancelRequest{VenueSlug: "venue", PurchaseID: 123})
}

func assertCancelCategory(t *testing.T, err, wantKind error, category string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %v category %q, got nil", wantKind, category)
	}
	if !errors.Is(err, wantKind) {
		t.Fatalf("errors.Is(%v, %v)=false", err, wantKind)
	}
	if !strings.Contains(err.Error(), `"error_category":"`+category+`"`) {
		t.Fatalf("error %q missing category %q", err, category)
	}
}
