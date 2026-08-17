// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
)

func newCurrentCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var radius int
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Fetch a compact OpenAQ latest-measurement snapshot near a point",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireChangedFlags(cmd, "lat", "lon"); err != nil {
				return err
			}
			query := map[string]any{"lat": lat, "lon": lon, "radius_m": radius}
			client := newOpenAQClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, openAQGuidance("current", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			result, err := currentForPoint(ctx, client, lat, lon, radius)
			if err != nil {
				return err
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude")
	cmd.Flags().IntVar(&radius, "radius", 25000, "OpenAQ search radius in meters, maximum 25000")
	return cmd
}

func newNearestCmd(flags *rootFlags) *cobra.Command {
	var lat, lon float64
	var radius, limit int
	cmd := &cobra.Command{
		Use:   "nearest",
		Short: "List nearby OpenAQ monitoring locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireChangedFlags(cmd, "lat", "lon"); err != nil {
				return err
			}
			query := map[string]any{"lat": lat, "lon": lon, "radius_m": radius, "limit": limit}
			client := newOpenAQClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, openAQGuidance("nearest", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			locations, err := client.locations(ctx, lat, lon, radius, limit)
			if err != nil {
				return err
			}
			result := NearestResult{
				Source:     "OpenAQ API v3",
				Configured: true,
				Query:      query,
				Locations:  locations,
				Caveats:    openAQCaveats(),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude")
	cmd.Flags().IntVar(&radius, "radius", 25000, "OpenAQ search radius in meters, maximum 25000")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum locations to return")
	return cmd
}

func newLocationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location <openaq-location-id>",
		Short: "Fetch an OpenAQ location and latest measurements",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			locationID := strings.TrimSpace(args[0])
			query := map[string]any{"location_id": locationID}
			client := newOpenAQClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, openAQGuidance("location", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			location, err := client.location(ctx, locationID)
			if err != nil {
				return err
			}
			measurements, freshness, _, err := client.latestByLocation(ctx, locationID, location.Sensors)
			if err != nil {
				return err
			}
			result := CurrentResult{
				Source:       "OpenAQ API v3",
				Configured:   true,
				Query:        query,
				Location:     location,
				Measurements: measurements,
				Freshness:    freshness,
				Caveats:      openAQCaveats(),
			}
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var sensorID string
	var days int
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Fetch recent OpenAQ measurements for one sensor",
		RunE: func(cmd *cobra.Command, args []string) error {
			sensorID = strings.TrimSpace(sensorID)
			if sensorID == "" {
				return usageErr("history requires --sensor")
			}
			query := map[string]any{"sensor_id": sensorID, "days": days}
			client := newOpenAQClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, openAQGuidance("history", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			measurements, freshness, err := client.measurementsBySensor(ctx, sensorID, days)
			if err != nil {
				return err
			}
			result := HistoryResult{
				Source:       "OpenAQ API v3",
				Configured:   true,
				Query:        query,
				Measurements: measurements,
				Freshness:    freshness,
				Caveats:      openAQCaveats(),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&sensorID, "sensor", "", "OpenAQ sensor ID")
	cmd.Flags().IntVar(&days, "days", 7, "Recent day window, capped at 31")
	return cmd
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var latA, lonA, latB, lonB float64
	var radius int
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare nearest OpenAQ snapshots for two points",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireChangedFlags(cmd, "lat-a", "lon-a", "lat-b", "lon-b"); err != nil {
				return err
			}
			query := map[string]any{
				"left":     map[string]any{"lat": latA, "lon": lonA},
				"right":    map[string]any{"lat": latB, "lon": lonB},
				"radius_m": radius,
			}
			client := newOpenAQClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, openAQGuidance("compare", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			left, err := currentForPoint(ctx, client, latA, lonA, radius)
			if err != nil {
				return err
			}
			right, err := currentForPoint(ctx, client, latB, lonB, radius)
			if err != nil {
				return err
			}
			result := CompareResult{
				Source:     "OpenAQ API v3",
				Configured: true,
				Query:      query,
				Left:       left,
				Right:      right,
				Caveats:    append(openAQCaveats(), "Comparison is a source-backed snapshot, not exposure or health advice."),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().Float64Var(&latA, "lat-a", 0, "Left latitude")
	cmd.Flags().Float64Var(&lonA, "lon-a", 0, "Left longitude")
	cmd.Flags().Float64Var(&latB, "lat-b", 0, "Right latitude")
	cmd.Flags().Float64Var(&lonB, "lon-b", 0, "Right longitude")
	cmd.Flags().IntVar(&radius, "radius", 25000, "OpenAQ search radius in meters, maximum 25000")
	return cmd
}

func newAirNowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "airnow",
		Short: "AirNow AQI observation recipes",
	}
	cmd.AddCommand(newAirNowCurrentCmd(flags))
	return cmd
}

func newAirNowCurrentCmd(flags *rootFlags) *cobra.Command {
	var zipCode string
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Fetch AirNow current AQI observations by ZIP code",
		RunE: func(cmd *cobra.Command, args []string) error {
			zipCode = strings.TrimSpace(zipCode)
			if zipCode == "" {
				return usageErr("airnow current requires --zip")
			}
			query := map[string]any{"zip": zipCode}
			client := newAirNowClient(flags.timeout)
			if !client.configured() {
				return printResult(cmd, flags, airNowGuidance("airnow current", query))
			}
			ctx, cancel := commandContext(cmd, flags)
			defer cancel()
			observations, err := client.currentByZip(ctx, zipCode)
			if err != nil {
				return err
			}
			result := AirNowResult{
				Source:       "AirNow API",
				Configured:   true,
				Query:        query,
				Observations: observations,
				Caveats:      airNowCaveats(),
			}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&zipCode, "zip", "", "ZIP code")
	return cmd
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "sources",
		Short: "Describe source coverage, auth, limits, and caveats",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := SourcesResult{
				Source: "air-quality-pp-cli",
				Sources: []SourceInfo{
					{
						Name:       "OpenAQ API v3",
						URL:        "https://docs.openaq.org/",
						Configured: strings.TrimSpace(env(openAQKeyEnv)) != "",
						Auth:       "X-API-Key header via AIR_QUALITY_OPENAQ_API_KEY",
						Limits:     []string{"60 requests per minute", "2,000 requests per hour"},
						Caveats:    openAQCaveats(),
					},
					{
						Name:       "AirNow API",
						URL:        "https://docs.airnowapi.org/webservices",
						Configured: strings.TrimSpace(env(airNowKeyEnv)) != "",
						Auth:       "API key via AIR_QUALITY_AIRNOW_API_KEY",
						Limits:     []string{"Rate limits vary by web service; cache daily and hourly observations."},
						Caveats:    airNowCaveats(),
					},
				},
				Caveats: []string{
					"No command gives medical advice or regulatory guidance.",
					"OpenAQ physical measurements and AirNow AQI categories are intentionally kept distinct.",
				},
			}
			return printResult(cmd, flags, result)
		},
	}
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report configured source families and command readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			openAQConfigured := strings.TrimSpace(env(openAQKeyEnv)) != ""
			airNowConfigured := strings.TrimSpace(env(airNowKeyEnv)) != ""
			enabled := []string{"sources", "doctor"}
			missing := []string{}
			if openAQConfigured {
				enabled = append(enabled, "current", "nearest", "location", "history", "compare")
			} else {
				missing = append(missing, "current", "nearest", "location", "history", "compare")
			}
			if airNowConfigured {
				enabled = append(enabled, "airnow current")
			} else {
				missing = append(missing, "airnow current")
			}
			result := DoctorResult{
				Source:           "air-quality-pp-cli",
				OpenAQConfigured: openAQConfigured,
				AirNowConfigured: airNowConfigured,
				EnabledCommands:  enabled,
				MissingCommands:  missing,
				Caveats:          []string{"Set only local environment variables; no API keys are committed or stored by this CLI."},
			}
			return printResult(cmd, flags, result)
		},
	}
}

func currentForPoint(ctx context.Context, client openAQClient, lat, lon float64, radius int) (CurrentResult, error) {
	locations, err := client.locations(ctx, lat, lon, radius, 1)
	if err != nil {
		return CurrentResult{}, err
	}
	query := map[string]any{"lat": lat, "lon": lon, "radius_m": radius}
	result := CurrentResult{
		Source:     "OpenAQ API v3",
		Configured: true,
		Query:      query,
		Caveats:    openAQCaveats(),
	}
	if len(locations) == 0 {
		result.Caveats = append(result.Caveats, "No OpenAQ location was returned for this point and radius.")
		return result, nil
	}
	result.Location = locations[0]
	if result.Location.ID == "" {
		result.Caveats = append(result.Caveats, "Nearest location did not include an id; latest measurements were not fetched.")
		return result, nil
	}
	measurements, freshness, _, err := client.latestByLocation(ctx, result.Location.ID, result.Location.Sensors)
	if err != nil {
		return CurrentResult{}, err
	}
	result.Measurements = measurements
	result.Freshness = freshness
	return result, nil
}

func requireChangedFlags(cmd *cobra.Command, names ...string) error {
	for _, name := range names {
		if !cmd.Flags().Changed(name) {
			return usageErr("missing --%s", name)
		}
	}
	return nil
}

func openAQCaveats() []string {
	return []string{
		"OpenAQ shares physical pollutant measurements in source units, not AQI categories.",
		"OpenAQ v1 and v2 endpoints are retired; this CLI uses API v3 only.",
		"Nearby measurements are snapshots from reported public monitoring data and are not medical advice.",
	}
}

func airNowCaveats() []string {
	return []string{
		"AirNow observations are preliminary and subject to change.",
		"AirNow data are intended for AQI reporting and forecasting, not regulatory decisions.",
		"For large time ranges or broad extracts, AirNow recommends file products over web services.",
	}
}
