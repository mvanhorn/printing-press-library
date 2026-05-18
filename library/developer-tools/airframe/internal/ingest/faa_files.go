// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"
)

// ingestMASTER reads MASTER.txt and inserts into the aircraft table.
// includeAddresses populates the owner_* address columns; otherwise only
// owner_name is stored.
func ingestMASTER(ctx context.Context, tx *sql.Tx, zf *zip.File, includeAddresses bool, progress ProgressFunc) (int, error) {
	rc, err := openZipCSV(zf)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	cr := newCSVReader(rc)

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("MASTER header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO aircraft (
		registration, serial_number, make_model_code, engine_code, year_mfr,
		type_registrant, type_aircraft, type_engine, status_code,
		cert_issue_date, last_action_date, airworthiness_date, expiration_date,
		mode_s_code_hex, owner_name, owner_street, owner_city, owner_state,
		owner_zip, owner_country
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing aircraft insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			progress("ingest_parse_error", map[string]any{"table": "aircraft", "row": count, "err": err.Error()})
			continue
		}

		registration := normalizeNNumber(col(row, idx, "N-NUMBER"))
		if registration == "" {
			continue
		}

		var ownerStreet, ownerCity, ownerState, ownerZip, ownerCountry any
		if includeAddresses {
			ownerStreet = nullableStr(col(row, idx, "STREET"))
			ownerCity = nullableStr(col(row, idx, "CITY"))
			ownerState = nullableStr(col(row, idx, "STATE"))
			ownerZip = nullableStr(col(row, idx, "ZIP CODE"))
			ownerCountry = nullableStr(col(row, idx, "COUNTRY"))
		}

		_, err = stmt.ExecContext(ctx,
			registration,
			nullableStr(col(row, idx, "SERIAL NUMBER")),
			nullableStr(col(row, idx, "MFR MDL CODE")),
			nullableStr(col(row, idx, "ENG MFR MDL")),
			nullableInt(row, idx, "YEAR MFR"),
			nullableStr(col(row, idx, "TYPE REGISTRANT")),
			nullableStr(col(row, idx, "TYPE AIRCRAFT")),
			nullableStr(col(row, idx, "TYPE ENGINE")),
			nullableStr(col(row, idx, "STATUS CODE")),
			nullableStr(faaDate(col(row, idx, "CERT ISSUE DATE"))),
			nullableStr(faaDate(col(row, idx, "LAST ACTION DATE"))),
			nullableStr(faaDate(col(row, idx, "AIR WORTH DATE"))),
			nullableStr(faaDate(col(row, idx, "EXPIRATION DATE"))),
			nullableStr(strings.ToUpper(col(row, idx, "MODE S CODE HEX"))),
			nullableStr(col(row, idx, "NAME")),
			ownerStreet, ownerCity, ownerState, ownerZip, ownerCountry,
		)
		if err != nil {
			return count, fmt.Errorf("insert aircraft (%s): %w", registration, err)
		}
		count++
		if count%50000 == 0 {
			progress("ingest_progress", map[string]any{"table": "aircraft", "rows": count})
		}
	}
	return count, nil
}

func ingestACFTREF(ctx context.Context, tx *sql.Tx, zf *zip.File, progress ProgressFunc) (int, error) {
	rc, err := openZipCSV(zf)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	cr := newCSVReader(rc)

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("ACFTREF header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO make_model (
		code, manufacturer, model, aircraft_type, engine_type, category,
		builder_certification, number_engines, number_seats, weight_class, cruising_speed
	) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing make_model insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			progress("ingest_parse_error", map[string]any{"table": "make_model", "row": count, "err": err.Error()})
			continue
		}
		code := col(row, idx, "CODE")
		mfr := col(row, idx, "MFR")
		model := col(row, idx, "MODEL")
		if code == "" || mfr == "" || model == "" {
			continue
		}
		_, err = stmt.ExecContext(ctx,
			code, mfr, model,
			nullableStr(col(row, idx, "TYPE-ACFT")),
			nullableStr(col(row, idx, "TYPE-ENG")),
			nullableStr(col(row, idx, "AC-CAT")),
			nullableStr(col(row, idx, "BUILD-CERT-IND")),
			nullableInt(row, idx, "NO-ENG"),
			nullableInt(row, idx, "NO-SEATS"),
			nullableStr(col(row, idx, "AC-WEIGHT")),
			nullableInt(row, idx, "SPEED"),
		)
		if err != nil {
			return count, fmt.Errorf("insert make_model (%s): %w", code, err)
		}
		count++
	}
	progress("ingest_progress", map[string]any{"table": "make_model", "rows": count})
	return count, nil
}

func ingestENGINE(ctx context.Context, tx *sql.Tx, zf *zip.File, progress ProgressFunc) (int, error) {
	rc, err := openZipCSV(zf)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	cr := newCSVReader(rc)

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("ENGINE header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO engine (
		code, manufacturer, model, engine_type, horsepower, thrust
	) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing engine insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			progress("ingest_parse_error", map[string]any{"table": "engine", "row": count, "err": err.Error()})
			continue
		}
		code := col(row, idx, "CODE")
		if code == "" {
			continue
		}
		_, err = stmt.ExecContext(ctx,
			code,
			nullableStr(col(row, idx, "MFR")),
			nullableStr(col(row, idx, "MODEL")),
			nullableStr(col(row, idx, "TYPE")),
			nullableInt(row, idx, "HORSEPOWER"),
			nullableInt(row, idx, "THRUST"),
		)
		if err != nil {
			return count, fmt.Errorf("insert engine (%s): %w", code, err)
		}
		count++
	}
	progress("ingest_progress", map[string]any{"table": "engine", "rows": count})
	return count, nil
}

func ingestDEREG(ctx context.Context, tx *sql.Tx, zf *zip.File, progress ProgressFunc) (int, error) {
	rc, err := openZipCSV(zf)
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	cr := newCSVReader(rc)

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("DEREG header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO dereg (
		registration, cancel_date, cancel_status_code, make_model_code,
		prev_owner, cert_issue_date, last_action_date
	) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing dereg insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			progress("ingest_parse_error", map[string]any{"table": "dereg", "row": count, "err": err.Error()})
			continue
		}
		registration := normalizeNNumber(col(row, idx, "N-NUMBER"))
		cancelDate := faaDate(col(row, idx, "CANCEL-DATE"))
		if registration == "" || cancelDate == "" {
			continue
		}
		_, err = stmt.ExecContext(ctx,
			registration, cancelDate,
			nullableStr(col(row, idx, "STATUS-CODE")),
			nullableStr(col(row, idx, "MFR-MDL-CODE")),
			nullableStr(col(row, idx, "NAME")),
			nullableStr(faaDate(col(row, idx, "CERT-ISSUE-DATE"))),
			nullableStr(faaDate(col(row, idx, "LAST-ACT-DATE"))),
		)
		if err != nil {
			return count, fmt.Errorf("insert dereg (%s/%s): %w", registration, cancelDate, err)
		}
		count++
		if count%50000 == 0 {
			progress("ingest_progress", map[string]any{"table": "dereg", "rows": count})
		}
	}
	return count, nil
}

// faaDate normalizes the FAA date format YYYYMMDD into ISO 8601 (YYYY-MM-DD).
// Empty input returns empty; malformed (non-8-digit) input returns the raw
// string so it surfaces for debugging rather than silently dropping.
func faaDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) != 8 {
		return s
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return s
		}
	}
	return s[:4] + "-" + s[4:6] + "-" + s[6:8]
}

// lookupSyncMeta returns the last_modified header and row count last seen
// for the given source, or empty values if no prior sync exists.
func lookupSyncMeta(ctx context.Context, st *store.Store, source string) (lastModified string, rowCount int64, err error) {
	row := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(last_modified, ''), COALESCE(row_count, 0) FROM sync_meta WHERE source = ?`, source)
	err = row.Scan(&lastModified, &rowCount)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	return lastModified, rowCount, err
}

// upsertSyncMetaTx writes a sync_meta row inside the caller's transaction.
func upsertSyncMetaTx(ctx context.Context, tx *sql.Tx, r store.SyncMetaRow) error {
	_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO sync_meta
		(source, source_url, last_modified, source_etag, last_synced_at, row_count, bytes_downloaded, schema_profile)
		VALUES (?,?,?,?,?,?,?,?)`,
		r.Source, r.SourceURL, r.LastModified, r.SourceETag,
		r.LastSyncedAt, r.RowCount, r.BytesDownloaded, r.SchemaProfile)
	if err != nil {
		return fmt.Errorf("upsert sync_meta(%s): %w", r.Source, err)
	}
	return nil
}
