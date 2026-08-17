// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the hand-authored Discogs transcendence commands
// (fills, portfolio, undervalued, comps, sell-plan, identify, pressings) and
// the hand-authored sync/limit commands. Kept in its own file so
// `generate --force` preserves it.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/store"
)

// --- Discogs response shapes (only the fields these features need) ---

type discogsPriceValue struct {
	Value    *float64 `json:"value"`
	Currency string   `json:"currency"`
}

type discogsStats struct {
	LowestPrice *discogsPriceValue `json:"lowest_price"`
	NumForSale  *int               `json:"num_for_sale"`
	Blocked     bool               `json:"blocked_from_sale"`
}

type discogsIdentity struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type discogsBasicInfo struct {
	ID       int64            `json:"id"`
	Title    string           `json:"title"`
	Year     int              `json:"year"`
	Artists  []discogsArtist  `json:"artists"`
	Labels   []discogsLabelBI `json:"labels"`
	Formats  []discogsFormat  `json:"formats"`
	ThumbURL string           `json:"thumb"`
	MasterID int64            `json:"master_id"`
}

type discogsArtist struct {
	Name string `json:"name"`
}

type discogsLabelBI struct {
	Name  string `json:"name"`
	Catno string `json:"catno"`
}

type discogsFormat struct {
	Name         string   `json:"name"`
	Descriptions []string `json:"descriptions"`
}

// wantItem is one entry from GET /users/{u}/wants — the top-level id is the
// release id.
type wantItem struct {
	ID        int64            `json:"id"`
	Rating    int              `json:"rating"`
	Notes     string           `json:"notes"`
	BasicInfo discogsBasicInfo `json:"basic_information"`
}

// collectionItem is one entry from a collection folder's releases list.
type collectionItem struct {
	ID         int64            `json:"id"`
	InstanceID int64            `json:"instance_id"`
	FolderID   int64            `json:"folder_id"`
	Rating     int              `json:"rating"`
	BasicInfo  discogsBasicInfo `json:"basic_information"`
}

func (b discogsBasicInfo) artistName() string {
	if len(b.Artists) > 0 {
		return b.Artists[0].Name
	}
	return ""
}

func (b discogsBasicInfo) display() string {
	a := b.artistName()
	if a != "" && b.Title != "" {
		return a + " – " + b.Title
	}
	if b.Title != "" {
		return b.Title
	}
	return fmt.Sprintf("release %d", b.ID)
}

// checkDataSource enforces the command's declared data-source strategy against
// the global --data-source flag. "live" commands reject --data-source local;
// "local" commands reject --data-source live; "auto"/empty always passes.
func checkDataSource(flags *rootFlags, strategy string) error {
	switch flags.dataSource {
	case "", "auto":
		return nil
	case "local":
		if strategy == "live" {
			return usageErr(fmt.Errorf("--data-source local is not supported: this command queries Discogs live"))
		}
	case "live":
		if strategy == "local" {
			return usageErr(fmt.Errorf("--data-source live is not supported: this command reads only local synced data (run 'sync' first)"))
		}
	}
	return nil
}

// --- store open ---

// openDiscogsStore opens the local mirror read-write and ensures the
// hand-authored tables exist. OpenWithContext creates the db file and runs
// the base migrations; EnsureDiscogsTables adds price_snapshots /
// wantlist_limits / discogs_meta. Readers get empty tables on first run
// rather than a missing-table error.
func openDiscogsStore(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	st, err := store.OpenWithContext(ctx, defaultDBPath("discogs-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := st.EnsureDiscogsTables(ctx); err != nil {
		_ = st.Close()
		return nil, err
	}
	return st, nil
}

// --- username resolution ---

// resolveUsername returns the username to use for user-scoped endpoints.
// Order: explicit --user flag, cached value in discogs_meta, then a live
// GET /oauth/identity (which caches the result). Returns an actionable error
// when no token is set and no --user was given.
func resolveUsername(ctx context.Context, c *client.Client, st *store.Store, userFlag string) (string, error) {
	if userFlag != "" {
		return userFlag, nil
	}
	if st != nil {
		if u, ok := st.MetaGet(ctx, "username"); ok {
			return u, nil
		}
	}
	data, err := c.Get(ctx, "/oauth/identity", nil)
	if err != nil {
		return "", fmt.Errorf("could not resolve your username: pass --user, or set DISCOGS_TOKEN (get one at discogs.com/settings/developers): %w", err)
	}
	var id discogsIdentity
	if err := json.Unmarshal(data, &id); err != nil || id.Username == "" {
		return "", fmt.Errorf("could not resolve your username from /oauth/identity; pass --user")
	}
	if st != nil {
		_ = st.MetaSet(ctx, "username", id.Username)
	}
	return id.Username, nil
}

// --- marketplace stats + snapshots ---

// fetchStats calls GET /marketplace/stats/{release_id} and parses it.
func fetchStats(ctx context.Context, c *client.Client, releaseID int64, currency string) (discogsStats, error) {
	params := map[string]string{}
	if currency != "" {
		params["curr_abbr"] = currency
	}
	data, err := c.Get(ctx, fmt.Sprintf("/marketplace/stats/%d", releaseID), params)
	if err != nil {
		return discogsStats{}, err
	}
	var s discogsStats
	if err := json.Unmarshal(data, &s); err != nil {
		return discogsStats{}, fmt.Errorf("parsing marketplace stats for release %d: %w", releaseID, err)
	}
	return s, nil
}

// captureSnapshot fetches current marketplace stats for a release and records
// a price_snapshots row. Returns the parsed stats so callers can reuse them
// without a second call.
func captureSnapshot(ctx context.Context, c *client.Client, st *store.Store, releaseID int64, currency string) (discogsStats, error) {
	s, err := fetchStats(ctx, c, releaseID, currency)
	if err != nil {
		return discogsStats{}, err
	}
	var lowest sql.NullFloat64
	cur := currency
	if s.LowestPrice != nil && s.LowestPrice.Value != nil {
		lowest = sql.NullFloat64{Float64: *s.LowestPrice.Value, Valid: true}
		if s.LowestPrice.Currency != "" {
			cur = s.LowestPrice.Currency
		}
	}
	var numForSale sql.NullInt64
	if s.NumForSale != nil {
		numForSale = sql.NullInt64{Int64: int64(*s.NumForSale), Valid: true}
	}
	if err := st.InsertPriceSnapshot(ctx, releaseID, cur, lowest, numForSale, s.Blocked, "stats"); err != nil {
		return s, fmt.Errorf("recording price snapshot for release %d: %w", releaseID, err)
	}
	return s, nil
}

type snapshotRow struct {
	CapturedAt string
	Lowest     sql.NullFloat64
	NumForSale sql.NullInt64
	Currency   string
}

// latestSnapshot returns the most recent snapshot for a release, or ok=false.
func latestSnapshot(ctx context.Context, st *store.Store, releaseID int64) (snapshotRow, bool) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT captured_at, lowest_price, num_for_sale, COALESCE(currency,'') FROM price_snapshots
		 WHERE release_id = ? ORDER BY captured_at DESC LIMIT 1`, releaseID)
	if err != nil {
		return snapshotRow{}, false
	}
	defer rows.Close()
	if !rows.Next() {
		return snapshotRow{}, false
	}
	var r snapshotRow
	if err := rows.Scan(&r.CapturedAt, &r.Lowest, &r.NumForSale, &r.Currency); err != nil {
		return snapshotRow{}, false
	}
	return r, true
}

// priorLowest returns the most recently recorded lowest_price for a release —
// the "since last check" baseline. Call it BEFORE recording this run's fresh
// snapshot, so the newest stored row is genuinely from the prior check.
func priorLowest(ctx context.Context, st *store.Store, releaseID int64) (float64, bool) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT lowest_price FROM price_snapshots
		 WHERE release_id = ? AND lowest_price IS NOT NULL
		 ORDER BY captured_at DESC LIMIT 1`, releaseID)
	if err != nil {
		return 0, false
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, false
	}
	var v sql.NullFloat64
	if err := rows.Scan(&v); err != nil || !v.Valid {
		return 0, false
	}
	return v.Float64, true
}

// trailingMedian computes the median of recorded lowest_price values for a
// release (drain-first: pull all values, close, compute in Go). ok=false when
// there is no price history yet.
func trailingMedian(ctx context.Context, st *store.Store, releaseID int64) (float64, bool) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT lowest_price FROM price_snapshots
		 WHERE release_id = ? AND lowest_price IS NOT NULL
		 ORDER BY captured_at`, releaseID)
	if err != nil {
		return 0, false
	}
	var vals []float64
	for rows.Next() {
		var v sql.NullFloat64
		if err := rows.Scan(&v); err != nil {
			continue
		}
		if v.Valid {
			vals = append(vals, v.Float64)
		}
	}
	_ = rows.Err()
	_ = rows.Close()
	if len(vals) == 0 {
		return 0, false
	}
	sort.Float64s(vals)
	n := len(vals)
	if n%2 == 1 {
		return vals[n/2], true
	}
	return (vals[n/2-1] + vals[n/2]) / 2, true
}
