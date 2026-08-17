// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Native hCaptcha solver via piloted Chrome. This package owns ALL browser
// automation (chromedp); no other package imports chromedp. Solve() mints a
// fresh hCaptcha token for a generation request by driving a dedicated,
// persistent, offscreen Chrome profile.

package captcha

import (
	"context"
	"errors"
	"time"
)

// Suno's hCaptcha sitekey, captured from the live web app's hcaptcha.render(...)
// arguments. Same value paperfoot/suno-cli uses.
const SunoHCaptchaSitekey = "d65453de-3f1a-4aac-9366-a0f06e52b2ce"

// Suno serves hCaptcha from its own first-party hosts.
const (
	hcaptchaEndpoint  = "https://hcaptcha-endpoint-prod.suno.com"
	hcaptchaAssetHost = "https://hcaptcha-assets-prod.suno.com"
	hcaptchaImgHost   = "https://hcaptcha-imgs-prod.suno.com"
	hcaptchaReportAPI = "https://hcaptcha-reportapi-prod.suno.com"
	sunoCreateURL     = "https://suno.com/create"
)

// ErrInteractiveRequired is returned by Solve when the invisible attempt failed
// (challenge-expired / interactive challenge) and Options.Interactive is false,
// i.e. running under --no-input/--agent where no human can solve.
var ErrInteractiveRequired = errors.New("captcha: interactive solve required but input is disabled")

// Options controls a single Solve call.
type Options struct {
	Profile     string        // resolved profile name (e.g. "default", "work")
	UserDataDir string        // dedicated profile dir; never the user's real Chrome
	CDPPort     int           // this profile's CDP port
	Interactive bool          // false under --no-input/--agent — suppress the visible fallback
	Timeout     time.Duration // overall solve budget (0 => DefaultTimeout)
}

// DefaultTimeout is the overall solve budget when Options.Timeout is zero. It
// must accommodate the macOS prompt to allow access to Chrome's protected
// storage: the solver reads Chrome's data, which can trigger an approval dialog
// the user needs time to accept before the budget expires.
const DefaultTimeout = 180 * time.Second

// Solver is the seam the generation flow depends on. Faked in tests so the CLI
// layer never spins up Chrome.
type Solver interface {
	Solve(ctx context.Context, opts Options) (token string, err error)
}

// SeedFunc returns the user's Suno cookies for first-use seeding. Injected so
// solver_test can supply fixtures without touching the real browser.
type SeedFunc func(ctx context.Context) ([]CDPCookie, error)

// ProfileStatus reports whether a managed Chrome is up for a profile.
type ProfileStatus struct {
	Profile string
	Port    int
	Running bool
}
