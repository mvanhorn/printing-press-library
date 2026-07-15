// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written write path: book / reschedule / cancel. These mutate real
// appointments, so every command is dry-run by default (prints the exact
// request it would send) and requires --confirm to actually submit. The write
// request shape follows Jane's REST conventions for patient booking; the
// concrete endpoint is verified against a live session before a real submit.

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newBookCmd(flags *rootFlags) *cobra.Command {
	var treatment, staff, location int
	var atStr string
	var confirm, debug bool

	cmd := &cobra.Command{
		Use:   "book",
		Short: "Book an appointment (dry-run by default; --confirm to submit)",
		Long: `Book an appointment at the active clinic. Requires a logged-in session.

Booking runs Jane's real reserve -> confirm transaction (holds the slot, then
confirms it under your patient profile). By default this is a DRY RUN: it prints
what it would book and changes nothing. Add --confirm to actually book.

Find IDs with 'treatments', 'staff', 'locations'; find a real open slot with
'next-opening' or 'openings'.`,
		Example:     "  janeapp-pp-cli book --clinic leahkangas --treatment 2 --staff 1 --location 1 --at 2026-08-21T16:00:00-07:00\n  janeapp-pp-cli book --clinic leahkangas --treatment 2 --staff 1 --location 1 --at 2026-08-21T16:00:00-07:00 --confirm",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if treatment <= 0 || staff <= 0 || location <= 0 || atStr == "" {
				return usageErr(fmt.Errorf("book requires --treatment, --staff, --location, and --at"))
			}
			if _, err := parseFlexibleDate(atStr); err != nil {
				return usageErr(fmt.Errorf("invalid --at %q: use RFC3339 or YYYY-MM-DDTHH:MM:SS", atStr))
			}
			clinic, err := requireActiveClinic(flags)
			if err != nil {
				return err
			}
			if clinic.Session == "" {
				return usageErr(fmt.Errorf("not logged in to clinic %q; run 'janeapp-pp-cli auth login --clinic %s --chrome'", clinic.Name, clinic.Name))
			}
			if !confirm {
				// Dry run: describe the transaction without touching Jane.
				plan := map[string]any{
					"dry_run": true, "clinic": clinic.Name, "action": "book",
					"treatment_id": treatment, "staff_member_id": staff,
					"location_id": location, "start_at": atStr,
					"steps": []string{
						"POST /api/v2/reservations (hold slot)",
						"POST /api/v2/appointments/{reservation_id}/book (confirm)",
					},
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN — would book at %s:\n", clinic.BaseURL)
				fmt.Fprintf(cmd.OutOrStdout(), "  treatment=%d staff=%d location=%d start=%s\n", treatment, staff, location, atStr)
				fmt.Fprintln(cmd.OutOrStdout(), "  via: reserve slot -> confirm booking")
				fmt.Fprintln(cmd.ErrOrStderr(), "(no changes made. Re-run with --confirm to book.)")
				return nil
			}
			// Real booking.
			var dbg io.Writer
			if debug {
				dbg = cmd.ErrOrStderr()
			}
			booker, err := newJaneBooker(cmd.Context(), clinic, flags.timeout, dbg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result, err := booker.Book(cmd.Context(), treatment, staff, location, atStr)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				var parsed any
				if json.Unmarshal([]byte(result), &parsed) == nil {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"booked": true, "clinic": clinic.Name, "appointment": parsed}, flags)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Booked at %s (clinic %q).\n", atStr, clinic.Name)
			return nil
		},
	}
	cmd.Flags().IntVar(&treatment, "treatment", 0, "Treatment ID (see 'treatments')")
	cmd.Flags().IntVar(&staff, "staff", 0, "Practitioner ID (see 'staff')")
	cmd.Flags().IntVar(&location, "location", 0, "Location ID (see 'locations')")
	cmd.Flags().StringVar(&atStr, "at", "", "Appointment start time (RFC3339 or YYYY-MM-DDTHH:MM:SS)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually submit the booking (otherwise dry-run)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Print the reserve/confirm HTTP trace")
	return cmd
}

func newRescheduleCmd(flags *rootFlags) *cobra.Command {
	var id, treatment, staff, location int
	var atStr string
	var confirm, debug bool

	cmd := &cobra.Command{
		Use:   "reschedule",
		Short: "Reschedule an appointment to a new time (dry-run by default; --confirm to submit)",
		Long: `Move an existing appointment to a new time. Requires a logged-in
session and the appointment's ID (see 'appointments upcoming').

Reschedule books the NEW slot first, then cancels the old one — so if the new
slot can't be booked, your original appointment is left untouched. The treatment,
practitioner, and location default to the existing appointment's; override with
--treatment/--staff/--location. Dry-run by default; add --confirm to submit.`,
		Example:     "  janeapp-pp-cli reschedule --clinic leahkangas --id 902 --at 2026-08-28T16:00:00-07:00\n  janeapp-pp-cli reschedule --clinic leahkangas --id 902 --at 2026-08-28T16:00:00-07:00 --confirm",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if id <= 0 || atStr == "" {
				return usageErr(fmt.Errorf("reschedule requires --id and --at"))
			}
			if _, err := parseFlexibleDate(atStr); err != nil {
				return usageErr(fmt.Errorf("invalid --at %q", atStr))
			}
			clinic, err := requireActiveClinic(flags)
			if err != nil {
				return err
			}
			if clinic.Session == "" {
				return usageErr(fmt.Errorf("not logged in to clinic %q; run 'janeapp-pp-cli auth login --clinic %s --chrome'", clinic.Name, clinic.Name))
			}
			var dbg io.Writer
			if debug {
				dbg = cmd.ErrOrStderr()
			}
			booker, err := newJaneBooker(cmd.Context(), clinic, flags.timeout, dbg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Resolve the existing appointment to inherit treatment/staff/location.
			det, err := booker.appointmentByID(cmd.Context(), id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !det.Found {
				return usageErr(fmt.Errorf("appointment %d not found at clinic %q (see 'appointments upcoming')", id, clinic.Name))
			}
			t := pick(treatment, det.TreatmentID)
			s := pick(staff, det.StaffMemberID)
			l := pick(location, det.LocationID)
			if t <= 0 || s <= 0 || l <= 0 {
				return usageErr(fmt.Errorf("could not determine treatment/staff/location for appointment %d (got treatment=%d staff=%d location=%d); pass --treatment/--staff/--location explicitly", id, t, s, l))
			}
			if !confirm {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "reschedule", "clinic": clinic.Name, "id": id, "from": det.StartAt, "to": atStr, "treatment_id": t, "staff_member_id": s, "location_id": l}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN — would reschedule appointment %d\n  from %s to %s (treatment=%d staff=%d location=%d)\n", id, det.StartAt, atStr, t, s, l)
				fmt.Fprintln(cmd.OutOrStdout(), "  via: book new slot -> cancel old")
				fmt.Fprintln(cmd.ErrOrStderr(), "(no changes made. Re-run with --confirm to reschedule.)")
				return nil
			}
			// Book the new slot first (keeps the old one if this fails).
			if _, err := booker.Book(cmd.Context(), t, s, l, atStr); err != nil {
				return fmt.Errorf("reschedule aborted — new slot could not be booked (original appointment untouched): %w", err)
			}
			// Cancel the old with a FRESH booker: Jane rotates the CSRF token
			// after a mutation, so reusing the post-booking token makes the
			// DELETE return 204 without actually cancelling. A new booker re-reads
			// a valid CSRF from the page.
			cancelBooker, err := newJaneBooker(cmd.Context(), clinic, flags.timeout, dbg)
			if err != nil {
				return fmt.Errorf("new slot %s was booked, but re-initializing to cancel old appointment %d failed — cancel it manually: %w", atStr, id, err)
			}
			if _, err := cancelBooker.Cancel(cmd.Context(), id, "rescheduled via janeapp-pp-cli"); err != nil {
				return fmt.Errorf("new slot %s was booked, but cancelling old appointment %d failed — you now have both, cancel the old one manually: %w", atStr, id, err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"rescheduled": true, "clinic": clinic.Name, "old_id": id, "new_start_at": atStr}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Rescheduled: booked %s and cancelled old appointment %d (clinic %q).\n", atStr, id, clinic.Name)
			return nil
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Appointment ID to reschedule (see 'appointments upcoming')")
	cmd.Flags().StringVar(&atStr, "at", "", "New start time (RFC3339 or YYYY-MM-DDTHH:MM:SS)")
	cmd.Flags().IntVar(&treatment, "treatment", 0, "Override the treatment ID (default: same as existing)")
	cmd.Flags().IntVar(&staff, "staff", 0, "Override the practitioner ID (default: same as existing)")
	cmd.Flags().IntVar(&location, "location", 0, "Override the location ID (default: same as existing)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually submit the change (otherwise dry-run)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Print the reserve/confirm/cancel HTTP trace")
	return cmd
}

func pick(override, fallback int) int {
	if override > 0 {
		return override
	}
	return fallback
}

func newCancelCmd(flags *rootFlags) *cobra.Command {
	var id int
	var reason string
	var confirm, debug bool

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an appointment (dry-run by default; --confirm to submit)",
		Long: `Cancel an existing appointment by ID (see 'appointments upcoming')
via Jane's native cancel endpoint. Requires a logged-in session. Dry-run by
default; add --confirm to actually cancel. Clinics may restrict cancellations
outside a notice window (Jane will return an error in that case).`,
		Example:     "  janeapp-pp-cli cancel --clinic leahkangas --id 902\n  janeapp-pp-cli cancel --clinic leahkangas --id 902 --confirm",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if id <= 0 {
				return usageErr(fmt.Errorf("cancel requires --id (see 'appointments upcoming')"))
			}
			clinic, err := requireActiveClinic(flags)
			if err != nil {
				return err
			}
			if clinic.Session == "" {
				return usageErr(fmt.Errorf("not logged in to clinic %q; run 'janeapp-pp-cli auth login --clinic %s --chrome'", clinic.Name, clinic.Name))
			}
			if !confirm {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "cancel", "clinic": clinic.Name, "id": id}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN — would cancel appointment %d at clinic %q via DELETE /api/v2/appointments/%d\n", id, clinic.Name, id)
				fmt.Fprintln(cmd.ErrOrStderr(), "(no changes made. Re-run with --confirm to cancel.)")
				return nil
			}
			var dbg io.Writer
			if debug {
				dbg = cmd.ErrOrStderr()
			}
			booker, err := newJaneBooker(cmd.Context(), clinic, flags.timeout, dbg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if _, err := booker.Cancel(cmd.Context(), id, reason); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cancelled": true, "clinic": clinic.Name, "id": id}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cancelled appointment %d (clinic %q).\n", id, clinic.Name)
			return nil
		},
	}
	cmd.Flags().IntVar(&id, "id", 0, "Appointment ID to cancel (see 'appointments upcoming')")
	cmd.Flags().StringVar(&reason, "reason", "", "Optional cancellation reason")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually submit the cancellation (otherwise dry-run)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Print the cancel HTTP trace")
	return cmd
}
