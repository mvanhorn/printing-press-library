// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: hiring-volume trends from the local listings history.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/store"
)

var trendsDimensions = map[string]bool{
	"classification": true, "subclassification": true, "region": true,
	"worktype": true, "workarrangement": true,
}

type trendBucket struct {
	Period string         `json:"period"`
	Total  int            `json:"total"`
	Delta  int            `json:"delta_vs_prev"`
	By     map[string]int `json:"by,omitempty"`
}

type trendsView struct {
	By          string        `json:"by"`
	Granularity string        `json:"granularity"`
	Since       string        `json:"since,omitempty"`
	Keywords    string        `json:"keywords,omitempty"`
	Where       string        `json:"where,omitempty"`
	Listings    int           `json:"listings_in_scope"`
	Buckets     []trendBucket `json:"buckets"`
	Note        string        `json:"note,omitempty"`
}

func newNovelTrendsCmd(flags *rootFlags) *cobra.Command {
	var flagBy, flagSince, flagKeywords, flagWhere, flagGranularity, flagDB string

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Hiring-volume trend series over the local listings history",
		Long: "Use trends for how job-listing volume changes over time, bucketed from the\n" +
			"listing dates of every job seek-pp-cli has cached locally, with period-over-period\n" +
			"deltas.\n\n" +
			"For a point-in-time breakdown of a live query with no local history, use\n" +
			"'listings facets'. For percentile pay statistics, use 'salary'.",
		Example: strings.Trim(`
  seek-pp-cli trends --by classification --since 180d --agent
  seek-pp-cli trends --by region --keywords "software" --granularity week`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trends")
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			flagBy = strings.ToLower(strings.TrimSpace(flagBy))
			if flagBy == "" {
				flagBy = "classification"
			}
			if !trendsDimensions[flagBy] {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by must be one of classification, subclassification, region, worktype, workarrangement"))
			}
			gran := strings.ToLower(strings.TrimSpace(flagGranularity))
			if gran != "week" {
				gran = "month"
			}

			var since time.Time
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = time.Now().Add(-d)
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("seek-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local listings history at %s\nrun a few searches or `seek-pp-cli salary ...` / `seek-pp-cli company ...` first to populate it.\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), trendsView{By: flagBy, Granularity: gran, Buckets: []trendBucket{}}, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM listings`)
			if err != nil {
				return fmt.Errorf("reading listings: %w", err)
			}
			var raws []json.RawMessage
			for rows.Next() {
				var d []byte
				if rows.Scan(&d) != nil {
					continue
				}
				raws = append(raws, append(json.RawMessage(nil), d...))
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating listings: %w", err)
			}
			_ = rows.Close()

			kw := strings.ToLower(strings.TrimSpace(flagKeywords))
			where := strings.ToLower(strings.TrimSpace(flagWhere))
			perBucket := map[string]map[string]int{}
			inScope := 0
			var minListed time.Time
			for _, raw := range raws {
				var j seekJob
				if json.Unmarshal(raw, &j) != nil {
					continue
				}
				listed, ok := parseSeekDate(j.ListingDate)
				if !ok {
					continue
				}
				if !since.IsZero() && listed.Before(since) {
					continue
				}
				if kw != "" && !strings.Contains(strings.ToLower(j.Title+" "+j.Teaser), kw) {
					continue
				}
				if where != "" && !strings.Contains(strings.ToLower(j.region()), where) {
					continue
				}
				dim := trendDimension(j, flagBy)
				if dim == "" {
					dim = "(unspecified)"
				}
				bkt := bucketKey(listed, gran)
				if perBucket[bkt] == nil {
					perBucket[bkt] = map[string]int{}
				}
				perBucket[bkt][dim]++
				inScope++
				if minListed.IsZero() || listed.Before(minListed) {
					minListed = listed
				}
			}

			// Materialise every calendar period across the window — an explicit
			// --since, or the earliest in-scope listing, through today — so a
			// month/week with zero cached listings still gets a bucket and the
			// period-over-period delta compares adjacent periods, not the two
			// nearest populated ones.
			keySet := map[string]bool{}
			var keys []string
			if inScope > 0 {
				windowStart := since
				if windowStart.IsZero() {
					windowStart = minListed
				}
				for _, k := range bucketRange(windowStart, time.Now(), gran) {
					if !keySet[k] {
						keySet[k] = true
						keys = append(keys, k)
					}
				}
			}
			for k := range perBucket {
				if !keySet[k] {
					keySet[k] = true
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)

			view := trendsView{
				By: flagBy, Granularity: gran, Keywords: flagKeywords, Where: flagWhere,
				Listings: inScope, Buckets: make([]trendBucket, 0, len(keys)),
			}
			if !since.IsZero() {
				view.Since = flagSince
			}
			prevTotal := 0
			for i, k := range keys {
				by := perBucket[k]
				total := 0
				for _, n := range by {
					total += n
				}
				b := trendBucket{Period: k, Total: total, By: by}
				if i > 0 {
					b.Delta = total - prevTotal
				}
				prevTotal = total
				view.Buckets = append(view.Buckets, b)
			}
			if inScope == 0 {
				view.Note = "no cached listings match this filter/window; widen --since or run more searches to build history."
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Hiring trend by %s (%sly) — %d cached listings in scope\n", view.By, gran, inScope)
			if inScope == 0 {
				fmt.Fprintf(w, "  %s\n", view.Note)
				return nil
			}
			for _, b := range view.Buckets {
				sign := ""
				if b.Delta > 0 {
					sign = fmt.Sprintf("  (+%d)", b.Delta)
				} else if b.Delta < 0 {
					sign = fmt.Sprintf("  (%d)", b.Delta)
				}
				fmt.Fprintf(w, "  %-10s %4d%s\n", b.Period, b.Total, sign)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "classification", "Dimension: classification, subclassification, region, worktype, workarrangement")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only listings posted within this window (e.g. 90d, 12w)")
	cmd.Flags().StringVar(&flagGranularity, "granularity", "month", "Bucket size: month or week")
	cmd.Flags().StringVar(&flagKeywords, "keywords", "", "Restrict to cached listings whose title/teaser contains this text")
	cmd.Flags().StringVar(&flagWhere, "where", "", "Restrict to cached listings in this region")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local store path (default: the CLI data dir)")
	return cmd
}

func trendDimension(j seekJob, by string) string {
	switch by {
	case "classification":
		_, n := j.topClassification()
		return n
	case "subclassification":
		_, n := j.subClassification()
		return n
	case "region":
		return j.region()
	case "worktype":
		return j.workType()
	case "workarrangement":
		return j.workArrangement()
	}
	return ""
}

func parseSeekDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func bucketKey(t time.Time, gran string) string {
	if gran == "week" {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	}
	return t.Format("2006-01")
}

// bucketRange returns every period key between start and end inclusive, so the
// caller can fill zero buckets for periods that have no cached listings and keep
// period-over-period deltas comparing adjacent calendar periods. It is bounded
// to a few thousand iterations so a pathological window can't spin.
func bucketRange(start, end time.Time, gran string) []string {
	if start.IsZero() || end.Before(start) {
		return nil
	}
	const maxBuckets = 4000
	var keys []string
	if gran == "week" {
		seen := map[string]bool{}
		for cur := start; !cur.After(end) && len(keys) < maxBuckets; cur = cur.AddDate(0, 0, 1) {
			k := bucketKey(cur, gran)
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		return keys
	}
	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(last) && len(keys) < maxBuckets {
		keys = append(keys, bucketKey(cur, gran))
		cur = cur.AddDate(0, 1, 0)
	}
	return keys
}
