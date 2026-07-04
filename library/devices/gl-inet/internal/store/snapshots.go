// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
)

// ConfigSnapshot is a named, restorable capture of a GL.iNet router's whole
// UCI configuration plus the provenance needed to safely apply it back.
type ConfigSnapshot struct {
	Name         string `json:"name"`
	CreatedAt    string `json:"created_at"`
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	OpenWrt      string `json:"openwrt"`
	Luci         string `json:"luci"`
	CountryCodes string `json:"country_codes"`
	Notes        string `json:"notes"`
	UCIExport    string `json:"uci_export"`
	UCIShow      string `json:"uci_show"`
}

// SaveSnapshot inserts or replaces a config snapshot row by name.
func (s *Store) SaveSnapshot(snap ConfigSnapshot) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO config_snapshots
			(name, created_at, model, firmware, openwrt, luci, country_codes, notes, uci_export, uci_show)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
			created_at = excluded.created_at,
			model = excluded.model,
			firmware = excluded.firmware,
			openwrt = excluded.openwrt,
			luci = excluded.luci,
			country_codes = excluded.country_codes,
			notes = excluded.notes,
			uci_export = excluded.uci_export,
			uci_show = excluded.uci_show`,
		snap.Name, snap.CreatedAt, snap.Model, snap.Firmware, snap.OpenWrt,
		snap.Luci, snap.CountryCodes, snap.Notes, snap.UCIExport, snap.UCIShow,
	)
	if err != nil {
		return fmt.Errorf("saving snapshot %q: %w", snap.Name, err)
	}
	return nil
}

// ListSnapshots returns all snapshots ordered by creation time (newest first).
func (s *Store) ListSnapshots() ([]ConfigSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT name, created_at, model, firmware, openwrt, luci, country_codes, notes, uci_export, uci_show
		 FROM config_snapshots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConfigSnapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// GetSnapshot returns one snapshot by name, or sql.ErrNoRows on a miss.
func (s *Store) GetSnapshot(name string) (*ConfigSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT name, created_at, model, firmware, openwrt, luci, country_codes, notes, uci_export, uci_show
		 FROM config_snapshots WHERE name = ?`, name)
	snap, err := scanSnapshot(row)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

// DeleteSnapshot removes a snapshot by name. Returns the number of rows deleted.
func (s *Store) DeleteSnapshot(name string) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM config_snapshots WHERE name = ?`, name)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// scanner abstracts *sql.Row and *sql.Rows so scanSnapshot serves both.
type scanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(sc scanner) (ConfigSnapshot, error) {
	var snap ConfigSnapshot
	var createdAt, model, firmware, openwrt, luci, country, notes, export, show sql.NullString
	if err := sc.Scan(&snap.Name, &createdAt, &model, &firmware, &openwrt, &luci, &country, &notes, &export, &show); err != nil {
		return ConfigSnapshot{}, err
	}
	snap.CreatedAt = createdAt.String
	snap.Model = model.String
	snap.Firmware = firmware.String
	snap.OpenWrt = openwrt.String
	snap.Luci = luci.String
	snap.CountryCodes = country.String
	snap.Notes = notes.String
	snap.UCIExport = export.String
	snap.UCIShow = show.String
	return snap, nil
}
