// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

const syncFirstMessage = "Run klaviyo-pp-cli sync first to populate local data."

type novelEmailEvent struct {
	ProfileID    string
	Email        string
	Metric       string
	Time         time.Time
	FlowID       string
	FlowName     string
	CampaignID   string
	CampaignName string
	MessageID    string
	MessageName  string
	Subject      string
	Revenue      float64
}

func newFlowCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var dbPath, window, last string
	cmd := &cobra.Command{
		Use:         "flow-cannibalization",
		Short:       "Find flows sending to the same profiles inside a short window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "window": window, "last": last, "source": "local_store", "planned_steps": []string{"read_received_email_events", "group_by_profile", "detect_flow_pairs", "rank_collisions"}}, flags)
			}
			rows, err := readNovelLocalRows(cmd.Context(), dbPath, "events", 50000)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), localDataRequiredResult(), flags)
			}
			windowDur, err := parseNovelDuration(window)
			if err != nil {
				return usageErr(fmt.Errorf("--window must be a duration like 24h or 2d"))
			}
			since, err := sinceFromLast(last)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), flowCannibalization(rows, windowDur, window, since, "last "+last), flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	cmd.Flags().StringVar(&window, "window", "24h", "Collision window")
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	return cmd
}

func newSendFatigueCmd(flags *rootFlags) *cobra.Command {
	var dbPath, window, last string
	var threshold int
	cmd := &cobra.Command{
		Use:         "send-fatigue",
		Short:       "Find profiles receiving too many emails in a rolling window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "threshold": threshold, "window": window, "last": last, "source": "local_store", "planned_steps": []string{"read_received_and_opened_events", "count_rolling_profile_sends", "rank_fatigued_profiles"}}, flags)
			}
			rows, err := readNovelLocalRows(cmd.Context(), dbPath, "events", 75000)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), localDataRequiredResult(), flags)
			}
			windowDur, err := parseNovelDuration(window)
			if err != nil {
				return usageErr(fmt.Errorf("--window must be a duration like 7d"))
			}
			since, err := sinceFromLast(last)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), sendFatigue(rows, threshold, windowDur, window, since), flags)
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 8, "Emails per window that counts as fatigued")
	cmd.Flags().StringVar(&window, "window", "7d", "Rolling count window")
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	return cmd
}

func newSubjectLineAnalysisCmd(flags *rootFlags) *cobra.Command {
	var last, source string
	cmd := &cobra.Command{
		Use:         "subject-line-analysis",
		Short:       "Analyze subject-line patterns against engagement rates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "source": source, "planned_steps": []string{"fetch_flow_and_campaign_subjects", "query_open_click_rates", "compare_subject_patterns"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			result, err := subjectLineAnalysis(c, since, until, source)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	cmd.Flags().StringVar(&source, "source", "all", "Analyze flow, campaign, or all")
	return cmd
}

func newOptimalSendTimeCmd(flags *rootFlags) *cobra.Command {
	var last, timezoneName, metric string
	cmd := &cobra.Command{
		Use:         "optimal-send-time",
		Short:       "Find day and hour windows with the strongest engagement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "timezone": timezoneName, "metric": metric, "planned_steps": []string{"query_hourly_engagement", "query_hourly_received", "build_heatmap", "rank_windows"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			result, err := optimalSendTime(c, since, until, timezoneName, metric)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	cmd.Flags().StringVar(&timezoneName, "timezone", "US/Central", "Timezone for bucketing")
	cmd.Flags().StringVar(&metric, "metric", "opened", "Engagement metric: opened or clicked")
	return cmd
}

func newRevenuePerEmailCmd(flags *rootFlags) *cobra.Command {
	var last, by string
	cmd := &cobra.Command{
		Use:         "revenue-per-email",
		Short:       "Rank flows and campaigns by attributed revenue per email sent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "by": by, "planned_steps": []string{"query_attributed_revenue", "query_received_email_counts", "divide_revenue_by_sends"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			result, err := revenuePerEmail(c, since, until, by, "last "+last)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	cmd.Flags().StringVar(&by, "by", "all", "Scope: flow, campaign, or all")
	return cmd
}

func newSegmentVelocityCmd(flags *rootFlags) *cobra.Command {
	var ids []string
	var allTagged, last, interval, dbPath string
	cmd := &cobra.Command{
		Use:   "segment-velocity",
		Short: "Snapshot segment size and compute growth velocity over time",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "ids": ids, "all_tagged": allTagged, "last": last, "interval": interval, "planned_steps": []string{"resolve_segments", "fetch_current_profile_count", "store_snapshot", "compute_trend"}}, flags)
			}
			if len(ids) == 0 && allTagged == "" {
				return usageErr(fmt.Errorf("provide --id at least once or --all-tagged"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if allTagged != "" {
				tagged, err := segmentIDsByTag(c, allTagged)
				if err != nil {
					return err
				}
				ids = append(ids, tagged...)
			}
			if len(ids) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"segments": []map[string]any{}, "message": "No matching segments found."}, flags)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("klaviyo-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			result, err := segmentVelocity(cmd.Context(), c, db, uniqueStrings(ids), last, interval)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringArrayVar(&ids, "id", nil, "Segment ID to snapshot; repeatable")
	cmd.Flags().StringVar(&allTagged, "all-tagged", "", "Snapshot all segments whose tag metadata includes this value")
	cmd.Flags().StringVar(&last, "last", "90d", "Lookback window")
	cmd.Flags().StringVar(&interval, "interval", "weekly", "Snapshot interval: daily or weekly")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	return cmd
}

func newFlowPathAnalysisCmd(flags *rootFlags) *cobra.Command {
	var id, last string
	cmd := &cobra.Command{
		Use:         "flow-path-analysis",
		Short:       "Compare conditional split branches inside a flow",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "id": id, "last": last, "planned_steps": []string{"fetch_flow_definition", "walk_split_branches", "query_message_metrics", "compare_conversion_rates"}}, flags)
			}
			if id == "" {
				return usageErr(fmt.Errorf("--id is required"))
			}
			c, since, until, err := clientAndWindow(flags, last)
			if err != nil {
				return err
			}
			result, err := flowPathAnalysis(c, id, since, until)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Flow ID")
	cmd.Flags().StringVar(&last, "last", "30d", "Lookback window")
	return cmd
}

func newCampaignTimeDecayCmd(flags *rootFlags) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:         "campaign-time-decay",
		Short:       "Bucket attributed conversions by hours after a campaign send",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "id": id, "planned_steps": []string{"fetch_campaign_send_time", "query_attributed_orders", "bucket_conversion_lag"}}, flags)
			}
			if id == "" {
				return usageErr(fmt.Errorf("--id is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := campaignTimeDecay(c, id)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Campaign ID")
	return cmd
}

func newListQualityScoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list-quality-score",
		Short:       "Score every list for engagement, hygiene, purchasing, and growth",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "planned_steps": []string{"fetch_lists", "query_list_metrics", "compute_weighted_quality_score"}}, flags)
			}
			c, since, until, err := clientAndWindow(flags, "30d")
			if err != nil {
				return err
			}
			result, err := listQualityScore(c, since, until)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func newContentFatigueCmd(flags *rootFlags) *cobra.Command {
	var dbPath, last string
	var minEmails int
	cmd := &cobra.Command{
		Use:         "content-fatigue",
		Short:       "Find profiles whose email engagement declined while sends continued",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "last": last, "min_emails": minEmails, "source": "local_store", "planned_steps": []string{"read_received_and_opened_events", "split_profile_history", "detect_decline_categories"}}, flags)
			}
			rows, err := readNovelLocalRows(cmd.Context(), dbPath, "events", 100000)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), localDataRequiredResult(), flags)
			}
			since, err := sinceFromLast(last)
			if err != nil {
				return usageErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), contentFatigue(rows, since, minEmails), flags)
		},
	}
	cmd.Flags().StringVar(&last, "last", "90d", "Analysis window")
	cmd.Flags().IntVar(&minEmails, "min-emails", 10, "Minimum received emails to qualify")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (defaults to local Klaviyo store)")
	return cmd
}

func readNovelLocalRows(ctx context.Context, dbPath, resourceType string, limit int) ([]resourceRow, error) {
	db, err := openNovelStore(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()
	return readResourceRows(ctx, db, resourceType, limit)
}

func localDataRequiredResult() map[string]any {
	return map[string]any{"message": syncFirstMessage}
}

func sinceFromLast(last string) (time.Time, error) {
	dur, err := parseNovelDuration(last)
	if err != nil || dur <= 0 {
		return time.Time{}, fmt.Errorf("--last must be a positive duration like 30d")
	}
	return time.Now().Add(-dur), nil
}

func parseNovelDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func titleASCII(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func eventsFromRows(rows []resourceRow, metric string, since time.Time) []novelEmailEvent {
	var out []novelEmailEvent
	for _, row := range rows {
		ev := eventFromRow(row)
		if metric != "" && !strings.EqualFold(ev.Metric, metric) {
			continue
		}
		if !since.IsZero() && (ev.Time.IsZero() || ev.Time.Before(since)) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

func eventFromRow(row resourceRow) novelEmailEvent {
	props := "data.attributes.properties."
	ev := novelEmailEvent{
		ProfileID:    rowValue(row, "profile_id", "data.relationships.profile.data.id", "relationships.profile.data.id", "data.attributes.profile_id", "attributes.profile_id", props+"profile_id", "attributes.properties.profile_id"),
		Email:        rowValue(row, "email", "data.attributes.email", "attributes.email", props+"email", props+"Email", "attributes.properties.email", "attributes.properties.Email"),
		Metric:       rowValue(row, "metric_name", "data.attributes.metric.name", "data.attributes.metric_name", "attributes.metric_name", "data.attributes.name", "attributes.name"),
		Time:         rowTime(row, "datetime", "timestamp", "data.attributes.datetime", "data.attributes.timestamp", "attributes.datetime", "attributes.timestamp"),
		FlowID:       rowValue(row, props+"$flow_id", props+"flow_id", "attributes.properties.$flow_id", "attributes.properties.flow_id", "data.relationships.flow.data.id", "relationships.flow.data.id"),
		FlowName:     rowValue(row, props+"$flow", props+"Flow Name", props+"flow", "attributes.properties.$flow", "attributes.properties.Flow Name", "attributes.properties.flow"),
		CampaignID:   rowValue(row, props+"$attributed_campaign_id", props+"campaign_id", "attributes.properties.$attributed_campaign_id", "attributes.properties.campaign_id", "data.relationships.campaign.data.id"),
		CampaignName: rowValue(row, props+"Campaign Name", props+"$attributed_campaign", props+"utm_campaign", "attributes.properties.Campaign Name", "attributes.properties.$attributed_campaign", "attributes.properties.utm_campaign"),
		MessageID:    rowValue(row, props+"$message_id", props+"message_id", "attributes.properties.$message_id", "attributes.properties.message_id"),
		MessageName:  rowValue(row, props+"$message", props+"Message Name", props+"message_name", "attributes.properties.$message", "attributes.properties.Message Name", "attributes.properties.message_name"),
		Subject:      rowValue(row, props+"Subject", props+"subject", "attributes.properties.Subject", "attributes.properties.subject"),
		Revenue:      rowFloat(row, "value", "data.attributes.value", props+"value", "attributes.value", "attributes.properties.value"),
	}
	if ev.ProfileID == "" && ev.Email != "" {
		ev.ProfileID = strings.ToLower(ev.Email)
	}
	return ev
}

func flowCannibalization(rows []resourceRow, window time.Duration, windowLabel string, since time.Time, period string) map[string]any {
	byProfile := map[string][]novelEmailEvent{}
	for _, ev := range eventsFromRows(rows, "Received Email", since) {
		if ev.ProfileID == "" || (ev.FlowID == "" && ev.FlowName == "") {
			continue
		}
		byProfile[ev.ProfileID] = append(byProfile[ev.ProfileID], ev)
	}
	type pairAgg struct {
		FlowA, FlowAID, FlowB, FlowBID string
		Collisions                     int
		Profiles                       map[string]bool
	}
	pairs := map[string]*pairAgg{}
	total := 0
	for profileID, events := range byProfile {
		sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
		for i := range events {
			for j := i + 1; j < len(events); j++ {
				if events[j].Time.Sub(events[i].Time) > window {
					break
				}
				a, b := events[i], events[j]
				keyA := firstNonEmptyString(a.FlowID, a.FlowName)
				keyB := firstNonEmptyString(b.FlowID, b.FlowName)
				if keyA == "" || keyB == "" || keyA == keyB {
					continue
				}
				if keyA > keyB {
					a, b = b, a
					keyA, keyB = keyB, keyA
				}
				key := keyA + "\x00" + keyB
				if pairs[key] == nil {
					pairs[key] = &pairAgg{FlowA: firstNonEmptyString(a.FlowName, a.FlowID), FlowAID: a.FlowID, FlowB: firstNonEmptyString(b.FlowName, b.FlowID), FlowBID: b.FlowID, Profiles: map[string]bool{}}
				}
				pairs[key].Collisions++
				pairs[key].Profiles[profileID] = true
				total++
			}
		}
	}
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{"flow_a": p.FlowA, "flow_a_id": p.FlowAID, "flow_b": p.FlowB, "flow_b_id": p.FlowBID, "collisions": p.Collisions, "affected_profiles": len(p.Profiles), "suggestion": "Add mutual exclusion filters or increase the delay between these flows."})
	}
	sort.Slice(out, func(i, j int) bool { return anyInt(out[i]["collisions"]) > anyInt(out[j]["collisions"]) })
	return map[string]any{"window": windowLabel, "period": period, "total_collisions": total, "pairs": out}
}

func sendFatigue(rows []resourceRow, threshold int, window time.Duration, windowLabel string, since time.Time) map[string]any {
	received := map[string][]novelEmailEvent{}
	lastOpen := map[string]time.Time{}
	for _, ev := range eventsFromRows(rows, "", since) {
		switch {
		case strings.EqualFold(ev.Metric, "Received Email") && ev.ProfileID != "":
			received[ev.ProfileID] = append(received[ev.ProfileID], ev)
		case strings.EqualFold(ev.Metric, "Opened Email") && ev.ProfileID != "":
			if ev.Time.After(lastOpen[ev.ProfileID]) {
				lastOpen[ev.ProfileID] = ev.Time
			}
		}
	}
	distribution := map[string]int{"1-2": 0, "3-4": 0, "5-7": 0, "8-10": 0, "11+": 0}
	var offenders []map[string]any
	for profileID, events := range received {
		maxCount := maxEventsInWindow(events, window)
		switch {
		case maxCount <= 0:
		case maxCount <= 2:
			distribution["1-2"]++
		case maxCount <= 4:
			distribution["3-4"]++
		case maxCount <= 7:
			distribution["5-7"]++
		case maxCount <= 10:
			distribution["8-10"]++
		default:
			distribution["11+"]++
		}
		if maxCount >= threshold {
			last := "never"
			if t := lastOpen[profileID]; !t.IsZero() {
				last = t.Format("2006-01-02")
			}
			email := ""
			for _, ev := range events {
				if ev.Email != "" {
					email = ev.Email
					break
				}
			}
			offenders = append(offenders, map[string]any{"profile_id": profileID, "email": email, "emails_received": maxCount, "last_open": last})
		}
	}
	sort.Slice(offenders, func(i, j int) bool {
		return anyInt(offenders[i]["emails_received"]) > anyInt(offenders[j]["emails_received"])
	})
	fatiguedCount := len(offenders)
	if len(offenders) > 25 {
		offenders = offenders[:25]
	}
	totalProfiles := len(received)
	pct := 0.0
	if totalProfiles > 0 {
		pct = float64(fatiguedCount) / float64(totalProfiles) * 100
	}
	return map[string]any{"threshold": threshold, "window": windowLabel, "distribution": distribution, "fatigued_profiles": fatiguedCount, "fatigued_percentage": round1(pct), "top_offenders": offenders, "recommendation": fmt.Sprintf("%d profiles received %d+ emails in %s. Consider frequency capping or exclusion segments.", fatiguedCount, threshold, windowLabel)}
}

func maxEventsInWindow(events []novelEmailEvent, window time.Duration) int {
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	maxCount, start := 0, 0
	for end := range events {
		for start <= end && events[end].Time.Sub(events[start].Time) > window {
			start++
		}
		if count := end - start + 1; count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

type subjectEmail struct {
	ID, Subject, Source, SourceType string
	OpenRate, ClickRate             float64
}

func subjectLineAnalysis(c flowClient, since, until time.Time, source string) (map[string]any, error) {
	emails, err := collectSubjectEmails(c, source)
	if err != nil {
		return nil, err
	}
	receivedID, err := resolveMetricID(c, "Received Email")
	if err != nil {
		return nil, err
	}
	openedID, err := resolveMetricID(c, "Opened Email")
	if err != nil {
		return nil, err
	}
	clickedID, err := resolveMetricID(c, "Clicked Email")
	if err != nil {
		return nil, err
	}
	received := keyedMetricValues(topAggregateRows(c, receivedID, "$message", "count", since, until, 0), "count")
	opened := keyedMetricValues(topAggregateRows(c, openedID, "$message", "count", since, until, 0), "count")
	clicked := keyedMetricValues(topAggregateRows(c, clickedID, "$message", "count", since, until, 0), "count")
	mergeMetricMap(received, keyedMetricValues(topAggregateRows(c, receivedID, "Campaign Name", "count", since, until, 0), "count"))
	mergeMetricMap(opened, keyedMetricValues(topAggregateRows(c, openedID, "Campaign Name", "count", since, until, 0), "count"))
	mergeMetricMap(clicked, keyedMetricValues(topAggregateRows(c, clickedID, "Campaign Name", "count", since, until, 0), "count"))
	mergeMetricMap(received, keyedMetricValues(topAggregateRows(c, receivedID, "$flow", "count", since, until, 0), "count"))
	mergeMetricMap(opened, keyedMetricValues(topAggregateRows(c, openedID, "$flow", "count", since, until, 0), "count"))
	mergeMetricMap(clicked, keyedMetricValues(topAggregateRows(c, clickedID, "$flow", "count", since, until, 0), "count"))
	for i := range emails {
		keys := []string{emails[i].ID, emails[i].Source, emails[i].Subject}
		sent := firstMetricValue(received, keys...)
		if sent > 0 {
			emails[i].OpenRate = clamp01(firstMetricValue(opened, keys...) / sent)
			emails[i].ClickRate = clamp01(firstMetricValue(clicked, keys...) / sent)
		}
	}
	patterns := subjectPatterns(emails)
	sort.Slice(emails, func(i, j int) bool { return emails[i].OpenRate > emails[j].OpenRate })
	top := subjectRows(emails, 5)
	bottomEmails := append([]subjectEmail(nil), emails...)
	sort.Slice(bottomEmails, func(i, j int) bool { return bottomEmails[i].OpenRate < bottomEmails[j].OpenRate })
	return map[string]any{"total_emails_analyzed": len(emails), "patterns": patterns, "top_subjects": top, "bottom_subjects": subjectRows(bottomEmails, 5)}, nil
}

func collectSubjectEmails(c flowClient, source string) ([]subjectEmail, error) {
	source = strings.ToLower(source)
	var out []subjectEmail
	if source == "all" || source == "flow" {
		flows, err := fetchAllJSONAPI(c, "/api/flows", map[string]string{"fields[flow]": "name,status", "page[size]": "50"}, 0)
		if err != nil {
			return nil, err
		}
		for _, flow := range flows {
			if status := strings.ToLower(stringFromMapPath(flow, "attributes.status")); status != "" && status != "live" {
				continue
			}
			flowID := fmt.Sprint(flow["id"])
			def, name, err := fetchAndTransformFlow(c, flowID, true)
			if err != nil {
				continue
			}
			for _, action := range mapSlice(def["actions"]) {
				if fmt.Sprint(action["type"]) != "send-email" {
					continue
				}
				msg, _ := anyPath(action, "data.message").(map[string]any)
				subject := firstNonEmptyString(fmt.Sprint(msg["subject"]), fmt.Sprint(anyPath(action, "data.message.content.subject")), fmt.Sprint(anyPath(action, "data.message.definition.content.subject")), fmt.Sprint(msg["name"]))
				if subject == "" || subject == "<nil>" {
					continue
				}
				out = append(out, subjectEmail{ID: fmt.Sprint(action["id"]), Subject: subject, Source: name, SourceType: "flow"})
			}
		}
	}
	if source == "all" || source == "campaign" {
		campaigns, err := fetchAllJSONAPI(c, "/api/campaigns", map[string]string{"filter": `equals(messages.channel,'email')`, "fields[campaign]": "name,status,send_time", "page[size]": "50"}, 200)
		if err != nil {
			return nil, err
		}
		for _, campaign := range campaigns {
			campaignID := fmt.Sprint(campaign["id"])
			name := stringFromMapPath(campaign, "attributes.name")
			messages, err := fetchAllJSONAPI(c, "/api/campaigns/"+url.PathEscape(campaignID)+"/campaign-messages", map[string]string{}, 0)
			if err != nil {
				continue
			}
			for _, msg := range messages {
				subject := firstNonEmptyString(stringFromMapPath(msg, "attributes.subject"), stringFromMapPath(msg, "attributes.definition.content.subject"), stringFromMapPath(msg, "attributes.definition.label"), stringFromMapPath(msg, "attributes.label"), name)
				out = append(out, subjectEmail{ID: fmt.Sprint(msg["id"]), Subject: subject, Source: name, SourceType: "campaign"})
			}
		}
	}
	return out, nil
}

func subjectPatterns(emails []subjectEmail) []map[string]any {
	checks := map[string]func(string) bool{
		"has_emoji": hasEmoji,
		"has_personalization": func(s string) bool {
			l := strings.ToLower(s)
			return strings.Contains(s, "{{") || strings.Contains(l, "first_name")
		},
		"length_under_30": func(s string) bool { return len([]rune(s)) < 30 },
		"length_30_50":    func(s string) bool { n := len([]rune(s)); return n >= 30 && n <= 50 },
		"length_50_70":    func(s string) bool { n := len([]rune(s)); return n > 50 && n <= 70 },
		"length_over_70":  func(s string) bool { return len([]rune(s)) > 70 },
		"ends_question":   func(s string) bool { return strings.HasSuffix(strings.TrimSpace(s), "?") },
		"has_number":      func(s string) bool { return strings.IndexFunc(s, unicode.IsDigit) >= 0 },
		"has_urgency": func(s string) bool {
			l := strings.ToLower(s)
			return strings.Contains(l, "last") || strings.Contains(l, "final") || strings.Contains(l, "ends") || strings.Contains(l, "expires") || strings.Contains(l, "limited")
		},
		"curiosity_gap": func(s string) bool {
			l := strings.ToLower(s)
			return strings.Contains(s, "...") || strings.Contains(l, "this") || strings.Contains(l, "here's")
		},
	}
	var rows []map[string]any
	for name, check := range checks {
		var with, without []float64
		for _, email := range emails {
			if check(email.Subject) {
				with = append(with, email.OpenRate)
			} else {
				without = append(without, email.OpenRate)
			}
		}
		wAvg, woAvg := avg(with), avg(without)
		lift := 0.0
		if woAvg > 0 {
			lift = (wAvg - woAvg) / woAvg * 100
		}
		rows = append(rows, map[string]any{"pattern": name, "with": map[string]any{"count": len(with), "avg_open_rate": round3(wAvg)}, "without": map[string]any{"count": len(without), "avg_open_rate": round3(woAvg)}, "lift": fmt.Sprintf("%+.1f%%", lift), "significant": len(with) >= 5 && len(without) >= 5})
	}
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i]["pattern"]) < fmt.Sprint(rows[j]["pattern"]) })
	return rows
}

func subjectRows(emails []subjectEmail, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, email := range emails {
		rows = append(rows, map[string]any{"subject": email.Subject, "open_rate": round3(email.OpenRate), "click_rate": round3(email.ClickRate), "source": email.Source, "source_type": email.SourceType})
		if len(rows) == limit {
			break
		}
	}
	return rows
}

func optimalSendTime(c flowClient, since, until time.Time, timezoneName, metric string) (map[string]any, error) {
	metricName := "Opened Email"
	if strings.EqualFold(metric, "clicked") {
		metricName = "Clicked Email"
	}
	metricID, err := resolveMetricID(c, metricName)
	if err != nil {
		return nil, err
	}
	receivedID, err := resolveMetricID(c, "Received Email")
	if err != nil {
		return nil, err
	}
	engagementRaw, _, err := c.Post("/api/metric-aggregates", metricAggregateBodyWithInterval(metricID, []string{"count"}, nil, since, until, "hour", timezoneName))
	if err != nil {
		return nil, classifyAPIError(err)
	}
	receivedRaw, _, err := c.Post("/api/metric-aggregates", metricAggregateBodyWithInterval(receivedID, []string{"count"}, nil, since, until, "hour", timezoneName))
	if err != nil {
		return nil, classifyAPIError(err)
	}
	engagement := metricTimeSeries(engagementRaw, "count")
	received := metricTimeSeries(receivedRaw, "count")
	loc := loadNovelLocation(timezoneName)
	type agg struct{ engagement, received float64 }
	grid := map[string]map[int]*agg{}
	for t, e := range engagement {
		local := t.In(loc)
		day := strings.ToLower(local.Weekday().String())
		hour := local.Hour()
		if grid[day] == nil {
			grid[day] = map[int]*agg{}
		}
		if grid[day][hour] == nil {
			grid[day][hour] = &agg{}
		}
		grid[day][hour].engagement += e
		grid[day][hour].received += received[t]
	}
	heatmap := map[string][]float64{}
	var windows []map[string]any
	for _, day := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
		heatmap[day] = make([]float64, 24)
		for hour := 0; hour < 24; hour++ {
			a := grid[day][hour]
			rate := 0.0
			if a != nil && a.received > 0 {
				rate = clamp01(a.engagement / a.received)
			}
			heatmap[day][hour] = round3(rate)
			windows = append(windows, map[string]any{"day": titleASCII(day), "hour": hour, "engagement_rate": round3(rate)})
		}
	}
	sort.Slice(windows, func(i, j int) bool {
		return anyFloat(windows[i]["engagement_rate"]) > anyFloat(windows[j]["engagement_rate"])
	})
	best := append([]map[string]any(nil), windows[:minInt(3, len(windows))]...)
	worst := append([]map[string]any(nil), windows...)
	sort.Slice(worst, func(i, j int) bool {
		return anyFloat(worst[i]["engagement_rate"]) < anyFloat(worst[j]["engagement_rate"])
	})
	worst = worst[:minInt(3, len(worst))]
	rec := "Not enough delivery-normalized engagement data to recommend a send window."
	if len(best) > 0 && anyFloat(best[0]["engagement_rate"]) > 0 {
		rec = fmt.Sprintf("Best send window: %s around %d:00 %s.", best[0]["day"], anyInt(best[0]["hour"]), timezoneName)
	}
	return map[string]any{"metric": metricName, "timezone": timezoneName, "best_windows": best, "worst_windows": worst, "heatmap": heatmap, "recommendation": rec}, nil
}

func revenuePerEmail(c flowClient, since, until time.Time, by, period string) (map[string]any, error) {
	placedID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, err
	}
	receivedID, err := resolveMetricID(c, "Received Email")
	if err != nil {
		return nil, err
	}
	result := map[string]any{"period": period, "flows": []map[string]any{}, "campaigns": []map[string]any{}}
	if by == "all" || by == "flow" {
		revenue := keyedMetricValues(topAggregateRows(c, placedID, "$attributed_flow", "sum_value", since, until, 0), "sum_value")
		sends := keyedMetricValues(topAggregateRows(c, receivedID, "$flow", "count", since, until, 0), "count")
		result["flows"] = revenuePerEmailRows(revenue, sends, "flow_id")
	}
	if by == "all" || by == "campaign" {
		revenue := keyedMetricValues(topAggregateRows(c, placedID, "Campaign Name", "sum_value", since, until, 0), "sum_value")
		sends := keyedMetricValues(topAggregateRows(c, receivedID, "Campaign Name", "count", since, until, 0), "count")
		result["campaigns"] = revenuePerEmailRows(revenue, sends, "campaign_id")
	}
	result["insight"] = revenuePerEmailInsight(result)
	return result, nil
}

func revenuePerEmailRows(revenue, sends map[string]float64, idField string) []map[string]any {
	keys := map[string]bool{}
	for k := range revenue {
		keys[k] = true
	}
	for k := range sends {
		keys[k] = true
	}
	var rows []map[string]any
	for key := range keys {
		sent := sends[key]
		rpe := 0.0
		if sent > 0 {
			rpe = revenue[key] / sent
		}
		rows = append(rows, map[string]any{"name": key, idField: key, "revenue": round2(revenue[key]), "emails_sent": int(sent), "revenue_per_email": round3(rpe)})
	}
	sort.Slice(rows, func(i, j int) bool {
		return anyFloat(rows[i]["revenue_per_email"]) > anyFloat(rows[j]["revenue_per_email"])
	})
	for i := range rows {
		rows[i]["rank"] = i + 1
	}
	return rows
}

func segmentIDsByTag(c flowClient, tag string) ([]string, error) {
	segments, err := fetchAllJSONAPI(c, "/api/segments", map[string]string{"fields[segment]": "name,tags,profile_count", "page[size]": "50"}, 0)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, segment := range segments {
		if strings.Contains(strings.ToLower(fmt.Sprint(segment)), strings.ToLower(tag)) {
			ids = append(ids, fmt.Sprint(segment["id"]))
		}
	}
	return ids, nil
}

func segmentVelocity(ctx context.Context, c flowClient, db *store.Store, ids []string, last, interval string) (map[string]any, error) {
	if _, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS segment_snapshots (segment_id TEXT NOT NULL, snapshot_date TEXT NOT NULL, name TEXT, count INTEGER NOT NULL, recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(segment_id, snapshot_date))`); err != nil {
		return nil, err
	}
	sinceTime, err := sinceFromLast(last)
	if err != nil {
		return nil, err
	}
	snapshotDate := segmentSnapshotDate(time.Now(), interval)
	minSnapshotDate := segmentSnapshotDate(sinceTime, interval)
	var results []map[string]any
	for _, id := range ids {
		resp, err := c.Get("/api/segments/"+url.PathEscape(id), map[string]string{"fields[segment]": "name,profile_count,profile_count_estimate"})
		if err != nil {
			return nil, classifyAPIError(err)
		}
		name := firstString(resp, "data.attributes.name")
		count := anyInt(firstJSONValue(resp, "data.attributes.profile_count", "data.attributes.profile_count_estimate", "data.attributes.profiles_count"))
		if _, err := db.DB().ExecContext(ctx, `INSERT OR REPLACE INTO segment_snapshots(segment_id, snapshot_date, name, count) VALUES (?, ?, ?, ?)`, id, snapshotDate, name, count); err != nil {
			return nil, err
		}
		rows, err := db.DB().QueryContext(ctx, `SELECT snapshot_date, count FROM segment_snapshots WHERE segment_id = ? AND snapshot_date >= ? ORDER BY snapshot_date`, id, minSnapshotDate)
		if err != nil {
			return nil, err
		}
		var trend []int
		var firstCount int
		for rows.Next() {
			var date string
			var n int
			if err := rows.Scan(&date, &n); err != nil {
				rows.Close()
				return nil, err
			}
			if len(trend) == 0 {
				firstCount = n
			}
			trend = append(trend, n)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		change := count - firstCount
		pct := 0.0
		if firstCount > 0 {
			pct = float64(change) / float64(firstCount) * 100
		}
		direction := "stable"
		if pct > 2 {
			direction = "growing"
		} else if pct < -2 {
			direction = "shrinking"
		}
		row := map[string]any{"name": name, "segment_id": id, "current_size": count, "size_" + strings.TrimSuffix(last, "d") + "d_ago": firstCount, "change": change, "change_pct": round1(pct), "direction": direction, "weekly_trend": trend}
		if len(trend) < 3 {
			row["message"] = "baseline recorded, run again later for trend data"
		}
		results = append(results, row)
	}
	return map[string]any{"segments": results}, nil
}

func segmentSnapshotDate(t time.Time, interval string) string {
	if interval == "weekly" {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	}
	return t.Format("2006-01-02")
}

func flowPathAnalysis(c flowClient, flowID string, since, until time.Time) (map[string]any, error) {
	def, flowName, err := fetchAndTransformFlow(c, flowID, true)
	if err != nil {
		return nil, err
	}
	stats, _ := messageStats(c, since, until)
	actions := mapSlice(def["actions"])
	actionByID := map[string]map[string]any{}
	for _, action := range actions {
		actionByID[fmt.Sprint(action["id"])] = action
	}
	var perEmail []map[string]any
	for _, action := range actions {
		if fmt.Sprint(action["type"]) != "send-email" {
			continue
		}
		id := fmt.Sprint(action["id"])
		name := firstNonEmptyString(fmt.Sprint(anyPath(action, "data.message.name")), fmt.Sprint(anyPath(action, "data.message.subject")), id)
		row := statsForKeys(stats, id, name)
		row["name"] = name
		row["action_id"] = id
		perEmail = append(perEmail, row)
	}
	var splits []map[string]any
	for _, action := range actions {
		if !strings.Contains(fmt.Sprint(action["type"]), "split") {
			continue
		}
		id := fmt.Sprint(action["id"])
		trueIDs := branchSendActions(actionByID, fmt.Sprint(anyPath(action, "links.next_if_true")))
		falseIDs := branchSendActions(actionByID, fmt.Sprint(anyPath(action, "links.next_if_false")))
		trueBranch := rollupBranch(stats, trueIDs, "True branch")
		falseBranch := rollupBranch(stats, falseIDs, "False branch")
		insight := branchInsight(trueBranch, falseBranch)
		splits = append(splits, map[string]any{"action_id": id, "split_type": fmt.Sprint(action["type"]), "true_branch": trueBranch, "false_branch": falseBranch, "insight": insight})
	}
	sort.Slice(perEmail, func(i, j int) bool { return fmt.Sprint(perEmail[i]["name"]) < fmt.Sprint(perEmail[j]["name"]) })
	return map[string]any{"flow": flowName, "flow_id": flowID, "splits": splits, "per_email_stats": perEmail, "dropoff": conversionDropoff(perEmail)}, nil
}

func campaignTimeDecay(c flowClient, campaignID string) (map[string]any, error) {
	resp, err := c.Get("/api/campaigns/"+url.PathEscape(campaignID), map[string]string{"fields[campaign]": "name,send_time,scheduled_at,updated"})
	if err != nil {
		return nil, classifyAPIError(err)
	}
	name := firstString(resp, "data.attributes.name")
	sentAt := parseDate(firstString(resp, "data.attributes.send_time", "data.attributes.scheduled_at", "data.attributes.updated"))
	if sentAt.IsZero() {
		return nil, fmt.Errorf("campaign %s does not include a usable send timestamp", campaignID)
	}
	metricID, err := resolveMetricID(c, "Placed Order")
	if err != nil {
		return nil, err
	}
	events, err := fetchMetricEvents(c, metricID, sentAt, sentAt.Add(7*24*time.Hour), 1000)
	if err != nil {
		return nil, err
	}
	buckets := []string{"0-1h", "1-6h", "6-24h", "24-72h", "3-5d", "5-7d"}
	curve := map[string]map[string]any{}
	for _, b := range buckets {
		curve[b] = map[string]any{"conversions": 0, "revenue": 0.0, "cumulative_pct": 0.0}
	}
	totalRevenue := 0.0
	totalConversions := 0
	for _, event := range events {
		if !campaignEventMatches(event, campaignID, name) {
			continue
		}
		delta := event.Time.Sub(sentAt)
		b := decayBucket(delta)
		if b == "" {
			continue
		}
		curve[b]["conversions"] = anyInt(curve[b]["conversions"]) + 1
		curve[b]["revenue"] = anyFloat(curve[b]["revenue"]) + event.Revenue
		totalConversions++
		totalRevenue += event.Revenue
	}
	cumulative := 0
	for _, b := range buckets {
		cumulative += anyInt(curve[b]["conversions"])
		if totalConversions > 0 {
			curve[b]["cumulative_pct"] = round1(float64(cumulative) / float64(totalConversions) * 100)
		}
		curve[b]["revenue"] = round2(anyFloat(curve[b]["revenue"]))
	}
	return map[string]any{"campaign": name, "campaign_id": campaignID, "sent_at": sentAt.Format(time.RFC3339), "total_conversions": totalConversions, "total_revenue": round2(totalRevenue), "decay_curve": curve, "insight": campaignDecayInsight(curve)}, nil
}

func campaignEventMatches(event novelEmailEvent, campaignID, campaignName string) bool {
	if campaignID == "" {
		return true
	}
	if event.CampaignID != "" {
		return event.CampaignID == campaignID
	}
	if event.CampaignName != "" {
		return strings.EqualFold(event.CampaignName, campaignName)
	}
	return false
}

func listQualityScore(c flowClient, since, until time.Time) (map[string]any, error) {
	lists, err := fetchAllJSONAPI(c, "/api/lists", map[string]string{"fields[list]": "name,created,updated", "page[size]": "10"}, 0)
	if err != nil {
		return nil, err
	}
	metrics := map[string]string{"received": "Received Email", "opened": "Opened Email", "clicked": "Clicked Email", "bounced": "Bounced Email", "unsub": "Unsubscribed Email", "spam": "Marked Email as Spam", "orders": "Placed Order"}
	values := map[string]map[string]float64{}
	for key, name := range metrics {
		id, err := resolveMetricID(c, name)
		if err != nil {
			values[key] = map[string]float64{}
			continue
		}
		values[key] = keyedMetricValues(topAggregateRows(c, id, "List ID", "count", since, until, 0), "count")
	}
	var rows []map[string]any
	for _, list := range lists {
		id := fmt.Sprint(list["id"])
		name := stringFromMapPath(list, "attributes.name")
		size := anyInt(firstNonEmptyString(stringFromMapPath(list, "attributes.profile_count"), stringFromMapPath(list, "attributes.profiles_count")))
		received := values["received"][id]
		engagement := 0.0
		if received > 0 {
			engagement = (values["opened"][id] + values["clicked"][id]) / received
		}
		bounce, unsub, spam := rate(values["bounced"][id], received), rate(values["unsub"][id], received), rate(values["spam"][id], received)
		purchase := rate(values["orders"][id], float64(maxInt(size, 1)))
		growth := 0.0
		score := qualityScore(engagement, bounce, unsub, spam, purchase, growth)
		issues := listQualityIssues(bounce, unsub, spam, growth)
		rows = append(rows, map[string]any{"name": name, "list_id": id, "size": size, "quality_score": score, "grade": qualityGrade(score), "engagement_rate": round3(engagement), "bounce_rate": round3(bounce), "unsub_rate": round3(unsub), "spam_rate": round4(spam), "purchase_rate": round3(purchase), "growth_rate": round3(growth), "issues": issues})
	}
	sort.Slice(rows, func(i, j int) bool { return anyInt(rows[i]["quality_score"]) > anyInt(rows[j]["quality_score"]) })
	return map[string]any{"lists": rows}, nil
}

func contentFatigue(rows []resourceRow, since time.Time, minEmails int) map[string]any {
	byProfile := map[string][]novelEmailEvent{}
	opened := map[string]map[string]bool{}
	for _, ev := range eventsFromRows(rows, "", since) {
		if ev.ProfileID == "" {
			continue
		}
		if strings.EqualFold(ev.Metric, "Received Email") {
			byProfile[ev.ProfileID] = append(byProfile[ev.ProfileID], ev)
		}
		if strings.EqualFold(ev.Metric, "Opened Email") {
			if opened[ev.ProfileID] == nil {
				opened[ev.ProfileID] = map[string]bool{}
			}
			opened[ev.ProfileID][ev.MessageID+ev.Time.Format(time.RFC3339)] = true
		}
	}
	categories := map[string]int{"recent_fatigue": 0, "gradual_decline": 0, "sudden_drop": 0}
	triggers := map[string]int{}
	analyzed, fatigued := 0, 0
	for profileID, events := range byProfile {
		if len(events) < minEmails {
			continue
		}
		analyzed++
		sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
		mid := len(events) / 2
		firstRate := profileOpenRate(events[:mid], opened[profileID])
		secondRate := profileOpenRate(events[mid:], opened[profileID])
		if firstRate <= 0.15 || secondRate >= 0.05 {
			continue
		}
		fatigued++
		category := "gradual_decline"
		if len(events) > 0 && events[len(events)-1].Time.After(time.Now().AddDate(0, 0, -30)) {
			category = "recent_fatigue"
		}
		lastOpened := ""
		for i := len(events) - 1; i >= 0; i-- {
			if opened[profileID][events[i].MessageID+events[i].Time.Format(time.RFC3339)] {
				lastOpened = firstNonEmptyString(events[i].Subject, events[i].MessageName, events[i].CampaignName, events[i].FlowName)
				break
			}
		}
		if lastOpened != "" && secondRate == 0 {
			category = "sudden_drop"
			triggers[lastOpened]++
		}
		categories[category]++
	}
	var triggerRows []map[string]any
	for name, count := range triggers {
		triggerRows = append(triggerRows, map[string]any{"last_opened_email": name, "profiles_affected": count, "interpretation": fmt.Sprintf("%d profiles disengaged after this email; review content, cadence, and audience fit.", count)})
	}
	sort.Slice(triggerRows, func(i, j int) bool {
		return anyInt(triggerRows[i]["profiles_affected"]) > anyInt(triggerRows[j]["profiles_affected"])
	})
	if len(triggerRows) > 10 {
		triggerRows = triggerRows[:10]
	}
	rate := 0.0
	if analyzed > 0 {
		rate = float64(fatigued) / float64(analyzed) * 100
	}
	return map[string]any{"total_profiles_analyzed": analyzed, "fatigued_profiles": fatigued, "fatigue_rate": round1(rate), "categories": categories, "sudden_drop_triggers": triggerRows, "recommendation": fmt.Sprintf("%d profiles are content-fatigued. Create a re-engagement segment for recent non-openers before they fully lapse.", fatigued)}
}

func metricAggregateBodyWithInterval(metricID string, measurements []string, by []string, since, until time.Time, interval, timezoneName string) map[string]any {
	body := metricAggregateBody(metricID, measurements, by, since, until)
	attrs, _ := anyPath(body, "data.attributes").(map[string]any)
	attrs["interval"] = interval
	attrs["timezone"] = timezoneName
	return body
}

func metricTimeSeries(raw json.RawMessage, measurement string) map[time.Time]float64 {
	var parsed map[string]any
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	dates := anySlice(anyPath(parsed, "data.attributes.dates"))
	results := anySlice(anyPath(parsed, "data.attributes.data"))
	out := map[time.Time]float64{}
	for _, item := range results {
		row, _ := item.(map[string]any)
		meas, _ := row["measurements"].(map[string]any)
		vals := anySlice(meas[measurement])
		for i, v := range vals {
			if i >= len(dates) {
				continue
			}
			t, err := time.Parse(time.RFC3339, fmt.Sprint(dates[i]))
			if err != nil {
				t = parseDate(fmt.Sprint(dates[i]))
			}
			if !t.IsZero() {
				out[t] += anyFloat(v)
			}
		}
	}
	return out
}

func messageStats(c flowClient, since, until time.Time) (map[string]map[string]float64, error) {
	names := map[string]string{"sent": "Received Email", "opened": "Opened Email", "clicked": "Clicked Email", "converted": "Placed Order"}
	out := map[string]map[string]float64{}
	for field, metric := range names {
		id, err := resolveMetricID(c, metric)
		if err != nil {
			continue
		}
		for k, v := range keyedMetricValues(topAggregateRows(c, id, "$message", "count", since, until, 0), "count") {
			if out[k] == nil {
				out[k] = map[string]float64{}
			}
			out[k][field] = v
		}
	}
	if placedID, err := resolveMetricID(c, "Placed Order"); err == nil {
		for k, v := range keyedMetricValues(topAggregateRows(c, placedID, "$message", "sum_value", since, until, 0), "sum_value") {
			if out[k] == nil {
				out[k] = map[string]float64{}
			}
			out[k]["revenue"] = v
		}
	}
	return out, nil
}

func statsForKeys(stats map[string]map[string]float64, keys ...string) map[string]any {
	var s map[string]float64
	for _, key := range keys {
		if stats[key] != nil {
			s = stats[key]
			break
		}
	}
	if s == nil {
		s = map[string]float64{}
	}
	row := map[string]any{"sent": int(s["sent"]), "opened": int(s["opened"]), "clicked": int(s["clicked"]), "converted": int(s["converted"]), "revenue": round2(s["revenue"])}
	if s["sent"] > 0 {
		row["conversion_rate"] = round3(s["converted"] / s["sent"])
	}
	return row
}

func branchSendActions(actions map[string]map[string]any, start string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(string)
	walk = func(id string) {
		if id == "" || id == "<nil>" || seen[id] {
			return
		}
		seen[id] = true
		action := actions[id]
		if action == nil {
			return
		}
		if fmt.Sprint(action["type"]) == "send-email" {
			out = append(out, id)
		}
		for _, key := range []string{"links.next", "links.next_if_true", "links.next_if_false"} {
			walk(fmt.Sprint(anyPath(action, key)))
		}
	}
	walk(start)
	return out
}

func rollupBranch(stats map[string]map[string]float64, ids []string, label string) map[string]any {
	total := map[string]float64{}
	for _, id := range ids {
		for k, v := range stats[id] {
			total[k] += v
		}
	}
	rate := 0.0
	if total["sent"] > 0 {
		rate = total["converted"] / total["sent"]
	}
	return map[string]any{"label": label, "entered": int(total["sent"]), "converted": int(total["converted"]), "conversion_rate": round3(rate), "revenue": round2(total["revenue"])}
}

func fetchMetricEvents(c flowClient, metricID string, since, until time.Time, limit int) ([]novelEmailEvent, error) {
	filter := fmt.Sprintf("equals(metric_id,\"%s\"),greater-or-equal(datetime,%s),less-than(datetime,%s)", metricID, since.Format(time.RFC3339), until.Format(time.RFC3339))
	items, err := fetchAllJSONAPI(c, "/api/events", map[string]string{"filter": filter, "include": "metric,profile", "page[size]": "200", "sort": "datetime"}, limit)
	if err != nil {
		return nil, err
	}
	var out []novelEmailEvent
	for _, item := range items {
		b, _ := json.Marshal(map[string]any{"data": item})
		var data map[string]any
		_ = json.Unmarshal(b, &data)
		out = append(out, eventFromRow(resourceRow{ID: fmt.Sprint(item["id"]), Data: data}))
	}
	return out, nil
}

func firstJSONValue(raw json.RawMessage, paths ...string) any {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	for _, path := range paths {
		if got := anyPath(v, path); got != nil {
			return got
		}
	}
	return nil
}

func firstMetricValue(values map[string]float64, keys ...string) float64 {
	for _, key := range keys {
		if values[key] != 0 {
			return values[key]
		}
	}
	return 0
}

func mergeMetricMap(dst, src map[string]float64) {
	for k, v := range src {
		if dst[k] == 0 {
			dst[k] = v
		}
	}
}

func hasEmoji(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.So) || (r >= 0x1F300 && r <= 0x1FAFF) {
			return true
		}
	}
	return false
}

func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func loadNovelLocation(name string) *time.Location {
	load := name
	if strings.EqualFold(name, "US/Central") {
		load = "America/Chicago"
	}
	loc, err := time.LoadLocation(load)
	if err != nil {
		return time.Local
	}
	return loc
}

func decayBucket(d time.Duration) string {
	switch {
	case d < 0:
		return ""
	case d <= time.Hour:
		return "0-1h"
	case d <= 6*time.Hour:
		return "1-6h"
	case d <= 24*time.Hour:
		return "6-24h"
	case d <= 72*time.Hour:
		return "24-72h"
	case d <= 5*24*time.Hour:
		return "3-5d"
	case d <= 7*24*time.Hour:
		return "5-7d"
	default:
		return ""
	}
}

func campaignDecayInsight(curve map[string]map[string]any) string {
	if anyFloat(curve["6-24h"]["cumulative_pct"]) >= 75 {
		return "Most conversions arrive within 24 hours. Consider a shorter attribution review window for campaign decisions."
	}
	return "Conversions are spread across several days. Keep the full attribution window when judging this campaign."
}

func branchInsight(a, b map[string]any) string {
	ar, br := anyFloat(a["conversion_rate"]), anyFloat(b["conversion_rate"])
	if ar == 0 && br == 0 {
		return "No conversion difference detected from available message-level aggregates."
	}
	if ar >= br {
		return fmt.Sprintf("%s converts at %.1fx the alternate branch.", a["label"], ratio(ar, br))
	}
	return fmt.Sprintf("%s converts at %.1fx the alternate branch.", b["label"], ratio(br, ar))
}

func conversionDropoff(rows []map[string]any) string {
	if len(rows) == 0 {
		return "No send-email actions found in the flow definition."
	}
	total := 0
	for _, row := range rows {
		total += anyInt(row["converted"])
	}
	if total == 0 {
		return "No attributed conversions found for flow messages in this window."
	}
	first := anyInt(rows[0]["converted"])
	return fmt.Sprintf("%.0f%% of conversions happen on the first email in the available action order.", float64(first)/float64(total)*100)
}

func revenuePerEmailInsight(result map[string]any) string {
	var top map[string]any
	for _, group := range []string{"flows", "campaigns"} {
		rows, _ := result[group].([]map[string]any)
		if len(rows) > 0 && (top == nil || anyFloat(rows[0]["revenue_per_email"]) > anyFloat(top["revenue_per_email"])) {
			top = rows[0]
		}
	}
	if top == nil {
		return "No revenue-per-email rows were available for this period."
	}
	return fmt.Sprintf("%s has the strongest revenue per email at $%.3f.", top["name"], anyFloat(top["revenue_per_email"]))
}

func qualityScore(engagement, bounce, unsub, spam, purchase, growth float64) int {
	score := 0.0
	score += clamp01(engagement/0.30) * 30
	score += clamp01(1-bounce/0.05) * 20
	score += clamp01(1-unsub/0.02) * 15
	score += clamp01(1-spam/0.001) * 15
	score += clamp01(purchase/0.10) * 10
	score += clamp01((growth+0.05)/0.15) * 10
	return int(math.Round(score))
}

func listQualityIssues(bounce, unsub, spam, growth float64) []string {
	issues := []string{}
	if bounce > 0.05 {
		issues = append(issues, fmt.Sprintf("bounce rate critical (%.1f%%)", bounce*100))
	} else if bounce > 0.01 {
		issues = append(issues, "bounce rate slightly elevated - check for stale addresses")
	}
	if unsub > 0.01 {
		issues = append(issues, "unsubscribe rate elevated")
	}
	if spam > 0.001 {
		issues = append(issues, "spam complaints above safe threshold")
	}
	if growth < 0 {
		issues = append(issues, "list is shrinking")
	}
	return issues
}

func qualityGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B+"
	case score >= 70:
		return "B"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

func profileOpenRate(events []novelEmailEvent, opened map[string]bool) float64 {
	if len(events) == 0 {
		return 0
	}
	count := 0
	for _, ev := range events {
		if opened[ev.MessageID+ev.Time.Format(time.RFC3339)] {
			count++
		}
	}
	return float64(count) / float64(len(events))
}

func rate(n, d float64) float64 {
	if d <= 0 {
		return 0
	}
	return n / d
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return a
	}
	return a / b
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
