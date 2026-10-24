// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

// Once the 5xx retry got an answer (here a redirect), a later pacing
// refusal is reported as itself, not as the old 5xx.
func TestRetryThenRedirectReportsThePacingRefusal(t *testing.T) {
	c, _, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request, n int) {
		switch n {
		case 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			w.Header().Set("Location", "https://apps.apple.com/us/iphone/charts/6014")
			w.WriteHeader(http.StatusMovedPermanently)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}, func(o *Options) {
		o.NoCache = true
		o.Jitter = -1
		o.RetryDelay = 10 * time.Millisecond
		o.Rates = map[RateClass]float64{RatePages: 2} // 500 ms apart
		o.WaitUntil = time.Now().Add(700 * time.Millisecond)
	})
	_, err := c.ChartsPage(context.Background(), ChartsPageQuery{GenreID: "36"})
	var pe *PacingError
	if !errors.As(err, &pe) {
		t.Fatalf("want the pacing refusal for the redirect hop, got %T %v", err, err)
	}
}
