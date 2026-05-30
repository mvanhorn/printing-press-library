// Hand-authored — NOT generated. Local cache queries + watchlist for the GFW
// novel features (vessel list, watch pin/unpin/refresh/since). Vessel identities
// are cached as flattened JSON in the generic `resources` table under
// resource_type="vessel" (keyed by GFW vessel id); these helpers read them back
// and join against the gfw_watchlist table.
package store

import (
	"fmt"
	"strings"
	"time"
)

// VesselRow is one cached vessel identity joined against the watchlist.
type VesselRow struct {
	VesselID  string `json:"vessel_id"`
	Name      string `json:"name,omitempty"`
	Flag      string `json:"flag,omitempty"`
	SSVID     string `json:"ssvid,omitempty"`
	IMO       string `json:"imo,omitempty"`
	CallSign  string `json:"call_sign,omitempty"`
	ShipType  string `json:"ship_type,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	SyncedAt  string `json:"synced_at,omitempty"`
	Pinned    bool   `json:"pinned"`
	PinLabel  string `json:"pin_label,omitempty"`
}

// PinRow is one watchlist entry.
type PinRow struct {
	VesselID string `json:"vessel_id"`
	Label    string `json:"label,omitempty"`
	PinnedAt string `json:"pinned_at,omitempty"`
}

// ListVesselsOptions filters ListVessels. Zero-value fields are ignored.
type ListVesselsOptions struct {
	Flag       string
	NameLike   string
	PinnedOnly bool
	Limit      int
}

const vesselSelect = `r.id,
	COALESCE(json_extract(r.data,'$.name'),''),
	COALESCE(json_extract(r.data,'$.flag'),''),
	COALESCE(json_extract(r.data,'$.ssvid'),''),
	COALESCE(json_extract(r.data,'$.imo'),''),
	COALESCE(json_extract(r.data,'$.call_sign'),''),
	COALESCE(json_extract(r.data,'$.ship_type'),''),
	COALESCE(json_extract(r.data,'$.fetched_at'),''),
	COALESCE(r.synced_at,''),
	CASE WHEN w.vessel_id IS NOT NULL THEN 1 ELSE 0 END,
	COALESCE(w.label,'')`

const vesselFrom = ` FROM resources r LEFT JOIN gfw_watchlist w ON w.vessel_id = r.id WHERE r.resource_type = 'vessel'`

// ListVessels returns cached vessel identities matching the filters, newest first.
func (s *Store) ListVessels(opts ListVesselsOptions) ([]VesselRow, error) {
	q := "SELECT " + vesselSelect + vesselFrom
	var args []any
	if opts.Flag != "" {
		q += " AND LOWER(json_extract(r.data,'$.flag')) = LOWER(?)"
		args = append(args, opts.Flag)
	}
	if opts.NameLike != "" {
		q += " AND (LOWER(json_extract(r.data,'$.name')) LIKE LOWER(?) OR LOWER(json_extract(r.data,'$.imo')) LIKE LOWER(?) OR LOWER(json_extract(r.data,'$.ssvid')) LIKE LOWER(?))"
		like := "%" + opts.NameLike + "%"
		args = append(args, like, like, like)
	}
	if opts.PinnedOnly {
		q += " AND w.vessel_id IS NOT NULL"
	}
	q += " ORDER BY r.synced_at DESC"
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
	out := []VesselRow{}
	for rows.Next() {
		var r VesselRow
		var pinned int
		if err := rows.Scan(&r.VesselID, &r.Name, &r.Flag, &r.SSVID, &r.IMO, &r.CallSign,
			&r.ShipType, &r.FetchedAt, &r.SyncedAt, &pinned, &r.PinLabel); err != nil {
			return nil, err
		}
		r.Pinned = pinned == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// PinVessel adds (or re-labels) a vessel on the watchlist.
func (s *Store) PinVessel(vesselID, label string) error {
	vesselID = strings.TrimSpace(vesselID)
	if vesselID == "" {
		return fmt.Errorf("cannot pin: empty vessel id")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO gfw_watchlist (vessel_id, label, pinned_at) VALUES (?, ?, ?)
		 ON CONFLICT(vessel_id) DO UPDATE SET label = excluded.label`,
		vesselID, label, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UnpinVessel removes a vessel from the watchlist; bool reports whether a row was removed.
func (s *Store) UnpinVessel(vesselID string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM gfw_watchlist WHERE vessel_id = ?`, strings.TrimSpace(vesselID))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListPins returns the watchlist, most-recently pinned first.
func (s *Store) ListPins() ([]PinRow, error) {
	rows, err := s.db.Query(`SELECT vessel_id, COALESCE(label,''), COALESCE(pinned_at,'') FROM gfw_watchlist ORDER BY pinned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PinRow{}
	for rows.Next() {
		var p PinRow
		if err := rows.Scan(&p.VesselID, &p.Label, &p.PinnedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PinnedVesselIDs returns just the watchlisted vessel ids.
func (s *Store) PinnedVesselIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT vessel_id FROM gfw_watchlist ORDER BY pinned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
