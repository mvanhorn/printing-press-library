// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestApertureFromAPEX(t *testing.T) {
	cases := []struct {
		av   float64
		want string
	}{
		{2, "f/2"},   // Av 2 => f/2
		{6, "f/8"},   // Av 6 => f/8
		{1, "f/1.4"}, // Av 1 => f/1.4
		{4.970854, "f/5.6"},
		{0, "f/1"},
	}
	for _, c := range cases {
		if got := ApertureFromAPEX(c.av); got != c.want {
			t.Errorf("ApertureFromAPEX(%v) = %q, want %q", c.av, got, c.want)
		}
	}
}

func TestShutterFromAPEX(t *testing.T) {
	cases := []struct {
		tv   float64
		want string
	}{
		{8, "1/256"},
		{0, "1s"},
		{-1, "2s"},
		{7.965784, "1/250"},
		{-1.584963, "3s"},
	}
	for _, c := range cases {
		if got := ShutterFromAPEX(c.tv); got != c.want {
			t.Errorf("ShutterFromAPEX(%v) = %q, want %q", c.tv, got, c.want)
		}
	}
}

func day(s string) time.Time {
	t, _ := time.ParseInLocation(dayFormat, s, time.Local)
	return t
}

func TestComputeStreaks(t *testing.T) {
	shot := map[string]DayCount{
		"2026-07-01": {}, "2026-07-02": {}, "2026-07-03": {},
		"2026-07-05": {}, "2026-07-06": {},
	}
	rep := computeStreaks(shot, day("2026-07-01"), day("2026-07-06"), day("2026-07-06"))
	if rep.LongestStreak != 3 {
		t.Errorf("longest = %d, want 3", rep.LongestStreak)
	}
	if rep.CurrentStreak != 2 {
		t.Errorf("current = %d, want 2 (Jul 5-6)", rep.CurrentStreak)
	}
	if len(rep.Gaps) != 1 || rep.Gaps[0] != "2026-07-04" {
		t.Errorf("gaps = %v, want [2026-07-04]", rep.Gaps)
	}
	if rep.DaysWithShots != 5 || rep.TotalDays != 6 {
		t.Errorf("coverage = %d/%d, want 5/6", rep.DaysWithShots, rep.TotalDays)
	}
}

func TestComputeStreaksEmptyTodayAnchorsYesterday(t *testing.T) {
	// Today has no photo yet: the streak through yesterday must survive.
	shot := map[string]DayCount{"2026-07-04": {}, "2026-07-05": {}}
	rep := computeStreaks(shot, day("2026-07-01"), day("2026-07-06"), day("2026-07-06"))
	if rep.CurrentStreak != 2 {
		t.Errorf("current = %d, want 2 (anchored to yesterday)", rep.CurrentStreak)
	}
}

// fixtureCatalog builds a minimal .lrcat-shaped SQLite file and opens it read-only.
func fixtureCatalog(t *testing.T) *Catalog {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.lrcat")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE Adobe_images (id_local INTEGER PRIMARY KEY, captureTime, rating, pick NOT NULL DEFAULT 0, colorLabels NOT NULL DEFAULT '', fileFormat NOT NULL DEFAULT 'RAW', rootFile INTEGER NOT NULL DEFAULT 0, touchTime NOT NULL DEFAULT 0)`,
		`CREATE TABLE AgLibraryFile (id_local INTEGER PRIMARY KEY, idx_filename, folder INTEGER)`,
		`CREATE TABLE AgLibraryFolder (id_local INTEGER PRIMARY KEY, pathFromRoot, rootFolder INTEGER)`,
		`CREATE TABLE AgLibraryRootFolder (id_local INTEGER PRIMARY KEY, absolutePath)`,
		`CREATE TABLE AgHarvestedExifMetadata (id_local INTEGER PRIMARY KEY, image INTEGER, aperture, shutterSpeed, isoSpeedRating, focalLength, cameraModelRef INTEGER, lensRef INTEGER, dateYear, dateMonth, dateDay)`,
		`CREATE TABLE AgInternedExifCameraModel (id_local INTEGER PRIMARY KEY, value)`,
		`CREATE TABLE AgInternedExifLens (id_local INTEGER PRIMARY KEY, value)`,
		`CREATE TABLE AgLibraryKeyword (id_local INTEGER PRIMARY KEY, name, lc_name)`,
		`CREATE TABLE AgLibraryKeywordImage (id_local INTEGER PRIMARY KEY, image INTEGER, tag INTEGER)`,
		`CREATE TABLE AgLibraryCollection (id_local INTEGER PRIMARY KEY, name, systemOnly NOT NULL DEFAULT 0)`,
		`CREATE TABLE AgLibraryCollectionImage (id_local INTEGER PRIMARY KEY, collection INTEGER, image INTEGER)`,
		`CREATE TABLE Adobe_imageDevelopSettings (id_local INTEGER PRIMARY KEY, image INTEGER, hasDevelopAdjustmentsEx)`,
		`INSERT INTO AgLibraryRootFolder VALUES (1, '/photos/')`,
		`INSERT INTO AgLibraryFolder VALUES (1, '2026/', 1)`,
		`INSERT INTO AgLibraryFile VALUES (1, 'a.arw', 1), (2, 'b.arw', 1), (3, 'c.arw', 1)`,
		`INSERT INTO Adobe_images VALUES
			(1, '2026-07-01T08:00:00', 5.0, 1, '', 'RAW', 1, 100),
			(2, '2026-07-01T09:00:00', 3.0, 0, 'Red', 'RAW', 2, 90),
			(3, '2026-07-03T10:00:00', NULL, 0, '', 'JPG', 3, 80)`,
		`INSERT INTO AgInternedExifCameraModel VALUES (1, 'LEICA Q2')`,
		`INSERT INTO AgInternedExifLens VALUES (1, 'SUMMILUX 1:1.7/28')`,
		`INSERT INTO AgHarvestedExifMetadata VALUES
			(1, 1, 2.0, 8.0, 200.0, 28.0, 1, 1, 2026, 7, 1),
			(2, 2, 6.0, 0.0, 1600.0, 28.0, 1, 1, 2026, 7, 1),
			(3, 3, NULL, NULL, NULL, NULL, NULL, NULL, 2026, 7, 3)`,
		`INSERT INTO AgLibraryKeyword VALUES (1, 'street', 'street'), (2, 'unused', 'unused')`,
		`INSERT INTO AgLibraryKeywordImage VALUES (1, 1, 1)`,
		`INSERT INTO AgLibraryCollection VALUES (1, '100 Faces', 0), (2, 'Empty One', 0), (3, 'quickCollection', 1.0)`,
		`INSERT INTO AgLibraryCollectionImage VALUES (1, 1, 1), (2, 1, 3)`,
		`INSERT INTO Adobe_imageDevelopSettings VALUES (1, 1, 1.0), (2, 2, -1.0), (3, 3, -1.0)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("fixture stmt failed: %v\n%s", err, s)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cat, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	return cat
}

func TestFindPhotosCriteria(t *testing.T) {
	cat := fixtureCatalog(t)
	ctx := context.Background()

	picked, err := cat.FindPhotos(ctx, FindFilters{Picked: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(picked) != 1 || picked[0].ID != 1 {
		t.Errorf("picked = %+v, want image 1 only", picked)
	}
	if picked[0].Aperture != "f/2" || picked[0].Shutter != "1/256" {
		t.Errorf("EXIF conversion wrong: %+v", picked[0])
	}
	if picked[0].Path != "/photos/2026/a.arw" {
		t.Errorf("path = %q", picked[0].Path)
	}

	rated, err := cat.FindPhotos(ctx, FindFilters{Rating: ">=4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rated) != 1 || rated[0].ID != 1 {
		t.Errorf("rating>=4 = %+v, want image 1", rated)
	}

	unrated, err := cat.FindPhotos(ctx, FindFilters{Rating: "unrated"})
	if err != nil {
		t.Fatal(err)
	}
	if len(unrated) != 1 || unrated[0].ID != 3 {
		t.Errorf("unrated = %+v, want image 3", unrated)
	}

	kw, err := cat.FindPhotos(ctx, FindFilters{Keyword: "street"})
	if err != nil {
		t.Fatal(err)
	}
	if len(kw) != 1 || kw[0].ID != 1 {
		t.Errorf("keyword street = %+v, want image 1", kw)
	}

	// Negative: a mismatching keyword returns nothing, not everything.
	none, err := cat.FindPhotos(ctx, FindFilters{Keyword: "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Errorf("keyword nope = %d results, want 0", len(none))
	}

	if _, err := cat.FindPhotos(ctx, FindFilters{Rating: "banana"}); err == nil {
		t.Error("expected error for junk rating expression")
	}
}

func TestListingsAndFunnel(t *testing.T) {
	cat := fixtureCatalog(t)
	ctx := context.Background()

	cols, err := cat.Collections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// systemOnly collection excluded; two user collections remain.
	if len(cols) != 2 || cols[0].Name != "100 Faces" || cols[0].ImageCount != 2 {
		t.Errorf("collections = %+v", cols)
	}

	kws, err := cat.Keywords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(kws) != 2 {
		t.Errorf("keywords = %+v, want street + unused", kws)
	}

	cams, err := cat.Cameras(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cams) != 1 || cams[0].Name != "LEICA Q2" || cams[0].ImageCount != 2 || cams[0].FirstSeen != "2026-07-01" {
		t.Errorf("cameras = %+v", cams)
	}

	fun, err := cat.Funnel(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	st := fun[0].Stages
	if st[0].Count != 3 || st[1].Count != 1 || st[2].Count != 2 || st[3].Count != 1 || st[4].Count != 2 {
		t.Errorf("funnel = %+v", st)
	}
}

func TestDailyCommands(t *testing.T) {
	cat := fixtureCatalog(t)
	ctx := context.Background()

	p, err := cat.PickOfDay(ctx, "2026-07-01")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.ID != 1 {
		t.Errorf("pick of 2026-07-01 = %+v, want image 1 (flagged)", p)
	}

	empty, err := cat.PickOfDay(ctx, "2026-07-02")
	if err != nil {
		t.Fatal(err)
	}
	if empty != nil {
		t.Errorf("pick of empty day = %+v, want nil", empty)
	}

	rng, err := cat.PickOfDayRange(ctx, "2026-07-01", "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(rng) != 2 {
		t.Errorf("range picks = %d, want 2 (days with photos only)", len(rng))
	}

	otd, err := cat.OnThisDay(ctx, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(otd) != 1 || otd[0].Photos != 2 || otd[0].Best == nil || otd[0].Best.ID != 1 {
		t.Errorf("on-this-day = %+v", otd)
	}

	backlog, err := cat.Backlog(ctx, 3, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Image 2 is rated 3 with no develop adjustments; image 1 is developed.
	if len(backlog) != 1 || backlog[0].ID != 2 {
		t.Errorf("backlog = %+v, want image 2", backlog)
	}
}

func TestReadOnlyEnforced(t *testing.T) {
	cat := fixtureCatalog(t)
	if _, err := cat.DB.Exec("UPDATE Adobe_images SET rating = 1"); err == nil {
		t.Fatal("write succeeded on read-only catalog — the safety contract is broken")
	}
}
