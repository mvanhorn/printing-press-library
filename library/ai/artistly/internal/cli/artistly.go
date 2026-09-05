// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored Artistly transport layer (NOT generator-emitted). Artistly
// (app.artistly.ai) has no public API: it is a Laravel + Inertia.js app behind
// a session cookie (artistly_session) with CSRF protection (XSRF-TOKEN cookie
// echoed as the X-XSRF-TOKEN header on writes). This file holds the shared
// helpers every hand-built command uses: CSRF header construction, the Design
// record shape, generation submit + poll, image download, and the Inertia
// shared-props read (quota + style catalog). Keep it in its own file so it
// survives regen as a whole hand-authored unit.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/client"
)

// Design mirrors the records returned by /fetch-personal-designs. Only the
// fields the CLI surfaces are typed; the upstream record has ~40 fields.
type Design struct {
	ID             int    `json:"id"`
	UUID           string `json:"uuid"`
	UserID         int    `json:"user_id"`
	PositivePrompt string `json:"positive_prompt"`
	NegativePrompt string `json:"negative_prompt"`
	Checkpoint     string `json:"checkpoint"`
	CheckpointID   int    `json:"checkpoint_id"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Quantity       int    `json:"quantity"`
	Images         string `json:"images"` // JSON-encoded string array of CDN URLs
	AspectRatio    string `json:"aspect_ratio"`
	Seed           string `json:"seed"`
	ToolUsed       string `json:"tool_used"`
	Status         string `json:"status"`
	Category       string `json:"category"`
	FolderID       *int   `json:"folder_id"`
	CreatedAt      string `json:"created_at"`
}

// designsEnvelope is the wrapper returned by the personal-designs endpoints.
type designsEnvelope struct {
	Designs []Design `json:"designs"`
}

// defaultTool is the feature slug for the standard text-to-image generator.
// All Artistly generators share POST /ai/{feature}/store; --tool overrides this.
const defaultTool = "image-designer-v6"

// readJarCookie reads a single cookie value from the persisted cookie jar that
// `auth login --chrome` populates. Laravel URL-encodes the XSRF-TOKEN value in
// the cookie; callers that need the header form should pass urlDecode=true.
func readJarCookie(name string, urlDecode bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(home, ".local", "share", "artistly-pp-cli", "cookies.json")
	data, err := os.ReadFile(path) // #nosec G304 -- path is the fixed session-cookie store, not user-supplied
	if err != nil {
		return "", fmt.Errorf("no stored session; run: artistly-pp-cli auth login --chrome")
	}
	var rows []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", fmt.Errorf("cookie store unreadable: %w", err)
	}
	for _, r := range rows {
		if r.Name == name {
			if urlDecode {
				if dec, derr := url.QueryUnescape(r.Value); derr == nil {
					return dec, nil
				}
			}
			return r.Value, nil
		}
	}
	return "", fmt.Errorf("cookie %q not found; run: artistly-pp-cli auth login --chrome", name)
}

// jarSessionPresent reports whether the persistent cookie jar holds a non-empty
// artistly_session cookie. The jar — not config.toml — is the sole source of
// outbound auth for cookie-auth requests (see client.persistLocked usage), so
// `auth status` and `doctor` consult this to recognize sessions written by
// `auth login --chrome` or the auth-refresh sidecar, which never touch
// config.toml's auth_header/access_token. Presence is enough to report
// "authenticated"; an expired session still surfaces as a 401 on the next real
// call, matching how config credentials are treated as "present, not verified".
func jarSessionPresent() bool {
	val, err := readJarCookie("artistly_session", false)
	return err == nil && strings.TrimSpace(val) != ""
}

// writeHeaders builds the headers required for Laravel state-changing requests:
// the CSRF token (decoded XSRF-TOKEN cookie -> X-XSRF-TOKEN) and the AJAX
// marker. X-Inertia is intentionally omitted: the /ai/{feature}/store endpoint
// queues a generation without it (verified during discovery), and omitting it
// avoids coupling to the deploy-specific Inertia asset version.
//
// When the session was seeded from ARTISTLY_SESSION_COOKIE (headless/CI), the
// jar has no XSRF-TOKEN yet; a priming GET makes Laravel issue one before the
// write proceeds.
func writeHeaders(ctx context.Context, c *client.Client) (map[string]string, error) {
	token, err := readJarCookie("XSRF-TOKEN", true)
	if err != nil || token == "" {
		if primeErr := primeCSRF(ctx, c); primeErr != nil {
			if err != nil {
				return nil, err
			}
			return nil, primeErr
		}
		token, err = readJarCookie("XSRF-TOKEN", true)
		if err != nil {
			return nil, err
		}
		if token == "" {
			return nil, authErr(fmt.Errorf("no CSRF token available after priming; run: artistly-pp-cli auth login --chrome"))
		}
	}
	return map[string]string{
		"X-XSRF-TOKEN":     token,
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "application/json, text/html",
	}, nil
}

// primeCSRF performs a GET so Laravel issues a fresh XSRF-TOKEN cookie, which
// the persistent jar saves to disk for the immediately following write. Used
// when the session arrived via ARTISTLY_SESSION_COOKIE without a paired CSRF
// cookie (a Chrome-imported login already carries one).
func primeCSRF(ctx context.Context, c *client.Client) error {
	_, err := c.GetWithHeadersNoCache(ctx, "/dashboard", map[string]string{}, map[string]string{"Accept": "text/html"})
	return err
}

// seedSessionFromEnv lets headless/CI/agent callers authenticate without
// `auth login --chrome` by supplying the artistly_session cookie value in
// ARTISTLY_SESSION_COOKIE. The value is merged into the persistent cookie jar
// (keyed to the configured host) so outbound requests carry it; the matching
// XSRF-TOKEN is fetched automatically on the first write (see primeCSRF).
func seedSessionFromEnv(baseURL string) {
	val := strings.TrimSpace(os.Getenv("ARTISTLY_SESSION_COOKIE"))
	if val == "" {
		return
	}
	val = strings.TrimPrefix(val, "artistly_session=")
	if cur, _ := readJarCookie("artistly_session", false); cur == val {
		return // already seeded; don't rewrite the jar on every invocation
	}
	_ = client.WriteCookieJarFromMap(hostFromBaseURL(baseURL), map[string]string{"artistly_session": val})
}

// hostFromBaseURL extracts the cookie host from the configured base URL,
// defaulting to the production app host.
func hostFromBaseURL(baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return "app.artistly.ai"
}

// fetchPersonalDesigns returns the caller's designs newest-first. It uses the
// non-cached GET so polling loops and live mutation/lookup commands always see
// current server state — the response cache's 5-minute TTL would otherwise make
// pollForNewDesigns read a stale "processing" snapshot and never observe
// completion.
func fetchPersonalDesigns(ctx context.Context, c *client.Client) ([]Design, error) {
	raw, err := c.GetNoCache(ctx, "/fetch-personal-designs", map[string]string{})
	if err != nil {
		return nil, err
	}
	return parseDesigns(raw)
}

func parseDesigns(raw json.RawMessage) ([]Design, error) {
	// An unauthenticated/expired session is served the login HTML page (HTTP
	// 200), not JSON. Detect that and return a clean auth error instead of a
	// cryptic "invalid character '<'" JSON parse failure.
	if trimmed := strings.TrimSpace(string(raw)); strings.HasPrefix(trimmed, "<") {
		return nil, authErr(fmt.Errorf("not authenticated or session expired; run: artistly-pp-cli auth login --chrome"))
	}
	// The endpoint returns {"designs":[...]}; resolveReadWithStrategy may also
	// hand back a bare array, so accept either shape.
	var env designsEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.Designs != nil {
		return env.Designs, nil
	}
	var arr []Design
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("parsing designs: %w", err)
	}
	return arr, nil
}

// designImageURLs decodes the JSON-encoded `images` string and returns the real
// CDN URLs, skipping the placehold.co placeholder used while a design is still
// processing.
func designImageURLs(d Design) []string {
	if strings.TrimSpace(d.Images) == "" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(d.Images), &urls); err != nil {
		return nil
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if u == "" || strings.Contains(u, "placehold.co") {
			continue
		}
		out = append(out, u)
	}
	return out
}

// isDone reports whether a design has finished rendering (status left
// "processing" and it has at least one real CDN image).
func isDone(d Design) bool {
	return !strings.EqualFold(d.Status, "processing") && len(designImageURLs(d)) > 0
}

// generationBody assembles the POST /ai/{feature}/store payload from the
// verified field contract.
func generationBody(prompt, negative, style string, checkpointID, width, height, quantity, seed int, aspect, quality string, folderID *int) map[string]any {
	body := map[string]any{
		"image_option":          "",
		"image_selected":        "no",
		"prompt":                prompt,
		"positive_prompt":       prompt,
		"negative_prompt":       negative,
		"tool_used":             "AI Image Designer v6",
		"category":              "AI Image Designer v6",
		"checkpointId":          checkpointID,
		"checkpoint":            nil,
		"seed":                  seed,
		"aspect_ratio":          aspect,
		"width":                 width,
		"height":                height,
		"quality":               quality,
		"quantity":              quantity,
		"theme":                 "",
		"image_url":             "",
		"image_base64":          "",
		"personal_design":       "",
		"denoise_strength":      "0.75",
		"prefix":                "",
		"suffix":                "",
		"lora":                  style, // style label/slug; null when empty handled below
		"should_enhance_prompt": false,
		"visibility":            "private",
		"folder_id":             folderID,
	}
	if style == "" {
		body["lora"] = nil
	}
	return body
}

// submitGeneration POSTs a generation request and returns nil on a queued
// (2xx/3xx) response. The endpoint replies 302 -> /personal-designs which the
// HTTP client follows to a 200 HTML page; either is success.
func submitGeneration(ctx context.Context, c *client.Client, tool string, body map[string]any) error {
	headers, err := writeHeaders(ctx, c)
	if err != nil {
		return err
	}
	if tool == "" {
		tool = defaultTool
	}
	_, status, err := c.PostWithHeaders(ctx, "/ai/"+tool+"/store", body, headers)
	if err != nil {
		// A non-2xx surfaces as an apiError; surface it. (302 is followed and
		// returns 200, so a real error here is a genuine failure.)
		return err
	}
	if status >= 200 && status < 400 {
		return nil
	}
	return fmt.Errorf("generation request returned HTTP %d", status)
}

// pollForNewDesigns polls /fetch-personal-designs until `want` designs with an
// ID greater than afterID have finished, or the timeout elapses. Returns the
// finished designs (may be fewer than `want` on timeout).
func pollForNewDesigns(ctx context.Context, c *client.Client, afterID, want int, timeout time.Duration, progress io.Writer) ([]Design, error) {
	deadline := time.Now().Add(timeout)
	delay := 3 * time.Second
	// Track the most recent successful "done" set so a fetch failure (e.g. the
	// context deadline firing mid-request) returns the designs already known to
	// have rendered instead of discarding them.
	var lastDone []Design
	for {
		designs, err := fetchPersonalDesigns(ctx, c)
		if err != nil {
			return lastDone, err
		}
		var done []Design
		for _, d := range designs {
			if d.ID > afterID && isDone(d) {
				done = append(done, d)
			}
		}
		lastDone = done
		if len(done) >= want {
			return done, nil
		}
		if time.Now().After(deadline) {
			return done, fmt.Errorf("timed out after %s waiting for %d design(s) to render (%d ready)", timeout, want, len(done))
		}
		if progress != nil {
			fmt.Fprintf(progress, "  ...rendering (%d/%d ready)\n", len(done), want)
		}
		select {
		case <-ctx.Done():
			return done, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// maxDesignID returns the highest design ID currently visible, used as the
// "after" cursor before submitting a new generation.
func maxDesignID(designs []Design) int {
	m := 0
	for _, d := range designs {
		if d.ID > m {
			m = d.ID
		}
	}
	return m
}

// downloadImage fetches a single CDN image URL to a file. The CDN is public, so
// no auth is attached.
func downloadImage(ctx context.Context, rawURL, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	f, err := os.Create(dest) // #nosec G304 -- dest is the user's chosen output path for the downloaded image
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// downloadDesign writes every CDN image of a design into dir, returning the
// written file paths. nameTemplate supports {id}, {uuid}, {prompt}, {n}.
func downloadDesign(ctx context.Context, d Design, dir, nameTemplate string) ([]string, error) {
	urls := designImageURLs(d)
	if len(urls) == 0 {
		return nil, nil
	}
	var written []string
	for i, u := range urls {
		ext := imageExt(u)
		dest := filepath.Join(dir, designImageName(nameTemplate, d, i, len(urls))+ext)
		if err := downloadImage(ctx, u, dest); err != nil {
			return written, err
		}
		written = append(written, dest)
	}
	return written, nil
}

// designImageName computes the output filename (without extension) for image i
// of total in a design. It disambiguates multi-image designs with a numeric
// suffix, but only when the template didn't already place the index via {n} —
// otherwise names double up as "1-1", "2-2".
func designImageName(nameTemplate string, d Design, i, total int) string {
	base := renderNameTemplate(nameTemplate, d, i+1)
	if total > 1 && !strings.Contains(nameTemplate, "{n}") {
		return fmt.Sprintf("%s-%d", base, i+1)
	}
	return base
}

// parseDesignTime parses a Design.CreatedAt value. The API returns timestamps
// with sub-second precision (e.g. "2026-06-15T06:09:15.000000Z"), which
// time.RFC3339 (no fractional seconds) cannot parse — so try the Nano layout
// first and fall back to RFC3339. Returns ok=false when neither matches.
func parseDesignTime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func imageExt(u string) string {
	if i := strings.LastIndex(u, "."); i >= 0 {
		ext := u[i:]
		if q := strings.IndexAny(ext, "?#"); q >= 0 {
			ext = ext[:q]
		}
		if len(ext) <= 5 {
			return ext
		}
	}
	return ".png"
}

func renderNameTemplate(tmpl string, d Design, n int) string {
	if tmpl == "" {
		tmpl = "{id}"
	}
	repl := strings.NewReplacer(
		"{id}", strconv.Itoa(d.ID),
		"{uuid}", d.UUID,
		"{n}", strconv.Itoa(n),
		"{prompt}", slugify(d.PositivePrompt),
	)
	return sanitizeFilename(repl.Replace(tmpl))
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
		if b.Len() >= 60 {
			break
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "design"
	}
	return out
}

func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune("/\\:*?\"<>|", r) {
			return '-'
		}
		return r
	}, s)
}

// sharedProps fetches an authenticated page and extracts the Inertia
// shared-props object embedded in the #app data-page attribute. Used for quota
// and the style catalog, which Artistly only exposes via page props.
func sharedProps(ctx context.Context, c *client.Client) (map[string]json.RawMessage, error) {
	// The dashboard page carries the shared props as an HTML data-page attr.
	// Bypass the read cache: quota is a preflight (a stale todays_design_count
	// could wave through a batch that actually breaches the ~400/day cap) and the
	// style catalog should reflect the live account.
	headers := map[string]string{"Accept": "text/html"}
	raw, err := c.GetWithHeadersNoCache(ctx, "/dashboard", map[string]string{}, headers)
	if err != nil {
		return nil, err
	}
	html := string(raw)
	props, err := extractDataPageProps(html)
	if err != nil {
		return nil, err
	}
	return props, nil
}

// extractDataPageProps pulls the JSON from <div id="app" data-page="{...}"> and
// returns its `props` object as a raw map.
func extractDataPageProps(html string) (map[string]json.RawMessage, error) {
	const marker = "data-page=\""
	i := strings.Index(html, marker)
	if i < 0 {
		return nil, fmt.Errorf("could not locate Inertia page data (are you authenticated? run: artistly-pp-cli auth login --chrome)")
	}
	rest := html[i+len(marker):]
	// data-page is HTML-attribute-escaped (&quot; for inner quotes), so the
	// literal `">` sequence terminates the attribute. Fall back to the next
	// bare double-quote if the close-tag form isn't found.
	end := strings.Index(rest, "\">")
	if end < 0 {
		end = strings.Index(rest, "\"")
	}
	if end < 0 {
		return nil, fmt.Errorf("malformed Inertia page data")
	}
	attr := rest[:end]
	attr = htmlUnescapeAttr(attr)
	var page struct {
		Props map[string]json.RawMessage `json:"props"`
	}
	if err := json.Unmarshal([]byte(attr), &page); err != nil {
		return nil, fmt.Errorf("parsing Inertia page data: %w", err)
	}
	return page.Props, nil
}

// htmlUnescapeAttr decodes the HTML-attribute escaping Inertia applies to the
// data-page JSON. html.UnescapeString covers the full HTML5 entity set
// (including numeric/Unicode entities), so a stray entity in the props can't
// break json.Unmarshal for quota/styles/folders.
func htmlUnescapeAttr(s string) string {
	return html.UnescapeString(s)
}

// quotaInfo holds the daily generation budget from shared props.
type quotaInfo struct {
	ConcurrentCount int `json:"concurrent_generation_count"`
	ConcurrentLimit int `json:"concurrent_generation_limit"`
	TodaysCount     int `json:"todays_design_count"`
}

func fetchQuota(ctx context.Context, c *client.Client) (quotaInfo, error) {
	props, err := sharedProps(ctx, c)
	if err != nil {
		return quotaInfo{}, err
	}
	q := quotaInfo{}
	intFromProp(props, "concurrent_generation_count", &q.ConcurrentCount)
	intFromProp(props, "concurrent_generation_limit", &q.ConcurrentLimit)
	intFromProp(props, "todays_design_count", &q.TodaysCount)
	return q, nil
}

func intFromProp(props map[string]json.RawMessage, key string, dst *int) {
	if raw, ok := props[key]; ok {
		_ = json.Unmarshal(raw, dst)
	}
}

func stringProp(props map[string]json.RawMessage, key string) string {
	if raw, ok := props[key]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	}
	return ""
}

// promptToolPage is the authenticated page whose Inertia props expose the
// enhancedPrompt / extractedPrompt flash results.
const promptToolPage = "/ai/image-designer-v6"

// runPromptTransform drives Artistly's prompt tools (enhance, extract). They are
// flash-redirect endpoints: POST returns a 302 and the result is flashed into
// the session, surfaced as an Inertia prop on the next page load. The result is
// mildly async, so this polls the page prop for a few seconds. body is the POST
// JSON; propKey is the prop carrying the result (enhancedPrompt / extractedPrompt).
func runPromptTransform(ctx context.Context, c *client.Client, route string, body map[string]any, propKey string) (string, error) {
	// Laravel flash survives exactly ONE subsequent request, so the page read
	// must be the immediate next request after the POST. The POST therefore must
	// NOT follow its 302 (the generated client does), or the followed hop spends
	// the one-request flash budget. Each attempt re-POSTs (re-arming the flash)
	// then does a single page GET.
	for attempt := 0; attempt < 4; attempt++ {
		if err := postNoFollow(ctx, c, route, body); err != nil {
			return "", err
		}
		// The transform is mildly async; give the server a moment to set flash.
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2500 * time.Millisecond):
		}
		raw, gerr := c.GetWithHeadersNoCache(ctx, promptToolPage, map[string]string{}, map[string]string{"Accept": "text/html"})
		if gerr != nil {
			return "", gerr
		}
		props, pperr := extractDataPageProps(string(raw))
		if pperr != nil {
			continue
		}
		if v := stringProp(props, propKey); v != "" {
			return html.UnescapeString(v), nil
		}
		if flashRaw, ok := props["flash"]; ok {
			var flash map[string]json.RawMessage
			if json.Unmarshal(flashRaw, &flash) == nil {
				if v := stringProp(flash, propKey); v != "" {
					return html.UnescapeString(v), nil
				}
			}
		}
	}
	return "", fmt.Errorf("no result returned (the prompt tool timed out or produced nothing)")
}

// postNoFollow sends a CSRF-protected POST that does NOT follow redirects, using
// the persisted cookie jar so the session (and any rotated Set-Cookie) stays in
// sync with the generated client's subsequent requests. Used by the prompt
// tools, whose result is flashed and read on the very next request.
func postNoFollow(ctx context.Context, c *client.Client, path string, body map[string]any) error {
	headers, err := writeHeaders(ctx, c)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	jar := client.LoadCookieJar()
	hc := &http.Client{
		Jar:     jar,
		Timeout: 90 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // do not follow; preserve the flash budget
		},
	}
	// Honor the client's configured BaseURL (e.g. ARTISTLY_BASE_URL) so prompt
	// enhance/extract hit the same host as every other request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RequestBaseURL()+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the jar persists Set-Cookie
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s returned HTTP %d", path, resp.StatusCode)
	}
	return nil
}

// writeCall performs a CSRF-protected state-changing request and treats any
// 2xx/3xx (Laravel/Inertia endpoints reply 302 on success) as success. For
// DELETE, body is ignored (the id rides in the path).
func writeCall(ctx context.Context, c *client.Client, method, path string, body map[string]any) error {
	headers, err := writeHeaders(ctx, c)
	if err != nil {
		return err
	}
	var status int
	switch method {
	case "POST":
		_, status, err = c.PostWithHeaders(ctx, path, body, headers)
	case "PUT":
		_, status, err = c.PutWithHeaders(ctx, path, body, headers)
	case "DELETE":
		_, status, err = c.DeleteWithHeaders(ctx, path, headers)
	default:
		return fmt.Errorf("unsupported method %q", method)
	}
	if err != nil {
		return err
	}
	if status >= 200 && status < 400 {
		return nil
	}
	return fmt.Errorf("%s %s returned HTTP %d", method, path, status)
}

// findDesign locates a design by numeric id or uuid from the live design list.
func findDesign(ctx context.Context, c *client.Client, ident string) (*Design, error) {
	designs, err := fetchPersonalDesigns(ctx, c)
	if err != nil {
		return nil, err
	}
	idNum, numErr := strconv.Atoi(strings.TrimSpace(ident))
	for i := range designs {
		if (numErr == nil && designs[i].ID == idNum) || designs[i].UUID == ident {
			return &designs[i], nil
		}
	}
	return nil, notFoundErr(fmt.Errorf("design %q not found in your recent designs", ident))
}

// foldersFromProps reads the user's folders from Inertia shared props.
func foldersFromProps(ctx context.Context, c *client.Client) ([]folderRec, error) {
	props, err := sharedProps(ctx, c)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var out []folderRec
	for _, key := range []string{"folders", "personal_folders"} {
		if raw, ok := props[key]; ok {
			var arr []folderRec
			if json.Unmarshal(raw, &arr) == nil {
				for _, f := range arr {
					if !seen[f.ID] {
						seen[f.ID] = true
						out = append(out, f)
					}
				}
			}
		}
	}
	return out, nil
}

type folderRec struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// truncateJSONArray returns the first n elements of a JSON array, or the input
// unchanged when it is not an array or n <= 0.
func truncateJSONArray(raw json.RawMessage, n int) json.RawMessage {
	if n <= 0 {
		return raw
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return raw
	}
	if len(items) <= n {
		return raw
	}
	out, err := json.Marshal(items[:n])
	if err != nil {
		return raw
	}
	return out
}
