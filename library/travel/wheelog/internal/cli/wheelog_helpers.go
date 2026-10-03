// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
)

const wheelogEvidenceNote = "Counts are crowdsourced question reports. Source record dates do not date individual reports or establish personal suitability, current conditions, measured dimensions or an accessible route."

type wheelogReadOptions struct {
	DB        string
	Questions []int
	MaxAge    string
}

func addWheelogReadFlags(cmd *cobra.Command, options *wheelogReadOptions) {
	cmd.Flags().StringVar(&options.DB, "db", defaultDBPath("wheelog-pp-cli"), "Local SQLite shortlist path; stores only normalized public-place evidence.")
	cmd.Flags().IntSliceVar(&options.Questions, "require-question", []int{}, "Exact source question IDs to assess; repeat or use comma-separated IDs.")
	cmd.Flags().StringVar(&options.MaxAge, "max-record-age", "180d", "Age threshold for source record updates and saved retrievals; accepts d/w durations.")
}

func validateWheelogOptions(options wheelogReadOptions) (time.Duration, error) {
	if len(options.Questions) > 10 {
		return 0, usageErr(fmt.Errorf("--require-question is limited to 10 source question IDs"))
	}
	seen := map[int]bool{}
	for _, id := range options.Questions {
		if seen[id] {
			return 0, usageErr(fmt.Errorf("--require-question IDs must be unique"))
		}
		seen[id] = true
		if _, known := wheelog.QuestionCategory(id); !known {
			return 0, usageErr(fmt.Errorf("--require-question %d is unknown; inspect a spot or run categories for source question IDs", id))
		}
	}
	age, err := cliutil.ParseDurationLoose(options.MaxAge)
	if err != nil || age <= 0 || age > 10*365*24*time.Hour {
		return 0, usageErr(fmt.Errorf("--max-record-age must be a positive duration up to 3650d"))
	}
	return age, nil
}

func wheelogID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 || id > 1000000000 {
		return 0, usageErr(fmt.Errorf("spot ID must be an integer between 1 and 1000000000"))
	}
	return id, nil
}

func wheelogMode(flags *rootFlags) (string, error) {
	if flags.timeout <= 0 || flags.timeout > 2*time.Minute {
		return "", usageErr(fmt.Errorf("--timeout must be positive and no more than 2m for bounded WheeLog reads"))
	}
	switch flags.dataSource {
	case "", "auto":
		return "auto", nil
	case "local", "live":
		return flags.dataSource, nil
	default:
		return "", usageErr(fmt.Errorf("--data-source must be auto, local or live"))
	}
}

func newWheelogClient(flags *rootFlags) (*client.Client, error) {
	if flags.timeout <= 0 || flags.timeout > 2*time.Minute {
		return nil, usageErr(fmt.Errorf("--timeout must be positive and no more than 2m for bounded WheeLog reads"))
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.NoCache = true
	return c, nil
}

func openWheelogStore(ctx context.Context, path string) (*store.Store, error) {
	db, err := store.OpenWheelogWithContext(ctx, path)
	if err != nil {
		return nil, configErr(err)
	}
	if err := db.InitWheelog(ctx); err != nil {
		_ = db.Close() // Preserve the initialization failure while releasing the handle.
		return nil, configErr(err)
	}
	return db, nil
}

func savedWheelog(ctx context.Context, path string) ([]store.WheelogObservation, error) {
	items, err := store.ReadWheelogSnapshot(ctx, path)
	if err != nil {
		return nil, configErr(err)
	}
	return items, nil
}

func wheelogLocalHint(cmd *cobra.Command, items []store.WheelogObservation, maxAge time.Duration) {
	if len(items) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "Saved shortlist is empty; use shortlist save with selected spot IDs.")
		return
	}
	for _, item := range items {
		if age := wheelog.Age(&item.Latest.ObservedAt, time.Now()); age == nil || *age > maxAge.Hours()/24 {
			fmt.Fprintln(cmd.ErrOrStderr(), "Saved observations are old or undated; use shortlist changes to request current source records.")
			return
		}
	}
}

func fetchWheelogSpot(ctx context.Context, c *client.Client, id int64) (wheelog.Spot, error) {
	data, _, err := c.PostQueryFormWithParams(ctx, "/poi/custom/user/getWebSpotDetail", nil, url.Values{"spotId": {strconv.FormatInt(id, 10)}})
	if err != nil {
		return wheelog.Spot{}, err
	}
	var envelope wheelog.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return wheelog.Spot{}, fmt.Errorf("decoding normalized WheeLog detail")
	}
	if envelope.Content == nil || envelope.Content.Spot == nil || envelope.Content.Spot.ID != id {
		return wheelog.Spot{}, fmt.Errorf("WheeLog returned a different or unavailable spot")
	}
	return wheelog.Normalize(*envelope.Content.Spot, time.Now().UTC())
}

func isWheelogThrottle(err error) bool {
	var throttle *cliutil.RateLimitError
	return errors.As(err, &throttle)
}
func wheelogError(err error) error {
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	if isWheelogThrottle(err) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

func resolveWheelogSpot(ctx context.Context, flags *rootFlags, options wheelogReadOptions, id int64, c *client.Client, cached []store.WheelogObservation) (wheelog.Spot, error) {
	mode, err := wheelogMode(flags)
	if err != nil {
		return wheelog.Spot{}, err
	}
	var liveErr error
	if mode != "local" {
		spot, err := fetchWheelogSpot(ctx, c, id)
		if err == nil {
			return spot, nil
		}
		if isWheelogThrottle(err) {
			return wheelog.Spot{}, err
		}
		liveErr = err
		if mode == "live" {
			return wheelog.Spot{}, err
		}
	}
	if mode == "auto" {
		cached, err = savedWheelog(ctx, options.DB)
		if err != nil {
			return wheelog.Spot{}, configErr(fmt.Errorf("live WheeLog read failed (%v); saved fallback unavailable: %w", liveErr, err))
		}
	}
	for _, item := range cached {
		if item.Latest.ID == id {
			spot := item.Latest
			spot.Source = "local"
			return spot, nil
		}
	}
	if liveErr != nil {
		return wheelog.Spot{}, liveErr
	}
	return wheelog.Spot{}, notFoundErr(fmt.Errorf("spot %d has no saved observation; use shortlist save first", id))
}

func emitWheelog(cmd *cobra.Command, flags *rootFlags, value any, source string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// A documented-field map is not a recursive compact keep contract.
	// These bounded normalized fields are decision evidence, including keys
	// inside result, question, requirement and change collections.
	documented := map[string]bool{
		"id": true, "name": true, "address": true, "category": true, "location": true, "source_url": true, "questions": true,
		"requirements": true, "results": true, "source_query": true, "coverage": true, "record_created_at": true, "record_updated_at": true,
		"retrieved_at": true, "data_source": true, "detail_status": true, "status": true, "fetch_failures": true, "unsupported_facts": true,
		"changes": true, "recheck_reasons": true, "straight_line_distance_m": true, "record_update_age_days": true, "cache_age_days": true,
		"note": true, "value": true, "fallback_reason": true, "question_ids": true, "previous_retrieved_at": true, "current_retrieved_at": true,
	}
	keep := make([]string, 0, len(documented)+12)
	for field := range documented {
		keep = append(keep, field)
	}
	keep = append(keep, "error", "label", "positive_reports", "negative_reports", "report_observed_at", "question_id", "state", "reason", "field", "previous", "current", "source_created_raw", "source_updated_raw")
	return printOutputWithFlagsMetaAndKeep(cmd.OutOrStdout(), data, flags, map[string]any{"source": source, "provider": "wheelog"}, keep, documented)
}

func wheelogCategories(values string) ([]string, error) {
	result := make([]string, 0)
	seen := map[string]bool{}
	if values == "" {
		return result, nil
	}
	for _, value := range strings.Split(values, ",") {
		value = strings.TrimSpace(value)
		if !wheelog.ValidCategory(value) {
			return nil, usageErr(fmt.Errorf("--category %q is unsupported; use exact values from categories", value))
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func wheelogDateWindow(from, to, timezone string) (*string, *string, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, nil, usageErr(fmt.Errorf("--timezone must name a valid IANA timezone such as Asia/Tokyo or UTC"))
	}
	var start, end *string
	var startTime, endTime time.Time
	if from != "" {
		parsed, err := time.ParseInLocation("2006-01-02", from, location)
		if err != nil {
			return nil, nil, usageErr(fmt.Errorf("--from must be a valid YYYY-MM-DD record date"))
		}
		startTime = parsed
		value := parsed.UTC().Format("2006-01-02 15:04:05")
		start = &value
	}
	if to != "" {
		parsed, err := time.ParseInLocation("2006-01-02", to, location)
		if err != nil {
			return nil, nil, usageErr(fmt.Errorf("--to must be a valid YYYY-MM-DD record date"))
		}
		endTime = parsed.AddDate(0, 0, 1).Add(-time.Second)
		value := endTime.UTC().Format("2006-01-02 15:04:05")
		end = &value
	}
	if from != "" && to != "" && startTime.After(endTime) {
		return nil, nil, usageErr(fmt.Errorf("--from must not be after --to"))
	}
	return start, end, nil
}

type wheelogFailure struct {
	ID    int64  `json:"id"`
	Error string `json:"error"`
}
type wheelogSpotView struct {
	wheelog.Spot
	Requirements []wheelog.Requirement `json:"requirements"`
	UpdateAge    *float64              `json:"record_update_age_days"`
	CacheAge     *float64              `json:"cache_age_days"`
	Recheck      []string              `json:"recheck_reasons"`
	Distance     *float64              `json:"straight_line_distance_m,omitempty"`
}

func viewWheelog(spot wheelog.Spot, questions []int, maxAge time.Duration) wheelogSpotView {
	now := time.Now().UTC()
	return wheelogSpotView{Spot: spot, Requirements: wheelog.Evaluate(spot, questions), UpdateAge: wheelog.Age(spot.Updated, now), CacheAge: wheelog.Age(&spot.ObservedAt, now), Recheck: wheelog.RecheckReasons(spot, questions, maxAge, now)}
}
