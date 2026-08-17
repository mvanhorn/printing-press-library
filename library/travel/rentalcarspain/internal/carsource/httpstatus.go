// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/cliutil"
)

// IsRateLimit reports whether err is (or wraps) a throttling error
// (cliutil.RateLimitError). Callers use it to tell genuine 429/503 throttling
// apart from an empty result set — otherwise a throttle silently looks like
// "no cars available".
func IsRateLimit(err error) bool {
	var rl *cliutil.RateLimitError
	return errors.As(err, &rl)
}

// httpStatusError inspects a response status and returns a typed
// cliutil.RateLimitError for throttling (429, or 503 carrying Retry-After) and
// a generic error for any other non-2xx status. It returns nil for 2xx so
// callers can use it as a one-line guard. It does not close the body.
func httpStatusError(resp *http.Response, supplier string) error {
	switch {
	case resp.StatusCode/100 == 2:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return &cliutil.RateLimitError{URL: supplier, RetryAfter: cliutil.RetryAfter(resp)}
	case resp.StatusCode == http.StatusServiceUnavailable:
		// Treat 503 as throttling only when it carries a Retry-After (an
		// overload signal); a bare 503 is a generic server error.
		if resp.Header.Get("Retry-After") != "" {
			return &cliutil.RateLimitError{URL: supplier, RetryAfter: cliutil.RetryAfter(resp)}
		}
		return fmt.Errorf("%s HTTP %d", supplier, resp.StatusCode)
	default:
		return fmt.Errorf("%s HTTP %d", supplier, resp.StatusCode)
	}
}
