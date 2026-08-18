// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: book — guided, safety-gated appointment booking.
//
// Safety model:
//   1. Dry-run by default: previews the exact service, staffer, time, and price
//      via Booksy's non-committal dry_run endpoint; nothing is booked.
//   2. Only --confirm sends the real booking POST, and that step refuses under
//      any test harness (verify/dogfood) so automated runs never touch a real
//      barber's calendar.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/cliutil"

	"github.com/spf13/cobra"
)

// dry_run / appointment response (only the preview fields we render).
type bkDryRunResp struct {
	Appointment struct {
		AppointmentUID any `json:"appointment_uid"`
		Subbookings    []struct {
			BookedFrom string `json:"booked_from"`
			BookedTill string `json:"booked_till"`
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
			"Requires BOOKSY_ACCESS_TOKEN. The --confirm step refuses under any automated test harness; the preview is always safe.",
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
			bookedFrom := flagDate + " " + flagTime

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"subbookings": []map[string]any{
					{
						"booked_from":     bookedFrom,
						"staffer_id":      flagStaffer,
						"service_variant": map[string]any{"mode": "variant", "id": variantID},
						"addons":          []any{},
						"combo_children":  []any{},
					},
				},
				"compatibilities": map[string]any{"prepayment": true},
			}

			// Always dry-run first (preview).
			previewData, status, err := c.Post(ctx, "/core/v2/customer_api/me/appointments/business/"+businessID+"/dry_run/", body)
			if err != nil {
				return err
			}
			if status >= 400 {
				return fmt.Errorf("booking preview failed (HTTP %d): %s", status, bkTruncate(string(previewData), 400))
			}
			var preview bkDryRunResp
			_ = json.Unmarshal(previewData, &preview)

			out := cmd.OutOrStdout()
			renderPreview := func() (svcName, staffer, priceLabel, from, till string) {
				if len(preview.Appointment.Subbookings) > 0 {
					sb := preview.Appointment.Subbookings[0]
					priceLabel = cliutilCleanPrice(sb.ServicePrice)
					if priceLabel == "" {
						priceLabel = fmt.Sprintf("%.2f zł", sb.Service.Variant.Price)
					}
					return sb.Service.Name, sb.Staffer.Name, priceLabel, sb.BookedFrom, sb.BookedTill
				}
				return "", "", "", bookedFrom, ""
			}
			svcName, staffer, priceLabel, from, till := renderPreview()

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
			// The preview above (dry_run) is non-committal and is allowed to run.
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "confirm a Booksy booking")
			}
			// --confirm: place the real booking. Same validated payload, posted
			// to the non-dry-run endpoint.
			bookData, bstatus, err := c.Post(ctx, "/core/v2/customer_api/me/appointments/business/"+businessID+"/", body)
			if err != nil {
				return err
			}
			if bstatus >= 400 {
				return fmt.Errorf("booking failed (HTTP %d): %s", bstatus, bkTruncate(string(bookData), 400))
			}
			var booked bkDryRunResp
			_ = json.Unmarshal(bookData, &booked)
			view := struct {
				Preview        bool            `json:"preview"`
				Committed      bool            `json:"committed"`
				BusinessID     string          `json:"business_id"`
				ServiceVariant int64           `json:"service_variant_id"`
				Service        string          `json:"service"`
				Staffer        string          `json:"staffer"`
				Price          string          `json:"price"`
				BookedFrom     string          `json:"booked_from"`
				Response       json.RawMessage `json:"response"`
			}{Preview: false, Committed: true, BusinessID: businessID, ServiceVariant: variantID, Service: svcName, Staffer: staffer, Price: priceLabel, BookedFrom: from, Response: bookData}
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, view, flags)
			}
			fmt.Fprintln(out, "✅ Booked!")
			fmt.Fprintf(out, "  Service : %s\n", orDash(svcName))
			fmt.Fprintf(out, "  Staffer : %s\n", orDash(staffer))
			fmt.Fprintf(out, "  When    : %s\n", from)
			fmt.Fprintf(out, "  Price   : %s\n", orDash(priceLabel))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagServiceVariant, "service-variant", "", "Service variant id to book (from `services`)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Appointment date, YYYY-MM-DD")
	cmd.Flags().StringVar(&flagTime, "time", "", "Appointment time, HH:MM")
	cmd.Flags().Int64Var(&flagStaffer, "staffer", -1, "Staffer id, or -1 for any staffer")
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
