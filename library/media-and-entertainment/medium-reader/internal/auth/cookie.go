// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

// Package auth loads the optional, user-supplied Medium session cookie that
// unlocks Tier-1 surfaces (member full bodies on the read path, and optionally
// the GraphQL path). It is a CONVENIENCE layer: every path is optional, and with
// nothing configured the loader returns a zero source.Cookies so the transport
// runs fully anonymous (Tier 0), which Gate 0 validated clears Cloudflare on its
// own via Chrome impersonation.
//
// Fallback chain (first hit wins, all optional):
//  1. MEDIUM_SESSION env — a raw cookie-header fragment ("sid=..; uid=..") or
//     just the bare sid value.
//  2. Cookie file — flat JSON {"sid":"..","uid":"..","cf_clearance":".."},
//     pointed at by the --cookie-file flag (Options.CookieFile) or the
//     MEDIUM_COOKIE_FILE env var. The explicit flag path wins over the env path.
//  3. kooky/Chrome auto-extract — DELIBERATELY not a compiled dependency.
//
// Why kooky is not a dependency: on macOS, reading Chrome's cookie store
// requires decrypting it with a key from the login Keychain, which triggers a
// per-binary GUI authorization prompt that is NOT pre-granted and cannot be
// satisfied in a headless/CI/test run. Pulling kooky in would also add a
// transitive dependency tree (CGO Keychain bindings) that can break a clean
// `go build ./...` on machines without the right toolchain. The $0/no-network,
// deterministic-build discipline of v2 is worth more than the convenience of
// auto-extraction, so the Chrome path is a clearly-messaged stub: it tells the
// user exactly how to supply a cookie via the env or file path instead. The two
// supported paths cover every real use case (paste once into the env, or save a
// small JSON file).
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// Env var names. Kept here (not inlined at call sites) so the loader and any
// help text refer to the same source of truth.
const (
	// EnvSession is the raw-cookie-fragment env var ("sid=..; uid=..") or bare
	// sid value.
	EnvSession = "MEDIUM_SESSION"
	// EnvCookieFile points at a flat-JSON cookie file (the --cookie-file flag
	// takes precedence over this when both are set).
	EnvCookieFile = "MEDIUM_COOKIE_FILE"
)

// Options carries the explicit, flag-supplied inputs to Load. CookieFile is the
// --cookie-file value (empty when the flag was not passed); it takes precedence
// over the MEDIUM_COOKIE_FILE env var.
type Options struct {
	CookieFile string
}

// Load resolves the Tier-1 cookie via the fallback chain, returning the first
// non-empty source. A zero Cookies with a nil error means "nothing configured"
// (run anonymous Tier 0) — that is the normal, expected default, never an error.
//
// The only error case is a cookie file that was explicitly configured (via the
// flag or the env path) but could not be read or parsed: that is a user mistake
// worth surfacing, not silently swallowing into an anonymous fallback.
func Load(opts Options) (source.Cookies, error) {
	// 1. MEDIUM_SESSION env (raw fragment or bare sid).
	if raw := strings.TrimSpace(os.Getenv(EnvSession)); raw != "" {
		if c := ParseSessionEnv(raw); !c.IsZero() {
			return c, nil
		}
	}

	// 2. Cookie file: explicit flag path beats the env-supplied path.
	filePath := strings.TrimSpace(opts.CookieFile)
	if filePath == "" {
		filePath = strings.TrimSpace(os.Getenv(EnvCookieFile))
	}
	if filePath != "" {
		c, err := ParseCookieFile(filePath)
		if err != nil {
			return source.Cookies{}, fmt.Errorf("loading cookie file %q: %w", filePath, err)
		}
		if !c.IsZero() {
			return c, nil
		}
	}

	// 3. kooky/Chrome auto-extract is not compiled in (see package doc). Nothing
	//    configured => anonymous Tier 0, which is a valid, key-free default.
	return source.Cookies{}, nil
}

// ErrChromeExtractUnavailable is returned by ExtractFromChrome to explain that
// auto-extraction is not built in and point the user at the supported paths. It
// is a typed value so the auth command can present it without re-deriving the
// message.
var ErrChromeExtractUnavailable = errors.New(
	"Chrome cookie auto-extraction is not built into this binary " +
		"(it would require a macOS Keychain authorization prompt and an extra " +
		"dependency). Import a cookie another way:\n" +
		"  • export MEDIUM_SESSION=\"sid=<your-sid>; uid=<your-uid>\"\n" +
		"  • or save a JSON file {\"sid\":\"<sid>\",\"uid\":\"<uid>\"} and pass --cookie-file <path> " +
		"(or set MEDIUM_COOKIE_FILE)\n" +
		"You can copy sid/uid from your browser's medium.com cookies (DevTools → Application → Cookies).")

// ExtractFromChrome reads the Medium session cookie directly from a local Chrome
// profile. It is split across two build-tagged files: the default build
// (cookie_chrome_stub.go, //go:build !kooky) returns ErrChromeExtractUnavailable
// with actionable guidance, keeping the shipped binary pure-Go and free of the
// kooky dependency; a `-tags kooky` build (cookie_chrome_kooky.go) provides the
// real extraction. Both call into the shared, hermetically tested
// selectMediumCookies selector below.

// ErrNoMediumCookies is returned when a browser cookie jar holds no medium.com
// session cookie. sid is the load-bearing session token; without it there is no
// Tier-1 session to import (uid/cf_clearance are supplementary).
var ErrNoMediumCookies = errors.New("no medium.com session cookie found in the browser (open and sign in to medium.com in Chrome, then retry)")

// BrowserCookie is a minimal, dependency-free view of a single browser cookie.
// It is the seam between the build-tagged kooky extractor (which converts
// kooky.Cookie values into this shape) and selectMediumCookies (the pure,
// hermetically tested selection logic). Keeping kooky's types out of this
// signature is what lets the selection be tested without kooky or a real
// Chrome/Keychain.
type BrowserCookie struct {
	Name   string
	Value  string
	Domain string
}

// selectMediumCookies picks the Medium session cookies (sid, uid, cf_clearance)
// out of a browser cookie jar, ignoring cookies set for any other domain. The
// first non-empty value wins per field. It returns ErrNoMediumCookies when no
// sid is found on medium.com.
func selectMediumCookies(cookies []BrowserCookie) (source.Cookies, error) {
	var c source.Cookies
	for _, ck := range cookies {
		if !isMediumDomain(ck.Domain) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(ck.Name)) {
		case "sid":
			if c.Sid == "" {
				c.Sid = strings.TrimSpace(ck.Value)
			}
		case "uid":
			if c.Uid == "" {
				c.Uid = strings.TrimSpace(ck.Value)
			}
		case "cf_clearance":
			if c.CfClearance == "" {
				c.CfClearance = strings.TrimSpace(ck.Value)
			}
		}
	}
	if c.Sid == "" {
		return source.Cookies{}, ErrNoMediumCookies
	}
	return c, nil
}

// isMediumDomain reports whether a cookie domain belongs to medium.com — either
// the bare apex ("medium.com") or any subdomain. The leading dot that browsers
// use for domain cookies is tolerated.
func isMediumDomain(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return d == "medium.com" || strings.HasSuffix(d, ".medium.com")
}

// ParseSessionEnv parses a MEDIUM_SESSION value into a source.Cookies.
//
// Two accepted shapes:
//   - A cookie-header fragment: "sid=..; uid=..; cf_clearance=..". Pairs are
//     split on ';', each on the first '='; recognized names (sid, uid,
//     cf_clearance) map to the matching field. Unknown names are ignored.
//   - A bare value with no '=': treated as the sid token (the common
//     copy-just-the-session-id case).
//
// Surrounding single/double quotes on a value are stripped (a frequent
// shell-paste artifact). Empty/whitespace input yields a zero Cookies.
func ParseSessionEnv(raw string) source.Cookies {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return source.Cookies{}
	}
	// Bare sid: no key=value structure at all.
	if !strings.Contains(raw, "=") {
		return source.Cookies{Sid: unquote(raw)}
	}
	var c source.Cookies
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(pair[:eq])
		val := unquote(strings.TrimSpace(pair[eq+1:]))
		switch strings.ToLower(name) {
		case "sid":
			c.Sid = val
		case "uid":
			c.Uid = val
		case "cf_clearance":
			c.CfClearance = val
		}
	}
	return c
}

// cookieFileShape is the flat JSON cookie-file schema. All fields optional.
type cookieFileShape struct {
	Sid         string `json:"sid"`
	Uid         string `json:"uid"`
	CfClearance string `json:"cf_clearance"`
}

// ParseCookieFile reads and parses a flat-JSON cookie file into a source.Cookies.
//
// An empty path returns a zero Cookies and a nil error (nothing configured). A
// non-empty path that cannot be read or whose JSON does not parse returns an
// error — when a file is explicitly named, a problem with it should be visible,
// not silently downgraded to anonymous.
func ParseCookieFile(path string) (source.Cookies, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return source.Cookies{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return source.Cookies{}, fmt.Errorf("reading cookie file: %w", err)
	}
	// Warn (non-fatally) when the cookie file is group/other-accessible. The
	// file holds the user's live Medium session, so 0600 is the right posture;
	// a looser mode is a leak worth flagging. This is only a hint — cookies
	// must never block a command, so a bad/loose mode degrades to a warning,
	// not an error. The check is meaningless on Windows (no POSIX mode bits),
	// so we skip it there.
	if runtime.GOOS != "windows" {
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(os.Stderr, "warning: cookie file %q is readable by group/others (mode %#o); run 'chmod 600 %s' to protect your Medium session\n", path, info.Mode().Perm(), path)
		}
	}
	var shape cookieFileShape
	if err := json.Unmarshal(data, &shape); err != nil {
		return source.Cookies{}, fmt.Errorf("parsing cookie file JSON (expected a flat object like {\"sid\":\"..\",\"uid\":\"..\"}): %w", err)
	}
	return source.Cookies{
		Sid:         strings.TrimSpace(shape.Sid),
		Uid:         strings.TrimSpace(shape.Uid),
		CfClearance: strings.TrimSpace(shape.CfClearance),
	}, nil
}

// WriteCookieFile writes c as the same flat-JSON shape ParseCookieFile reads
// ({"sid":"..","uid":"..","cf_clearance":".."}), creating parent directories as
// needed and restricting the file to 0600. It holds a live Medium session, so it
// must never be group/other-readable. Used by `auth login --chrome` to persist
// the cookie it extracts from the browser without ever printing the raw token.
func WriteCookieFile(path string, c source.Cookies) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("cookie file path is empty")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating cookie file dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(cookieFileShape{
		Sid:         c.Sid,
		Uid:         c.Uid,
		CfClearance: c.CfClearance,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cookie file: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing cookie file: %w", err)
	}
	return nil
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
