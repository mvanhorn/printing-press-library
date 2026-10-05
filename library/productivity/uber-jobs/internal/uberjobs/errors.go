// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package uberjobs holds the hand-written data layer for uber-jobs-pp-cli:
// the paced single-attempt client for the Uber careers site, the Oracle
// candidate-experience fallback, posting normalization, and the local store
// tables that new-since, check, screen, and stats read.
package uberjobs

import (
	"errors"
	"fmt"
	"net"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
)

// RefusalError means a host refused the request: HTTP 403, HTTP 429, or a
// bot-protection challenge page. It is never retried. It unwraps to
// *cliutil.RateLimitError so the CLI maps it to the rate-limit exit code.
type RefusalError struct {
	URL       string
	Status    int
	Challenge bool
	Host      string
	// Latched means no request was sent: an earlier refusal from this host,
	// recorded by any process, is still inside its cooldown.
	Latched   bool
	LatchedAt string
	Until     string
	LatchFile string
	rate      *cliutil.RateLimitError
}

// NewRefusal builds a refusal whose Unwrap reaches a *cliutil.RateLimitError.
// Always build refusals through it: a zero rate field breaks errors.As.
func NewRefusal(url, host string, status int, challenge bool, body string) *RefusalError {
	return newRefusal(url, host, status, challenge, body)
}

func newRefusal(url, host string, status int, challenge bool, body string) *RefusalError {
	return &RefusalError{
		URL:       url,
		Status:    status,
		Challenge: challenge,
		Host:      host,
		rate:      &cliutil.RateLimitError{URL: url, Body: body},
	}
}

func (e *RefusalError) Error() string {
	kind := fmt.Sprintf("HTTP %d", e.Status)
	if e.Challenge {
		kind = fmt.Sprintf("a bot-protection challenge (HTTP %d)", e.Status)
	}
	if e.Latched {
		return fmt.Sprintf("%s refused a request at %s with %s; nothing is sent to it until %s (delete %s to allow a request sooner)", e.Host, e.LatchedAt, kind, e.Until, e.LatchFile)
	}
	return fmt.Sprintf("%s refused the request with %s; not retrying", e.Host, kind)
}

func (e *RefusalError) Unwrap() error {
	if e.rate == nil {
		return nil
	}
	return e.rate
}

// TransportError is a DNS, connection, TLS, or timeout failure before any
// HTTP response arrived. DNS is set when the resolver could not resolve the host.
type TransportError struct {
	URL string
	DNS bool
	Err error
}

func (e *TransportError) Error() string {
	if e.DNS {
		return fmt.Sprintf("DNS lookup failed for %s: %v", e.URL, e.Err)
	}
	return fmt.Sprintf("network error reaching %s: %v", e.URL, e.Err)
}

func (e *TransportError) Unwrap() error { return e.Err }

// StatusError is any other non-2xx HTTP response.
type StatusError struct {
	URL    string
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s answered HTTP %d", e.URL, e.Status)
}

// ContentError means a 2xx response failed the positive content check, so it
// is never treated as an empty result.
type ContentError struct {
	URL    string
	Reason string
}

func (e *ContentError) Error() string {
	return fmt.Sprintf("unexpected content from %s: %s", e.URL, e.Reason)
}

// NotFoundError means the requested posting id is not listed.
type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("posting %s is not listed on the Uber careers site", e.ID)
}

// IsRefusal reports whether err is (or wraps) a refusal.
func IsRefusal(err error) bool {
	var r *RefusalError
	return errors.As(err, &r)
}

// IsTransport reports whether err is (or wraps) a transport failure.
func IsTransport(err error) bool {
	var t *TransportError
	return errors.As(err, &t)
}

func classifyTransport(url string, err error) *TransportError {
	var dnsErr *net.DNSError
	return &TransportError{URL: url, DNS: errors.As(err, &dnsErr), Err: err}
}
