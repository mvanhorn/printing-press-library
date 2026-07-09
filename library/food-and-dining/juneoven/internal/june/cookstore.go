package june

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/juneoven/internal/cliutil"

	_ "modernc.org/sqlite"
)

// CookStore is the local SQLite record of cooks and their telemetry — the data
// June's live-only cloud never keeps. Schema lives here (not in the generated
// store) so it survives regeneration.
type CookStore struct {
	db *sql.DB
}

// CookDBPath returns the on-disk path of the cook database.
func CookDBPath() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cooks.db"), nil
}

// OpenCookStore opens (creating if needed) the cook database and applies migrations.
func OpenCookStore(ctx context.Context) (*CookStore, error) {
	path, err := CookDBPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	cs := &CookStore{db: db}
	if err := cs.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return cs, nil
}

func (cs *CookStore) Close() error { return cs.db.Close() }

func (cs *CookStore) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT,
			mode TEXT,
			target_f INTEGER,
			started_at INTEGER NOT NULL,
			ended_at INTEGER,
			outcome TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS samples (
			session_id INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			cavity_f INTEGER,
			progress INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_session ON samples(session_id)`,
		`CREATE TABLE IF NOT EXISTS presets (
			name TEXT PRIMARY KEY,
			mode TEXT NOT NULL,
			target_f INTEGER NOT NULL,
			timer_min INTEGER
		)`,
	}
	for _, s := range stmts {
		if _, err := cs.db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migrating cook store: %w", err)
		}
	}
	return nil
}

// Session is one recorded cook.
type Session struct {
	ID          int64   `json:"id"`
	Label       string  `json:"label,omitempty"`
	Mode        string  `json:"mode"`
	TargetF     int     `json:"target_f"`
	StartedAt   string  `json:"started_at"`
	EndedAt     string  `json:"ended_at,omitempty"`
	Outcome     string  `json:"outcome,omitempty"`
	DurationMin float64 `json:"duration_min,omitempty"`
}

// Sample is one telemetry point in a session.
type Sample struct {
	TS       string `json:"ts"`
	CavityF  int    `json:"cavity_f"`
	Progress int    `json:"progress"`
}

// StartSession inserts a new open session and returns its id.
func (cs *CookStore) StartSession(ctx context.Context, label, mode string, targetF int, at time.Time) (int64, error) {
	res, err := cs.db.ExecContext(ctx,
		`INSERT INTO sessions (label, mode, target_f, started_at) VALUES (?,?,?,?)`,
		label, mode, targetF, at.Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AppendSample records one telemetry point.
func (cs *CookStore) AppendSample(ctx context.Context, sessionID int64, at time.Time, cavityF, progress int) error {
	_, err := cs.db.ExecContext(ctx,
		`INSERT INTO samples (session_id, ts, cavity_f, progress) VALUES (?,?,?,?)`,
		sessionID, at.Unix(), cavityF, progress)
	return err
}

// EndSession closes a session with an outcome.
func (cs *CookStore) EndSession(ctx context.Context, sessionID int64, outcome string, at time.Time) error {
	_, err := cs.db.ExecContext(ctx,
		`UPDATE sessions SET ended_at=?, outcome=? WHERE id=?`,
		at.Unix(), outcome, sessionID)
	return err
}

func fmtTS(unix sql.NullInt64) string {
	if !unix.Valid {
		return ""
	}
	return time.Unix(unix.Int64, 0).Format(time.RFC3339)
}

// ListSessions returns recent sessions, newest first.
func (cs *CookStore) ListSessions(ctx context.Context, limit int, since time.Duration) ([]Session, error) {
	q := `SELECT id, label, mode, target_f, started_at, ended_at, outcome FROM sessions`
	var args []any
	if since > 0 {
		q += ` WHERE started_at >= ?`
		args = append(args, time.Now().Add(-since).Unix())
	}
	q += ` ORDER BY started_at DESC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := cs.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Session, 0)
	for rows.Next() {
		var s Session
		var label, mode, outcome sql.NullString
		var started sql.NullInt64
		var ended sql.NullInt64
		var target sql.NullInt64
		if err := rows.Scan(&s.ID, &label, &mode, &target, &started, &ended, &outcome); err != nil {
			continue
		}
		s.Label, s.Mode, s.Outcome = label.String, mode.String, outcome.String
		s.TargetF = int(target.Int64)
		s.StartedAt, s.EndedAt = fmtTS(started), fmtTS(ended)
		if started.Valid && ended.Valid {
			s.DurationMin = time.Unix(ended.Int64, 0).Sub(time.Unix(started.Int64, 0)).Minutes()
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SessionSamples returns the telemetry samples for one session in time order.
func (cs *CookStore) SessionSamples(ctx context.Context, sessionID int64) ([]Sample, error) {
	rows, err := cs.db.QueryContext(ctx,
		`SELECT ts, cavity_f, progress FROM samples WHERE session_id=? ORDER BY ts ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Sample, 0)
	for rows.Next() {
		var s Sample
		var ts, cavity, prog sql.NullInt64
		if err := rows.Scan(&ts, &cavity, &prog); err != nil {
			continue
		}
		s.TS = fmtTS(ts)
		s.CavityF, s.Progress = int(cavity.Int64), int(prog.Int64)
		out = append(out, s)
	}
	return out, rows.Err()
}

// PreheatStat is derived time-to-target statistics for a mode.
type PreheatStat struct {
	Mode          string  `json:"mode"`
	Cooks         int     `json:"cooks_analyzed"`
	MedianSeconds float64 `json:"median_seconds_to_target"`
	FastestSec    float64 `json:"fastest_seconds"`
	SlowestSec    float64 `json:"slowest_seconds"`
}

// PreheatStats computes per-mode median/min/max time from a session's first
// sample to the first sample that reaches the target, over completed sessions.
// mode == "" analyzes all modes together per mode grouping.
func (cs *CookStore) PreheatStats(ctx context.Context, modeFilter string) ([]PreheatStat, error) {
	sessions, err := cs.ListSessions(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	byMode := map[string][]float64{}
	for _, s := range sessions {
		if modeFilter != "" && s.Mode != modeFilter {
			continue
		}
		if s.TargetF == 0 {
			continue
		}
		samples, err := cs.SessionSamples(ctx, s.ID)
		if err != nil || len(samples) < 2 {
			continue
		}
		start, err := time.Parse(time.RFC3339, samples[0].TS)
		if err != nil {
			continue
		}
		for _, sm := range samples {
			if sm.CavityF >= s.TargetF {
				reached, err := time.Parse(time.RFC3339, sm.TS)
				if err != nil {
					break
				}
				byMode[s.Mode] = append(byMode[s.Mode], reached.Sub(start).Seconds())
				break
			}
		}
	}
	out := make([]PreheatStat, 0, len(byMode))
	for mode, secs := range byMode {
		if len(secs) == 0 {
			continue
		}
		sort.Float64s(secs)
		out = append(out, PreheatStat{
			Mode:          mode,
			Cooks:         len(secs),
			MedianSeconds: median(secs),
			FastestSec:    secs[0],
			SlowestSec:    secs[len(secs)-1],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mode < out[j].Mode })
	return out, nil
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// Preset is a saved named cook.
type Preset struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"`
	TargetF  int    `json:"target_f"`
	TimerMin int    `json:"timer_min,omitempty"`
}

// SavePreset upserts a named preset.
func (cs *CookStore) SavePreset(ctx context.Context, p Preset) error {
	_, err := cs.db.ExecContext(ctx,
		`INSERT INTO presets (name, mode, target_f, timer_min) VALUES (?,?,?,?)
		 ON CONFLICT(name) DO UPDATE SET mode=excluded.mode, target_f=excluded.target_f, timer_min=excluded.timer_min`,
		p.Name, p.Mode, p.TargetF, p.TimerMin)
	return err
}

// GetPreset fetches a named preset.
func (cs *CookStore) GetPreset(ctx context.Context, name string) (Preset, bool, error) {
	var p Preset
	var timer sql.NullInt64
	err := cs.db.QueryRowContext(ctx,
		`SELECT name, mode, target_f, timer_min FROM presets WHERE name=?`, name).
		Scan(&p.Name, &p.Mode, &p.TargetF, &timer)
	if err == sql.ErrNoRows {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	p.TimerMin = int(timer.Int64)
	return p, true, nil
}

// ListPresets returns all saved presets by name.
func (cs *CookStore) ListPresets(ctx context.Context) ([]Preset, error) {
	rows, err := cs.db.QueryContext(ctx, `SELECT name, mode, target_f, timer_min FROM presets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Preset, 0)
	for rows.Next() {
		var p Preset
		var timer sql.NullInt64
		if err := rows.Scan(&p.Name, &p.Mode, &p.TargetF, &timer); err != nil {
			continue
		}
		p.TimerMin = int(timer.Int64)
		out = append(out, p)
	}
	return out, rows.Err()
}
