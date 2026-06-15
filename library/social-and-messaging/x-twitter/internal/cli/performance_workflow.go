// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type performanceSnapshot struct {
	TweetID            string            `json:"tweet_id"`
	URL                string            `json:"url"`
	Label              string            `json:"label,omitempty"`
	CapturedAt         string            `json:"captured_at"`
	MetricSource       string            `json:"metric_source"`
	Metrics            map[string]any    `json:"metrics,omitempty"`
	PublicMetrics      map[string]any    `json:"public_metrics,omitempty"`
	NonPublicMetrics   map[string]any    `json:"non_public_metrics,omitempty"`
	MetricAvailability map[string]string `json:"metric_availability,omitempty"`
	PostAgeSeconds     *int64            `json:"post_age_seconds,omitempty"`
	Source             string            `json:"source"`
}

type performanceAnalyzeGroup struct {
	Group       string             `json:"group"`
	Count       int                `json:"count"`
	Averages    map[string]float64 `json:"averages,omitempty"`
	Totals      map[string]float64 `json:"totals,omitempty"`
	TopExamples []string           `json:"top_examples,omitempty"`
}

func newNovelPerformanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Snapshot, backfill, and analyze post performance metrics",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPerformanceSnapshotCmd(flags))
	cmd.AddCommand(newNovelPerformanceBackfillCmd(flags))
	cmd.AddCommand(newNovelPerformanceAnalyzeCmd(flags))
	return cmd
}

func newNovelPerformanceSnapshotCmd(flags *rootFlags) *cobra.Command {
	var dbPath, ids, collection, label string
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture timestamped metrics for posts",
		Example: `  x-twitter-pp-cli performance snapshot --ids 123,456 --label 24h --agent
  x-twitter-pp-cli performance snapshot --collection launch-posts --label 72h --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			records, err := performanceInputRecords(cmd, flags, dbPath, ids, collection)
			if err != nil {
				return err
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snapshots, err := savePerformanceSnapshots(cmd, db, records, label)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("performance snapshot", "mixed"),
				Results: snapshots,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated post IDs or URLs")
	cmd.Flags().StringVar(&collection, "collection", "", "Snapshot posts in a local collection")
	cmd.Flags().StringVar(&label, "label", "", "Capture label such as 1h, 24h, 72h, or launch-day")
	return cmd
}

func newNovelPerformanceBackfillCmd(flags *rootFlags) *cobra.Command {
	var dbPath, account, label string
	var mine bool
	var days, limit int
	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill performance snapshots from recent account posts",
		Example: `  x-twitter-pp-cli performance backfill --mine --days 90 --agent
  x-twitter-pp-cli performance backfill --account @username --days 30 --agent`,
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !mine && strings.TrimSpace(account) == "" {
				return usageErr(fmt.Errorf("set --mine or --account"))
			}
			if label == "" {
				label = fmt.Sprintf("backfill-%dd", days)
			}
			var profile *accountSnapshotProfile
			if mine {
				var err error
				profile, err = resolveMeAccount(cmd, flags, dbPath)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			} else {
				var err error
				profile, err = resolveAccountProfile(cmd, flags, account, dbPath, "live", false)
				if err != nil {
					return err
				}
			}
			if limit <= 0 {
				limit = 100
			}
			records, err := liveRecentPostsForAccount(cmd, flags, profile.ID, limit, parseIncludeSet("author,links,refs,metrics"))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if days > 0 {
				cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
				filtered := records[:0]
				for _, rec := range records {
					if postCreatedAfter(rec, cutoff) {
						filtered = append(filtered, rec)
					}
				}
				records = filtered
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snapshots, err := savePerformanceSnapshots(cmd, db, records, label)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta: workflowMeta("performance backfill", "live"),
				Results: map[string]any{
					"account":   profile,
					"snapshots": snapshots,
				},
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().BoolVar(&mine, "mine", false, "Backfill the authenticated user's recent posts")
	cmd.Flags().StringVar(&account, "account", "", "Backfill a specific account's recent posts")
	cmd.Flags().IntVar(&days, "days", 90, "Historical window in days")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum recent posts to inspect")
	cmd.Flags().StringVar(&label, "label", "", "Snapshot label")
	return cmd
}

func newNovelPerformanceAnalyzeCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, groupBy string
	cmd := &cobra.Command{
		Use:         "analyze",
		Short:       "Analyze locally stored performance snapshots",
		Example:     `  x-twitter-pp-cli performance analyze --since 90d --group-by type,hour,has_media,has_link --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openWorkflowDB(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			groups, err := analyzePerformanceSnapshots(cmd, db, since, groupBy)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), workflowEnvelope{
				Meta:    workflowMeta("performance analyze", "local"),
				Results: groups,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database")
	cmd.Flags().StringVar(&since, "since", "90d", "Analyze snapshots since this window")
	cmd.Flags().StringVar(&groupBy, "group-by", "type", "Comma-separated grouping dimensions: type,hour,has_media,has_link,label")
	return cmd
}

func performanceInputRecords(cmd *cobra.Command, flags *rootFlags, dbPath, ids, collection string) ([]*resolvedPostRecord, error) {
	if strings.TrimSpace(ids) == "" && strings.TrimSpace(collection) == "" {
		return nil, usageErr(fmt.Errorf("set --ids or --collection"))
	}
	if strings.TrimSpace(ids) != "" && strings.TrimSpace(collection) != "" {
		return nil, usageErr(fmt.Errorf("--ids and --collection cannot be combined"))
	}
	include := parseIncludeSet("author,links,refs,metrics")
	if collection != "" {
		db, err := openWorkflowDB(cmd, dbPath)
		if err != nil {
			return nil, err
		}
		defer db.Close()
		items, err := listCollectionItems(cmd, db, collection, 0, true)
		if err != nil {
			return nil, err
		}
		var out []*resolvedPostRecord
		for _, item := range items {
			if item.Snapshot != nil {
				out = append(out, item.Snapshot)
			} else if item.TweetID != "" {
				rec, err := resolvePost(cmd, flags, item.TweetID, dbPath, flags.dataSource, include)
				if err != nil {
					return nil, err
				}
				out = append(out, rec)
			}
		}
		return out, nil
	}
	var out []*resolvedPostRecord
	for _, input := range strings.Split(ids, ",") {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		rec, err := resolvePost(cmd, flags, input, dbPath, flags.dataSource, include)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func savePerformanceSnapshots(cmd *cobra.Command, db *store.Store, records []*resolvedPostRecord, label string) ([]performanceSnapshot, error) {
	now := generatedAt()
	var snapshots []performanceSnapshot
	tx, err := db.DB().BeginTx(cmd.Context(), nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(cmd.Context(),
		`INSERT INTO post_performance_snapshots(tweet_id, label, captured_at, metrics_json, tweet_json, source_url, post_age_seconds, source)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	deleteStmt, err := tx.PrepareContext(cmd.Context(),
		`DELETE FROM post_performance_snapshots WHERE tweet_id = ? AND COALESCE(label, '') = COALESCE(?, '')`)
	if err != nil {
		return nil, err
	}
	defer deleteStmt.Close()
	for _, rec := range records {
		if rec == nil {
			continue
		}
		raw, err := json.Marshal(rec)
		if err != nil {
			return nil, err
		}
		metricsRaw, err := json.Marshal(rec.PublicMetrics)
		if err != nil {
			return nil, err
		}
		var age *int64
		if created, err := time.Parse(time.RFC3339, rec.CreatedAt); err == nil {
			seconds := int64(time.Since(created).Seconds())
			age = &seconds
		}
		var ageArg any
		if age != nil {
			ageArg = *age
		}
		if _, err := deleteStmt.ExecContext(cmd.Context(), rec.TweetID, label); err != nil {
			return nil, err
		}
		if _, err := stmt.ExecContext(cmd.Context(),
			rec.TweetID, label, now, string(metricsRaw), string(raw), rec.URL, ageArg, rec.Source); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, performanceSnapshot{
			TweetID:            rec.TweetID,
			URL:                rec.URL,
			Label:              label,
			CapturedAt:         now,
			MetricSource:       performanceMetricSource(rec),
			Metrics:            rec.PublicMetrics,
			PublicMetrics:      rec.PublicMetrics,
			NonPublicMetrics:   mergedNonPublicMetrics(rec),
			MetricAvailability: metricAvailability(rec),
			PostAgeSeconds:     age,
			Source:             rec.Source,
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func performanceMetricSource(rec *resolvedPostRecord) string {
	if rec != nil && rec.Source == "live" {
		return "x_api_owned_or_public_post"
	}
	return "local_resolved_post"
}

func mergedNonPublicMetrics(rec *resolvedPostRecord) map[string]any {
	if rec == nil {
		return nil
	}
	out := map[string]any{}
	for k, v := range rec.NonPublicMetrics {
		out[k] = v
	}
	for k, v := range rec.OrganicMetrics {
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func metricAvailability(rec *resolvedPostRecord) map[string]string {
	availability := map[string]string{}
	publicMetrics := map[string]any{}
	missingStatus := "not_requested"
	if rec != nil {
		publicMetrics = rec.PublicMetrics
		if performanceMetricSource(rec) != "local_resolved_post" {
			missingStatus = "not_returned_or_plan_restricted"
		}
	}
	for _, key := range []string{"like_count", "reply_count", "repost_count", "quote_count", "bookmark_count", "impression_count"} {
		if _, ok := publicMetrics[key]; ok {
			availability[key] = "available"
		} else {
			availability[key] = missingStatus
		}
	}
	nonPublic := mergedNonPublicMetrics(rec)
	for _, key := range []string{"profile_clicks", "user_profile_clicks", "url_link_clicks", "url_clicks", "detail_expands"} {
		if _, ok := nonPublic[key]; ok {
			availability[key] = "available"
		} else {
			availability[key] = missingStatus
		}
	}
	return availability
}

func resolveMeAccount(cmd *cobra.Command, flags *rootFlags, dbPath string) (*accountSnapshotProfile, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(cmd.Context(), "/2/users/me", map[string]string{
		"user.fields": "created_at,description,location,pinned_tweet_id,profile_image_url,protected,public_metrics,url,username,verified,verified_type",
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	if db, err := store.OpenWithContext(cmd.Context(), dbPath); err == nil {
		_ = db.UpsertUsers(envelope.Data)
		_ = db.Close()
	}
	return normalizeAccountProfile(envelope.Data, "live", "not_synced", false)
}

func postCreatedAfter(rec *resolvedPostRecord, cutoff time.Time) bool {
	if rec == nil || rec.CreatedAt == "" {
		return true
	}
	created, err := time.Parse(time.RFC3339, rec.CreatedAt)
	return err != nil || created.After(cutoff)
}

func analyzePerformanceSnapshots(cmd *cobra.Command, db *store.Store, since, groupBy string) ([]performanceAnalyzeGroup, error) {
	q := `SELECT label, captured_at, metrics_json, tweet_json FROM post_performance_snapshots`
	args := []any{}
	if start, ok, err := sinceStartTime(since); err != nil {
		return nil, err
	} else if ok {
		q += ` WHERE captured_at >= ?`
		args = append(args, start.Format(time.RFC3339))
	}
	rows, err := db.DB().QueryContext(cmd.Context(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type accum struct {
		count int
		total map[string]float64
		top   []string
	}
	groups := map[string]*accum{}
	dims := parseIncludeSet(groupBy)
	for rows.Next() {
		var label, capturedAt, metricsRaw, tweetRaw string
		if err := rows.Scan(&label, &capturedAt, &metricsRaw, &tweetRaw); err != nil {
			return nil, err
		}
		var metrics map[string]any
		_ = json.Unmarshal([]byte(metricsRaw), &metrics)
		var rec resolvedPostRecord
		_ = json.Unmarshal([]byte(tweetRaw), &rec)
		key := performanceGroupKey(dims, label, &rec)
		a := groups[key]
		if a == nil {
			a = &accum{total: map[string]float64{}}
			groups[key] = a
		}
		a.count++
		for k, v := range metrics {
			if n, ok := numericAny(v); ok {
				a.total[k] += n
			}
		}
		if len(a.top) < 5 && rec.URL != "" {
			a.top = append(a.top, rec.URL)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]performanceAnalyzeGroup, 0, len(keys))
	for _, k := range keys {
		a := groups[k]
		avg := map[string]float64{}
		for metric, total := range a.total {
			if a.count > 0 {
				avg[metric] = total / float64(a.count)
			}
		}
		out = append(out, performanceAnalyzeGroup{Group: k, Count: a.count, Totals: a.total, Averages: avg, TopExamples: a.top})
	}
	return out, nil
}

func performanceGroupKey(dims map[string]bool, label string, rec *resolvedPostRecord) string {
	if len(dims) == 0 {
		return "all"
	}
	parts := []string{}
	if dims["label"] {
		parts = append(parts, "label="+label)
	}
	if dims["type"] && rec != nil {
		parts = append(parts, "type="+rec.PostType)
	}
	if dims["hour"] && rec != nil {
		if t, err := time.Parse(time.RFC3339, rec.CreatedAt); err == nil {
			parts = append(parts, fmt.Sprintf("hour=%02d", t.Hour()))
		}
	}
	if dims["has_media"] && rec != nil {
		parts = append(parts, fmt.Sprintf("has_media=%t", len(rec.Media) > 0))
	}
	if dims["has_link"] && rec != nil {
		parts = append(parts, fmt.Sprintf("has_link=%t", postHasURL(rec)))
	}
	if len(parts) == 0 {
		return "all"
	}
	return strings.Join(parts, ",")
}

func postHasURL(rec *resolvedPostRecord) bool {
	if rec == nil || len(rec.Entities) == 0 {
		return false
	}
	switch urls := rec.Entities["urls"].(type) {
	case []any:
		return len(urls) > 0
	case []map[string]any:
		return len(urls) > 0
	default:
		return false
	}
}

func numericAny(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
