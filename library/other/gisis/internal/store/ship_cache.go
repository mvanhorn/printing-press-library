// Hand-authored — NOT generated. Cache + watchlist layer for the GISIS novel
// features (ship list / stale / batch / pin / refresh, owner fleet).
//
// The generated UpsertShip keys rows via extractObjectID, which for a GISIS
// ship payload (no "id" field) falls through to the ship name — wrong, since
// vessels rename. GISIS ships must be keyed by IMO number, so this file adds
// UpsertShipByIMO plus the typed query helpers the novel commands read from.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ShipRow is one row of the typed "ship" cache table, joined against the
// watchlist. Text/integer columns are COALESCEd to zero values in SQL so a
// NULL never breaks the scan.
type ShipRow struct {
	IMONumber             string `json:"imo_number"`
	Name                  string `json:"name,omitempty"`
	CallSign              string `json:"call_sign,omitempty"`
	Flag                  string `json:"flag,omitempty"`
	ShipType              string `json:"ship_type,omitempty"`
	GrossTonnage          int    `json:"gross_tonnage,omitempty"`
	Deadweight            int    `json:"deadweight,omitempty"`
	YearBuilt             int    `json:"year_built,omitempty"`
	RegisteredOwner       string `json:"registered_owner,omitempty"`
	Operator              string `json:"operator,omitempty"`
	ShipManager           string `json:"ship_manager,omitempty"`
	ClassificationSociety string `json:"classification_society,omitempty"`
	Status                string `json:"status,omitempty"`
	SourceURL             string `json:"source_url,omitempty"`
	FetchedAt             string `json:"fetched_at,omitempty"`
	SyncedAt              string `json:"synced_at,omitempty"`
	Pinned                bool   `json:"pinned"`
	PinLabel              string `json:"pin_label,omitempty"`
}

// PinRow is one watchlist entry.
type PinRow struct {
	IMONumber string `json:"imo_number"`
	Label     string `json:"label,omitempty"`
	PinnedAt  string `json:"pinned_at,omitempty"`
}

// ListShipsOptions filters ListShips. Zero-value fields are ignored.
type ListShipsOptions struct {
	Flag       string
	Owner      string
	ShipType   string
	NameLike   string
	PinnedOnly bool
	Limit      int
}

// shipSelectColumns is the projection shared by every cache query. Every
// column is COALESCEd so a NULL scans cleanly into the Go zero value, and the
// LEFT JOIN against ship_pins yields the pinned flag + label in one round trip.
const shipSelectColumns = `COALESCE(s.imo_number,''), COALESCE(s.name,''), COALESCE(s.call_sign,''), ` +
	`COALESCE(s.flag,''), COALESCE(s.ship_type,''), COALESCE(s.gross_tonnage,0), COALESCE(s.deadweight,0), ` +
	`COALESCE(s.year_built,0), COALESCE(s.registered_owner,''), COALESCE(s.operator,''), COALESCE(s.ship_manager,''), ` +
	`COALESCE(s.classification_society,''), COALESCE(s.status,''), COALESCE(s.source_url,''), COALESCE(s.fetched_at,''), ` +
	`COALESCE(s.synced_at,''), CASE WHEN p.imo_number IS NOT NULL THEN 1 ELSE 0 END, COALESCE(p.label,'')`

const shipFromJoin = ` FROM "ship" s LEFT JOIN ship_pins p ON p.imo_number = s.imo_number`

var yearRE = regexp.MustCompile(`\b(\d{4})\b`)

// UpsertShipByIMO caches a parsed ship payload keyed by IMO number — the GISIS
// primary identity. It populates the typed "ship" columns (for the cache
// queries) and the generic resources row (for FTS + provenance), in one
// transaction. The stored data blob is left untouched (no synthetic id is
// written into it) so `ship get` output stays clean.
func (s *Store) UpsertShipByIMO(imo string, data json.RawMessage) error {
	imo = strings.TrimSpace(imo)
	if imo == "" {
		return fmt.Errorf("cannot cache ship: empty IMO number")
	}
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return fmt.Errorf("decoding ship payload: %w", err)
	}
	// Derive year_built from the parser's date_of_build so `ship list` can show
	// a build year (the typed column exists but the parser emits date_of_build).
	if _, ok := obj["year_built"]; !ok {
		if y := yearFromDateOfBuild(obj); y != 0 {
			obj["year_built"] = y
		}
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "ship", imo, data); err != nil {
		return err
	}
	if err := s.upsertShipTx(tx, imo, obj, data); err != nil {
		return err
	}
	return tx.Commit()
}

func yearFromDateOfBuild(obj map[string]any) int {
	v, ok := obj["date_of_build"]
	if !ok {
		return 0
	}
	str, ok := v.(string)
	if !ok {
		return 0
	}
	m := yearRE.FindStringSubmatch(str)
	if len(m) < 2 {
		return 0
	}
	y, err := strconv.Atoi(m[1])
	if err != nil || y < 1800 || y > 2200 {
		return 0
	}
	return y
}

// ListShips returns cached vessels matching the given filters, newest first.
func (s *Store) ListShips(opts ListShipsOptions) ([]ShipRow, error) {
	var where []string
	var args []any
	if opts.Flag != "" {
		where = append(where, "LOWER(s.flag) = LOWER(?)")
		args = append(args, opts.Flag)
	}
	if opts.Owner != "" {
		where = append(where, "LOWER(s.registered_owner) LIKE LOWER(?)")
		args = append(args, "%"+opts.Owner+"%")
	}
	if opts.ShipType != "" {
		where = append(where, "LOWER(s.ship_type) = LOWER(?)")
		args = append(args, opts.ShipType)
	}
	if opts.NameLike != "" {
		where = append(where, "(LOWER(s.name) LIKE LOWER(?) OR LOWER(s.registered_owner) LIKE LOWER(?))")
		args = append(args, "%"+opts.NameLike+"%", "%"+opts.NameLike+"%")
	}
	if opts.PinnedOnly {
		where = append(where, "p.imo_number IS NOT NULL")
	}

	q := "SELECT " + shipSelectColumns + shipFromJoin
	if len(where) > 0 {
		// #nosec G202 -- every joined fragment is a compile-time string literal; all user
		// values bind through `args` and the `?` placeholders in s.db.Query below.
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY s.synced_at DESC"
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	q += " LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShipRows(rows)
}

// OwnerFleet returns every cached vessel whose registered owner matches owner.
// With like=false the match is exact (case-insensitive); with like=true it is a
// substring match.
func (s *Store) OwnerFleet(owner string, like bool) ([]ShipRow, error) {
	cond := "LOWER(s.registered_owner) = LOWER(?)"
	arg := owner
	if like {
		cond = "LOWER(s.registered_owner) LIKE LOWER(?)"
		arg = "%" + owner + "%"
	}
	q := "SELECT " + shipSelectColumns + shipFromJoin + " WHERE " + cond + " ORDER BY s.name"
	rows, err := s.db.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShipRows(rows)
}

// StaleShips returns cached vessels whose last sync predates cutoff, oldest
// first. When pinnedOnly is set, only watchlisted vessels are considered.
func (s *Store) StaleShips(cutoff time.Time, pinnedOnly bool) ([]ShipRow, error) {
	where := []string{"s.synced_at < ?"}
	args := []any{cutoff.UTC().Format(time.RFC3339)}
	if pinnedOnly {
		where = append(where, "p.imo_number IS NOT NULL")
	}
	// #nosec G202 -- every joined fragment is a compile-time string literal; the only
	// user value (cutoff) binds through `args` and the `?` placeholder in s.db.Query below.
	q := "SELECT " + shipSelectColumns + shipFromJoin + " WHERE " + strings.Join(where, " AND ") + " ORDER BY s.synced_at ASC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanShipRows(rows)
}

func scanShipRows(rows *sql.Rows) ([]ShipRow, error) {
	out := []ShipRow{}
	for rows.Next() {
		var r ShipRow
		var pinned int
		if err := rows.Scan(
			&r.IMONumber, &r.Name, &r.CallSign, &r.Flag, &r.ShipType,
			&r.GrossTonnage, &r.Deadweight, &r.YearBuilt, &r.RegisteredOwner,
			&r.Operator, &r.ShipManager, &r.ClassificationSociety, &r.Status,
			&r.SourceURL, &r.FetchedAt, &r.SyncedAt, &pinned, &r.PinLabel,
		); err != nil {
			return nil, err
		}
		r.Pinned = pinned == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// PinShip adds (or re-labels) an IMO on the watchlist.
func (s *Store) PinShip(imo, label string) error {
	imo = strings.TrimSpace(imo)
	if imo == "" {
		return fmt.Errorf("cannot pin: empty IMO number")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO ship_pins (imo_number, label, pinned_at) VALUES (?, ?, ?)
		 ON CONFLICT(imo_number) DO UPDATE SET label = excluded.label`,
		imo, label, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UnpinShip removes an IMO from the watchlist. The bool reports whether a row
// was actually removed (false = it wasn't pinned).
func (s *Store) UnpinShip(imo string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM ship_pins WHERE imo_number = ?`, strings.TrimSpace(imo))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListPins returns the watchlist, most-recently pinned first.
func (s *Store) ListPins() ([]PinRow, error) {
	rows, err := s.db.Query(`SELECT imo_number, COALESCE(label,''), COALESCE(pinned_at,'') FROM ship_pins ORDER BY pinned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PinRow{}
	for rows.Next() {
		var p PinRow
		if err := rows.Scan(&p.IMONumber, &p.Label, &p.PinnedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PinnedIMOs returns just the IMO numbers on the watchlist.
func (s *Store) PinnedIMOs() ([]string, error) {
	rows, err := s.db.Query(`SELECT imo_number FROM ship_pins ORDER BY pinned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var imo string
		if err := rows.Scan(&imo); err != nil {
			return nil, err
		}
		out = append(out, imo)
	}
	return out, rows.Err()
}
