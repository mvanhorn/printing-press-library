// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsfetch"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsparse"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/store"
)

// baseURL is the site origin. Release paths in the index are relative to it,
// and some are absolute already, so resolution happens per value in the parser.
const pbsBaseURL = "https://www.pbs.gov.pk"

// indexPath is the only enumerator. Release URLs cannot be constructed: the
// filenames use dozens of shapes including one with no date at all, and a
// guessed URL returns a clean 404.
const pbsIndexPath = "/price-statistics/"

// panelDBPath resolves the local store path.
func panelDBPath(override string) string {
	if override != "" {
		return override
	}
	return defaultDBPath("pbs-pp-cli")
}

// openPanel opens the store and applies the hand-authored panel schema.
func openPanel(ctx context.Context, dbPath string) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	if err := store.EnsurePBSSchema(ctx, db.DB()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// openPanelForRead opens an existing store, or reports the absence honestly.
//
// Returns ok=false when no store exists yet. Callers must then emit an empty
// machine result and a human hint naming the sync command, never a SQLite error.
func openPanelForRead(ctx context.Context, dbPath string) (db *store.Store, ok bool, err error) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, false, nil
	}
	db, err = openPanel(ctx, dbPath)
	if err != nil {
		return nil, false, err
	}
	return db, true, nil
}

// noMirror writes the standard missing-store hint to stderr.
//
// It goes to stderr, not stdout, so a machine caller reading stdout still sees
// a clean empty result rather than prose mixed into its JSON.
func noMirror(w io.Writer, dbPath string) {
	fmt.Fprintf(w, "no local panel at %s\nrun: pbs-pp-cli sync --full --timeout 6h --db %s\n", dbPath, dbPath)
}

// warnIfStale reports when the newest stored release is older than the caller's
// --max-age. It writes to stderr and never fails the command: the data is real,
// it is simply older than asked for, and silently serving it is the failure mode
// worth avoiding.
func warnIfStale(ctx context.Context, db *sql.DB, w io.Writer, flags *rootFlags, cmd *cobra.Command) {
	// Only honour an EXPLICIT --max-age. The generated root flag carries a
	// non-zero default, so keying on the value alone made this warning fire on
	// essentially every read — a warning that always fires is noise, and it
	// trains a caller to ignore the one time it matters.
	if flags == nil || flags.maxAge <= 0 || cmd == nil {
		return
	}
	explicit := cmd.Flags().Changed("max-age")
	if !explicit {
		if root := cmd.Root(); root != nil {
			explicit = root.PersistentFlags().Changed("max-age")
		}
	}
	if !explicit {
		return
	}
	var newest sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT MAX(as_of) FROM pbs_price`).Scan(&newest); err != nil {
		return
	}
	if !newest.Valid || newest.String == "" {
		return
	}
	t, err := time.Parse("2006-01-02", newest.String)
	if err != nil {
		return
	}
	age := time.Since(t)
	if age > flags.maxAge {
		fmt.Fprintf(w, "warning: newest stored release is %s, %d days old, older than the requested --max-age of %s\nrun: pbs-pp-cli sync --timeout 6h\n",
			newest.String, int(age.Hours()/24), flags.maxAge)
	}
}

// panelEmpty reports whether the store exists but holds no price rows yet.
//
// This is a DIFFERENT state from a missing store and from an unmatched item, and
// conflating them produced a genuinely misleading message: a first-time caller
// asking for a real item was told the item did not exist.
func panelEmpty(ctx context.Context, db *sql.DB) bool {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pbs_price`).Scan(&n); err != nil {
		return false
	}
	return n == 0
}

// emptyPanelHint writes the guidance for a store that exists but is unpopulated.
func emptyPanelHint(w io.Writer) {
	fmt.Fprintf(w, "the local panel is empty\nrun: pbs-pp-cli sync --max-releases 4\n")
}

// fetchIndex retrieves and parses the release index.
func fetchIndex(ctx context.Context, fc *pbsfetch.Client) (*pbsparse.Index, *pbsfetch.Result, error) {
	res, err := fc.Get(ctx, pbsBaseURL+pbsIndexPath)
	if err != nil {
		return nil, nil, err
	}
	idx, err := pbsparse.ParseIndex(string(res.Body), pbsBaseURL)
	if err != nil {
		return nil, res, err
	}
	return idx, res, nil
}

// coverageState enumerates the DISTINCT outcomes a release fetch can have.
//
// These are never collapsed. "not fetched" and "fetched and genuinely empty"
// and "blocked by an error" answer different questions, and a coverage map that
// merges them cannot tell a caller whether a gap is upstream or local.
const (
	covNotFetched = "not_fetched"
	covFetched    = "fetched"
	covEmpty      = "fetched_empty"
	covRot        = "upstream_404"
	covError      = "transport_error"
	covUnparsed   = "parse_failed"
)

// upsertReleaseIndex persists the scraped index. Index rows are replaced
// wholesale because the upstream array is hand-edited and rows do get
// re-pointed; the previous shape is preserved in pbs_coverage instead.
func upsertReleaseIndex(ctx context.Context, db *sql.DB, idx *pbsparse.Index) (int, int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	relStmt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_release
		(as_of, kind, as_of_raw, index_pos, filename_mismatch)
		VALUES (?,?,?,?,?)
		ON CONFLICT(as_of, kind) DO UPDATE SET
			as_of_raw=excluded.as_of_raw,
			index_pos=excluded.index_pos,
			filename_mismatch=excluded.filename_mismatch`)
	if err != nil {
		return 0, 0, err
	}
	defer relStmt.Close()
	fileStmt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_release_file
		(as_of, kind, role, url, filename, ext, index_key)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(as_of, kind, url) DO UPDATE SET
			role=excluded.role, filename=excluded.filename,
			ext=excluded.ext, index_key=excluded.index_key`)
	if err != nil {
		return 0, 0, err
	}
	defer fileStmt.Close()

	var releases, files int
	for _, r := range append(append([]pbsparse.Release{}, idx.Weekly...), idx.Monthly...) {
		mismatch := 0
		if r.FilenameDateMismatch {
			mismatch = 1
		}
		if _, err := relStmt.ExecContext(ctx, r.AsOfKey(), string(r.Kind), r.AsOfRaw, r.IndexPos, mismatch); err != nil {
			return releases, files, err
		}
		releases++
		for _, f := range r.Files {
			if _, err := fileStmt.ExecContext(ctx, r.AsOfKey(), string(r.Kind), string(f.Role), f.URL, f.Filename, f.Ext, f.IndexKey); err != nil {
				return releases, files, err
			}
			files++
		}
	}
	if err := tx.Commit(); err != nil {
		return releases, files, err
	}
	return releases, files, nil
}

// nullable converts a parsed cell into arguments safe for a NULLABLE column.
//
// A value that is not present is stored as SQL NULL with its state recorded
// separately. It is never stored as 0: PBS writes numeric zero for an
// uncollected price, and treating that as a real observation moves a measured
// national average by -16.6%.
func nullable(v pbsparse.Value) (any, string) {
	if v.Present() {
		return v.Num, string(v.State)
	}
	return nil, string(v.State)
}

func nullableNum(v pbsparse.Value) any {
	if v.Present() {
		return v.Num
	}
	return nil
}

// deleteReleaseRows clears one release's rows from the named tables.
//
// Table names are package-local literals from the callers below, never user
// input, so they are safe to interpolate — SQLite does not accept a bound
// parameter in place of a table name. The as_of value is still bound.
func deleteReleaseRows(ctx context.Context, tx *sql.Tx, asOf string, tables ...string) error {
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE as_of = ?`, asOf); err != nil {
			return fmt.Errorf("clear %s for %s: %w", t, asOf, err)
		}
	}
	return nil
}

// persistAnnexure writes one parsed annexure.
func persistAnnexure(ctx context.Context, db *sql.DB, a *pbsparse.Annexure, source string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Replace the release rather than merely upserting over it.
	//
	// The item basket is NOT constant upstream and PBS does rewrite releases
	// (which is what `revisions` exists to detect). Upserting alone left rows
	// from the previous vintage sitting at the same as_of, so a rewrite that
	// DROPPED an item kept that item alive and a later basket or national
	// average silently summed a mixture of two vintages. The delete is inside
	// this transaction so a crash cannot leave the release empty.
	if err := deleteReleaseRows(ctx, tx, a.AsOf, "pbs_price", "pbs_national"); err != nil {
		return 0, err
	}
	st, err := tx.PrepareContext(ctx, `INSERT INTO pbs_price
		(as_of, surface, city, city_code, item_desc, item_no, unit, stat, value, value_state, block, desc_suspect, source)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(as_of, surface, city, item_desc, stat) DO UPDATE SET
			value=excluded.value, value_state=excluded.value_state,
			city_code=excluded.city_code, item_no=excluded.item_no,
			unit=excluded.unit, block=excluded.block,
			desc_suspect=excluded.desc_suspect, source=excluded.source`)
	if err != nil {
		return 0, err
	}
	defer st.Close()
	var n int
	for _, r := range a.Rows {
		val, state := nullable(r.Value)
		suspect := 0
		if r.DescSuspect {
			suspect = 1
		}
		if _, err := st.ExecContext(ctx, a.AsOf, string(r.Surface), r.City, r.CityCode,
			r.ItemDesc, r.ItemNo, r.Unit, string(r.Stat), val, state, r.Block, suspect, source); err != nil {
			return n, err
		}
		n++
	}
	natSt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_national
		(as_of, item_desc, series, value, value_state) VALUES (?,?,?,?,?)
		ON CONFLICT(as_of, item_desc, series) DO UPDATE SET
			value=excluded.value, value_state=excluded.value_state`)
	if err != nil {
		return n, err
	}
	defer natSt.Close()
	for item, v := range a.NationalAvg {
		val, state := nullable(v)
		if _, err := natSt.ExecContext(ctx, a.AsOf, item, "national_avg", val, state); err != nil {
			return n, err
		}
	}
	for item, series := range a.Derived {
		for name, v := range series {
			val, state := nullable(v)
			if _, err := natSt.ExecContext(ctx, a.AsOf, item, name, val, state); err != nil {
				return n, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return n, err
	}
	return n, nil
}

// persistReport writes one parsed executive summary.
func persistReport(ctx context.Context, db *sql.DB, rep *pbsparse.Report) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Same replace-semantics as persistAnnexure: a rewritten report with
	// fewer items or fewer quintile series must not leave the dropped ones
	// behind, or `weights --check-total` would be asserting 100.0000 over a
	// mixture of two vintages.
	if err := deleteReleaseRows(ctx, tx, rep.AsOf, "pbs_weight", "pbs_weight_total", "pbs_index"); err != nil {
		return 0, err
	}

	wSt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_weight
		(as_of, item_desc, section, sr, unit, national_price, price_prev_week, price_cor_week,
		 pct_prev_week, pct_cor_week, weight_lowest, weight_combined, impact_lowest, impact_combined)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(as_of, item_desc) DO UPDATE SET
			section=excluded.section, sr=excluded.sr, unit=excluded.unit,
			national_price=excluded.national_price, price_prev_week=excluded.price_prev_week,
			price_cor_week=excluded.price_cor_week, pct_prev_week=excluded.pct_prev_week,
			pct_cor_week=excluded.pct_cor_week, weight_lowest=excluded.weight_lowest,
			weight_combined=excluded.weight_combined, impact_lowest=excluded.impact_lowest,
			impact_combined=excluded.impact_combined`)
	if err != nil {
		return 0, err
	}
	defer wSt.Close()
	var n int
	for _, it := range rep.Items {
		if _, err := wSt.ExecContext(ctx, rep.AsOf, it.ItemDesc, string(it.Section), it.Sr, it.Unit,
			nullableNum(it.NationalPrice), nullableNum(it.PricePrevWeek), nullableNum(it.PriceCorWeek),
			nullableNum(it.PctPrevWeek), nullableNum(it.PctCorWeek),
			nullableNum(it.WeightLowest), nullableNum(it.WeightCombined),
			nullableNum(it.ImpactLowest), nullableNum(it.ImpactCombined)); err != nil {
			return n, err
		}
		n++
	}

	tSt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_weight_total
		(as_of, section, declared_count, observed_count, weight_lowest, weight_combined, impact_lowest, impact_combined)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(as_of, section) DO UPDATE SET
			declared_count=excluded.declared_count, observed_count=excluded.observed_count,
			weight_lowest=excluded.weight_lowest, weight_combined=excluded.weight_combined,
			impact_lowest=excluded.impact_lowest, impact_combined=excluded.impact_combined`)
	if err != nil {
		return n, err
	}
	defer tSt.Close()
	for _, t := range rep.Totals {
		if _, err := tSt.ExecContext(ctx, rep.AsOf, string(t.Section), t.DeclaredCount, t.ObservedCount,
			nullableNum(t.WeightLowest), nullableNum(t.WeightCombined),
			nullableNum(t.ImpactLowest), nullableNum(t.ImpactCombined)); err != nil {
			return n, err
		}
	}

	iSt, err := tx.PrepareContext(ctx, `INSERT INTO pbs_index
		(as_of, quintile, band_raw, band_low, band_high, idx_value, prev_week, cor_week, pct_prev, pct_cor)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(as_of, quintile) DO UPDATE SET
			band_raw=excluded.band_raw, band_low=excluded.band_low, band_high=excluded.band_high,
			idx_value=excluded.idx_value, prev_week=excluded.prev_week, cor_week=excluded.cor_week,
			pct_prev=excluded.pct_prev, pct_cor=excluded.pct_cor`)
	if err != nil {
		return n, err
	}
	defer iSt.Close()
	for _, q := range rep.Quintiles {
		var low, high any
		if q.BandLow > 0 {
			low = q.BandLow
		}
		if q.BandHigh > 0 {
			high = q.BandHigh
		}
		if _, err := iSt.ExecContext(ctx, rep.AsOf, q.Quintile, q.BandRaw, low, high,
			nullableNum(q.Index), nullableNum(q.PrevWeek), nullableNum(q.CorWeek),
			nullableNum(q.PctPrev), nullableNum(q.PctCorWk)); err != nil {
			return n, err
		}
	}
	if err := tx.Commit(); err != nil {
		return n, err
	}
	return n, nil
}

// recordCoverage writes one release-file outcome.
func recordCoverage(ctx context.Context, db *sql.DB, asOf, kind, role, url, state string,
	status int, sha string, bytes, rows int, cen pbsparse.StateCensus, parser, note string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO pbs_coverage
		(as_of, kind, role, url, state, http_status, sha256, bytes, rows_parsed,
		 present_cells, zero_cells, blank_cells, na_cells, unparseable_cells, parser, note, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(as_of, kind, role) DO UPDATE SET
			url=excluded.url,
			state=excluded.state, http_status=excluded.http_status, sha256=excluded.sha256,
			bytes=excluded.bytes, rows_parsed=excluded.rows_parsed,
			present_cells=excluded.present_cells, zero_cells=excluded.zero_cells,
			blank_cells=excluded.blank_cells, na_cells=excluded.na_cells,
			unparseable_cells=excluded.unparseable_cells, parser=excluded.parser,
			note=excluded.note, fetched_at=excluded.fetched_at`,
		asOf, kind, role, url, state, status, sha, bytes, rows,
		cen.Present, cen.Zero, cen.Blank, cen.NA, cen.Unparseable,
		parser, note, time.Now().UTC().Format(time.RFC3339))
	return err
}
