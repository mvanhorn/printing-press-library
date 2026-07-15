// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: Jane has no token API. Patient auth is a Rails two-step form
// login (POST /sign_in twice) that yields a _front_desk_session cookie. This
// file performs that handshake with a throwaway cookie jar and returns the
// session cookie for storage in the clinic store. Credentials are never written
// to disk or logged; only the resulting session cookie is persisted.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

const janeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

var reAuthToken = regexp.MustCompile(`name="authenticity_token"[^>]*value="([^"]+)"`)

// janeLoginResult is the outcome of a successful two-step login.
type janeLoginResult struct {
	// Session is a cookie-jar string ("_front_desk_session=<value>") ready to
	// seed the HTTP client's jar for this clinic's host.
	Session string
}

// firstMatch returns the first capture group of re against s, or "".
func firstMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

var reHiddenAuthKey = regexp.MustCompile(`name=["']auth_key["'][^>]*\bvalue=["']([^"']+)["']`)

func janeTrace(w io.Writer, step string, status int, extra string) {
	if w == nil {
		return
	}
	if extra != "" {
		fmt.Fprintf(w, "  [debug] %-26s status=%d  %s\n", step, status, extra)
	} else {
		fmt.Fprintf(w, "  [debug] %-26s status=%d\n", step, status)
	}
}

// janeLogin performs Jane's two-step patient sign-in against baseURL and
// returns the authenticated session cookie. It fails with a clear error on bad
// credentials (the API stays on the password step / appointments still 401).
// When dbg is non-nil, it traces each HTTP step's status and redirect target —
// never any credential value — so login problems can be diagnosed in the field.
func janeLogin(ctx context.Context, baseURL, username, password string, timeout time.Duration, dbg io.Writer) (janeLoginResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return janeLoginResult{}, err
	}
	hc := &http.Client{
		Timeout: timeout,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	base := strings.TrimRight(baseURL, "/")

	// Step 0: GET /login to seed a session cookie and capture the CSRF token.
	s0, loginHTML, _, err := janeReq(ctx, hc, http.MethodGet, base+"/login", "", nil)
	if err != nil {
		return janeLoginResult{}, fmt.Errorf("loading login page: %w", err)
	}
	token := firstMatch(reAuthToken, loginHTML)
	janeTrace(dbg, "GET /login", s0, "token="+yn(token != ""))
	if token == "" {
		return janeLoginResult{}, fmt.Errorf("could not find login form on %s (is this a Jane clinic URL?)", base)
	}

	// Step 1: POST the username; Jane responds with the password form.
	s1, step1HTML, u1, err := janeReq(ctx, hc, http.MethodPost, base+"/sign_in", base+"/login", url.Values{
		"utf8":               {"✓"},
		"authenticity_token": {token},
		"auth_key":           {username},
	})
	if err != nil {
		return janeLoginResult{}, fmt.Errorf("submitting username: %w", err)
	}
	token2 := firstMatch(reAuthToken, step1HTML)
	if token2 == "" {
		token2 = token
	}
	// The password form echoes a hidden auth_key; submit exactly what it rendered.
	authKey := firstMatch(reHiddenAuthKey, step1HTML)
	if authKey == "" {
		authKey = username
	}
	hasPwForm := strings.Contains(step1HTML, "name='password'") || strings.Contains(step1HTML, `name="password"`)
	janeTrace(dbg, "POST /sign_in", s1, fmt.Sprintf("landed=%s pw_form=%s", shortURL(u1, base), yn(hasPwForm)))
	if !hasPwForm {
		return janeLoginResult{}, fmt.Errorf("username step did not return a password form (username may be unknown for %s)", base)
	}

	// Step 2: POST the password to /sessions to complete sign-in.
	s2, _, u2, err := janeReq(ctx, hc, http.MethodPost, base+"/sessions", base+"/sign_in", url.Values{
		"utf8":               {"✓"},
		"authenticity_token": {token2},
		"auth_key":           {authKey},
		"password":           {password},
	})
	if err != nil {
		return janeLoginResult{}, fmt.Errorf("submitting password: %w", err)
	}
	// The redirect target after /sessions is the real success signal: a
	// successful login lands on the account area; a failure bounces back to
	// /login or /sign_in.
	landed := shortURL(u2, base)
	janeTrace(dbg, "POST /sessions", s2, "landed="+landed)

	// Verify against the API.
	sv, vbody, _, err := janeReqAccept(ctx, hc, base+"/api/v2/appointments")
	if err != nil {
		return janeLoginResult{}, fmt.Errorf("verifying session: %w", err)
	}
	janeTrace(dbg, "GET /api/v2/appointments", sv, "body="+snippet(vbody))

	// Report cookie capture for diagnostics.
	u, _ := url.Parse(base)
	var session string
	for _, ck := range jar.Cookies(u) {
		if ck.Name == "_front_desk_session" {
			session = ck.Name + "=" + ck.Value
			break
		}
	}
	janeTrace(dbg, "session cookie", 0, "captured="+yn(session != ""))

	if strings.Contains(landed, "/auth/failure") {
		// Jane's identity (password) provider is gated behind reCAPTCHA, which a
		// CLI can't solve. This is by design, not a credential problem.
		return janeLoginResult{}, fmt.Errorf("Jane blocks command-line password login with reCAPTCHA (landed on /auth/failure).\n"+
			"Log in with your existing browser session instead:\n"+
			"  1. Log in to %s in Chrome (you already have).\n"+
			"  2. janeapp-pp-cli auth login --clinic <name> --chrome\n"+
			"     (or export the _front_desk_session cookie and use --cookies-file <file>)", base)
	}
	if sv == 401 || sv == 403 {
		return janeLoginResult{}, fmt.Errorf("login did not establish a session for %s (landed on %s)", base, landed)
	}
	if sv >= 400 {
		return janeLoginResult{}, fmt.Errorf("unexpected status %d verifying session against %s", sv, base)
	}
	if session == "" {
		return janeLoginResult{}, fmt.Errorf("login appeared to succeed but no session cookie was set by %s", base)
	}
	return janeLoginResult{Session: session}, nil
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func shortURL(full, base string) string {
	return strings.TrimPrefix(full, base)
}

// snippet returns a short, credential-free preview of a response body for debug.
func snippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 100 {
		s = s[:100]
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// janeReq issues a GET or form POST and returns status, body, and the final URL
// after redirects.
func janeReq(ctx context.Context, hc *http.Client, method, u, referer string, form url.Values) (int, string, string, error) {
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("User-Agent", janeUserAgent)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, "", "", err
	}
	finalURL := u
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return resp.StatusCode, string(b), finalURL, nil
}

func janeReqAccept(ctx context.Context, hc *http.Client, u string) (int, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("User-Agent", janeUserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	finalURL := u
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return resp.StatusCode, string(b), finalURL, nil
}

// readJanePassword resolves the patient password without ever putting it on the
// command line (visible in `ps`). Precedence: --password-stdin, then the
// JANEAPP_PASSWORD env var, then an interactive TTY prompt.
func readJanePassword(in io.Reader, out io.Writer, passwordStdin bool) (string, error) {
	if passwordStdin {
		data, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("reading password from stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	if v := os.Getenv("JANEAPP_PASSWORD"); v != "" {
		return v, nil
	}
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(out, "Password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(out)
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return "", fmt.Errorf("no password provided: set JANEAPP_PASSWORD, pass --password-stdin, or run in an interactive terminal")
}
