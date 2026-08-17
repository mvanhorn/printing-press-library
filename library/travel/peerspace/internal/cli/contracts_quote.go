// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Guest quote / request-to-book pricing (contact space owner flow HAR 2026-07-16).
// POST /v1/contracts/request/guest/quote?preferred_locale=en-US

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newContractsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "contracts",
		Short:       "Guest booking quotes and request contracts",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newContractsGuestQuoteCmd(flags))
	cmd.AddCommand(newContractsInquiryQuoteCmd(flags))
	cmd.AddCommand(newContractsInquirySendCmd(flags))
	return cmd
}

func newContractsGuestQuoteCmd(flags *rootFlags) *cobra.Command {
	var (
		flagListingID   string
		flagSpaceID     string
		flagHostID      string
		flagGuestID     string
		flagYear        int
		flagMonth       int
		flagDay         int
		flagStartIndex  int
		flagStartHour   int
		flagStartMinute int
		flagStartLabel  string
		flagEndIndex    int
		flagEndHour     int
		flagEndMinute   int
		flagEndLabel    string
		flagRate        float64
		flagGuests      int
		flagLocale      string
		flagPrepareOnly bool
		flagType        string
	)

	cmd := &cobra.Command{
		Use:   "guest-quote",
		Short: "Price a guest booking request (POST /v1/contracts/request/guest/quote).",
		Long: `Build a prepare-only guest quote for a listing window — the same call the
listing page uses when contacting a host / starting a request.

HAR body shape (simplified): host_sso_id, guest_sso_id, type=REQUEST,
payment_option=PAY_IN_FULL, items[0].info with schedule start/end, rate,
listing_id, space_id, number_of_guests, prepare_only=true.

Host/guest default from hydrated listing + PSUser cookie when omitted.`,
		Example: `  peerspace-pp-cli contracts guest-quote --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 9 --day 25 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --rate 85 --prepare-only --json`,
		Annotations: map[string]string{
			"pp:endpoint":         "contracts.guest_quote",
			"pp:method":           "POST",
			"pp:path":             "/v1/contracts/request/guest/quote",
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
				}
				return nil
			}
			listingID := strings.TrimSpace(flagListingID)
			if listingID == "" {
				return fmt.Errorf("--listing-id is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Auto-fill space/host/rate from hydrated detail when missing.
			if flagSpaceID == "" || flagHostID == "" || flagRate == 0 {
				if data, err := fetchListingDetail(ctx, c, listingID); err == nil {
					var m map[string]any
					if json.Unmarshal(data, &m) == nil {
						if flagSpaceID == "" {
							flagSpaceID = firstMapString(m, "parentSpaceId", "parent_space_id", "space_id")
						}
						if flagHostID == "" {
							flagHostID = firstMapString(m, "ownerId", "owner_id", "host_id")
						}
						if flagRate == 0 {
							if l, ok := venuex.ParseListing(data); ok && l.PriceHourly > 0 {
								flagRate = l.PriceHourly
							}
						}
					}
				}
			}
			guestID := strings.TrimSpace(flagGuestID)
			if guestID == "" && c.Config != nil {
				guestID = cookieValue(c.Config.CookieCredential(), "PSUser")
			}
			if strings.TrimSpace(flagHostID) == "" || strings.TrimSpace(flagSpaceID) == "" || guestID == "" {
				return fmt.Errorf("need --host-id, --space-id, and guest id (login or --guest-id); try venues get <listing> first")
			}
			if flagYear < 2000 || flagMonth < 1 || flagDay < 1 {
				return fmt.Errorf("--year/--month/--day required")
			}
			if flagStartIndex < 0 || flagEndIndex < 0 {
				return fmt.Errorf("--start-index and --end-index required (from calendar availability)")
			}
			if flagGuests <= 0 {
				flagGuests = 1
			}
			if flagType == "" {
				flagType = "REQUEST"
			}
			if flagLocale == "" {
				flagLocale = "en-US"
			}

			body := map[string]any{
				"host_sso_id":           strings.TrimSpace(flagHostID),
				"guest_sso_id":          guestID,
				"sales_tax_disable":     true,
				"type":                  flagType,
				"payment_option":        "PAY_IN_FULL",
				"enforce_minimum_hours": true,
				"guest_fee_test_group":  "full",
				"items": []any{
					map[string]any{
						"type": "BOOKING",
						"info": map[string]any{
							"host_id":  strings.TrimSpace(flagHostID),
							"guest_id": guestID,
							"schedule": []any{
								map[string]any{
									"start": map[string]any{
										"date": map[string]any{"y": flagYear, "m": flagMonth, "d": flagDay},
										"time": map[string]any{
											"time_index": flagStartIndex,
											"time":       defaultLabel(flagStartLabel, flagStartHour, flagStartMinute),
											"h":          flagStartHour,
											"m":          flagStartMinute,
											"overnight":  false,
											"available":  true,
										},
									},
									"end": map[string]any{
										"date": map[string]any{"y": flagYear, "m": flagMonth, "d": flagDay},
										"time": map[string]any{
											"time_index": flagEndIndex,
											"time":       defaultLabel(flagEndLabel, flagEndHour, flagEndMinute),
											"h":          flagEndHour,
											"m":          flagEndMinute,
											"overnight":  false,
											"available":  true,
										},
									},
								},
							},
							"rate":             flagRate,
							"duration_type":    "HOURLY_RATE",
							"listing_id":       listingID,
							"space_id":         strings.TrimSpace(flagSpaceID),
							"prepare_only":     flagPrepareOnly,
							"ga_client_id":     "",
							"number_of_guests": flagGuests,
						},
					},
				},
			}

			path := "/v1/contracts/request/guest/quote"
			params := map[string]string{"preferred_locale": flagLocale}
			headers := listingAuthHeaders(c)
			data, status, err := c.PostWithParamsAndHeaders(ctx, path, params, body, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status >= 400 {
				return classifyAPIError(fmt.Errorf("POST %s returned HTTP %d: %s", path, status, truncateForErr(data, 240)), flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&flagListingID, "listing-id", "", "Listing id (required)")
	cmd.Flags().StringVar(&flagSpaceID, "space-id", "", "Space id (parentSpaceId); auto from venues get when omitted")
	cmd.Flags().StringVar(&flagHostID, "host-id", "", "Host SSO id (ownerId); auto from venues get when omitted")
	cmd.Flags().StringVar(&flagGuestID, "guest-id", "", "Guest SSO id (defaults to PSUser cookie)")
	cmd.Flags().IntVar(&flagYear, "year", 0, "Event year")
	cmd.Flags().IntVar(&flagMonth, "month", 0, "Event month 1-12")
	cmd.Flags().IntVar(&flagDay, "day", 0, "Event day")
	cmd.Flags().IntVar(&flagStartIndex, "start-index", -1, "Start time_index from calendar availability-start")
	cmd.Flags().IntVar(&flagStartHour, "start-hour", 17, "Start hour 0-23")
	cmd.Flags().IntVar(&flagStartMinute, "start-minute", 0, "Start minute")
	cmd.Flags().StringVar(&flagStartLabel, "start-label", "", "Start label e.g. \"5:00 pm\"")
	cmd.Flags().IntVar(&flagEndIndex, "end-index", -1, "End time_index from calendar availability-end")
	cmd.Flags().IntVar(&flagEndHour, "end-hour", 22, "End hour 0-23")
	cmd.Flags().IntVar(&flagEndMinute, "end-minute", 0, "End minute")
	cmd.Flags().StringVar(&flagEndLabel, "end-label", "", "End label e.g. \"10:00 pm\"")
	cmd.Flags().Float64Var(&flagRate, "rate", 0, "Hourly rate (auto from listing when omitted)")
	cmd.Flags().IntVar(&flagGuests, "guests", 1, "Number of guests")
	cmd.Flags().StringVar(&flagLocale, "locale", "en-US", "preferred_locale query")
	cmd.Flags().BoolVar(&flagPrepareOnly, "prepare-only", true, "prepare_only flag (HAR default true for quote UI)")
	cmd.Flags().StringVar(&flagType, "type", "REQUEST", "Contract type (REQUEST)")
	_ = cmd.MarkFlagRequired("listing-id")
	_ = cmd.MarkFlagRequired("year")
	_ = cmd.MarkFlagRequired("month")
	_ = cmd.MarkFlagRequired("day")
	_ = cmd.MarkFlagRequired("start-index")
	_ = cmd.MarkFlagRequired("end-index")
	return cmd
}

func defaultLabel(label string, h, m int) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	ampm := "am"
	hh := h
	if h >= 12 {
		ampm = "pm"
		if h > 12 {
			hh = h - 12
		}
	}
	if hh == 0 {
		hh = 12
	}
	return fmt.Sprintf("%d:%02d %s", hh, m, ampm)
}

func firstMapString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
