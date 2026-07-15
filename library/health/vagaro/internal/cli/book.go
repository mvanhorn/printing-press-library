// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: book an appointment. Follows the side-effect convention:
// print-by-default (verify the slot and show what WOULD be booked), require
// --confirm to act, and short-circuit under PRINTING_PRESS_VERIFY=1.
//
// The real booking-submit endpoint was NOT captured during discovery (Angular
// checkout widget), so --confirm returns the tightest booking URL (the service
// list, /{slug}/services) plus explicit numbered steps naming the verified
// service/provider/time, so the manual finish is as few clicks as possible.
// A real submit call drops into placeBooking() once the endpoint is captured.
// generate --force preserves this body.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/vagaro/internal/vagaro"
	"github.com/spf13/cobra"
)

// atLayout is the --at timestamp format (business-local wall-clock, minute
// precision). The value is compared against Vagaro's business-local slot labels
// as naive wall-clock, so it is parsed in a fixed neutral frame (UTC) rather
// than the caller's zone — see slotOpen.
const atLayout = "2006-01-02T15:04"

type bookResult struct {
	Action        string   `json:"action"` // "would_book" or "confirm_via_url"
	Slug          string   `json:"slug"`
	BusinessID    string   `json:"business_id,omitempty"`
	BusinessName  string   `json:"business_name,omitempty"`
	ServiceID     string   `json:"service_id"`
	ServiceName   string   `json:"service_name,omitempty"`
	ProviderID    string   `json:"provider_id"`
	ProviderName  string   `json:"provider_name,omitempty"`
	At            string   `json:"at"`
	SlotOpen      bool     `json:"slot_open"`
	BookNowURL    string   `json:"book_now_url,omitempty"`
	Steps         []string `json:"steps,omitempty"`
	Message       string   `json:"message"`
	AvailableSame []string `json:"available_that_day,omitempty"`
}

// bookingURL returns the landing page that gets a manual booking closest to done.
// Vagaro's widget does not accept service/provider/date URL params (verified), and
// /{slug}/services lands directly on the service list with a per-service "Book Now"
// button — one step tighter than /{slug}/book-now, which needs a service picked first.
func bookingURL(slug string) string {
	return fmt.Sprintf("https://www.vagaro.com/%s/services", slug)
}

// bookingSteps spells out the minimal manual clicks so the handoff is unambiguous.
func bookingSteps(res bookResult) []string {
	svc := firstNonEmpty(res.ServiceName, "service "+res.ServiceID)
	prov := firstNonEmpty(res.ProviderName, "the provider")
	steps := []string{
		"Open " + res.BookNowURL,
		fmt.Sprintf("Click \"Book Now\" next to %q", svc),
	}
	if res.ProviderID != "" {
		steps = append(steps, fmt.Sprintf("Choose provider %s", prov))
	}
	steps = append(steps,
		fmt.Sprintf("Pick %s", res.At),
		"Review and confirm",
	)
	return steps
}

func newBookCmd(flags *rootFlags) *cobra.Command {
	var (
		flagService  string
		flagProvider string
		flagAt       string
		flagConfirm  bool
	)

	cmd := &cobra.Command{
		Use:     "booking-link <slug> --service <id> --provider <id> --at <YYYY-MM-DDTHH:MM>",
		Aliases: []string{"book"},
		Short:   "Verify a slot is open and get a ready-to-finish booking link (does not place the appointment).",
		Long: `Verify that a specific appointment slot is open and hand off a booking link.

This does NOT place the appointment — Vagaro has no booking-submit API (the
checkout is an Angular widget). By default it only VERIFIES the slot against the
live availability endpoint and prints what it would book. Pass --confirm for a
one-click-away handoff: the tightest booking URL plus the exact steps (which
service to click, which provider, which time) to finish in the browser in as few
clicks as possible. Aliased as "book".`,
		Example:     "  vagaro-pp-cli booking-link centralbarber --service 34098477 --provider 43931725 --at 2026-07-24T10:00",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "slug=centralbarber;service=34098477;provider=43931725;at=2026-07-24T10:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug> --service <id> --provider <id> --at <YYYY-MM-DDTHH:MM>", cmd.CommandPath()))
			}
			service := strings.TrimSpace(flagService)
			provider := strings.TrimSpace(flagProvider)
			atStr := strings.TrimSpace(flagAt)
			if service == "" || atStr == "" {
				return usageErr(fmt.Errorf("--service and --at are required\nUsage: %s <slug> --service <id> --provider <id> --at <YYYY-MM-DDTHH:MM>", cmd.CommandPath()))
			}
			// Parse in a fixed neutral frame (UTC) so the entered wall-clock
			// time is never shifted by the caller's local zone before it is
			// compared, as naive wall-clock, against business-local slot labels.
			at, err := time.ParseInLocation(atLayout, atStr, time.UTC)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --at %q (want YYYY-MM-DDTHH:MM, e.g. 2026-07-24T10:00): %w", atStr, err))
			}
			if dryRunOK(flags) {
				return nil
			}
			timeLabel := at.Format("3:04 PM")

			// Side-effect floor: under the verifier, never dial. Print what it
			// would book and return success without a network call.
			if cliutil.IsVerifyEnv() {
				res := bookResult{
					Action: "would_book", Slug: slug, ServiceID: service, ProviderID: provider,
					At:      at.Format("Mon Jan 2 3:04 PM"),
					Message: fmt.Sprintf("would book: service %s with provider %s at %s on %s (verify mode: no network call)", service, provider, slug, at.Format("Mon Jan 2 3:04 PM")),
				}
				return emitVagaro(cmd, flags, res)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			prof, err := c.FetchProfile(ctx, slug)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			businessID := prof.BusinessID

			appDate, err := vagaro.FormatAppDate(at.Format("2006-01-02"))
			if err != nil {
				return err
			}
			groups, err := c.Availability(ctx, businessID, service, provider, appDate)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			open, sameDay := slotOpen(groups, at, timeLabel, provider)

			res := bookResult{
				Slug:          slug,
				BusinessID:    businessID,
				BusinessName:  prof.Name,
				ServiceID:     service,
				ServiceName:   lookupServiceName(ctx, c, businessID, service),
				ProviderID:    provider,
				ProviderName:  lookupProviderName(ctx, c, businessID, provider),
				At:            at.Format("Mon Jan 2 3:04 PM"),
				SlotOpen:      open,
				AvailableSame: sameDay,
			}
			label := bookLabel(res)

			if !open {
				res.Action = "slot_unavailable"
				res.Message = fmt.Sprintf("requested slot is NOT open: %s", label)
				if len(sameDay) > 0 {
					res.Message += fmt.Sprintf(" — open that day: %s", strings.Join(sameDay, ", "))
				}
				// Not an error exit: the verification succeeded, the answer is "no".
				return emitVagaro(cmd, flags, res)
			}

			if !flagConfirm {
				res.Action = "would_book"
				res.Message = "would book: " + label + "  (pass --confirm to proceed)"
				return emitVagaro(cmd, flags, res)
			}

			// --confirm: the real submit endpoint is not wired. Hand off to the
			// browser as smoothly as possible — land on the service list (the
			// closest deep-link Vagaro's widget allows) and spell out the exact
			// clicks with the verified service/provider/time.
			res.Action = "confirm_via_url"
			res.BookNowURL = bookingURL(slug)
			res.Steps = bookingSteps(res)
			res.Message = fmt.Sprintf("slot is open. Finish this booking in the browser (Vagaro has no submit API): %s — %s",
				res.BookNowURL, strings.Join(res.Steps, " → "))
			return placeBooking(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&flagService, "service", "", "Service ID to book (required)")
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Provider ID to book with")
	cmd.Flags().StringVar(&flagAt, "at", "", "Slot time as YYYY-MM-DDTHH:MM (required)")
	cmd.Flags().BoolVar(&flagConfirm, "confirm", false, "Proceed with the booking (prints the book-now URL to finish in-browser)")
	return cmd
}

// placeBooking is the seam where a real submit POST drops in once the
// booking-submit endpoint is captured. For now it only emits the confirm result
// carrying the book-now URL.
//
// TODO: wire real submit endpoint once captured. When available, POST the
// booking payload (businessID, serviceID, providerID, slot datetime) via the
// authed client here and set res.Action = "booked" on success.
func placeBooking(cmd *cobra.Command, flags *rootFlags, res bookResult) error {
	return emitVagaro(cmd, flags, res)
}

// slotOpen reports whether the requested datetime appears in the availability
// groups. Also returns the times open on that same day for a helpful hint when
// the exact slot is taken.
func slotOpen(groups []vagaro.SlotGroup, at time.Time, timeLabel, wantProvider string) (bool, []string) {
	// Compare as naive wall-clock: the requested --at and Vagaro's slot labels
	// are both business-local wall-clock strings, so match on year/month/day/
	// hour/minute components rather than as absolute instants that could differ
	// by zone. at.Year()/at.Hour()/etc. read the wall-clock in at's own zone.
	wantY, wantMo, wantD := at.Date()
	wantH, wantMin := at.Hour(), at.Minute()
	wantProvider = strings.TrimSpace(wantProvider)
	sameDay := make([]string, 0)
	open := false
	for _, g := range groups {
		// Respect per-slot provider attribution: when booking a specific
		// provider, never treat a group explicitly attributed to a DIFFERENT
		// provider as availability for the requested one. Unattributed groups
		// (empty ProviderID) are accepted because the availability call is
		// already scoped to the requested provider.
		if wantProvider != "" && g.ProviderID != "" && g.ProviderID != wantProvider {
			continue
		}
		for _, t := range g.Times {
			dt, ok := vagaro.ParseSlotDateTime(g.Date, t)
			if !ok {
				// Dateless fallback: match on the clock label alone.
				if strings.EqualFold(strings.TrimSpace(t), timeLabel) {
					open = true
				}
				continue
			}
			y, mo, d := dt.Date()
			if y != wantY || mo != wantMo || d != wantD {
				continue
			}
			sameDay = append(sameDay, t)
			if dt.Hour() == wantH && dt.Minute() == wantMin {
				open = true
			}
		}
	}
	return open, sameDay
}

func bookLabel(res bookResult) string {
	svc := firstNonEmpty(res.ServiceName, "service "+res.ServiceID)
	prov := firstNonEmpty(res.ProviderName, "provider "+res.ProviderID)
	biz := firstNonEmpty(res.BusinessName, res.Slug)
	return fmt.Sprintf("%s with %s at %s on %s", svc, prov, biz, res.At)
}

func lookupServiceName(ctx context.Context, c *vagaro.Client, businessID, serviceID string) string {
	services, err := c.Services(ctx, businessID)
	if err != nil {
		return ""
	}
	for _, s := range services {
		if strconv.FormatInt(s.ServiceID, 10) == serviceID {
			return s.ServiceTitle
		}
	}
	return ""
}

func lookupProviderName(ctx context.Context, c *vagaro.Client, businessID, providerID string) string {
	if providerID == "" {
		return ""
	}
	staff, err := c.Staff(ctx, businessID)
	if err != nil {
		return ""
	}
	for _, p := range staff {
		if strconv.FormatInt(p.ServiceProviderID, 10) == providerID {
			return p.Name
		}
	}
	return ""
}
