// uk-train-goat hand-authored: RJFAF feed download, authentication, and
// freshness probe for the National Rail Open Data portal.
package fares

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/cliutil"
)

// portalBase is the root URL of the National Rail Open Data portal.
// Tests override this to point at an httptest.Server.
var portalBase = "https://opendata.nationalrail.co.uk"

// sequenceRe extracts the numeric sequence suffix from an RJFAF filename,
// e.g. "RJFAF999.ZIP" -> "999".
var sequenceRe = regexp.MustCompile(`(?i)^RJFAF(\d+)\.ZIP$`)

// Authenticate posts credentials to the portal and returns a session token.
// When running under the printing-press verifier (PRINTING_PRESS_VERIFY=1),
// it returns a synthetic token immediately without any network call.
func Authenticate(ctx context.Context, user, pass string) (token string, err error) {
	if cliutil.IsVerifyEnv() {
		return "verify-token", nil
	}

	body := url.Values{"username": {user}, "password": {pass}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, portalBase+"/authenticate", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("authenticate: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authenticate: server returned %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("authenticate: decode response: %w", err)
	}
	return result.Token, nil
}

// FetchFeed downloads the RJFAF fares zip to a temp file and returns its path
// along with metadata parsed from the response headers. The caller is
// responsible for removing the temp file when it is no longer needed.
//
// When running under the printing-press verifier (PRINTING_PRESS_VERIFY=1),
// it returns empty values immediately without any network call.
func FetchFeed(ctx context.Context, token string) (zipPath string, meta FeedMeta, err error) {
	if cliutil.IsVerifyEnv() {
		return "", FeedMeta{}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, portalBase+"/api/staticfeeds/2.0/fares", nil)
	if err != nil {
		return "", FeedMeta{}, fmt.Errorf("fetch feed: build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", FeedMeta{}, fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", FeedMeta{}, fmt.Errorf("fetch feed: server returned %d", resp.StatusCode)
	}

	// Parse sequence from Content-Disposition before streaming so we can set
	// meta even if the file write fails.
	meta.LastModified = resp.Header.Get("Last-Modified")
	meta.Sequence = parseSequence(resp.Header.Get("Content-Disposition"))

	f, err := os.CreateTemp("", "rjfaf-*.zip")
	if err != nil {
		return "", FeedMeta{}, fmt.Errorf("fetch feed: create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", FeedMeta{}, fmt.Errorf("fetch feed: stream to temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", FeedMeta{}, fmt.Errorf("fetch feed: close temp file: %w", err)
	}

	return f.Name(), meta, nil
}

// ProbeFreshness issues a HEAD request to the fares endpoint and returns the
// Last-Modified response header without downloading the feed body.
//
// When running under the printing-press verifier (PRINTING_PRESS_VERIFY=1),
// it returns an empty string immediately without any network call.
func ProbeFreshness(ctx context.Context, token string) (lastModified string, err error) {
	if cliutil.IsVerifyEnv() {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, portalBase+"/api/staticfeeds/2.0/fares", nil)
	if err != nil {
		return "", fmt.Errorf("probe freshness: build request: %w", err)
	}
	req.Header.Set("X-Auth-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("probe freshness: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("probe freshness: server returned %d", resp.StatusCode)
	}

	return resp.Header.Get("Last-Modified"), nil
}

// parseSequence extracts the numeric suffix from an RJFAF Content-Disposition
// filename, e.g. `attachment; filename="RJFAF999.ZIP"` returns "999".
// Returns "" when the header is missing or does not match the expected pattern.
func parseSequence(header string) string {
	if header == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(header)
	if err != nil {
		return ""
	}
	filename := params["filename"]
	if m := sequenceRe.FindStringSubmatch(filename); m != nil {
		return m[1]
	}
	return ""
}
