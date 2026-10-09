// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"
)

// Each source failure must tell the reader what to do next, not only what
// went wrong.
func TestClassifyLockerErrHints(t *testing.T) {
	timeout := &url.Error{Op: "Get", URL: "https://www.coinlocker-navi.com/search", Err: context.DeadlineExceeded}
	dns := &url.Error{Op: "Get", URL: "https://api.multiecube.com/v1/location/ph2", Err: &net.DNSError{Err: "no such host", Name: "api.multiecube.com"}}
	cases := []struct {
		name string
		err  error
		code int
		hint string
	}{
		{"timeout", timeout, 5, "--timeout"},
		{"wrapped timeout", fmt.Errorf("page 2: %w", context.DeadlineExceeded), 5, "--timeout"},
		{"network", dns, 5, "doctor"},
		{"429", &cliutil.RateLimitError{URL: "https://www.coinlocker-navi.com/search"}, 7, "wait"},
		{"403", &locker.HTTPError{URL: "https://www.coinlocker-navi.com/cl/1", Status: 403}, 7, "wait"},
		{"503", &locker.HTTPError{URL: "https://api.multiecube.com/v1/location/ph2", Status: 503}, 5, "retry later"},
		{"404", locker.ErrNotFound, 3, "near"},
	}
	for _, c := range cases {
		got := classifyLockerErr("source", c.err)
		var ce *cliError
		if !errors.As(got, &ce) {
			t.Fatalf("%s: not a cliError: %v", c.name, got)
		}
		if ce.code != c.code {
			t.Errorf("%s: exit code %d, want %d", c.name, ce.code, c.code)
		}
		msg := got.Error()
		if !strings.Contains(msg, "\nhint: ") || !strings.Contains(msg, c.hint) {
			t.Errorf("%s: message lacks a hint naming %q: %q", c.name, c.hint, msg)
		}
		if !errors.Is(got, c.err) {
			t.Errorf("%s: cause not kept", c.name)
		}
	}
}
