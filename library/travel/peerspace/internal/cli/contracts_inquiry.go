// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Message-host / inquiry flow (HAR: www.peerspace.com-message host.har, 2026-07-16).
//
//   POST /v1/contracts/inquiry/guest/quote  — price while filling the inquiry form
//   POST /v1/contracts/inquiry/guest        — submit inquiry + message to host

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/peerspace/internal/venuex"
	"github.com/spf13/cobra"
)

func newContractsInquiryQuoteCmd(flags *rootFlags) *cobra.Command {
	p := defaultInquiryFlags()
	cmd := &cobra.Command{
		Use:   "inquiry-quote",
		Short: "Price a host inquiry while composing a message (POST /v1/contracts/inquiry/guest/quote).",
		Long: `Quote an INQUIRY (message-host form) without submitting.

HAR path: POST /v1/contracts/inquiry/guest/quote
Body type=INQUIRY with schedule, guest/host ids, listing_id, number_of_guests,
optional tags.activity and inquiry_message (usually empty until submit).`,
		Example: `  peerspace-pp-cli contracts inquiry-quote --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 9 --day 25 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --json`,
		Annotations: map[string]string{
			"pp:endpoint":         "contracts.inquiry_quote",
			"pp:method":           "POST",
			"pp:path":             "/v1/contracts/inquiry/guest/quote",
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInquiryPost(cmd, flags, p, "/v1/contracts/inquiry/guest/quote", false)
		},
	}
	bindInquiryFlags(cmd, p)
	return cmd
}

func newContractsInquirySendCmd(flags *rootFlags) *cobra.Command {
	p := defaultInquiryFlags()
	cmd := &cobra.Command{
		Use:   "inquiry-send",
		Short: "Send a message/inquiry to a listing host (POST /v1/contracts/inquiry/guest).",
		Long: `Submit the message-host inquiry form.

HAR path: POST /v1/contracts/inquiry/guest
Same body as inquiry-quote plus required inquiry_message (and optional alcohol_consumed).

This contacts the host — use carefully.`,
		Example: `  peerspace-pp-cli contracts inquiry-send --listing-id 68d468bb44492187e415d4a6 --year 2026 --month 9 --day 25 --start-index 22 --end-index 32 --start-hour 17 --end-hour 22 --guests 50 --activity meetup --message "Hi — interested in a meetup." --yes --json`,
		Annotations: map[string]string{
			"pp:endpoint":         "contracts.inquiry_send",
			"pp:method":           "POST",
			"pp:path":             "/v1/contracts/inquiry/guest",
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(p.message) == "" {
				return fmt.Errorf("--message is required to send an inquiry")
			}
			return runInquiryPost(cmd, flags, p, "/v1/contracts/inquiry/guest", true)
		},
	}
	bindInquiryFlags(cmd, p)
	cmd.Flags().StringVar(&p.message, "message", "", "Inquiry message to the host (required for send)")
	cmd.Flags().BoolVar(&p.alcohol, "alcohol-consumed", false, "Whether alcohol will be consumed")
	return cmd
}

type inquiryFlags struct {
	listingID   string
	spaceID     string
	hostID      string
	guestID     string
	year        int
	month       int
	day         int
	startIndex  int
	startHour   int
	startMinute int
	startLabel  string
	endIndex    int
	endHour     int
	endMinute   int
	endLabel    string
	guests      int
	activity    string
	message     string
	alcohol     bool
	flexible    bool
}

func defaultInquiryFlags() *inquiryFlags {
	return &inquiryFlags{
		startIndex: -1,
		endIndex:   -1,
		guests:     1,
	}
}

func bindInquiryFlags(cmd *cobra.Command, p *inquiryFlags) {
	cmd.Flags().StringVar(&p.listingID, "listing-id", "", "Listing id (required)")
	cmd.Flags().StringVar(&p.spaceID, "space-id", "", "Space id (optional; not required by inquiry body)")
	cmd.Flags().StringVar(&p.hostID, "host-id", "", "Host SSO id (auto from venues get)")
	cmd.Flags().StringVar(&p.guestID, "guest-id", "", "Guest SSO id (defaults to PSUser cookie)")
	cmd.Flags().IntVar(&p.year, "year", 0, "Event year")
	cmd.Flags().IntVar(&p.month, "month", 0, "Event month 1-12")
	cmd.Flags().IntVar(&p.day, "day", 0, "Event day")
	cmd.Flags().IntVar(&p.startIndex, "start-index", -1, "Start time_index")
	cmd.Flags().IntVar(&p.startHour, "start-hour", 17, "Start hour 0-23")
	cmd.Flags().IntVar(&p.startMinute, "start-minute", 0, "Start minute")
	cmd.Flags().StringVar(&p.startLabel, "start-label", "", "Start label e.g. \"3:30 pm\"")
	cmd.Flags().IntVar(&p.endIndex, "end-index", -1, "End time_index")
	cmd.Flags().IntVar(&p.endHour, "end-hour", 22, "End hour 0-23")
	cmd.Flags().IntVar(&p.endMinute, "end-minute", 0, "End minute")
	cmd.Flags().StringVar(&p.endLabel, "end-label", "", "End label e.g. \"9:30 pm\"")
	cmd.Flags().IntVar(&p.guests, "guests", 1, "Number of guests")
	cmd.Flags().StringVar(&p.activity, "activity", "", "Activity tag slug e.g. art-exhibit, meetup")
	cmd.Flags().BoolVar(&p.flexible, "flexible-date-time", false, "flexible_date_time flag")
	_ = cmd.MarkFlagRequired("listing-id")
	_ = cmd.MarkFlagRequired("year")
	_ = cmd.MarkFlagRequired("month")
	_ = cmd.MarkFlagRequired("day")
	_ = cmd.MarkFlagRequired("start-index")
	_ = cmd.MarkFlagRequired("end-index")
}

func runInquiryPost(cmd *cobra.Command, flags *rootFlags, p *inquiryFlags, path string, isSend bool) error {
	if dryRunOK(flags) {
		if flags.asJSON {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
		}
		return nil
	}
	listingID := strings.TrimSpace(p.listingID)
	if listingID == "" {
		return fmt.Errorf("--listing-id is required")
	}
	if p.year < 2000 || p.month < 1 || p.day < 1 {
		return fmt.Errorf("invalid --year/--month/--day")
	}
	if p.startIndex < 0 || p.endIndex < 0 {
		return fmt.Errorf("--start-index and --end-index required")
	}
	if p.guests <= 0 {
		p.guests = 1
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	// Auto-fill host from listing detail
	if strings.TrimSpace(p.hostID) == "" {
		if data, err := fetchListingDetail(ctx, c, listingID); err == nil {
			if l, ok := venuex.ParseListing(data); ok && l.HostID != "" {
				p.hostID = l.HostID
			} else {
				var m map[string]any
				if json.Unmarshal(data, &m) == nil {
					p.hostID = firstMapString(m, "ownerId", "owner_id", "host_id")
				}
			}
		}
	}
	guestID := strings.TrimSpace(p.guestID)
	if guestID == "" && c.Config != nil {
		guestID = cookieValue(c.Config.CookieCredential(), "PSUser")
	}
	if strings.TrimSpace(p.hostID) == "" || guestID == "" {
		return fmt.Errorf("need host id and guest id (login or pass --host-id / --guest-id)")
	}

	tags := map[string]any{}
	if a := strings.TrimSpace(p.activity); a != "" {
		tags["activity"] = a
	}

	info := map[string]any{
		"guest_id":   guestID,
		"host_id":    strings.TrimSpace(p.hostID),
		"listing_id": listingID,
		"tags":       tags,
		"schedule": []any{
			map[string]any{
				"start": map[string]any{
					"date": map[string]any{"y": p.year, "m": p.month, "d": p.day},
					"time": map[string]any{
						"time_index": p.startIndex,
						"time":       defaultLabel(p.startLabel, p.startHour, p.startMinute),
						"h":          p.startHour,
						"m":          p.startMinute,
						"overnight":  false,
						"available":  true,
					},
				},
				"end": map[string]any{
					"date": map[string]any{"y": p.year, "m": p.month, "d": p.day},
					"time": map[string]any{
						"time_index": p.endIndex,
						"time":       defaultLabel(p.endLabel, p.endHour, p.endMinute),
						"h":          p.endHour,
						"m":          p.endMinute,
						"overnight":  false,
						"available":  true,
					},
				},
			},
		},
		"number_of_guests":   p.guests,
		"inquiry_message":    p.message,
		"flexible_date_time": p.flexible,
	}
	if isSend {
		info["alcohol_consumed"] = p.alcohol
	}

	body := map[string]any{
		"guest_sso_id":          guestID,
		"host_sso_id":           strings.TrimSpace(p.hostID),
		"type":                  "INQUIRY",
		"enforce_minimum_hours": true,
		"guest_fee_test_group":  "full",
		"items": []any{
			map[string]any{
				"type": "BOOKING",
				"info": info,
			},
		},
	}

	// Safety: require --yes for real send (contacts host)
	if isSend && !flags.yes {
		return fmt.Errorf("inquiry-send contacts the host; pass --yes to confirm")
	}

	data, status, err := c.PostWithHeaders(ctx, path, body, listingAuthHeaders(c))
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status >= 400 {
		return classifyAPIError(fmt.Errorf("POST %s returned HTTP %d: %s", path, status, truncateForErr(data, 240)), flags)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) && !wantsMachineOutput(flags) {
		if isSend {
			fmt.Fprintf(cmd.OutOrStdout(), "Inquiry sent for listing %s\n", listingID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Inquiry quote for listing %s\n", listingID)
		}
	}
	return printJSONFiltered(cmd.OutOrStdout(), data, flags)
}
