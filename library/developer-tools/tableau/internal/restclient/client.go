package client

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultAPIVersion is the Tableau REST API version used when none is set.
const DefaultAPIVersion = "3.21"

// pageSize is the page size for paginated list calls.
const pageSize = 100

// Config holds connection settings for the Tableau REST client.
// Never log PatSecret.
type Config struct {
	Server         string // full origin, e.g. https://10ay.online.tableau.com
	SiteContentURL string // empty = default site
	PatName        string
	PatSecret      string
	APIVersion     string
	HTTPClient     *http.Client
}

// Client is a Tableau REST API client authenticated via PAT.
type Client struct {
	cfg  Config
	http *http.Client
	cred *Credentials
}

// New creates a Client. Call SignIn before site-scoped operations.
func New(cfg Config) (*Client, error) {
	if cfg.Server == "" {
		return nil, fmt.Errorf("server URL is required")
	}
	server := strings.TrimRight(cfg.Server, "/")
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return nil, fmt.Errorf("server URL must include scheme (https://...): %q", cfg.Server)
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	cfg.Server = server
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{cfg: cfg, http: hc}, nil
}

// Credentials returns the current session credentials, or nil if not signed in.
func (c *Client) Credentials() *Credentials {
	return c.cred
}

// Config returns a copy of the client config with PatSecret cleared.
// Safe for logging / display.
func (c *Client) Config() Config {
	cp := c.cfg
	cp.PatSecret = ""
	return cp
}

// SignIn authenticates with a personal access token.
func (c *Client) SignIn() (*Credentials, error) {
	if c.cfg.PatName == "" || c.cfg.PatSecret == "" {
		return nil, fmt.Errorf("PAT name and secret are required (flags --pat-name/--pat-secret or env TABLEAU_PAT_NAME/TABLEAU_PAT_SECRET)")
	}
	body := BuildSignInPATRequest(c.cfg.PatName, c.cfg.PatSecret, c.cfg.SiteContentURL)
	req, err := http.NewRequest(http.MethodPost, c.apiURL("/auth/signin"), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("Accept", "application/xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sign-in request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sign-in response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if apiErr := ParseErrorResponse(bytes.NewReader(data)); apiErr != nil {
			return nil, fmt.Errorf("sign-in failed (HTTP %d): %w", resp.StatusCode, apiErr)
		}
		return nil, fmt.Errorf("sign-in failed (HTTP %d): %s", resp.StatusCode, truncate(string(data), 300))
	}
	cred, err := ParseSignInResponse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.cred = cred
	return cred, nil
}

// SignOut invalidates the current session token.
func (c *Client) SignOut() error {
	if c.cred == nil || c.cred.Token == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, c.apiURL("/auth/signout"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Tableau-Auth", c.cred.Token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sign-out request: %w", err)
	}
	defer resp.Body.Close()
	c.cred = nil
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sign-out failed (HTTP %d)", resp.StatusCode)
	}
	return nil
}

// EnsureSignedIn signs in if there is no active session.
func (c *Client) EnsureSignedIn() error {
	if c.cred != nil && c.cred.Token != "" && c.cred.SiteID != "" {
		return nil
	}
	_, err := c.SignIn()
	return err
}

func (c *Client) apiURL(path string) string {
	path = strings.TrimPrefix(path, "/")
	return fmt.Sprintf("%s/api/%s/%s", c.cfg.Server, c.cfg.APIVersion, path)
}

func (c *Client) siteURL(path string) string {
	path = strings.TrimPrefix(path, "/")
	return c.apiURL(fmt.Sprintf("sites/%s/%s", c.cred.SiteID, path))
}

// doAuth performs an authenticated request and returns status + body.
func (c *Client) doAuth(method, rawURL string, body io.Reader, contentType string) (int, []byte, error) {
	if err := c.EnsureSignedIn(); err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Tableau-Auth", c.cred.Token)
	req.Header.Set("Accept", "application/xml")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, data, nil
}

// getAllPages fetches all pages of a list endpoint.
// fetchPage receives pageNumber (1-based) and returns items, totalAvailable (or -1 if unknown), error.
func getAllPages[T any](fetchPage func(page int) ([]T, int, error)) ([]T, error) {
	var all []T
	page := 1
	for {
		items, total, err := fetchPage(page)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) == 0 {
			break
		}
		if total >= 0 && len(all) >= total {
			break
		}
		if len(items) < pageSize {
			break
		}
		page++
	}
	return all, nil
}

func paginationTotal(p *xmlPagination) int {
	if p == nil || p.TotalAvailable == "" {
		return -1
	}
	n, err := strconv.Atoi(p.TotalAvailable)
	if err != nil {
		return -1
	}
	return n
}

func withPageQuery(rawURL string, page int) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("pageNumber", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
