package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/omnilogic"
)

// UpsertSites replaces the sites table with the latest cloud snapshot. We
// don't soft-delete vanished sites — if Hayward removes one from your
// account, it stops appearing here too.
func (s *Store) UpsertSites(sites []omnilogic.Site) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, site := range sites {
		if _, err := tx.Exec(
			`INSERT INTO sites (msp_system_id, backyard_name, last_seen_at) VALUES (?, ?, ?)
			 ON CONFLICT(msp_system_id) DO UPDATE SET backyard_name=excluded.backyard_name, last_seen_at=excluded.last_seen_at`,
			site.MspSystemID, site.BackyardName, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertMspConfig writes a full MSP config snapshot AND upserts the
// flattened BoW + equipment rows. The raw XML is preserved for replay /
// audit.
func (s *Store) UpsertMspConfig(cfg *omnilogic.MspConfig) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	summary, _ := json.Marshal(cfg)
	if _, err := tx.Exec(
		`INSERT INTO msp_config_snapshots (site_msp_system_id, fetched_at, raw_xml, summary_json) VALUES (?, ?, ?, ?)`,
		cfg.MspSystemID, now, cfg.RawXML, string(summary),
	); err != nil {
		return err
	}
	for _, bow := range cfg.BodiesOfWater {
		if _, err := tx.Exec(
			`INSERT INTO bodies_of_water (bow_system_id, site_msp_system_id, name, type, shared_type, shared_equip_id, supports_spillover, last_seen_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(site_msp_system_id, bow_system_id) DO UPDATE SET
			   name=excluded.name, type=excluded.type, shared_type=excluded.shared_type,
			   shared_equip_id=excluded.shared_equip_id, supports_spillover=excluded.supports_spillover,
			   last_seen_at=excluded.last_seen_at`,
			bow.SystemID, cfg.MspSystemID, bow.Name, bow.Type, bow.SharedType,
			bow.SharedEquipID, bow.SupportsSpillover, now,
		); err != nil {
			return err
		}
		for _, p := range bow.Pumps {
			if err := upsertEquip(tx, cfg.MspSystemID, bow.SystemID, "pump", p, now); err != nil {
				return err
			}
		}
		for _, h := range bow.Heaters {
			eq := omnilogic.Equipment{
				SystemID: h.SystemID, Name: h.Name, Type: h.HeaterType, Function: "heater",
			}
			if err := upsertEquip(tx, cfg.MspSystemID, bow.SystemID, "heater", eq, now); err != nil {
				return err
			}
		}
		for _, l := range bow.Lights {
			if err := upsertEquip(tx, cfg.MspSystemID, bow.SystemID, "light", l, now); err != nil {
				return err
			}
		}
		for _, r := range bow.Relays {
			if err := upsertEquip(tx, cfg.MspSystemID, bow.SystemID, "relay", r, now); err != nil {
				return err
			}
		}
		if bow.Chlorinator != nil {
			if err := upsertEquip(tx, cfg.MspSystemID, bow.SystemID, "chlorinator", *bow.Chlorinator, now); err != nil {
				return err
			}
		}
	}
	for _, r := range cfg.Relays {
		if err := upsertEquip(tx, cfg.MspSystemID, "", "relay", r, now); err != nil {
			return err
		}
	}
	// Refresh equipment FTS
	if _, err := tx.Exec(`INSERT INTO equipment_fts(equipment_fts) VALUES('rebuild')`); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertEquip(tx *sql.Tx, siteID int, bowSystemID, kind string, eq omnilogic.Equipment, now string) error {
	if eq.SystemID == "" {
		return nil
	}
	bowVal := sql.NullString{String: bowSystemID, Valid: bowSystemID != ""}
	_, err := tx.Exec(
		`INSERT INTO equipment (equipment_system_id, site_msp_system_id, bow_system_id, name, kind, type, function, min_speed, max_speed, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(site_msp_system_id, equipment_system_id) DO UPDATE SET
		   bow_system_id=excluded.bow_system_id, name=excluded.name, kind=excluded.kind, type=excluded.type,
		   function=excluded.function, min_speed=excluded.min_speed, max_speed=excluded.max_speed,
		   last_seen_at=excluded.last_seen_at`,
		eq.SystemID, siteID, bowVal, eq.Name, kind, eq.Type, eq.Function, eq.MinSpeed, eq.MaxSpeed, now,
	)
	return err
}

// AppendTelemetry records every reading in a telemetry snapshot as its own
// row in telemetry_samples. The append-only design is what makes drift /
// runtime / chemistry log possible.
func (s *Store) AppendTelemetry(t *omnilogic.Telemetry) (int, error) {
	if t == nil {
		return 0, nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	ts := t.SampledAt.UTC().Format(time.RFC3339)
	count := 0
	// Hayward emits -1 (and occasionally 0) as a "sensor not reading"
	// sentinel for chemistry probes and water-temp sensors. Writing
	// those to the telemetry_samples store would corrupt every
	// downstream analysis: drift baselines drag toward zero, chemistry
	// log shows false readings, runtime calculations break. Filter at
	// write-time so the time-series table only contains real
	// measurements. Defense-in-depth: analysis functions also re-filter
	// at read-time to defend against legacy rows already in the store.
	// Greptile P1 #3216464198.
	if t.AirTemp != nil && *t.AirTemp > 0 {
		if err := appendSample(tx, t.MspSystemID, "", "", "air_temp", *t.AirTemp, ts); err != nil {
			return count, err
		}
		count++
	}
	for _, bow := range t.BodiesOfWater {
		if bow.WaterTemp != nil && *bow.WaterTemp > 0 {
			if err := appendSample(tx, t.MspSystemID, bow.SystemID, "", "water_temp", *bow.WaterTemp, ts); err != nil {
				return count, err
			}
			count++
		}
		if bow.PH != nil && *bow.PH > 0 {
			if err := appendSampleReal(tx, t.MspSystemID, bow.SystemID, "", "ph", *bow.PH, ts); err != nil {
				return count, err
			}
			count++
		}
		if bow.ORP != nil && *bow.ORP > 0 {
			if err := appendSample(tx, t.MspSystemID, bow.SystemID, "", "orp", *bow.ORP, ts); err != nil {
				return count, err
			}
			count++
		}
		if bow.SaltPPM != nil && *bow.SaltPPM > 0 {
			if err := appendSample(tx, t.MspSystemID, bow.SystemID, "", "salt_ppm", *bow.SaltPPM, ts); err != nil {
				return count, err
			}
			count++
		}
		if bow.ChlorOutputPct != nil {
			if err := appendSample(tx, t.MspSystemID, bow.SystemID, "", "chlor_output_pct", *bow.ChlorOutputPct, ts); err != nil {
				return count, err
			}
			count++
		}
		for _, p := range bow.Pumps {
			if p.Speed != nil {
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, p.SystemID, "pump_speed", *p.Speed, ts); err != nil {
					return count, err
				}
				count++
			}
			if p.IsOn != nil {
				v := 0
				if *p.IsOn {
					v = 1
				}
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, p.SystemID, "pump_on", v, ts); err != nil {
					return count, err
				}
				count++
			}
		}
		for _, h := range bow.Heaters {
			if h.Enabled != nil {
				v := 0
				if *h.Enabled {
					v = 1
				}
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, h.SystemID, "heater_enabled", v, ts); err != nil {
					return count, err
				}
				count++
			}
			if h.SetPoint != nil {
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, h.SystemID, "heater_setpoint", *h.SetPoint, ts); err != nil {
					return count, err
				}
				count++
			}
		}
		for _, l := range bow.Lights {
			if l.IsOn != nil {
				v := 0
				if *l.IsOn {
					v = 1
				}
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, l.SystemID, "light_on", v, ts); err != nil {
					return count, err
				}
				count++
			}
		}
		for _, r := range bow.Relays {
			if r.IsOn != nil {
				v := 0
				if *r.IsOn {
					v = 1
				}
				if err := appendSample(tx, t.MspSystemID, bow.SystemID, r.SystemID, "relay_on", v, ts); err != nil {
					return count, err
				}
				count++
			}
		}
	}
	for _, r := range t.Relays {
		if r.IsOn != nil {
			v := 0
			if *r.IsOn {
				v = 1
			}
			if err := appendSample(tx, t.MspSystemID, "", r.SystemID, "relay_on", v, ts); err != nil {
				return count, err
			}
			count++
		}
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func appendSample(tx *sql.Tx, site int, bow, equip, metric string, val int, ts string) error {
	bowVal := sql.NullString{String: bow, Valid: bow != ""}
	eqVal := sql.NullString{String: equip, Valid: equip != ""}
	_, err := tx.Exec(
		`INSERT INTO telemetry_samples (site_msp_system_id, bow_system_id, equipment_system_id, metric, value_int, sampled_at) VALUES (?, ?, ?, ?, ?, ?)`,
		site, bowVal, eqVal, metric, val, ts,
	)
	return err
}

func appendSampleReal(tx *sql.Tx, site int, bow, equip, metric string, val float64, ts string) error {
	bowVal := sql.NullString{String: bow, Valid: bow != ""}
	eqVal := sql.NullString{String: equip, Valid: equip != ""}
	_, err := tx.Exec(
		`INSERT INTO telemetry_samples (site_msp_system_id, bow_system_id, equipment_system_id, metric, value_real, sampled_at) VALUES (?, ?, ?, ?, ?, ?)`,
		site, bowVal, eqVal, metric, val, ts,
	)
	return err
}

// UpsertAlarms updates the alarms table from a cloud snapshot. Existing
// alarms get their last_seen bumped; new alarms get a new row; alarms that
// no longer appear get cleared_at set. The alarm_key is composite so the
// same code on different equipment doesn't merge.
func (s *Store) UpsertAlarms(siteID int, alarms []omnilogic.Alarm) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	currentKeys := map[string]bool{}
	for _, a := range alarms {
		key := fmt.Sprintf("%d|%s|%s|%s", siteID, a.EquipmentID, a.Code, a.Message)
		currentKeys[key] = true
		raw, _ := json.Marshal(a.Raw)
		_, err := tx.Exec(
			`INSERT INTO alarms (alarm_key, site_msp_system_id, bow_system_id, equipment_system_id, code, severity, message, raw_json, first_seen, last_seen, cleared_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
			 ON CONFLICT(alarm_key) DO UPDATE SET last_seen=excluded.last_seen, cleared_at=NULL`,
			key, siteID, sql.NullString{String: a.BowID, Valid: a.BowID != ""},
			sql.NullString{String: a.EquipmentID, Valid: a.EquipmentID != ""},
			a.Code, a.Severity, a.Message, string(raw), now, now,
		)
		if err != nil {
			return err
		}
	}
	// Clear alarms that were present last sync but aren't now.
	rows, err := tx.Query(`SELECT alarm_key FROM alarms WHERE site_msp_system_id = ? AND cleared_at IS NULL`, siteID)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var key string
		_ = rows.Scan(&key)
		if !currentKeys[key] {
			stale = append(stale, key)
		}
	}
	rows.Close()
	for _, key := range stale {
		if _, err := tx.Exec(`UPDATE alarms SET cleared_at = ? WHERE alarm_key = ?`, now, key); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO alarms_fts(alarms_fts) VALUES('rebuild')`); err != nil {
		return err
	}
	return tx.Commit()
}

// LogCommand persists a Set* (or any side-effecting) operation invocation.
type CommandLogEntry struct {
	ID     int64
	Ts     time.Time
	Op     string
	Target string
	Params map[string]any
	Status string
	Detail string
	DryRun bool
}

func (s *Store) LogCommand(e CommandLogEntry) (int64, error) {
	paramsJSON, _ := json.Marshal(e.Params)
	ts := e.Ts
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	dryRun := 0
	if e.DryRun {
		dryRun = 1
	}
	res, err := s.DB.Exec(
		`INSERT INTO command_log (ts, op, target, params_json, status, detail, dry_run) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ts.Format(time.RFC3339), e.Op, e.Target, string(paramsJSON), e.Status, e.Detail, dryRun,
	)
	if err != nil {
		return 0, err
	}
	_, _ = s.DB.Exec(`INSERT INTO command_log_fts(command_log_fts) VALUES('rebuild')`)
	return res.LastInsertId()
}

// ListSites returns sites in stable msp_system_id order.
func (s *Store) ListSites() ([]omnilogic.Site, error) {
	rows, err := s.DB.Query(`SELECT msp_system_id, backyard_name FROM sites ORDER BY msp_system_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []omnilogic.Site
	for rows.Next() {
		var s omnilogic.Site
		if err := rows.Scan(&s.MspSystemID, &s.BackyardName); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// LatestMspConfig returns the most recent MSP config snapshot for a site,
// parsed back into MspConfig. Returns nil if no snapshot exists.
func (s *Store) LatestMspConfig(siteID int) (*omnilogic.MspConfig, error) {
	row := s.DB.QueryRow(
		`SELECT summary_json FROM msp_config_snapshots WHERE site_msp_system_id = ? ORDER BY fetched_at DESC LIMIT 1`,
		siteID,
	)
	var jsonStr string
	if err := row.Scan(&jsonStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	var cfg omnilogic.MspConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// QueryRecent telemetry samples ordered by sampled_at desc.
type TelemetrySample struct {
	SiteMspSystemID int
	BowSystemID     string
	EquipmentID     string
	Metric          string
	ValueInt        sql.NullInt64
	ValueReal       sql.NullFloat64
	ValueText       sql.NullString
	SampledAt       string
}

func (s *Store) QueryTelemetry(siteID int, metric, since string, limit int) ([]TelemetrySample, error) {
	q := `SELECT site_msp_system_id, COALESCE(bow_system_id,''), COALESCE(equipment_system_id,''), metric, value_int, value_real, value_text, sampled_at
	      FROM telemetry_samples WHERE 1=1`
	args := []any{}
	if siteID != 0 {
		q += ` AND site_msp_system_id = ?`
		args = append(args, siteID)
	}
	if metric != "" {
		q += ` AND metric = ?`
		args = append(args, metric)
	}
	if since != "" {
		q += ` AND sampled_at >= ?`
		args = append(args, since)
	}
	q += ` ORDER BY sampled_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TelemetrySample
	for rows.Next() {
		var t TelemetrySample
		if err := rows.Scan(&t.SiteMspSystemID, &t.BowSystemID, &t.EquipmentID, &t.Metric, &t.ValueInt, &t.ValueReal, &t.ValueText, &t.SampledAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// FormatSampleValue renders the active value column as a string. Used by
// chemistry log CSV emission so callers don't have to switch on column.
func (t TelemetrySample) FormatValue() string {
	if t.ValueReal.Valid {
		return strconv.FormatFloat(t.ValueReal.Float64, 'f', -1, 64)
	}
	if t.ValueInt.Valid {
		return strconv.FormatInt(t.ValueInt.Int64, 10)
	}
	if t.ValueText.Valid {
		return t.ValueText.String
	}
	return ""
}

// Compose a SQL fragment for limit_offset application.
func ApplyLimitOffset(b *strings.Builder, limit, offset int) {
	if limit > 0 {
		fmt.Fprintf(b, " LIMIT %d", limit)
	}
	if offset > 0 {
		fmt.Fprintf(b, " OFFSET %d", offset)
	}
}
