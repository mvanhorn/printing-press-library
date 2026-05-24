package store

import (
	"database/sql"
	"errors"
	"time"
)

// SiteCapabilities records which sensors a given OmniLogic install actually
// has. Hayward returns -1/null for absent sensors and the CLI can't
// distinguish "sensor missing entirely" from "sensor offline right now"
// without operator input. status / chemistry get / telemetry get filter
// their verdicts against this row; when no row exists the consumers default
// to "assume all sensors are equipped" (the most common OmniLogic config).
//
// TempNeedsFlow=true captures the common quirk where the water-temperature
// sensor only reports while the pump is moving water past it — when the pump
// is idle, water_temp=-1 is the expected reading, not a sensor failure.
type SiteCapabilities struct {
	SiteMspSystemID int
	HasPHSensor     bool
	HasORPSensor    bool
	HasSaltSensor   bool
	TempNeedsFlow   bool
	ConfiguredAt    time.Time
	Notes           string
}

// AssumeAllEquipped returns the default capability row used when no row is
// stored for a site. Backward-compatible with pre-capabilities CLI behavior.
func AssumeAllEquipped(siteID int) SiteCapabilities {
	return SiteCapabilities{
		SiteMspSystemID: siteID,
		HasPHSensor:     true,
		HasORPSensor:    true,
		HasSaltSensor:   true,
		TempNeedsFlow:   false,
	}
}

// GetSiteCapabilities returns the stored capability row for a site, or nil
// when no row is configured. Callers should distinguish "not configured"
// from "configured" to know whether to surface the setup hint.
func (s *Store) GetSiteCapabilities(siteID int) (*SiteCapabilities, error) {
	row := s.DB.QueryRow(
		`SELECT has_ph_sensor, has_orp_sensor, has_salt_sensor, temp_needs_flow, configured_at, COALESCE(notes,'')
		 FROM site_capabilities WHERE site_msp_system_id = ?`,
		siteID,
	)
	var ph, orp, salt, tempFlow int
	var configuredAt, notes string
	if err := row.Scan(&ph, &orp, &salt, &tempFlow, &configuredAt, &notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	ts, _ := time.Parse(time.RFC3339, configuredAt)
	return &SiteCapabilities{
		SiteMspSystemID: siteID,
		HasPHSensor:     ph != 0,
		HasORPSensor:    orp != 0,
		HasSaltSensor:   salt != 0,
		TempNeedsFlow:   tempFlow != 0,
		ConfiguredAt:    ts,
		Notes:           notes,
	}, nil
}

// SetSiteCapabilities upserts a capability row. The configured_at timestamp
// is set to time.Now() if SiteCapabilities.ConfiguredAt is zero.
func (s *Store) SetSiteCapabilities(c SiteCapabilities) error {
	ts := c.ConfiguredAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	notes := sql.NullString{String: c.Notes, Valid: c.Notes != ""}
	_, err := s.DB.Exec(
		`INSERT INTO site_capabilities
		   (site_msp_system_id, has_ph_sensor, has_orp_sensor, has_salt_sensor, temp_needs_flow, configured_at, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(site_msp_system_id) DO UPDATE SET
		   has_ph_sensor=excluded.has_ph_sensor,
		   has_orp_sensor=excluded.has_orp_sensor,
		   has_salt_sensor=excluded.has_salt_sensor,
		   temp_needs_flow=excluded.temp_needs_flow,
		   configured_at=excluded.configured_at,
		   notes=excluded.notes`,
		c.SiteMspSystemID,
		boolToInt(c.HasPHSensor),
		boolToInt(c.HasORPSensor),
		boolToInt(c.HasSaltSensor),
		boolToInt(c.TempNeedsFlow),
		ts.UTC().Format(time.RFC3339),
		notes,
	)
	return err
}

// ClearSiteCapabilities removes the capability row for a site. After clear,
// consumers fall back to "assume all sensors are equipped" and (when chemistry
// is null) re-emit the setup_hint.
func (s *Store) ClearSiteCapabilities(siteID int) error {
	_, err := s.DB.Exec(`DELETE FROM site_capabilities WHERE site_msp_system_id = ?`, siteID)
	return err
}

// ListSiteCapabilities returns every configured site_capabilities row,
// stable-sorted by site ID.
func (s *Store) ListSiteCapabilities() ([]SiteCapabilities, error) {
	rows, err := s.DB.Query(
		`SELECT site_msp_system_id, has_ph_sensor, has_orp_sensor, has_salt_sensor, temp_needs_flow, configured_at, COALESCE(notes,'')
		 FROM site_capabilities ORDER BY site_msp_system_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteCapabilities
	for rows.Next() {
		var c SiteCapabilities
		var ph, orp, salt, tempFlow int
		var ts string
		if err := rows.Scan(&c.SiteMspSystemID, &ph, &orp, &salt, &tempFlow, &ts, &c.Notes); err != nil {
			return nil, err
		}
		c.HasPHSensor = ph != 0
		c.HasORPSensor = orp != 0
		c.HasSaltSensor = salt != 0
		c.TempNeedsFlow = tempFlow != 0
		c.ConfiguredAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, c)
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
