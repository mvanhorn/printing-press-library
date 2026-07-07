// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package captcha

import (
	"context"
	"errors"
	"testing"
)

// loginFakeBrowser records whether the visible window was navigated and,
// critically, whether it was torn down — so we can prove Login leaves it open
// for the user to sign in.
type loginFakeBrowser struct {
	navigated bool
	closed    bool
	navErr    error
}

func (f *loginFakeBrowser) setCookies(context.Context, []CDPCookie) error { return nil }
func (f *loginFakeBrowser) navigate(context.Context) error                { f.navigated = true; return f.navErr }
func (f *loginFakeBrowser) evaluate(context.Context, string) (string, error) {
	return "", nil
}
func (f *loginFakeBrowser) showOnScreen(context.Context) error { return nil }
func (f *loginFakeBrowser) close()                             { f.closed = true }

func withLoginOpen(fb *loginFakeBrowser) func() {
	orig := loginOpen
	loginOpen = func(context.Context, Options, bool) (browser, error) { return fb, nil }
	return func() { loginOpen = orig }
}

// The regression: `auth captcha login` must leave the visible Chrome open so the
// user can sign in. Tearing it down on return closes the window ~1s after it
// appears.
func TestLogin_LeavesWindowOpen(t *testing.T) {
	fb := &loginFakeBrowser{}
	defer withLoginOpen(fb)()

	if err := Login(context.Background(), Options{Profile: "default"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !fb.navigated {
		t.Fatal("Login must navigate the visible window to Suno")
	}
	if fb.closed {
		t.Fatal("Login must NOT close the window — the user needs it open to sign in")
	}
}

// A window that never finished navigating is useless to the user, so a navigate
// failure must tear it down rather than leak a blank window.
func TestLogin_TearsDownOnNavigateError(t *testing.T) {
	fb := &loginFakeBrowser{navErr: errors.New("boom")}
	defer withLoginOpen(fb)()

	if err := Login(context.Background(), Options{Profile: "default"}); err == nil {
		t.Fatal("expected navigate error to propagate")
	}
	if !fb.closed {
		t.Fatal("a window that failed to navigate must be torn down, not leaked")
	}
}
