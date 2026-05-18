// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

// reCAPTCHA Enterprise v3 token acquisition for the Miami-Dade Clerk
// Official Records portal. The clerk's /api/home/standardsearch and
// /api/home/propertysearch endpoints require a fresh per-call
// x-recaptcha-token header obtained by executing
// grecaptcha.execute(siteKey, {action: 'submit'}) in a real browser
// context. We drive a headless Chromium via chromedp and reuse a single
// browser allocator at package scope so successive calls don't pay the
// ~2s cold-start cost on every search.

package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	// ClerkSiteURL is the public landing page where grecaptcha is loaded.
	// The site key is bound to this origin — calls from other URLs return
	// "invalid-site-key" silently as a zero-score token.
	ClerkSiteURL = "https://onlineservices.miamidadeclerk.gov/officialrecords/"

	// ClerkSiteKey is the reCAPTCHA Enterprise v3 site key for the Clerk
	// portal. Public value; safe to commit (the secret key lives server-side).
	ClerkSiteKey = "6LfI8ikaAAAAAH0qlQMApskMGd1U6EqDyniH5t0x"

	// recaptchaTimeout caps a single token acquisition. grecaptcha.execute
	// typically returns in <500ms once the page is ready; the bulk of the
	// budget covers the first-call cold start and Chromium navigation.
	recaptchaTimeout = 15 * time.Second
)

var (
	// browserMu serializes access to the package-scoped allocator. chromedp
	// allocators are themselves safe for concurrent use, but we lazily
	// initialize browserAllocCtx + browserAllocCancel and need a mutex to
	// guard the one-time setup. Subsequent calls acquire the mutex only to
	// read the live context.
	browserMu          sync.Mutex
	browserAllocCtx    context.Context
	browserAllocCancel context.CancelFunc

	// pageReady tracks whether the package-scoped Chromium tab has already
	// navigated to the Clerk site. grecaptcha.execute requires the page to
	// be loaded and the reCAPTCHA script to have registered the site key;
	// navigating once per process is much faster than per-call.
	pageMu     sync.Mutex
	pageCtx    context.Context
	pageCancel context.CancelFunc
	pageLoaded bool
)

// GetRecaptchaToken returns a fresh reCAPTCHA Enterprise v3 token for the
// given (siteURL, siteKey). Tokens are single-use and expire after 2
// minutes — callers must request a new one per protected API call.
//
// The first call in a process pays ~2s for Chromium launch + page load;
// subsequent calls reuse the same browser tab and complete in <500ms.
// The browser is left running for the lifetime of the process; the OS
// reaps it on exit. Callers that need explicit shutdown can call
// CloseRecaptchaBrowser.
func GetRecaptchaToken(ctx context.Context, siteURL, siteKey string) (string, error) {
	if siteURL == "" {
		siteURL = ClerkSiteURL
	}
	if siteKey == "" {
		siteKey = ClerkSiteKey
	}

	allocCtx, err := ensureAllocator()
	if err != nil {
		return "", fmt.Errorf("recaptcha: starting browser: %w", err)
	}

	tabCtx, err := ensurePage(allocCtx, siteURL)
	if err != nil {
		return "", fmt.Errorf("recaptcha: opening page: %w", err)
	}

	// Bound the actual execute call to recaptchaTimeout regardless of the
	// caller's ctx — a hung grecaptcha.execute must not block forever even
	// if the caller passed context.Background().
	execCtx, cancel := context.WithTimeout(tabCtx, recaptchaTimeout)
	defer cancel()

	// Honor cancellation from the user-supplied ctx too.
	if ctx != nil {
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
	}

	token, err := executeRecaptcha(execCtx, siteKey)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", errors.New("recaptcha: grecaptcha.execute returned empty token (site key may be invalid or page not ready)")
	}
	return token, nil
}

// CloseRecaptchaBrowser shuts down the package-scoped Chromium instance.
// Safe to call multiple times; a no-op if nothing was started.
func CloseRecaptchaBrowser() {
	pageMu.Lock()
	if pageCancel != nil {
		pageCancel()
		pageCancel = nil
		pageCtx = nil
		pageLoaded = false
	}
	pageMu.Unlock()

	browserMu.Lock()
	if browserAllocCancel != nil {
		browserAllocCancel()
		browserAllocCancel = nil
		browserAllocCtx = nil
	}
	browserMu.Unlock()
}

func ensureAllocator() (context.Context, error) {
	browserMu.Lock()
	defer browserMu.Unlock()
	if browserAllocCtx != nil {
		// Verify the allocator hasn't been canceled (e.g. via os.Interrupt).
		if browserAllocCtx.Err() == nil {
			return browserAllocCtx, nil
		}
		// Reset state and re-init below.
		browserAllocCancel = nil
		browserAllocCtx = nil
		pageMu.Lock()
		pageCtx = nil
		pageCancel = nil
		pageLoaded = false
		pageMu.Unlock()
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserAllocCtx = allocCtx
	browserAllocCancel = cancel
	return allocCtx, nil
}

func ensurePage(allocCtx context.Context, siteURL string) (context.Context, error) {
	pageMu.Lock()
	defer pageMu.Unlock()
	if pageLoaded && pageCtx != nil && pageCtx.Err() == nil {
		return pageCtx, nil
	}

	if pageCancel != nil {
		pageCancel()
	}
	tabCtx, cancel := chromedp.NewContext(allocCtx)
	pageCtx = tabCtx
	pageCancel = cancel

	loadCtx, loadCancel := context.WithTimeout(tabCtx, recaptchaTimeout)
	defer loadCancel()

	// Navigate, then wait for grecaptcha global to register so the first
	// execute() call doesn't race the script load. We poll instead of
	// chromedp.WaitVisible because grecaptcha is invisible by design.
	err := chromedp.Run(loadCtx,
		chromedp.Navigate(siteURL),
		chromedp.Poll(`typeof grecaptcha !== 'undefined' && typeof grecaptcha.execute === 'function'`, nil,
			chromedp.WithPollingTimeout(recaptchaTimeout),
		),
	)
	if err != nil {
		pageCancel()
		pageCtx = nil
		pageCancel = nil
		return nil, err
	}
	pageLoaded = true
	return tabCtx, nil
}

func executeRecaptcha(ctx context.Context, siteKey string) (string, error) {
	// grecaptcha.execute returns a Promise. chromedp.Evaluate awaits
	// promises when the third arg is opts including AwaitPromise(true).
	// We use chromedp.EvaluateAsDevTools to get promise-aware evaluation.
	var token string
	expr := fmt.Sprintf(
		`grecaptcha.execute(%q, {action: 'submit'}).then(function(t) { return t; })`,
		siteKey,
	)
	if err := chromedp.Run(ctx, chromedp.Evaluate(expr, &token, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return "", err
	}
	return token, nil
}
