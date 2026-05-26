// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/strava/internal/cliutil"
)

type driftRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	StartDate   string  `json:"start_date"`
	Type        string  `json:"type"`
	DurationMin float64 `json:"duration_min"`
	HR1         float64 `json:"hr_first_half"`
	HR2         float64 `json:"hr_second_half"`
	Pace1       float64 `json:"pace_first_half_mps"`
	Pace2       float64 `json:"pace_second_half_mps"`
	DriftPct    float64 `json:"drift_pct"`
	Flagged     bool    `json:"flagged"`
}

func newActivitiesDriftCmd(flags *rootFlags) *cobra.Command {
	var minDuration string
	var threshold float64
	var since string
	var activityType string

	cmd := &cobra.Command{
		Use:         "drift [id]",
		Short:       "Measure aerobic decoupling (HR drift vs pace) in activities",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Computes aerobic decoupling: the percentage difference between HR:pace ratio
in the second half vs first half of an activity. Values above 5% indicate the
athlete exceeded their aerobic threshold during the effort.

Fetches streams live from the Strava API. For a single activity, provide its ID.
Without an ID, analyzes recent activities above --min-duration.`,
		Example: strings.Trim(`
  strava-pp-cli activities drift 12345678
  strava-pp-cli activities drift --min-duration 60m --since 2025-01-01 --threshold 8
  strava-pp-cli activities drift --type Run --agent --select id,name,drift_pct,flagged`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				sample := []driftRow{
					{ID: "12345678", Name: "Long Sunday Run", StartDate: "2026-05-18T07:00:00Z",
						Type: "Run", DurationMin: 90, HR1: 138.5, HR2: 144.2,
						Pace1: 3.1, Pace2: 3.0, DriftPct: 6.2, Flagged: true},
				}
				return printJSONFiltered(cmd.OutOrStdout(), sample, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Parse min duration
			minSecs := 45 * 60 // default 45 minutes
			if minDuration != "" {
				minSecs, err = parseDuration(minDuration)
				if err != nil {
					return fmt.Errorf("invalid --min-duration %q: use formats like 45m, 1h, 90m", minDuration)
				}
			}

			// actCandidate pairs an activity ID with its already-fetched summary data.
			type actCandidate struct {
				id   string
				meta map[string]any // nil when only the ID is known (single-ID mode)
			}
			var candidates []actCandidate
			if len(args) > 0 {
				candidates = []actCandidate{{id: args[0]}}
			} else {
				// List recent activities; keep full metadata to avoid a second GET per activity.
				params := map[string]string{"per_page": "100"}
				if since != "" {
					params["after"] = sinceToEpoch(since)
				}
				listData, err := c.Get(cmd.Context(), "/athlete/activities", params)
				if err != nil {
					return fmt.Errorf("listing activities: %w", err)
				}
				var activities []map[string]any
				if err := json.Unmarshal(listData, &activities); err != nil {
					return fmt.Errorf("parsing activities: %w", err)
				}
				for _, act := range activities {
					if activityType != "" {
						if t, _ := act["type"].(string); t != activityType {
							continue
						}
					}
					if mt := jsonFloat(act, "moving_time"); mt < float64(minSecs) {
						continue
					}
					if id := jsonFloat(act, "id"); id > 0 {
						actCopy := act
						candidates = append(candidates, actCandidate{
							id:   fmt.Sprintf("%.0f", id),
							meta: actCopy,
						})
					}
				}
			}

			var result []driftRow
			for i, cand := range candidates {
				actID := cand.id
				if cliutil.IsDogfoodEnv() && len(result) >= 2 {
					break
				}

				// Rate-limit: ~2 req/sec for stream fetches
				if i > 0 {
					time.Sleep(500 * time.Millisecond)
				}

				// Fetch streams: heartrate + velocity_smooth
				streamData, err := c.Get(cmd.Context(),
					"/activities/"+actID+"/streams",
					map[string]string{
						"keys":        "heartrate,velocity_smooth,time",
						"key_by_type": "true",
						"resolution":  "medium",
					})
				if err != nil {
					continue
				}

				// Use cached metadata from list; only fetch detail for single-ID mode.
				act := cand.meta
				if act == nil {
					actData, err := c.Get(cmd.Context(), "/activities/"+actID, nil)
					if err != nil {
						continue
					}
					var fetched map[string]any
					if err := json.Unmarshal(actData, &fetched); err != nil {
						continue
					}
					act = fetched
				}

				row, ok := computeDrift(actID, act, streamData, threshold)
				if ok {
					result = append(result, row)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&minDuration, "min-duration", "45m", "Minimum activity duration to analyze (e.g. 45m, 1h)")
	cmd.Flags().Float64Var(&threshold, "threshold", 5.0, "Flag activities with drift above this percentage")
	cmd.Flags().StringVar(&since, "since", "", "Only analyze activities since this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&activityType, "type", "", "Filter by activity type (Run, Ride, etc.)")
	return cmd
}

func computeDrift(actID string, act map[string]any, streamData json.RawMessage, threshold float64) (driftRow, bool) {
	hrStream := extractStreamValues(string(streamData), "heartrate")
	velStream := extractStreamValues(string(streamData), "velocity_smooth")
	if len(hrStream) < 60 || len(velStream) < 60 {
		return driftRow{}, false
	}

	n := len(hrStream)
	if len(velStream) < n {
		n = len(velStream)
	}
	mid := n / 2

	hr1 := meanSlice(hrStream[:mid])
	hr2 := meanSlice(hrStream[mid:n])
	v1 := meanSlice(velStream[:mid])
	v2 := meanSlice(velStream[mid:n])

	if hr1 == 0 || v1 == 0 || v2 == 0 {
		return driftRow{}, false
	}

	// Aerobic decoupling = (HR2/v2) / (HR1/v1) - 1
	drift := ((hr2 / v2) / (hr1 / v1)) - 1
	driftPct := math.Round(drift*1000) / 10

	name, _ := act["name"].(string)
	startDate, _ := act["start_date"].(string)
	actType, _ := act["type"].(string)
	movingTime := jsonFloat(act, "moving_time")

	return driftRow{
		ID:          actID,
		Name:        name,
		StartDate:   startDate,
		Type:        actType,
		DurationMin: math.Round(movingTime / 60),
		HR1:         math.Round(hr1*10) / 10,
		HR2:         math.Round(hr2*10) / 10,
		Pace1:       math.Round(v1*100) / 100,
		Pace2:       math.Round(v2*100) / 100,
		DriftPct:    driftPct,
		Flagged:     driftPct > threshold,
	}, true
}

func meanSlice(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// parseDuration parses strings like "45m", "1h", "90m" into seconds.
func parseDuration(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil {
			return 0, err
		}
		return n * 3600, nil
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
		if err != nil {
			return 0, err
		}
		return n * 60, nil
	}
	return 0, fmt.Errorf("unrecognized format")
}

// sinceToEpoch converts a YYYY-MM-DD string to a Unix epoch string for Strava's after param.
func sinceToEpoch(date string) string {
	parts := strings.SplitN(date, "-", 3)
	if len(parts) != 3 {
		return ""
	}
	year, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return strconv.FormatInt(t.Unix(), 10)
}
