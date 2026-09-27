package jalan

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jalan/internal/cliutil"
)

const sourceBaseURL = "https://www.jalan.net"
const maxResponseBytes = 4 << 20
const maxCacheAge = 5 * time.Minute

type Options struct {
	Timeout      time.Duration
	MaxAge       time.Duration
	Refresh      bool
	DisableCache bool
	CacheDir     string
}

type Response struct {
	Meta          map[string]any   `json:"meta"`
	Results       []any            `json:"results"`
	Pagination    map[string]any   `json:"pagination"`
	FetchFailures []map[string]any `json:"fetch_failures"`
}

// Error preserves actionable source/usage failures without leaking raw HTML.
type Error struct {
	Code          string           `json:"code"`
	Message       string           `json:"message"`
	Hint          string           `json:"hint,omitempty"`
	URL           string           `json:"url,omitempty"`
	Cause         error            `json:"-"`
	FetchFailures []map[string]any `json:"fetch_failures,omitempty"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }
func usage(code, message, hint string) *Error {
	return &Error{Code: code, Message: message, Hint: hint}
}

// PartialError returns the observed subset and keeps every failed alternative.
// Unwrap retains a source rate-limit error for callers using errors.As.
type PartialError struct {
	Failures []map[string]any
	Cause    error
}

func (e *PartialError) Error() string {
	return fmt.Sprintf("partial source coverage: %d fetch failure(s)", len(e.Failures))
}
func (e *PartialError) Unwrap() error { return e.Cause }

// Client performs anonymous, bounded read-only source requests. HTTP is exposed
// for callers that need a custom transport; no credentials or cookies are used.
type Client struct {
	HTTP      *http.Client
	limiter   *cliutil.AdaptiveLimiter
	options   Options
	optionErr error
	baseURL   string
	semaphore chan struct{}
	now       func() time.Time
}

func NewClient(options Options) *Client {
	c := &Client{options: options, baseURL: sourceBaseURL, now: time.Now, limiter: cliutil.NewAdaptiveLimiter(2), semaphore: make(chan struct{}, 2)}
	if options.Timeout < 0 {
		c.optionErr = usage("invalid_query", "timeout cannot be negative", "Use a positive timeout no greater than 60s.")
	}
	if options.Timeout == 0 {
		c.options.Timeout = 60 * time.Second
	}
	if options.Timeout > 60*time.Second {
		c.options.Timeout = 60 * time.Second
	}
	if options.MaxAge < 0 || options.MaxAge > maxCacheAge {
		c.optionErr = usage("invalid_query", "max-age must be between 0 and 5m", "Inventory is live by default; opt in with --max-age 1m..5m.")
	}
	c.HTTP = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return &Error{Code: "access_failure", Message: "source exceeded redirect limit", URL: req.URL.String()}
		}
		if req.URL.Scheme != "https" || req.URL.Host != "www.jalan.net" {
			return &Error{Code: "access_failure", Message: "source redirected outside the anonymous Jalan origin", URL: req.URL.String()}
		}
		return nil
	}}
	return c
}

func (c *Client) commandContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if c.optionErr != nil {
		return nil, nil, c.optionErr
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bounded, cancel := context.WithTimeout(ctx, c.options.Timeout)
	return bounded, cancel, nil
}

func failure(err error, sourceURL string) map[string]any {
	out := map[string]any{"code": "fetch_failure", "message": err.Error(), "url": sourceURL}
	var typed *Error
	var rate *cliutil.RateLimitError
	if errors.As(err, &rate) {
		out["code"] = "rate_limit"
		out["retry_after_ms"] = rate.RetryAfter.Milliseconds()
	} else if errors.As(err, &typed) {
		out["code"] = typed.Code
		if typed.Hint != "" {
			out["hint"] = typed.Hint
		}
	}
	return out
}
