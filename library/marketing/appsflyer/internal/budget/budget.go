// Package budget tracks AppsFlyer Pull API daily-call usage so the CLI can
// warn users before they exhaust their plan's daily cap. Persists to a small
// JSON file under the user config directory so the count survives across
// invocations within the same day.
package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultDailyLimit is the conservative default budget. Real users override
// this in config.yaml under `calls_per_day` to match their AppsFlyer
// subscription tier.
const DefaultDailyLimit = 20

// State is the on-disk representation of today's budget usage.
type State struct {
	Date  string `json:"date"`  // YYYY-MM-DD in UTC
	Used  int    `json:"used"`  // calls consumed today
	Limit int    `json:"limit"` // limit at the time of last write (informational)
}

// Tracker is a thread-safe daily-budget counter persisted to disk.
type Tracker struct {
	mu    sync.Mutex
	path  string
	limit int
	state State
}

// New returns a Tracker that loads existing state on first use. When path is
// empty, the tracker resolves the default location under the user's config
// dir; tests can pass an explicit path. limit must be > 0 or it falls back
// to DefaultDailyLimit.
func New(path string, limit int) (*Tracker, error) {
	if limit <= 0 {
		limit = DefaultDailyLimit
	}
	if path == "" {
		dir, err := defaultDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "budget.json")
	}
	t := &Tracker{path: path, limit: limit}
	if err := t.load(); err != nil {
		return nil, err
	}
	return t, nil
}

func defaultDir() (string, error) {
	if v := os.Getenv("APPSFLYER_CONFIG_DIR"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "appsflyer-pp-cli"), nil
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func (t *Tracker) load() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state = State{Date: todayUTC(), Used: 0, Limit: t.limit}
	data, err := os.ReadFile(t.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading budget state: %w", err)
	}
	var disk State
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil
	}
	if disk.Date == todayUTC() {
		t.state = disk
		t.state.Limit = t.limit
	}
	return nil
}

func (t *Tracker) flush() error {
	if t.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.path)
}

// Snapshot returns the current state without consuming a call.
func (t *Tracker) Snapshot() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state.Date != todayUTC() {
		t.state = State{Date: todayUTC(), Used: 0, Limit: t.limit}
	}
	return t.state
}

// Limit returns the configured daily limit.
func (t *Tracker) Limit() int { return t.limit }

// Remaining returns the number of calls still available today.
func (t *Tracker) Remaining() int {
	s := t.Snapshot()
	r := t.limit - s.Used
	if r < 0 {
		return 0
	}
	return r
}

// Charge records n consumed API calls. Returns the new state after the charge.
func (t *Tracker) Charge(n int) State {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.state.Date != todayUTC() {
		t.state = State{Date: todayUTC(), Used: 0, Limit: t.limit}
	}
	t.state.Used += n
	t.state.Limit = t.limit
	_ = t.flush()
	return t.state
}

// Path returns the on-disk location of the budget file (or empty when the
// tracker is memory-only).
func (t *Tracker) Path() string { return t.path }
