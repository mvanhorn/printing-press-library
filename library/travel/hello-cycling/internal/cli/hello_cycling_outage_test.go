package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
)

func TestHCStatusOutageFallbackAndSnapshotProtection(t *testing.T) {
	testenv.Isolate(t)
	now := time.Now().UTC()
	count, spaces, yes := 1, 11, true
	baselineTime := now.Add(-10 * time.Minute)
	cached := cycling.Snapshot{ObservedAt: baselineTime, Feeds: map[string]cycling.FeedMeta{"station_status": {LastUpdated: baselineTime.Unix()}}, Information: []cycling.Info{{ID: "5112", Name: "プラーズタワー東新宿"}}, Statuses: []cycling.Status{{ID: "5112", Bikes: &count, Docks: &spaces, Reported: baselineTime.Unix(), Installed: &yes, Renting: &yes, Returning: &yes}}}
	partial := cached
	partial.Statuses = nil
	partial.Warnings = []string{"station_status unavailable: HTTP 500"}
	oldFetch := hcFetchSnapshot
	defer func() { hcFetchSnapshot = oldFetch }()
	hcFetchSnapshot = func(context.Context) (cycling.Snapshot, error) {
		return partial, &cycling.StatusFeedError{Cause: &cycling.HTTPError{Status: 500, Host: "source.example"}}
	}
	path := filepath.Join(t.TempDir(), "baseline.db")
	if err := cycling.SaveSnapshot(path, cached); err != nil {
		t.Fatal(err)
	}
	preimage, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, mode, source string
		noCache            bool
	}{
		{"auto fallback", "auto", "local", false},
		{"explicit live partial", "live", "live", false},
		{"no-cache partial", "auto", "live", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &rootFlags{dataSource: tc.mode, noCache: tc.noCache}
			cmd := newNovelStationsShowCmd(f)
			cmd.SetContext(context.Background())
			snap, source, err := hcSnapshot(cmd, f, hcOptions{db: path})
			if err != nil || source != tc.source {
				t.Fatalf("source=%s error=%v", source, err)
			}
			row := snap.Stations(now, 5*time.Minute, "")[0]
			if source == "local" && (row.Bikes == nil || *row.Bikes != 1 || len(snap.Warnings) == 0) {
				t.Fatal("usable cached status or warning lost")
			}
			if source == "live" && (row.Bikes != nil || row.RentalState != "source_missing") {
				t.Fatal("partial status was represented as an availability count")
			}
		})
	}
	for _, command := range []string{"sync", "changes"} {
		root := RootCmd()
		root.SetArgs([]string{"stations", command, "--snapshot-db", path, "--data-source", "live", "--json"})
		var out, errs bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errs)
		if err := root.Execute(); err == nil {
			t.Fatalf("%s accepted a status outage", command)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(preimage, after) {
			t.Fatalf("%s changed baseline during outage: %v", command, err)
		}
	}
	// Changes must compare the baseline at its original observation time.
	current := cached
	current.ObservedAt = now
	current.Feeds = map[string]cycling.FeedMeta{"station_status": {LastUpdated: current.ObservedAt.Unix()}}
	current.Statuses = append([]cycling.Status(nil), cached.Statuses...)
	hcFetchSnapshot = func(context.Context) (cycling.Snapshot, error) { return current, nil }
	root := RootCmd()
	root.SetArgs([]string{"stations", "changes", "--snapshot-db", path, "--data-source", "live", "--json"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results []cycling.Change `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || len(result.Results) != 1 || result.Results[0].After.RentalState != "stale" {
		t.Fatalf("derived-state change missing: %s (%v)", out.String(), err)
	}
}
