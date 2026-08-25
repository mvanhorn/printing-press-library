// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Tier-1 session loading for the Substack Reader. Substack's entitlement
// decision is server-side, keyed on the session cookie you present, so
// unlocking paid posts you already subscribe to needs the user's OWN
// `substack.sid` cookie — nothing else. This mirrors Medium Reader's cookie
// hygiene (env/file precedence, 0600 file, masked in every diagnostic, warn on
// loose perms) but is far simpler: the attended crack proved `substack.sid`
// ALONE authenticates every subscription (account-level, not per-publication),
// so there is no uid/connect.sid to juggle.
//
// Every path is optional. With nothing configured LoadSession returns a zero
// Session and the client runs fully anonymous (Tier 0) — free posts still work
// keyless. Tier 1 is strictly opt-in and never redistributed.
package substack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cliutil"
)

// Env var names. Kept here (not inlined at call sites) so the loader and any
// help text refer to the same source of truth. The SUBSTACK_ prefix matches the
// path env vars in cliutil (SUBSTACK_CONFIG_DIR, ...).
const (
	// EnvSession carries the raw session value: either a bare substack.sid
	// value ("s%3A...") or a cookie-header fragment ("substack.sid=s%3A...").
	EnvSession = "SUBSTACK_SESSION"
	// EnvCookieFile points at a flat-JSON cookie file; the env path is a
	// fallback below the default config-dir file only if that file is absent.
	EnvCookieFile = "SUBSTACK_COOKIE_FILE"
	// cookieFileName is the default cookie file basename inside the config dir.
	cookieFileName = "cookie.json"
	// cookieName is the single load-bearing cookie. Sent verbatim as
	// `Cookie: substack.sid=<value>` in its stored URL-encoded s%3A… form.
	cookieName = "substack.sid"
)

// Session carries the optional, user-supplied Substack session material. Only
// substack.sid is needed (the crack proved connect.sid/substack.lli are not
// required). The SID is stored and sent verbatim in its URL-encoded s%3A… form.
type Session struct {
	SID string
}

// IsZero reports whether no session material is present — i.e. the client runs
// anonymous Tier 0.
func (s Session) IsZero() bool { return strings.TrimSpace(s.SID) == "" }

// CookieHeader renders the session as a single Cookie request-header value.
// Returns "" when no session is set, so callers can guard with an emptiness
// check before attaching.
func (s Session) CookieHeader() string {
	sid := strings.TrimSpace(s.SID)
	if sid == "" {
		return ""
	}
	return cookieName + "=" + sid
}

// Masked returns a presence-revealing, value-hiding rendering of the SID for
// diagnostics — never log or print the raw token. Shows a short head so the
// user can sanity-check which cookie is loaded without exposing it.
func (s Session) Masked() string {
	return MaskToken(s.SID)
}

// MaskToken masks a secret to "abcdef…(NN chars)" — enough to recognise, not
// enough to reuse. An empty token renders as "<none>".
func MaskToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "<none>"
	}
	// Suppress the head entirely for short tokens so a small secret is never
	// disclosed whole or in majority; reveal a 6-char head only once the token is
	// long enough that 6 chars is a small fraction of it. (A real substack.sid is
	// 100+ chars, so in practice this only hardens the general-purpose contract.)
	if len(tok) <= 12 {
		return fmt.Sprintf("…(%d chars)", len(tok))
	}
	return fmt.Sprintf("%s…(%d chars)", tok[:6], len(tok))
}

// DefaultCookieFilePath returns the config-dir cookie file path
// (<config>/cookie.json), the standard place to persist a Substack session.
func DefaultCookieFilePath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cookieFileName), nil
}

// LoadSession resolves the Tier-1 session via the precedence chain, returning
// the first non-empty source. A zero Session with a nil error means "nothing
// configured" (run anonymous Tier 0) — the normal, expected default.
//
// Precedence (first hit wins, all optional):
//  1. SUBSTACK_SESSION env — bare sid or "substack.sid=<value>" fragment.
//  2. SUBSTACK_COOKIE_FILE env — an explicitly-pointed cookie file. This is
//     authoritative: when set it wins over the default and its result is final.
//     A missing, unreadable, unparseable, or SID-less explicit file is a hard
//     error (surface the user's mistake), never a silent fall-through.
//  3. Default config-dir cookie file (<config>/cookie.json) — best-effort:
//     absent, empty, SID-less, or corrupt all degrade to anonymous (a corrupt
//     default warns but never blocks). Only an explicit cookie errors, so a
//     non-nil LoadSession error always means "your explicit cookie failed."
func LoadSession() (Session, error) {
	if raw := strings.TrimSpace(os.Getenv(EnvSession)); raw != "" {
		if s := ParseSessionEnv(raw); !s.IsZero() {
			return s, nil
		}
		// The var is set but held no recognizable substack.sid (e.g. a value with
		// an '=' but no known cookie key, or an empty pair). Warn rather than
		// silently treat it as unset — a misconfigured SUBSTACK_SESSION otherwise
		// looks identical to "no session", and the user would wrongly conclude the
		// tool ignored their credential on purpose. Then fall through to the cookie
		// files / anonymous.
		fmt.Fprintf(os.Stderr, "warning: %s is set but no substack.sid could be parsed from it; ignoring it and falling back to the cookie file / anonymous. Set it to your substack.sid value or a \"substack.sid=<value>\" cookie fragment.\n", EnvSession)
	}

	// The explicit env-pointed file is authoritative: once SUBSTACK_COOKIE_FILE
	// is set, its result wins and the lookup stops here. A missing, unreadable,
	// unparseable, or SID-less explicit file is a hard error (surface the user's
	// mistake) — it never silently falls through to a stale default cookie.
	if envPath := strings.TrimSpace(os.Getenv(EnvCookieFile)); envPath != "" {
		s, err := ParseCookieFile(envPath)
		if err != nil {
			return Session{}, fmt.Errorf("loading cookie file %q (SUBSTACK_COOKIE_FILE): %w", envPath, err)
		}
		if s.IsZero() {
			return Session{}, fmt.Errorf("cookie file %q (SUBSTACK_COOKIE_FILE) has no substack.sid; unset SUBSTACK_COOKIE_FILE to read anonymously", envPath)
		}
		return s, nil
	}

	// Default config-dir cookie file — best-effort. Absent, empty, SID-less, or
	// even corrupt all degrade to anonymous: a broken *default* cookie must never
	// block a read (a caller who wants authoritative behavior sets an explicit
	// SUBSTACK_COOKIE_FILE). A corrupt default is surfaced as a warning, not an
	// error — so a non-nil LoadSession error always means an explicit cookie
	// failed, which the caller can propagate unconditionally.
	if def, err := DefaultCookieFilePath(); err == nil && def != "" {
		// #nosec G703 -- the user's own default cookie file path.
		if _, statErr := os.Stat(def); statErr == nil {
			if s, err := ParseCookieFile(def); err != nil {
				fmt.Fprintf(os.Stderr, "warning: ignoring unreadable default cookie file %q: %v\n", def, err)
			} else if !s.IsZero() {
				return s, nil
			}
		}
	}

	return Session{}, nil
}

// ParseSessionEnv parses a SUBSTACK_SESSION value into a Session. Accepts a
// bare sid value, or a cookie-header fragment where the substack.sid pair is
// picked out (other pairs ignored). Surrounding quotes (a shell-paste artifact)
// are stripped.
func ParseSessionEnv(raw string) Session {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Session{}
	}
	if !strings.Contains(raw, "=") {
		return Session{SID: unquote(raw)}
	}
	var sid string
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(pair[:eq])
		val := unquote(strings.TrimSpace(pair[eq+1:]))
		if strings.EqualFold(name, cookieName) || strings.EqualFold(name, "sid") {
			sid = val
		}
	}
	return Session{SID: sid}
}

// cookieFileShape is the flat-JSON cookie-file schema. Both keys are accepted;
// "substack.sid" is canonical, "sid" is a convenience alias.
type cookieFileShape struct {
	SubstackSID string `json:"substack.sid"`
	SID         string `json:"sid"`
}

// ParseCookieFile reads and parses a flat-JSON cookie file into a Session. An
// empty path returns a zero Session, nil. A non-empty path that cannot be read
// or parsed returns an error — when a file is explicitly present, a problem
// with it should be visible, not silently downgraded to anonymous.
//
// Warns (non-fatally) when the file is group/other-readable: it holds the
// user's live Substack session, so 0600 is the right posture. The check is
// meaningless on Windows (no POSIX mode bits), so it is skipped there.
func ParseCookieFile(path string) (Session, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Session{}, nil
	}
	// #nosec G304 G703 -- reads the user's OWN cookie file at a path they
	// supplied (config dir or SUBSTACK_COOKIE_FILE env); reading one's own
	// credential file is the whole point and is never attacker-controlled.
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("reading cookie file: %w", err)
	}
	if runtime.GOOS != "windows" {
		// #nosec G703 -- same user-owned cookie file path as the ReadFile above.
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: cookie file %q is readable by group/others (mode %#o); run 'chmod 600 %s' to protect your Substack session\n", path, info.Mode().Perm(), path)
		}
	}
	var shape cookieFileShape
	if err := json.Unmarshal(data, &shape); err != nil {
		return Session{}, fmt.Errorf("parsing cookie file JSON (expected a flat object like {\"substack.sid\":\"s%%3A..\"}): %w", err)
	}
	sid := strings.TrimSpace(shape.SubstackSID)
	if sid == "" {
		sid = strings.TrimSpace(shape.SID)
	}
	return Session{SID: sid}, nil
}

// WriteCookieFile writes s as the flat-JSON shape ParseCookieFile reads,
// atomically and restricted to 0600 (it holds a live Substack session, so it
// must never be group/other-readable). Reuses cliutil.AtomicWritePrivateFile
// so parent dirs are created 0700 and the write is crash-safe.
func WriteCookieFile(path string, s Session) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("cookie file path is empty")
	}
	data, err := json.MarshalIndent(cookieFileShape{SubstackSID: s.SID}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cookie file: %w", err)
	}
	return cliutil.AtomicWritePrivateFile(path, append(data, '\n'), 0o600, 0o700)
}

// unquote strips a single matching pair of surrounding single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
