// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"fmt"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/cliutil"
)

func TestDoyouspainRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"redirect token missing", fmt.Errorf("could not find results redirect token"), true},
		{"transient network", fmt.Errorf("search POST: connection reset"), true},
		{"server 503", fmt.Errorf("doyouspain results HTTP 503"), true},
		{"WAF 406 not retryable", fmt.Errorf("doyouspain returned HTTP 406 (WAF)"), false},
		{"rate limit not retryable", &cliutil.RateLimitError{URL: "doyouspain"}, false},
	}
	for _, c := range cases {
		if got := doyouspainRetryable(c.err); got != c.want {
			t.Errorf("%s: doyouspainRetryable = %v, want %v", c.name, got, c.want)
		}
	}
}
