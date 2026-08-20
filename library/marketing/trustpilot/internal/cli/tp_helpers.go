// Trustpilot-specific helpers shared across every hand-written command.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	tpkg "github.com/mvanhorn/printing-press-library/library/marketing/trustpilot/internal/trustpilot"

	_ "modernc.org/sqlite"
)

const (
	NoticeLocalStoreEmpty           = "local_store_empty_for_domain"
	NoticeIsCollectingFalse         = "isCollectingReviews_false"
	NoticeLastReviewOlderThanWindow = "last_review_older_than_window"
	NoticeHistogramEmptyFromAPI     = "histogram_empty_from_api"
	NoticeHistogramKeysUnrecognized = "histogram_keys_unrecognized"
	NoticeAISummaryEmpty            = "ai_summary_empty_from_api"
	NoticeTopicsEmpty               = "topics_empty_from_api"
	NoticeTrustpilotCapHit          = "trustpilot_filter_cap_hit"
	NoticeLiveFallback              = "live_fallback_after_local_miss"
)

// PATCH: Add agent-readable meta envelopes and notice codes to read-command JSON.
type Meta struct {
	Source              string     `json:"source"`
	Notices             []string   `json:"notices"`
	NewestReviewAt      *time.Time `json:"newestReviewAt,omitempty"`
	LocalCount          *int       `json:"localCount,omitempty"`
	IsCollectingReviews *bool      `json:"isCollectingReviews,omitempty"`
	FetchedAt           time.Time  `json:"fetchedAt"`
	Mode                string     `json:"mode,omitempty"`
}

func NewMeta(source string) Meta {
	return Meta{Source: source, Notices: []string{}, FetchedAt: time.Now().UTC()}
}

func (m *Meta) AddNotice(code string) {
	if code == "" {
		return
	}
	for _, existing := range m.Notices {
		if existing == code {
			return
		}
	}
	m.Notices = append(m.Notices, code)
}

func attachMeta(payload map[string]any, meta Meta) {
	payload["meta"] = meta
}

func intPtr(v int) *int {
	return &v
}

func newestReviewAt(groups ...[]tpkg.Review) *time.Time {
	var newest time.Time
	for _, reviews := range groups {
		for _, r := range reviews {
			if r.PublishedDate.IsZero() {
				continue
			}
			if newest.IsZero() || r.PublishedDate.After(newest) {
				newest = r.PublishedDate
			}
		}
	}
	if newest.IsZero() {
		return nil
	}
	newest = newest.UTC()
	return &newest
}

func addReviewFreshnessMeta(meta *Meta, newest *time.Time, minDate time.Time) {
	meta.NewestReviewAt = newest
	if minDate.IsZero() {
		return
	}
	if newest != nil && newest.Before(minDate) {
		meta.AddNotice(NoticeLastReviewOlderThanWindow)
	}
}

func addBusinessUnitMeta(meta *Meta, bu tpkg.BusinessUnit) {
	collecting := bu.IsCollectingReviews
	meta.IsCollectingReviews = &collecting
	if !collecting {
		meta.AddNotice(NoticeIsCollectingFalse)
	}
}

// PATCH: Resolve auto/local/live reads before Trustpilot-specific commands fetch data.
func resolveDataSource(flags *rootFlags, hasLocalData bool) (source string, fallback bool) {
	if hasLocalData {
		return "local", false
	}
	if flags != nil && flags.dataSource == "live" {
		return "live", false
	}
	if flags != nil && flags.dataSource == "local" {
		return "local", false
	}
	return "live (fallback)", true
}

// tpDBPath returns the local SQLite path. Mirrors the generator's
// defaultDBPath shape but keeps the Trustpilot CLI's data in its own file.
func tpDBPath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		if runtime.GOOS == "darwin" {
			base = filepath.Join(os.Getenv("HOME"), "Library", "Caches")
		} else {
			base = filepath.Join(os.Getenv("HOME"), ".cache")
		}
	}
	dir := filepath.Join(base, "trustpilot-pp-cli")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "trustpilot.db")
}

// openTPStore opens the SQLite file and ensures the Trustpilot schema is in
// place. Caller is responsible for db.Close().
func openTPStore(ctx context.Context) (*sql.DB, error) {
	// PATCH(greptile P1 PR#588): pass DSN pragmas so concurrent CLI invocations
	// (e.g. sync-trustpilot + search-reviews) wait on writer locks instead of
	// getting immediate SQLITE_BUSY. Matches store.OpenWithContext defaults.
	dsn := tpDBPath() + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open trustpilot store: %w", err)
	}
	if err := tpkg.EnsureSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// loadOrHarvestSession returns the persisted session if it's still within the
// freshness window; otherwise it runs a Chrome harvest and persists the new
// session.
func loadOrHarvestSession(ctx context.Context, db *sql.DB, allowHarvest bool) (tpkg.Session, bool, error) {
	s, err := tpkg.LoadSession(ctx, db)
	if err != nil {
		return tpkg.Session{}, false, err
	}
	if s.IsFresh() {
		return s, false, nil
	}
	if !allowHarvest {
		return s, false, fmt.Errorf("no fresh Trustpilot session; run `trustpilot-pp-cli auth login --chrome`")
	}
	fmt.Fprintln(os.Stderr, "Trustpilot session is missing or stale; launching Chrome to harvest WAF cookie (about 10 seconds)...")
	fresh, err := tpkg.HarvestSession(ctx, tpkg.HarvestOptions{})
	if err != nil {
		return tpkg.Session{}, false, err
	}
	if err := tpkg.SaveSession(ctx, db, fresh); err != nil {
		return tpkg.Session{}, false, err
	}
	return fresh, true, nil
}

// fetchPageWithRetry retries once on cookie expiry. After re-harvest the
// caller's session reference is updated in place.
func fetchPageWithRetry(ctx context.Context, db *sql.DB, sess *tpkg.Session, domain string, filters tpkg.PageFilters) (tpkg.ReviewsPage, error) {
	cli := tpkg.NewClient(*sess)
	page, err := cli.FetchPage(ctx, domain, filters)
	if err == nil {
		return page, nil
	}
	// PATCH: Unsupported filter errors are deterministic; session refresh will not fix them.
	if _, unsupported := err.(*tpkg.FilterUnsupportedError); unsupported {
		return tpkg.ReviewsPage{}, err
	}
	if _, expired := err.(*tpkg.CookieExpiredError); !expired {
		if _, stale := err.(*tpkg.BuildIDStaleError); !stale {
			return tpkg.ReviewsPage{}, err
		}
	}
	fresh, _, rerr := loadOrHarvestSession(ctx, db, true)
	if rerr != nil {
		return tpkg.ReviewsPage{}, fmt.Errorf("re-harvest after cookie/buildid failure: %w (original: %v)", rerr, err)
	}
	*sess = fresh
	cli = tpkg.NewClient(*sess)
	return cli.FetchPage(ctx, domain, filters)
}

// fetchSearchWithRetry mirrors fetchPageWithRetry for the search endpoint.
func fetchSearchWithRetry(ctx context.Context, db *sql.DB, sess *tpkg.Session, query string) ([]tpkg.SearchHit, error) {
	cli := tpkg.NewClient(*sess)
	hits, err := cli.FetchSearch(ctx, query)
	if err == nil {
		return hits, nil
	}
	if _, expired := err.(*tpkg.CookieExpiredError); !expired {
		if _, stale := err.(*tpkg.BuildIDStaleError); !stale {
			return nil, err
		}
	}
	fresh, _, rerr := loadOrHarvestSession(ctx, db, true)
	if rerr != nil {
		return nil, fmt.Errorf("re-harvest after cookie/buildid failure: %w (original: %v)", rerr, err)
	}
	*sess = fresh
	cli = tpkg.NewClient(*sess)
	return cli.FetchSearch(ctx, query)
}

// normalizeDomain accepts a user-supplied company key in any of the shapes
// Trustpilot understands (bare domain, domain with www, full URL) and returns
// the canonical "www.thriftbooks.com" form Trustpilot uses as its identifying
// name.
func normalizeDomain(input string) string {
	d := input
	// Strip scheme
	for _, prefix := range []string{"https://", "http://"} {
		if len(d) > len(prefix) && d[:len(prefix)] == prefix {
			d = d[len(prefix):]
			break
		}
	}
	// Strip path
	if idx := indexByte(d, '/'); idx >= 0 {
		d = d[:idx]
	}
	return d
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
