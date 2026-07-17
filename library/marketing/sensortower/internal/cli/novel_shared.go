// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared plumbing for the novel commands (movers, divergence,
// teardown, watch, compare). Markerless on purpose: `generate --force` must not
// clobber it.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/client"
	"github.com/spf13/cobra"
)

// novelFileExists reports whether the local mirror file is present. A missing
// file is "never synced", not an error.
func novelFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// --- Sensor Tower response shapes -------------------------------------------

// humanizedDownloads is Sensor Tower's pre-bucketed download estimate. The
// numeric fields are rounded to ONE significant figure by the API; only String
// is safe to render, and only as the bucket label it is.
type humanizedDownloads struct {
	Downloads        float64 `json:"downloads"`
	DownloadsRounded float64 `json:"downloads_rounded"`
	Prefix           *string `json:"prefix"`
	String           string  `json:"string"`
	Units            string  `json:"units"`
}

// humanizedRevenue is the revenue counterpart of humanizedDownloads. Same
// one-significant-figure bucketing; render String ("< $5k", "$9m"), never the
// raw number.
type humanizedRevenue struct {
	Prefix         *string `json:"prefix"`
	Revenue        float64 `json:"revenue"`
	RevenueRounded float64 `json:"revenue_rounded"`
	String         string  `json:"string"`
	Units          string  `json:"units"`
}

// bucketedValue is the shape the single-app hub endpoints use for money and
// installs: {"unit":"unit","type":"integer","value":2000000}. It carries no
// display string, so callers must humanize it themselves and keep the result a
// label rather than a precise figure.
type bucketedValue struct {
	Unit     string   `json:"unit"`
	Type     string   `json:"type"`
	Currency string   `json:"currency"`
	Value    *float64 `json:"value"`
}

// downloadsBucket returns the API's own bucket label, or nil when absent.
// Never returns a number: the bucket invariant is that callers can only ever
// print the label.
func downloadsBucket(h *humanizedDownloads) *string {
	if h == nil || strings.TrimSpace(h.String) == "" {
		return nil
	}
	s := h.String
	return &s
}

// revenueBucket returns the API's own revenue bucket label, or nil when absent.
func revenueBucket(h *humanizedRevenue) *string {
	if h == nil || strings.TrimSpace(h.String) == "" {
		return nil
	}
	s := h.String
	return &s
}

// humanizeBucket renders a hub-endpoint bucketedValue as a one-significant-
// figure label ("2m", "300k") matching the style the ranking endpoints return.
// The hub endpoints ship no display string of their own, so this is the only
// way to honor the bucket invariant for `compare`. Returns nil when absent.
func humanizeBucket(b *bucketedValue) *string {
	if b == nil || b.Value == nil {
		return nil
	}
	v := *b.Value
	neg := ""
	if v < 0 {
		neg = "-"
		v = -v
	}
	var s string
	switch {
	case v >= 1e9:
		s = fmt.Sprintf("%s%gb", neg, trimBucket(v/1e9))
	case v >= 1e6:
		s = fmt.Sprintf("%s%gm", neg, trimBucket(v/1e6))
	case v >= 1e3:
		s = fmt.Sprintf("%s%gk", neg, trimBucket(v/1e3))
	default:
		s = fmt.Sprintf("%s%g", neg, trimBucket(v))
	}
	return &s
}

func trimBucket(v float64) float64 {
	// The upstream value is already 1-sig-fig; this only strips float noise.
	return float64(int64(v*100+0.5)) / 100
}

// rankingRow is one row of a category_rankings chart. AppID stays raw because
// iOS app IDs are integers and Android app IDs are package-name strings.
type rankingRow struct {
	AppID              json.RawMessage     `json:"app_id"`
	Name               string              `json:"name"`
	OS                 string              `json:"os"`
	Rank               int                 `json:"rank"`
	PreviousRank       *int                `json:"previous_rank"`
	PublisherID        json.RawMessage     `json:"publisher_id"`
	PublisherName      string              `json:"publisher_name"`
	HumanizedDownloads *humanizedDownloads `json:"humanized_worldwide_last_month_downloads"`
	HumanizedRevenue   *humanizedRevenue   `json:"humanized_worldwide_last_month_revenue"`
}

// categoryRankingsResponse is the whole category_rankings body. One request
// returns all three charts, which is why `movers` and `divergence` each need
// exactly one API call.
type categoryRankingsResponse struct {
	Data struct {
		Free     []rankingRow `json:"free"`
		Grossing []rankingRow `json:"grossing"`
		Paid     []rankingRow `json:"paid"`
	} `json:"data"`
	Date       string `json:"date"`
	TotalCount int    `json:"total_count"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
}

func (r *categoryRankingsResponse) chart(name string) ([]rankingRow, error) {
	switch name {
	case "free":
		return r.Data.Free, nil
	case "grossing":
		return r.Data.Grossing, nil
	case "paid":
		return r.Data.Paid, nil
	}
	return nil, fmt.Errorf("invalid value %q for --chart: must be one of [free grossing paid]", name)
}

// --- small utilities ---------------------------------------------------------

// rawIDKey renders a raw JSON app_id (int on iOS, string on Android) as a
// stable string usable as a store key and join key.
func rawIDKey(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if unq, err := strconv.Unquote(s); err == nil {
		return unq
	}
	return s
}

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// validateChoice keeps flag validation inside RunE so --dry-run probes still
// short-circuit cleanly (cobra's Args/MarkFlagRequired run before RunE).
func validateChoice(name, value string, allowed ...string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return usageErr(fmt.Errorf("invalid value %q for --%s: must be one of %v", value, name, allowed))
}

// resolveOSDevice normalizes --os/--device, defaulting the device to each
// platform's phone chart when the caller did not pick one.
func resolveOSDevice(osName, device string) (string, string, error) {
	if err := validateChoice("os", osName, "ios", "android"); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(device) == "" {
		if osName == "ios" {
			device = "iphone"
		} else {
			device = "phone"
		}
	}
	return osName, device, nil
}

func rankingsPathFor(osName string) string {
	if osName == "android" {
		return "/api/android/category_rankings"
	}
	return "/api/ios/category_rankings"
}

// defaultChartDate is today in UTC. The category_rankings endpoint rejects a
// missing date with HTTP 422 {"errors":{"date":["can't be blank"]}}, so every
// caller must send one.
func defaultChartDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

// fetchCategoryRankings performs the single category_rankings request that both
// `movers` and `divergence` are budgeted for. All three charts come back in
// this one response; never call it twice to get a second chart.
func fetchCategoryRankings(ctx context.Context, c *client.Client, flags *rootFlags, osName, category, country, device, date string, limit int) (*categoryRankingsResponse, error) {
	params := map[string]string{
		"category": category,
		"country":  country,
		"device":   device,
		"date":     date,
	}
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	data, err := c.Get(ctx, rankingsPathFor(osName), params)
	if err != nil {
		// classifyAPIError maps HTTP 429 -> rateLimitErr (exit 7) and 401/403 ->
		// authErr, so a throttled run never degrades into empty results.
		return nil, classifyAPIError(err, flags)
	}
	var parsed categoryRankingsResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, apiErr(fmt.Errorf("decoding category_rankings response: %w", err))
	}
	if strings.TrimSpace(parsed.Date) == "" {
		parsed.Date = date
	}
	return &parsed, nil
}

// epochMSTime converts Sensor Tower's epoch-millisecond timestamps. Returns
// false for absent/zero values rather than silently yielding the epoch.
func epochMSTime(raw json.Number) (time.Time, bool) {
	if strings.TrimSpace(raw.String()) == "" {
		return time.Time{}, false
	}
	ms, err := raw.Int64()
	if err != nil || ms <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(ms).UTC(), true
}

// --- output ------------------------------------------------------------------

// emitNovelResult renders a novel command's result: an auto table for humans at
// a TTY, the standard JSON pipeline (honoring --select/--compact/--agent) for
// everyone else.
//
// rowsKey names the field holding the table rows. Pass "" when the payload is
// itself the row array (as `watch list` does): a top-level array does not
// unmarshal into an envelope object, so without that case a human at a TTY would
// silently fall through to raw JSON while every sibling command printed a table.
func emitNovelResult(cmd *cobra.Command, flags *rootFlags, payload any, rowsKey string) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if rowsKey == "" {
			// The payload is the row array itself; there is no envelope and so
			// no note to trail it with.
			var rows []map[string]any
			if json.Unmarshal(raw, &rows) == nil && len(rows) > 0 {
				return printAutoTable(cmd.OutOrStdout(), rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
		}
		var envelope map[string]json.RawMessage
		if json.Unmarshal(raw, &envelope) == nil {
			if rowsRaw, ok := envelope[rowsKey]; ok {
				var rows []map[string]any
				if json.Unmarshal(rowsRaw, &rows) == nil && len(rows) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
						return err
					}
					if noteRaw, ok := envelope["note"]; ok {
						var note string
						if json.Unmarshal(noteRaw, &note) == nil && note != "" {
							fmt.Fprintf(cmd.ErrOrStderr(), "\nnote: %s\n", note)
						}
					}
					return nil
				}
			}
		}
	}
	return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
}

// --- local rank snapshot store ----------------------------------------------

const rankSnapshotResource = "rank_snapshot"

// rankSnapshot is what `movers` writes and what `teardown`/`watch digest` read
// back. Every field the readers filter on is stored explicitly so the SQLite
// json_extract predicates never have to parse a composite key.
type rankSnapshot struct {
	AppID    json.RawMessage `json:"app_id"`
	AppIDKey string          `json:"app_id_key"`
	Name     string          `json:"name"`
	OS       string          `json:"os"`
	Category string          `json:"category"`
	Country  string          `json:"country"`
	Chart    string          `json:"chart"`
	Device   string          `json:"device"`
	Date     string          `json:"date"`
	Rank     int             `json:"rank"`
	// PreviousRank is the API's own prior-day rank, kept distinct from the
	// delta this CLI derives against its own snapshot history.
	PreviousRank *int   `json:"previous_rank"`
	Downloads    string `json:"downloads,omitempty"`
	Revenue      string `json:"revenue,omitempty"`
	CapturedAt   string `json:"captured_at"`
	// Limit records how deep the chart was captured, so a reader can tell
	// "fell off the tracked window" from "left the chart".
	Limit int `json:"limit"`
}

func rankSnapshotID(osName, category, country, chart, date, appIDKey string) string {
	return strings.Join([]string{osName, category, country, chart, date, appIDKey}, ":")
}
