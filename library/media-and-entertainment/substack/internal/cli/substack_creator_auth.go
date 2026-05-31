// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: custom-domain Creator-endpoint auth capture. Not generator-emitted.

package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/client"
)

// validPublication reports whether s is a single DNS label safe to interpolate
// into the {pub}.substack.com Creator host. A publication is always one
// subdomain label; rejecting punctuation (/, @, :, ., %, whitespace, …) stops a
// crafted SUBSTACK_PUBLICATION from steering the host — e.g. "x.evil.com/" would
// otherwise parse to host "x.evil.com" and route the authenticated connect.sid
// request off-platform.
func validPublication(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-' && i != 0 && i != len(s)-1:
		default:
			return false
		}
	}
	return true
}

// captureCreatorDomainCookie supplements the global .substack.com session
// (substack.sid) with the publication's Creator-session cookie (connect.sid).
// Substack stores connect.sid on the publication's Creator host — for a
// custom-domain publication that is the custom domain (e.g. trevinsays.com),
// reached when {pub}.substack.com 301-redirects there. Creator endpoints
// (drafts, scheduled_release) require connect.sid on that host and return 403
// without it. We write connect.sid into the cookie jar scoped to the resolved
// host; the stdlib http.Client then carries it across the 301 hop on every
// Creator request automatically — no per-endpoint routing change needed.
//
// Best-effort: any failure (no/invalid publication, no redirect, no connect.sid
// in the browser jar) leaves the global session untouched and apex endpoints
// working. Only Creator endpoints depend on this. tool may be the zero value
// (e.g. the press-auth path resolved no tool); we detect one in that case so
// the capture still runs.
func captureCreatorDomainCookie(w io.Writer, tool cookieTool, profileDir string) {
	pub := strings.TrimSpace(os.Getenv("SUBSTACK_PUBLICATION"))
	if !validPublication(pub) {
		return
	}
	if tool.name == "" {
		detected, err := detectCookieTool()
		if err != nil {
			return
		}
		tool = detected
	}
	host := resolveCreatorHost(w, pub)
	if host == "" || host == "substack.com" || host == pub+".substack.com" {
		// No custom-domain redirect: connect.sid (if any) shares the
		// .substack.com jar already captured above; nothing extra to do.
		return
	}
	raw, err := extractCookies(tool, host, profileDir)
	if err != nil || raw == "" {
		return
	}
	v, ok := parseCookieString(raw)["connect.sid"]
	if !ok || v == "" {
		return
	}
	if err := client.WriteCookieJarFromMap(host, map[string]string{"connect.sid": v}); err != nil {
		fmt.Fprintf(w, "warning: persisting Creator-session cookie for %s: %v\n", host, err)
		return
	}
	fmt.Fprintf(w, "OK Captured Creator session for %s (publication %q)\n", host, pub)
}

// resolveCreatorHost returns the host serving the publication's Creator API.
// It HEADs a Creator endpoint on the hardcoded https://{pub}.substack.com and
// follows a single redirect: custom-domain publications 301 to their domain;
// others stay on {pub}.substack.com. pub is assumed validated by
// validPublication, so the base host cannot be hijacked. The redirect target is
// trusted only when it is a 3xx with an HTTPS Location and a non-empty host, so
// a non-redirect Location, a plaintext downgrade, or a protocol-relative
// //evil.com cannot point the Creator cookie off-platform. A forged HTTPS
// Location still requires a Substack-origin compromise or TLS MITM — residual
// and out of scope for a local cookie-reuse CLI.
func resolveCreatorHost(w io.Writer, pub string) string {
	sub := pub + ".substack.com"
	httpc := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := httpc.Head("https://" + sub + "/api/v1/drafts")
	if err != nil {
		fmt.Fprintf(w, "warning: probing Creator host for %q failed: %v (Creator endpoints may 403; re-run auth login when connected)\n", pub, err)
		return sub
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return sub
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return sub
	}
	u, err := url.Parse(loc)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return sub
	}
	return u.Host
}
