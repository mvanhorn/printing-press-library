package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "fellow-stagg-ekg-pp-cli/1.0"

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Fetch(command string) (string, error) {
	endpoint, err := commandURL(c.baseURL, command)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}

	text := strings.TrimSpace(string(body))
	if resp.StatusCode >= http.StatusBadRequest {
		if text == "" {
			text = resp.Status
		}
		return "", fmt.Errorf("%s: HTTP %d %s: %s", command, resp.StatusCode, resp.Status, text)
	}

	return text, nil
}

func commandURL(baseURL, command string) (string, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		return "", fmt.Errorf("missing base URL")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/cli"
	values := parsed.Query()
	values.Set("cmd", command)
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

