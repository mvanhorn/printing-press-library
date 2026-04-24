package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func withStubbedSearchTrendFetch(t *testing.T, stub func(flags *rootFlags, tag string) (json.RawMessage, error)) {
	t.Helper()
	oldFetch := fetchSearchTrendData
	fetchSearchTrendData = stub
	t.Cleanup(func() {
		fetchSearchTrendData = oldFetch
	})
}

func TestParseAccountBudgetDataHandlesWrappedCreditCount(t *testing.T) {
	balance := []byte(`{"results":{"success":true,"creditCount":42551,"message":"You have 42551 credits remaining."}}`)
	daily := []byte(`{"results":{"usage":[{"count":100},{"count":50}]}}`)

	creditsRemaining, dailyBurn := parseAccountBudgetData(balance, daily)
	if creditsRemaining != 42551 {
		t.Fatalf("creditsRemaining = %v, want 42551", creditsRemaining)
	}
	if dailyBurn != 75 {
		t.Fatalf("dailyBurn = %v, want 75", dailyBurn)
	}
}

func TestParseAccountBudgetDataHandlesLegacyShape(t *testing.T) {
	balance := []byte(`{"credits_remaining":1234}`)
	daily := []byte(`{"usage":[{"count":10},{"count":20}]}`)

	creditsRemaining, dailyBurn := parseAccountBudgetData(balance, daily)
	if creditsRemaining != 1234 {
		t.Fatalf("creditsRemaining = %v, want 1234", creditsRemaining)
	}
	if dailyBurn != 15 {
		t.Fatalf("dailyBurn = %v, want 15", dailyBurn)
	}
}

func TestSearchTrendsHistoryDoesNotCreateTrackingDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cmd := newSearchTrendsCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--hashtag", "BookTok", "--history"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var got hashtagTrendHistoryResponse
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal history response: %v", err)
	}
	if got.Hashtag != "#BookTok" {
		t.Fatalf("history hashtag = %q, want %q", got.Hashtag, "#BookTok")
	}
	if got.SnapshotsCount != 0 {
		t.Fatalf("history snapshots_count = %d, want 0", got.SnapshotsCount)
	}

	trackingDir := searchTrendTrackingDir()
	if _, err := os.Stat(trackingDir); !os.IsNotExist(err) {
		t.Fatalf("tracking dir should not be created for --history; stat err = %v", err)
	}
}

func TestSearchTrendsSnapshotReturnsJSONWhenPersistenceFails(t *testing.T) {
	tempRoot := t.TempDir()
	homeFile := filepath.Join(tempRoot, "home-file")
	if err := os.WriteFile(homeFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write fake home file: %v", err)
	}
	t.Setenv("HOME", homeFile)

	withStubbedSearchTrendFetch(t, func(flags *rootFlags, tag string) (json.RawMessage, error) {
		if tag != "BookTok" {
			t.Fatalf("fetch tag = %q, want %q", tag, "BookTok")
		}
		return json.RawMessage(`{
			"aweme_list": [
				{"aweme_id":"vid-2","desc":"second","statistics":{"play_count":200,"digg_count":20}},
				{"aweme_id":"vid-1","desc":"first","statistics":{"play_count":100,"digg_count":10}}
			]
		}`), nil
	})

	cmd := newSearchTrendsCmd(&rootFlags{asJSON: true})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--hashtag", " #BookTok "})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty for non-TTY json output", stderr.String())
	}

	var got struct {
		Hashtag    string `json:"hashtag"`
		VideoCount int    `json:"video_count"`
		TopVideos  []struct {
			ID string `json:"id"`
		} `json:"top_videos"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal snapshot response: %v", err)
	}
	if got.Hashtag != "#BookTok" {
		t.Fatalf("snapshot hashtag = %q, want %q", got.Hashtag, "#BookTok")
	}
	if got.VideoCount != 2 {
		t.Fatalf("video_count = %d, want 2", got.VideoCount)
	}
	if len(got.TopVideos) != 2 {
		t.Fatalf("top_videos len = %d, want 2", len(got.TopVideos))
	}
	if got.TopVideos[0].ID != "vid-2" {
		t.Fatalf("top_videos[0].id = %q, want %q", got.TopVideos[0].ID, "vid-2")
	}
}

func TestBuildTrendHistoryComputesDeltasAndStickyLeaders(t *testing.T) {
	snapshots := []hashtagTrendSnapshot{
		{
			Date:       "2026-04-20",
			SnapshotAt: "2026-04-20T00:00:00Z",
			Hashtag:    "#BookTok",
			VideoCount: 100,
			TopVideos: []storedTrendVideo{
				{ID: "vid-a", Rank: 1},
				{ID: "vid-b", Rank: 2},
				{ID: "vid-c", Rank: 3},
			},
		},
		{
			Date:       "2026-04-21",
			SnapshotAt: "2026-04-21T00:00:00Z",
			Hashtag:    "#BookTok",
			VideoCount: 125,
			TopVideos: []storedTrendVideo{
				{ID: "vid-a", Rank: 2},
				{ID: "vid-b", Rank: 1},
				{ID: "vid-d", Rank: 3},
			},
		},
		{
			Date:       "2026-04-22",
			SnapshotAt: "2026-04-22T00:00:00Z",
			Hashtag:    "#BookTok",
			VideoCount: 120,
			TopVideos: []storedTrendVideo{
				{ID: "vid-b", Rank: 1},
				{ID: "vid-d", Rank: 2},
				{ID: "vid-e", Rank: 3},
			},
		},
	}

	got := buildTrendHistory("#ignored", snapshots)

	if got.Hashtag != "#BookTok" {
		t.Fatalf("Hashtag = %q, want %q", got.Hashtag, "#BookTok")
	}
	if got.SnapshotsCount != 3 {
		t.Fatalf("SnapshotsCount = %d, want 3", got.SnapshotsCount)
	}

	row0 := got.Snapshots[0]
	if row0.Direction != "baseline" || row0.Delta != 0 || row0.PersistentTopVideos != 0 || row0.NewTopVideos != 3 {
		t.Fatalf("baseline row = %+v, want baseline with 3 new videos", row0)
	}

	row1 := got.Snapshots[1]
	if row1.Direction != "up" || row1.Delta != 25 || row1.PersistentTopVideos != 2 || row1.NewTopVideos != 1 {
		t.Fatalf("row1 = %+v, want up/+25 with 2 persistent and 1 new", row1)
	}

	row2 := got.Snapshots[2]
	if row2.Direction != "down" || row2.Delta != -5 || row2.PersistentTopVideos != 2 || row2.NewTopVideos != 1 {
		t.Fatalf("row2 = %+v, want down/-5 with 2 persistent and 1 new", row2)
	}

	if len(got.StickyTopVideos) != 3 {
		t.Fatalf("StickyTopVideos len = %d, want 3", len(got.StickyTopVideos))
	}

	leader := got.StickyTopVideos[0]
	if leader.ID != "vid-b" || leader.Appearances != 3 || leader.LongestStreak != 3 || leader.BestRank != 1 {
		t.Fatalf("top sticky leader = %+v, want vid-b with 3 appearances, streak 3, best rank 1", leader)
	}

	second := got.StickyTopVideos[1]
	if second.ID != "vid-a" || second.Appearances != 2 || second.LongestStreak != 2 || second.BestRank != 1 {
		t.Fatalf("second sticky leader = %+v, want vid-a with 2 appearances, streak 2, best rank 1", second)
	}

	third := got.StickyTopVideos[2]
	if third.ID != "vid-d" || third.Appearances != 2 || third.LongestStreak != 2 || third.BestRank != 2 {
		t.Fatalf("third sticky leader = %+v, want vid-d with 2 appearances, streak 2, best rank 2", third)
	}
}

func TestSearchTrendsSameDayRunUpsertsSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	call := 0
	withStubbedSearchTrendFetch(t, func(flags *rootFlags, tag string) (json.RawMessage, error) {
		call++
		if tag != "BookTok" {
			t.Fatalf("fetch tag = %q, want %q", tag, "BookTok")
		}
		if call == 1 {
			return json.RawMessage(`{
				"aweme_list": [
					{"aweme_id":"vid-1","desc":"first","statistics":{"play_count":100,"digg_count":10}}
				]
			}`), nil
		}
		return json.RawMessage(`{
			"aweme_list": [
				{"aweme_id":"vid-2","desc":"second","statistics":{"play_count":200,"digg_count":20}},
				{"aweme_id":"vid-1","desc":"first","statistics":{"play_count":100,"digg_count":10}}
			]
		}`), nil
	})

	run := func() {
		t.Helper()
		cmd := newSearchTrendsCmd(&rootFlags{asJSON: true})
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--hashtag", "BookTok"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}

	run()
	run()

	raw, err := os.ReadFile(searchTrendSnapshotFile("BookTok"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var snapshots []hashtagTrendSnapshot
	if err := json.Unmarshal(raw, &snapshots); err != nil {
		t.Fatalf("unmarshal snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snapshots))
	}
	if snapshots[0].VideoCount != 2 {
		t.Fatalf("VideoCount = %d, want 2 after upsert", snapshots[0].VideoCount)
	}
	if len(snapshots[0].TopVideos) != 2 || snapshots[0].TopVideos[0].ID != "vid-2" {
		t.Fatalf("TopVideos = %+v, want updated second snapshot data", snapshots[0].TopVideos)
	}
}
