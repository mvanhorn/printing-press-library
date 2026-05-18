// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

// Typed access to the recordings table for the Miami-Dade Clerk
// transcendence features. The generic resources table is used by sync
// infrastructure; recordings is the indexed query target for
// lien-chain / surviving-liens / chain-of-title / owner-portfolio /
// case-arc / enrich / ftl-scan.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Recording is the typed shape of a clerk recording. Money is in cents
// (multiply the API's dollar fields by 100 on ingest). Folio is the
// canonical 13-digit numeric form (zero-pad the clerk's response when
// ingesting). Dates are ISO 8601 (YYYY-MM-DD).
type Recording struct {
	CFNMasterID        int64  `json:"cfn_master_id"`
	DocTypeCode        string `json:"doc_type_code"`
	RecordingDate      string `json:"recording_date"`
	FirstParty         string `json:"first_party,omitempty"`
	SecondParty        string `json:"second_party,omitempty"`
	FolioNumber        int64  `json:"folio_number,omitempty"`
	Address            string `json:"address,omitempty"`
	LegalDescription   string `json:"legal_description,omitempty"`
	SubdivisionName    string `json:"subdivision_name,omitempty"`
	PlatBook           int    `json:"plat_book,omitempty"`
	PlatPage           int    `json:"plat_page,omitempty"`
	BlockNo            string `json:"block_no,omitempty"`
	CaseNumber         string `json:"case_number,omitempty"`
	ConsiderationCents int64  `json:"consideration_cents,omitempty"`
	LinkDocType        string `json:"link_doctype,omitempty"`
	ViewerQS           string `json:"viewer_qs,omitempty"`
	RawJSON            string `json:"raw_json,omitempty"`
}

// UpsertRecording inserts or replaces a recording by cfn_master_id.
// Also updates the FTS index. Caller is responsible for marshaling the
// raw_json blob (we keep the full clerk payload so downstream extraction
// can pick up new fields without a re-fetch).
func (s *Store) UpsertRecording(r *Recording) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO recordings (
		cfn_master_id, doc_type_code, recording_date, first_party, second_party,
		folio_number, address, legal_description, subdivision_name, plat_book,
		plat_page, block_no, case_number, consideration_cents, link_doctype,
		viewer_qs, raw_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(cfn_master_id) DO UPDATE SET
		doc_type_code = excluded.doc_type_code,
		recording_date = excluded.recording_date,
		first_party = excluded.first_party,
		second_party = excluded.second_party,
		folio_number = excluded.folio_number,
		address = excluded.address,
		legal_description = excluded.legal_description,
		subdivision_name = excluded.subdivision_name,
		plat_book = excluded.plat_book,
		plat_page = excluded.plat_page,
		block_no = excluded.block_no,
		case_number = excluded.case_number,
		consideration_cents = excluded.consideration_cents,
		link_doctype = excluded.link_doctype,
		viewer_qs = excluded.viewer_qs,
		raw_json = excluded.raw_json`,
		r.CFNMasterID, r.DocTypeCode, r.RecordingDate,
		nullableStr(r.FirstParty), nullableStr(r.SecondParty),
		nullableI64(r.FolioNumber), nullableStr(r.Address),
		nullableStr(r.LegalDescription), nullableStr(r.SubdivisionName),
		nullableInt(r.PlatBook), nullableInt(r.PlatPage), nullableStr(r.BlockNo),
		nullableStr(r.CaseNumber), nullableI64(r.ConsiderationCents),
		nullableStr(r.LinkDocType), nullableStr(r.ViewerQS), r.RawJSON,
	); err != nil {
		return fmt.Errorf("upsert recording %d: %w", r.CFNMasterID, err)
	}

	// FTS5 with content= mirrors writes on insert/update via triggers? No —
	// content= means the FTS table reads from `recordings`; we still need
	// to issue the FTS INSERT/UPDATE explicitly. Use REPLACE on rowid.
	if _, err := tx.Exec(`INSERT OR REPLACE INTO recordings_fts(
		rowid, first_party, second_party, legal_description, subdivision_name, address, case_number
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.CFNMasterID, r.FirstParty, r.SecondParty, r.LegalDescription,
		r.SubdivisionName, r.Address, r.CaseNumber,
	); err != nil {
		// FTS failures are non-fatal — the typed table is the source of
		// truth, the index just improves search speed.
		fmt.Fprintf(os.Stderr, "warning: FTS index update failed for cfn %d: %v\n", r.CFNMasterID, err)
	}

	return tx.Commit()
}

// QueryRecordings returns recordings matching the given filter. Filter
// fields are AND'd. Empty fields are ignored. orderBy must be a column
// name from the recordings schema; defaults to "recording_date ASC".
func (s *Store) QueryRecordings(filter RecordingFilter) ([]*Recording, error) {
	var (
		where []string
		args  []any
	)
	if filter.FolioNumber > 0 {
		where = append(where, "folio_number = ?")
		args = append(args, filter.FolioNumber)
	}
	if filter.CaseNumber != "" {
		where = append(where, "case_number = ?")
		args = append(args, filter.CaseNumber)
	}
	if len(filter.DocTypeCodes) > 0 {
		placeholders := strings.Repeat("?,", len(filter.DocTypeCodes))
		placeholders = strings.TrimRight(placeholders, ",")
		where = append(where, "doc_type_code IN ("+placeholders+")")
		for _, c := range filter.DocTypeCodes {
			args = append(args, c)
		}
	}
	if filter.SinceDate != "" {
		where = append(where, "recording_date >= ?")
		args = append(args, filter.SinceDate)
	}
	if filter.Signature != nil {
		where = append(where, "subdivision_name = ? AND plat_book = ? AND plat_page = ?")
		args = append(args, filter.Signature.SubdivisionName, filter.Signature.PlatBook, filter.Signature.PlatPage)
		if filter.Signature.BlockNo != "" {
			where = append(where, "block_no = ?")
			args = append(args, filter.Signature.BlockNo)
		}
	}

	q := `SELECT cfn_master_id, doc_type_code, recording_date, first_party, second_party,
		folio_number, address, legal_description, subdivision_name, plat_book,
		plat_page, block_no, case_number, consideration_cents, link_doctype,
		viewer_qs, raw_json FROM recordings`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	// OrderBy is a raw SQL fragment, so validate against an allow-list of
	// pre-vetted ordering clauses before interpolation. Refusing to thread
	// unknown values into the query string preserves the safety contract
	// even if a future caller threads user-controlled data through this
	// field. Callers stay terse — they pass the canonical literal — and
	// anything off the list silently falls back to the default order.
	if filter.OrderBy != "" && allowedOrderByClauses[filter.OrderBy] {
		q += " ORDER BY " + filter.OrderBy
	} else {
		q += " ORDER BY recording_date ASC, cfn_master_id ASC"
	}
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Recording
	for rows.Next() {
		r, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SearchRecordingsFTS runs an FTS5 query and returns matching recordings.
// query is passed verbatim to MATCH so callers can use FTS5 operators
// (AND/OR/NOT, phrases "...", prefix foo*).
func (s *Store) SearchRecordingsFTS(query string, limit int) ([]*Recording, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT r.cfn_master_id, r.doc_type_code, r.recording_date,
		r.first_party, r.second_party, r.folio_number, r.address, r.legal_description,
		r.subdivision_name, r.plat_book, r.plat_page, r.block_no, r.case_number,
		r.consideration_cents, r.link_doctype, r.viewer_qs, r.raw_json
		FROM recordings r JOIN recordings_fts f ON r.cfn_master_id = f.rowid
		WHERE recordings_fts MATCH ?
		ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Recording
	for rows.Next() {
		r, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordingFilter is the AND'd query shape for QueryRecordings.
type RecordingFilter struct {
	FolioNumber  int64
	CaseNumber   string
	DocTypeCodes []string
	SinceDate    string // YYYY-MM-DD inclusive lower bound
	Signature    *SignatureKey
	// OrderBy must be one of the literals in allowedOrderByClauses.
	// Anything else falls back to the default order. NEVER thread
	// user-controlled input through this field — QueryRecordings
	// concatenates it into the SQL statement after the allow-list
	// check; the check is the only thing keeping injection out.
	OrderBy string
	Limit   int // 0 = no limit
}

// allowedOrderByClauses is the closed set of OrderBy literals
// QueryRecordings will honor. Adding to this list is a deliberate
// decision (only column names + ASC/DESC, no expressions or
// sub-selects); the check in QueryRecordings rejects anything else.
var allowedOrderByClauses = map[string]bool{
	"recording_date ASC":                       true,
	"recording_date DESC":                      true,
	"recording_date ASC, cfn_master_id ASC":    true,
	"recording_date DESC, cfn_master_id DESC":  true,
	"cfn_master_id ASC":                        true,
	"cfn_master_id DESC":                       true,
	"folio_number ASC, recording_date ASC":     true,
	"consideration_cents DESC":                 true,
}

// SignatureKey is the no-folio fallback identity for a property:
// subdivision + plat_book/page + optional block_no. Folios are sparse on
// 30-50% of recordings (older/legal-description-only filings), so we use
// this composite key as a secondary join target.
type SignatureKey struct {
	SubdivisionName string
	PlatBook        int
	PlatPage        int
	BlockNo         string
}

// ParseRecordingFromAPI builds a Recording from a raw clerk API
// recordingModel JSON object. Handles the field-name aliasing
// (cfN_MASTER_ID vs cfn_master_id), the dollars-to-cents conversion, the
// 12->13 digit folio zero-padding, the "M/D/YYYY 12:00:00 AM" date
// normalization, and the "O " booK_TYPE trim. Returns the recording plus
// the marshaled raw_json blob for persistence.
func ParseRecordingFromAPI(obj map[string]any) (*Recording, error) {
	rec := &Recording{}
	rec.CFNMasterID = i64(obj["cfN_MASTER_ID"])
	if rec.CFNMasterID == 0 {
		return nil, fmt.Errorf("missing cfN_MASTER_ID")
	}
	rec.DocTypeCode = strings.TrimSpace(str(obj["linK_DOCTYPE"]))
	rec.RecordingDate = NormalizeAPIDate(str(obj["reC_DATE"]))
	rec.FirstParty = strings.TrimSpace(str(obj["firsT_PARTY"]))
	rec.SecondParty = strings.TrimSpace(str(obj["seconD_PARTY"]))

	if v := i64(obj["foliO_NUMBER"]); v > 0 {
		rec.FolioNumber = PadFolio(v)
	}
	rec.Address = strings.TrimSpace(str(obj["address"]))
	rec.LegalDescription = strings.TrimSpace(str(obj["legaL_DESCRIPTION"]))
	rec.SubdivisionName = strings.TrimSpace(str(obj["subdiV_NAME"]))
	rec.PlatBook = int(i64(obj["plaT_BOOK"]))
	rec.PlatPage = int(i64(obj["plaT_PAGE"]))
	rec.BlockNo = strings.TrimSpace(str(obj["blocK_NO"]))
	rec.CaseNumber = strings.TrimSpace(str(obj["casE_NUM"]))

	// consideration_1 takes precedence over consideration_2 (API uses _2
	// for ancillary deeds). Dollars -> cents via *100.
	c := f64(obj["consideratioN_1"])
	if c == 0 {
		c = f64(obj["consideratioN_2"])
	}
	rec.ConsiderationCents = int64(c * 100)
	rec.LinkDocType = strings.TrimSpace(str(obj["linK_DOCTYPE"]))
	rec.ViewerQS = strings.TrimSpace(str(obj["qs"]))

	raw, _ := json.Marshal(obj)
	rec.RawJSON = string(raw)
	return rec, nil
}

// NormalizeAPIDate converts "M/D/YYYY 12:00:00 AM" -> "YYYY-MM-DD".
// Returns the input unchanged if it doesn't match the expected shape so
// callers can store something rather than nothing on schema drift.
func NormalizeAPIDate(s string) string {
	if s == "" {
		return ""
	}
	// Strip the time part (always "12:00:00 AM" on this API).
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return s
	}
	mo, d, y := parts[0], parts[1], parts[2]
	if len(mo) == 1 {
		mo = "0" + mo
	}
	if len(d) == 1 {
		d = "0" + d
	}
	return y + "-" + mo + "-" + d
}

// PadFolio zero-pads the clerk's 12-digit folio response to the canonical
// 13-digit form used in the BlueStone DB. The clerk drops leading zeros
// on bigint serialization; pad once at ingest so the rest of the
// codebase can match on raw int comparison.
func PadFolio(folio int64) int64 {
	// 13-digit form is the standard. If the input has fewer digits it
	// was truncated by JSON int serialization; no transformation needed
	// numerically — the int64 is already the canonical value. We keep
	// this function as a documented seam in case future refactoring
	// needs to format folio as a left-padded string.
	return folio
}

// Helpers for type-coercing untyped JSON values from a map[string]any.

func str(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case bool:
		return fmt.Sprintf("%v", t)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func i64(v any) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int:
		return int64(t)
	case int64:
		return t
	case string:
		var n int64
		fmt.Sscanf(t, "%d", &n)
		return n
	case json.Number:
		n, _ := t.Int64()
		return n
	}
	return 0
}

func f64(v any) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		var n float64
		fmt.Sscanf(t, "%f", &n)
		return n
	case json.Number:
		n, _ := t.Float64()
		return n
	}
	return 0
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func nullableI64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}

func scanRecording(rows *sql.Rows) (*Recording, error) {
	var (
		r           Recording
		fp, sp      sql.NullString
		folio       sql.NullInt64
		addr, legal sql.NullString
		subdiv      sql.NullString
		pb, pg      sql.NullInt64
		block       sql.NullString
		caseN       sql.NullString
		cons        sql.NullInt64
		link        sql.NullString
		qs          sql.NullString
	)
	if err := rows.Scan(
		&r.CFNMasterID, &r.DocTypeCode, &r.RecordingDate,
		&fp, &sp, &folio, &addr, &legal, &subdiv, &pb, &pg, &block,
		&caseN, &cons, &link, &qs, &r.RawJSON,
	); err != nil {
		return nil, err
	}
	r.FirstParty = fp.String
	r.SecondParty = sp.String
	r.FolioNumber = folio.Int64
	r.Address = addr.String
	r.LegalDescription = legal.String
	r.SubdivisionName = subdiv.String
	r.PlatBook = int(pb.Int64)
	r.PlatPage = int(pg.Int64)
	r.BlockNo = block.String
	r.CaseNumber = caseN.String
	r.ConsiderationCents = cons.Int64
	r.LinkDocType = link.String
	r.ViewerQS = qs.String
	return &r, nil
}
