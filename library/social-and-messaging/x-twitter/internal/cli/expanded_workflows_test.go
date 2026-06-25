// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

func TestExpandedWorkflowCommandsAreWired(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{
		{"monitor", "create"},
		{"monitor", "run"},
		{"brief"},
		{"account", "snapshot"},
		{"url", "mentions"},
		{"performance", "snapshot"},
		{"performance", "backfill"},
		{"performance", "analyze"},
		{"timeline", "export"},
	} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("RootCmd missing %v: cmd=%v err=%v", path, cmd, err)
		}
	}
}

func TestBuildMonitorDefinition(t *testing.T) {
	def, err := buildMonitorDefinition("launch", "", "https://Example.com/docs?q=1", "")
	if err != nil {
		t.Fatalf("build url monitor: %v", err)
	}
	if def.Kind != "url" || def.Query != `url:"example.com/docs"` || def.SourceURL != "example.com/docs" {
		t.Fatalf("url monitor = %+v", def)
	}
	def, err = buildMonitorDefinition("founder", "", "", "@sama")
	if err != nil {
		t.Fatalf("build account monitor: %v", err)
	}
	if def.Query != "from:sama" || def.Account != "sama" {
		t.Fatalf("account monitor = %+v", def)
	}
	if _, err := buildMonitorDefinition("bad", "ai", "example.com", ""); err == nil {
		t.Fatal("buildMonitorDefinition accepted multiple monitor sources")
	}
}

func TestMonitorResultsDedupeAndBriefItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	def := monitorDefinition{Name: "launch", Kind: "query", Query: "launch"}
	if err := upsertMonitorDefinition(cmd, db, def); err != nil {
		t.Fatalf("upsert monitor: %v", err)
	}
	rec := &resolvedPostRecord{TweetID: "12345", URL: "https://x.com/i/web/status/12345", Text: "launch day", Source: "live", PublicMetrics: map[string]any{"like_count": float64(7)}}
	added, err := saveMonitorResult(cmd, db, "launch", rec, "2026-01-01T00:00:00Z")
	if err != nil || !added {
		t.Fatalf("first save added=%v err=%v", added, err)
	}
	added, err = saveMonitorResult(cmd, db, "launch", rec, "2026-01-01T00:01:00Z")
	if err != nil || added {
		t.Fatalf("duplicate save added=%v err=%v", added, err)
	}
	exists, err := monitorResultExists(cmd, db, "launch", rec.TweetID)
	if err != nil || !exists {
		t.Fatalf("monitorResultExists existing=%v err=%v", exists, err)
	}
	exists, err = monitorResultExists(cmd, db, "launch", "99999")
	if err != nil || exists {
		t.Fatalf("monitorResultExists missing=%v err=%v", exists, err)
	}
	items, err := listMonitorResultItems(cmd, db, "launch", "", 10)
	if err != nil {
		t.Fatalf("list monitor results: %v", err)
	}
	if len(items) != 1 || items[0].TweetID != "12345" || items[0].Text != "launch day" {
		t.Fatalf("items = %+v", items)
	}
	brief := buildBrief("local", "24h", items)
	if brief.ItemCount != 1 || len(brief.Highlights) != 1 || !strings.Contains(brief.Highlights[0].Reason, "Recent source item") {
		t.Fatalf("brief = %+v", brief)
	}
}

func TestListMonitorResultItemsAllowsNullSourceURL(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	def := monitorDefinition{Name: "launch", Kind: "query", Query: "launch"}
	if err := upsertMonitorDefinition(cmd, db, def); err != nil {
		t.Fatalf("upsert monitor: %v", err)
	}
	raw := `{"id":"12345","text":"launch day","created_at":"2026-01-01T00:00:00Z"}`
	if _, err := db.DB().ExecContext(cmd.Context(),
		`INSERT INTO workflow_monitor_results(monitor_name, tweet_id, tweet_json, source_url, seen_at, run_id)
		 VALUES(?, ?, ?, NULL, ?, ?)`,
		"launch", "12345", raw, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert monitor result: %v", err)
	}
	items, err := listMonitorResultItems(cmd, db, "launch", "", 10)
	if err != nil {
		t.Fatalf("list monitor results: %v", err)
	}
	if len(items) != 1 || items[0].TweetID != "12345" || items[0].Text != "launch day" {
		t.Fatalf("items = %+v", items)
	}
}

func TestNormalizeAccountProfile(t *testing.T) {
	profile, err := normalizeAccountProfile(json.RawMessage(`{
		"id":"42",
		"username":"alice",
		"name":"Alice",
		"description":"builds things",
		"public_metrics":{"followers_count":10},
		"pinned_tweet_id":"999"
	}`), "local", "synced", false)
	if err != nil {
		t.Fatalf("normalizeAccountProfile: %v", err)
	}
	if profile.ProfileURL != "https://x.com/alice" || profile.PinnedTweetID != "999" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestPerformanceSnapshotsAnalyzeAndGrouping(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	records := []*resolvedPostRecord{
		{TweetID: "11111", URL: "https://x.com/i/web/status/11111", CreatedAt: "2026-01-01T00:00:00Z", PostType: "original", PublicMetrics: map[string]any{"like_count": float64(4), "impression_count": float64(100)}, NonPublicMetrics: map[string]any{"profile_clicks": float64(3)}, Source: "live"},
		{TweetID: "22222", URL: "https://x.com/i/web/status/22222", CreatedAt: "2026-01-01T01:00:00Z", PostType: "original", PublicMetrics: map[string]any{"like_count": float64(8)}, Source: "local"},
	}
	snapshots, err := savePerformanceSnapshots(cmd, db, records, "24h")
	if err != nil {
		t.Fatalf("save snapshots: %v", err)
	}
	if len(snapshots) != 2 || snapshots[0].Metrics["like_count"].(float64) != 4 {
		t.Fatalf("snapshots = %+v", snapshots)
	}
	if snapshots[0].PublicMetrics["impression_count"].(float64) != 100 {
		t.Fatalf("public metrics = %+v", snapshots[0].PublicMetrics)
	}
	if snapshots[0].NonPublicMetrics["profile_clicks"].(float64) != 3 {
		t.Fatalf("non-public metrics = %+v", snapshots[0].NonPublicMetrics)
	}
	if snapshots[0].MetricAvailability["impression_count"] != "available" ||
		snapshots[0].MetricAvailability["bookmark_count"] != "not_returned_or_plan_restricted" ||
		snapshots[1].MetricAvailability["impression_count"] != "not_requested" {
		t.Fatalf("metric availability = %+v / %+v", snapshots[0].MetricAvailability, snapshots[1].MetricAvailability)
	}
	groups, err := analyzePerformanceSnapshots(cmd, db, "", "type,label")
	if err != nil {
		t.Fatalf("analyze snapshots: %v", err)
	}
	if len(groups) != 1 || groups[0].Count != 2 || groups[0].Averages["like_count"] != 6 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestPerformanceSnapshotsReplaceSameTweetAndLabel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	first := []*resolvedPostRecord{
		{TweetID: "11111", URL: "https://x.com/i/web/status/11111", CreatedAt: "2026-01-01T00:00:00Z", PostType: "original", PublicMetrics: map[string]any{"like_count": float64(4)}, Source: "local"},
	}
	second := []*resolvedPostRecord{
		{TweetID: "11111", URL: "https://x.com/i/web/status/11111", CreatedAt: "2026-01-01T00:00:00Z", PostType: "original", PublicMetrics: map[string]any{"like_count": float64(10)}, Source: "local"},
	}
	if _, err := savePerformanceSnapshots(cmd, db, first, "24h"); err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if _, err := savePerformanceSnapshots(cmd, db, second, "24h"); err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	groups, err := analyzePerformanceSnapshots(cmd, db, "", "label")
	if err != nil {
		t.Fatalf("analyze snapshots: %v", err)
	}
	if len(groups) != 1 || groups[0].Count != 1 || groups[0].Averages["like_count"] != 10 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestPerformanceHasLinkOnlyCountsURLEntities(t *testing.T) {
	dims := parseIncludeSet("has_link")
	hashtagOnly := &resolvedPostRecord{Entities: map[string]any{
		"hashtags": []any{map[string]any{"tag": "launch"}},
	}}
	withURL := &resolvedPostRecord{Entities: map[string]any{
		"urls": []any{map[string]any{"expanded_url": "https://example.com"}},
	}}
	if key := performanceGroupKey(dims, "", hashtagOnly); key != "has_link=false" {
		t.Fatalf("hashtag-only key = %q", key)
	}
	if key := performanceGroupKey(dims, "", withURL); key != "has_link=true" {
		t.Fatalf("url key = %q", key)
	}
}

func TestPerformanceHourGroupsByPostCreationTime(t *testing.T) {
	dims := parseIncludeSet("hour")
	rec := &resolvedPostRecord{CreatedAt: "2026-01-01T03:15:00Z"}
	key := performanceGroupKey(dims, "", rec)
	if key != "hour=03" {
		t.Fatalf("hour key = %q", key)
	}
}

func TestTimelineAndBriefMarkdownWriters(t *testing.T) {
	item := collectionItemSnapshot{TweetID: "12345", URL: "https://x.com/i/web/status/12345", Text: "hello"}
	var timeline bytes.Buffer
	if err := writeTimelineExport(&timeline, timelineExportResult{Subject: "@alice", Source: "local", GeneratedAt: "2026-01-01T00:00:00Z", Items: []collectionItemSnapshot{item}}, "markdown"); err != nil {
		t.Fatalf("timeline markdown: %v", err)
	}
	if !strings.Contains(timeline.String(), "X timeline export") || !strings.Contains(timeline.String(), "hello") {
		t.Fatalf("timeline markdown = %s", timeline.String())
	}
	var brief bytes.Buffer
	if err := writeBriefMarkdown(&brief, buildBrief("local", "24h", []collectionItemSnapshot{item})); err != nil {
		t.Fatalf("brief markdown: %v", err)
	}
	if !strings.Contains(brief.String(), "Highlights") || !strings.Contains(brief.String(), "Sources") {
		t.Fatalf("brief markdown = %s", brief.String())
	}
}

func TestLocalTimelineQueryEscapesLikeWildcards(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	rows := []struct {
		id   string
		text string
	}{
		{"1", "node_modules setup"},
		{"2", "nodeXmodules setup"},
	}
	for _, row := range rows {
		raw := `{"id":"` + row.id + `","text":"` + row.text + `","created_at":"2026-01-01T00:00:00Z"}`
		if _, err := db.DB().ExecContext(cmd.Context(),
			`INSERT INTO tweets(id, data, text, created_at) VALUES(?, ?, ?, ?)`,
			row.id, raw, row.text, "2026-01-01T00:00:00Z"); err != nil {
			t.Fatalf("insert tweet %s: %v", row.id, err)
		}
	}
	records, err := localTimelineQuery(cmd, dbPath, "node_modules", 10)
	if err != nil {
		t.Fatalf("localTimelineQuery: %v", err)
	}
	if len(records) != 1 || records[0].TweetID != "1" {
		t.Fatalf("records = %+v", records)
	}
}

func TestBuildTimelineExportFiltersLocalQuerySince(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	cmd := testCommand()
	rows := []struct {
		id        string
		createdAt string
	}{
		{"1", "2026-01-01T00:00:00Z"},
		{"2", "2026-02-01T00:00:00Z"},
	}
	for _, row := range rows {
		raw := `{"id":"` + row.id + `","text":"node update","created_at":"` + row.createdAt + `"}`
		if _, err := db.DB().ExecContext(cmd.Context(),
			`INSERT INTO tweets(id, data, text, created_at) VALUES(?, ?, ?, ?)`,
			row.id, raw, "node update", row.createdAt); err != nil {
			t.Fatalf("insert tweet %s: %v", row.id, err)
		}
	}
	result, err := buildTimelineExport(cmd, &rootFlags{}, dbPath, "local", nil, "node", "2026-01-15", 10)
	if err != nil {
		t.Fatalf("buildTimelineExport: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].TweetID != "2" {
		t.Fatalf("items = %+v", result.Items)
	}
}

func TestFilterRecordsSince(t *testing.T) {
	records := []*resolvedPostRecord{
		{TweetID: "old", CreatedAt: "2026-01-01T00:00:00Z"},
		{TweetID: "new", CreatedAt: "2026-02-01T00:00:00Z"},
		{TweetID: "unknown"},
	}
	filtered, err := filterRecordsSince(records, "2026-01-15")
	if err != nil {
		t.Fatalf("filterRecordsSince returned error: %v", err)
	}
	if len(filtered) != 2 || filtered[0].TweetID != "new" || filtered[1].TweetID != "unknown" {
		t.Fatalf("filtered = %+v", filtered)
	}
}

func TestSinceStartTimeParsesRFC3339WithoutLowercasing(t *testing.T) {
	start, ok, err := sinceStartTime("2024-06-01T12:00:00Z")
	if err != nil {
		t.Fatalf("sinceStartTime returned error: %v", err)
	}
	if !ok || start.Format("2006-01-02T15:04:05Z07:00") != "2024-06-01T12:00:00Z" {
		t.Fatalf("start=%s ok=%v", start.Format("2006-01-02T15:04:05Z07:00"), ok)
	}
}

func testCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}
