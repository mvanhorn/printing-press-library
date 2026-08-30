package graphqlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/bestfoodtrucks/internal/cliutil"
)

const DefaultEndpoint = "https://api.bestfoodtrucks.com/graphql"

type Client struct {
	endpoint   string
	httpClient *http.Client
	limiter    *cliutil.AdaptiveLimiter
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		endpoint:   DefaultEndpoint,
		httpClient: &http.Client{Timeout: timeout},
		// No ceiling: this API has shown zero rate-limiting evidence across
		// extensive discovery testing (no 429s, no X-Ratelimit-* headers).
		// Start reasonably fast and let the adaptive limiter ramp up freely
		// on sustained success; it still backs off automatically the moment
		// a real 429 is observed.
		limiter: cliutil.NewAdaptiveLimiterAuto(20.0),
	}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors,omitempty"`
}

// Query executes a GraphQL query/mutation and unmarshals the "data" field into result.
// Always sends the full query text (never a persisted-query hash) so it never
// depends on the server's Apollo Automatic Persisted Query cache.
func (c *Client) Query(ctx context.Context, query string, variables map[string]any, result any) error {
	reqBody, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("marshaling graphql request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("building graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	c.limiter.Wait()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling bestfoodtrucks graphql api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return &cliutil.RateLimitError{
			URL:        c.endpoint,
			RetryAfter: cliutil.RetryAfter(resp),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading graphql response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("bestfoodtrucks graphql api returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	c.limiter.OnSuccess()

	var gr gqlResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return fmt.Errorf("parsing graphql response: %w", err)
	}
	if len(gr.Errors) > 0 {
		return fmt.Errorf("bestfoodtrucks graphql error: %s", gr.Errors[0].Message)
	}
	if result != nil && len(gr.Data) > 0 {
		if err := json.Unmarshal(gr.Data, result); err != nil {
			return fmt.Errorf("decoding graphql data: %w", err)
		}
	}
	return nil
}
