// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-built ht-ml.app commands (publish, update,
// assets, password, templates) and the novel commands. Survives generate --force.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/ht-ml/internal/store"
)

// htmlxStorePath resolves the local SQLite path for the registry.
func htmlxStorePath(flags *rootFlags) string {
	return defaultDBPath("ht-ml-pp-cli")
}

// htmlxOpenStore opens the local store and ensures the ht-ml.app side tables
// exist. Callers must Close the returned store.
func htmlxOpenStore(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, htmlxStorePath(flags))
	if err != nil {
		return nil, err
	}
	if err := db.EnsureHTMLSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// siteAPIResponse is the shape returned by POST /v1/sites and PUT
// /v1/sites/{id}. GET /v1/sites/{id} returns site_id/status/assets only.
type siteAPIResponse struct {
	SiteID    string `json:"site_id"`
	UpdateKey string `json:"update_key"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

func parseSiteAPIResponse(raw json.RawMessage) (siteAPIResponse, error) {
	var r siteAPIResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("parsing API response: %w", err)
	}
	return r, nil
}

// siteAssetsResponse is the shape returned by GET /v1/sites/{id}.
type siteAssetsResponse struct {
	SiteID string `json:"site_id"`
	Status string `json:"status"`
	Assets []struct {
		RelativePath string `json:"relative_path"`
		AssetType    string `json:"asset_type"`
		Status       string `json:"status"`
	} `json:"assets"`
}

// bearerHeaders builds the per-site Authorization header for writes.
func bearerHeaders(updateKey string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + updateKey}
}

// siteLiveURL is the public URL a site is served at.
func siteLiveURL(siteID string) string {
	return "https://" + siteID + ".ht-ml.app/"
}

// liveAssetURL is the public URL a referenced asset is served at.
func liveAssetURL(siteID, relativePath string) string {
	return strings.TrimRight(siteLiveURL(siteID), "/") + "/" + strings.TrimLeft(relativePath, "/")
}

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractTitle pulls the <title> text from an HTML document, collapsed to one
// line. Returns "" when there is no title.
func extractTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	t := cliutil.CleanText(m[1])
	t = strings.Join(strings.Fields(t), " ")
	return t
}

// readHTMLInput resolves HTML content from (in priority order): the --html
// literal flag, stdin when the source arg is "-", or a file path. Returns the
// HTML, a human label for the source, and the resolved file path ("" for
// stdin/literal) so callers can resolve relative asset roots.
func readHTMLInput(cmd interface {
	InOrStdin() io.Reader
}, args []string, htmlFlag string) (htmlContent, sourceLabel, filePath string, err error) {
	if htmlFlag != "" {
		return htmlFlag, "--html", "", nil
	}
	if len(args) == 0 {
		return "", "", "", fmt.Errorf("provide an HTML file path, '-' for stdin, or --html <string>")
	}
	src := args[0]
	if src == "-" {
		b, rerr := io.ReadAll(cmd.InOrStdin())
		if rerr != nil {
			return "", "", "", fmt.Errorf("reading stdin: %w", rerr)
		}
		if len(bytes.TrimSpace(b)) == 0 {
			return "", "", "", fmt.Errorf("stdin was empty")
		}
		return string(b), "stdin", "", nil
	}
	b, rerr := os.ReadFile(src)
	if rerr != nil {
		return "", "", "", fmt.Errorf("reading %s: %w", src, rerr)
	}
	return string(b), src, src, nil
}

// publicContentWarn prints the API-required reminder that everything published
// to ht-ml.app is public and permanent. It writes to stderr so it never
// corrupts JSON on stdout.
func publicContentWarn(cmd interface{ ErrOrStderr() io.Writer }) {
	fmt.Fprintln(cmd.ErrOrStderr(), "warning: ht-ml.app sites are PUBLIC and permanent: anyone with the URL can read them, and there is no delete endpoint. Do not publish private or sensitive content. If acting for a person, confirm they want this public.")
}

// htmlxWriteGuard returns true and prints a synthetic-safe message when the
// command must not perform a real write: under --dry-run or PRINTING_PRESS_VERIFY=1.
// Hand-built writes that bypass internal/client (multipart uploads) and even
// those that use it get this belt-and-suspenders guard so verify never mutates
// real sites.
func htmlxWriteGuard(cmd interface{ OutOrStdout() io.Writer }, flags *rootFlags, action string) bool {
	mode := ""
	if dryRunOK(flags) {
		mode = "dry-run"
	} else if cliutil.IsVerifyEnv() {
		mode = "verify"
	}
	if mode == "" {
		return false
	}
	// Stay machine-parseable under --json/--agent so a dry-run invocation still
	// emits valid JSON on stdout (the live-dogfood json fidelity check runs
	// mutating commands with --dry-run --json).
	if flags.asJSON {
		_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "mode": mode, "action": action}, flags)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: would %s\n", mode, action)
	}
	return true
}

// mapSiteHTTPError converts an ht-ml.app non-2xx status + JSON body into a
// typed CLI error with the API's actionable message and a recovery hint.
func mapSiteHTTPError(status int, body []byte, flags *rootFlags) error {
	msg := extractAPIMessage(body)
	switch status {
	case http.StatusUnauthorized: // 401
		return authErr(fmt.Errorf("ht-ml.app 401: %s\nhint: the update_key is missing. For a private site read, set its password; for writes, ensure the site_id is in your store (run 'ht-ml-pp-cli list')", firstNonEmpty(msg, "unauthorized")))
	case http.StatusForbidden: // 403
		return authErr(fmt.Errorf("ht-ml.app 403: %s\nhint: the update_key is wrong, or (for asset upload) the path is not referenced in the site's HTML", firstNonEmpty(msg, "forbidden")))
	case http.StatusNotFound: // 404
		return notFoundErr(fmt.Errorf("ht-ml.app 404: %s\nhint: no such site_id; run 'ht-ml-pp-cli list' to see your sites", firstNonEmpty(msg, "not found")))
	case 422:
		return apiErr(fmt.Errorf("ht-ml.app 422 (content scan failed): %s\nhint: revise the HTML and try again", firstNonEmpty(msg, "unprocessable")))
	case http.StatusTooManyRequests: // 429
		return rateLimitErr(fmt.Errorf("ht-ml.app 429: %s", firstNonEmpty(msg, "rate limited")))
	default:
		return apiErr(fmt.Errorf("ht-ml.app HTTP %d: %s", status, firstNonEmpty(msg, string(body))))
	}
}

func extractAPIMessage(body []byte) string {
	var m struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &m) == nil {
		return firstNonEmpty(m.Message, m.Error, m.Detail)
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// uploadAssetMultipart POSTs a referenced asset to
// /sites/{id}/assets?relative_path=PATH as multipart/form-data (field name
// "file") with the per-site update_key as a Bearer token. It bypasses
// internal/client because that client JSON-encodes bodies; callers MUST guard
// with htmlxWriteGuard first so verify/dry-run never reach here.
func uploadAssetMultipart(ctx context.Context, baseURL, siteID, updateKey, relativePath string, fileData []byte, fileName string) (int, []byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return 0, nil, err
	}
	if _, err := fw.Write(fileData); err != nil {
		return 0, nil, err
	}
	if err := w.Close(); err != nil {
		return 0, nil, err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/sites/" + url.PathEscape(siteID) + "/assets?relative_path=" + url.QueryEscape(relativePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+updateKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, respBody, nil
}

// liveAssetPresent reports whether a site's asset actually resolves on the CDN.
// The ht-ml.app GET /v1/sites/{id} assets[].status field is unreliable (it can
// stay "missing" after a successful upload), so presence is verified by
// fetching the live asset URL — the authoritative signal.
func liveAssetPresent(ctx context.Context, siteID, relativePath string) (bool, int) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, liveAssetURL(siteID, relativePath), nil)
	if err != nil {
		return false, 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode
}

// fetchSiteAssets calls GET /v1/sites/{id} and returns its referenced-asset
// list. No auth needed for public sites.
func fetchSiteAssets(ctx context.Context, flags *rootFlags, siteID string) (*siteAssetsResponse, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.Get(ctx, "/sites/"+url.PathEscape(siteID), nil)
	if err != nil {
		return nil, err
	}
	var r siteAssetsResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing site assets: %w", err)
	}
	if r.SiteID == "" {
		r.SiteID = siteID
	}
	return &r, nil
}

// nowRFC3339 is a tiny indirection so command code reads cleanly.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// trimNL strips a single leading/trailing newline from raw string literals used
// for Cobra Long/Example fields so they render with correct indentation.
func trimNL(s string) string { return strings.Trim(s, "\n") }

// resolveAssetFile reads a referenced asset from a local root directory,
// returning its bytes and base filename. relativePath is the path as it appears
// in the HTML (and on the wire).
func resolveAssetFile(root, relativePath string) ([]byte, string, error) {
	clean := filepath.Clean(relativePath)
	full := filepath.Join(root, clean)
	// filepath.Clean normalises ".." segments but does not constrain the result
	// to root: a reference like "../../.env" resolves outside it. Reject anything
	// that escapes root rather than reading (and potentially uploading) it.
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, "", fmt.Errorf("asset path %q escapes root directory %q", relativePath, root)
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return nil, "", err
	}
	return b, filepath.Base(clean), nil
}
