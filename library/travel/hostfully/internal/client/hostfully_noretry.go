package client

import (
	"context"
	"encoding/json"
)

type noRetryKey struct{}

// WithNoRetry marks ctx so requests made with it get exactly one attempt.
func WithNoRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, noRetryKey{}, true)
}

func noRetryRequested(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(noRetryKey{}).(bool)
	return v
}

// GetNoRetry performs one uncached GET attempt. Callers that interpret a
// repeatable server error structurally (for example, bisecting a Hostfully
// leads page whose REST representation cannot serialize) use this instead of
// paying for retries that are guaranteed to fail the same way.
func (c *Client) GetNoRetry(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	result, _, err := c.do(WithNoRetry(ctx), "GET", path, params, nil, nil)
	return result, err
}
