// Compute a serial, bounded matrix from live dated public-source observations.
// pp:data-source computed
package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

type publicFetchFailure struct {
	HotelID string `json:"hotel_id"`
	Checkin string `json:"checkin"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
type publicCompareCell struct {
	Query   travel.OfferQuery   `json:"query"`
	Status  string              `json:"status"`
	Source  *travel.SourceInfo  `json:"source"`
	Page    *travel.PageInfo    `json:"page"`
	Minimum *publicOfferHit     `json:"bounded_source_page_minimum"`
	Error   *publicFetchFailure `json:"error"`
}
type publicCompareResult struct {
	Cells         []publicCompareCell  `json:"cells"`
	FetchFailures []publicFetchFailure `json:"fetch_failures"`
}

func compareCellFailure(query travel.OfferQuery, err error) publicCompareCell {
	kind := "fetch_error"
	var source *travel.SourceError
	if errors.As(err, &source) {
		kind = source.Kind
	}
	failure := &publicFetchFailure{query.HotelID, query.Checkin, kind, err.Error()}
	return publicCompareCell{Query: query, Status: "error", Error: failure}
}

func compareTravelCell(ctx context.Context, api travel.API, query travel.OfferQuery) publicCompareCell {
	cell := publicCompareCell{Query: query, Status: travel.StatusNoAvailability}
	scanQuery := query
	seenOffsets := map[int]bool{}
	for {
		if seenOffsets[scanQuery.Offset] {
			return compareCellFailure(query, fmt.Errorf("repeated within-page continuation"))
		}
		seenOffsets[scanQuery.Offset] = true
		result, err := api.Offers(ctx, scanQuery)
		if err != nil {
			return compareCellFailure(query, err)
		}
		source, page := result.Source, result.Page
		cell.Source, cell.Page = &source, &page
		for _, offer := range result.Offers {
			if offer.Price.PerRoomStayJPY <= 0 || offer.Price.Currency != "JPY" {
				return compareCellFailure(query, fmt.Errorf("source returned a nonpositive or unsupported quote"))
			}
			if cell.Minimum == nil || offer.Price.PerRoomStayJPY < cell.Minimum.Price.PerRoomStayJPY {
				selected := compactOffer(offer)
				cell.Minimum = &selected
			}
		}
		if !page.HasMore || page.NextPage == nil || *page.NextPage != query.Page {
			break
		}
		if page.NextOffset == nil || *page.NextOffset <= scanQuery.Offset {
			return compareCellFailure(query, fmt.Errorf("invalid within-page continuation"))
		}
		scanQuery.Offset = *page.NextOffset
	}
	if cell.Minimum != nil {
		cell.Status = travel.StatusOK
	}
	return cell
}

func distinctTravelValues(raw, label string) ([]string, error) {
	values := strings.Split(raw, ",")
	if len(values) > 9 {
		return nil, usageErr(fmt.Errorf("--%s has too many values; matrix maximum is 9 cells", label))
	}
	seen := map[string]bool{}
	for i, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return nil, usageErr(fmt.Errorf("--%s must contain distinct nonempty comma-separated values", label))
		}
		values[i], seen[value] = value, true
	}
	return values, nil
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var hotelsCSV, checkinsCSV string
	var nights int
	var party travel.OfferQuery
	cmd := &cobra.Command{
		Use: "compare", Short: "Compare at most nine explicit hotel/date cells with equal nights and per-room party",
		Example:     "  rakuten-travel-pp-cli compare --hotels 51870,72056 --checkins 2026-11-08,2026-11-09 --nights 2 --rooms 1 --adults-per-room 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--hotels=51870;--checkins=2026-11-08;--nights=2;--rooms=1;--adults-per-room=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := requireTravelFlags(cmd, "hotels", "checkins", "nights", "rooms", "adults-per-room"); err != nil {
				return err
			}
			hotels, err := distinctTravelValues(hotelsCSV, "hotels")
			if err != nil {
				return err
			}
			checkins, err := distinctTravelValues(checkinsCSV, "checkins")
			if err != nil {
				return err
			}
			if len(hotels)*len(checkins) > 9 {
				return usageErr(fmt.Errorf("comparison matrix is %d cells; maximum is 9", len(hotels)*len(checkins)))
			}
			if nights < 1 || nights > 28 {
				return usageErr(fmt.Errorf("--nights must be 1–28"))
			}
			queries := make([]travel.OfferQuery, 0, len(hotels)*len(checkins))
			for _, hotel := range hotels {
				for _, checkin := range checkins {
					in, err := time.Parse("2006-01-02", checkin)
					if err != nil || in.Format("2006-01-02") != checkin {
						return usageErr(fmt.Errorf("--checkins must contain explicit YYYY-MM-DD dates"))
					}
					query := party
					query.HotelID, query.Checkin, query.Checkout = hotel, checkin, in.AddDate(0, 0, nights).Format("2006-01-02")
					query.Page, query.Offset, query.Limit = 1, 0, 100
					if err := query.Validate(); err != nil {
						return usageErr(err)
					}
					queries = append(queries, query)
				}
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result := publicCompareResult{Cells: make([]publicCompareCell, 0, len(queries)), FetchFailures: []publicFetchFailure{}}
			for _, query := range queries {
				cell := compareTravelCell(ctx, api, query)
				result.Cells = append(result.Cells, cell)
				if cell.Error != nil {
					result.FetchFailures = append(result.FetchFailures, *cell.Error)
				}
			}
			status := "ok"
			if len(result.FetchFailures) != 0 {
				status = "partial_failure"
				fmt.Fprintf(cmd.ErrOrStderr(), "compare: %d of %d cells failed; inspect results.fetch_failures\n", len(result.FetchFailures), len(result.Cells))
			}
			stats := api.Stats()
			transport := "live"
			if stats.CacheHits > 0 {
				transport = "cache"
				if stats.Requests > 0 {
					transport = "mixed"
				}
			}
			meta := map[string]any{
				"source": "computed", "transport": transport, "status": status,
				"coverage": "one_source_plan_page_per_cell", "price_comparison_scope": "bounded_source_page_only",
				"query":    map[string]any{"hotels": hotels, "checkins": checkins, "nights": nights, "rooms": party.Rooms, "adults_per_room": party.AdultsPerRoom, "children_per_room": party.Children},
				"requests": api.Stats(),
			}
			if err := options.output(cmd, flags, meta, result); err != nil {
				return err
			}
			if len(result.FetchFailures) == len(result.Cells) {
				return apiErr(fmt.Errorf("every comparison cell failed"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&hotelsCSV, "hotels", "", "Required: distinct numeric property IDs, comma-separated")
	cmd.Flags().StringVar(&checkinsCSV, "checkins", "", "Required: distinct check-in dates YYYY-MM-DD, comma-separated")
	cmd.Flags().IntVar(&nights, "nights", 0, "Required: equal stay length for every cell (1–28 nights)")
	bindTravelParty(cmd, &party)
	options.bind(cmd)
	return cmd
}
