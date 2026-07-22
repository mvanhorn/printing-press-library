// pp:data-source live
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/client"
	"github.com/spf13/cobra"
)

type searchEvent struct {
	ID                json.Number `json:"id"`
	Title             string      `json:"title"`
	Performer         string      `json:"performer"`
	PerformerSlug     string      `json:"performer_slug"`
	Venue             string      `json:"venue"`
	VenueSlug         string      `json:"venue_slug"`
	City              string      `json:"city"`
	State             string      `json:"state"`
	Date              string      `json:"date"`
	CategoryType      string      `json:"category_type"`
	EventCategoryName string      `json:"event_category_name"`
	GetInPrice        float64     `json:"get_in_price"`
	ThreeDayChangePct float64     `json:"three_day_change_pct"`
	Conditional       bool        `json:"conditional,omitempty"`
}

type searchMeta struct {
	UpcomingCount int `json:"upcoming_count"`
	PastCount     int `json:"past_count"`
	TotalMatches  int `json:"total_matches"`
}

type searchFilters struct {
	MinGetIn  float64
	MaxGetIn  float64
	GamesOnly bool
	Category  string
}

type rawSearchEvent struct {
	ID                json.Number       `json:"id"`
	Title             string            `json:"title"`
	Performer         string            `json:"performer"`
	PerformerSlug     string            `json:"performer_slug"`
	Venue             string            `json:"venue"`
	VenueSlug         string            `json:"venue_slug"`
	City              string            `json:"city"`
	State             string            `json:"state"`
	Date              string            `json:"date"`
	CategoryType      string            `json:"category_type"`
	EventCategoryName string            `json:"event_category_name"`
	GetInPrice        searchFloat       `json:"get_in_price"`
	ThreeDayChange    searchPriceChange `json:"3day_price_change"`
}

type searchPriceChange struct {
	Percent searchFloat `json:"percent"`
}

type searchFloat float64

func (f *searchFloat) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*f = 0
		return nil
	}
	var num json.Number
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&num); err == nil {
		v, err := num.Float64()
		if err != nil {
			return err
		}
		*f = searchFloat(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		*f = searchFloat(v)
		return nil
	}
	return fmt.Errorf("expected numeric value, got %s", string(data))
}

func (f searchFloat) Float64() float64 {
	return float64(f)
}

func newEventsListCmd(flags *rootFlags) *cobra.Command {
	var venue string
	var performer string
	var searchQuery string
	var minGetIn float64
	var maxGetIn float64
	var upcoming bool
	var past bool
	var all bool
	var since string
	var until string
	var day string
	var sortBy string
	var gamesOnly bool
	var category string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List events from TicketData search by venue, performer, or text query",
		Example:     "  ticketdata-pp-cli events list --venue lumen-field --min-get-in 200 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			params := buildSearchListParams(venue, performer, searchQuery, since, until, day, limit, offset)
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s\n", formatGETPreview("/search", params))
				return nil
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("events list does not accept arguments"))
			}
			if selectorCount(venue, performer, searchQuery) != 1 {
				return usageErr(fmt.Errorf("provide exactly one of --venue, --performer, or --search"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			if offset < 0 {
				return usageErr(fmt.Errorf("--offset must be non-negative"))
			}
			if err := validateSearchSort(sortBy); err != nil {
				return usageErr(err)
			}
			tab, err := searchTabFromFlags(cmd, upcoming, past, all)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			events, _, err := fetchSearchEvents(cmd.Context(), c, params, tab)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events = filterSearchEvents(events, searchFilters{
				MinGetIn:  minGetIn,
				MaxGetIn:  maxGetIn,
				GamesOnly: gamesOnly,
				Category:  category,
			})
			sortSearchEvents(events, sortBy)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			}
			return printSearchEventsTable(cmd, events)
		},
	}
	cmd.Flags().StringVar(&venue, "venue", "", "Venue slug to search")
	cmd.Flags().StringVar(&performer, "performer", "", "Performer slug to search")
	cmd.Flags().StringVar(&searchQuery, "search", "", "Free-text fuzzy search")
	cmd.Flags().Float64Var(&minGetIn, "min-get-in", 0, "Minimum get-in price")
	cmd.Flags().Float64Var(&maxGetIn, "max-get-in", 0, "Maximum get-in price; 0 means no ceiling")
	cmd.Flags().BoolVar(&upcoming, "upcoming", true, "Use upcoming events")
	cmd.Flags().BoolVar(&past, "past", false, "Use past events")
	cmd.Flags().BoolVar(&all, "all", false, "Use all events")
	cmd.Flags().StringVar(&since, "since", "", "Start date filter passed as start_date")
	cmd.Flags().StringVar(&until, "until", "", "End date filter passed as end_date")
	cmd.Flags().StringVar(&day, "day", "", "Comma-separated days of week passed as days_of_week")
	cmd.Flags().StringVar(&sortBy, "sort", "get_in", "Sort by get_in, date, or movers")
	cmd.Flags().BoolVar(&gamesOnly, "games-only", false, "Keep sports events and exclude tour rows")
	cmd.Flags().StringVar(&category, "category", "", "Keep events whose category name contains this value")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum events requested from the API")
	cmd.Flags().IntVar(&offset, "offset", 0, "API result offset")
	return cmd
}

func fetchSearchEvents(ctx context.Context, c *client.Client, params map[string]string, tab string) ([]searchEvent, searchMeta, error) {
	raw, err := c.Get(ctx, "/search", params)
	if err != nil {
		return nil, searchMeta{}, err
	}
	return parseSearchEvents(raw, tab)
}

func parseSearchEvents(raw json.RawMessage, tab string) ([]searchEvent, searchMeta, error) {
	var env struct {
		Status string `json:"status"`
		Data   struct {
			Events struct {
				Upcoming []rawSearchEvent `json:"upcoming"`
				Past     []rawSearchEvent `json:"past"`
				All      []rawSearchEvent `json:"all"`
			} `json:"events"`
			Metadata struct {
				Categorization struct {
					UpcomingCount int `json:"upcoming_count"`
					PastCount     int `json:"past_count"`
					TotalMatches  int `json:"total_matches"`
					AllCount      int `json:"all_count"`
					TotalCount    int `json:"total_count"`
				} `json:"categorization"`
			} `json:"metadata"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&env); err != nil {
		return make([]searchEvent, 0), searchMeta{}, err
	}
	if env.Status != "" && !strings.EqualFold(env.Status, "success") {
		return make([]searchEvent, 0), searchMeta{}, fmt.Errorf("search returned status %q", env.Status)
	}
	selected, err := selectRawSearchEvents(env.Data.Events.Upcoming, env.Data.Events.Past, env.Data.Events.All, tab)
	if err != nil {
		return make([]searchEvent, 0), searchMeta{}, err
	}
	events := make([]searchEvent, 0, len(selected))
	for _, rawEvent := range selected {
		events = append(events, mapRawSearchEvent(rawEvent))
	}
	cat := env.Data.Metadata.Categorization
	total := cat.TotalMatches
	if total == 0 {
		switch {
		case cat.AllCount > 0:
			total = cat.AllCount
		case cat.TotalCount > 0:
			total = cat.TotalCount
		default:
			total = cat.UpcomingCount + cat.PastCount
		}
	}
	return events, searchMeta{UpcomingCount: cat.UpcomingCount, PastCount: cat.PastCount, TotalMatches: total}, nil
}

func selectRawSearchEvents(upcoming, past, all []rawSearchEvent, tab string) ([]rawSearchEvent, error) {
	switch strings.ToLower(strings.TrimSpace(tab)) {
	case "", "upcoming":
		return nonNilRawSearchEvents(upcoming), nil
	case "past":
		return nonNilRawSearchEvents(past), nil
	case "all":
		return nonNilRawSearchEvents(all), nil
	default:
		return nil, fmt.Errorf("tab must be one of upcoming, past, all")
	}
}

func nonNilRawSearchEvents(events []rawSearchEvent) []rawSearchEvent {
	if events == nil {
		return make([]rawSearchEvent, 0)
	}
	return events
}

func mapRawSearchEvent(raw rawSearchEvent) searchEvent {
	event := searchEvent{
		ID:                raw.ID,
		Title:             raw.Title,
		Performer:         raw.Performer,
		PerformerSlug:     raw.PerformerSlug,
		Venue:             raw.Venue,
		VenueSlug:         raw.VenueSlug,
		City:              raw.City,
		State:             raw.State,
		Date:              raw.Date,
		CategoryType:      raw.CategoryType,
		EventCategoryName: raw.EventCategoryName,
		GetInPrice:        raw.GetInPrice.Float64(),
		ThreeDayChangePct: raw.ThreeDayChange.Percent.Float64(),
	}
	lowerTitle := strings.ToLower(event.Title)
	event.Conditional = strings.Contains(lowerTitle, "if necessary") || strings.Contains(lowerTitle, "tbd")
	return event
}

func filterSearchEvents(events []searchEvent, filters searchFilters) []searchEvent {
	out := make([]searchEvent, 0, len(events))
	for _, event := range events {
		if filters.MinGetIn > 0 && event.GetInPrice < filters.MinGetIn {
			continue
		}
		if filters.MaxGetIn > 0 && event.GetInPrice > filters.MaxGetIn {
			continue
		}
		if filters.GamesOnly && (!strings.EqualFold(event.CategoryType, "SPORT") || excludeTours(event)) {
			continue
		}
		if category := strings.TrimSpace(filters.Category); category != "" && !containsFold(event.EventCategoryName, category) {
			continue
		}
		out = append(out, event)
	}
	return out
}

func excludeTours(event searchEvent) bool {
	return containsFold(event.Title, "tour")
}

func sortSearchEvents(events []searchEvent, sortBy string) {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "date":
		sort.SliceStable(events, func(i, j int) bool {
			return searchDateLess(events[i].Date, events[j].Date)
		})
	case "movers":
		sort.SliceStable(events, func(i, j int) bool {
			return math.Abs(events[i].ThreeDayChangePct) > math.Abs(events[j].ThreeDayChangePct)
		})
	default:
		sort.SliceStable(events, func(i, j int) bool {
			return events[i].GetInPrice > events[j].GetInPrice
		})
	}
}

func searchDateLess(a, b string) bool {
	const layout = "Mon, 01/02/2006 03:04 PM"
	at, aerr := time.Parse(layout, a)
	bt, berr := time.Parse(layout, b)
	if aerr == nil && berr == nil {
		return at.Before(bt)
	}
	if aerr == nil {
		return true
	}
	if berr == nil {
		return false
	}
	return a < b
}

func limitSearchEvents(events []searchEvent, limit int) []searchEvent {
	if limit <= 0 || len(events) <= limit {
		return events
	}
	return events[:limit]
}

func validateSearchSort(sortBy string) error {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "get_in", "date", "movers":
		return nil
	default:
		return fmt.Errorf("--sort must be one of get_in, date, movers")
	}
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func searchTabFromFlags(cmd *cobra.Command, upcoming, past, all bool) (string, error) {
	selected := 0
	if cmd.Flags().Changed("upcoming") && upcoming {
		selected++
	}
	if past {
		selected++
	}
	if all {
		selected++
	}
	if selected > 1 {
		return "", fmt.Errorf("provide only one of --upcoming, --past, or --all")
	}
	if past {
		return "past", nil
	}
	if all {
		return "all", nil
	}
	return "upcoming", nil
}

func buildSearchListParams(venue, performer, searchQuery, since, until, day string, limit, offset int) map[string]string {
	params := map[string]string{
		"limit":  strconv.Itoa(limit),
		"offset": strconv.Itoa(offset),
	}
	if v := strings.TrimSpace(venue); v != "" {
		params["venue_slug"] = v
	}
	if v := strings.TrimSpace(performer); v != "" {
		params["performer_slug"] = v
	}
	if v := strings.TrimSpace(searchQuery); v != "" {
		params["f"] = v
	}
	if v := strings.TrimSpace(day); v != "" {
		params["days_of_week"] = v
	}
	if v := strings.TrimSpace(since); v != "" {
		params["start_date"] = v
	}
	if v := strings.TrimSpace(until); v != "" {
		params["end_date"] = v
	}
	return params
}

func selectorCount(values ...string) int {
	n := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			n++
		}
	}
	return n
}

func formatGETPreview(path string, params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		if key == "" {
			continue
		}
		values.Set(key, value)
	}
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func printSearchEventsTable(cmd *cobra.Command, events []searchEvent) error {
	if len(events) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no events found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "GET-IN\tDATE\tTITLE\tVENUE\tCATEGORY\t3D%")
	for _, event := range events {
		category := event.EventCategoryName
		if category == "" {
			category = event.CategoryType
		}
		fmt.Fprintf(tw, "%.2f\t%s\t%s\t%s\t%s\t%.2f\n",
			event.GetInPrice,
			event.Date,
			truncate(event.Title, 44),
			truncate(event.Venue, 28),
			truncate(category, 24),
			event.ThreeDayChangePct,
		)
	}
	return tw.Flush()
}
