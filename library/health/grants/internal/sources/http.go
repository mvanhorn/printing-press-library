package sources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Keyless, no exec.Command — every call is direct HTTP (retraction-checker pattern).
var client = &http.Client{Timeout: 20 * time.Second}

const userAgent = "grants-pp-cli/1.0 (keyless research tool)"

// do executes a request with one retry on network error / 5xx.
func do(build func() (*http.Request, error)) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Second)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: read: %w", req.URL.Host, err)
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("%s: HTTP %d: %.200s", req.URL.Host, resp.StatusCode, body)
			time.Sleep(time.Second)
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%s: HTTP %d: %.200s", req.URL.Host, resp.StatusCode, body)
		}
		return body, nil
	}
	return nil, lastErr
}

func getJSON(url string, out any) error {
	body, err := do(func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, url, nil)
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func postJSON(url string, payload, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body, err := do(func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
