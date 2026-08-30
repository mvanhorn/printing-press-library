// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/cliutil"
)

func mkResp(code int, retryAfter string) *http.Response {
	h := http.Header{}
	if retryAfter != "" {
		h.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: code, Header: h}
}

func TestHTTPStatusError(t *testing.T) {
	if err := httpStatusError(mkResp(200, ""), "X"); err != nil {
		t.Errorf("200 should be nil, got %v", err)
	}

	// 429 → typed RateLimitError with parsed Retry-After and supplier in URL.
	err := httpStatusError(mkResp(429, "30"), "Goldcar")
	if !IsRateLimit(err) {
		t.Fatalf("429 should be a RateLimitError, got %v", err)
	}
	var rl *cliutil.RateLimitError
	if !cliutilAs(err, &rl) || rl.RetryAfter != 30*time.Second || rl.URL != "Goldcar" {
		t.Errorf("unexpected rate-limit fields: %+v", rl)
	}

	// 503 without Retry-After → generic (not treated as throttling).
	if IsRateLimit(httpStatusError(mkResp(503, ""), "X")) {
		t.Error("503 without Retry-After should not be a RateLimitError")
	}
	// 503 with Retry-After → throttling.
	if !IsRateLimit(httpStatusError(mkResp(503, "5"), "X")) {
		t.Error("503 with Retry-After should be a RateLimitError")
	}

	// Other non-2xx → generic error, not rate-limit.
	err = httpStatusError(mkResp(500, ""), "X")
	if err == nil || IsRateLimit(err) {
		t.Errorf("500 should be a generic error, got %v", err)
	}
}

func cliutilAs(err error, target **cliutil.RateLimitError) bool {
	rl, ok := err.(*cliutil.RateLimitError)
	if ok {
		*target = rl
	}
	return ok
}
