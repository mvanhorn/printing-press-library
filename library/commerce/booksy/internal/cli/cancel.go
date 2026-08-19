// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cancel — cancel one of your Booksy appointments.
//
// Captured from a real Booksy cancellation:
//   GET  /me/appointments/{id}/            -> appointment (incl. _version)
//   POST /me/appointments/{id}/action/     -> {"action":"cancel","_version":<v>,"no_thumbs":"true"}
//
// Same safety model as book: preview by default; --confirm performs the real
// cancel and refuses under any test harness.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelCancelCmd(flags))
	})
}

func newNovelCancelCmd(flags *rootFlags) *cobra.Command {
	var flagConfirm bool

	cmd := &cobra.Command{
		Use:   "cancel <appointment_id>",
		Short: "Cancel one of your Booksy appointments — previews by default, only cancels with --confirm",
		Long: "Cancel a Booksy appointment by id (the appointment_uid shown by `book`).\n\n" +
			"By default this PREVIEWS the appointment and commits nothing.\n" +
			"Pass --confirm to actually cancel it.\n" +
			"Requires BOOKSY_ACCESS_TOKEN. The --confirm step refuses under any automated test harness.",
		Example:     "  booksy-pp-cli cancel 746784544 --confirm",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "appointment_id=746784544;--confirm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cancel")
			}
			// --confirm mutates state; refuse under any harness before any call.
			if flagConfirm && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "cancel a Booksy appointment")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("appointment_id is required (the appointment_uid from `book`)"))
			}
			apptID := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch the appointment to show what will be cancelled and to read
			// its _version (required by the cancel action).
			detailData, err := c.Get(ctx, "/core/v2/customer_api/me/appointments/"+apptID+"/", map[string]string{"no_thumbs": "true"})
			if err != nil {
				return fmt.Errorf("appointment %s not found or not accessible: %w", apptID, err)
			}
			var appt bkApptResp
			_ = json.Unmarshal(detailData, &appt)
			var svcName, staffer, from string
			if len(appt.Appointment.Subbookings) > 0 {
				sb := appt.Appointment.Subbookings[0]
				svcName, staffer, from = sb.Service.Name, sb.Staffer.Name, sb.BookedFrom
			}
			version := appt.Appointment.Version.String()
			out := cmd.OutOrStdout()

			if !flagConfirm {
				view := struct {
					Preview        bool   `json:"preview"`
					Cancelled      bool   `json:"cancelled"`
					AppointmentUID string `json:"appointment_uid"`
					Service        string `json:"service"`
					Staffer        string `json:"staffer"`
					BookedFrom     string `json:"booked_from"`
					Note           string `json:"note"`
				}{Preview: true, AppointmentUID: apptID, Service: svcName, Staffer: staffer, BookedFrom: from, Note: "This was a preview. Re-run with --confirm to cancel."}
				if !wantsHumanTable(out, flags) {
					return printJSONFiltered(out, view, flags)
				}
				fmt.Fprintln(out, "Cancel preview (nothing cancelled yet):")
				fmt.Fprintf(out, "  Appointment : #%s\n", apptID)
				fmt.Fprintf(out, "  Service     : %s\n", orDash(svcName))
				fmt.Fprintf(out, "  Staffer     : %s\n", orDash(staffer))
				fmt.Fprintf(out, "  When        : %s\n", orDash(from))
				fmt.Fprintf(out, "\nRe-run with --confirm to cancel:\n  booksy-pp-cli cancel %s --confirm\n", apptID)
				return nil
			}

			body := map[string]any{"action": "cancel", "no_thumbs": "true"}
			if version != "" {
				body["_version"] = json.RawMessage(version)
			}
			cancelData, cstatus, err := c.Post(ctx, "/core/v2/customer_api/me/appointments/"+apptID+"/action/", body)
			if err != nil {
				return err
			}
			if cstatus >= 400 {
				return fmt.Errorf("cancel failed (HTTP %d): %s", cstatus, bkTruncate(string(cancelData), 300))
			}
			view := struct {
				Preview        bool   `json:"preview"`
				Cancelled      bool   `json:"cancelled"`
				AppointmentUID string `json:"appointment_uid"`
				Service        string `json:"service"`
				BookedFrom     string `json:"booked_from"`
			}{Preview: false, Cancelled: true, AppointmentUID: apptID, Service: svcName, BookedFrom: from}
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, view, flags)
			}
			fmt.Fprintf(out, "✅ Cancelled appointment #%s\n", apptID)
			if svcName != "" {
				fmt.Fprintf(out, "  Service : %s\n", svcName)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Actually cancel the appointment (default is a safe preview)")
	return cmd
}
