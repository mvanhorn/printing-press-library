// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import (
	"archive/zip"
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DatabaseURL is the FAA's daily Releasable Aircraft Database archive.
const DatabaseURL = "https://registry.faa.gov/database/ReleasableAircraft.zip"

// browserUA satisfies the Akamai front door, which rejects non-browser
// User-Agents with 403.
const browserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

// SyncResult reports what a sync run did.
type SyncResult struct {
	Downloaded   bool           `json:"downloaded"`
	NotModified  bool           `json:"not_modified"`
	LastModified string         `json:"last_modified,omitempty"`
	Counts       map[string]int `json:"counts,omitempty"`
	SyncedAt     string         `json:"synced_at,omitempty"`
}

// Sync downloads the daily zip (honoring Last-Modified) and imports every
// table. force re-imports even when the upstream file hasn't changed.
// progress, when non-nil, receives one-line status updates.
func (d *DB) Sync(ctx context.Context, zipPath string, force bool, progress func(string)) (*SyncResult, error) {
	say := func(format string, args ...any) {
		if progress != nil {
			progress(fmt.Sprintf(format, args...))
		}
	}
	res := &SyncResult{}

	prevLM, _ := d.Meta(ctx, "last_modified")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DatabaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	if prevLM != "" && !force {
		req.Header.Set("If-Modified-Since", prevLM)
	}
	say("downloading %s ...", DatabaseURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading registry database: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		res.NotModified = true
		res.LastModified = prevLM
		say("registry database unchanged upstream (Last-Modified: %s)", prevLM)
		if synced, _ := d.Synced(ctx); synced && !force {
			return res, nil
		}
	case http.StatusOK:
		// fall through to download
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("downloading registry database: HTTP 429 (rate limited by the FAA front door) — wait a minute and re-run sync; the download is a single daily request, not a burst")
	default:
		return nil, fmt.Errorf("downloading registry database: HTTP %d", resp.StatusCode)
	}

	if !res.NotModified {
		out, err := os.Create(zipPath)
		if err != nil {
			return nil, err
		}
		n, err := io.Copy(out, resp.Body)
		cerr := out.Close()
		if err != nil {
			return nil, fmt.Errorf("saving registry database: %w", err)
		}
		if cerr != nil {
			return nil, cerr
		}
		res.Downloaded = true
		res.LastModified = resp.Header.Get("Last-Modified")
		say("downloaded %.1f MB", float64(n)/1e6)
	}

	counts, err := d.importZip(ctx, zipPath, say)
	if err != nil {
		return nil, err
	}
	res.Counts = counts
	res.SyncedAt = time.Now().UTC().Format(time.RFC3339)

	if res.LastModified != "" {
		if err := d.SetMeta(ctx, "last_modified", res.LastModified); err != nil {
			return nil, err
		}
	}
	if err := d.SetMeta(ctx, "synced_at", res.SyncedAt); err != nil {
		return nil, err
	}
	return res, nil
}

// ImportZip imports all registry files from a local copy of
// ReleasableAircraft.zip (exported for tests and offline import).
func (d *DB) ImportZip(ctx context.Context, zipPath string, progress func(string)) (map[string]int, error) {
	say := func(format string, args ...any) {
		if progress != nil {
			progress(fmt.Sprintf(format, args...))
		}
	}
	return d.importZip(ctx, zipPath, say)
}

type tableSpec struct {
	file  string
	table string
	// columns maps normalized CSV header name -> destination column.
	columns map[string]string
	key     string // primary-key column ("" = plain insert)
}

// Column maps are keyed by normalized header names (non-alphanumerics
// collapsed to underscores) so the importer survives the FAA's occasional
// column reorderings and additions; unknown columns are ignored, missing
// columns import as empty strings.
var tableSpecs = []tableSpec{
	{
		file:  "MASTER.txt",
		table: "faa_master",
		key:   "n_number",
		columns: map[string]string{
			"N_NUMBER": "n_number", "SERIAL_NUMBER": "serial_number",
			"MFR_MDL_CODE": "mfr_mdl_code", "ENG_MFR_MDL": "eng_mfr_mdl",
			"YEAR_MFR": "year_mfr", "TYPE_REGISTRANT": "type_registrant",
			"NAME": "name", "STREET": "street", "STREET2": "street2",
			"CITY": "city", "STATE": "state", "ZIP_CODE": "zip_code",
			"REGION": "region", "COUNTY": "county", "COUNTRY": "country",
			"LAST_ACTION_DATE": "last_action_date", "CERT_ISSUE_DATE": "cert_issue_date",
			"CERTIFICATION": "certification", "TYPE_AIRCRAFT": "type_aircraft",
			"TYPE_ENGINE": "type_engine", "STATUS_CODE": "status_code",
			"MODE_S_CODE": "mode_s_code", "FRACT_OWNER": "fract_owner",
			"AIR_WORTH_DATE": "air_worth_date",
			"OTHER_NAMES_1":  "other_name_1", "OTHER_NAMES_2": "other_name_2",
			"OTHER_NAMES_3": "other_name_3", "OTHER_NAMES_4": "other_name_4",
			"OTHER_NAMES_5":   "other_name_5",
			"EXPIRATION_DATE": "expiration_date", "UNIQUE_ID": "unique_id",
			"KIT_MFR": "kit_mfr", "KIT_MODEL": "kit_model",
			"MODE_S_CODE_HEX": "mode_s_code_hex",
		},
	},
	{
		file:  "DEREG.txt",
		table: "faa_dereg",
		columns: map[string]string{
			"N_NUMBER": "n_number", "SERIAL_NUMBER": "serial_number",
			"MFR_MDL_CODE": "mfr_mdl_code", "STATUS_CODE": "status_code",
			"NAME": "name", "STREET_MAIL": "street_mail", "STREET2_MAIL": "street2_mail",
			"CITY_MAIL": "city_mail", "STATE_ABBREV_MAIL": "state_abbrev_mail",
			"ZIP_CODE_MAIL": "zip_code_mail", "ENG_MFR_MDL": "eng_mfr_mdl",
			"YEAR_MFR": "year_mfr", "CERTIFICATION": "certification",
			"REGION": "region", "COUNTY_MAIL": "county_mail", "COUNTRY_MAIL": "country_mail",
			"AIR_WORTH_DATE": "air_worth_date", "CANCEL_DATE": "cancel_date",
			"MODE_S_CODE": "mode_s_code", "INDICATOR_GROUP": "indicator_group",
			"EXP_COUNTRY": "exp_country", "LAST_ACT_DATE": "last_act_date",
			"CERT_ISSUE_DATE": "cert_issue_date",
			"OTHER_NAMES_1":   "other_name_1", "OTHER_NAMES_2": "other_name_2",
			"OTHER_NAMES_3": "other_name_3", "OTHER_NAMES_4": "other_name_4",
			"OTHER_NAMES_5": "other_name_5",
			"KIT_MFR":       "kit_mfr", "KIT_MODEL": "kit_model",
			"MODE_S_CODE_HEX": "mode_s_code_hex",
		},
	},
	{
		file:  "RESERVED.txt",
		table: "faa_reserved",
		key:   "n_number",
		columns: map[string]string{
			"N_NUMBER": "n_number", "REGISTRANT": "registrant",
			"STREET": "street", "STREET2": "street2", "CITY": "city",
			"STATE": "state", "ZIP_CODE": "zip_code", "RSV_DATE": "rsv_date",
			"TR": "tr", "EXP_DATE": "exp_date", "N_NUM_CHG": "n_num_chg",
			"PURGE_DATE": "purge_date",
		},
	},
	{
		file:  "ACFTREF.txt",
		table: "faa_acftref",
		key:   "code",
		columns: map[string]string{
			"CODE": "code", "MFR": "mfr", "MODEL": "model",
			"TYPE_ACFT": "type_acft", "TYPE_ENG": "type_eng", "AC_CAT": "ac_cat",
			"BUILD_CERT_IND": "build_cert_ind", "NO_ENG": "no_eng",
			"NO_SEATS": "no_seats", "AC_WEIGHT": "ac_weight", "SPEED": "speed",
			"TC_DATA_SHEET": "tc_data_sheet", "TC_DATA_HOLDER": "tc_data_holder",
		},
	},
	{
		file:  "ENGINE.txt",
		table: "faa_engine",
		key:   "code",
		columns: map[string]string{
			"CODE": "code", "MFR": "mfr", "MODEL": "model",
			"TYPE": "type", "HORSEPOWER": "horsepower", "THRUST": "thrust",
		},
	},
}

func (d *DB) importZip(ctx context.Context, zipPath string, say func(string, ...any)) (map[string]int, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("opening registry archive: %w", err)
	}
	defer zr.Close()

	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		byName[strings.ToUpper(f.Name)] = f
	}

	// Validate every required file is present BEFORE mutating any table. A
	// partial archive (missing MASTER, ACFTREF, ENGINE, ...) would otherwise
	// import some tables and skip others, leaving lookups and fleet reports
	// with empty manufacturer/model/engine joins while still looking synced.
	// Failing here leaves the previous good import (and its sync_complete
	// marker) untouched.
	var missing []string
	for _, spec := range tableSpecs {
		if _, ok := byName[strings.ToUpper(spec.file)]; !ok {
			missing = append(missing, spec.file)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("registry archive is incomplete — missing required file(s): %s; not importing (previous data kept)", strings.Join(missing, ", "))
	}

	// All table replacements, the FTS rebuild, and the completeness marker run
	// in ONE transaction so they commit or roll back together. Any failure
	// mid-import (a bad row, a failed FTS rebuild, a killed process) rolls the
	// whole thing back, leaving the previous good sync fully intact and
	// consistent — the base tables and the faa_master_fts index never diverge.
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Invalidate the marker inside the transaction. If we commit, it's set back
	// to "1" below; if we roll back, the previous "1" is restored intact.
	if _, err := tx.ExecContext(ctx, `INSERT INTO faa_meta (key, value) VALUES ('sync_complete','')
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, spec := range tableSpecs {
		zf := byName[strings.ToUpper(spec.file)]
		rc, err := zf.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", spec.file, err)
		}
		n, err := d.importCSV(ctx, tx, rc, spec)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("importing %s: %w", spec.file, err)
		}
		counts[spec.table] = n
		say("imported %s: %d rows", spec.file, n)
	}

	if err := d.rebuildFTS(ctx, tx); err != nil {
		return nil, fmt.Errorf("rebuilding search index: %w", err)
	}
	say("search index rebuilt")

	// Mark the import complete only after every table AND the search index
	// landed. Synced() gates on this marker.
	if _, err := tx.ExecContext(ctx, `INSERT INTO faa_meta (key, value) VALUES ('sync_complete','1')
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing registry import: %w", err)
	}
	return counts, nil
}

// normalizeHeader collapses a CSV header name to the key format used in
// tableSpecs: uppercase with every non-alphanumeric run as one underscore.
func normalizeHeader(h string) string {
	h = strings.TrimPrefix(h, "\ufeff") // UTF-8 BOM on the first header
	h = strings.ToUpper(strings.TrimSpace(h))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range h {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.TrimSuffix(b.String(), "_")
}

// The FAA files are plain comma-delimited with NO quoting or escaping (values
// are space-padded, and some contain bare double quotes like `"DR 107"`), so
// rows are split manually. encoding/csv would treat a leading quote as a
// quoted field and silently swallow rows across line boundaries.
// importCSV imports one table within the caller's transaction. It never
// commits or rolls back — importZip owns the single transaction spanning every
// table and the FTS rebuild.
func (d *DB) importCSV(ctx context.Context, tx *sql.Tx, r io.Reader, spec tableSpec) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return 0, fmt.Errorf("reading header: %w", err)
		}
		return 0, fmt.Errorf("reading header: empty file")
	}
	header := strings.Split(sc.Text(), ",")

	// Destination column for each CSV index ("" = ignore).
	dest := make([]string, len(header))
	var cols []string
	idxFor := map[string]int{}
	for i, h := range header {
		norm := normalizeHeader(h)
		if col, ok := spec.columns[norm]; ok {
			dest[i] = col
			idxFor[col] = i
			cols = append(cols, col)
		}
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("no known columns in header %v", header)
	}
	if spec.key != "" {
		if _, ok := idxFor[spec.key]; !ok {
			return 0, fmt.Errorf("key column %s missing from header", spec.key)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM "+spec.table); err != nil {
		return 0, err
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	insertSQL := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		spec.table, strings.Join(cols, ","), placeholders)
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	args := make([]any, len(cols))
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		rec := strings.Split(line, ",")
		for j, col := range cols {
			i := idxFor[col]
			v := ""
			if i < len(rec) {
				v = strings.TrimSpace(rec[i])
			}
			args[j] = v
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return 0, fmt.Errorf("row %d: %w", count+2, err)
		}
		count++
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("row %d: %w", count+2, err)
	}
	return count, nil
}

// rebuildFTS rebuilds the search index within the caller's transaction.
func (d *DB) rebuildFTS(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM faa_master_fts`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO faa_master_fts (n_number, name, other_names, model, mfr)
		SELECT m.n_number, m.name,
			TRIM(m.other_name_1 || ' ' || m.other_name_2 || ' ' || m.other_name_3 || ' ' || m.other_name_4 || ' ' || m.other_name_5),
			COALESCE(a.model, ''), COALESCE(a.mfr, '')
		FROM faa_master m
		LEFT JOIN faa_acftref a ON a.code = m.mfr_mdl_code`); err != nil {
		return err
	}
	return nil
}
