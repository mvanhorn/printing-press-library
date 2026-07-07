// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Solve orchestrator: ensure the dedicated Chrome -> (seed once) -> navigate ->
// invisible execute(); on challenge-expired, fall back to a visible manual
// solve ONLY when Options.Interactive is true. Under --no-input/--agent the
// visible window is never shown — ErrInteractiveRequired is returned instead.

package captcha

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// solver is the default Solver. Fields are function seams for testability.
type solver struct {
	open func(ctx context.Context, opts Options, visible bool) (browser, error)
	seed SeedFunc
}

// New returns the production Solver.
func New() Solver {
	return &solver{
		open: func(ctx context.Context, opts Options, visible bool) (browser, error) {
			return openBrowser(ctx, opts, visible)
		},
		seed: seedFromBrowser,
	}
}

// interactivePollInterval is how often the visible-fallback re-checks for a
// solved token and for the challenge having rendered. Var, not const, so tests
// can shrink it.
var interactivePollInterval = 2 * time.Second

// challengeRenderGrace bounds how long we wait, after foregrounding the window,
// for the hCaptcha challenge to actually appear. If it never renders we fail
// with a clear error instead of blocking on a window the user can't act on.
var challengeRenderGrace = 15 * time.Second

// hcaptchaLoadPollInterval is how often we re-check whether the hCaptcha API
// has finished loading on the page.
const hcaptchaLoadPollInterval = 250 * time.Millisecond

// hcaptchaReadyJS resolves to "ready" once the hCaptcha API is loaded and
// hcaptcha.render is callable, and "waiting" otherwise.
func hcaptchaReadyJS() string {
	return `(typeof hcaptcha !== 'undefined' && typeof hcaptcha.render === 'function') ? 'ready' : 'waiting'`
}

// waitForHCaptchaReady polls the page until the hCaptcha API is loaded, or the
// context deadline is hit. It returns a descriptive error on timeout so the
// caller can distinguish "page never loaded hCaptcha" from a solve failure.
func waitForHCaptchaReady(ctx context.Context, b browser) error {
	for {
		state, err := b.evaluate(ctx, hcaptchaReadyJS())
		if err == nil && strings.TrimSpace(state) == "ready" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("hcaptcha never finished loading on %s: %w", sunoCreateURL, ctx.Err())
		case <-time.After(hcaptchaLoadPollInterval):
		}
	}
}

func (s *solver) Solve(ctx context.Context, opts Options) (string, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	b, err := s.open(ctx, opts, false)
	if err != nil {
		return "", err
	}
	defer b.close()

	// Always seed the session from the user's logged-in Chrome cookies before
	// navigating: suno.com/create only loads the hCaptcha API for an
	// authenticated session, and the dedicated solver profile is not a reliable
	// persistence store — a persisted profile session can outlive the actual
	// login, which left the page logged out and hCaptcha unloaded. Re-seeding the
	// full cookie jar every solve is also what keeps us clear of
	// paperfoot/suno-cli#3's per-generation OAuth popup.
	cookies, serr := s.seed(ctx)
	if serr != nil {
		return "", serr
	}
	if err := b.setCookies(ctx, cookies); err != nil {
		return "", err
	}
	if err := b.navigate(ctx); err != nil {
		return "", err
	}

	// suno.com/create loads the hCaptcha API asynchronously; calling
	// hcaptcha.render() before it finishes throws "hcaptcha is not defined".
	// Poll until the API is ready (matching paperfoot/suno-cli) before solving.
	if err := waitForHCaptchaReady(ctx, b); err != nil {
		return "", err
	}

	raw, err := b.evaluate(ctx, solveJS())
	if err != nil {
		return "", err
	}
	tok, interactiveNeeded, cerr := classifyToken(raw)
	if cerr != nil {
		return "", cerr
	}
	if tok != "" {
		return tok, nil
	}

	if !interactiveNeeded {
		return "", ErrInteractiveRequired
	}
	if !opts.Interactive {
		return "", ErrInteractiveRequired
	}
	return solveInteractively(ctx, b, timeout)
}

// kickInteractive renders+executes a fresh invisible hCaptcha challenge. The
// kick script's outer try/catch returns "ERR:..." synchronously when
// hcaptcha.render/execute throws (e.g. hcaptcha not loaded, widget limit hit)
// — a path that never sets window.__ppErr, so the token poll would otherwise
// stay empty and the caller would wait out the render-grace before reporting a
// misleading "never rendered". Surface that JS failure immediately instead.
func kickInteractive(ctx context.Context, b browser) error {
	raw, err := b.evaluate(ctx, interactiveKickJS())
	if err != nil {
		return err
	}
	if r := strings.TrimSpace(raw); strings.HasPrefix(r, "ERR:") {
		return fmt.Errorf("captcha: could not start the interactive challenge: %s", strings.TrimPrefix(r, "ERR:"))
	}
	return nil
}

// solveInteractively brings the offscreen solver window onto the desktop,
// presents a fresh hCaptcha challenge, verifies it actually rendered, then polls
// for the user-submitted token until the deadline. It distinguishes three
// failure modes so the user knows what went wrong: the window couldn't be shown,
// the challenge never rendered, or it rendered but wasn't solved in time.
func solveInteractively(ctx context.Context, b browser, budget time.Duration) (string, error) {
	if err := b.showOnScreen(ctx); err != nil {
		return "", fmt.Errorf("captcha: could not bring the solver window on-screen: %w", err)
	}
	if err := kickInteractive(ctx, b); err != nil {
		return "", err
	}

	graceDeadline := time.Now().Add(challengeRenderGrace)
	sawChallenge := false
	for {
		select {
		case <-ctx.Done():
			if !sawChallenge {
				return "", fmt.Errorf("captcha: the challenge never appeared in the Chrome window within %s — re-run, or check you're logged into suno.com in Chrome: %w", budget, ctx.Err())
			}
			return "", fmt.Errorf("captcha: challenge was shown but not solved within %s — solve it sooner or re-run: %w", budget, ctx.Err())
		case <-time.After(interactivePollInterval):
		}

		raw, err := b.evaluate(ctx, interactiveTokenJS())
		if err != nil {
			return "", err
		}
		raw = strings.TrimSpace(raw)
		switch {
		case raw == "":
			// still waiting on the user
		case strings.HasPrefix(raw, "ERR:"):
			reason := strings.ToLower(strings.TrimPrefix(raw, "ERR:"))
			if strings.Contains(reason, "expired") {
				// The challenge timed out before the user finished. Present a
				// fresh one and restart the render-grace window.
				if err := kickInteractive(ctx, b); err != nil {
					return "", err
				}
				sawChallenge = false
				graceDeadline = time.Now().Add(challengeRenderGrace)
				continue
			}
			return "", fmt.Errorf("captcha solver: %s", strings.TrimPrefix(raw, "ERR:"))
		default:
			return raw, nil // solved
		}

		// Verify the challenge is genuinely on-screen; fail fast if it never is.
		if !sawChallenge {
			vis, _ := b.evaluate(ctx, challengeVisibleJS())
			if strings.TrimSpace(vis) == "visible" {
				sawChallenge = true
			} else if time.Now().After(graceDeadline) {
				return "", fmt.Errorf("captcha: the hCaptcha challenge did not render in the Chrome window within %s — the page presented no solvable challenge; re-run or verify your suno.com login", challengeRenderGrace)
			}
		}
	}
}

// Stop tears down the managed Chrome for a single profile port.
func Stop(ctx context.Context, port int) error {
	return killManagedChrome(ctx, port)
}

// StatusFor reports whether a managed Chrome is running for the given profile.
func StatusFor(profile string, port int) ProfileStatus {
	return ProfileStatus{Profile: profile, Port: port, Running: portAlive(port)}
}

// loginOpen opens the visible profile window. It is a package var so login_test
// can substitute a fake browser without launching real Chrome.
var loginOpen = func(ctx context.Context, opts Options, visible bool) (browser, error) {
	return openBrowser(ctx, opts, visible)
}

// Login opens a visible window for the profile and navigates to suno.com so the
// user can establish/switch the account session, which then persists in the
// dedicated profile. It returns once the page has loaded; the window is left
// running so the user can sign in (and Solve reconnects to it via the CDP
// port). `auth captcha stop`, or closing the window, tears it down.
func Login(ctx context.Context, opts Options) error {
	b, err := loginOpen(ctx, opts, true)
	if err != nil {
		return err
	}
	if err := b.navigate(ctx); err != nil {
		// The window never reached Suno; tear it down rather than leave a
		// blank, useless Chrome running. (cmd.Context() is context.Background,
		// so a successfully-launched window survives process exit on its own —
		// closing it here on the happy path is the regression we are fixing.)
		b.close()
		return err
	}
	return nil
}
