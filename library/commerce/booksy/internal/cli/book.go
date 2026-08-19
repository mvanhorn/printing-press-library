// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: book — guided, safety-gated appointment booking.
//
// Safety model:
//   1. Dry-run by default: previews the exact service, staffer, time, and price
//      via Booksy's non-committal dry_run endpoint; nothing is booked.
//   2. Only --confirm sends the real booking POST, and that step refuses under
//      any test harness (verify/dogfood) so automated runs never touch a real
//      barber's calendar.
//
// The confirm request shape is captured from a real completed Booksy booking:
// POST /me/appointments/business/{id}/ with the full agreements/pre-auth payload,
// returning the created appointment (appointment_uid). Prepayment businesses also
// require a --payment-method id.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/cliutil"

	"github.com/spf13/cobra"
)

// bkApptResp parses the dry_run preview and the real confirm response
// (identical shape). Only the fields we render/reuse are declared.
type bkApptResp struct {
	Appointment struct {
		AppointmentUID json.Number `json:"appointment_uid"`
		Version        json.Number `json:"_version"`
		Subbookings    []struct {
			BookedFrom string `json:"booked_from"`
			BookedTill string `json:"booked_till"`
			StafferID  int64  `json:"staffer_id"`
			Service    struct {
				Name    string `json:"name"`
				Variant struct {
					Price    float64 `json:"price"`
					Duration int     `json:"duration"`
				} `json:"variant"`
			} `json:"service"`
			ServicePrice string `json:"service_price"`
			Staffer      struct {
				Name string `json:"name"`
			} `json:"staffer"`
		} `json:"subbookings"`
	} `json:"appointment"`
}

func newNovelBookCmd(flags *rootFlags) *cobra.Command {
	var flagServiceVariant string
	var flagDate string
	var flagTime string
	var flagStaffer int64
	var flagPaymentMethod int64
	var flagNote string
	var flagConfirm bool

	cmd := &cobra.Command{
		Use:   "book <business_id>",
		Short: "Book an appointment end to end — previews the service, staffer, time, and price, and only commits with --confirm",
		Long: "Book a Booksy appointment.\n\n" +
			"By default this is a PREVIEW: it validates and shows the exact service,\n" +
			"staffer, time, and price via Booksy's dry-run endpoint and commits nothing.\n" +
			"Pass --confirm to actually place the booking.\n\n" +
			"Get --service-variant from `booksy-pp-cli services <business_id>` and a\n" +
			"--date/--time from `booksy-pp-cli availability <business_id> --service-variant <id>`.\n" +
			"Businesses that require prepayment also need --payment-method <id>.\n" +
			"Requires BOOKSY_ACCESS_TOKEN. The --confirm step refuses under any automated test harness; the preview is always safe.\n" +
			"Cancel a booking with `booksy-pp-cli cancel <appointment_id>`.",
		Example:     "  booksy-pp-cli book 297360 --service-variant 20193554 --date 2026-08-19 --time 10:00",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "business_id=297360;--service-variant=20193554;--date=2026-08-19;--time=10:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "book")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("business_id is required"))
			}
			businessID := args[0]
			variantID, err := strconv.ParseInt(flagServiceVariant, 10, 64)
			if err != nil || variantID == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--service-variant is required (see `booksy-pp-cli services %s`)", businessID))
			}
			if flagDate == "" || flagTime == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--date and --time are required (see `booksy-pp-cli availability %s --service-variant %d`)", businessID, variantID))
			}
			// Booksy expects an ISO datetime (date + "T" + time).
			bookedFrom := flagDate + "T" + flagTime

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			subbooking := func(stafferID int64) map[string]any {
				return map[string]any{
					"staffer_id":      stafferID,
					"booked_from":     bookedFrom,
					"service_variant": map[string]any{"mode": "variant", "id": variantID},
					"combo_children":  []any{},
				}
			}

			// Always dry-run first (non-committal preview + staffer resolution).
			dryBody := map[string]any{
				"subbookings":     []any{subbooking(flagStaffer)},
				"compatibilities": map[string]any{"prepayment": true},
			}
			previewData, status, err := c.Post(ctx, "/core/v2/customer_api/me/appointments/business/"+businessID+"/dry_run/", dryBody)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("booking preview failed (HTTP %d): %s", status, bkTruncate(string(previewData), 400))
			}
			var preview bkApptResp
			_ = json.Unmarshal(previewData, &preview)

			out := cmd.OutOrStdout()
			render := func(a bkApptResp) (svcName, staffer, priceLabel, from, till string, stafferID int64) {
				if len(a.Appointment.Subbookings) > 0 {
					sb := a.Appointment.Subbookings[0]
					priceLabel = cliutilCleanPrice(sb.ServicePrice)
					if priceLabel == "" {
						priceLabel = fmt.Sprintf("%.2f zł", sb.Service.Variant.Price)
					}
					return sb.Service.Name, sb.Staffer.Name, priceLabel, sb.BookedFrom, sb.BookedTill, sb.StafferID
				}
				return "", "", "", bookedFrom, "", 0
			}
			svcName, staffer, priceLabel, from, till, resolvedStaffer := render(preview)

			if !flagConfirm {
				view := struct {
					Preview        bool   `json:"preview"`
					Committed      bool   `json:"committed"`
					BusinessID     string `json:"business_id"`
					ServiceVariant int64  `json:"service_variant_id"`
					Service        string `json:"service"`
					Staffer        string `json:"staffer"`
					Price          string `json:"price"`
					BookedFrom     string `json:"booked_from"`
					BookedTill     string `json:"booked_till,omitempty"`
					Note           string `json:"note"`
				}{Preview: true, BusinessID: businessID, ServiceVariant: variantID, Service: svcName, Staffer: staffer, Price: priceLabel, BookedFrom: from, BookedTill: till, Note: "This was a preview. Re-run with --confirm to place the booking."}
				if !wantsHumanTable(out, flags) {
					return printJSONFiltered(out, view, flags)
				}
				fmt.Fprintln(out, "Booking preview (nothing booked yet):")
				fmt.Fprintf(out, "  Service : %s\n", orDash(svcName))
				fmt.Fprintf(out, "  Staffer : %s\n", orDash(staffer))
				fmt.Fprintf(out, "  When    : %s%s\n", from, tillSuffix(till))
				fmt.Fprintf(out, "  Price   : %s\n", orDash(priceLabel))
				fmt.Fprintf(out, "\nRe-run with --confirm to book:\n  booksy-pp-cli book %s --service-variant %d --date %s --time %s --confirm\n", businessID, variantID, flagDate, flagTime)
				return nil
			}

			// --confirm places a REAL appointment. Refuse under any test harness
			// so automated runs (verify/dogfood) never touch a barber's calendar.
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "confirm a Booksy booking")
			}

			// Use the staffer the dry-run resolved (Booksy turns -1 into a real
			// staffer); fall back to the requested value if none came back.
			confirmStaffer := resolvedStaffer
			if confirmStaffer == 0 {
				confirmStaffer = flagStaffer
			}
			confirmBody := map[string]any{
				"subbookings":                      []any{subbooking(confirmStaffer)},
				"is_cancellation_fee_preauth_flow": false,
				"pre_auth_type":                    "control",
				"customer_note":                    flagNote,
				"service_questions":                []any{},
				"bci_agreements":                   map[string]any{"web_communication_agreement": false, "disclosure_obligation_agreement": false},
				"recurring":                        false,
				"ask_for_consent":                  false,
				"compatibilities":                  map[string]any{"prepayment": true},
			}
			if flagPaymentMethod > 0 {
				confirmBody["payment_method"] = flagPaymentMethod
			}

			bookData, bstatus, err := c.Post(ctx, "/core/v2/customer_api/me/appointments/business/"+businessID+"/", confirmBody)
			if err != nil {
				return err
			}
			if bstatus >= 400 {
				hint := ""
				if bstatus == 400 && flagPaymentMethod == 0 {
					hint = " (this business may require prepayment — retry with --payment-method <id>)"
				}
				return fmt.Errorf("booking failed (HTTP %d)%s: %s", bstatus, hint, bkTruncate(string(bookData), 400))
			}
			var booked bkApptResp
			_ = json.Unmarshal(bookData, &booked)
			bSvc, bStaffer, bPrice, bFrom, bTill, _ := render(booked)
			if bSvc == "" {
				bSvc, bStaffer, bPrice, bFrom = svcName, staffer, priceLabel, from
			}
			apptUID := booked.Appointment.AppointmentUID.String()

			view := struct {
				Preview        bool   `json:"preview"`
				Committed      bool   `json:"committed"`
				AppointmentUID string `json:"appointment_uid,omitempty"`
				BusinessID     string `json:"business_id"`
				ServiceVariant int64  `json:"service_variant_id"`
				Service        string `json:"service"`
				Staffer        string `json:"staffer"`
				Price          string `json:"price"`
				BookedFrom     string `json:"booked_from"`
				BookedTill     string `json:"booked_till,omitempty"`
			}{Preview: false, Committed: true, AppointmentUID: apptUID, BusinessID: businessID, ServiceVariant: variantID, Service: bSvc, Staffer: bStaffer, Price: bPrice, BookedFrom: bFrom, BookedTill: bTill}
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, view, flags)
			}
			fmt.Fprintln(out, "✅ Booked!")
			if apptUID != "" {
				fmt.Fprintf(out, "  Appointment : #%s\n", apptUID)
			}
			fmt.Fprintf(out, "  Service     : %s\n", orDash(bSvc))
			fmt.Fprintf(out, "  Staffer     : %s\n", orDash(bStaffer))
			fmt.Fprintf(out, "  When        : %s%s\n", bFrom, tillSuffix(bTill))
			fmt.Fprintf(out, "  Price       : %s\n", orDash(bPrice))
			if apptUID != "" {
				fmt.Fprintf(out, "\nCancel with:\n  booksy-pp-cli cancel %s --confirm\n", apptUID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagServiceVariant, "service-variant", "", "Service variant id to book (from `services`)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Appointment date, YYYY-MM-DD")
	cmd.Flags().StringVar(&flagTime, "time", "", "Appointment time, HH:MM")
	cmd.Flags().Int64Var(&flagStaffer, "staffer", -1, "Staffer id, or -1 for any staffer")
	cmd.Flags().Int64Var(&flagPaymentMethod, "payment-method", 0, "Payment method id (required by prepayment businesses)")
	cmd.Flags().StringVar(&flagNote, "note", "", "Optional note to the business")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Actually place the booking (default is a safe preview)")
	return cmd
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func tillSuffix(till string) string {
	if till == "" {
		return ""
	}
	return " → " + till
}
