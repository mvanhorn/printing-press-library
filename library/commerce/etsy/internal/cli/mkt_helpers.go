// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written shared helpers for the `market` command group (Slice 4).

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const mktCLIName = "etsy-pp-cli"

const mktNoSnapshotHint = `no market snapshots yet; run 'etsy-pp-cli market snapshot --term "SEARCH TERM"' first`

// mktParseWindow parses a window such as 12mo, 1y, 90d, 4w. "mo" is 30 days
// and "y" is 365 days; everything else defers to cliutil.ParseDurationLoose.
func mktParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	for _, u := range []struct {
		suffix string
		d      time.Duration
	}{{"mo", 30 * 24 * time.Hour}, {"y", 365 * 24 * time.Hour}} {
		if strings.HasSuffix(s, u.suffix) {
			n, err := strconv.Atoi(strings.TrimSuffix(s, u.suffix))
			if err != nil || n < 0 {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			return time.Duration(n) * u.d, nil
		}
	}
	return cliutil.ParseDurationLoose(s)
}

// mktWindowLabel renders a window parsed by mktParseWindow in plain words
// ("1 year", "6 months", "30 days") instead of a Go duration like 8760h0m0s.
func mktWindowLabel(d time.Duration) string {
	day := 24 * time.Hour
	switch {
	case d > 0 && d%(365*day) == 0:
		return mktCount(int(d/(365*day)), "year")
	case d > 0 && d%(30*day) == 0:
		return mktCount(int(d/(30*day)), "month")
	case d > 0 && d%day == 0:
		return mktCount(int(d/day), "day")
	}
	return d.String()
}

// mktCount formats n with a singular or plural noun ("1 listing", "2 listings").
func mktCount(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// mktSnapshot is a row of market_snapshots.
type mktSnapshot struct {
	ID               int64     `json:"id"`
	TakenAt          string    `json:"taken_at"`
	Terms            []string  `json:"terms"`
	Pages            int       `json:"pages"`
	SortOn           string    `json:"sort_on"`
	MaxCalls         int       `json:"max_calls"`
	CallsUsed        int       `json:"calls_used"`
	Status           string    `json:"status"`
	WithReviews      bool      `json:"with_reviews"`
	ReviewWindowDays int       `json:"review_window_days"`
	Progress         mktProg   `json:"-"`
	takenTime        time.Time `json:"-"`
}

// mktProg tracks completed work so a partial snapshot can be resumed.
type mktProg struct {
	ReviewsCapped []int64  `json:"reviews_capped"`
	Pages         []string `json:"pages"`        // "term|page"
	ReviewsDone   []int64  `json:"reviews_done"` // listing ids whose reviews were fetched
}

func (p *mktProg) hasPage(term string, page int) bool {
	k := fmt.Sprintf("%s|%d", term, page)
	for _, x := range p.Pages {
		if x == k {
			return true
		}
	}
	return false
}

func (p *mktProg) hasReview(id int64) bool {
	for _, x := range p.ReviewsDone {
		if x == id {
			return true
		}
	}
	return false
}

const mktSnapshotCols = `id, taken_at, terms_json, pages, sort_on, max_calls, calls_used, status, with_reviews, review_window_days, progress_json`

func mktScanSnapshot(sc interface{ Scan(...any) error }) (*mktSnapshot, error) {
	var s mktSnapshot
	var terms, prog string
	var wr int
	if err := sc.Scan(&s.ID, &s.TakenAt, &terms, &s.Pages, &s.SortOn, &s.MaxCalls, &s.CallsUsed, &s.Status, &wr, &s.ReviewWindowDays, &prog); err != nil {
		return nil, err
	}
	s.WithReviews = wr != 0
	_ = json.Unmarshal([]byte(terms), &s.Terms)
	_ = json.Unmarshal([]byte(prog), &s.Progress)
	if s.Terms == nil {
		s.Terms = []string{}
	}
	s.takenTime, _ = time.Parse(time.RFC3339, s.TakenAt)
	return &s, nil
}

// mktHasTables reports whether the market tables exist (they are lazy).
func mktHasTables(db *sql.DB) bool {
	var n int
	err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='market_snapshots'`).Scan(&n)
	return err == nil && n > 0
}

// mktOpenRead opens the local store read-only. A nil db with nil error means
// "nothing collected yet" (no store file, or no market tables).
func mktOpenRead(ctx context.Context) (*store.Store, error) {
	db, err := openStoreForRead(ctx, mktCLIName)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	if !mktHasTables(db.DB()) {
		_ = db.Close()
		return nil, nil
	}
	return db, nil
}

func mktGetSnapshot(db *sql.DB, id int64) (*mktSnapshot, error) {
	s, err := mktScanSnapshot(db.QueryRow(`SELECT `+mktSnapshotCols+` FROM market_snapshots WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// mktCompleteSnapshots returns complete snapshots, newest first.
func mktCompleteSnapshots(db *sql.DB, limit int) ([]*mktSnapshot, error) {
	q := `SELECT ` + mktSnapshotCols + ` FROM market_snapshots WHERE status = 'complete' ORDER BY id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit) // #nosec G202 -- limit is an int rendered with %d onto a compile-time column list.
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*mktSnapshot, 0)
	for rows.Next() {
		s, err := mktScanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

const mktSnapshotUsage = "Positive snapshot ID; omit for the latest complete snapshot"

// mktPositiveIDValue suppresses help defaults while preserving int64 parsing.
type mktPositiveIDValue struct{ pflag.Value }

func (v *mktPositiveIDValue) String() string {
	if v.Value.String() == "0" {
		return ""
	}
	return v.Value.String()
}

// mktHideZeroDefault keeps omitted IDs empty in help and MCP metadata.
// The literal flag registration stays at each command for static discovery.
func mktHideZeroDefault(cmd *cobra.Command, name string) {
	flag := cmd.Flags().Lookup(name)
	flag.Value = &mktPositiveIDValue{Value: flag.Value}
	flag.DefValue = ""
}

// mktValidateSnapshotFlag distinguishes an explicit ID from the default latest selection.
func mktValidateSnapshotFlag(cmd *cobra.Command, id int64) error {
	if cmd.Flags().Changed("snapshot") && id <= 0 {
		return usageErr(fmt.Errorf("--snapshot must be a positive snapshot ID (omit it for the latest complete snapshot)"))
	}
	return nil
}

// mktPickSnapshot resolves --snapshot ID or the latest complete snapshot.
// A nil snapshot with a note means there is nothing to analyze.
func mktPickSnapshot(db *store.Store, id int64) (*mktSnapshot, string, error) {
	if db == nil {
		if id > 0 {
			return nil, "", notFoundErr(fmt.Errorf("snapshot %d not found", id))
		}
		return nil, mktNoSnapshotHint, nil
	}
	if id > 0 {
		s, err := mktGetSnapshot(db.DB(), id)
		if err != nil {
			return nil, "", err
		}
		if s == nil {
			return nil, "", notFoundErr(fmt.Errorf("snapshot %d not found", id))
		}
		return s, "", nil
	}
	snaps, err := mktCompleteSnapshots(db.DB(), 1)
	if err != nil {
		return nil, "", err
	}
	if len(snaps) == 0 {
		var count int
		if err := db.DB().QueryRow(`SELECT count(*) FROM market_snapshots`).Scan(&count); err != nil {
			return nil, "", err
		}
		if count == 0 {
			return nil, mktNoSnapshotHint, nil
		}
		return nil, "no complete market snapshot yet (partial snapshots can be analyzed with --snapshot ID, or finished with 'market snapshot --resume ID')", nil
	}
	return snaps[0], "", nil
}

// mktRow is a row of market_results with NULL-safe columns.
type mktRow struct {
	Term        string
	Page        int
	Position    int
	ListingID   int64
	ShopID      sql.NullInt64
	Title       string
	Price       sql.NullFloat64 // major units
	Currency    string
	Favorers    sql.NullInt64
	Views       sql.NullInt64
	Tags        []string
	TaxonomyID  sql.NullInt64
	CreatedTS   sql.NullInt64
	IsDigital   sql.NullBool
	ResultCount sql.NullInt64
}

func mktLoadResults(db *sql.DB, snapshotID int64) ([]mktRow, error) {
	rows, err := db.Query(`SELECT term, page, position, listing_id, shop_id, title, price_amount, price_divisor, currency,
		num_favorers, views, tags_json, taxonomy_id, original_creation_ts, is_digital, result_count
		FROM market_results WHERE snapshot_id = ? ORDER BY term, position, listing_id`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]mktRow, 0)
	for rows.Next() {
		var r mktRow
		var title, cur, tags sql.NullString
		var amt, div sql.NullInt64
		var dig sql.NullInt64
		if err := rows.Scan(&r.Term, &r.Page, &r.Position, &r.ListingID, &r.ShopID, &title, &amt, &div, &cur,
			&r.Favorers, &r.Views, &tags, &r.TaxonomyID, &r.CreatedTS, &dig, &r.ResultCount); err != nil {
			return nil, err
		}
		r.Title, r.Currency = title.String, cur.String
		if amt.Valid && div.Valid && div.Int64 != 0 {
			r.Price = sql.NullFloat64{Float64: float64(amt.Int64) / float64(div.Int64), Valid: true}
		}
		if dig.Valid {
			r.IsDigital = sql.NullBool{Bool: dig.Int64 != 0, Valid: true}
		}
		if tags.Valid {
			_ = json.Unmarshal([]byte(tags.String), &r.Tags)
		}
		if r.Tags == nil {
			r.Tags = []string{}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// mktShop is a row of market_shops (NULL-safe).
type mktShop struct {
	Name      sql.NullString
	ShopID    int64
	Sold      sql.NullInt64
	Active    sql.NullInt64
	Created   sql.NullInt64
	RatingAvg sql.NullFloat64
	Reviews   sql.NullInt64
	Vacation  sql.NullBool
}

func mktLoadShops(db *sql.DB, snapshotID int64) (map[int64]mktShop, error) {
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('market_shops') WHERE name='shop_name'`).Scan(&count); err != nil {
		return nil, err
	}
	nameColumn := "NULL"
	if count > 0 {
		nameColumn = "shop_name"
	}
	rows, err := db.Query(`SELECT shop_id, `+nameColumn+`, transaction_sold_count, listing_active_count, create_date, review_average, review_count, is_vacation
		FROM market_shops WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]mktShop{}
	for rows.Next() {
		var s mktShop
		var vac sql.NullInt64
		if err := rows.Scan(&s.ShopID, &s.Name, &s.Sold, &s.Active, &s.Created, &s.RatingAvg, &s.Reviews, &vac); err != nil {
			return nil, err
		}
		if vac.Valid {
			s.Vacation = sql.NullBool{Bool: vac.Int64 != 0, Valid: true}
		}
		out[s.ShopID] = s
	}
	return out, rows.Err()
}

// mktListingDemand is the per-listing review-volume view of a snapshot.
type mktListingDemand struct {
	Collected bool // review data was collected for this listing (0 is a real 0)
	Reviews   int  // reviews inside the window
}

// mktLoadDemand counts reviews inside window for every listing that has
// collected review data. A listing absent from the map has NO review data:
// that is "absent", not zero.
func mktLoadDemand(db *sql.DB, snap *mktSnapshot, window time.Duration) (map[int64]mktListingDemand, error) {
	out := map[int64]mktListingDemand{}
	for _, id := range snap.Progress.ReviewsDone {
		out[id] = mktListingDemand{Collected: true}
	}
	cutoff := snap.takenTime.Add(-window).Unix()
	rows, err := db.Query(`SELECT listing_id, created_ts FROM market_reviews WHERE snapshot_id = ?`, snap.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ts int64
		if err := rows.Scan(&id, &ts); err != nil {
			return nil, err
		}
		d := out[id]
		d.Collected = true
		if ts >= cutoff {
			d.Reviews++
		}
		out[id] = d
	}
	return out, rows.Err()
}

// mktRank returns average ranks (ties share the mean rank), 1-based.
func mktRank(v []float64) []float64 {
	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return v[idx[a]] < v[idx[b]] })
	r := make([]float64, len(v))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && v[idx[j+1]] == v[idx[i]] {
			j++
		}
		avg := float64(i+j)/2 + 1
		for k := i; k <= j; k++ {
			r[idx[k]] = avg
		}
		i = j + 1
	}
	return r
}

// mktSpearman returns Spearman's rho, or ok=false when it is undefined
// (fewer than 2 pairs, mismatched input, or a constant series).
func mktSpearman(x, y []float64) (float64, bool) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, false
	}
	rx, ry := mktRank(x), mktRank(y)
	n := float64(len(x))
	var mx, my float64
	for i := range rx {
		mx += rx[i]
		my += ry[i]
	}
	mx /= n
	my /= n
	var cov, vx, vy float64
	for i := range rx {
		cov += (rx[i] - mx) * (ry[i] - my)
		vx += (rx[i] - mx) * (rx[i] - mx)
		vy += (ry[i] - my) * (ry[i] - my)
	}
	if vx == 0 || vy == 0 {
		return 0, false
	}
	return cov / math.Sqrt(vx*vy), true
}

func mktMedian(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func mktRound(f float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(f*p) / p
}

// mktBestRows collapses a snapshot's rows to one per listing (best position).
func mktBestRows(rows []mktRow) []mktRow {
	best := map[int64]mktRow{}
	order := make([]int64, 0)
	for _, r := range rows {
		cur, ok := best[r.ListingID]
		if !ok {
			order = append(order, r.ListingID)
			best[r.ListingID] = r
		} else if r.Position < cur.Position {
			best[r.ListingID] = r
		}
	}
	out := make([]mktRow, 0, len(order))
	for _, id := range order {
		out = append(out, best[id])
	}
	return out
}

func mktNullInt(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

// mktReadLines reads a one-item-per-line file, skipping blanks and # comments.
func mktReadLines(path string) ([]string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied list file is this flag's documented input.
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, ln := range strings.Split(string(data), "\n") {
		if i := strings.Index(ln, "#"); i >= 0 {
			ln = ln[:i]
		}
		ln = strings.TrimSpace(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out, nil
}

func mktTitleWords(title string) []string {
	stop := map[string]bool{"the": true, "and": true, "for": true, "with": true, "your": true, "you": true, "this": true, "that": true, "from": true, "set": true, "are": true}
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, w := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(w) < 3 || stop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// mktLoadTermPages reads page outcomes without migrating older read-only stores.
func mktLoadTermPages(db *sql.DB, snapshotID int64) (map[string]int, error) {
	totals := make(map[string]int)
	var exists int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='market_term_pages'`).Scan(&exists); err != nil {
		return totals, err
	}
	if exists == 0 {
		return totals, nil
	}
	rows, err := db.Query(`SELECT term, SUM(listings) FROM market_term_pages WHERE snapshot_id = ? GROUP BY term`, snapshotID)
	if err != nil {
		return totals, err
	}
	defer rows.Close()
	for rows.Next() {
		var term string
		var total int
		if err := rows.Scan(&term, &total); err != nil {
			return totals, err
		}
		totals[term] = total
	}
	return totals, rows.Err()
}

// mktOlderReviewSnapshot finds the newest other complete review-bearing snapshot
// covering every required term, compared exactly as stored.
func mktOlderReviewSnapshot(db *sql.DB, selected *mktSnapshot, requiredTerms ...string) (*mktSnapshot, error) {
	if requiredTerms == nil {
		requiredTerms = selected.Terms
	}
	snapshots, err := mktCompleteSnapshots(db, 0)
	if err != nil {
		return nil, err
	}
	for _, snap := range snapshots {
		if snap.ID == selected.ID {
			continue
		}
		terms := make(map[string]bool, len(snap.Terms))
		for _, term := range snap.Terms {
			terms[term] = true
		}
		covers := true
		for _, term := range requiredTerms {
			if !terms[term] {
				covers = false
				break
			}
		}
		if !covers {
			continue
		}
		has, err := mktHasReviewData(db, snap)
		if err != nil {
			return nil, err
		}
		if has {
			return snap, nil
		}
	}
	return nil, nil
}

func mktHasReviewData(db *sql.DB, snap *mktSnapshot) (bool, error) {
	if len(snap.Progress.ReviewsDone) > 0 {
		return true, nil
	}
	var has bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM market_reviews WHERE snapshot_id = ?)`, snap.ID).Scan(&has)
	return has, err
}

func mktReviewSnapshotHint(id int64) string {
	return fmt.Sprintf("snapshot %d has review data; use --snapshot %d, or re-run market snapshot --with-reviews", id, id)
}

func mktQuoteJoin(terms []string) string {
	quoted := make([]string, len(terms))
	for i, term := range terms {
		quoted[i] = strconv.Quote(term)
	}
	return strings.Join(quoted, ", ")
}

// mktEmit preserves advisory notes through selection and agent compaction.
func mktEmit(cmd *cobra.Command, flags *rootFlags, results any, meta map[string]any, note string, notes ...string) error {
	m := map[string]any{}
	for k, v := range meta {
		if k != "results" && k != "notes" && k != "note" {
			m[k] = v
		}
	}
	if results != nil {
		v := reflect.ValueOf(results)
		switch v.Kind() {
		case reflect.Slice, reflect.Map, reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				results = nil
			}
		}
		if raw, ok := results.(json.RawMessage); ok && strings.TrimSpace(string(raw)) == "null" {
			results = nil
		}
	}
	if results == nil {
		results = []any{}
	}
	m["results"] = results
	unique := []string{}
	seen := map[string]bool{}
	if notes == nil {
		notes = []string{note}
	}
	for _, n := range notes {
		if n != "" && !seen[n] {
			unique = append(unique, n)
			seen[n] = true
		}
	}
	if note != "" {
		m["note"] = note
	}
	m["notes"] = unique
	if flags != nil && flags.selectFields != "" {
		delete(m, "notes")
		raw, err := json.Marshal(m)
		m["notes"] = unique
		if err != nil {
			return err
		}
		var paths [][]string
		var requested []string
		appendNotes := false
		notesRequested := false
		indeterminate := false
		for _, path := range strings.Split(flags.selectFields, ",") {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			parts := strings.Split(strings.ToLower(path), ".")
			notesRequested = notesRequested || parts[0] == "notes"
			paths = append(paths, parts)
			requested = append(requested, path)
			indeterminate = mktSelectIndeterminate(raw, parts) || indeterminate
			if _, exists := m[parts[0]]; exists && parts[0] != "notes" {
				appendNotes = true
			}
		}
		_, state := filterFieldsRec(raw, paths, true)
		if len(paths) > 0 && !state.matched && !indeterminate && !notesRequested {
			envelopeRaw, err := json.Marshal(m)
			if err != nil {
				return err
			}
			valid := strings.Join(selectFieldKeys(envelopeRaw), ", ")
			if valid == "" {
				valid = "none"
			}
			for _, path := range requested {
				fmt.Fprintf(os.Stderr, "warning: --select %q matched no fields; valid fields: %s\n", path, valid)
			}
			return usageErr(fmt.Errorf("--select matched no fields: %s", flags.selectFields))
		}
		if notesRequested {
			envelopeRaw, err := json.Marshal(m)
			if err != nil {
				return err
			}
			for i, path := range paths {
				if path[0] == "notes" {
					continue
				}
				_, fullState := filterFieldsRec(envelopeRaw, [][]string{path}, true)
				_, rowState := filterFieldsRec(raw, [][]string{path}, true)
				// Empty notes can make the generic fallback indeterminate.
				// Warn here only when that fallback hides a definite row miss.
				if !fullState.matched && (fullState.anchoredIndeterminate || fullState.fallbackIndeterminate) && !rowState.matched && !mktSelectIndeterminate(raw, path) {
					valid := strings.Join(selectFieldKeys(envelopeRaw), ", ")
					fmt.Fprintf(os.Stderr, "warning: --select %q matched no fields; valid fields: %s\n", requested[i], valid)
				}
			}
		}
		if appendNotes {
			copied := *flags
			copied.selectFields += ",notes"
			flags = &copied
		}
	}
	if flags != nil && cmd.Flags().Changed("compact") && flags.compact && flags.selectFields == "" &&
		(flags.asJSON || !isTerminal(cmd.OutOrStdout())) && cmd.Parent() != nil && cmd.Parent().Name() == "market" {
		if keys, ok := mktCompactKeys[cmd.Name()]; ok {
			raw, err := json.Marshal(m["results"])
			if err != nil {
				return err
			}
			var projected any
			if err := json.Unmarshal(raw, &projected); err != nil {
				return err
			}
			if cmd.Name() == "tags" {
				if obj, ok := projected.(map[string]any); ok {
					for _, key := range []string{"tags", "title_words"} {
						mktProjectRows(obj[key], keys)
					}
				}
			} else {
				mktProjectRows(projected, keys)
				if cmd.Name() == "demand" {
					if rows, ok := projected.([]any); ok {
						for _, row := range rows {
							if obj, ok := row.(map[string]any); ok {
								mktProjectRows(obj["listings"], mktCompactListingKeys)
							}
						}
					}
				}
			}
			m["results"] = projected
			// D-K replaces generic compaction; preserve every projected field.
			copied := *flags
			copied.compact = false
			flags = &copied
		}
	}
	return printJSONFilteredKeep(cmd.OutOrStdout(), m, flags, "notes")
}

// mktSelectIndeterminate limits uncertain selection to the results collection.
func mktSelectIndeterminate(raw json.RawMessage, path []string) bool {
	var envelope map[string]any
	if json.Unmarshal(raw, &envelope) != nil || len(path) == 0 {
		return false
	}
	results := envelope["results"]
	if strings.ToLower(path[0]) != "results" {
		var arrays func(any) (bool, bool)
		arrays = func(v any) (bool, bool) {
			switch x := v.(type) {
			case []any:
				return true, len(x) == 0
			case map[string]any:
				found, empty := false, true
				for _, child := range x {
					f, e := arrays(child)
					found = found || f
					empty = empty && e
				}
				return found, empty
			}
			return false, true
		}
		found, empty := arrays(results)
		return found && empty
	}
	var walk func(any, []string) bool
	walk = func(v any, remaining []string) bool {
		if v == nil {
			return true
		}
		if a, ok := v.([]any); ok {
			if len(a) == 0 {
				return true
			}
			for _, child := range a {
				if walk(child, remaining) {
					return true
				}
			}
			return false
		}
		if len(remaining) == 0 {
			return false
		}
		if obj, ok := v.(map[string]any); ok {
			for key, child := range obj {
				if strings.ToLower(key) == strings.ToLower(remaining[0]) {
					return walk(child, remaining[1:])
				}
			}
		}
		return false
	}
	return walk(results, path[1:])
}

// mktRowNotes explicitly collects row advisories, preserving first-seen order.
// Equal texts share a bounded list of attributed keys.
func mktRowNotes(keys, texts []string) []string {
	order := []string{}
	groups := map[string][]string{}
	for i, text := range texts {
		if text == "" {
			continue
		}
		if _, ok := groups[text]; !ok {
			order = append(order, text)
		}
		key := strconv.Quote(keys[i])
		duplicate := false
		for _, old := range groups[text] {
			if old == key {
				duplicate = true
			}
		}
		if !duplicate {
			groups[text] = append(groups[text], key)
		}
	}
	out := []string{}
	for _, text := range order {
		keys := groups[text]
		n := len(keys)
		if n > 5 {
			keys = keys[:5]
		}
		prefix := strings.Join(keys, ", ")
		if n > 5 {
			prefix += fmt.Sprintf(" (+%d more)", n-5)
		}
		out = append(out, prefix+": "+text)
	}
	return out
}

// mktEmitReport collects only explicitly declared report advisories. An explicit
// notes list supplies atomic entries; legacy notes may join those entries.
func mktEmitReport(cmd *cobra.Command, flags *rootFlags, report map[string]any) error {
	notes := []string{}
	seen := map[string]bool{}
	add := func(text string) {
		if text != "" && !seen[text] {
			notes = append(notes, text)
			seen[text] = true
		}
	}
	note, _ := report["note"].(string)
	ns, hasExplicitList := report["notes"].([]string)
	if hasExplicitList {
		for _, n := range ns {
			add(n)
		}
	} else {
		add(note)
	}
	explicit := append([]string(nil), notes...)
	composite := hasExplicitList && len(explicit) >= 2 && note == strings.Join(explicit, "; ")
	for _, key := range []string{"snapshot_note", "resume_hint", "error"} {
		if text, ok := report[key].(string); ok {
			add(text)
		}
	}
	if ns, ok := report["row_notes"].([]string); ok {
		for _, n := range ns {
			add(n)
		}
	}
	if composite && len(notes) > len(explicit) {
		// Extra advisories extend the atomic list and its legacy composite.
		note = strings.Join(notes, "; ")
	} else if !composite {
		add(note)
	}
	meta := make(map[string]any, len(report))
	for key, value := range report {
		if key != "row_notes" {
			meta[key] = value
		}
	}
	return mktEmit(cmd, flags, report["results"], meta, note, notes...)
}

// mktDryRun emits flat market previews without changing the generic helper.
func mktDryRun(cmd *cobra.Command, flags *rootFlags, action string) error {
	if flags != nil && flags.asJSON {
		return mktWriteDryRun(cmd, action, nil)
	}
	return writeDryRun(cmd.OutOrStdout(), flags, action)
}

// mktWriteDryRun deliberately bypasses selection and the agent wrapper.
func mktWriteDryRun(cmd *cobra.Command, action string, extra map[string]any) error {
	m := map[string]any{"dry_run": true, "success": true, "action": action, "would": "run " + action + "; no changes made", "results": []any{}, "notes": []string{}}
	for k, v := range extra {
		m[k] = v
	}
	if note, ok := extra["note"].(string); ok && note != "" {
		if _, exists := extra["notes"]; !exists {
			m["notes"] = []string{note}
		}
	}
	return writeEncodedJSON(cmd.OutOrStdout(), m, "", "")
}

// mktTermReviewAlternatives indexes review-bearing terms once, newest snapshot first.
// Call lazily when an absent term actually needs advice.
func mktTermReviewAlternatives(db *sql.DB, selected *mktSnapshot) (map[string]int64, error) {
	alternatives := make(map[string]int64)
	snapshots, err := mktCompleteSnapshots(db, 0)
	if err != nil {
		return nil, err
	}
	for _, snap := range snapshots {
		if snap.ID == selected.ID {
			continue
		}
		declared := make(map[string]bool)
		for _, term := range snap.Terms {
			declared[term] = true
		}
		// One query per snapshot includes persisted reviews and zero-review fetches.
		query := `SELECT DISTINCT r.term FROM market_results r
            WHERE r.snapshot_id=? AND (EXISTS(SELECT 1 FROM market_reviews v
            WHERE v.snapshot_id=r.snapshot_id AND v.listing_id=r.listing_id)`
		args := []any{snap.ID}
		if len(snap.Progress.ReviewsDone) > 0 {
			query += ` OR r.listing_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(snap.Progress.ReviewsDone)), ",") + `)`
			for _, id := range snap.Progress.ReviewsDone {
				args = append(args, id)
			}
		}
		query += `)`
		rows, err := db.Query(query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var term string
			if err := rows.Scan(&term); err != nil {
				rows.Close()
				return nil, err
			}
			if declared[term] && alternatives[term] == 0 {
				alternatives[term] = snap.ID
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return alternatives, nil
}

// Explicit market projections are separate from the generic compact heuristics.
var mktCompactKeys = map[string][]string{
	"demand": {"term", "state", "listings_total", "reviews_in_window", "note", "listings"},
	"shops":  {"shop_id", "shop_name", "top_n_appearances", "new_shop"},
	"price":  {"group", "currency", "n", "median", "spearman_rho", "note", "matches_all"},
	"gaps":   {"name", "matching_listings", "statement", "matches_all"},
	"tags":   {"term", "top_count", "rest_count"},
}
var mktCompactListingKeys = []string{"listing_id", "best_position", "reviews_in_window"}

func mktProjectRows(value any, keys []string) {
	rows, ok := value.([]any)
	if !ok {
		return
	}
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		for key := range obj {
			keep := false
			for _, allowed := range keys {
				if key == allowed {
					keep = true
					break
				}
			}
			if !keep {
				delete(obj, key)
			}
		}
	}
}

// mktReviewCoverage exposes the collection horizon without refusing wider reports.
func mktReviewCoverage(report map[string]any, snap *mktSnapshot, window time.Duration, hasReviews bool) map[string]any {
	if hasReviews && snap.ReviewWindowDays > 0 && window > time.Duration(snap.ReviewWindowDays)*24*time.Hour {
		report["review_coverage_days"] = snap.ReviewWindowDays
		note := fmt.Sprintf("reviews were collected for the last %d days; counts for the requested %d-day window cover only %d days", snap.ReviewWindowDays, int(window/(24*time.Hour)), snap.ReviewWindowDays)
		notes, _ := report["notes"].([]string)
		// etsy-ext: Preserve a pre-existing composite note when extending notes.
		composite := report["note"] == strings.Join(notes, "; ")
		notes = append(notes, note)
		report["notes"] = notes
		if composite {
			report["note"] = strings.Join(notes, "; ")
		}
	}
	return report
}
