// Inspect an exact dated hotel/plan/room tuple with bounded source-page scans.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
	"strings"
	"unicode"
)

type publicOfferInspection struct {
	Offer                      *travel.Offer  `json:"offer"`
	Property                   *travel.Hotel  `json:"property"`
	PropertyFeeNotes           []string       `json:"property_fee_notes"`
	PropertyCancellationPolicy *travel.Policy `json:"property_cancellation_policy"`
}

func validateOfferIdentity(plan, room string) error {
	if len(plan) == 0 || len(plan) > 64 {
		return usageErr(fmt.Errorf("--plan must be a numeric source plan ID"))
	}
	for _, char := range plan {
		if char < '0' || char > '9' {
			return usageErr(fmt.Errorf("--plan must be a numeric source plan ID"))
		}
	}
	if room == "" || len(room) > 200 || strings.TrimSpace(room) != room {
		return usageErr(fmt.Errorf("--room must be a nonempty exact source room ID"))
	}
	for _, char := range room {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return usageErr(fmt.Errorf("--room cannot contain whitespace or control characters"))
		}
	}
	return nil
}

func newNovelOffersShowCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var query travel.OfferQuery
	var plan, room string
	var maxScanPages int
	cmd := &cobra.Command{
		Use: "show", Short: "Inspect one exact dated offer, property fee notes and separately scoped policies",
		Example:     "  rakuten-travel-pp-cli offers show --hotel 51870 --plan 3951989 --room s-double- --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--hotel=51870;--plan=3951989;--room=s-double-;--checkin=2026-11-08;--checkout=2026-11-10;--rooms=1;--adults-per-room=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := validateTravelOfferQuery(cmd, query); err != nil {
				return err
			}
			if err := requireTravelFlags(cmd, "plan", "room"); err != nil {
				return err
			}
			if err := validateOfferIdentity(plan, room); err != nil {
				return err
			}
			if maxScanPages < 1 || maxScanPages > 3 {
				return usageErr(fmt.Errorf("--max-scan-pages must be 1–3"))
			}
			if options.maxRequests > 12 {
				return usageErr(fmt.Errorf("offers show --max-requests cannot exceed 12"))
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			var selected *travel.Offer
			var last travel.OfferResult
			pages := make([]int, 0, maxScanPages)
			inspectedRows := 0
			scanQuery := query
			for n := 0; n < maxScanPages && selected == nil; n++ {
				pages = append(pages, scanQuery.Page)
				seenOffsets := map[int]bool{}
				for {
					if seenOffsets[scanQuery.Offset] {
						return apiErr(fmt.Errorf("source returned a repeated within-page continuation"))
					}
					seenOffsets[scanQuery.Offset] = true
					fetched, err := api.Offers(ctx, scanQuery)
					if err != nil {
						meta := publicTravelMeta("error", last.Source, &last.Page, query, api)
						meta["identity"] = map[string]string{"hotel_id": query.HotelID, "plan_id": plan, "room_id": room}
						meta["inspected_source_pages"], meta["error"] = pages, publicTravelErrorInfo(err)
						if last.Source.URL == "" {
							meta["source_info"], meta["page"] = nil, nil
						}
						if outputErr := options.output(cmd, flags, meta, publicOfferInspection{PropertyFeeNotes: []string{}}); outputErr != nil {
							return outputErr
						}
						return publicTravelError(err)
					}
					last = fetched
					if scanQuery.Offset == 0 {
						inspectedRows += last.Page.RowsScanned
					}
					for _, offer := range last.Offers {
						if offer.HotelID == query.HotelID && offer.PlanID == plan && offer.RoomID == room {
							value := offer
							selected = &value
							break
						}
					}
					if selected != nil || !last.Page.HasMore || last.Page.NextPage == nil || *last.Page.NextPage != scanQuery.Page {
						break
					}
					if last.Page.NextOffset == nil || *last.Page.NextOffset <= scanQuery.Offset {
						return apiErr(fmt.Errorf("source returned an invalid within-page continuation"))
					}
					scanQuery.Offset = *last.Page.NextOffset
				}
				if selected != nil || !last.Page.HasMore || last.Page.NextPage == nil || *last.Page.NextPage <= scanQuery.Page {
					break
				}
				scanQuery.Page = *last.Page.NextPage
				scanQuery.Offset = 0
			}
			meta := publicTravelMeta("ok", last.Source, &last.Page, query, api)
			meta["identity"] = map[string]string{"hotel_id": query.HotelID, "plan_id": plan, "room_id": room}
			meta["inspected_source_pages"], meta["max_scan_pages"] = pages, maxScanPages
			meta["inspected_room_rows"] = inspectedRows
			if selected == nil {
				meta["status"] = "not_found_within_inspected_pages"
				meta["note"] = "The exact tuple was absent only from inspected source pages; increase --max-scan-pages (up to 3) or choose a different --page. This does not prove that the offer is nonexistent."
				if err := options.output(cmd, flags, meta, publicOfferInspection{PropertyFeeNotes: []string{}}); err != nil {
					return err
				}
				return notFoundErr(fmt.Errorf("offer %s/%s/%s was not found within the inspected source pages %v; this does not establish that the offer never exists", query.HotelID, plan, room, pages))
			}
			property, err := api.Hotel(ctx, query.HotelID)
			if err != nil {
				meta["status"], meta["error"], meta["requests"] = "property_inspection_error", publicTravelErrorInfo(err), api.Stats()
				if outputErr := options.output(cmd, flags, meta, publicOfferInspection{Offer: selected, PropertyFeeNotes: []string{}}); outputErr != nil {
					return outputErr
				}
				return publicTravelError(fmt.Errorf("offer found but property inspection failed: %w", err))
			}
			meta["property_source"], meta["property_details_source"], meta["requests"] = property.Source, property.DetailsSource, api.Stats()
			policy := &travel.Policy{Level: "property", Text: append([]string{}, property.Hotel.CancellationPolicy...), Caveat: property.Hotel.PolicyCaveat}
			return options.output(cmd, flags, meta, publicOfferInspection{selected, &property.Hotel, append([]string{}, property.Hotel.Notes...), policy})
		},
	}
	bindTravelOfferQuery(cmd, &query, false)
	bindTravelParty(cmd, &query)
	cmd.Flags().StringVar(&plan, "plan", "", "Required: exact numeric source plan ID")
	cmd.Flags().StringVar(&room, "room", "", "Required: exact opaque source room ID; hyphens are significant")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 1, "Maximum source plan pages to inspect, starting at --page (1–3)")
	options.bind(cmd)
	return cmd
}
