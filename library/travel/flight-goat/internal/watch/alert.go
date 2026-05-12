// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Dispatcher delivers a CheckResult to the user's chosen alert sink. The
// concrete implementations are stdout, json (stdout with no extra
// framing), and webhook. Tests substitute a recorder.
type Dispatcher interface {
	Dispatch(ctx context.Context, res CheckResult) error
	Name() string
}

// DispatcherFor parses a notify spec string and returns the matching
// Dispatcher. Empty spec means "stdout" — that's the default `watch
// check` behavior. A spec that fails validation (we already validate in
// Watch.Validate) is treated as stdout to avoid silently swallowing the
// alert at dispatch time.
func DispatcherFor(spec string) Dispatcher {
	spec = strings.TrimSpace(spec)
	switch {
	case spec == "", spec == "stdout":
		return &stdoutDispatcher{out: nil}
	case spec == "json":
		return &stdoutDispatcher{out: nil, jsonOnly: true}
	case strings.HasPrefix(spec, "webhook:"):
		return &webhookDispatcher{url: strings.TrimPrefix(spec, "webhook:")}
	default:
		return &stdoutDispatcher{out: nil}
	}
}

// SetStdoutWriter wires a custom writer into the stdoutDispatcher. The
// CLI watch.go uses this so output flows through cobra.Command.OutOrStdout
// (so --deliver, --quiet, and friends work).
type StdoutWriterSetter interface {
	SetStdoutWriter(w io.Writer)
}

type stdoutDispatcher struct {
	out      io.Writer
	jsonOnly bool
}

func (d *stdoutDispatcher) Name() string {
	if d.jsonOnly {
		return "stdout(json)"
	}
	return "stdout"
}

func (d *stdoutDispatcher) SetStdoutWriter(w io.Writer) { d.out = w }

func (d *stdoutDispatcher) Dispatch(_ context.Context, res CheckResult) error {
	// PATCH(greptile P2): default to os.Stdout when no writer was
	// injected so library callers that pass opts.Dispatcher == nil don't
	// silently lose alerts. The CLI still injects cobra.Command's writer
	// via SetStdoutWriter when it wants --deliver / output capture.
	w := d.out
	if w == nil {
		w = os.Stdout
	}
	if d.jsonOnly {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Fprintln(w, FormatAlertText(res))
	return nil
}

type webhookDispatcher struct {
	url    string
	client *http.Client
}

func (d *webhookDispatcher) Name() string { return "webhook:" + d.url }

func (d *webhookDispatcher) Dispatch(ctx context.Context, res CheckResult) error {
	if d.client == nil {
		d.client = &http.Client{Timeout: 15 * time.Second}
	}
	body, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal webhook body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "flight-goat-pp-cli/watch")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, strings.TrimSpace(string(excerpt)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// FormatAlertText renders a CheckResult into a short, human-readable
// stdout alert. The SafetyNotice is always the last line so even
// truncated output (e.g. inside a Slack incoming-webhook preview that
// clips long messages) still surfaces the warning.
func FormatAlertText(r CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[flight-goat watch] %s %s%s %s\n",
		r.WatchID, r.Airline, r.FlightNo, r.Date)
	fmt.Fprintf(&b, "  Route:    %s -> %s", r.Origin, r.Destination)
	if r.Cabin != "" {
		fmt.Fprintf(&b, " (%s)", r.Cabin)
	}
	if r.FareBrand != "" {
		fmt.Fprintf(&b, " [%s]", r.FareBrand)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "  Paid:     %s %.2f\n", r.Currency, r.OriginalPrice)
	if r.FoundPrice != nil {
		fmt.Fprintf(&b, "  Now:      %s %.2f", r.Currency, *r.FoundPrice)
		if r.Delta != nil {
			fmt.Fprintf(&b, "  (delta %+.2f)", *r.Delta)
		}
		fmt.Fprintln(&b)
	}
	if mf := r.MatchedFlight; mf != nil {
		dep := trimISOTime(mf.DepartureTime)
		arr := trimISOTime(mf.ArrivalTime)
		if dep != "" || arr != "" {
			fmt.Fprintf(&b, "  Schedule: %s -> %s", dep, arr)
			if mf.DurationMinutes > 0 {
				fmt.Fprintf(&b, "  (%dh%02dm, %d stop%s)", mf.DurationMinutes/60, mf.DurationMinutes%60, mf.Stops, pluralS(mf.Stops))
			}
			fmt.Fprintln(&b)
		}
	}
	fmt.Fprintf(&b, "  Match:    %s\n", r.Confidence)
	if r.MatchReason != "" {
		fmt.Fprintf(&b, "  Why:      %s\n", r.MatchReason)
	}
	if r.MatchMismatchReason != "" {
		fmt.Fprintf(&b, "  Note:     %s\n", r.MatchMismatchReason)
	}
	if r.AlertSuppressed {
		fmt.Fprintf(&b, "  Status:   suppressed (%s)\n", r.AlertSuppressReason)
	} else if r.AlertDispatched {
		fmt.Fprintf(&b, "  Status:   alerted via %s\n", r.AlertDispatchedTo)
	}
	if r.BookingURL != "" {
		fmt.Fprintf(&b, "  Book:     %s\n", r.BookingURL)
	}
	fmt.Fprintf(&b, "  ⚠ %s", r.SafetyNotice)
	return b.String()
}

// trimISOTime collapses an ISO-8601 string like 2026-06-21T07:25:00 to
// "06-21 07:25" for the human alert. Returns the input unchanged if
// it's shorter than the expected slice.
func trimISOTime(s string) string {
	if len(s) >= 16 {
		return s[5:10] + " " + s[11:16]
	}
	return s
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// PostWebhook is a convenience for `watch alert-test` when a user wants
// to ping their webhook without running a full check. It returns an
// error rather than embedding the failure in a CheckResult so the CLI
// can surface a non-zero exit.
func PostWebhook(ctx context.Context, url string, res CheckResult) error {
	d := &webhookDispatcher{url: url}
	return d.Dispatch(ctx, res)
}

// SampleResult builds a synthetic CheckResult for `watch alert-test`.
// It's intentionally NOT marked AlertDispatched/Suppressed — the
// dispatcher fills those in based on the actual send.
func SampleResult(w *Watch, now time.Time) CheckResult {
	mock := w.OriginalPrice * 0.85
	delta := w.OriginalPrice - mock
	depHM := w.DepartureTime
	if depHM == "" {
		depHM = "07:25"
	}
	res := CheckResult{
		Schema:           "flight-goat.watch.check.v1",
		WatchID:          w.ID,
		CheckedAt:        now.UTC(),
		Origin:           w.Origin,
		Destination:      w.Destination,
		Date:             w.DepartureDate,
		DepartureTime:    w.DepartureTime,
		Airline:          w.Airline,
		FlightNo:         w.FlightNumber,
		Cabin:            w.Cabin,
		FareBrand:        w.FareBrand,
		OriginalPrice:    w.OriginalPrice,
		Threshold:        w.Threshold,
		Currency:         w.Currency,
		BookingURL:       BookingSearchURL(w),
		FoundPrice:       &mock,
		Delta:            &delta,
		Confidence:       MatchExact,
		ThresholdCrossed: true,
		MatchedFlight: &MatchedFlight{
			Airline:         w.Airline,
			FlightNumber:    w.FlightNumber,
			Price:           mock,
			Currency:        w.Currency,
			Cabin:           w.Cabin,
			FareBrand:       w.FareBrand,
			DepartureTime:   fmt.Sprintf("%sT%s:00", w.DepartureDate, depHM),
			ArrivalTime:     fmt.Sprintf("%sT11:55:00", w.DepartureDate),
			DurationMinutes: 330,
			Stops:           0,
		},
		SafetyNotice: SafetyNoticeText,
	}
	res.MatchReason = explainMatch(w, nil, MatchExact, nil, "")
	return res
}
