// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type movieGluFilm struct {
	FilmID   int    `json:"film_id"`
	FilmName string `json:"film_name"`
}

type movieGluTime struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type movieGluCinema struct {
	CinemaID   int     `json:"cinema_id"`
	CinemaName string  `json:"cinema_name"`
	Distance   float64 `json:"distance"`
	Showings   map[string]struct {
		Times []movieGluTime `json:"times"`
	} `json:"showings"`
}

type movieNightOption struct {
	CinemaID   int     `json:"cinema_id"`
	CinemaName string  `json:"cinema_name"`
	Distance   float64 `json:"distance_miles"`
	Format     string  `json:"format"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newMovieNightCmd(flags))
	})
}

func newMovieNightCmd(flags *rootFlags) *cobra.Command {
	var date, after string
	var limit int
	var bookingLink, launch bool
	cmd := &cobra.Command{
		Use:   "movie-night <film name>",
		Short: "Find and rank nearby showtimes, with an optional cinema booking link",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if launch && !bookingLink {
				return usageErr(fmt.Errorf("--launch requires --booking-link"))
			}
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("--date must use YYYY-MM-DD"))
			}
			afterMinutes, err := parseClock(after)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun || os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": flags.dryRun,
					"workflow": []string{
						"GET /filmsNowShowing/?n=25",
						"GET /filmShowTimes/?film_id=<resolved>&date=" + date + "&n=25",
					},
					"booking_link_requested": bookingLink,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rawFilms, filmsProv, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "films-now-showing", true, "/filmsNowShowing/", map[string]string{"n": "25"}, nil, "films", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var films []movieGluFilm
			if err := json.Unmarshal(rawFilms, &films); err != nil {
				return fmt.Errorf("decode filmsNowShowing response: %w", err)
			}
			film, err := chooseFilm(films, args[0])
			if err != nil {
				return err
			}
			rawShowtimes, showtimesProv, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "film-show-times", true, "/filmShowTimes/", map[string]string{
				"film_id": strconv.Itoa(film.FilmID),
				"date":    date,
				"n":       "25",
			}, nil, "cinemas", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var cinemas []movieGluCinema
			if err := json.Unmarshal(rawShowtimes, &cinemas); err != nil {
				return fmt.Errorf("decode filmShowTimes response: %w", err)
			}
			options := flattenMovieNightOptions(cinemas, afterMinutes)
			if limit > 0 && len(options) > limit {
				options = options[:limit]
			}
			result := map[string]any{
				"film":     film,
				"date":     date,
				"after":    after,
				"options":  options,
				"count":    len(options),
				"source":   showtimesProv.Source,
				"purchase": "MovieGlu supplies cinema booking links only; seat selection and payment happen on the cinema website.",
			}
			if filmsProv.Source != showtimesProv.Source {
				result["sources"] = map[string]string{"films": filmsProv.Source, "showtimes": showtimesProv.Source}
			}
			if bookingLink {
				if len(options) == 0 {
					return fmt.Errorf("no matching showtimes available for booking")
				}
				if flags.dataSource == "local" {
					return fmt.Errorf("--booking-link requires --data-source live or auto because purchase handoff URLs are requested on demand")
				}
				selected := options[0]
				result["selected"] = selected
				if showtimesProv.Source == "local" {
					result["booking_link_unavailable"] = "Booking URLs are not stored locally; rerun with live MovieGlu credentials and connectivity."
					if launch {
						result["launch_unavailable"] = true
					}
				} else {
					rawBooking, err := c.GetWithHeadersNoCache(cmd.Context(), "/purchaseConfirmation/", map[string]string{
						"cinema_id": strconv.Itoa(selected.CinemaID),
						"film_id":   strconv.Itoa(film.FilmID),
						"date":      date,
						"time":      selected.StartTime,
					}, nil)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					var booking struct {
						URL string `json:"url"`
					}
					if err := json.Unmarshal(rawBooking, &booking); err != nil {
						return fmt.Errorf("decode purchaseConfirmation response: %w", err)
					}
					if !strings.HasPrefix(booking.URL, "https://") {
						return fmt.Errorf("MovieGlu returned an unsafe booking URL")
					}
					result["booking_url"] = booking.URL
					if launch {
						if err := openMovieGluURL(booking.URL); err != nil {
							return fmt.Errorf("launch booking URL: %w", err)
						}
						result["launched"] = true
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Showtime date (YYYY-MM-DD; default today)")
	cmd.Flags().StringVar(&after, "after", "", "Only include showtimes at or after HH:MM")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum ranked options")
	cmd.Flags().BoolVar(&bookingLink, "booking-link", false, "Fetch the cinema booking URL for the first ranked option")
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the selected HTTPS booking URL (requires --booking-link)")
	return cmd
}

func openMovieGluURL(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Start()
}

func chooseFilm(films []movieGluFilm, query string) (movieGluFilm, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, film := range films {
		if strings.EqualFold(film.FilmName, needle) {
			return film, nil
		}
	}
	for _, film := range films {
		if strings.Contains(strings.ToLower(film.FilmName), needle) {
			return film, nil
		}
	}
	return movieGluFilm{}, fmt.Errorf("film %q was not found among films now showing", query)
}

func parseClock(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("--after must use HH:MM")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func flattenMovieNightOptions(cinemas []movieGluCinema, afterMinutes int) []movieNightOption {
	var options []movieNightOption
	for _, cinema := range cinemas {
		for format, showing := range cinema.Showings {
			for _, slot := range showing.Times {
				minutes, err := parseClock(slot.StartTime)
				if err != nil || minutes < afterMinutes {
					continue
				}
				options = append(options, movieNightOption{
					CinemaID: cinema.CinemaID, CinemaName: cinema.CinemaName,
					Distance: cinema.Distance, Format: format,
					StartTime: slot.StartTime, EndTime: slot.EndTime,
				})
			}
		}
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Distance != options[j].Distance {
			return options[i].Distance < options[j].Distance
		}
		return options[i].StartTime < options[j].StartTime
	})
	return options
}
