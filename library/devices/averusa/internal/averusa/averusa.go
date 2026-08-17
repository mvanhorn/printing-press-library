// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

// Package averusa harvests the AVer USA document catalog from the two vendor
// surfaces that carry it:
//
//   - The Salesforce Experience Cloud support portal (averusa.my.site.com)
//     serves knowledge articles as crawler-UA SSR HTML, publishes a 737-URL
//     article sitemap, resolves each article's Salesforce entityId through the
//     Aura action API, and serves attached files (manuals, spec sheets,
//     dimensions) via the fileField servlet.
//   - www.averusa.com carries the product catalog (75 product pages across 15
//     categories) and datasheet PDFs under /business/downloads/datasheet-brochure/.
//
// The split matters because no single surface answers "get the manual for a
// CAM570" — the portal article titles name models, the product pages name
// datasheets, and the entityId join makes downloads possible. The harvested
// corpus is joined locally.
//
// Transport notes (all verified replayable over plain HTTP, no cookies):
//   - Article SSR, sitemaps, fileField, and the Aura action API all accept a
//     Googlebot crawler UA; a default UA gets only a JS shell from the portal.
//   - The Aura framework uid (fwuid) needed for the action API is only in the
//     portal shell HTML, which requires a browser-like UA to fetch — so the
//     fwuid fetch uses a browser UA explicitly.
//   - Auth: none. Every surface is public.
package averusa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/cliutil"
)

// Surface URLs and paths.
const (
	PortalBase     = "https://averusa.my.site.com"
	PortalPath     = "/support/s"
	MainBase       = "https://www.averusa.com"
	ArticleSitemap = PortalBase + PortalPath + "/sitemap-topicarticle-1.xml"
	MainSitemap    = MainBase + "/sitemap.xml"
	SupportPath    = MainBase + "/support/"
	AuraPath       = PortalBase + PortalPath + "/sfsites/aura"
	FileFieldPath  = PortalBase + "/support/servlet/fileField"
	ShellURL       = PortalBase + PortalPath + "/"

	crawlerUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	// Documents and there is no reason to hammer a vendor server; the full
	// docs harvest touches ~1,500 requests (737 SSRs + 737 Aura resolutions).
	defaultRate = 3.0
)

// Client fetches from both AVer hosts under a shared adaptive rate limiter.
type Client struct {
	hc  *http.Client
	lim *cliutil.AdaptiveLimiter
}

func New() *Client {
	return &Client{
		hc:  &http.Client{Timeout: 45 * time.Second},
		lim: cliutil.NewAdaptiveLimiter(defaultRate),
	}
}

// get performs a rate-limited GET with the crawler UA. A 429 or 503 returns
// *cliutil.RateLimitError rather than an empty body, so callers can
// distinguish "throttled" from "nothing there".
func (c *Client) get(ctx context.Context, url, ua string) ([]byte, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(body),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	return io.ReadAll(io.LimitReader(resp.Body, 48<<20))
}

// postForm performs a rate-limited form POST with the crawler UA (Aura guest
// API: token=null, no cookies).
func (c *Client) postForm(ctx context.Context, url string, form url.Values) ([]byte, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", crawlerUA)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("posting %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
			Body:       string(body),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("posting %s: HTTP %d", url, resp.StatusCode)
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

// Download fetches a file's bytes with the crawler UA (used by docs pack for
// fileField PDFs and main-site datasheets).
func (c *Client) Download(ctx context.Context, url string) ([]byte, error) {
	return c.get(ctx, url, crawlerUA)
}

// Head performs a rate-limited HEAD request with the crawler UA, returning
// the status code and Content-Length. Used by `docs audit` link checks.
func (c *Client) Head(ctx context.Context, url string) (int, int64, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", crawlerUA)
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("head %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		return resp.StatusCode, 0, &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	return resp.StatusCode, resp.ContentLength, nil
}

// ProbeFile HEAD-checks a document URL and falls back to a bounded GET when the
// server rejects HEAD (403/405/501) or returns a 200 whose content-type does
// not confirm a PDF. It reports whether the resource actually serves a PDF, so
// GET-only document endpoints are never misclassified as broken. size is the
// response size used for display; headLen is the HEAD Content-Length (used by
// the caller to recognize the vendor's 61301-byte soft-404 shell).
func (c *Client) ProbeFile(ctx context.Context, url string) (status int, size int64, headLen int64, isPDF bool, err error) {
	status, headLen, ct, err := c.headWithType(ctx, url)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if status == http.StatusOK && strings.Contains(strings.ToLower(ct), "application/pdf") {
		return status, headLen, headLen, true, nil
	}
	switch status {
	case http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		// HEAD rejected — probe with a bounded ranged GET.
		gs, gsz, gpdf, gerr := c.probeGET(ctx, url)
		return gs, gsz, headLen, gpdf, gerr
	case http.StatusOK:
		// 200 with a non-PDF content-type is ambiguous (some endpoints serve
		// files only to GET); confirm with a bounded GET before any verdict.
		gs, gsz, gpdf, gerr := c.probeGET(ctx, url)
		return gs, gsz, headLen, gpdf, gerr
	}
	return status, 0, headLen, false, nil
}

// headWithType is Head plus the response Content-Type.
func (c *Client) headWithType(ctx context.Context, url string) (int, int64, string, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, 0, "", err
	}
	req.Header.Set("User-Agent", crawlerUA)
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, 0, "", fmt.Errorf("head %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		return resp.StatusCode, 0, "", &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	return resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), nil
}

// probeGET fetches the first bytes of a document to classify it.
func (c *Client) probeGET(ctx context.Context, url string) (int, int64, bool, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, false, err
	}
	req.Header.Set("User-Agent", crawlerUA)
	req.Header.Set("Range", "bytes=0-63")
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, 0, false, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		if c.lim != nil {
			c.lim.OnRateLimit()
		}
		return resp.StatusCode, 0, false, &cliutil.RateLimitError{URL: url, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if c.lim != nil {
		c.lim.OnSuccess()
	}
	// Bounded read: only the header bytes matter for classification, and some
	// servers ignore Range and stream the whole file.
	head := make([]byte, 64)
	n, _ := io.ReadFull(resp.Body, head)
	isPDF := n >= 5 && string(head[:5]) == "%PDF-"
	return resp.StatusCode, int64(n), isPDF, nil
}

// ---------- sitemaps ----------

var locRE = regexp.MustCompile(`(?s)<loc>\s*(.*?)\s*</loc>`)

// ArticleNames returns every article URL name from the portal article sitemap.
func (c *Client) ArticleNames(ctx context.Context) ([]string, error) {
	body, err := c.get(ctx, ArticleSitemap, crawlerUA)
	if err != nil {
		return nil, err
	}
	matches := locRE.FindAllStringSubmatch(string(body), -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		u := cliutil.CleanText(m[1])
		// URL form: .../support/s/article/{URL-Name}
		if i := strings.LastIndex(u, "/article/"); i >= 0 {
			name := strings.Trim(u[i+len("/article/"):], "/")
			if name != "" {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

// ProductRef is one product-page URL from the main-site sitemap.
type ProductRef struct {
	URL      string
	Category string
	Slug     string
}

// ProductRefs returns product page references from the main-site sitemap.
func (c *Client) ProductRefs(ctx context.Context) ([]ProductRef, error) {
	body, err := c.get(ctx, MainSitemap, crawlerUA)
	if err != nil {
		return nil, err
	}
	matches := locRE.FindAllStringSubmatch(string(body), -1)
	out := []ProductRef{}
	for _, m := range matches {
		u := cliutil.CleanText(m[1])
		if !strings.Contains(u, "/products/") {
			continue
		}
		rest := strings.Trim(strings.SplitN(u, "/products/", 2)[1], "/")
		if rest == "" {
			continue
		}
		segs := strings.Split(rest, "/")
		if len(segs) < 2 {
			// A bare category index page (e.g. /products/charging-cart/) —
			// not a product page.
			continue
		}
		cat, slug := segs[0], segs[len(segs)-1]
		slug = strings.TrimSuffix(strings.TrimSuffix(slug, ".asp"), ".html")
		if slug == "" {
			continue
		}
		out = append(out, ProductRef{URL: u, Category: cat, Slug: slug})
	}
	return out, nil
}

// ---------- HTML helpers ----------

var (
	dropRE  = regexp.MustCompile(`(?is)<script\b.*?</script>|<style\b.*?</style>|<head\b.*?</head>|<nav\b.*?</nav>|<footer\b.*?</footer>|<svg\b.*?</svg>`)
	tagRE   = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
	titleRE = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	h1RE    = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
)

// Text strips markup to readable prose.
func Text(htmlSrc string) string {
	s := dropRE.ReplaceAllString(htmlSrc, " ")
	s = tagRE.ReplaceAllString(s, " ")
	s = cliutil.CleanText(s)
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

// Title prefers <h1> over <title>: the portal leaves title tags verbatim.
func Title(htmlSrc string) string {
	if m := h1RE.FindStringSubmatch(htmlSrc); m != nil {
		if t := Text(m[1]); t != "" {
			return t
		}
	}
	if m := titleRE.FindStringSubmatch(htmlSrc); m != nil {
		return Text(m[1])
	}
	return ""
}

// ---------- knowledge articles ----------

// Article is one knowledge article harvested from the portal.
type Article struct {
	URLName     string `json:"url_name"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HasFile     bool   `json:"has_file"`
	PublishedAt string `json:"published_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

var (
	putFileLinkRE = regexp.MustCompile(`(?i)putFileLink`)
	// Salesforce SSR articles embed their record metadata as a base64
	// sfdcPage/PagePayload blob; published dates surface in rendered prose,
	// so we read dates from the Aura record instead of the SSR page.
	timeTagRE = regexp.MustCompile(`(?i)(?:created|published|modified|updated)[^<]{0,80}\b(20\d\d-\d\d-\d\d)[^<]{0,40}`)
)

// Article fetches one article's crawler-UA SSR page and extracts metadata.
func (c *Client) Article(ctx context.Context, urlName string) (Article, error) {
	body, err := c.get(ctx, PortalBase+PortalPath+"/article/"+urlName, crawlerUA)
	if err != nil {
		return Article{}, err
	}
	src := string(body)
	a := Article{
		URLName: urlName,
		Title:   Title(src),
		Body:    Text(src),
		HasFile: putFileLinkRE.MatchString(src),
	}
	if m := timeTagRE.FindStringSubmatch(src); m != nil {
		a.UpdatedAt = m[1]
	}
	if a.Title == "" {
		a.Title = urlName
	}
	return a, nil
}

// ---------- Aura entityId resolution ----------

const (
	auraAction = "ui-comm-runtime-components-aura-components-siteforce-recordservicecomponent.RecordServiceComponent.getArticleVersionId"
	auraDesc   = "serviceComponent://ui.comm.runtime.components.aura.components.siteforce.recordservicecomponent.RecordServiceComponentController/ACTION$getArticleVersionId"
	auraLoaded = "APPLICATION@markup://siteforce:communityApp"
)

// FetchFWUID grabs a fresh Aura framework uid from the portal shell HTML. The
// shell is only served to browser-like UAs, so this fetch uses one explicitly.
func (c *Client) FetchFWUID(ctx context.Context) (string, error) {
	body, err := c.get(ctx, ShellURL, browserUA)
	if err != nil {
		return "", err
	}
	fwuid, err := parseFWUID(string(body))
	if err != nil {
		return "", fmt.Errorf("parsing fwuid from portal shell: %w", err)
	}
	return fwuid, nil
}

// parseFWUID pulls fwuid out of the auraConfig JSON embedded in the shell.
func parseFWUID(html string) (string, error) {
	idx := strings.Index(html, "auraConfig")
	if idx < 0 {
		return "", fmt.Errorf("no auraConfig in shell")
	}
	start := strings.Index(html[idx:], "{")
	if start < 0 {
		return "", fmt.Errorf("no JSON after auraConfig")
	}
	start += idx
	depth := 0
	inStr := false
	esc := false
	end := start
	for i := start; i < len(html); i++ {
		ch := html[i]
		if inStr {
			if esc {
				esc = false
			} else if ch == '\\' {
				esc = true
			} else if ch == '"' {
				inStr = false
			}
		} else {
			switch ch {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i + 1
					i = len(html)
				}
			}
		}
	}
	var cfg struct {
		Context struct {
			FWUID string `json:"fwuid"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(html[start:end]), &cfg); err != nil {
		return "", err
	}
	if cfg.Context.FWUID == "" {
		return "", fmt.Errorf("fwuid empty")
	}
	return cfg.Context.FWUID, nil
}

type auraResponse struct {
	Actions []struct {
		State       string          `json:"state"`
		ReturnValue json.RawMessage `json:"returnValue"`
		Error       []struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"actions"`
}

// ResolveEntityID maps an article URL name to its Salesforce entityId via the
// Aura getArticleVersionId action. An empty result (article has no published
// version id) returns ("", nil).
func (c *Client) ResolveEntityID(ctx context.Context, fwuid, urlName string) (string, error) {
	action := map[string]any{
		"id":                "1;a",
		"descriptor":        auraDesc,
		"callingDescriptor": "UNKNOWN",
		"params":            map[string]any{"urlName": urlName},
		"storable":          true,
	}
	msg, err := json.Marshal(map[string]any{"actions": []any{action}})
	if err != nil {
		return "", err
	}
	ctxJSON, err := json.Marshal(map[string]any{
		"mode":   "PROD",
		"fwuid":  fwuid,
		"app":    "siteforce:communityApp",
		"loaded": map[string]string{auraLoaded: "1692_mtviYwT4OQy30JbfgmF_yA"},
		"dn":     []any{}, "globals": map[string]any{}, "uad": true,
	})
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("message", string(msg))
	form.Set("aura.context", string(ctxJSON))
	form.Set("aura.pageURI", PortalPath+"/article/"+urlName)
	form.Set("aura.token", "null")

	body, err := c.postForm(ctx, AuraPath+"?r=1&"+auraAction+"=1", form)
	if err != nil {
		return "", err
	}
	var resp auraResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parsing aura response for %s: %w", urlName, err)
	}
	if len(resp.Actions) == 0 {
		return "", nil
	}
	a := resp.Actions[0]
	if a.State != "SUCCESS" {
		if len(a.Error) > 0 {
			return "", fmt.Errorf("aura %s for %s: %s", a.State, urlName, a.Error[0].Message)
		}
		return "", fmt.Errorf("aura %s for %s", a.State, urlName)
	}
	var id string
	if err := json.Unmarshal(a.ReturnValue, &id); err != nil || id == "" {
		return "", nil
	}
	return id, nil
}

// ---------- products ----------

// Product is one product page harvested from www.averusa.com.
type Product struct {
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	DatasheetURL string `json:"datasheet_url,omitempty"`
}

var datasheetRE = regexp.MustCompile(`(?i)(?:datasheet-brochure/|downloads/)([a-z0-9-]+)-datasheet\.pdf`)

// ProductPage fetches one product page and extracts the model name and any
// datasheet PDF links.
func (c *Client) ProductPage(ctx context.Context, ref ProductRef) (Product, error) {
	body, err := c.get(ctx, ref.URL, crawlerUA)
	if err != nil {
		return Product{}, err
	}
	src := string(body)
	p := Product{Slug: ref.Slug, Category: ref.Category, URL: ref.URL}
	p.Name = Title(src)
	if p.Name == "" {
		p.Name = ref.Slug
	}
	// The vendor renders category breadcrumbs in h1 like "AVerCharge --> X16";
	// keep the product name, drop the breadcrumb arrow.
	p.Name = strings.TrimSpace(strings.ReplaceAll(p.Name, "-->", "-"))
	seen := map[string]bool{}
	for _, m := range datasheetRE.FindAllStringSubmatch(src, -1) {
		slug := strings.ToLower(m[1])
		// Prefer the datasheet named after THIS model over mislinked ones
		// (e.g. the CAM570 page also links cam520pro3-datasheet.pdf).
		if !seen[slug] {
			seen[slug] = true
		}
		if slug == strings.ToLower(ref.Slug) {
			p.DatasheetURL = MainBase + "/business/downloads/datasheet-brochure/" + m[1] + "-datasheet.pdf"
		}
	}
	if p.DatasheetURL == "" {
		for slug := range seen {
			p.DatasheetURL = MainBase + "/business/downloads/datasheet-brochure/" + slug + "-datasheet.pdf"
			break
		}
	}
	return p, nil
}

// DiscontinuedModel is one entry from the /support/ page's "Discontinued
// Devices"/"Discontinued Software" sections.
type DiscontinuedModel struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// DiscontinuedModels parses the /support/ download-center <select>: every
// option that follows a "Discontinued ..." label (until the next label) is a
// discontinued model. Option values are anchors like "##tr530plus".
func (c *Client) DiscontinuedModels(ctx context.Context) ([]DiscontinuedModel, error) {
	body, err := c.get(ctx, SupportPath, crawlerUA)
	if err != nil {
		return nil, err
	}
	src := string(body)
	optRE := regexp.MustCompile(`(?is)<option([^>]*)>\s*([^<]+?)\s*</option>`)
	inDisc := false
	var out []DiscontinuedModel
	seen := map[string]bool{}
	for _, m := range optRE.FindAllStringSubmatch(src, -1) {
		label := strings.TrimSpace(m[2])
		lower := strings.ToLower(label)
		if strings.HasPrefix(lower, "discontinued") {
			inDisc = true
			continue
		}
		// Category labels that end a discontinued section.
		if inDisc && (strings.HasPrefix(lower, "featured") || strings.HasPrefix(lower, "current") ||
			strings.HasPrefix(lower, "new")) {
			inDisc = false
			continue
		}
		if !inDisc {
			continue
		}
		v := optionValueRE.FindStringSubmatch(m[1])
		slug := ""
		if v != nil {
			slug = strings.ToLower(strings.TrimLeft(strings.TrimSpace(v[1]), "#"))
		}
		if slug == "" {
			continue
		}
		if !seen[slug] {
			seen[slug] = true
			out = append(out, DiscontinuedModel{Slug: slug, Name: label})
		}
	}
	return out, nil
}

var optionValueRE = regexp.MustCompile(`(?i)value\s*=\s*"([^"]*)"`)

// ---------- datasheet spec extraction ----------

// PDFText extracts the text layer of a PDF via pdftotext. Missing pdftotext
// is a soft degrade: the caller keeps the PDF URL and reports the gap.
func (c *Client) PDFText(ctx context.Context, pdfURL string) (string, error) {
	if c.lim != nil {
		c.lim.Wait()
	}
	body, err := c.get(ctx, pdfURL, crawlerUA)
	if err != nil {
		return "", err
	}
	// averusa.com serves soft-404 HTML shells for dead datasheet links; reject
	// them before pdftotext with a clear reason instead of a confusing exit 1.
	if len(body) < 5 || string(body[:5]) != "%PDF-" {
		head := strings.TrimSpace(string(body[:min(len(body), 120)]))
		if strings.Contains(strings.ToLower(head), "<html") || strings.Contains(strings.ToLower(head), "<!doctype") {
			return "", fmt.Errorf("datasheet URL returns HTML (dead/soft-404 link), not a PDF")
		}
		return "", fmt.Errorf("datasheet URL does not return a PDF (first bytes: %q)", head)
	}
	tmp, err := os.CreateTemp("", "averusa-ds-*.pdf")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return "", err
	}
	_ = tmp.Close()

	out, err := exec.CommandContext(ctx, "pdftotext", "-layout", tmp.Name(), "-").Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(out), nil
}

// ---------- classification ----------

var noiseModels = map[string]bool{
	"rs232": true, "rs422": true, "rs485": true, "usb": true,
	"hdmi": true, "poe": true, "vga": true, "4k": true, "1080p": true,
	"720p": true, "3d": true, "1d": true, "2d": true, "m1": true, "m2": true,
	"a1": true, "a2": true, "u1": true, "u2": true, "h1": true, "h2": true,
}

func isNoiseModel(slug string) bool {
	return noiseModels[slug] || len(slug) < 3
}

var modelTokRE = regexp.MustCompile(`\b[A-Z]{1,5}[0-9]{2,4}[A-Z0-9]*\b`)

// ExtractModel pulls a model slug out of an article title/URL name. Returns ""
// when no model-shaped token is present.
func ExtractModel(title, urlName string) string {
	hay := strings.ToUpper(title + " " + urlName)
	cands := []string{}
	seen := map[string]bool{}
	for _, tok := range modelTokRE.FindAllString(hay, -1) {
		slug := strings.ToLower(tok)
		if isNoiseModel(slug) || seen[slug] {
			continue
		}
		seen[slug] = true
		cands = append(cands, slug)
	}
	// Prefer the longest candidate (CAM570 over 570): model slugs are the
	// full token, not the trailing digits.
	if len(cands) == 0 {
		return ""
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if len(c) > len(best) {
			best = c
		}
	}
	return best
}

// ClassifyDocType buckets an article into the doc-type enum by title/URL
// keywords. The portal has no structured type field, so this is heuristic.
func ClassifyDocType(title, urlName string) string {
	s := strings.ToLower(title + " " + urlName)
	switch {
	case strings.Contains(s, "white paper") || strings.Contains(s, "whitepaper"):
		return "white-paper"
	case strings.Contains(s, "quick start") || strings.Contains(s, "quickstart") || strings.Contains(s, "qsg"):
		return "quick-start"
	case strings.Contains(s, "datasheet") || strings.Contains(s, "spec"):
		return "spec-sheet"
	case strings.Contains(s, "manual"):
		return "user-manual"
	case strings.Contains(s, "brochure"):
		return "brochure"
	case strings.Contains(s, "comparison"):
		return "comparison-chart"
	case strings.Contains(s, "firmware") || strings.Contains(s, "software") ||
		strings.Contains(s, " driver") || strings.Contains(s, " app ") || strings.Contains(s, "ptzapp"):
		return "software"
	default:
		return "article"
	}
}

// specFieldRE matches a line whose leading "Field:" is one of the allowlisted
// spec dimensions (optionally after a bullet glyph or whitespace).
var specFieldRE = regexp.MustCompile(`(?im)^\s*[．•\-*]?\s*(image sensor|sensor|optical zoom|digital zoom|total zoom|field of view|fov|resolution|focus|focal length|aperture|lens|pan range|tilt range|pan speed|tilt speed|video output|video resolution|frame rate|weight|dimensions|power consumption|power supply|power|interface|connectivity|ports|port|audio input|audio output|microphone|speaker|memory|storage|battery|certification|compliance|warranty|color|remote control)\s*:\s*(.{2,160})\s*$`)

// ExtractSpecFields pulls "Field: value" lines out of datasheet text. It is
// deliberately high-precision: only allowlisted spec dimensions are kept, and
// a value containing another colon is rejected. Bounded per model.
func ExtractSpecFields(model, pdfText string) map[string]string {
	out := map[string]string{}
	for _, m := range specFieldRE.FindAllStringSubmatch(pdfText, -1) {
		field := strings.ToLower(strings.TrimSpace(m[1]))
		value := strings.TrimSpace(m[2])
		if field == "" || value == "" {
			continue
		}
		if strings.Contains(value, ":") {
			continue
		}
		// Layout mode merges adjacent datasheet columns onto one line; keep
		// only the first column (separated by 2+ spaces or a fullwidth dot).
		value = strings.TrimSpace(spaceRE.Split(value, 2)[0])
		if value == "" {
			continue
		}
		if _, ok := out[field]; !ok {
			out[field] = value
		}
		if len(out) >= 40 {
			break
		}
	}
	return out
}
