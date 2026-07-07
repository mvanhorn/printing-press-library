package captcha

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// readinessFake reports hcaptcha not-ready for notReadyLeft checks, then ready;
// the solve JS returns token.
type readinessFake struct {
	notReadyLeft int
	token        string
	readyChecks  int
}

func (f *readinessFake) setCookies(context.Context, []CDPCookie) error { return nil }
func (f *readinessFake) navigate(context.Context) error                { return nil }
func (f *readinessFake) showOnScreen(context.Context) error            { return nil }
func (f *readinessFake) close()                                        {}
func (f *readinessFake) evaluate(_ context.Context, js string) (string, error) {
	if strings.Contains(js, "hcaptcha.render === 'function'") {
		f.readyChecks++
		if f.notReadyLeft > 0 {
			f.notReadyLeft--
			return "waiting", nil
		}
		return "ready", nil
	}
	return f.token, nil
}

func newReadinessSolver(fb *readinessFake) *solver {
	return &solver{
		open: func(context.Context, Options, bool) (browser, error) { return fb, nil },
		seed: func(context.Context) ([]CDPCookie, error) { return nil, nil },
	}
}

func TestSolve_WaitsForHCaptchaThenSolves(t *testing.T) {
	fb := &readinessFake{notReadyLeft: 2, token: "P1_after_load"}
	tok, err := newReadinessSolver(fb).Solve(context.Background(), Options{Profile: "default"})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "P1_after_load" {
		t.Fatalf("token=%q", tok)
	}
	if fb.readyChecks < 3 {
		t.Fatalf("expected >=3 readiness checks (2 waiting + 1 ready), got %d", fb.readyChecks)
	}
}

func TestSolve_HCaptchaNeverLoads_Errors(t *testing.T) {
	fb := &readinessFake{notReadyLeft: 1 << 30, token: "x"}
	_, err := newReadinessSolver(fb).Solve(context.Background(),
		Options{Profile: "default", Timeout: 300 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "never finished loading") {
		t.Fatalf("want 'never finished loading' error, got %v", err)
	}
}

// fakeBrowser drives solver branch tests without real Chrome. It dispatches on
// the JS markers emitted by execute.go, so each phase (the invisible attempt,
// the interactive kick, the token poll, the visibility probe) is steered
// independently rather than via a brittle call-sequence.
type fakeBrowser struct {
	firstResult  string   // /*pp:invisible*/ result (headless attempt)
	tokenResults []string // /*pp:token*/ results, in sequence
	tokenCalls   int
	visible      string // /*pp:visible*/ result; "" defaults to "visible"
	kickResult   string // /*pp:kick*/ result; "" defaults to "ok"
	shown        bool
	cookiesSet   bool
	kicks        int
}

func (f *fakeBrowser) setCookies(_ context.Context, _ []CDPCookie) error {
	f.cookiesSet = true
	return nil
}
func (f *fakeBrowser) navigate(_ context.Context) error     { return nil }
func (f *fakeBrowser) showOnScreen(_ context.Context) error { f.shown = true; return nil }
func (f *fakeBrowser) close()                               {}
func (f *fakeBrowser) evaluate(_ context.Context, js string) (string, error) {
	switch {
	case strings.Contains(js, "render === 'function'"): // readiness poll
		return "ready", nil
	case strings.Contains(js, "/*pp:invisible*/"):
		return f.firstResult, nil
	case strings.Contains(js, "/*pp:kick*/"):
		f.kicks++
		if f.kickResult != "" {
			return f.kickResult, nil
		}
		return "ok", nil
	case strings.Contains(js, "/*pp:token*/"):
		r := ""
		if f.tokenCalls < len(f.tokenResults) {
			r = f.tokenResults[f.tokenCalls]
		}
		f.tokenCalls++
		return r, nil
	case strings.Contains(js, "/*pp:visible*/"):
		if f.visible == "" {
			return "visible", nil
		}
		return f.visible, nil
	}
	return "", nil
}

func newTestSolver(fb *fakeBrowser, seed SeedFunc) *solver {
	return &solver{
		open: func(_ context.Context, _ Options, _ bool) (browser, error) { return fb, nil },
		seed: seed,
	}
}

// swapTiming shrinks the interactive timing knobs for tests, returning a restore
// func to defer.
func swapTiming(poll, grace time.Duration) func() {
	op, og := interactivePollInterval, challengeRenderGrace
	interactivePollInterval, challengeRenderGrace = poll, grace
	return func() { interactivePollInterval, challengeRenderGrace = op, og }
}

func TestSolve_InvisibleSuccess(t *testing.T) {
	fb := &fakeBrowser{firstResult: "P1_goodtoken"}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	tok, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tok != "P1_goodtoken" {
		t.Fatalf("token=%q", tok)
	}
	if fb.shown {
		t.Fatal("should not have shown window on invisible success")
	}
}

// TestSolve_AlwaysSeeds locks in the issue #3 invariant: every solve re-seeds
// the full cookie jar from the user's browser (there is no "already seeded"
// shortcut to gate on). Skipping the re-seed lets the dedicated profile's
// Clerk/Google-SSO session go stale and resurfaces the per-generation
// "Continue as <user>" OAuth popup.
func TestSolve_AlwaysSeeds(t *testing.T) {
	fb := &fakeBrowser{firstResult: "P1_tok"}
	seeded := false
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) {
		seeded = true
		return []CDPCookie{{Name: "__client", Value: "x", Domain: ".suno.com", Path: "/"}}, nil
	})
	_, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true})
	if err != nil {
		t.Fatal(err)
	}
	if !seeded || !fb.cookiesSet {
		t.Fatal("every solve must re-seed cookies from the browser (keeps us clear of issue #3)")
	}
}

func TestSolve_InteractiveNeeded_NoInput_ReturnsErr(t *testing.T) {
	fb := &fakeBrowser{firstResult: ""}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	_, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: false})
	if !errors.Is(err, ErrInteractiveRequired) {
		t.Fatalf("want ErrInteractiveRequired, got %v", err)
	}
	if fb.shown {
		t.Fatal("must NOT show window under --no-input")
	}
}

func TestSolve_InteractiveFallback_ShowsVerifiesAndSolves(t *testing.T) {
	defer swapTiming(1*time.Millisecond, 50*time.Millisecond)()
	fb := &fakeBrowser{firstResult: "", visible: "visible", tokenResults: []string{"", "P1_after_manual"}}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	tok, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true})
	if err != nil {
		t.Fatal(err)
	}
	if tok != "P1_after_manual" {
		t.Fatalf("token=%q", tok)
	}
	if !fb.shown {
		t.Fatal("interactive fallback must show the window")
	}
	if fb.kicks < 1 {
		t.Fatal("interactive fallback must present a challenge")
	}
}

// TestSolve_InteractiveChallengeNeverRenders_Errors covers the exact failure the
// user hit: the window comes forward but no solvable challenge appears. We must
// fail fast with a clear message instead of blocking the whole budget.
func TestSolve_InteractiveChallengeNeverRenders_Errors(t *testing.T) {
	defer swapTiming(1*time.Millisecond, 20*time.Millisecond)()
	fb := &fakeBrowser{firstResult: "", visible: "hidden"}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	_, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true})
	if err == nil || !strings.Contains(err.Error(), "did not render") {
		t.Fatalf("want 'did not render' error, got %v", err)
	}
}

// TestSolve_InteractiveKickError_SurfacesImmediately: when the kick script throws
// synchronously (hcaptcha.render/execute), it returns "ERR:..." without ever
// setting window.__ppErr. The solver must report that JS failure right away
// rather than waiting out the render-grace and blaming a non-rendered challenge.
func TestSolve_InteractiveKickError_SurfacesImmediately(t *testing.T) {
	defer swapTiming(1*time.Millisecond, time.Hour)() // huge grace: a render-grace fallthrough would hang, not fail fast
	fb := &fakeBrowser{firstResult: "", kickResult: "ERR:hcaptcha is not defined"}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	_, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true})
	if err == nil || !strings.Contains(err.Error(), "hcaptcha is not defined") {
		t.Fatalf("want kick failure surfaced, got %v", err)
	}
	if strings.Contains(err.Error(), "did not render") {
		t.Fatalf("must not mask the real JS error as a render timeout: %v", err)
	}
}

// TestSolve_InteractiveShownButUnsolved_TimesOut: challenge rendered, user never
// solved it before the budget elapsed.
func TestSolve_InteractiveShownButUnsolved_TimesOut(t *testing.T) {
	defer swapTiming(1*time.Millisecond, time.Hour)() // huge grace so we hit the budget, not the render check
	fb := &fakeBrowser{firstResult: "", visible: "visible"}
	s := newTestSolver(fb, func(context.Context) ([]CDPCookie, error) { return nil, nil })
	_, err := s.Solve(context.Background(), Options{Profile: "default", Interactive: true, Timeout: 30 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "not solved within") {
		t.Fatalf("want 'not solved within' error, got %v", err)
	}
}
