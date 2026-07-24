// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type amcShowtime struct {
	ID          string  `json:"id,omitempty"`
	Movie       string  `json:"movie,omitempty"`
	Theatre     string  `json:"theatre,omitempty"`
	Start       string  `json:"start"`
	Format      string  `json:"format,omitempty"`
	Distance    float64 `json:"distance_miles,omitempty"`
	PurchaseURL string  `json:"purchase_url,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newMoviePlanCmd(flags))
	})
}

func newMoviePlanCmd(flags *rootFlags) *cobra.Command {
	var theatre, date, after, format string
	var latitude, longitude float64
	var limit int
	cmd := &cobra.Command{
		Use:   "movie-plan [movie]",
		Short: "Find and rank AMC showtimes without purchasing tickets",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			hasTheatre := strings.TrimSpace(theatre) != ""
			hasCoordinates := cmd.Flags().Changed("latitude") || cmd.Flags().Changed("longitude")
			if hasTheatre == hasCoordinates {
				return usageErr(fmt.Errorf("set either --theatre or both --latitude and --longitude"))
			}
			if hasCoordinates && (!cmd.Flags().Changed("latitude") || !cmd.Flags().Changed("longitude")) {
				return usageErr(fmt.Errorf("--latitude and --longitude must be provided together"))
			}
			if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
				return usageErr(fmt.Errorf("coordinates are outside valid latitude/longitude ranges"))
			}
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("--date must use YYYY-MM-DD"))
			}
			afterMinutes, err := parseAMCTime(after)
			if err != nil {
				return usageErr(err)
			}
			movie := ""
			if len(args) == 1 {
				movie = strings.TrimSpace(args[0])
			}
			path := ""
			if hasTheatre {
				if _, err := strconv.Atoi(theatre); err != nil {
					return usageErr(fmt.Errorf("--theatre must be a numeric AMC theatre number"))
				}
				path = "/v2/theatres/" + theatre + "/showtimes/" + date
			} else {
				path = fmt.Sprintf("/v2/showtimes/views/current-location/%s/%s/%s",
					date, strconv.FormatFloat(latitude, 'f', -1, 64), strconv.FormatFloat(longitude, 'f', -1, 64))
			}
			params := map[string]string{"page-size": "100"}
			if flags.dryRun || os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": flags.dryRun, "method": "GET", "path": path,
					"params": params, "movie": movie, "after": after, "format": format,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetWithHeadersNoCache(cmd.Context(), path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows, err := normalizeAMCShowtimes(raw)
			if err != nil {
				return err
			}
			rows = filterAMCShowtimes(rows, movie, afterMinutes, format)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"date": date, "movie_query": movie, "count": len(rows), "showtimes": rows,
				"purchase": "Read-only planning only. Ticket selection, seats, and payment are not performed.",
			}, flags)
		},
	}
	cmd.Flags().StringVar(&theatre, "theatre", "", "AMC theatre number")
	cmd.Flags().Float64Var(&latitude, "latitude", 0, "Latitude for current-location search")
	cmd.Flags().Float64Var(&longitude, "longitude", 0, "Longitude for current-location search")
	cmd.Flags().StringVar(&date, "date", "", "Showtime date (YYYY-MM-DD; default today)")
	cmd.Flags().StringVar(&after, "after", "", "Only include showtimes at or after HH:MM")
	cmd.Flags().StringVar(&format, "format", "", "Only include formats containing this value (for example IMAX)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum ranked showtimes")
	return cmd
}

func normalizeAMCShowtimes(raw json.RawMessage) ([]amcShowtime, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode AMC showtime response: %w", err)
	}
	var rows []amcShowtime
	walkAMCObjects(value, func(obj map[string]any) {
		start := firstAMCString(obj, "showDateTimeLocal", "startTime", "startDateTime", "showtime", "dateTime")
		if start == "" {
			return
		}
		row := amcShowtime{
			ID:          firstAMCScalar(obj, "id", "showtimeId", "showtimeNumber"),
			Movie:       firstAMCString(obj, "movieName", "filmName", "title", "name"),
			Theatre:     firstAMCString(obj, "theatreName", "cinemaName"),
			Start:       start,
			Format:      firstAMCString(obj, "presentationName", "format", "experience"),
			Distance:    firstAMCFloat(obj, "distance", "distanceMiles"),
			PurchaseURL: firstAMCString(obj, "purchaseUrl", "ticketUrl", "url"),
		}
		if nested, ok := obj["movie"].(map[string]any); ok {
			if name := firstAMCString(nested, "name", "title", "movieName"); name != "" {
				row.Movie = name
			}
		}
		if nested, ok := obj["theatre"].(map[string]any); ok {
			if name := firstAMCString(nested, "name", "theatreName"); name != "" {
				row.Theatre = name
			}
		}
		if nested, ok := obj["presentation"].(map[string]any); ok {
			if name := firstAMCString(nested, "name", "format", "experience"); name != "" {
				row.Format = name
			}
		}
		rows = append(rows, row)
	})
	sort.SliceStable(rows, func(i, j int) bool {
		di, dj := rows[i].Distance, rows[j].Distance
		if di == 0 {
			di = math.MaxFloat64
		}
		if dj == 0 {
			dj = math.MaxFloat64
		}
		if di != dj {
			return di < dj
		}
		return rows[i].Start < rows[j].Start
	})
	return rows, nil
}

func walkAMCObjects(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			walkAMCObjects(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkAMCObjects(child, visit)
		}
	}
}

func firstAMCString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstAMCScalar(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := obj[key].(type) {
		case string:
			return value
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func firstAMCFloat(obj map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := obj[key].(type) {
		case float64:
			return value
		case string:
			parsed, _ := strconv.ParseFloat(value, 64)
			return parsed
		}
	}
	return 0
}

func parseAMCTime(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("--after must use HH:MM")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func filterAMCShowtimes(rows []amcShowtime, movie string, afterMinutes int, format string) []amcShowtime {
	movie = strings.ToLower(strings.TrimSpace(movie))
	format = strings.ToLower(strings.TrimSpace(format))
	filtered := make([]amcShowtime, 0, len(rows))
	for _, row := range rows {
		if movie != "" && !strings.Contains(strings.ToLower(row.Movie), movie) {
			continue
		}
		if format != "" && !strings.Contains(strings.ToLower(row.Format), format) {
			continue
		}
		clock := row.Start
		if len(clock) >= 16 && strings.Contains(clock, "T") {
			clock = clock[11:16]
		} else if len(clock) >= 5 {
			clock = clock[len(clock)-5:]
		}
		minutes, err := parseAMCTime(clock)
		if err == nil && minutes < afterMinutes {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}
