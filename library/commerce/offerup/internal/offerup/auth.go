package offerup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// OfferUp authenticated commands ride on the logged-in web session cookie
// (confirmed: GraphQL authenticates with the cookie alone — the x-ou-* request
// headers the web app adds are device/context, not auth). The cookie is
// captured and served by the `press-auth` companion, which owns a controlled
// Chrome login window and stores the session encrypted at rest.

// AuthDomain is the press-auth domain key for OfferUp.
const AuthDomain = "offerup.com"

// LoginURL / CompleteSelector drive the press-auth controlled-login window.
// CompleteSelector is the "My Items" header button, visible only when logged
// in (OfferUp keeps logout behind the avatar menu, so press-auth's default
// "logout link visible" heuristic never fires for it).
const (
	LoginURL         = "https://offerup.com/login"
	CompleteSelector = `[data-testid="SellingMenu"]`
)

// ErrNotLoggedIn signals no captured OfferUp session is available.
var ErrNotLoggedIn = errors.New("not logged in to OfferUp — run 'offerup-pp-cli auth login --chrome'")

// ErrNoPressAuth signals the press-auth companion is not installed.
var ErrNoPressAuth = errors.New("the 'press-auth' companion is required for authenticated commands; install it, then run 'offerup-pp-cli auth login --chrome'")

// PressAuthBin resolves the press-auth binary: PRESS_AUTH_BIN override first
// (used by tests and the verify harness), then PATH. Returns "" when absent.
func PressAuthBin() string {
	if v := strings.TrimSpace(os.Getenv("PRESS_AUTH_BIN")); v != "" {
		return v
	}
	if p, err := exec.LookPath("press-auth"); err == nil {
		return p
	}
	return ""
}

// LoginArgs are the press-auth args that capture an OfferUp session.
func LoginArgs() []string {
	return []string{"login", AuthDomain, "--login-url", LoginURL, "--complete-selector", CompleteSelector}
}

// CookieHeader returns the captured Cookie header for OfferUp via press-auth,
// or ErrNotLoggedIn / ErrNoPressAuth. The value is never logged.
func CookieHeader(ctx context.Context) (string, error) {
	bin := PressAuthBin()
	if bin == "" {
		return "", ErrNoPressAuth
	}
	// Retry once: press-auth decrypts via the OS keychain, which can fail
	// transiently under rapid repeated invocation (keychain contention).
	for attempt := 0; attempt < 2; attempt++ {
		out, err := exec.CommandContext(ctx, bin, "cookies", AuthDomain).Output()
		if err == nil {
			if cookie := strings.TrimSpace(string(out)); cookie != "" {
				return cookie, nil
			}
		}
		if attempt == 0 {
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
	return "", ErrNotLoggedIn
}

// LoggedIn reports whether a captured OfferUp session is available.
func LoggedIn(ctx context.Context) bool {
	_, err := CookieHeader(ctx)
	return err == nil
}

// RunLogin shells out to press-auth to capture an OfferUp session. press-auth
// owns the controlled Chrome window; its stdout/stderr stream to the user.
func RunLogin(ctx context.Context) error {
	bin := PressAuthBin()
	if bin == "" {
		return ErrNoPressAuth
	}
	cmd := exec.CommandContext(ctx, bin, LoginArgs()...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("press-auth login failed: %w", err)
	}
	return nil
}

// Logout removes the captured OfferUp session via press-auth.
func Logout(ctx context.Context) error {
	bin := PressAuthBin()
	if bin == "" {
		return ErrNoPressAuth
	}
	return exec.CommandContext(ctx, bin, "forget", AuthDomain, "--yes").Run()
}
