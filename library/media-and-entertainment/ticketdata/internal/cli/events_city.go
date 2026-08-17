// pp:data-source live
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/cliutil"
	"github.com/spf13/cobra"
)

type cityEventsEnvelope struct {
	City          string             `json:"city"`
	VenuesScanned int                `json:"venues_scanned"`
	Events        []searchEvent      `json:"events"`
	FetchFailures []cityFetchFailure `json:"fetch_failures"`
}

type cityFetchFailure struct {
	Venue string `json:"venue"`
	Error string `json:"error"`
}

func newEventsCityCmd(flags *rootFlags) *cobra.Command {
	var minGetIn float64
	var gamesOnly bool
	var category string
	var sortBy string
	var limit int
	var upcoming bool
	var all bool

	cmd := &cobra.Command{
		Use:         "city <city>",
		Short:       "Scan a city's event search results and ranked venue events",
		Example:     "  ticketdata-pp-cli events city seattle --min-get-in 200 --games-only --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "seattle"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			city := normalizeCity(strings.Join(args, " "))
			if dryRunOK(flags) {
				if city == "" {
					city = "<city>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would scan mapped venues for the city\n")
				return nil
			}
			if city == "" {
				return usageErr(fmt.Errorf("city is required"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			if err := validateSearchSort(sortBy); err != nil {
				return usageErr(err)
			}
			tab := "upcoming"
			if all {
				tab = "all"
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			events, venuesScanned, failures, err := fetchCitySearchEvents(cmd.Context(), c, city, tab, limit, flags)
			if err != nil {
				return err
			}
			events = filterSearchEvents(events, searchFilters{
				MinGetIn:  minGetIn,
				GamesOnly: gamesOnly,
				Category:  category,
			})
			sortSearchEvents(events, sortBy)
			events = limitSearchEvents(events, limit)
			if events == nil {
				events = make([]searchEvent, 0)
			}
			if failures == nil {
				failures = make([]cityFetchFailure, 0)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d venue fetches failed: %s\n", len(failures), strings.Join(cityFailureVenues(failures), ", "))
			}
			view := cityEventsEnvelope{
				City:          city,
				VenuesScanned: venuesScanned,
				Events:        events,
				FetchFailures: failures,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printSearchEventsTable(cmd, events)
		},
	}
	cmd.Flags().Float64Var(&minGetIn, "min-get-in", 0, "Minimum get-in price")
	cmd.Flags().BoolVar(&gamesOnly, "games-only", false, "Keep sports events and exclude tour rows")
	cmd.Flags().StringVar(&category, "category", "", "Keep events whose category name contains this value")
	cmd.Flags().StringVar(&sortBy, "sort", "get_in", "Sort by get_in, date, or movers")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum events returned after ranking")
	cmd.Flags().BoolVar(&upcoming, "upcoming", true, "Use upcoming events")
	cmd.Flags().BoolVar(&all, "all", false, "Use all events")
	return cmd
}

type searchClient interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

func fetchCitySearchEvents(ctx context.Context, c searchClient, city, tab string, limit int, flags *rootFlags) ([]searchEvent, int, []cityFetchFailure, error) {
	// The API's free-text fuzzy param (f=) matches the city word inside team
	// names ("Seattle Storm at Los Angeles"), not the event's city, so it is
	// unreliable for a city scan. Resolve the city to its venues and fan out.
	venues := resolveCityVenues(city)
	if len(venues) == 0 {
		return nil, 0, nil, usageErr(fmt.Errorf("no venue mapping for city %q; pass venues directly with `ticketdata-pp-cli events list --venue <slug>`", city))
	}
	if cliutil.IsDogfoodEnv() && len(venues) > 3 {
		venues = venues[:3]
	}
	limiterRate := 2.0
	if flags != nil && flags.rateLimit > 0 {
		limiterRate = flags.rateLimit
	}
	limiter := cliutil.NewAdaptiveLimiter(limiterRate)
	results, errs := cliutil.FanoutRun(ctx, venues,
		func(venue string) string { return venue },
		func(ctx context.Context, venue string) ([]searchEvent, error) {
			limiter.Wait()
			events, _, err := fetchSearchEventsWithClient(ctx, c, map[string]string{"venue_slug": venue, "limit": fmt.Sprintf("%d", limit)}, tab)
			return events, err
		},
	)
	events, failures := aggregateCitySearchResults(results, errs)
	return events, len(venues), failures, nil
}

func fetchSearchEventsWithClient(ctx context.Context, c searchClient, params map[string]string, tab string) ([]searchEvent, searchMeta, error) {
	raw, err := c.Get(ctx, "/search", params)
	if err != nil {
		return nil, searchMeta{}, err
	}
	return parseSearchEvents(raw, tab)
}

func aggregateCitySearchResults(results []cliutil.FanoutResult[[]searchEvent], errs []cliutil.FanoutError) ([]searchEvent, []cityFetchFailure) {
	events := make([]searchEvent, 0)
	seen := make(map[string]bool)
	for _, result := range results {
		for _, event := range result.Value {
			key := event.ID.String()
			if key == "" {
				key = result.Source + "|" + event.Title + "|" + event.Date
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			events = append(events, event)
		}
	}
	failures := make([]cityFetchFailure, 0, len(errs))
	for _, err := range errs {
		msg := ""
		if err.Err != nil {
			msg = err.Err.Error()
		}
		failures = append(failures, cityFetchFailure{Venue: err.Source, Error: msg})
	}
	return events, failures
}

func cityFailureVenues(failures []cityFetchFailure) []string {
	venues := make([]string, 0, len(failures))
	for _, failure := range failures {
		venues = append(venues, failure.Venue)
	}
	return venues
}
