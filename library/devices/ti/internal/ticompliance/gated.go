// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package ticompliance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// opnPattern is the accepted charset for a TI orderable part number. It bounds
// both the JS-eval interpolation (injection) and the on-disk FMD filename
// (path traversal) surfaces.
var opnPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{2,64}$`)

// ValidateOPN rejects part numbers outside the safe charset. Slashes are
// allowed (some TI OPNs contain them) but "." segments that would traverse are
// rejected.
func ValidateOPN(opn string) error {
	o := strings.TrimSpace(opn)
	if !opnPattern.MatchString(o) {
		return fmt.Errorf("invalid part number %q (allowed: letters, digits, . _ - /)", opn)
	}
	for _, seg := range strings.Split(o, "/") {
		if seg == ".." || seg == "." {
			return fmt.Errorf("invalid part number %q", opn)
		}
	}
	return nil
}

// sleepCtx sleeps for d or returns ctx.Err() if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// This file is the login-gated lane: cookie-injected Browserbase access to
// TI's Material Content surfaces. The CLI NEVER performs a login — cookies
// come from a human-in-the-loop Browserbase login captured by the cowork
// agent (SupplierLoginManager deposit) and are passed in as a JSON snapshot.
// Ported from the agent repo's tiscrape package (the legacy TIExtractor path).

const (
	tiHomeURL    = "https://www.ti.com/materialcontent/home"
	searchURLFmt = "https://www.ti.com/materialcontent/en/search?partNumber=%s&partType=tiPartNumber"
)

// ErrAuthRedirect means the session landed on login.ti.com: the deposited
// cookies are stale or missing. Callers surface this as a distinct
// "login required" failure so the orchestrator can trigger a fresh HIL login.
var ErrAuthRedirect = errors.New("login required or cookies stale (redirected to login.ti.com)")

// CookieSpec mirrors the attributes `agent-browser cookies set` accepts and
// the cowork agent's cookie-deposit shape.
type CookieSpec struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
}

// LoadCookies reads a cookie snapshot file: either a bare JSON array of
// CookieSpec or an object with a "cookies" array (the deposit envelope).
// Cookies are filtered to ti.com domains.
func LoadCookies(path string) ([]CookieSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list []CookieSpec
	if err := json.Unmarshal(raw, &list); err != nil {
		var envelope struct {
			Cookies []CookieSpec `json:"cookies"`
		}
		if err2 := json.Unmarshal(raw, &envelope); err2 != nil || len(envelope.Cookies) == 0 {
			return nil, fmt.Errorf("cookie file %s: neither a cookie array nor a {cookies:[...]} envelope", path)
		}
		list = envelope.Cookies
	}
	var out []CookieSpec
	for _, c := range list {
		d := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if d == "ti.com" || strings.HasSuffix(d, ".ti.com") {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("cookie file %s contained no ti.com cookies", path)
	}
	return out, nil
}

// Browser is a container-aware agent-browser runner pinned to the Browserbase
// provider. When AGENT_BROWSER_CONTAINER is set, commands run via docker exec.
type Browser struct {
	apiKey     string
	container  string
	dockerPath string
	verbose    bool
}

// NewBrowser constructs a Browser. The Browserbase API key is required for
// every gated command.
func NewBrowser(verbose bool) (*Browser, error) {
	key := os.Getenv("BROWSERBASE_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("BROWSERBASE_API_KEY is not set (required for login-gated commands)")
	}
	b := &Browser{apiKey: key, container: os.Getenv("AGENT_BROWSER_CONTAINER"), verbose: verbose}
	if b.container != "" {
		if p, err := exec.LookPath("docker"); err == nil {
			b.dockerPath = p
		} else {
			return nil, fmt.Errorf("AGENT_BROWSER_CONTAINER set but docker binary not found")
		}
	} else if _, err := exec.LookPath("agent-browser"); err != nil {
		return nil, fmt.Errorf("agent-browser not found on PATH (required for login-gated commands)")
	}
	return b, nil
}

func (b *Browser) run(ctx context.Context, args ...string) (string, error) {
	allArgs := append([]string{"-p", "browserbase"}, args...)
	var cmd *exec.Cmd
	if b.container != "" {
		dockerArgs := append([]string{"exec", "-e", "BROWSERBASE_API_KEY=" + b.apiKey, b.container, "agent-browser"}, allArgs...)
		cmd = exec.CommandContext(ctx, b.dockerPath, dockerArgs...)
	} else {
		cmd = exec.CommandContext(ctx, "agent-browser", allArgs...)
		cmd.Env = append(os.Environ(), "BROWSERBASE_API_KEY="+b.apiKey)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("agent-browser %s: %w\noutput: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// redactArgs joins args for error messages, masking the value of a
// "cookies set <name> <value>" invocation so secrets never reach logs.
func redactArgs(args []string) string {
	if len(args) >= 4 && args[0] == "cookies" && args[1] == "set" {
		masked := append([]string{}, args[:3]...)
		masked = append(masked, "***")
		return strings.Join(append(masked, args[4:]...), " ")
	}
	return strings.Join(args, " ")
}

// Close ends the Browserbase session (best-effort). It uses a fresh short
// context so a session started under a --timeout/cancelled ctx is still torn
// down (an already-Done ctx would orphan the billable remote session).
func (b *Browser) Close(context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = b.run(ctx, "close")
}

// eval runs a JS expression and JSON-decodes agent-browser's quoted output.
func (b *Browser) eval(ctx context.Context, script string) (string, error) {
	out, err := b.run(ctx, "eval", script)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(out)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		var decoded string
		if json.Unmarshal([]byte(s), &decoded) == nil {
			return decoded, nil
		}
	}
	return s, nil
}

// InjectCookies bootstraps a session at the Material Content home page and
// injects the snapshot. Fails when zero cookies inject.
func (b *Browser) InjectCookies(ctx context.Context, cookies []CookieSpec) error {
	if _, err := b.run(ctx, "open", tiHomeURL); err != nil {
		return fmt.Errorf("bootstrapping session: %w", err)
	}
	injected := 0
	for _, c := range cookies {
		args := []string{"cookies", "set", c.Name, c.Value}
		if c.Domain != "" {
			args = append(args, "--domain", c.Domain)
		}
		if c.Path != "" {
			args = append(args, "--path", c.Path)
		}
		if c.HTTPOnly {
			args = append(args, "--httpOnly")
		}
		if c.Secure {
			args = append(args, "--secure")
		}
		if c.SameSite != "" {
			args = append(args, "--sameSite", c.SameSite)
		}
		if c.Expires > 0 {
			args = append(args, "--expires", fmt.Sprintf("%d", int64(c.Expires)))
		}
		if _, err := b.run(ctx, args...); err != nil {
			continue
		}
		injected++
	}
	if injected == 0 {
		return fmt.Errorf("no cookies injected; extraction would be unauthenticated")
	}
	return nil
}

func (b *Browser) landedURL(ctx context.Context) string {
	landed, err := b.eval(ctx, "window.location.href")
	if err != nil {
		return ""
	}
	return strings.Trim(landed, "'\"")
}

func (b *Browser) navigate(ctx context.Context, url string) error {
	if _, err := b.run(ctx, "open", url); err != nil {
		return fmt.Errorf("navigating to %s: %w", url, err)
	}
	if err := sleepCtx(ctx, 3*time.Second); err != nil { // client-side render + OIDC settle
		return err
	}
	if landed := b.landedURL(ctx); strings.Contains(landed, "login.ti.com") {
		return fmt.Errorf("%w (landed=%s)", ErrAuthRedirect, landed)
	}
	return nil
}

// AuthCheck reports whether the injected cookies still authenticate against
// Material Content. Returns nil when alive, ErrAuthRedirect when stale.
func (b *Browser) AuthCheck(ctx context.Context) error {
	return b.navigate(ctx, tiHomeURL)
}

// RatingsResult holds the raw cells of the Environmental Ratings table.
type RatingsResult struct {
	OrderableOPN string `json:"orderable_opn,omitempty"`
	ReportURL    string `json:"report_url,omitempty"`
	RoHSCell     string `json:"rohs_cell"`
	REACHCell    string `json:"reach_cell"`
	SVHCSentence string `json:"svhc_sentence,omitempty"`
}

const ratingsEvalScript = `
(function(){
  function txt(el){ return el ? el.textContent.replace(/\s+/g,' ').trim() : ''; }
  var tables = document.querySelectorAll('table.ti-table');
  for (var i=0;i<tables.length;i++){
    var ths = tables[i].querySelectorAll('thead th');
    if (ths.length >= 2 &&
        txt(ths[0]).toUpperCase().indexOf('ROHS') === 0 &&
        txt(ths[1]).toUpperCase().indexOf('REACH') === 0){
      var tds = tables[i].querySelectorAll('tbody tr td');
      var svhc = '';
      var ds = document.querySelectorAll('.disclaim');
      for (var j=0;j<ds.length;j++){
        var d = txt(ds[j]);
        if (d.indexOf('SVHC') >= 0 || d.indexOf('CAS#') >= 0){ svhc = d; break; }
      }
      return ['FOUND', txt(tds[0]), txt(tds[1]), tds[2]?txt(tds[2]):'', svhc].join('|||');
    }
  }
  return ['NOTFOUND', location.href, document.title].join('|||');
})()
`

// resolveReport navigates the part's search page and follows the report link.
// Returns (reportURL, orderableOPN).
func (b *Browser) resolveReport(ctx context.Context, opn string) (string, string, error) {
	if err := ValidateOPN(opn); err != nil {
		return "", "", err
	}
	if err := b.navigate(ctx, fmt.Sprintf(searchURLFmt, url.QueryEscape(opn))); err != nil {
		return "", "", err
	}
	if err := sleepCtx(ctx, 2*time.Second); err != nil {
		return "", "", err
	}
	// opn is JSON-encoded into a real JS string literal so a crafted part
	// number cannot break out of the quotes and run arbitrary JS in the
	// cookie-injected (credentialed) session. ValidateOPN already rejects
	// such input; this is defense in depth.
	opnLit, _ := json.Marshal(opn)
	script := fmt.Sprintf(`
		(function() {
			const links = document.querySelectorAll('a[href*="/materialcontent/en/report"]');
			if (links.length > 0) { return links[0].href; }
			const pcidInput = document.querySelector('input[name="pcid"]');
			if (pcidInput) {
				return 'https://www.ti.com/materialcontent/en/report?pcid=' + encodeURIComponent(pcidInput.value) + '&opn=' + encodeURIComponent(%s);
			}
			return '';
		})()
	`, string(opnLit))
	reportURL, err := b.eval(ctx, script)
	if err != nil {
		return "", "", fmt.Errorf("finding report link: %w", err)
	}
	reportURL = strings.Trim(reportURL, "'\"")
	if reportURL == "" {
		return "", "", fmt.Errorf("no material-content report link found for %s (part not found or page did not render)", opn)
	}
	if err := b.navigate(ctx, reportURL); err != nil {
		return "", "", err
	}
	orderable := queryParam(reportURL, "opn")
	if orderable == "" {
		orderable = opn
	}
	return reportURL, orderable, nil
}

// ScrapeRatings resolves the part's report page and reads the Environmental
// Ratings table (RoHS cell with exemption wording, REACH cell, SVHC sentence).
func (b *Browser) ScrapeRatings(ctx context.Context, opn string) (*RatingsResult, error) {
	reportURL, orderable, err := b.resolveReport(ctx, opn)
	if err != nil {
		return nil, err
	}
	out, err := b.eval(ctx, ratingsEvalScript)
	if err != nil {
		return nil, fmt.Errorf("eval ratings table: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(out), "|||")
	if len(parts) == 0 || parts[0] != "FOUND" {
		return nil, fmt.Errorf("Environmental Ratings table not found (page=%s)", strings.Join(parts[1:], " | "))
	}
	res := &RatingsResult{ReportURL: reportURL, OrderableOPN: orderable}
	if len(parts) > 1 {
		res.RoHSCell = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		res.REACHCell = strings.TrimSpace(parts[2])
	}
	if len(parts) > 4 {
		res.SVHCSentence = strings.TrimSpace(parts[4])
	}
	if res.RoHSCell == "" && res.REACHCell == "" {
		return nil, fmt.Errorf("ratings table found but both RoHS and REACH cells empty — refusing partial result")
	}
	return res, nil
}

// fmdFetchScript drives the report page's own #downloadStandardsXml form,
// selecting IPC-1752A Class D (standardsId=3) and fetching with credentials.
const fmdFetchScript = `
(async () => {
	try {
		const form = document.querySelector('#downloadStandardsXml')
			|| document.querySelector('form[action*="standards"]')
			|| document.querySelector('form[name="downloadStandardsXml"]');
		if (!form) return 'ERROR:could not find the standards download form (#downloadStandardsXml)';
		let chosen = '';
		const select = form.querySelector('select[name="standardsId"]')
			|| document.querySelector('select[name="standardsId"]');
		if (select) {
			for (const opt of Array.prototype.slice.call(select.options || [])) {
				const t = (opt.textContent || '').toLowerCase();
				if (t.indexOf('1752a') >= 0 && t.indexOf('class d') >= 0) {
					select.value = opt.value; chosen = opt.value; break;
				}
			}
			if (chosen === '') { select.value = '3'; chosen = '3'; }
			select.dispatchEvent(new Event('change', {bubbles: true}));
		} else { chosen = '3'; }
		const action = form.getAttribute('action') || '/materialcontent/api/opn/pcid/standards';
		const params = new URLSearchParams(new FormData(form));
		const supplement = function(key, val) { if (!params.get(key) && val) params.set(key, val); };
		const fieldVal = function(name) {
			const el = document.querySelector('[name="' + name + '"]');
			return el ? (el.value || '') : '';
		};
		supplement('opn', fieldVal('opn'));
		supplement('pcid', fieldVal('pcid'));
		supplement('sitecode', fieldVal('sitecode'));
		supplement('standardsId', chosen);
		const url = new URL(action, window.location.origin).toString() + '?' + params.toString();
		const r = await fetch(url, {credentials: 'include', headers: {Accept: 'application/xml,text/xml,*/*'}});
		if (!r.ok) return 'ERROR:HTTP ' + r.status + ' url=' + url;
		return await r.text();
	} catch (e) {
		return 'ERROR:' + (e && e.message ? e.message : e);
	}
})()
`

// FMDResult is the saved Class-D FMD XML for one orderable.
type FMDResult struct {
	OrderableOPN string `json:"orderable_opn"`
	ReportURL    string `json:"report_url"`
	XMLPath      string `json:"xml_path"`
	Bytes        int    `json:"bytes"`
}

// AcquireFMD resolves the report page and drives its standards form to fetch
// the IPC-1752A Class D XML, writing it to xmlPath.
func (b *Browser) AcquireFMD(ctx context.Context, opn, xmlPath string) (*FMDResult, error) {
	reportURL, orderable, err := b.resolveReport(ctx, opn)
	if err != nil {
		return nil, err
	}
	body, err := b.eval(ctx, fmdFetchScript)
	if err != nil {
		return nil, fmt.Errorf("standards-form eval failed: %w", err)
	}
	body = strings.Trim(body, "'\"")
	if strings.HasPrefix(body, "ERROR:") {
		return nil, fmt.Errorf("standards-form fetch: %s", strings.TrimPrefix(body, "ERROR:"))
	}
	if !isFMDXML(body) {
		return nil, fmt.Errorf("response is not IPC-1752A XML (got %d bytes starting %q)", len(body), truncate(body, 60))
	}
	if err := os.WriteFile(xmlPath, []byte(body), 0o644); err != nil {
		return nil, fmt.Errorf("writing XML: %w", err)
	}
	return &FMDResult{OrderableOPN: orderable, ReportURL: reportURL, XMLPath: xmlPath, Bytes: len(body)}, nil
}

// isFMDXML strictly accepts only bodies that are clearly the IPC-1752A XML.
func isFMDXML(s string) bool {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	prefix := trimmed
	if len(prefix) > 512 {
		prefix = prefix[:512]
	}
	lp := strings.ToLower(prefix)
	if strings.HasPrefix(lp, "<!doctype html") || strings.HasPrefix(lp, "<html") || strings.Contains(lp, "<html ") {
		return false
	}
	if strings.HasPrefix(lp, "<?xml") || strings.Contains(lp, "<maindeclaration") {
		return true
	}
	return strings.Contains(strings.ToLower(trimmed), "webstds.ipc.org/175x")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func queryParam(rawURL, key string) string {
	marker := key + "="
	idx := strings.Index(rawURL, marker)
	if idx < 0 {
		return ""
	}
	v := rawURL[idx+len(marker):]
	if amp := strings.IndexByte(v, '&'); amp >= 0 {
		v = v[:amp]
	}
	return v
}
