// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Extron literature client (not generator-emitted). The Extron
// literature library is a public website; this package fetches the
// server-rendered alphabetical index pages and downloads PDF documents.

package extron

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil"
)

// Doc is one row of the Extron literature index: a spec sheet, manual,
// declaration, guide, or BIM family with its download URL and metadata.
type Doc struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Rev      string `json:"rev"`
	Date     string `json:"date"`
	Size     string `json:"size"`
	Type     string `json:"type"`
	URL      string `json:"url"`
}

// DefaultLetters are the alphabetical index buckets the site exposes:
// "#" (id=0, digit/other titles) plus A-Z.
var DefaultLetters = func() []string {
	letters := []string{"0"}
	for c := 'A'; c <= 'Z'; c++ {
		letters = append(letters, string(c))
	}
	return letters
}()

// Categories are the literature types the site renders on each index page.
var Categories = []string{"Brochure", "Declaration of Conformity", "Design Guide", "Product Guide", "Manual", "Revit BIM"}

const (
	indexPath = "/technology/literature.aspx"
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ErrReset is a retryable transport failure: Extron's WAF intermittently
// resets connections for non-browser clients, and a retry normally succeeds.
var ErrReset = errors.New("connection reset by upstream WAF")

// Client fetches Extron literature index pages and PDFs.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

// New returns a Client with a browser-fingerprinted HTTP stack and adaptive
// per-request pacing.
func New() *Client {
	tr := &http.Transport{
		ForceAttemptHTTP2: false,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	}
	return &Client{
		BaseURL: "https://www.extron.com",
		HTTP: &http.Client{
			Transport: tr,
			Timeout:   45 * time.Second,
		},
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

func (c *Client) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	return req, nil
}

// doGet performs a GET with one automatic retry on connection reset, the
// observed WAF failure mode. 429s feed the adaptive limiter and surface as a
// typed RateLimitError after one retry.
func (c *Client) doGet(ctx context.Context, rawURL string) (*http.Response, error) {
	c.limiter.Wait()
	for attempt := 0; attempt < 2; attempt++ {
		req, err := c.newRequest(ctx, rawURL)
		if err != nil {
			return nil, err
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
			}
			if isReset(err) {
				// WAF reset: brief pause, then one retry.
				select {
				case <-time.After(800 * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			c.limiter.OnRateLimit()
			retryAfter := cliutil.RetryAfter(resp)
			resp.Body.Close()
			return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: retryAfter}
		}
		c.limiter.OnSuccess()
		return resp, nil
	}
	return nil, fmt.Errorf("fetching %s: %w", rawURL, ErrReset)
}

func isReset(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "read tcp") && strings.Contains(msg, "reset")
}

// IndexURL builds the alphabetical index URL for a letter (0-9, A-Z, or All).
func (c *Client) IndexURL(letter string) string {
	v := url.Values{}
	v.Set("tabid", "5")
	v.Set("defaultLang", "true")
	if letter == "" {
		letter = "All"
	}
	v.Set("id", letter)
	return c.BaseURL + indexPath + "?" + v.Encode()
}

// FetchIndex fetches and parses one alphabetical index page, returning the
// docs and the per-category pagination refs (page=2 "Next" links).
func (c *Client) FetchIndex(ctx context.Context, letter string) ([]Doc, map[string]PageRef, error) {
	rawURL := c.IndexURL(letter)
	resp, err := c.doGet(ctx, rawURL)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("literature index %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	docs, refs, err := ParseIndexWithRefs(body)
	return docs, refs, err
}

// CategoryPageURL builds the URL for a paginated single-category page.
func (c *Client) CategoryPageURL(letter string, ref PageRef) string {
	v := url.Values{}
	v.Set("tabid", "5")
	v.Set("defaultLang", "true")
	if letter == "" {
		letter = "All"
	}
	v.Set("id", letter)
	if ref.Filetype != "" {
		v.Set("filetype", ref.Filetype)
	}
	if ref.Tabid != "" {
		v.Set("tabid", ref.Tabid)
	}
	if ref.Page > 1 {
		v.Set("page", fmt.Sprintf("%d", ref.Page))
	}
	return c.BaseURL + indexPath + "?" + v.Encode()
}

// FetchCategoryPage fetches one paginated category page (20 rows) and tags
// every row with the given category. hasNext reports whether the page carries
// another "Next" link (i.e. a further page exists).
func (c *Client) FetchCategoryPage(ctx context.Context, letter string, ref PageRef) ([]Doc, bool, error) {
	rawURL := c.CategoryPageURL(letter, ref)
	resp, err := c.doGet(ctx, rawURL)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, false, fmt.Errorf("reading %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("literature category page %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	docs, err := ParseCategoryPage(body, ref.Category)
	return docs, firstNextHref(string(body)) != "", err
}

// Download streams a literature PDF to dest and returns the number of bytes
// written. The destination directory is created if missing.
func (c *Client) Download(ctx context.Context, docURL, dest string) (int64, error) {
	resp, err := c.doGet(ctx, docURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download %s returned HTTP %d", docURL, resp.StatusCode)
	}
	// Only accept PDF payloads: a 200 HTML error page or challenge body must
	// not be written to disk or recorded as a valid document.
	head := make([]byte, 5)
	hn, err := io.ReadFull(resp.Body, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, fmt.Errorf("reading %s: %w", docURL, err)
	}
	head = head[:hn]
	if !strings.HasPrefix(string(head), "%PDF") {
		return 0, fmt.Errorf("download %s returned a non-PDF body (content-type %q); not writing it to disk", docURL, resp.Header.Get("Content-Type"))
	}
	body := io.MultiReader(bytes.NewReader(head), resp.Body)
	dir := filepath.Dir(dest)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, fmt.Errorf("creating download dir: %w", err)
		}
	}
	// Refuse to write through a pre-planted symlink at the destination.
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("refusing to overwrite symlink at %s", dest)
	}
	// Stream to a temp file first, then rename into place: an interrupted
	// stream (WAF reset, timeout) leaves no truncated file at the final path.
	tmp, err := os.CreateTemp(dir, ".download-*.part")
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	n, err := io.Copy(tmp, body)
	if err != nil {
		tmp.Close()
		return n, fmt.Errorf("writing %s: %w", dest, err)
	}
	if err := tmp.Close(); err != nil {
		return n, fmt.Errorf("closing %s: %w", dest, err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return n, fmt.Errorf("finalizing %s: %w", dest, err)
	}
	return n, nil
}

// AbsoluteURL resolves a possibly-relative download path against the base.
func (c *Client) AbsoluteURL(docURL string) string {
	if strings.HasPrefix(docURL, "http://") || strings.HasPrefix(docURL, "https://") {
		return docURL
	}
	return c.BaseURL + "/" + strings.TrimPrefix(docURL, "/")
}
