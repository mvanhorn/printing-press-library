// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
)

// withTempStore swaps in a temp DB so each test gets a clean slate.
// The CLI commands resolve their DB via defaultDBPath, which reads
// the user home dir. We point HOME at a temp dir so the test's DB
// path lands inside it.
func withTempStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmp); err != nil {
		t.Fatalf("setenv HOME: %v", err)
	}
	dbPath := filepath.Join(tmp, ".local", "share", "coffee-goat-pp-cli", "data.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return s, func() {
		_ = s.Close()
		_ = os.Setenv("HOME", origHome)
	}
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	rootCmd := newRootCmd(&flags)
	rootCmd.SetArgs(args)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	err := rootCmd.Execute()
	return buf.String(), err
}

func seedProduct(t *testing.T, s *store.Store, roaster, handle string, fields map[string]any) {
	t.Helper()
	if err := s.UpsertRoasterProduct(roaster, handle, fields); err != nil {
		t.Fatalf("seed product %s/%s: %v", roaster, handle, err)
	}
}

func TestSearchAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	seedProduct(t, s, "onyx", "ethiopia-natural", map[string]any{
		"title":    "Ethiopia Natural Adola",
		"origin":   "Ethiopia",
		"process":  "natural",
		"in_stock": 1,
	})
	seedProduct(t, s, "sey", "ethiopia-washed", map[string]any{
		"title":    "Ethiopia Washed Worka",
		"origin":   "Ethiopia",
		"process":  "washed",
		"in_stock": 1,
	})
	seedProduct(t, s, "prodigal", "colombia-pink", map[string]any{
		"title":    "Colombia Pink Bourbon",
		"origin":   "Colombia",
		"process":  "washed",
		"in_stock": 1,
	})

	out, err := runCmd(t, "search", "ethiopia", "--json")
	if err != nil {
		t.Fatalf("search: %v\nout=%s", err, out)
	}
	t.Logf("search output: %s", out)
	var hits []searchHit
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &hits); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d (hits=%+v)", len(hits), hits)
	}
	for _, h := range hits {
		if !strings.Contains(strings.ToLower(h.Title), "ethiopia") {
			t.Errorf("hit %q does not match ethiopia query", h.Title)
		}
	}
}

func TestWatchAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	seedProduct(t, s, "onyx", "kenya-gathaithi", map[string]any{
		"title":    "Kenya Gathaithi",
		"origin":   "Kenya",
		"process":  "washed",
		"in_stock": 1,
	})

	// Save a watch.
	out, err := runCmd(t, "watch", "save", "kenyas", "kenya")
	if err != nil {
		t.Fatalf("watch save: %v\nout=%s", err, out)
	}

	// First run should emit the match.
	out, err = runCmd(t, "watch", "run", "--json")
	if err != nil {
		t.Fatalf("watch run #1: %v\nout=%s", err, out)
	}
	t.Logf("watch run #1: %s", out)
	if !strings.Contains(out, "kenya") && !strings.Contains(out, "Kenya") {
		t.Fatalf("first watch run should mention kenya match; got %q", out)
	}

	// Second run should be empty / no new items (anchor advanced).
	out, err = runCmd(t, "watch", "run", "--json")
	if err != nil {
		t.Fatalf("watch run #2: %v\nout=%s", err, out)
	}
	t.Logf("watch run #2: %s", out)
	// JSON output should be either empty array or no results entry.
	trimmed := strings.TrimSpace(out)
	if trimmed != "null" && trimmed != "[]" && trimmed != "" {
		// Tolerate empty-list outputs; flag substantive output.
		if strings.Contains(trimmed, "kenya-gathaithi") || strings.Contains(trimmed, "Kenya Gathaithi") {
			t.Fatalf("second watch run should be empty but emitted: %s", out)
		}
	}
}

func TestTwinAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	// Reference bean: Ethiopian natural Heirloom at 2000m
	seedProduct(t, s, "onyx", "ref", map[string]any{
		"title":    "Ethiopia Natural Adola",
		"origin":   "Ethiopia",
		"process":  "natural",
		"varietal": "Heirloom",
		"altitude": "2000 masl",
	})
	// Twin candidate: same origin/process/varietal/altitude band.
	seedProduct(t, s, "sey", "twin", map[string]any{
		"title":    "Ethiopia Natural Sidamo",
		"origin":   "Ethiopia",
		"process":  "natural",
		"varietal": "Heirloom",
		"altitude": "2000 masl",
	})
	// Distant candidate: different attrs.
	seedProduct(t, s, "verve", "distant1", map[string]any{
		"title":    "Brazil Yellow Bourbon",
		"origin":   "Brazil",
		"process":  "natural",
		"varietal": "Yellow Bourbon",
		"altitude": "1100 masl",
	})
	seedProduct(t, s, "april", "distant2", map[string]any{
		"title":    "Colombia Washed Castillo",
		"origin":   "Colombia",
		"process":  "washed",
		"varietal": "Castillo",
		"altitude": "1700 masl",
	})
	seedProduct(t, s, "the-barn", "distant3", map[string]any{
		"title":    "Guatemala Honey Bourbon",
		"origin":   "Guatemala",
		"process":  "honey",
		"varietal": "Bourbon",
		"altitude": "1500 masl",
	})

	out, err := runCmd(t, "twin", "ref", "--json", "--top", "3")
	if err != nil {
		t.Fatalf("twin: %v\nout=%s", err, out)
	}
	t.Logf("twin output: %s", out)
	var results []twinResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &results); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 twin result")
	}
	if results[0].Handle == "ref" {
		t.Errorf("top result should not be the input bean itself")
	}
	if results[0].Handle != "twin" {
		t.Errorf("top result should be 'twin' (closest by similarity), got %q", results[0].Handle)
	}
}

func TestCreatorReviewAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	// One video mentions onyx, one doesn't.
	_, err := s.DB().Exec(
		`INSERT INTO youtube_reviews (video_id, creator, channel_id, video_title, video_published_at, transcript_text, mentioned_roaster_slugs_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"vid-onyx", "hoffmann", "UC1", "I tried Onyx", "2026-04-01T00:00:00Z", "Onyx coffee is bright and floral",
		`["onyx"]`,
	)
	if err != nil {
		t.Fatalf("seed yt 1: %v", err)
	}
	_, err = s.DB().Exec(
		`INSERT INTO youtube_reviews (video_id, creator, channel_id, video_title, video_published_at, transcript_text, mentioned_roaster_slugs_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"vid-other", "hedrick", "UC2", "Espresso grind size talk", "2026-04-02T00:00:00Z", "Today I'm not talking about any specific roaster", "",
	)
	if err != nil {
		t.Fatalf("seed yt 2: %v", err)
	}

	out, err := runCmd(t, "creator-review", "onyx", "--json")
	if err != nil {
		t.Fatalf("creator-review: %v\nout=%s", err, out)
	}
	t.Logf("creator-review output: %s", out)
	var clips []creatorReviewClip
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &clips); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if len(clips) != 1 {
		t.Fatalf("expected exactly 1 clip, got %d (clips=%+v)", len(clips), clips)
	}
	if clips[0].Creator != "hoffmann" {
		t.Errorf("expected creator hoffmann, got %q", clips[0].Creator)
	}
}

func TestFlavorWheelAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	// Seed: 1 bean attached to a product with floral tag; 6 brews with rating>=7 mentioning "Jasmine" via product body.
	seedProduct(t, s, "onyx", "test-product", map[string]any{
		"title":     "Floral Ethiopia",
		"body_text": "Jasmine, floral, bright",
	})
	res, err := s.DB().Exec(`INSERT INTO beans (roaster_slug, product_slug) VALUES (?, ?)`, "onyx", "test-product")
	if err != nil {
		t.Fatalf("seed bean: %v", err)
	}
	beanID, _ := res.LastInsertId()
	for i := 0; i < 6; i++ {
		_, err := s.DB().Exec(
			`INSERT INTO brews (bean_id, method, rating) VALUES (?, ?, ?)`,
			beanID, "v60", 9,
		)
		if err != nil {
			t.Fatalf("seed brew: %v", err)
		}
	}

	out, err := runCmd(t, "flavor-wheel", "--json")
	if err != nil {
		t.Fatalf("flavor-wheel: %v\nout=%s", err, out)
	}
	t.Logf("flavor-wheel output: %s", out)
	var report flavorWheelReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &report); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if report.BrewsScored != 6 {
		t.Errorf("brews_scored = %d, want 6", report.BrewsScored)
	}
	// At least one preferred section.
	if len(report.Preferred) == 0 {
		t.Errorf("expected at least one preferred section; got %+v", report)
	}
	foundFloral := false
	for _, p := range report.Preferred {
		if strings.Contains(strings.ToLower(p.Section), "floral") || strings.Contains(strings.ToLower(p.Section), "jasmine") {
			foundFloral = true
			break
		}
	}
	if !foundFloral {
		t.Errorf("expected a Floral/Jasmine section in preferred; got %+v", report.Preferred)
	}
}

func TestFriendPickAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	// Self brews: 4 brews on an Ethiopia natural at rating 9 -> favours Ethiopia/natural tokens.
	seedProduct(t, s, "onyx", "eth-nat", map[string]any{
		"title":     "Ethiopia Natural",
		"origin":    "Ethiopia",
		"process":   "natural",
		"body_text": "jasmine floral",
	})
	res, err := s.DB().Exec(`INSERT INTO beans (roaster_slug, product_slug) VALUES (?, ?)`, "onyx", "eth-nat")
	if err != nil {
		t.Fatalf("seed bean: %v", err)
	}
	beanID, _ := res.LastInsertId()
	for i := 0; i < 4; i++ {
		_, _ = s.DB().Exec(`INSERT INTO brews (bean_id, method, rating) VALUES (?, ?, ?)`, beanID, "v60", 9)
	}
	// Market candidate.
	seedProduct(t, s, "sey", "eth-nat-2", map[string]any{
		"title":     "Ethiopia Natural Worka",
		"origin":    "Ethiopia",
		"process":   "natural",
		"body_text": "jasmine peach",
	})
	seedProduct(t, s, "verve", "brazil", map[string]any{
		"title":     "Brazil Yellow Bourbon",
		"origin":    "Brazil",
		"process":   "natural",
		"body_text": "chocolate nutty",
	})

	// Export -> import -> pick
	tmp := t.TempDir()
	palatePath := filepath.Join(tmp, "anne.palate.json")
	if out, err := runCmd(t, "friend-pick", "palate-export", "anne", "--out", palatePath); err != nil {
		t.Fatalf("palate-export: %v\nout=%s", err, out)
	}
	if _, err := os.Stat(palatePath); err != nil {
		t.Fatalf("palate file not written: %v", err)
	}

	if out, err := runCmd(t, "friend-pick", "palate-import", palatePath); err != nil {
		t.Fatalf("palate-import: %v\nout=%s", err, out)
	}

	out, err := runCmd(t, "friend-pick", "pick", "anne", "--json", "--top", "3")
	if err != nil {
		t.Fatalf("friend-pick pick: %v\nout=%s", err, out)
	}
	t.Logf("friend-pick output: %s", out)
	if !strings.Contains(out, "anne") {
		t.Errorf("expected rationale to reference friend name 'anne'; got %s", out)
	}
	if !strings.Contains(out, "candidates") {
		t.Errorf("expected non-empty results; got %s", out)
	}
}

func TestGodCupAcceptance(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	// 1 bean + 5+ brews + 1 review + 1 youtube_review.
	seedProduct(t, s, "onyx", "test-bean", map[string]any{
		"title":    "Onyx Test Bean",
		"origin":   "Ethiopia",
		"in_stock": 1,
	})
	res, err := s.DB().Exec(`INSERT INTO beans (roaster_slug, product_slug, roast_date) VALUES (?, ?, date('now', '-10 days'))`, "onyx", "test-bean")
	if err != nil {
		t.Fatalf("seed bean: %v", err)
	}
	beanID, _ := res.LastInsertId()
	for i := 0; i < 5; i++ {
		_, _ = s.DB().Exec(`INSERT INTO brews (bean_id, method, rating) VALUES (?, ?, ?)`, beanID, "v60", 8)
	}
	_, _ = s.DB().Exec(`INSERT INTO reviews (id, source, roaster_name, bean_name, score) VALUES (?, ?, ?, ?, ?)`, "rev-1", "coffeereview", "Onyx Coffee Lab", "Test Bean", 93)
	_, _ = s.DB().Exec(
		`INSERT INTO youtube_reviews (video_id, creator, channel_id, video_title, video_published_at, transcript_text, mentioned_roaster_slugs_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"vid-1", "hoffmann", "UC1", "Onyx review", "2026-04-01T00:00:00Z", "Onyx makes great coffee", `["onyx"]`,
	)

	out, err := runCmd(t, "god-cup", "--json")
	if err != nil {
		t.Fatalf("god-cup: %v\nout=%s", err, out)
	}
	t.Logf("god-cup output: %s", out)
	var report godCupReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &report); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if report.BrewPick == nil {
		t.Fatalf("expected non-nil brew_pick; got %+v", report)
	}
	if !strings.Contains(report.BrewPick.Rationale, "freshness") && !strings.Contains(report.BrewPick.Rationale, "dial-in") {
		t.Errorf("rationale should mention specific factors; got %q", report.BrewPick.Rationale)
	}
}
