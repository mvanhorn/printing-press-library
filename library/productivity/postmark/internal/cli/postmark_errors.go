// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/platform"
)

// Postmark ErrorCode values that commands branch on.
const (
	// The recipient is inactive (hard bounce, spam complaint, or manual
	// suppression), so Postmark refused the send.
	postmarkErrInactiveRecipient = 406
	// GET /messages/outbound/{id}/details answers with this once a message is
	// past the server's retention window.
	postmarkErrMessageNotFound = 701
	// A referenced template or layout does not exist.
	postmarkErrTemplateNotFound = 1101
	// Postmark refuses to delete a layout that templates still use.
	postmarkErrLayoutInUse = 1130
)

// postmarkFailure is a Postmark error response: HTTP status plus the
// ErrorCode/Message body Postmark returns on 4xx.
type postmarkFailure struct {
	Status    int
	ErrorCode int
	Message   string
}

func (f postmarkFailure) String() string {
	if f.ErrorCode != 0 {
		return fmt.Sprintf("HTTP %d ErrorCode %d: %s", f.Status, f.ErrorCode, f.Message)
	}
	return fmt.Sprintf("HTTP %d: %s", f.Status, f.Message)
}

// postmarkAPIFailure extracts a Postmark API error. Rate-limit errors are not
// API failures here: callers must surface those instead of recording them
// per item.
func postmarkAPIFailure(err error) (postmarkFailure, bool) {
	if err == nil || isRateLimited(err) {
		return postmarkFailure{}, false
	}
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		return postmarkFailure{}, false
	}
	f := postmarkFailure{Status: apiErr.StatusCode}
	var body struct {
		ErrorCode int    `json:"ErrorCode"`
		Message   string `json:"Message"`
	}
	if json.Unmarshal([]byte(apiErr.Body), &body) == nil && (body.ErrorCode != 0 || body.Message != "") {
		f.ErrorCode, f.Message = body.ErrorCode, body.Message
	} else {
		f.Message = strings.TrimSpace(apiErr.Body)
	}
	return f, true
}

func isRateLimited(err error) bool {
	var platformErr *platform.RateLimitedError
	var cliErr *cliutil.RateLimitError
	return errors.As(err, &platformErr) || errors.As(err, &cliErr)
}

// postmarkErrorText renders an error for a per-item result row.
func postmarkErrorText(err error) string {
	if f, ok := postmarkAPIFailure(err); ok {
		return f.String()
	}
	return err.Error()
}

// postmarkFetchFailure is one unit a multi-fetch command could not read.
// Fan-outs label it by Server or Unit; per-item reads label it by ID.
type postmarkFetchFailure struct {
	Server string `json:"server,omitempty"`
	Unit   string `json:"unit,omitempty"`
	ID     string `json:"id,omitempty"`
	Error  string `json:"error"`
}

// itemFailure records a per-item read that failed, labeled by the item's ID.
func itemFailure(id string, err error) postmarkFetchFailure {
	return postmarkFetchFailure{ID: id, Error: postmarkErrorText(err)}
}

// postmarkAllFailed turns a fan-out where every target failed into an API
// error, so a revoked token or an outage never reads as a clean, empty report.
func postmarkAllFailed(failures []postmarkFetchFailure, total int) error {
	if total == 0 || len(failures) < total {
		return nil
	}
	return apiErr(fmt.Errorf("every server check failed (%d of %d): %s", len(failures), total, failures[0].Error))
}

// warnPartialFailures tells the caller that failed of total units produced
// no result, so a short report is never read as complete. outcome finishes
// the "N of M" clause; note, when set, follows a semicolon.
func warnPartialFailures(w io.Writer, failed, total int, outcome, note string) {
	if failed == 0 {
		return
	}
	msg := fmt.Sprintf("warning: %d of %d %s", failed, total, outcome)
	if note != "" {
		msg += "; " + note
	}
	fmt.Fprintln(w, msg)
}

// remainingNote is the warning note for a report that still covers the units
// that did not fail.
func remainingNote(covered int) string {
	return fmt.Sprintf("results cover the remaining %d", covered)
}

// warnFanoutFailures is the partial-failure warning for a fan-out across
// servers or domains.
func warnFanoutFailures(w io.Writer, failed, total int, unit string) {
	warnPartialFailures(w, failed, total, unit+" fetches failed", remainingNote(total-failed))
}
