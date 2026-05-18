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
	"log"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

var debugRecaptcha = os.Getenv("MDC_RECAPTCHA_DEBUG") != ""

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
	// browserMu serializes both the lazy allocator init and per-call tab
	// creation. chromedp's allocator is safe for concurrent use, but
	// serializing tab creation keeps the headless Chrome footprint small
	// and matches the rate-limited single-request-at-a-time pattern of
	// the surrounding HTTP client.
	browserMu          sync.Mutex
	browserAllocCtx    context.Context
	browserAllocCancel context.CancelFunc
)

// GetRecaptchaToken returns a fresh reCAPTCHA Enterprise v3 token for the
// given (siteURL, siteKey). Tokens are single-use and expire after 2
// minutes — callers must request a new one per protected API call.
//
// Each call opens a fresh chromedp tab inside a long-lived process-wide
// allocator so the underlying Chromium browser is only spawned on the
// first call. End-to-end takes ~2-3s per token (cold-start chrome ~1s
// reused after the first call, navigate ~1s, script load+execute ~1s).
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

	var tabCtx context.Context
	var tabCancel context.CancelFunc
	if debugRecaptcha {
		tabCtx, tabCancel = chromedp.NewContext(allocCtx, chromedp.WithErrorf(log.Printf))
	} else {
		tabCtx, tabCancel = chromedp.NewContext(allocCtx)
	}
	defer tabCancel()

	// Bound the whole sequence (navigate + inject + execute) to a single
	// timeout. recaptchaTimeout is generous enough for cold-start Chromium
	// boot (~2s) + script load (~1s) + execute (<500ms).
	runCtx, runCancel := context.WithTimeout(tabCtx, 2*recaptchaTimeout)
	defer runCancel()
	if ctx != nil {
		stop := context.AfterFunc(ctx, runCancel)
		defer stop()
	}

	// Inject the reCAPTCHA script ourselves (the Clerk SPA only loads it
	// after the user mounts a search form) and call grecaptcha.execute
	// in the same Promise chain. Doing this in a single chromedp.Run
	// avoids cross-call context lifetime issues with tab contexts.
	combined := fmt.Sprintf(`new Promise(function(resolve, reject) {
  function run() {
    try {
      window.grecaptcha.ready(function() {
        window.grecaptcha.execute(%q, {action: 'submit'})
          .then(function(t) { resolve(t); })
          .catch(function(e) { reject(String(e)); });
      });
    } catch (e) { reject(String(e)); }
  }
  if (window.grecaptcha && window.grecaptcha.execute) { run(); return; }
  var s = document.createElement('script');
  s.src = 'https://www.google.com/recaptcha/api.js?render=%s';
  s.async = true;
  s.onload = run;
  s.onerror = function() { reject('failed to load recaptcha/api.js'); };
  document.head.appendChild(s);
})`, siteKey, siteKey)

	var token string
	var t0 time.Time
	if debugRecaptcha {
		t0 = time.Now()
	}
	err = chromedp.Run(runCtx,
		chromedp.Navigate(siteURL),
		chromedp.Evaluate(combined, &token, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
			return p.WithAwaitPromise(true)
		}),
	)
	if debugRecaptcha {
		fmt.Fprintf(os.Stderr, "[recaptcha] elapsed=%v err=%v runCtx.Err=%v token.len=%d\n",
			time.Since(t0), err, runCtx.Err(), len(token))
	}
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

