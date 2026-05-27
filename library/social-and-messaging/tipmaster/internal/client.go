package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const DefaultBase = "https://tipmaster.onrender.com"

type Client struct {
	Base       string
	HTTPClient *http.Client
}

func NewClient() *Client {
	base := os.Getenv("TIPMASTER_BASE_URL")
	if base == "" {
		base = DefaultBase
	}
	return &Client{
		Base:       base,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Get(path string) (map[string]any, error) {
	req, err := http.NewRequest("GET", c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tipmaster-pp-cli/1.0")
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
