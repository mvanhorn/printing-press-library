// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): library-side new file. Adds a
// ParseStats type so RSC parsers can surface partial-failure telemetry
// (X of Y objects decoded; first errors). Used by the new rankings
// commands to detect schema drift — if too many objects fail to decode,
// the CLI exits non-zero rather than silently emitting an empty result.
//
// Standalone type; not wired into the existing roster_1000 / cluster /
// event extractors to preserve their return signatures.

package diggparse

import (
	"errors"
	"fmt"
)

// ParseStatsMaxErrors caps how many decode errors we retain on a
// ParseStats. The cap exists to keep the value cheap to copy and to
// avoid unbounded memory growth on a wildly malformed payload; the
// counters (Attempted, Decoded, Skipped) remain accurate beyond the
// cap.
const ParseStatsMaxErrors = 5

// ParseStats summarises the outcome of decoding a batch of object
// substrings out of an RSC stream. Callers Add one outcome per object
// (whether or not it decoded), then either render the stats for
// diagnostics or check them against a Threshold to decide whether to
// fail the command.
type ParseStats struct {
	// Attempted is the total number of object substrings the parser
	// tried to JSON-decode.
	Attempted int
	// Decoded is the number that decoded cleanly. Note an extractor
	// may still drop a Decoded object for semantic reasons (e.g.
	// empty username) — that drop is its own concern, not a Skipped.
	Decoded int
	// Skipped is the number that failed json.Unmarshal.
	Skipped int
	// Errors holds the first ParseStatsMaxErrors decode errors for
	// diagnostics. Later errors are dropped to keep this bounded.
	Errors []error
}

// Add accumulates one decode outcome. Pass nil err for a successful
// decode; pass the underlying error for a failure. Safe to call on a
// nil-receiver-equivalent: callers should pass &ParseStats{} or
// construct a value before Add.
func (s *ParseStats) Add(err error) {
	s.Attempted++
	if err == nil {
		s.Decoded++
		return
	}
	s.Skipped++
	if len(s.Errors) < ParseStatsMaxErrors {
		s.Errors = append(s.Errors, err)
	}
}

// SkipRatio is Skipped / Attempted as a float in [0, 1]. Returns 0
// when nothing was attempted (the "no input" case is not a drift).
func (s ParseStats) SkipRatio() float64 {
	if s.Attempted == 0 {
		return 0
	}
	return float64(s.Skipped) / float64(s.Attempted)
}

// Threshold returns a *ThresholdError when the skip ratio meets or
// exceeds maxRatio AND the parser attempted at least one object.
// maxRatio is a fraction in [0, 1]:
//
//   - 0 means "no tolerance" — any single Skipped entry trips the gate.
//   - 1 means "only fail when every entry fails" — useful as a
//     near-disable (a 99% skip rate still passes; the gate only trips
//     when 100% of attempts were Skipped).
//   - Values outside [0, 1] disable the check entirely; a caller
//     misconfiguration shouldn't silently fail every run.
//
// Empty inputs (Attempted == 0) never trip the threshold — that's
// for the caller to interpret separately (e.g., "page shape changed,
// no objects found at all" is a distinct error class).
func (s ParseStats) Threshold(maxRatio float64) error {
	if s.Attempted == 0 || s.Skipped == 0 {
		// A clean parse never trips — even at zero tolerance, the
		// presence of zero failures isn't a failure.
		return nil
	}
	if maxRatio < 0 || maxRatio > 1 {
		return nil
	}
	if s.SkipRatio() < maxRatio {
		return nil
	}
	return &ThresholdError{Stats: s, MaxRatio: maxRatio}
}

// ThresholdError signals that the fraction of skipped objects in a
// ParseStats exceeded the caller's tolerance. The Stats value is
// embedded so handlers can inspect counts and the first errors
// without re-parsing the formatted message.
type ThresholdError struct {
	Stats    ParseStats
	MaxRatio float64
}

// Error formats a diagnostic line suitable for stderr.
func (e *ThresholdError) Error() string {
	return fmt.Sprintf(
		"parse skip ratio %.2f reached threshold %.2f (%d/%d objects skipped); first error: %v",
		e.Stats.SkipRatio(), e.MaxRatio, e.Stats.Skipped, e.Stats.Attempted, e.firstError(),
	)
}

func (e *ThresholdError) firstError() error {
	if len(e.Stats.Errors) == 0 {
		return errors.New("no captured error details")
	}
	return e.Stats.Errors[0]
}

// Unwrap exposes the underlying first decode error so callers using
// errors.Is/As can match against typed errors from json.Unmarshal.
func (e *ThresholdError) Unwrap() error {
	if len(e.Stats.Errors) == 0 {
		return nil
	}
	return e.Stats.Errors[0]
}
