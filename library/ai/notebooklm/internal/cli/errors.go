// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Documented exit codes for agents: code: 2 usage, code: 3 not found, code: 4 auth,
// code: 5 api, code: 6 conflict, code: 7 rate limit, code: 10 config.

type cliError struct {
	code int
	err  error
	hint string
}

func (e *cliError) Error() string {
	if e.hint != "" {
		return fmt.Sprintf("%s\nhint: %s", e.err.Error(), e.hint)
	}
	return e.err.Error()
}

func (e *cliError) Unwrap() error { return e.err }

// usageErr is code: 2 — invalid flags or arguments.
func usageErr(err error) error { return &cliError{code: 2, err: err} }

// notFoundErr is code: 3 — resource missing (HTTP 404).
func notFoundErr(err error) error {
	return &cliError{code: 3, err: err, hint: "Check the notebook id or title; run notebooklm-pp-cli notebook list --json"}
}

// authErr is code: 4 — missing or invalid session cookies.
func authErr(err error) error {
	return &cliError{code: 4, err: err, hint: "Try: run notebooklm-pp-cli auth login --chrome, then run notebooklm-pp-cli doctor"}
}

// apiErr is code: 5 — upstream API failure.
func apiErr(err error) error { return &cliError{code: 5, err: err} }

// conflictErr is code: 6 — resource already exists (HTTP 409).
func conflictErr(err error) error {
	return &cliError{code: 6, err: err, hint: "Resource already exists (409); treat as success when creating idempotently"}
}

// rateLimitErr is code: 7 — HTTP 429 with retry guidance.
func rateLimitErr(err error) error {
	return &cliError{code: 7, err: err, hint: "Rate limited (429); wait for Retry-After or retry with exponential backoff"}
}

// configErr is code: 10 — config file problems.
func configErr(err error) error {
	return &cliError{code: 10, err: err, hint: "Run notebooklm-pp-cli doctor to inspect config path and permissions"}
}

func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "404") || strings.Contains(msg, "not found") {
		return notFoundErr(err)
	}
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "not authenticated") {
		return authErr(err)
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return rateLimitErr(err)
	}
	if strings.Contains(msg, "409") || strings.Contains(msg, "already exists") {
		return conflictErr(err)
	}
	return apiErr(err)
}

func mapHTTPStatus(status int, body string) error {
	switch status {
	case http.StatusNotFound:
		return notFoundErr(fmt.Errorf("404 not found: %s", truncateErr(body, 120)))
	case http.StatusUnauthorized, http.StatusForbidden:
		return authErr(fmt.Errorf("auth failed: status %d", status))
	case http.StatusTooManyRequests:
		return rateLimitErr(fmt.Errorf("429 rate limited; Retry-After may apply"))
	default:
		if status >= 400 {
			return apiErr(fmt.Errorf("API error %d: %s", status, truncateErr(body, 120)))
		}
	}
	return nil
}

func truncateErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func exitCodeFor(err error) int {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	return 1
}
