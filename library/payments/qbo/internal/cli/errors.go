// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/qbo/internal/cliutil"
	"os"
	"strings"
)

var As = errors.As

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErr(err error) error     { return &cliError{code: 2, err: err} }
func notFoundErr(err error) error  { return &cliError{code: 3, err: err} }
func authErr(err error) error      { return &cliError{code: 4, err: err} }
func apiErr(err error) error       { return &cliError{code: 5, err: err} }
func configErr(err error) error    { return &cliError{code: 10, err: err} }
func rateLimitErr(err error) error { return &cliError{code: 7, err: err} }

// partialFailureErr signals that the upstream API returned a 2xx with a
// body shape indicating some operations in a batch failed (e.g. Google
// Ads `partialFailureError`, similar shapes from Drive batch, Sheets
// batchUpdate, Cloud Resource Manager). Distinct from apiErr (HTTP-level
// failure) so callers can distinguish "request rejected" from "request
// accepted but some ops failed".
func partialFailureErr(err error) error { return &cliError{code: 6, err: err} }

// partialFailureReport describes the structured detection result for a
// mutate-style response body. Emitted in the envelope under
// "partial_failure" so machine-readable callers can route per-operation
// remediation.
type partialFailureReport struct {
	Field         string   `json:"field"`
	Message       string   `json:"message,omitempty"`
	Code          int      `json:"code,omitempty"`
	Details       any      `json:"details,omitempty"`
	ResourceNames []string `json:"resource_names,omitempty"`
}

// detectPartialFailure inspects a mutate-style JSON response for a
// partial-failure-shaped field. Returns nil when no partial failure is
// detected. The detector is intentionally generic across APIs that emit
// 2xx-with-batch-errors. New partial-failure-shaped fields are added to
// partialFailureFields, not at call sites. When `results[]` is present
// (Google Ads convention) it extracts per-op `resourceName` so callers
// can see which operations did succeed.
func detectPartialFailure(data []byte) *partialFailureReport {
	if len(data) == 0 {
		return nil
	}
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	partialFailureFields := []string{"partialFailureError"}
	for _, field := range partialFailureFields {
		raw, ok := top[field]
		if !ok || raw == nil {
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		message, _ := obj["message"].(string)
		var code int
		if n, ok := obj["code"].(float64); ok {
			code = int(n)
		}
		// Empty object means partial-failure mode was off or no ops
		// failed; do not flag.
		if code == 0 && strings.TrimSpace(message) == "" {
			continue
		}
		report := &partialFailureReport{
			Field:   field,
			Message: message,
			Code:    code,
			Details: obj["details"],
		}
		if results, ok := top["results"].([]any); ok {
			for _, r := range results {
				if rm, ok := r.(map[string]any); ok {
					if name, ok := rm["resourceName"].(string); ok && name != "" {
						report.ResourceNames = append(report.ResourceNames, name)
					}
				}
			}
		}
		return report
	}
	return nil
}

func writeAPIErrorEnvelope(flags *rootFlags, err error, code int) {
	if flags == nil || !flags.asJSON {
		return
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"error": err.Error(),
		"code":  code,
	})
}

// classifyAPIError maps API errors to structured exit codes with actionable hints.
func classifyAPIError(err error, flags *rootFlags) error {
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "HTTP 409"):
		if flags != nil && flags.idempotent {
			return writeNoop(flags, "already_exists", "already exists (no-op)")
		}
		classified := apiErr(err)
		writeAPIErrorEnvelope(flags, classified, ExitCode(classified))
		return classified
	case errors.Is(err, client.ErrPlaceholderCredential):
		return authErr(err)
	case strings.Contains(msg, "HTTP 400") && cliutil.LooksLikeAuthError(msg):
		return authErr(fmt.Errorf("%w\nhint: the API rejected the request — this usually means auth is missing or invalid."+
			"\n      Set your API key: export QBO_CLIENT_ID=<your-key>"+
			"\n      Run 'qbo-pp-cli doctor' to check auth status."+
			"\n      Response: "+cliutil.SanitizeErrorBody(msg), err))
	case strings.Contains(msg, "HTTP 401"):
		return authErr(fmt.Errorf("%w\nhint: check your token. Set it with: qbo-pp-cli auth set-token <token>"+
			"\n      or: export QBO_CLIENT_ID=<your-token>"+
			"\n      Run 'qbo-pp-cli doctor' to check auth status.", err))
	case strings.Contains(msg, "HTTP 403"):
		return authErr(fmt.Errorf("%w\nhint: permission denied. Your credentials are valid but lack access to this resource."+
			"\n      Check that your API key has the required permissions."+
			"\n      Set it with: export QBO_CLIENT_ID=<your-key>"+
			"\n      Run 'qbo-pp-cli doctor' to check auth status.", err))
	case strings.Contains(msg, "HTTP 404"):
		return notFoundErr(fmt.Errorf("%w\nhint: resource not found. Run the 'list' command to see available items", err))
	case strings.Contains(msg, "HTTP 429"):
		return rateLimitErr(err)
	default:
		return apiErr(err)
	}
}
