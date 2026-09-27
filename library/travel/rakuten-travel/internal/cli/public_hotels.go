// Search and inspect anonymous public Rakuten Travel property documents.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
)

func newPublicHotelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "hotels", Short: "Find properties by keyword or website area", Example: "  rakuten-travel-pp-cli hotels search --query 品川 --limit 5\n  rakuten-travel-pp-cli hotels show --hotel 51870", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}, RunE: publicTravelGroup}
	cmd.AddCommand(newPublicHotelsSearchCmd(flags))
	cmd.AddCommand(newPublicHotelsShowCmd(flags))
	return cmd
}

type publicHotelHit struct {
	ID                string              `json:"hotel_id"`
	Name              *string             `json:"name"`
	URL               string              `json:"url"`
	Rating            *travel.Rating      `json:"rating"`
	Address           *string             `json:"address"`
	Coordinates       *travel.Coordinates `json:"coordinates"`
	CoordinatesReason string              `json:"coordinates_reason"`
}

func newPublicHotelsSearchCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var query travel.HotelQuery
	cmd := &cobra.Command{
		Use: "search", Short: "Find compact property candidates using one literal selector",
		Example:     "  rakuten-travel-pp-cli hotels search --query 品川 --limit 5\n  rakuten-travel-pp-cli hotels search --area tokyo/E --page 1",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=品川;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := validateTravelWindow(query.Page, query.Offset, query.Limit); err != nil {
				return err
			}
			if err := query.Validate(); err != nil {
				return usageErr(err)
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result, err := api.SearchHotels(ctx, query)
			if err != nil {
				return publicTravelError(err)
			}
			hits := make([]publicHotelHit, 0, len(result.Hotels))
			for _, h := range result.Hotels {
				hits = append(hits, publicHotelHit{h.ID, h.Name, h.URL, h.Rating, h.Address, h.Coordinates, h.CoordinatesReason})
			}
			return options.output(cmd, flags, publicTravelMeta(result.Status, result.Source, &result.Page, result.Query, api), hits)
		},
	}
	cmd.Flags().StringVar(&query.Query, "query", "", "Literal Japanese or English keyword; exclusive with --area")
	cmd.Flags().StringVar(&query.Area, "area", "", "Website area path ID, e.g. tokyo/E; exclusive with --query")
	cmd.Flags().IntVar(&query.Page, "page", 1, "Source page to inspect")
	cmd.Flags().IntVar(&query.Offset, "offset", 0, "Skip this many hotels within the source page")
	cmd.Flags().IntVar(&query.Limit, "limit", 5, "Maximum emitted hotels (1–100)")
	options.bind(cmd)
	return cmd
}

func newPublicHotelsShowCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var hotel string
	cmd := &cobra.Command{
		Use: "show", Short: "Inspect facilities, source rating, access and property policies",
		Example:     "  rakuten-travel-pp-cli hotels show --hotel 51870",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--hotel=51870"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := requireTravelFlags(cmd, "hotel"); err != nil {
				return err
			}
			if err := travel.ValidateHotelID(hotel); err != nil {
				return usageErr(err)
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result, err := api.Hotel(ctx, hotel)
			if err != nil {
				return publicTravelError(err)
			}
			meta := publicTravelMeta(result.Status, result.Source, nil, map[string]string{"hotel_id": hotel}, api)
			meta["details_source"] = result.DetailsSource
			return options.output(cmd, flags, meta, result.Hotel)
		},
	}
	cmd.Flags().StringVar(&hotel, "hotel", "", "Numeric property ID")
	options.bind(cmd)
	return cmd
}
