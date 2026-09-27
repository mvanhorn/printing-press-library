package cli

// pp:data-source auto

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/planning"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
	"github.com/spf13/cobra"
)

type tenkiFetchFailure struct {
	Place string `json:"place"`
	Stage string `json:"stage"`
	Error string `json:"error"`
}
type tenkiForecastFetch struct {
	result tenki.ForecastResult
	err    error
}

// The generated root uses this constructor name for its declared novel feature.
// Keep the adapter in the preserved implementation file so regeneration and
// the real lifecycle both construct the implemented command.
func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	return newTenkiCompare(flags, func(config tenki.Config) tenkiSource { return tenki.NewClient(config) })
}

func newTenkiCompare(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var places []string
	var from, hours, season, condition, sortBy string
	var days, year, limit int
	var maxPOP, maxRain, minTemp, maxTemp, maxWind float64
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "compare", Short: "Compare explicit weather thresholds across 1–5 places and 1–14 source dates", Long: "Build a bounded evidence matrix, not a universal score. Supply at least one weather criterion. --sort selects a supplied criterion (default: first supplied in max-pop, max-rain, min-temp, max-temp, max-wind order). Whole-day mode uses source daily fields and five source wind instants; --hours uses only true hourly coverage. Rain threshold units are mm per source daily period or mm/h per hourly interval. A mountain destination cannot be ranked for summit suitability from foothill evidence. --season pairs a selected seasonal spot's report with its source-linked municipality; --season-condition optionally supplies an exact raw condition preference. Current reports cannot certify future seasonal conditions.", Example: strings.Trim(`
  tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --days 3 --max-pop 30 --max-temp 30 --sort max-pop --agent
  tenki-pp-cli compare --place https://tenki.jp/forecast/3/16/4410/13101/ --place https://tenki.jp/forecast/6/29/6110/26103/ --days 2 --hours 09:00-17:00 --max-rain 0 --max-wind 5 --sort max-rain
  tenki-pp-cli compare --place https://tenki.jp/kouyou/3/13/30139.html --days 2 --season kouyou --max-pop 30 --sort max-pop --agent
`, "\n"), Annotations: tenkiAnnotations("--place=" + tenkiTokyo + ";--days=1;--max-pop=100;--sort=max-pop")}
	cmd.Flags().StringArrayVar(&places, "place", nil, "Canonical tenki.jp URL; repeat for up to five selected destinations")
	cmd.Flags().StringVar(&from, "from", "", "First JST date YYYY-MM-DD (default: today)")
	cmd.Flags().IntVar(&days, "days", 3, "Requested dates (1–14)")
	cmd.Flags().StringVar(&hours, "hours", "", "Optional hourly activity window HH:00-HH:00; no daily substitution")
	cmd.Flags().StringVar(&season, "season", "", "Optional seasonal evidence pairing: sakura or kouyou")
	cmd.Flags().StringVar(&condition, "season-condition", "", "Optional exact source condition preference; requires --season")
	cmd.Flags().IntVar(&year, "year", 0, "Requested seasonal year (default: current JST year; requires --season)")
	cmd.Flags().Float64Var(&maxPOP, "max-pop", 0, "Maximum source interval precipitation probability (0–100 percent)")
	cmd.Flags().Float64Var(&maxRain, "max-rain", 0, "Maximum rain: daily source amount mm, or hourly source rate mm/h with --hours")
	cmd.Flags().Float64Var(&minTemp, "min-temp", 0, "Minimum source morning low (daily) or instantaneous temperature (hourly), °C")
	cmd.Flags().Float64Var(&maxTemp, "max-temp", 0, "Maximum source daytime high (daily) or instantaneous temperature (hourly), °C")
	cmd.Flags().Float64Var(&maxWind, "max-wind", 0, "Maximum wind speed at compatible source instants (m/s)")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Supplied criterion: max-pop, max-rain, min-temp, max-temp or max-wind")
	cmd.Flags().IntVar(&limit, "limit", 70, "Maximum returned cells (1–70)")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if from == "" {
			from = tenkiToday()
		}
		if year == 0 && !cmd.Flags().Changed("year") {
			year = time.Now().In(time.FixedZone("JST", 9*60*60)).Year()
		}
		if cmd.Flags().Changed("year") && season == "" {
			return usageErr(fmt.Errorf("--year requires --season"))
		}
		options := planning.Options{From: from, Days: days, Sort: sortBy, Limit: limit, Season: season, Year: year, SeasonCondition: condition}
		if hours != "" {
			window, err := parseTenkiHours(hours)
			if err != nil {
				return err
			}
			options.Hours = &planning.Window{Start: window.Start, End: window.End}
		}
		if cmd.Flags().Changed("max-pop") {
			options.Criteria.MaxPOP = &maxPOP
		}
		if cmd.Flags().Changed("max-rain") {
			options.Criteria.MaxRain = &maxRain
		}
		if cmd.Flags().Changed("min-temp") {
			options.Criteria.MinTemp = &minTemp
		}
		if cmd.Flags().Changed("max-temp") {
			options.Criteria.MaxTemp = &maxTemp
		}
		if cmd.Flags().Changed("max-wind") {
			options.Criteria.MaxWind = &maxWind
		}
		if err := planning.Validate(options, len(places)); err != nil {
			return usageErr(err)
		}
		for _, place := range places {
			if err := validateTenkiPlace(place); err != nil {
				return err
			}
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		inputs, failures, sources, err := fetchTenkiComparison(ctx, cmd, client, places, options)
		if err != nil {
			return err
		}
		result, err := planning.Evaluate(inputs, options)
		if err != nil {
			return usageErr(err)
		}
		failedPlaces := map[string]bool{}
		for _, failure := range failures {
			failedPlaces[failure.Place] = true
		}
		weatherAvailable := 0
		for _, input := range inputs {
			if input.WeatherError == "" && input.Weather.Status == "ok" {
				weatherAvailable++
			}
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d places had fetch failures; forecast evidence available for %d of %d places; missing cells are preserved\n", len(failedPlaces), len(inputs), weatherAvailable, len(inputs))
		}
		meta := map[string]any{"schema_version": "1", "provider": "tenki.jp", "source": comparisonTransport(sources), "data_origin": "computed", "timezone": tenki.Timezone, "metrics": client.Metrics()}
		view := map[string]any{"comparison": result, "fetch_failures": failures, "partial": len(failures) > 0, "forecast_places_available": weatherAvailable, "selected_places": len(inputs)}
		return printTenki(cmd, flags, map[string]any{"meta": meta, "results": view}, sources...)
	}
	return cmd
}

func fetchTenkiComparison(ctx context.Context, cmd *cobra.Command, client tenkiSource, targets []string, options planning.Options) ([]planning.Input, []tenkiFetchFailure, []tenki.Source, error) {
	inputs := []planning.Input{}
	failures := []tenkiFetchFailure{}
	sources := []tenki.Source{}
	forecasts := map[string]tenkiForecastFetch{}
	seen := map[string]bool{}
	for _, target := range targets {
		target = strings.TrimSpace(target)
		direct := strings.HasPrefix(target, "https://tenki.jp/forecast/")
		reference := ""
		identityKey := target
		if direct {
			reference = strings.TrimSuffix(strings.TrimSuffix(target, "1hour.html"), "10days.html")
			identityKey = reference
		}
		if seen[identityKey] {
			continue
		}
		seen[identityKey] = true
		input := planning.Input{Place: tenki.Place{URL: target}}
		if direct {
			input.Place = tenki.Place{URL: reference, Kind: "municipality", Scope: "municipal", ForecastReferenceURL: reference}
		} else {
			place, err := client.Resolve(ctx, target)
			if err != nil {
				if isTenkiRateLimit(err) {
					return nil, nil, nil, tenkiProductError(err)
				}
				input.WeatherError = err.Error()
				failures = append(failures, tenkiFetchFailure{Place: target, Stage: "resolve", Error: err.Error()})
				inputs = append(inputs, input)
				continue
			}
			input.Place = place.Place
			sources = append(sources, place.Source)
			tenkiWarnings(cmd, place.Warnings)
			reference = input.Place.ForecastReferenceURL
		}
		if reference == "" && input.Place.Kind == "municipality" {
			reference = input.Place.URL
			input.Place.ForecastReferenceURL = reference
		}
		if reference == "" {
			input.WeatherError = "Source does not identify a forecast reference municipality."
			failures = append(failures, tenkiFetchFailure{Place: target, Stage: "forecast_reference", Error: input.WeatherError})
		} else {
			fetched, ok := forecasts[reference]
			if !ok {
				if options.Hours != nil {
					fetched.result, fetched.err = client.Hourly(ctx, reference)
				} else {
					fetched.result, fetched.err = client.Daily(ctx, reference)
				}
				forecasts[reference] = fetched
			}
			if fetched.err != nil {
				if isTenkiRateLimit(fetched.err) {
					return nil, nil, nil, tenkiProductError(fetched.err)
				}
				input.WeatherError = fetched.err.Error()
				failures = append(failures, tenkiFetchFailure{Place: target, Stage: "forecast", Error: fetched.err.Error()})
			} else {
				input.Weather = fetched.result
				if direct && fetched.result.Place.URL != "" {
					input.Place = fetched.result.Place
				}
				sources = append(sources, fetched.result.Source)
				if !ok {
					tenkiWarnings(cmd, fetched.result.Warnings)
				}
			}
		}
		if options.Season != "" {
			if !strings.Contains(input.Place.URL, "/"+options.Season+"/") {
				input.SeasonError = "Selected URL has no spot-level " + options.Season + " report; select a matching seasonal spot URL."
				failures = append(failures, tenkiFetchFailure{Place: target, Stage: "seasonal_reference", Error: input.SeasonError})
			} else {
				season, err := client.Seasonal(ctx, options.Season, input.Place.URL, options.Year)
				if err != nil {
					if isTenkiRateLimit(err) {
						return nil, nil, nil, tenkiProductError(err)
					}
					input.SeasonError = err.Error()
					failures = append(failures, tenkiFetchFailure{Place: target, Stage: "seasonal", Error: err.Error()})
				} else {
					input.Season = &season
					sources = append(sources, season.Source)
					tenkiWarnings(cmd, season.Warnings)
				}
			}
		}
		inputs = append(inputs, input)
	}
	return inputs, failures, sources, nil
}

func isTenkiRateLimit(err error) bool {
	var rateErr *cliutil.RateLimitError
	return errors.As(err, &rateErr)
}
func comparisonTransport(sources []tenki.Source) string {
	live, local := false, false
	for _, source := range sources {
		if source.URL == "" {
			continue
		}
		if source.FromCache {
			local = true
		} else {
			live = true
		}
	}
	if live && local {
		return "mixed"
	}
	if local {
		return "local"
	}
	if live {
		return "live"
	}
	return "unavailable"
}
