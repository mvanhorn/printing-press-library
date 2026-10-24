// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Synthetic store fixtures for the slice-3a local history commands. App ids,
// names and prices are public catalogue data; every reviewer name, review
// title and review body is invented ("reviewer-N", made-up text).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// s3aT0 is the base capture time of the synthetic snapshots.
var s3aT0 = time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

type s3aResult struct {
	stdout, stderr string
	err            error
	code           int
}

// s3aRun executes the CLI in-process and captures stdout, stderr and the
// typed exit code (0 on success).
func s3aRun(t *testing.T, args ...string) s3aResult {
	t.Helper()
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
	}
	return s3aResult{stdout: out.String(), stderr: errb.String(), err: err, code: code}
}

// s3aDecode parses a JSON result into v or fails the test.
func s3aDecode(t *testing.T, r s3aResult, v any) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("exit %d (%v)\nstdout: %s\nstderr: %s", r.code, r.err, r.stdout, r.stderr)
	}
	if err := json.Unmarshal([]byte(r.stdout), v); err != nil {
		t.Fatalf("stdout is not the expected JSON: %v\n%s", err, r.stdout)
	}
}

// s3aSeededDB returns the path of a fresh store holding the full synthetic
// fixture.
func s3aSeededDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()
	if err := s3aSeed(context.Background(), st); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return path
}

// s3aEmptyDB returns the path of a migrated store with no rows.
func s3aEmptyDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "empty.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	_ = st.Close()
	return path
}

type s3aChartFixture struct {
	key  store.ChartKey
	at   time.Time
	rows []store.ChartRow
}

func s3aRows(entries ...string) []store.ChartRow {
	// entries are "appId|title|developer" in rank order.
	out := make([]store.ChartRow, 0, len(entries))
	for i, e := range entries {
		parts := strings.SplitN(e, "|", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		out = append(out, store.ChartRow{Rank: i + 1, AppID: parts[0], Title: parts[1], DeveloperName: parts[2]})
	}
	return out
}

const (
	s3aCandy    = "553834731|Candy Crush Saga|King"
	s3aRoyal    = "1482155847|Royal Match|Dream Games"
	s3aPokemon  = "1094591345|Pokémon GO|Niantic, Inc."
	s3aRoblox   = "431946152|Roblox|Roblox Corporation"
	s3aMonopoly = "1621328561|MONOPOLY GO!|Scopely, Inc."
	s3aNewGame  = "6446308106|Sample New Game|Sample Studio"
	s3aSpotify  = "324684580|Spotify: Music and Podcasts|Spotify"
)

var (
	// s3aKeyGames has two web captures: the movers fixture.
	s3aKeyGames = store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "6014", Chart: "top-free", Source: store.SourceWebChart}
	// s3aKeyPaidGB has a single capture.
	s3aKeyPaidGB = store.ChartKey{Storefront: "gb", Platform: "iphone", GenreID: "36", Chart: "top-paid", Source: store.SourceWebChart}
	// s3aKeyGrossing has three RSS captures; Candy Crush is absent from the middle one.
	s3aKeyGrossing = store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-grossing", Source: store.SourceRSS}
)

func s3aCharts() []s3aChartFixture {
	return []s3aChartFixture{
		{s3aKeyGames, s3aT0, s3aRows(s3aCandy, s3aRoyal, s3aPokemon, s3aRoblox, s3aMonopoly)},
		{s3aKeyGames, s3aT0.Add(24 * time.Hour), s3aRows(s3aRoyal, s3aCandy, s3aMonopoly, s3aNewGame, s3aPokemon)},
		{s3aKeyPaidGB, s3aT0, s3aRows("932747118|Shadowrocket|Shadow Launch Technology Limited", "1444383602|Procreate Pocket|Savage Interactive Pty Ltd")},
		{s3aKeyGrossing, s3aT0.Add(-24 * time.Hour), s3aRows(s3aMonopoly, s3aRoyal, s3aSpotify, s3aPokemon, s3aCandy)},
		{s3aKeyGrossing, s3aT0, s3aRows(s3aMonopoly, s3aRoyal, s3aSpotify, s3aPokemon)},
		{s3aKeyGrossing, s3aT0.Add(24 * time.Hour), s3aRows(s3aMonopoly, s3aSpotify, s3aCandy, s3aRoyal)},
	}
}

type s3aListing struct {
	at            time.Time
	storefront    string
	appID         int64
	title         string
	subtitle      string
	version       string
	releasedAt    string
	notes         string
	price         float64
	formatted     string
	editorsChoice bool
	iaps          []appstore.IAPItem
	privacy       []string
	events        []appstore.AppEvent
	screenshots   []string
	lookupOnly    bool
}

func s3aRecordListing(ctx context.Context, st *store.Store, l s3aListing) error {
	lk := appstore.LookupResult{
		WrapperType: "software", Kind: "software", TrackID: l.appID, TrackName: l.title,
		Version: l.version, ReleaseNotes: l.notes, CurrentVersionReleaseDate: l.releasedAt,
		Price: l.price, Currency: "USD", FormattedPrice: l.formatted, SellerName: "Sample Seller",
		ArtistName: "Sample Developer", PrimaryGenreID: 6011, PrimaryGenreName: "Music",
		TrackViewURL:      fmt.Sprintf("https://apps.apple.com/%s/app/id%d?uo=4", l.storefront, l.appID),
		AverageUserRating: 4.5, UserRatingCount: 1000,
	}
	if l.lookupOnly {
		return store.RecordLookupSnapshot(ctx, st, l.storefront, lk, l.at)
	}
	types := make([]appstore.PrivacyType, 0, len(l.privacy))
	for _, p := range l.privacy {
		types = append(types, appstore.PrivacyType{Identifier: p, Title: p, Categories: []appstore.PrivacyCategory{}})
	}
	page := appstore.ProductPage{
		AdamID: fmt.Sprint(l.appID), Title: l.title, Subtitle: l.subtitle, EditorsChoice: l.editorsChoice,
		InAppPurchases: l.iaps, HasInAppPurchases: len(l.iaps) > 0, Privacy: appstore.Privacy{Types: types},
		Events: l.events, Media: []appstore.MediaSet{{Device: "iphone", Screenshots: l.screenshots}},
		Versions:  []appstore.VersionEntry{{Version: l.version, ReleasedAt: l.releasedAt, Notes: l.notes}},
		FetchedAt: l.at,
	}
	return store.RecordProductPage(ctx, st, l.storefront, page, lk)
}

// s3aSpotifyEvent is the in-app event added in the third Spotify snapshot.
var s3aSpotifyEvent = appstore.AppEvent{EventID: "6740000001", Title: "Sample Live Session", Kind: "Live Event", StartDate: "2026-10-25T18:00:00Z", EndDate: "2026-10-25T20:00:00Z"}

func s3aListings() []s3aListing {
	spotify := s3aListing{
		storefront: "us", appID: 324684580, title: "Spotify: Music and Podcasts", subtitle: "Music, podcasts and audiobooks",
		version: "9.1.90", releasedAt: "2026-10-12T10:00:00Z", notes: "Bug fixes and improvements.",
		price: 0, formatted: "Free",
		iaps:        []appstore.IAPItem{{Position: 1, Name: "Premium Individual", Price: "$11.99"}, {Position: 2, Name: "Premium Duo", Price: "$16.99"}},
		privacy:     []string{"DATA_LINKED_TO_YOU", "DATA_NOT_LINKED_TO_YOU"},
		screenshots: []string{"https://example.invalid/s1.png", "https://example.invalid/s2.png", "https://example.invalid/s3.png"},
	}
	s1, s2, s3 := spotify, spotify, spotify
	s1.at, s2.at, s3.at = s3aT0, s3aT0.Add(24*time.Hour), s3aT0.Add(48*time.Hour)
	s3.version, s3.releasedAt, s3.notes = "9.1.92", "2026-10-19T13:57:56Z", "New: smarter shuffle."
	s3.iaps = []appstore.IAPItem{{Position: 1, Name: "Premium Individual", Price: "$12.99"}, {Position: 2, Name: "Premium Duo", Price: "$16.99"}}
	s3.events = []appstore.AppEvent{s3aSpotifyEvent}

	candy := s3aListing{
		storefront: "us", appID: 553834731, title: "Candy Crush Saga", subtitle: "Match 3 puzzle adventure",
		version: "1.300.0", releasedAt: "2026-10-14T09:00:00Z", notes: "New levels.", formatted: "Free",
		iaps:    []appstore.IAPItem{{Position: 1, Name: "Bag of Gold", Price: "$1.99"}},
		privacy: []string{"DATA_USED_TO_TRACK_YOU"}, screenshots: []string{"https://example.invalid/c1.png"},
	}
	c1, c2 := candy, candy
	c1.at, c2.at = s3aT0, s3aT0.Add(24*time.Hour)

	chatgpt := s3aListing{at: s3aT0, storefront: "us", appID: 6448311069, title: "ChatGPT", version: "1.2026.290", formatted: "Free", lookupOnly: true}
	royalGB := s3aListing{at: s3aT0, storefront: "gb", appID: 1482155847, title: "Royal Match", subtitle: "King's puzzle", version: "30100", formatted: "Free"}
	return []s3aListing{s1, s2, s3, c1, c2, chatgpt, royalGB}
}

type s3aReview struct {
	storefront, version, title, body string
	rating                           int
}

// s3aChatGPTReviews: 10 in us, 4 in gb; low-star texts mention crash and login.
var s3aChatGPTReviews = []s3aReview{
	{"us", "1.2026.290", "Great helper", "Answers are clear and quick", 5},
	{"us", "1.2026.290", "Love it", "Writes good drafts for work", 5},
	{"us", "1.2026.280", "Daily use", "Use it daily for research", 5},
	{"us", "1.2026.290", "Useful", "Mostly useful, sometimes verbose", 4},
	{"us", "1.2026.280", "Solid", "Solid, but voice mode lags", 4},
	{"us", "1.2026.280", "Okay", "Fine for quick questions", 3},
	{"us", "1.2026.290", "Crash on start", "Crash right after login, subscription lost", 2},
	{"us", "1.2026.280", "Login loop", "Login fails then a crash", 2},
	{"us", "1.2026.290", "Crash again", "Every crash wipes my chat history", 1},
	{"us", "1.2026.290", "Too slow", "Slow answers and the login keeps timing out", 1},
	{"gb", "1.2026.290", "Brilliant", "Brilliant for revision", 5},
	{"gb", "1.2026.290", "Good", "Good answers overall", 4},
	{"gb", "1.2026.290", "Crash", "Crash when uploading a photo", 1},
	{"gb", "", "No sync", "History does not sync across devices", 1},
}

func s3aSeedReviews(ctx context.Context, st *store.Store) error {
	byStorefront := map[string][]appstore.Review{}
	for i, r := range s3aChatGPTReviews {
		byStorefront[r.storefront] = append(byStorefront[r.storefront], appstore.Review{
			ID: fmt.Sprintf("9000000%04d", i+1), Rating: r.rating, Version: r.version, Title: r.title, Body: r.body,
			Author: fmt.Sprintf("reviewer-%d", i+1), ReviewedAt: appstore.FormatTime(s3aT0.Add(-time.Duration(i) * time.Hour)),
		})
	}
	for _, sf := range []string{"us", "gb"} {
		if _, err := store.UpsertReviews(ctx, st, store.ReviewUpsert{
			Storefront: sf, AppID: "6448311069", Sort: appstore.SortMostRecent, Source: store.SourceRSS, SeenAt: s3aT0, Reviews: byStorefront[sf],
		}); err != nil {
			return err
		}
	}
	return nil
}

func s3aKeywordCaptures() []store.KeywordRank {
	return []store.KeywordRank{
		{CapturedAt: s3aT0, Storefront: "us", Platform: "iphone", Term: "spotify", Source: store.SourceSearchAPI, AppID: "324684580", Rank: 1, ResultCount: 50, TopIDs: []string{"324684580", "1017492454", "284035177"}},
		{CapturedAt: s3aT0.Add(24 * time.Hour), Storefront: "us", Platform: "iphone", Term: "spotify", Source: store.SourceSearchAPI, AppID: "324684580", Rank: 2, ResultCount: 50, TopIDs: []string{"1017492454", "324684580", "284035177"}},
		{CapturedAt: s3aT0.Add(48 * time.Hour), Storefront: "us", Platform: "iphone", Term: "spotify", Source: store.SourceSearchAPI, ResultCount: 48, TopIDs: []string{"324684580", "284035177", "1017492454"}},
		{CapturedAt: s3aT0.Add(24 * time.Hour), Storefront: "us", Platform: "iphone", Term: "music", Source: store.SourceSearchAPI, AppID: "324684580", Rank: 6, ResultCount: 200, TopIDs: []string{"1", "2", "3", "4", "5", "324684580"}},
		{CapturedAt: s3aT0.Add(48 * time.Hour), Storefront: "us", Platform: "iphone", Term: "music", Source: store.SourceWebSearch, AppID: "324684580", Rank: 4, ResultCount: 30, TopIDs: []string{"1", "2", "3", "324684580"}},
		{CapturedAt: s3aT0, Storefront: "us", Platform: "iphone", Term: "podcast app", Source: store.SourceSearchAPI, AppID: "553834731", Rank: 0, ResultCount: 200, TopIDs: []string{"324684580", "1", "2"}},
	}
}

// s3aSeed writes the whole synthetic fixture through the store accessors.
func s3aSeed(ctx context.Context, st *store.Store) error {
	for _, c := range s3aCharts() {
		snap := store.ChartSnapshot{CapturedAt: c.at, Storefront: c.key.Storefront, Platform: c.key.Platform, GenreID: c.key.GenreID,
			Chart: c.key.Chart, Source: c.key.Source, RowLimit: 200, Rows: c.rows}
		if c.key.Source == store.SourceRSS {
			snap.RowLimit = 100
		}
		if err := store.RecordChart(ctx, st, snap); err != nil {
			return fmt.Errorf("chart: %w", err)
		}
	}
	for _, l := range s3aListings() {
		if err := s3aRecordListing(ctx, st, l); err != nil {
			return fmt.Errorf("listing %d: %w", l.appID, err)
		}
	}
	if err := s3aSeedReviews(ctx, st); err != nil {
		return fmt.Errorf("reviews: %w", err)
	}
	for _, k := range s3aKeywordCaptures() {
		if err := store.RecordKeywordRank(ctx, st, k); err != nil {
			return fmt.Errorf("keyword: %w", err)
		}
	}
	return nil
}

// TestS3ASeedAcceptanceStore writes the synthetic fixture into the store at
// $S3A_SEED_DB (with S3A_SEED_EMPTY=1 it only creates the empty tables). It
// is skipped in ordinary runs; the slice-3a acceptance run uses it to fill
// an isolated --home store.
func TestS3ASeedAcceptanceStore(t *testing.T) {
	path := os.Getenv("S3A_SEED_DB")
	if path == "" {
		t.Skip("set S3A_SEED_DB=/abs/path/data.db to write the slice-3a acceptance store")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if os.Getenv("S3A_SEED_EMPTY") != "" {
		t.Logf("created empty store at %s", path)
		return
	}
	if err := s3aSeed(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %s", path)
}

func TestS3ASeedIsDeterministic(t *testing.T) {
	path := s3aSeededDB(t)
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	if n, _ := store.CountReviews(ctx, st, store.ReviewFilter{AppID: "6448311069"}); n != len(s3aChatGPTReviews) {
		t.Fatalf("seeded reviews = %d, want %d", n, len(s3aChatGPTReviews))
	}
	snaps, _ := store.AppSnapshots(ctx, st, "324684580", "us", time.Time{})
	if len(snaps) != 3 || snaps[0].ContentHash != snaps[1].ContentHash || snaps[1].ContentHash == snaps[2].ContentHash {
		t.Fatalf("spotify snapshots: %d, hashes must be equal for 1-2 and differ for 2-3", len(snaps))
	}
	for key, want := range map[store.ChartKey]int{s3aKeyGames: 2, s3aKeyPaidGB: 1, s3aKeyGrossing: 3} {
		caps, err := store.ChartCaptures(ctx, st, key)
		if err != nil || len(caps) != want {
			t.Fatalf("captures of %+v = %d (%v), want %d", key, len(caps), err, want)
		}
	}
}
