// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Per-profile Chrome lifecycle. Reconnects to an already-running managed Chrome
// on the profile's CDP port; otherwise launches a headed (NOT --headless:
// hCaptcha trips on headless) Chrome shoved far offscreen with the profile's
// dedicated --user-data-dir.
//
// Persistence is established by `auth captcha login`, which launches an instance
// and deliberately leaves it running for later reconnects; Stop tears it down.
// On the reconnect path close() cancels only the RemoteAllocator connection, so
// that managed instance survives. A plain Solve with no managed instance running
// cold-launches its own offscreen Chrome via an ExecAllocator and tears it down
// when Solve returns (close() -> allocCancel kills the launched process) — that
// path is ephemeral per-solve, not persistent.

package captcha

import (
	"context"
	"fmt"
	"net"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// browser abstracts the page operations Solve needs, so solver.go can be
// unit-tested with a fake.
type browser interface {
	setCookies(ctx context.Context, cookies []CDPCookie) error
	navigate(ctx context.Context) error
	evaluate(ctx context.Context, js string) (string, error)
	showOnScreen(ctx context.Context) error
	close()
}

// portAlive reports whether something is listening on the profile's CDP port.
func portAlive(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// chromedpBrowser is the production browser backed by chromedp.
type chromedpBrowser struct {
	allocCancel context.CancelFunc
	ctxCancel   context.CancelFunc
	runCtx      context.Context
}

// openBrowser reconnects to a running managed Chrome on opts.CDPPort, or
// launches a new offscreen one bound to that port + opts.UserDataDir.
func openBrowser(parent context.Context, opts Options, visible bool) (*chromedpBrowser, error) {
	var allocCtx context.Context
	var allocCancel context.CancelFunc

	if portAlive(opts.CDPPort) {
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(parent,
			fmt.Sprintf("http://127.0.0.1:%d", opts.CDPPort))
	} else {
		execOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.UserDataDir(opts.UserDataDir),
			chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", opts.CDPPort)),
			chromedp.Flag("window-size", "1280,900"),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
			chromedp.Flag("disable-search-engine-choice-screen", true),
		)
		// Position once based on visibility: a visible solve sits at 0,0; a
		// hidden one parks fully off-screen. (Previously set twice, with the
		// first assignment dead when visible.)
		if visible {
			execOpts = append(execOpts, chromedp.Flag("window-position", "0,0"))
		} else {
			execOpts = append(execOpts, chromedp.Flag("window-position", "-32000,-32000"))
		}
		allocCtx, allocCancel = chromedp.NewExecAllocator(parent, execOpts...)
	}

	runCtx, ctxCancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(runCtx); err != nil {
		ctxCancel()
		allocCancel()
		return nil, fmt.Errorf("starting chrome (profile %q, port %d): %w", opts.Profile, opts.CDPPort, err)
	}
	return &chromedpBrowser{allocCancel: allocCancel, ctxCancel: ctxCancel, runCtx: runCtx}, nil
}

func (b *chromedpBrowser) setCookies(_ context.Context, cookies []CDPCookie) error {
	if len(cookies) == 0 {
		return nil
	}
	params := make([]*network.CookieParam, 0, len(cookies))
	for _, c := range cookies {
		params = append(params, &network.CookieParam{
			Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly,
		})
	}
	return chromedp.Run(b.runCtx, network.SetCookies(params))
}

func (b *chromedpBrowser) navigate(_ context.Context) error {
	return chromedp.Run(b.runCtx, chromedp.Navigate(sunoCreateURL))
}

func (b *chromedpBrowser) evaluate(_ context.Context, js string) (string, error) {
	var out string
	err := chromedp.Run(b.runCtx, chromedp.Evaluate(js, &out, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	return out, err
}

// showOnScreen repositions the offscreen solver window back onto the visible
// desktop via CDP Browser.setWindowBounds. The window is launched parked at
// x=-32000; window.moveTo() does NOT move a top-level browser window (it only
// works on script-opened popups), which is why the interactive fallback used to
// leave the challenge invisible and the user had to kill Chrome by hand. Left/
// Top are positive on purpose: cdproto's Bounds tags Left/Top omitzero, so 0
// would be dropped from the JSON and the window would stay parked offscreen.
func (b *chromedpBrowser) showOnScreen(_ context.Context) error {
	return chromedp.Run(b.runCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		winID, _, err := cdpbrowser.GetWindowForTarget().Do(ctx)
		if err != nil {
			return err
		}
		bounds := &cdpbrowser.Bounds{
			Left: 60, Top: 60, Width: 1280, Height: 900,
			WindowState: cdpbrowser.WindowStateNormal,
		}
		return cdpbrowser.SetWindowBounds(winID, bounds).Do(ctx)
	}))
}

func (b *chromedpBrowser) close() {
	if b.ctxCancel != nil {
		b.ctxCancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}

// killManagedChrome terminates the managed Chrome listening on port.
func killManagedChrome(parent context.Context, port int) error {
	if !portAlive(port) {
		return nil
	}
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(parent,
		fmt.Sprintf("http://127.0.0.1:%d", port))
	defer allocCancel()
	runCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	return chromedp.Cancel(runCtx)
}
