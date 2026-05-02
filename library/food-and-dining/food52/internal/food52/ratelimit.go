// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package food52

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/cliutil"
)

// foodLimiter paces outbound requests to food52.com surfaces (Typesense and
// the public _app.js bundle for discovery). Started conservatively at
// 5 req/s — Food52 is on Vercel; AdaptiveLimiter ramps up on sustained
// success and halves on 429. Per-process; not persisted.
var foodLimiter = cliutil.NewAdaptiveLimiter(5)

// doWithLimiter runs the request through the package limiter and the
// 429-retry contract: it ramps on success, halves on 429, retries up to
// twice with Retry-After, and returns a typed *cliutil.RateLimitError
// when retries are exhausted. Callers must surface that error rather
// than treating empty-on-throttle as "no data."
func doWithLimiter(httpc HTTPClient, req *http.Request) (*http.Response, []byte, error) {
	const maxRetries = 2
	for attempt := 0; ; attempt++ {
		foodLimiter.Wait()
		resp, err := httpc.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return resp, nil, readErr
		}
		if resp.StatusCode == 429 {
			foodLimiter.OnRateLimit()
			if attempt >= maxRetries {
				return resp, body, &cliutil.RateLimitError{
					URL:        req.URL.String(),
					RetryAfter: cliutil.RetryAfter(resp),
					Body:       truncateBody(body),
				}
			}
			time.Sleep(cliutil.RetryAfter(resp))
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			foodLimiter.OnSuccess()
		}
		return resp, body, nil
	}
}

func truncateBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
