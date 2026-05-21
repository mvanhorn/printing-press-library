// PATCH(crawl-stats: read Google session cookies from a Netscape-format
// cookie jar). The Crawl Stats batchexecute endpoint requires the seven
// Google web session cookies (SAPISID, __Secure-1PSID, __Secure-3PSID, SID,
// HSID, SSID, APISID). Users export them from Chrome via an extension and
// point the CLI at the jar via GSC_COOKIE_JAR or the `auth login --chrome`
// flow. See .printing-press-patches.json patch id: crawl-stats.

package client

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// requiredCrawlStatsCookies lists the seven cookies the batchexecute endpoint
// requires. SAPISID is special — its value is also fed into SAPISIDHash().
var requiredCrawlStatsCookies = []string{
	"SAPISID",
	"__Secure-1PSID",
	"__Secure-3PSID",
	"SID",
	"HSID",
	"SSID",
	"APISID",
}

// CookieJarFile carries parsed Google session cookies plus a marker for
// which ones came from the jar (so the client can fail-fast with a clear
// "missing cookies" error rather than silently posting an unauthenticated
// request that gets 401-back-redirected to the login page).
type CookieJarFile struct {
	Path    string
	Cookies map[string]*http.Cookie
}

// LoadNetscapeCookieJar reads a Netscape-format cookie jar (the .cookies.txt
// shape Chrome/Firefox/curl all share) and returns the subset matching
// requiredCrawlStatsCookies. Format reference:
//
//	# Netscape HTTP Cookie File
//	# domain	include_subdomains	path	secure	expiry	name	value
//	.google.com	TRUE	/	TRUE	0	SAPISID	xxxxx
//
// Lines starting with '#' (and blank lines) are skipped. Fields are
// tab-separated. Cookies whose names aren't in requiredCrawlStatsCookies are
// ignored to keep the in-memory map lean.
func LoadNetscapeCookieJar(path string) (*CookieJarFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening cookie jar %s: %w", path, err)
	}
	defer f.Close()

	required := make(map[string]bool, len(requiredCrawlStatsCookies))
	for _, name := range requiredCrawlStatsCookies {
		required[name] = true
	}

	out := &CookieJarFile{
		Path:    path,
		Cookies: make(map[string]*http.Cookie),
	}

	scan := bufio.NewScanner(f)
	// Cookie values can exceed bufio's default 64 KiB line cap for some
	// browsers' session cookies; raise to 1 MiB which is well beyond any
	// realistic cookie size.
	scan.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scan.Scan() {
		line := scan.Text()
		// Allow optional "#HttpOnly_" prefix (Firefox/curl convention) on the
		// domain field so HttpOnly cookies parse identically to non-HttpOnly.
		raw := strings.TrimPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(raw, "#") || strings.TrimSpace(raw) == "" {
			continue
		}
		fields := strings.Split(raw, "\t")
		if len(fields) < 7 {
			continue
		}
		domain := fields[0]
		path := fields[2]
		secure := strings.EqualFold(fields[3], "TRUE")
		expiry, _ := strconv.ParseInt(fields[4], 10, 64)
		name := fields[5]
		value := fields[6]

		if !required[name] {
			continue
		}

		c := &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: strings.TrimPrefix(domain, "."),
			Path:   path,
			Secure: secure,
		}
		if expiry > 0 {
			c.Expires = time.Unix(expiry, 0)
		}
		// Take the first occurrence per name. Cookie jars sometimes contain
		// duplicates (different domains / paths); the first-write-wins
		// behavior keeps the parse deterministic.
		if _, exists := out.Cookies[name]; !exists {
			out.Cookies[name] = c
		}
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("scanning cookie jar %s: %w", path, err)
	}
	return out, nil
}

// MissingCookies returns the subset of requiredCrawlStatsCookies absent from
// the jar. An empty slice means the jar is fully populated and ready for
// crawl-stats requests.
func (j *CookieJarFile) MissingCookies() []string {
	if j == nil {
		return append([]string{}, requiredCrawlStatsCookies...)
	}
	missing := []string{}
	for _, name := range requiredCrawlStatsCookies {
		if _, ok := j.Cookies[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// SAPISIDValue returns the SAPISID cookie value (needed for SAPISIDHash) or
// an empty string when missing.
func (j *CookieJarFile) SAPISIDValue() string {
	if j == nil {
		return ""
	}
	if c, ok := j.Cookies["SAPISID"]; ok {
		return c.Value
	}
	return ""
}

// CookieHeader formats the seven cookies as a single "Cookie:" header value
// (name1=value1; name2=value2; ...). Order is stable
// (requiredCrawlStatsCookies order) so the header value is deterministic
// across calls — useful for tests and for any future request-signing layer.
func (j *CookieJarFile) CookieHeader() string {
	if j == nil {
		return ""
	}
	parts := make([]string, 0, len(requiredCrawlStatsCookies))
	for _, name := range requiredCrawlStatsCookies {
		if c, ok := j.Cookies[name]; ok {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}
