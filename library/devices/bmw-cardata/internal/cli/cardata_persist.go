// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"

	"github.com/spf13/cobra"
)

// resolveDBPath returns the explicit --db path when set, otherwise the
// default on-disk location for this CLI.
func resolveDBPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return defaultDBPath("bmw-cardata-pp-cli")
}

// openCardataStore opens the local store read-only-style, returning nil (no
// error) when the database does not exist yet. Used by transcendence read
// commands which treat a missing mirror as empty local state.
func openCardataStore(dbPath string) (*store.Store, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	return store.OpenWithContext(context.Background(), dbPath)
}

// openCardataStoreRW opens (creating if needed) the local store. Used by
// write-through and archive import paths so the first fetch/import populates
// the store rather than being silently dropped.
func openCardataStoreRW(dbPath string) (*store.Store, error) {
	return store.OpenWithContext(context.Background(), dbPath)
}

// recordCardataAPICall increments today's (UTC) API-call counter used by the
// quota tracker. It is best-effort: a missing or unwritable store must never
// fail a successful live fetch.
func recordCardataAPICall(dbPath string) {
	db, err := openCardataStoreRW(dbPath)
	if err != nil || db == nil {
		return
	}
	defer db.Close()
	day := time.Now().UTC().Format("2006-01-02")
	if _, err := db.DB().Exec(
		`INSERT INTO cardata_api_calls(day, count) VALUES(?,1)
		 ON CONFLICT(day) DO UPDATE SET count = count + 1`, day,
	); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not record api call: %v\n", err)
	}
}

// persistCardataBasicData upserts a vehicle's basic data into cardata_vehicles.
func persistCardataBasicData(dbPath, vin string, raw json.RawMessage) {
	db, err := openCardataStoreRW(dbPath)
	if err != nil || db == nil {
		return
	}
	defer db.Close()
	var v struct {
		Brand                string `json:"brand"`
		ModelRange           string `json:"modelRange"`
		Series               string `json:"series"`
		ModelName            string `json:"modelName"`
		PropulsionType       string `json:"propulsionType"`
		DriveTrain           string `json:"driveTrain"`
		Engine               string `json:"engine"`
		HvsMaxEnergyAbsolute string `json:"hvsMaxEnergyAbsolute"`
		ConstructionDate     string `json:"constructionDate"`
		SimStatus            string `json:"simStatus"`
		IsTelematicsCapable  bool   `json:"isTelematicsCapable"`
	}
	_ = json.Unmarshal(raw, &v)
	tc := 0
	if v.IsTelematicsCapable {
		tc = 1
	}
	if _, err := db.DB().Exec(
		`INSERT INTO cardata_vehicles(vin, brand, model_range, series, model_name,
		     propulsion_type, drive_train, engine, hvs_max_energy_absolute,
		     construction_date, sim_status, is_telematics_capable, raw)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(vin) DO UPDATE SET brand=excluded.brand, model_range=excluded.model_range,
		   series=excluded.series, model_name=excluded.model_name, propulsion_type=excluded.propulsion_type,
		   drive_train=excluded.drive_train, engine=excluded.engine, hvs_max_energy_absolute=excluded.hvs_max_energy_absolute,
		   construction_date=excluded.construction_date, sim_status=excluded.sim_status,
		   is_telematics_capable=excluded.is_telematics_capable, raw=excluded.raw`,
		vin, v.Brand, v.ModelRange, v.Series, v.ModelName, v.PropulsionType, v.DriveTrain,
		v.Engine, v.HvsMaxEnergyAbsolute, v.ConstructionDate, v.SimStatus, tc, string(raw),
	); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not persist basic data: %v\n", err)
	}
}

// persistCardataTelematicData appends every descriptor in a telematic-data
// response to the append-only time-series table. The response shape is
// {"telematicData": {<descriptor>: {"value","unit","timestamp"}}}.
func persistCardataTelematicData(dbPath, vin string, raw json.RawMessage) {
	db, err := openCardataStoreRW(dbPath)
	if err != nil || db == nil {
		return
	}
	defer db.Close()
	var outer struct {
		TelematicData map[string]struct {
			Value     string `json:"value"`
			Unit      string `json:"unit"`
			Timestamp string `json:"timestamp"`
		} `json:"telematicData"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil || outer.TelematicData == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(context.Background(), nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT OR IGNORE INTO cardata_telematic_snapshots(vin, descriptor, value, unit, ts, fetched_at)
		 VALUES(?,?,?,?,?,?)`)
	if err != nil {
		return
	}
	defer stmt.Close()
	for desc, e := range outer.TelematicData {
		var timestamp any
		if strings.TrimSpace(e.Timestamp) != "" {
			timestamp = e.Timestamp
		}
		if _, err := stmt.ExecContext(context.Background(), vin, desc, e.Value, e.Unit, timestamp, now); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

// persistCardataChargingHistory upserts charging sessions keyed by (vin, start_time).
func persistCardataChargingHistory(dbPath, vin string, raw json.RawMessage) {
	db, err := openCardataStoreRW(dbPath)
	if err != nil || db == nil {
		return
	}
	defer db.Close()
	var resp struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.DB().BeginTx(context.Background(), nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO cardata_charging_sessions(vin, start_time, data, fetched_at)
		 VALUES(?,?,?,?)
		 ON CONFLICT(vin, start_time) DO UPDATE SET data=excluded.data, fetched_at=excluded.fetched_at`)
	if err != nil {
		return
	}
	defer stmt.Close()
	for _, s := range resp.Data {
		var sess struct {
			StartTime json.Number `json:"startTime"`
		}
		start := int64(0)
		if json.Unmarshal(s, &sess) == nil {
			if n, err := sess.StartTime.Int64(); err == nil {
				start = n
			}
		}
		if _, err := stmt.ExecContext(context.Background(), vin, start, string(s), now); err != nil {
			return
		}
	}
	_ = tx.Commit()
}

// persistCardataMappings records each mapped VIN as a minimal vehicle row so
// fleet status works even before basic data has been fetched.
func persistCardataMappings(dbPath string, raw json.RawMessage) {
	db, err := openCardataStoreRW(dbPath)
	if err != nil || db == nil {
		return
	}
	defer db.Close()
	var mappings []struct {
		VIN         string `json:"vin"`
		MappingType string `json:"mappingType"`
	}
	if err := json.Unmarshal(raw, &mappings); err != nil {
		return
	}
	for _, m := range mappings {
		if m.VIN == "" {
			continue
		}
		if _, err := db.DB().Exec(
			`INSERT INTO cardata_vehicles(vin) VALUES(?) ON CONFLICT(vin) DO NOTHING`, m.VIN,
		); err != nil {
			return
		}
	}
}

// ---- readers used by transcendence commands ----

type cardataVehicle struct {
	VIN                 string `json:"vin"`
	Brand               string `json:"brand"`
	ModelRange          string `json:"model_range"`
	Series              string `json:"series"`
	ModelName           string `json:"model_name"`
	PropulsionType      string `json:"propulsion_type"`
	HvsMaxEnergyAbs     string `json:"hvs_max_energy_absolute"`
	ConstructionDate    string `json:"construction_date"`
	SimStatus           string `json:"sim_status"`
	IsTelematicsCapable bool   `json:"is_telematics_capable"`
}

func listCardataVehicles(db *store.Store) ([]cardataVehicle, error) {
	rows, err := db.DB().Query(
		`SELECT vin, COALESCE(brand,''), COALESCE(model_range,''), COALESCE(series,''),
		        COALESCE(model_name,''), COALESCE(propulsion_type,''),
		        COALESCE(hvs_max_energy_absolute,''), COALESCE(construction_date,''),
		        COALESCE(sim_status,''), COALESCE(is_telematics_capable,0)
		 FROM cardata_vehicles ORDER BY vin`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]cardataVehicle, 0)
	for rows.Next() {
		var v cardataVehicle
		var tc int
		if err := rows.Scan(&v.VIN, &v.Brand, &v.ModelRange, &v.Series, &v.ModelName,
			&v.PropulsionType, &v.HvsMaxEnergyAbs, &v.ConstructionDate, &v.SimStatus, &tc); err != nil {
			return nil, err
		}
		v.IsTelematicsCapable = tc != 0
		out = append(out, v)
	}
	return out, rows.Err()
}

type telematicPoint struct {
	Descriptor string `json:"descriptor"`
	Value      string `json:"value"`
	Unit       string `json:"unit"`
	Timestamp  string `json:"timestamp"`
}

// latestCardataSnapshot returns the most recent value+unit+ts for every
// descriptor of a VIN.
func latestCardataSnapshot(db *store.Store, vin string) ([]telematicPoint, error) {
	rows, err := db.DB().Query(
		`SELECT descriptor, value, COALESCE(unit,''), COALESCE(ts,'')
		 FROM cardata_telematic_snapshots s
		 WHERE vin = ? AND id = (
		     SELECT MAX(id) FROM cardata_telematic_snapshots
		     WHERE vin = s.vin AND descriptor = s.descriptor)
		 ORDER BY descriptor`, vin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]telematicPoint, 0)
	for rows.Next() {
		var p telematicPoint
		if err := rows.Scan(&p.Descriptor, &p.Value, &p.Unit, &p.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// snapshotValue returns the latest point for one descriptor of a VIN.
func snapshotValue(db *store.Store, vin, descriptor string) (telematicPoint, bool, error) {
	var p telematicPoint
	err := db.DB().QueryRow(
		`SELECT descriptor, value, COALESCE(unit,''), COALESCE(ts,'')
		 FROM cardata_telematic_snapshots
		 WHERE vin = ? AND descriptor = ?
		 ORDER BY id DESC LIMIT 1`, vin, descriptor,
	).Scan(&p.Descriptor, &p.Value, &p.Unit, &p.Timestamp)
	if err == sql.ErrNoRows {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// snapshotAsOf returns the most recent value+unit+ts for every descriptor of
// a VIN recorded at or before cutoff (groupwise max id per descriptor).
func snapshotAsOf(db *store.Store, vin string, cutoff time.Time) ([]telematicPoint, error) {
	rows, err := db.DB().Query(
		`SELECT descriptor, value, COALESCE(unit,''), COALESCE(ts,'')
		 FROM cardata_telematic_snapshots s
		 WHERE vin = ? AND fetched_at <= ? AND id = (
		     SELECT MAX(id) FROM cardata_telematic_snapshots
		     WHERE vin = s.vin AND descriptor = s.descriptor AND fetched_at <= ?)
		 ORDER BY descriptor`, vin, cutoff.Format(time.RFC3339), cutoff.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]telematicPoint, 0)
	for rows.Next() {
		var p telematicPoint
		if err := rows.Scan(&p.Descriptor, &p.Value, &p.Unit, &p.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// cardataSnapshotSeries returns time-ordered points for one descriptor.
func cardataSnapshotSeries(db *store.Store, vin, descriptor string, since time.Time) ([]telematicPoint, error) {
	rows, err := db.DB().Query(
		`SELECT descriptor, value, COALESCE(unit,''), COALESCE(ts,'')
		 FROM cardata_telematic_snapshots
		 WHERE vin = ? AND descriptor = ? AND fetched_at >= ?
		 ORDER BY id ASC`, vin, descriptor, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]telematicPoint, 0)
	for rows.Next() {
		var p telematicPoint
		if err := rows.Scan(&p.Descriptor, &p.Value, &p.Unit, &p.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type chargingSession struct {
	StartTime                int64   `json:"start_time"`
	EndTime                  int64   `json:"end_time"`
	EnergyFromGridKwh        float64 `json:"energy_consumed_from_power_grid_kwh"`
	TotalChargingDurationSec int64   `json:"total_charging_duration_sec"`
	DisplayedSoc             int     `json:"displayed_soc"`
	DisplayedStartSoc        int     `json:"displayed_start_soc"`
	IsPreconditioning        bool    `json:"is_preconditioning_activated"`
	Raw                      string  `json:"-"`
}

func listCardataChargingSessions(db *store.Store, vin string, since time.Time) ([]chargingSession, error) {
	rows, err := db.DB().Query(
		`SELECT start_time, data FROM cardata_charging_sessions
		 WHERE vin = ? AND datetime(fetched_at) >= datetime(?)
		 ORDER BY start_time ASC`, vin, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]chargingSession, 0)
	for rows.Next() {
		var start int64
		var data string
		if err := rows.Scan(&start, &data); err != nil {
			return nil, err
		}
		var s chargingSession
		s.StartTime = start
		_ = json.Unmarshal([]byte(data), &s)
		s.Raw = data
		out = append(out, s)
	}
	return out, rows.Err()
}

// cardataQuota returns today's recorded API-call count (UTC day).
func cardataQuota(db *store.Store) (day string, count int, err error) {
	day = time.Now().UTC().Format("2006-01-02")
	err = db.DB().QueryRow(`SELECT COALESCE(count,0) FROM cardata_api_calls WHERE day = ?`, day).Scan(&count)
	if err == sql.ErrNoRows {
		return day, 0, nil
	}
	return day, count, err
}

// descriptorsByPrefix returns catalogue entries whose descriptor path contains
// the (lowercased) query, ordered by relevance.
func descriptorsByPrefix(db *store.Store, query string, limit int) ([]map[string]string, error) {
	q := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := db.DB().Query(
		`SELECT descriptor, COALESCE(unit,''), COALESCE(domain,''), COALESCE(description,'')
		 FROM cardata_descriptor_catalogue
		 WHERE LOWER(descriptor) LIKE ? OR LOWER(description) LIKE ? OR LOWER(domain) LIKE ?
		 ORDER BY (descriptor = ?) DESC, descriptor
		 LIMIT ?`, q, q, q, strings.ToLower(strings.TrimSpace(query)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]string, 0)
	for rows.Next() {
		var d, u, dom, desc string
		if err := rows.Scan(&d, &u, &dom, &desc); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"path": d, "unit": u, "domain": dom, "description": desc})
	}
	return out, rows.Err()
}

// sortPointsByTimestamp orders points by their VSS timestamp ascending.
func sortPointsByTimestamp(pts []telematicPoint) {
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Timestamp < pts[j].Timestamp })
}

func nowUTCDay() string { return time.Now().UTC().Format("2006-01-02") }

// nextUTCMidnight returns the next midnight in UTC.
func nextUTCMidnight() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}

// wantsMachine reports whether output should be JSON for agents/scripts.
func wantsMachine(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())
}

// humanWindow returns the window string or a default.
func humanWindow(w string) string {
	if w == "" {
		return "7d"
	}
	return w
}

// sinceFromWindow parses a duration like "7d"/"30d"/"24h" and returns the
// cutoff time. Defaults to 7 days. Uses cliutil.ParseDurationLoose for d/w suffixes.
func sinceFromWindow(w string) time.Time {
	w = humanWindow(w)
	d, err := cliutil.ParseDurationLoose(w)
	if err != nil || d <= 0 {
		d = 7 * 24 * time.Hour
	}
	return time.Now().UTC().Add(-d)
}
