// Package history is the local walk-history store: append-only JSONL files for
// sessions and per-tick samples, with daily-rollup and streak queries. It is
// pure Go (no SQLite, no CGO) so the CLI stays a single static binary. The belt
// only remembers its last run and loses it on power-cut; this store is the
// durable record the device and the official app never keep.
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Session is one recorded walk.
type Session struct {
	ID          string  `json:"id"`
	StartTS     float64 `json:"start_ts"`
	EndTS       float64 `json:"end_ts"`
	DurationS   int     `json:"duration_s"`
	DistanceM   int     `json:"distance_m"`
	Steps       int     `json:"steps"`
	AvgSpeedKmh float64 `json:"avg_speed_kmh"`
	MaxSpeedKmh float64 `json:"max_speed_kmh"`
}

// Sample is one telemetry tick within a session.
type Sample struct {
	SessionID string  `json:"session_id"`
	TS        float64 `json:"ts"`
	SpeedKmh  float64 `json:"speed_kmh"`
	DistanceM int     `json:"distance_m"`
	Steps     int     `json:"steps"`
	BeltState int     `json:"belt_state"`
}

// Totals is an aggregate over a set of sessions.
type Totals struct {
	Date      string `json:"date,omitempty"`
	DistanceM int    `json:"distance_m"`
	Steps     int    `json:"steps"`
	DurationS int    `json:"duration_s"`
	Sessions  int    `json:"sessions"`
}

// Store persists sessions and samples as JSONL under a directory.
type Store struct {
	dir string
}

// DefaultDir returns the per-user history directory.
func DefaultDir(cliName string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(cacheDir, cliName, "history"), nil
}

// Open returns a Store rooted at dir, creating it if needed.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("history dir is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create history dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir returns the store's directory.
func (s *Store) Dir() string { return s.dir }

func (s *Store) sessionsPath() string { return filepath.Join(s.dir, "sessions.jsonl") }
func (s *Store) samplesPath() string  { return filepath.Join(s.dir, "samples.jsonl") }

func appendJSONL(path string, records ...any) error {
	if len(records) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("write %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// AddSession appends a completed session.
func (s *Store) AddSession(session Session) error {
	return appendJSONL(s.sessionsPath(), session)
}

// AddSamples appends telemetry samples.
func (s *Store) AddSamples(samples []Sample) error {
	records := make([]any, len(samples))
	for i := range samples {
		records[i] = samples[i]
	}
	return appendJSONL(s.samplesPath(), records...)
}

// Sessions returns all recorded sessions, oldest first.
func (s *Store) Sessions() ([]Session, error) {
	f, err := os.Open(s.sessionsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open sessions: %w", err)
	}
	defer f.Close()
	var out []Session
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var sess Session
		if err := json.Unmarshal(line, &sess); err != nil {
			return nil, fmt.Errorf("decode session: %w", err)
		}
		out = append(out, sess)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartTS < out[j].StartTS })
	return out, nil
}

func dayKey(ts float64, loc *time.Location) string {
	return time.Unix(int64(ts), 0).In(loc).Format("2006-01-02")
}

// SessionsOn returns the sessions whose start falls on date (YYYY-MM-DD, local).
func (s *Store) SessionsOn(date string, loc *time.Location) ([]Session, error) {
	all, err := s.Sessions()
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, sess := range all {
		if dayKey(sess.StartTS, loc) == date {
			out = append(out, sess)
		}
	}
	return out, nil
}

// TotalsOn returns aggregate totals for a single local date.
func (s *Store) TotalsOn(date string, loc *time.Location) (Totals, error) {
	sessions, err := s.SessionsOn(date, loc)
	if err != nil {
		return Totals{}, err
	}
	t := Totals{Date: date}
	for _, sess := range sessions {
		t.DistanceM += sess.DistanceM
		t.Steps += sess.Steps
		t.DurationS += sess.DurationS
		t.Sessions++
	}
	return t, nil
}

// DailySeries returns per-day totals for the last n days (most recent last),
// including days with zero activity so the series has no gaps.
func (s *Store) DailySeries(days int, now time.Time) ([]Totals, error) {
	if days < 1 {
		days = 1
	}
	loc := now.Location()
	all, err := s.Sessions()
	if err != nil {
		return nil, err
	}
	byDay := map[string]*Totals{}
	for _, sess := range all {
		k := dayKey(sess.StartTS, loc)
		t := byDay[k]
		if t == nil {
			t = &Totals{Date: k}
			byDay[k] = t
		}
		t.DistanceM += sess.DistanceM
		t.Steps += sess.Steps
		t.DurationS += sess.DurationS
		t.Sessions++
	}
	out := make([]Totals, 0, days)
	start := now.AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		k := start.AddDate(0, 0, i).Format("2006-01-02")
		if t, ok := byDay[k]; ok {
			out = append(out, *t)
		} else {
			out = append(out, Totals{Date: k})
		}
	}
	return out, nil
}

// Streak returns the count of consecutive days up to today (or yesterday, if
// today has no walk yet) that have at least one walk with distance > 0.
func (s *Store) Streak(now time.Time) (int, error) {
	loc := now.Location()
	all, err := s.Sessions()
	if err != nil {
		return 0, err
	}
	active := map[string]bool{}
	for _, sess := range all {
		if sess.DistanceM > 0 {
			active[dayKey(sess.StartTS, loc)] = true
		}
	}
	day := now
	if !active[day.Format("2006-01-02")] {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for active[day.Format("2006-01-02")] {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak, nil
}
