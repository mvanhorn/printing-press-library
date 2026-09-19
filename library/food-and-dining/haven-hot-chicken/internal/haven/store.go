package haven

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const schema = `CREATE TABLE IF NOT EXISTS haven_locations (id INTEGER PRIMARY KEY, payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS haven_snapshots (id INTEGER PRIMARY KEY AUTOINCREMENT, location_id INTEGER NOT NULL, fetched_at TEXT NOT NULL, payload TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS haven_snapshots_location ON haven_snapshots(location_id,fetched_at DESC,id DESC);`

func Save(ctx context.Context, db *sql.DB, locations []Location, snapshots []Snapshot) error {
	// Validate the whole refresh before opening one write transaction. No store.Upsert calls inside it.
	for _, s := range snapshots {
		if err := validateSnapshot(s); err != nil {
			return err
		}
		if !s.Complete {
			return fmt.Errorf("refusing incomplete snapshot for location %d", s.LocationID)
		}
	}
	raw, err := json.Marshal(map[string]any{"locations": locations})
	if err != nil {
		return err
	}
	if locations != nil {
		if _, err = ParseLocations(raw); err != nil {
			return err
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	// A nonnil directory is a complete current API list; retain snapshots separately.
	if locations != nil {
		if _, err = tx.ExecContext(ctx, `DELETE FROM haven_locations`); err != nil {
			return err
		}
	}
	for _, l := range locations {
		b, e := json.Marshal(l)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO haven_locations(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, l.ID, string(b)); e != nil {
			return e
		}
	}
	for _, s := range snapshots {
		s.ID = 0
		b, e := json.Marshal(s)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO haven_snapshots(location_id,fetched_at,payload) VALUES(?,?,?)`, s.LocationID, s.FetchedAt.UTC().Format("2006-01-02T15:04:05.000000000Z"), string(b)); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func hasTable(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&count)
	return count > 0, err
}
func LoadLocations(ctx context.Context, db *sql.DB) ([]Location, error) {
	result := []Location{}
	if exists, err := hasTable(ctx, db, "haven_locations"); err != nil || !exists {
		return result, err
	}
	rows, err := db.QueryContext(ctx, `SELECT payload FROM haven_locations ORDER BY id`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var l Location
		if err = rows.Scan(&b); err != nil {
			return result, err
		}
		if err = json.Unmarshal([]byte(b), &l); err != nil {
			return result, err
		}
		result = append(result, l)
	}
	return result, rows.Err()
}
func Latest(ctx context.Context, db *sql.DB, locationID int64, count int) ([]Snapshot, error) {
	result := []Snapshot{}
	if locationID <= 0 || count < 1 || count > 1000 {
		return result, fmt.Errorf("location id must be positive and snapshot count between 1 and 1000")
	}
	if exists, err := hasTable(ctx, db, "haven_snapshots"); err != nil || !exists {
		return result, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id,payload FROM haven_snapshots WHERE location_id=? ORDER BY fetched_at DESC,id DESC LIMIT ?`, locationID, count)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var b string
		var s Snapshot
		if err = rows.Scan(&id, &b); err != nil {
			return result, err
		}
		if err = json.Unmarshal([]byte(b), &s); err != nil {
			return result, err
		}
		s.ID = id
		s.FetchedAt = s.FetchedAt.UTC()
		if err = validateSnapshot(s); err != nil {
			return result, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
