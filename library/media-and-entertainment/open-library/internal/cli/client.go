// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://openlibrary.org"
	baseURLEnv      = "OPEN_LIBRARY_BASE_URL"
	userAgentEnv    = "OPEN_LIBRARY_USER_AGENT"
	contactEmailEnv = "OPEN_LIBRARY_CONTACT_EMAIL"
)

type openLibraryClient struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
	identified bool
}

var newHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func newOpenLibraryClient(timeout time.Duration) *openLibraryClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	baseURL := strings.TrimRight(firstNonEmpty(env(baseURLEnv), defaultBaseURL), "/")
	contact := strings.TrimSpace(env(contactEmailEnv))
	userAgent := firstNonEmpty(env(userAgentEnv), "open-library-pp-cli/1.0")
	identified := contact != ""
	if contact != "" && !strings.Contains(userAgent, contact) {
		userAgent = fmt.Sprintf("%s (%s)", userAgent, contact)
	}
	return &openLibraryClient{
		baseURL:    baseURL,
		httpClient: newHTTPClient(timeout),
		userAgent:  userAgent,
		identified: identified,
	}
}

func (c *openLibraryClient) getJSON(ctx context.Context, path string, query url.Values, dest any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("open library request failed: %s %s", resp.Status, u.Path)
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode Open Library response: %w", err)
	}
	return nil
}

func sourceCaveats(identified bool) []string {
	caveats := []string{
		"Open Library asks API clients not to scrape HTML and not to harvest data in bulk; use monthly data dumps for bulk access.",
		"Metadata can vary by work and edition and should be cited with Open Library source URLs.",
	}
	if identified {
		caveats = append(caveats, "Requests include a contact-bearing User-Agent and should stay near the documented identified-client limit of 3 requests per second.")
	} else {
		caveats = append(caveats, "OPEN_LIBRARY_CONTACT_EMAIL is not set, so keep requests near the documented non-identified limit of 1 request per second.")
	}
	return caveats
}

func freshnessNote() string {
	return "Live Open Library JSON response at request time; Open Library metadata can be revised by community edits."
}
