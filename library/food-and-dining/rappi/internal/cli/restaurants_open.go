// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchRestaurantListPage / fetchRestaurantDetail -> rappi.Client.FetchHTML.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newRestaurantsOpenCmd(flags *rootFlags) *cobra.Command {
	var (
		city        string
		category    string
		atRaw       string
		day         string
		fetchDetail bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Restaurants open at a specific time (parsed from schema.org openingHours)",
		Long: `Filter restaurants in a (city, category) by whether they are open at
the requested local time, parsed from each restaurant's
schema.org openingHoursSpecification. Set --at to "HH:MM" and
--day to a weekday (or omit for today). Without --fetch-detail
the openingHours data has to be available from a prior detail
fetch; with --fetch-detail (slow), the command fetches detail
pages live for each candidate restaurant.`,
		Example:     "  rappi-pp-cli restaurants open --city ciudad-de-mexico --category sushi --at 23:30 --day sunday --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if city == "" && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseHM(atRaw)
			if err != nil {
				return err
			}
			weekday := strings.ToLower(strings.TrimSpace(day))
			if weekday == "" {
				weekday = strings.ToLower(time.Now().Weekday().String())
			}
			// PATCH: Reuse the configured Rappi client across list and detail fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			rows, err := fetchRestaurantListPage(cmd.Context(), rappiClient, city, category)
			if err != nil {
				return err
			}
			type result struct {
				rappi.Restaurant
				MatchedHours string `json:"matched_hours,omitempty"`
			}
			out := []result{}
			for _, r := range rows {
				if r.ID == "" {
					continue
				}
				if !fetchDetail {
					// without --fetch-detail we cannot evaluate hours
					continue
				}
				det, err := fetchRestaurantDetail(cmd.Context(), rappiClient, r.ID+"-"+slugFromURL(r.URL), city, category)
				if err != nil {
					stderrf("warning: detail fetch failed for %s: %v\n", r.URL, err)
					continue
				}
				match, hrs := isOpenAt(det.OpeningHours, weekday, t)
				if match {
					out = append(out, result{Restaurant: *det, MatchedHours: hrs})
					if limit > 0 && len(out) >= limit {
						break
					}
				}
			}
			if !fetchDetail {
				stderrf("note: --fetch-detail is required to evaluate openingHours; this run filtered nothing without it.\n")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Open at %s (%s) in %s/%s:\n", atRaw, weekday, city, category)
			for _, r := range out {
				fmt.Fprintf(w, "  %s  %s  %.1f★  %s\n", r.Name, r.MatchedHours, r.RatingValue, r.AddressStreet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&city, "city", "", "City slug (required)")
	cmd.Flags().StringVar(&category, "category", "", "Cuisine category slug")
	cmd.Flags().StringVar(&atRaw, "at", "20:00", "Local time HH:MM (24-hour)")
	cmd.Flags().StringVar(&day, "day", "", "Weekday name (monday, tuesday, ...); defaults to today")
	cmd.Flags().BoolVar(&fetchDetail, "fetch-detail", false, "Fetch each restaurant's detail page to read openingHours (slow but accurate)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max matches to return")
	return cmd
}

func parseHM(s string) (time.Time, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("--at must be HH:MM (24-hour): %w", err)
	}
	return t, nil
}

// PATCH: Compare opening windows by minute-of-day so midnight crossings work.
func isOpenAt(hours []rappi.OpeningHoursSpec, weekday string, t time.Time) (bool, string) {
	wd := strings.ToLower(weekday)
	previousWd := previousWeekday(wd)
	targetMinute := minuteOfDay(t)
	for _, h := range hours {
		hwd := strings.ToLower(h.DayOfWeek)
		opensMinute, ok := parseOpeningMinute(h.Opens)
		if !ok {
			continue
		}
		closesMinute, ok := parseOpeningMinute(h.Closes)
		if !ok {
			continue
		}
		if openingWindowMatchesWeekday(hwd, wd, previousWd, targetMinute, opensMinute, closesMinute) {
			return true, fmt.Sprintf("%s %s-%s", h.DayOfWeek, h.Opens, h.Closes)
		}
	}
	return false, ""
}

func parseOpeningMinute(s string) (int, bool) {
	for _, layout := range []string{"15:04:05", "15:04"} {
		t, err := time.Parse(layout, strings.TrimSpace(s))
		if err == nil {
			return minuteOfDay(t), true
		}
	}
	return 0, false
}

func minuteOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

func openingWindowMatchesWeekday(hwd, wd, previousWd string, target, opens, closes int) bool {
	if opens == closes {
		return hwd == wd
	}
	if opens < closes {
		return hwd == wd && target >= opens && target < closes
	}
	return (hwd == wd && target >= opens) || (hwd == previousWd && target < closes)
}

func previousWeekday(weekday string) string {
	switch strings.ToLower(weekday) {
	case "monday":
		return "sunday"
	case "tuesday":
		return "monday"
	case "wednesday":
		return "tuesday"
	case "thursday":
		return "wednesday"
	case "friday":
		return "thursday"
	case "saturday":
		return "friday"
	case "sunday":
		return "saturday"
	default:
		return ""
	}
}

func slugFromURL(url string) string {
	// /restaurantes/10000295-el-farolito -> "el-farolito"
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if i := strings.Index(last, "-"); i >= 0 {
		return last[i+1:]
	}
	return last
}
