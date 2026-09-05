// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"io"
	"net/http"
	"strings"
)

// rateLimitBodySnippet caps how much of a 429 body is carried into the error.
// Genuine throttle responses are short; anything longer is a proxy's HTML
// error page, and quoting it in full would bury the retry-after the caller
// actually needs to act on.
const rateLimitBodySnippet = 512

// RateLimitFromResponse converts a 429 response into the typed *RateLimitError
// that callers match with errors.As.
//
// Call sites that drive a bare *http.Client -- paths that deliberately do not
// go through internal/client, which already classifies 429 itself -- must use
// this instead of folding 429 into a generic non-2xx error. A throttle and a
// rejected request are different conditions: the first says "this same request
// will succeed later", the second says "this request will never succeed".
// Collapsing them is what lets a date-walking retry loop keep hammering a host
// that just asked it to stop, and then report the damage as though every date
// had been individually malformed.
//
// The caller keeps ownership of resp.Body (it is expected to already hold a
// deferred Close); only a bounded snippet is consumed here.
func RateLimitFromResponse(resp *http.Response, url string) *RateLimitError {
	if resp == nil {
		return &RateLimitError{URL: url, RetryAfter: defaultRetryWait}
	}
	err := &RateLimitError{URL: url, RetryAfter: RetryAfter(resp)}
	if resp.Body != nil {
		if snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, rateLimitBodySnippet)); readErr == nil {
			err.Body = SanitizeErrorBody(strings.TrimSpace(string(snippet)))
		}
	}
	return err
}
