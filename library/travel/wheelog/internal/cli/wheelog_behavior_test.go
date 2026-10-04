// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
	"github.com/spf13/cobra"
	"path/filepath"
	"testing"
	"time"
)

func TestWheelogRecordDatesRespectTimezoneAndCalendarDay(t *testing.T) {
	cases := []struct{ from, to, zone, start, end string }{
		{"2026-10-02", "2026-10-02", "Asia/Tokyo", "2026-10-01 15:00:00", "2026-10-02 14:59:59"},
		{"2026-03-08", "2026-03-08", "America/New_York", "2026-03-08 05:00:00", "2026-03-09 03:59:59"},
		{"", "2026-10-02", "UTC", "", "2026-10-02 23:59:59"},
	}
	for _, c := range cases {
		start, end, err := wheelogDateWindow(c.from, c.to, c.zone)
		if err != nil {
			t.Fatal(err)
		}
		if c.start == "" {
			if start != nil {
				t.Fatal("invented lower date bound")
			}
		} else if start == nil || *start != c.start {
			t.Fatalf("start %v", start)
		}
		if end == nil || *end != c.end {
			t.Fatalf("end %v", end)
		}
	}
	for _, c := range [][3]string{{"2026-02-30", "", "UTC"}, {"2026-10-03", "2026-10-02", "UTC"}, {"", "", "invalid/zone"}} {
		if _, _, err := wheelogDateWindow(c[0], c[1], c[2]); err == nil {
			t.Fatalf("accepted invalid date window %v", c)
		}
	}
}

func runWheelogCommand(t *testing.T, factory func(*rootFlags) *cobra.Command, args ...string) ([]byte, error) {
	t.Helper()
	f := &rootFlags{asJSON: true, timeout: time.Minute, noLearn: true, dataSource: "local"}
	cmd := factory(f)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.Bytes(), err
}

func TestWheelogSavedProximityAuditAndBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.db")
	db, err := openWheelogStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	zero, one := 0, 1
	for i, lon := range []float64{140.388, 140.389} {
		spot := wheelog.Spot{ID: int64(166345 + i), Name: "Public place", Category: "toilet", Source: "live", DetailStatus: "checked", ObservedAt: time.Now().Add(-14 * 24 * time.Hour).UTC().Format(time.RFC3339),
			Location:  &wheelog.Coordinate{Latitude: 35.7742, Longitude: lon},
			Questions: []wheelog.Question{{ID: 102, Positive: &one, Negative: &zero, State: "reported_affirmative"}}, Gaps: []string{}}
		if err := db.ObserveWheelog(context.Background(), spot); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	raw, err := runWheelogCommand(t, newNovelShortlistListCmd, "--db", path, "--origin", "35.7742,140.3879", "--radius-m", "500", "--audit", "--max-record-age", "7d", "--require-question", "102")
	if err != nil {
		t.Fatal(err)
	}
	var list struct{ Results []wheelogSpotView }
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Results) != 2 || list.Results[0].ID != 166345 || list.Results[0].Distance == nil || *list.Results[0].Distance >= *list.Results[1].Distance {
		t.Fatalf("incorrect proximity %s", raw)
	}
	if list.Results[0].UpdateAge != nil || len(list.Results[0].Recheck) == 0 || list.Results[0].Source != "local" {
		t.Fatalf("lost undated record/saved freshness distinction %s", raw)
	}
	raw, err = runWheelogCommand(t, newNovelShortlistChangesCmd, "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	var changes struct {
		Results []struct {
			Status  string
			Changes []wheelog.Change
		}
	}
	if err := json.Unmarshal(raw, &changes); err != nil {
		t.Fatal(err)
	}
	if len(changes.Results) != 2 || changes.Results[0].Status != "no_baseline" || len(changes.Results[0].Changes) != 0 {
		t.Fatalf("invented saved transition %s", raw)
	}
}

func TestWheelogUsageAndEmptyShortlistDoNotNeedSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	raw, err := runWheelogCommand(t, newNovelShortlistListCmd, "--db", path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Results []json.RawMessage }
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Results == nil || len(doc.Results) != 0 {
		t.Fatalf("empty list must be []: %s (%v)", raw, err)
	}
	for _, args := range [][]string{{"--db", path, "--origin", "NaN,0"}, {"--db", path, "--radius-m", "20"}, {"--db", path, "--require-question", "9999"}} {
		if _, err := runWheelogCommand(t, newNovelShortlistListCmd, args...); err == nil {
			t.Fatalf("invalid local args accepted %v", args)
		}
	}
	if _, err := runWheelogCommand(t, newSpotsSearchCmd, "--db", path, "--details", "--limit", "6"); err == nil {
		t.Fatal("unbounded detail expansion accepted")
	}
	if _, err := runWheelogCommand(t, newNovelSpotsCompareCmd, "--db", path, "166345", "166345"); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
}

func TestWheelogLocalDateSearchIncludesFractionalFinalSecond(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.db")
	db, err := openWheelogStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for i, stamp := range []string{"2026-10-02T14:59:59.500000Z", "2026-10-02T15:00:00Z"} {
		created := stamp
		spot := wheelog.Spot{ID: int64(166345 + i), Name: "Public restroom", Category: "toilet", Created: &created, ObservedAt: time.Now().UTC().Format(time.RFC3339), Questions: []wheelog.Question{}, Gaps: []string{}}
		if err := db.ObserveWheelog(context.Background(), spot); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	raw, err := runWheelogCommand(t, newSpotsSearchCmd, "--db", path, "--category", "toilet", "--from", "2026-10-02", "--to", "2026-10-02", "--timezone", "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	var result struct{ Results []wheelogSpotView }
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].ID != 166345 {
		t.Fatalf("fractional final second excluded or next day included: %s", raw)
	}
}
