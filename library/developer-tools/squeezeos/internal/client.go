package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const DefaultBase = "https://squeezeos-api.onrender.com"

type Client struct {
	Base       string
	Token      string
	HTTPClient *http.Client
}

func NewClient() *Client {
	base := os.Getenv("SQUEEZEOS_BASE_URL")
	if base == "" {
		base = DefaultBase
	}
	return &Client{
		Base:       base,
		Token:      os.Getenv("SQUEEZEOS_TOKEN"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Get(path string) (map[string]any, error) {
	req, err := http.NewRequest("GET", c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("X-Payment-Token", c.Token)
	}
	req.Header.Set("User-Agent", "squeezeos-pp-cli/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decode(resp)
}

func (c *Client) Post(path string, body any) (map[string]any, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.Base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("X-Payment-Token", c.Token)
	}
	req.Header.Set("User-Agent", "squeezeos-pp-cli/1.0")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decode(resp)
}

func decode(resp *http.Response) (map[string]any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode: %w\nbody: %s", err, string(body))
	}
	return out, nil
}

func Print(w io.Writer, v any, compact bool) error {
	enc := json.NewEncoder(w)
	if !compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}
