package parks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func Save(ctx context.Context, path string, records []Park) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	db, e := sql.Open("sqlite", cacheURI(ctx, path, false))
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS park_observations (id TEXT PRIMARY KEY, source_level TEXT NOT NULL, observed_at TEXT NOT NULL, data TEXT NOT NULL)`); e != nil {
		return e
	}
	for _, p := range records {
		raw, e := json.Marshal(p)
		if e != nil {
			return e
		}
		_, e = db.ExecContext(ctx, `INSERT INTO park_observations(id,source_level,observed_at,data) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET source_level=excluded.source_level,observed_at=excluded.observed_at,data=excluded.data WHERE excluded.source_level='detail' OR park_observations.source_level<>'detail'`, p.ID, p.SourceLevel, p.ObservedAt, string(raw))
		if e != nil {
			return fmt.Errorf("cache park %s: %w", p.ID, e)
		}
	}
	return nil
}
func Load(ctx context.Context, path string) ([]Park, error) {
	out := []Park{}
	if _, e := os.Stat(path); os.IsNotExist(e) {
		return out, nil
	} else if e != nil {
		return nil, e
	}
	u := cacheURI(ctx, path, true)
	db, e := sql.Open("sqlite", u)
	if e != nil {
		return nil, e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var table string
	if e = db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='park_observations'`).Scan(&table); e == sql.ErrNoRows {
		return out, nil
	} else if e != nil {
		return nil, e
	}
	rows, e := db.QueryContext(ctx, `SELECT data FROM park_observations ORDER BY id LIMIT 10001`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		var p Park
		if e = json.Unmarshal([]byte(raw), &p); e != nil {
			return nil, fmt.Errorf("corrupt cached park record: %w", e)
		}
		normalizeLists(&p)
		out = append(out, p)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if len(out) > 10000 {
		return nil, fmt.Errorf("cache contains more than 10000 parks; reduce the cache before reading")
	}
	return out, nil
}

// Parallel read-only planning commands may refresh the same observation store.
// Wait briefly for another SQLite writer; the caller's deadline bounds the wait.
func cacheURI(ctx context.Context, path string, readOnly bool) string {
	busyMS := int64(5000)
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline).Milliseconds(); remaining < busyMS {
			busyMS = remaining
		}
	}
	if busyMS < 1 {
		busyMS = 1
	}
	q := url.Values{"_pragma": []string{fmt.Sprintf("busy_timeout(%d)", busyMS)}}
	if readOnly {
		q.Set("mode", "ro")
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: q.Encode()}
	return u.String()
}

func normalizeLists(p *Park) {
	if p.Vehicles == nil {
		p.Vehicles = []string{}
	}
	if p.AvailabilityPeriods == nil {
		p.AvailabilityPeriods = []string{}
	}
	if p.Tariffs == nil {
		p.Tariffs = []Tariff{}
	}
	if p.Booking.URLs == nil {
		p.Booking.URLs = []string{}
	}
	if p.Booking.Phones == nil {
		p.Booking.Phones = []string{}
	}
	if p.Booking.Emails == nil {
		p.Booking.Emails = []string{}
	}
	if p.Icons == nil {
		p.Icons = []Icon{}
	}
	if p.Warnings == nil {
		p.Warnings = []string{}
	}
	if p.Sections == nil {
		p.Sections = map[string]string{}
	}
	if p.Facilities == nil {
		p.Facilities = map[string]Facility{}
	}
	for _, key := range FacilityKeys {
		f, ok := p.Facilities[key]
		if !ok {
			f.Status = "unknown"
			f.Fee = "unknown"
		}
		if f.Evidence == nil {
			f.Evidence = []string{}
		}
		p.Facilities[key] = f
	}
}
