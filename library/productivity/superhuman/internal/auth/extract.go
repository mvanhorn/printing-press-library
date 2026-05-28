// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

// This file implements U2 of the auto-auth-chrome-cdp plan:
// the JavaScript IIFE that runs inside a logged-in Superhuman tab via
// CDP Runtime.evaluate, plus the Go-side parser for the response.
//
// The IIFE is ported from edwinhu/superhuman-cli (src/token-api.ts
// `extractToken`), with these adaptations:
//
//   - Wrapped in try/catch so the Go side sees typed JS errors, not
//     uncaught exceptions that surface as opaque CDP exception details.
//   - Probes for required globals (window.GoogleAccount /
//     window.MicrosoftAccount, window.Superhuman) before touching them
//     so we emit a precise "page not ready" hint instead of a
//     TypeError on `undefined`.
//   - Includes a Microsoft account branch (provider: "microsoft") that
//     mirrors the Google branch but reads window.MicrosoftAccount.
//   - Computes userPrefix (chars 7-10 of the user_... external ID after
//     stripping the "user_" prefix) inside the IIFE so the Go side
//     doesn't need to know the convention.
//
// On the Go side, Parse decodes the value Chrome returns under
// result.result.value into ExtractedTokens. Two top-level entry points
// drive the full flow:
//
//   - ExtractFromTab — runs the IIFE against a specific Tab. Used in
//     tests and by U6 when the user has already chosen a tab.
//   - Extract — discovers the port, lists tabs, picks one (by email if
//     provided, or by activeAccountEmail), and runs the IIFE.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ExtractedTokens is the Go-side mirror of the JSON object returned by
// the IIFE. Field tags match the IIFE's return shape exactly so a
// single json.Unmarshal lifts everything in one pass.
//
// Timestamps are epoch milliseconds (matching the on-disk store
// schema), not Time values, so they're trivially round-trippable
// through Save/Load without timezone surprises.
type ExtractedTokens struct {
	Email              string `json:"email"`
	IDToken            string `json:"idToken"`
	IDTokenExpires     int64  `json:"idTokenExpires"`
	RefreshToken       string `json:"refreshToken"`
	AccessToken        string `json:"accessToken"`
	AccessTokenExpires int64  `json:"accessTokenExpires"`
	UserID             string `json:"userId"`
	UserPrefix         string `json:"userPrefix"`
	UserExternalID     string `json:"userExternalId"`
	DeviceID           string `json:"deviceId"`
	Provider           string `json:"provider"`
}

// Typed errors callers branch on with errors.Is.
//
// ErrPageNotReady is intentionally NOT redefined here — cdp.go (U1)
// already exports it for the "exception thrown inside Runtime.evaluate"
// case. We tag the IIFE-emitted "page not ready" error message so
// Parse can detect it from the exception text and wrap with the same
// sentinel that the lower-level CDP layer uses, keeping the surface
// area for callers small.
var (
	// ErrMultipleAccounts is returned by Extract when the user has more
	// than one Superhuman tab open and didn't disambiguate via the email
	// argument. The error message lists the candidate emails so the
	// caller can re-invoke with --account <email>.
	ErrMultipleAccounts = errors.New("multiple Superhuman accounts found; specify --account")

	// ErrSessionExpired is returned when the in-page getIDTokenAsync()
	// call rejects — Superhuman's Firebase session has expired and the
	// user needs to re-log into the tab in Chrome. Distinct from
	// ErrPageNotReady because the remediation is different (re-login vs
	// refresh-the-tab).
	ErrSessionExpired = errors.New("Superhuman session expired in Chrome; re-log in")

	// ErrNoSuperhumanTab is returned by Extract when no tab in the
	// browser matches mail.superhuman.com. Distinct from ErrPageNotReady
	// (which means a Superhuman tab exists but isn't ready) so the CLI
	// can surface different hints.
	ErrNoSuperhumanTab = errors.New("no Superhuman tab found in Chrome; open https://mail.superhuman.com first")
)

// extractIIFE is the JavaScript that runs inside the chosen Superhuman
// tab. Chrome wraps it in an async evaluator (awaitPromise=true on the
// Go side), so the IIFE may return a Promise — we do that by using an
// async IIFE and letting Chrome resolve it.
//
// The leading `(async () => { ... })()` form is intentional: it gives
// us await syntax for getIDTokenAsync without polluting the page
// namespace and without needing eval helpers.
//
// The shape of the returned object MUST match ExtractedTokens' json
// tags exactly. Order doesn't matter; presence does.

// === IIFE source ===
const extractIIFE = `(async () => {
  try {
    // ---- 1. Locate the active account ----
    // Superhuman keeps the umbrella account map on either
    // window.GoogleAccount (for personal/Workspace Gmail) or
    // window.MicrosoftAccount (for Office 365). Each map is keyed by
    // email; the active account is named in activeAccountEmail.
    const __requestedEmail = (typeof __email === 'string' && __email) ? __email : '';

    let provider = '';
    let umbrella = null;
    if (typeof window !== 'undefined' && window.GoogleAccount) {
      umbrella = window.GoogleAccount;
      provider = 'google';
    } else if (typeof window !== 'undefined' && window.MicrosoftAccount) {
      umbrella = window.MicrosoftAccount;
      provider = 'microsoft';
    } else {
      throw new Error('extract: page not ready (GoogleAccount missing)');
    }

    // activeAccountEmail is the canonical handle for the currently
    // selected mailbox. When the caller hasn't pinned an email, we use
    // it. When the caller pins an email, prefer that — but fall back to
    // active if the pinned email isn't present (so the error message
    // points at "page not ready" instead of a silent mismatch).
    const activeEmail = umbrella.activeAccountEmail || '';
    let email = __requestedEmail || activeEmail;
    if (!email) {
      throw new Error('extract: page not ready (no active account)');
    }

    // Account lookup: Superhuman keeps sub-accounts directly on the
    // umbrella object keyed by email, and also under .accounts in some
    // bundle versions. Probe both, prefer the keyed form.
    let account = umbrella[email];
    if (!account && umbrella.accounts) {
      account = umbrella.accounts[email];
    }
    if (!account) {
      // Try the active account if a specific email wasn't found.
      if (activeEmail && activeEmail !== email) {
        email = activeEmail;
        account = umbrella[email] || (umbrella.accounts && umbrella.accounts[email]);
      }
    }
    if (!account) {
      throw new Error('extract: page not ready (account not found for ' + email + ')');
    }

    // ---- 2. Force a fresh Firebase ID token ----
    const credential = account.credential;
    if (!credential || typeof credential.getIDTokenAsync !== 'function') {
      throw new Error('extract: page not ready (credential missing)');
    }
    let idToken = '';
    try {
      idToken = await credential.getIDTokenAsync();
    } catch (e) {
      throw new Error('extract: session expired (re-log into Superhuman in Chrome)');
    }
    if (!idToken) {
      throw new Error('extract: session expired (empty id token)');
    }

    // ---- 3. Read _authData for the OAuth tokens + metadata ----
    const authData = credential._authData || {};
    const accountInfo = authData.accountInfo || {};
    const refreshToken = authData.refreshToken || '';
    const accessToken = authData.accessToken || '';
    const providerFromAuth = accountInfo.provider || provider;

    // ---- 4. Resolve userId / userExternalId ----
    // The "user_..." external ID lives in a few places across Superhuman
    // bundle versions. AnonymousProfiler is the most-stable surface; we
    // probe it first, then fall back to the activeAccount user.id, then
    // to anything credential exposes.
    let userExternalId = '';
    try {
      if (window.AnonymousProfiler && window.AnonymousProfiler._user && window.AnonymousProfiler._user._id) {
        userExternalId = window.AnonymousProfiler._user._id;
      }
    } catch (e) { /* swallow — fallback below */ }
    if (!userExternalId) {
      try {
        if (umbrella.activeAccount && umbrella.activeAccount.user && umbrella.activeAccount.user.id) {
          userExternalId = umbrella.activeAccount.user.id;
        }
      } catch (e) { /* swallow — fallback below */ }
    }
    if (!userExternalId && account.user && account.user.id) {
      userExternalId = account.user.id;
    }

    // userId is the full string we persist as-is. The plan calls this
    // "the upstream user_... external ID; store the full string."
    const userId = userExternalId || '';

    // ---- 5. Derive userPrefix ----
    // Chars 7-10 (zero-indexed, inclusive) of the userId AFTER stripping
    // the "user_" prefix. Example from the plan:
    //   user_11SzDPi4sKPTbHQRMQ -> strip -> 11SzDPi4sKPTbHQRMQ
    //                           -> chars 7-10 -> "4sKP"
    // If the userId is shorter than 11 chars after stripping, fall back
    // to whatever's there — the AI question_event_id format needs 4
    // chars but the upstream API will validate; we don't want to throw
    // here over a degenerate ID.
    let userPrefix = '';
    if (userId) {
      const stripped = userId.startsWith('user_') ? userId.slice(5) : userId;
      // substring(7, 11) gives indices 7, 8, 9, 10 — four chars.
      userPrefix = stripped.substring(7, 11);
    }

    // ---- 6. Resolve deviceId ----
    let deviceId = '';
    try {
      if (window.device && window.device.id) {
        deviceId = window.device.id;
      }
    } catch (e) { /* swallow */ }
    if (!deviceId) {
      // crypto.randomUUID exists in every Chrome >= 92 — we don't need
      // a polyfill, but guard for the test fixtures.
      if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
        deviceId = crypto.randomUUID();
      } else {
        // Last-resort pseudo-random — only hit by very old browsers,
        // which the user can't run Superhuman in anyway.
        deviceId = 'dev_' + Math.random().toString(36).slice(2) + Date.now().toString(36);
      }
    }

    // ---- 7. Resolve expiry timestamps ----
    // Firebase id tokens are 1 hour. The auth object usually carries an
    // explicit expirationTimestamp; if not, we conservatively set
    // now + 3600s so the refresh path fires before the server rejects.
    let idTokenExpires = 0;
    if (typeof authData.expirationTimestamp === 'number' && isFinite(authData.expirationTimestamp)) {
      idTokenExpires = authData.expirationTimestamp;
    } else if (typeof authData.expires === 'number' && isFinite(authData.expires)) {
      idTokenExpires = authData.expires;
    }
    if (!idTokenExpires) {
      idTokenExpires = Date.now() + 3600 * 1000;
    }
    // accessTokenExpires is provider-dependent (Google: 1h, Microsoft:
    // varies). Use authData.accessTokenExpires when present, else
    // mirror idTokenExpires as a safe default.
    let accessTokenExpires = 0;
    if (typeof authData.accessTokenExpires === 'number' && isFinite(authData.accessTokenExpires)) {
      accessTokenExpires = authData.accessTokenExpires;
    } else if (typeof authData.accessTokenExpirationTimestamp === 'number' && isFinite(authData.accessTokenExpirationTimestamp)) {
      accessTokenExpires = authData.accessTokenExpirationTimestamp;
    }
    if (!accessTokenExpires) {
      accessTokenExpires = idTokenExpires;
    }

    return {
      email: email,
      idToken: idToken,
      idTokenExpires: idTokenExpires,
      refreshToken: refreshToken,
      accessToken: accessToken,
      accessTokenExpires: accessTokenExpires,
      userId: userId,
      userPrefix: userPrefix,
      userExternalId: userExternalId,
      deviceId: deviceId,
      provider: providerFromAuth,
    };
  } catch (e) {
    // Re-throw with the prefix preserved so the Go side's exception
    // text matching stays deterministic.
    if (e && e.message) {
      throw e;
    }
    throw new Error('extract: unexpected error: ' + String(e));
  }
})()`

// IIFE returns the JavaScript source of the extract IIFE. Exposed so
// debugging tooling (and the doctor command) can dump the exact source
// without reaching into package internals.
func IIFE() string {
	return extractIIFE
}

// buildIIFE wraps the raw IIFE source with an optional email pin. When
// email is non-empty, the IIFE prefers that account; otherwise it uses
// activeAccountEmail. Defining __email at the top of the evaluated
// expression keeps the IIFE source itself parametric without needing a
// template-string interpolation step on the Go side.
func buildIIFE(email string) string {
	if email == "" {
		return "var __email = '';\n" + extractIIFE
	}
	// JSON-encode the email so quotes and unicode in the rare
	// internationalized address survive the round-trip.
	enc, err := json.Marshal(email)
	if err != nil {
		// Marshal of a plain string never fails; fall back to empty so
		// the extract still runs.
		return "var __email = '';\n" + extractIIFE
	}
	return "var __email = " + string(enc) + ";\n" + extractIIFE
}

// Parse decodes the raw value returned by Runtime.evaluate (the bytes
// pulled out of result.result.value) into ExtractedTokens.
//
// Validation: the IIFE guarantees these fields are present on the
// happy path, but we still check the load-bearing ones explicitly so a
// silent Superhuman bundle change shows up as a clear "missing
// idToken" error instead of a downstream HTTP 401.
//
// Defensive defaults: a missing or malformed idTokenExpires falls back
// to now + 1h so the refresh-and-retry path (U4/U5) fires at the right
// time. The plan's test scenario #5 asserts this exact behavior.
func Parse(raw json.RawMessage) (*ExtractedTokens, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("extract: empty response from page")
	}
	var t ExtractedTokens
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("extract: decode page response: %w", err)
	}
	if t.IDToken == "" {
		return nil, fmt.Errorf("extract: page returned no idToken (bundle drift?)")
	}
	if t.Email == "" {
		return nil, fmt.Errorf("extract: page returned no email (bundle drift?)")
	}
	// Defensive default: epoch-ms timestamps under ~Jan 2 1970 are
	// almost certainly a bundle bug or missing field. Snap to
	// now + 1h so the refresh loop converges instead of thrashing.
	if t.IDTokenExpires <= 0 {
		t.IDTokenExpires = time.Now().Add(time.Hour).UnixMilli()
	}
	if t.AccessTokenExpires <= 0 {
		t.AccessTokenExpires = t.IDTokenExpires
	}
	if t.Provider == "" {
		// Default to google — the most common case — so downstream
		// code never has to handle empty-provider.
		t.Provider = "google"
	}
	return &t, nil
}

// ExtractFromTab runs the IIFE against a specific tab and parses the
// result. Lower-level than Extract: useful when the caller has already
// resolved the tab list and picked one.
//
// Errors classified:
//   - ErrSessionExpired when the IIFE-thrown "session expired" message
//     is observed in the CDP exception text.
//   - ErrPageNotReady when the IIFE throws "page not ready" OR when the
//     underlying Evaluate returns ErrPageNotReady directly (e.g., a
//     ReferenceError because window.GoogleAccount itself is missing).
//   - Any other Evaluate error wrapped as "extract: %w".
func ExtractFromTab(ctx context.Context, c *CDPClient, tab Tab) (*ExtractedTokens, error) {
	if c == nil {
		return nil, fmt.Errorf("extract: nil CDPClient")
	}
	if tab.WebSocketDebuggerURL == "" {
		return nil, fmt.Errorf("extract: tab has no webSocketDebuggerUrl")
	}

	expression := buildIIFE("")
	raw, err := c.Evaluate(ctx, tab.WebSocketDebuggerURL, expression)
	if err != nil {
		// Inspect the wrapped error message for the IIFE's typed-string
		// signals. We can't use errors.Is for the IIFE's internal sentinels
		// because they're JS-side; matching the message is the contract
		// the IIFE and Parse share.
		msg := err.Error()
		if errors.Is(err, ErrPageNotReady) {
			// Distinguish session-expired from generic page-not-ready by
			// scanning the message body, then re-wrap with the more
			// specific sentinel.
			if containsSessionExpired(msg) {
				return nil, fmt.Errorf("extract: %w", ErrSessionExpired)
			}
			return nil, fmt.Errorf("extract: %w", ErrPageNotReady)
		}
		return nil, fmt.Errorf("extract: %w", err)
	}

	return Parse(raw)
}

// Extract is the top-level orchestration: discover port, list tabs,
// find a Superhuman tab, run the IIFE.
//
// When email == "", picks the active account's tab. When multiple tabs
// match and email == "", returns ErrMultipleAccounts so the caller can
// re-invoke with the right pin.
func Extract(ctx context.Context, c *CDPClient, email string) (*ExtractedTokens, error) {
	if c == nil {
		return nil, fmt.Errorf("extract: nil CDPClient")
	}
	port, err := c.DiscoverPort(ctx)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	tabs, err := c.ListTabs(ctx, port)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	sh := FilterSuperhumanTabs(tabs)
	if len(sh) == 0 {
		return nil, fmt.Errorf("extract: %w", ErrNoSuperhumanTab)
	}

	tab, err := pickTab(sh, email)
	if err != nil {
		return nil, err
	}
	return ExtractFromTab(ctx, c, tab)
}

// pickTab selects the right Tab based on the user's email pin.
//
//   - email == "" + single tab -> that tab
//   - email == "" + multiple tabs -> ErrMultipleAccounts (with emails)
//   - email != "" -> first tab whose URL contains the email, or error
//     listing what we found.
//
// Superhuman tab URLs follow https://mail.superhuman.com/<email>/...
// so a substring match on the URL is reliable enough for routing.
// The CDP IIFE re-validates the account identity from inside the page,
// so a mismatch here only delays the real error by one round trip.
func pickTab(tabs []Tab, email string) (Tab, error) {
	if email == "" {
		if len(tabs) == 1 {
			return tabs[0], nil
		}
		emails := emailsFromTabs(tabs)
		return Tab{}, fmt.Errorf("%w: found %v", ErrMultipleAccounts, emails)
	}
	for _, t := range tabs {
		if tabMatchesEmail(t, email) {
			return t, nil
		}
	}
	emails := emailsFromTabs(tabs)
	return Tab{}, fmt.Errorf("extract: account %q not found in open tabs; available: %v", email, emails)
}

// tabMatchesEmail reports whether a tab's URL appears to belong to the
// given account. Superhuman tabs look like
// https://mail.superhuman.com/<email>/... so a substring match catches
// every routing variant (threads, drafts, settings, etc.).
func tabMatchesEmail(t Tab, email string) bool {
	return email != "" && len(t.URL) > 0 && containsEmail(t.URL, email)
}

// containsEmail and containsSessionExpired are tiny indirections so the
// test file can override or extend them. Kept package-private and
// inlined-style to avoid a strings.Contains import on every call site.
func containsEmail(haystack, needle string) bool {
	return indexOf(haystack, needle) >= 0
}

func containsSessionExpired(msg string) bool {
	return indexOf(msg, "session expired") >= 0
}

// indexOf is a tiny stand-in for strings.Contains that doesn't need
// the strings import (we deliberately keep this file's imports lean).
func indexOf(s, sub string) int {
	if sub == "" {
		return 0
	}
	n, m := len(s), len(sub)
	if m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// emailsFromTabs extracts the email segment from each Superhuman tab
// URL. Used to produce actionable error messages — never required by
// the happy path.
func emailsFromTabs(tabs []Tab) []string {
	out := make([]string, 0, len(tabs))
	for _, t := range tabs {
		if e := emailFromURL(t.URL); e != "" {
			out = append(out, e)
		}
	}
	return out
}

// emailFromURL pulls the <email> segment out of
// https://mail.superhuman.com/<email>/... Returns "" if the URL doesn't
// match the expected shape.
func emailFromURL(u string) string {
	const prefix = "https://mail.superhuman.com/"
	if len(u) <= len(prefix) {
		return ""
	}
	if u[:len(prefix)] != prefix {
		return ""
	}
	rest := u[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' || rest[i] == '?' || rest[i] == '#' {
			return rest[:i]
		}
	}
	return rest
}
