// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"

	"github.com/spf13/cobra"
)

func newNovelCancelCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "cancel <booking-id>",
		Short:   "Cancels a booking and confirms it landed by re-reading status",
		Example: "human-goat-pp-cli cancel task_abc123 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing booking-id"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("cancel accepts one booking-id"))
			}
			id := strings.TrimSpace(args[0])
			if id == "" {
				return usageErr(fmt.Errorf("missing booking-id"))
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would cancel booking %s and re-read status to verify\n", id)
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tr := taskrabbit.New(c)

			jobID, convErr := strconv.Atoi(id)
			if convErr != nil {
				return usageErr(fmt.Errorf("booking-id must be numeric (the jobId from `tasks list`), got %q", id))
			}

			// Look up the booking to get its rabbitId (taskers[0].id), which
			// cancelTask requires alongside jobId. Page through active bookings so
			// a booking past the first page is still found.
			active, err := listAllActiveBookings(ctx, tr)
			if err != nil {
				return classifyAPIError(fmt.Errorf("cancel: look up booking %d: %w", jobID, err), flags)
			}
			var rabbitID int
			var appointment string
			var found bool
			for _, b := range active {
				if b.JobID == jobID {
					rabbitID = b.RabbitID
					appointment = b.Appointment
					found = true
					break
				}
			}
			if !found {
				return classifyAPIError(fmt.Errorf("cancel: no active booking with id %d (run `tasks list` to see current bookings)", jobID), flags)
			}

			// A missing CSRF token guarantees a 403 on the mutation, so fail fast
			// with the root cause rather than surfacing it as a downstream API error.
			token, tokenErr := csrfToken(ctx, flags)
			if tokenErr != nil {
				return classifyAPIError(fmt.Errorf("cancel TaskRabbit booking %d: obtain CSRF token: %w", jobID, tokenErr), flags)
			}

			if _, err := tr.CancelTask(ctx, jobID, rabbitID, "Plans changed, no longer need the help", token); err != nil {
				return classifyAPIError(fmt.Errorf("cancel TaskRabbit booking %d: %w", jobID, err), flags)
			}

			// Verify: re-read active bookings (all pages); the cancelled booking must be gone.
			after, err := listAllActiveBookings(ctx, tr)
			if err != nil {
				return classifyAPIError(fmt.Errorf("cancel TaskRabbit booking %d succeeded, but verify re-read failed: %w", jobID, err), flags)
			}
			stillActive := false
			for _, b := range after {
				if b.JobID == jobID {
					stillActive = true
					break
				}
			}
			result := cancelResult{
				BookingID:        id,
				CancelResponseOK: true,
			}
			if stillActive {
				result.VerifiedStatus = "still-active"
				result.Note = joinNotes(result.Note, "WARNING: booking still appears active after cancel; verify in the app")
			} else {
				result.VerifiedStatus = "cancelled"
				result.Note = joinNotes(result.Note, "verified: booking no longer active", refundStatusNote(appointment))
			}
			return printCancelResult(cmd, flags, result)
		},
	}
	return cmd
}

type cancelResult struct {
	BookingID        string `json:"booking_id"`
	CancelResponseOK bool   `json:"cancel_response_ok"`
	VerifiedStatus   string `json:"verified_status"`
	Note             string `json:"note,omitempty"`
}

// listAllActiveBookings pages through active TaskRabbit bookings so a booking
// past the first page is still found. Capped so a paging bug can't loop forever.
func listAllActiveBookings(ctx context.Context, tr *taskrabbit.Client) ([]taskrabbit.Booking, error) {
	const perPage = 50
	const maxPages = 40 // 2000 active bookings is far beyond any real account
	var all []taskrabbit.Booking
	for page := 1; page <= maxPages; page++ {
		batch, err := tr.ListTasks(ctx, page, perPage, map[string]any{"status": "active"}, "en-US")
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
	}
	return all, nil
}

func csrfToken(ctx context.Context, flags *rootFlags) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	body, err := c.GetWithHeadersNoCache(ctx, "/dashboard", nil, map[string]string{
		client.BinaryResponseHeader: "true",
		"Accept":                    "text/html,*/*",
	})
	if err != nil {
		return "", err
	}
	// TaskRabbit renders `<meta name="csrf-token" content="..." />` (self-closing,
	// space before />), so match up to the closing quote rather than a literal ">".
	matches := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`).FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("csrf-token meta tag not found")
	}
	return string(matches[1]), nil
}

func joinNotes(notes ...string) string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note != "" {
			out = append(out, note)
		}
	}
	return strings.Join(out, "; ")
}

func printCancelResult(cmd *cobra.Command, flags *rootFlags, result cancelResult) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	rows := [][]string{
		{"Booking ID", result.BookingID},
		{"Cancel response OK", fmt.Sprintf("%t", result.CancelResponseOK)},
		{"Verified status", result.VerifiedStatus},
	}
	if result.Note != "" {
		rows = append(rows, []string{"Note", result.Note})
	}
	return flags.printTable(cmd, []string{"FIELD", "VALUE"}, rows)
}

// refundStatusNote reports the deposit-refund situation from the appointment
// time relative to now instead of unconditionally claiming a refund. When the
// appointment time cannot be parsed it stays neutral rather than over-promising.
func refundStatusNote(appointment string) string {
	appt, ok := parseAppointmentTime(appointment)
	if !ok {
		return "deposit refund depends on the 24h cancellation window; verify in the app"
	}
	if time.Until(appt) >= 24*time.Hour {
		return "cancelled >=24h before the appointment; deposit is refundable"
	}
	return "cancelled within 24h of the appointment; deposit may be forfeited"
}

// parseAppointmentTime tolerates the common timestamp shapes TaskRabbit returns.
func parseAppointmentTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
