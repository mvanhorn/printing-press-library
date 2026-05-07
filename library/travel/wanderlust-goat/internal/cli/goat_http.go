package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// httpGetJSON does a User-Agent-stamped GET and returns the body.
// Used by the goat orchestrator to call Nominatim directly without
// pulling in the typed client (Nominatim already has its own typed
// path through the spec-generated client; this is a one-off geocode).
func httpGetJSON(ctx context.Context, url, ua string, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	c := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, truncBody(body))
	}
	return body, nil
}

func truncBody(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

func jsonUnmarshal(body []byte, dst any) error {
	return json.Unmarshal(body, dst)
}
