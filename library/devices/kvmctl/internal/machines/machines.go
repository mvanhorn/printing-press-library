// PATCH(library): machine inventory, locked target selection, and verification parity.
package machines

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Target struct {
	Name           string   `json:"name" toml:"name"`
	Port           int      `json:"port" toml:"port"`
	Description    string   `json:"description,omitempty" toml:"description,omitempty"`
	Enabled        bool     `json:"enabled" toml:"enabled"`
	OCRPatterns    []string `json:"ocr_patterns,omitempty" toml:"ocr_patterns,omitempty"`
	PromptPatterns []string `json:"prompt_patterns,omitempty" toml:"prompt_patterns,omitempty"`
}
type Inventory struct {
	Targets []Target `json:"targets" toml:"targets"`
}

func DefaultInventory() Inventory {
	return Inventory{Targets: []Target{{"pve1", 1, "Proxmox pve1", true, []string{"pve1"}, []string{`pve1\s+login:`}}, {"pve2", 2, "Proxmox pve2", true, []string{"pve2"}, []string{`pve2\s+login:`}}, {"kodi-build", 3, "Kodi build box, M1 Mac mini", true, []string{"macos", "mac mini", "kodi"}, []string{`keyboard setup assistant`}}, {"pve3", 4, "Proxmox pve3", true, []string{"pve3"}, []string{`pve3\s+login:`}}}}
}
func (i Inventory) Validate() error {
	seen := map[string]bool{}
	for _, t := range i.Targets {
		if strings.TrimSpace(t.Name) == "" || seen[t.Name] {
			return fmt.Errorf("invalid or duplicate target name %q", t.Name)
		}
		seen[t.Name] = true
		if t.Port < 1 || t.Port > 4 {
			return fmt.Errorf("target %q port must be 1..4", t.Name)
		}
		for _, p := range t.PromptPatterns {
			if _, e := regexp.Compile(p); e != nil {
				return fmt.Errorf("target %q prompt pattern: %w", t.Name, e)
			}
		}
	}
	return nil
}
func (i Inventory) Resolve(name string) (Target, error) {
	if err := i.Validate(); err != nil {
		return Target{}, err
	}
	for _, t := range i.Targets {
		if t.Name == name {
			if !t.Enabled {
				return Target{}, fmt.Errorf("target %q is disabled", name)
			}
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unknown target %q", name)
}

type SelectionState string

const (
	Unknown            SelectionState = "unknown"
	SelectedUnverified SelectionState = "selected_unverified"
	Verified           SelectionState = "verified"
	VerifyFailed       SelectionState = "verify_failed"
)

type SelectionRecord struct {
	Target Target         `json:"target"`
	State  SelectionState `json:"state"`
	Detail string         `json:"detail,omitempty"`
	At     time.Time      `json:"at"`
}
type SelectOptions struct {
	Rearm          bool
	Settle         time.Duration
	Timeout        time.Duration
	Sleep          func(context.Context, time.Duration) error
	VerifyPolicy   *VerifyPolicy
	VerifyAttempts int
	VerifyDelay    time.Duration
	Hold           time.Duration
	Gap            time.Duration
}

const (
	HoldMS = 120 * time.Millisecond
	GapMS  = 150 * time.Millisecond
)

type Locker interface {
	Acquire(context.Context) error
	TryAcquire() error
	Release() error
}

var ErrLocked = errors.New("device is locked")

type fileLock struct {
	path string
	f    *os.File
}

func NewDeviceLock(path string) (Locker, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("lock path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0700)
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("unsafe lock directory")
	}
	return &fileLock{path: path}, nil
}
func (l *fileLock) TryAcquire() error {
	f, e := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		if os.IsExist(e) {
			return ErrLocked
		}
		return e
	}
	l.f = f
	return nil
}
func (l *fileLock) Acquire(ctx context.Context) error {
	for {
		e := l.TryAcquire()
		if e == nil {
			return nil
		}
		if !errors.Is(e, ErrLocked) {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
func (l *fileLock) Release() error {
	if l.f == nil {
		return nil
	}
	e := l.f.Close()
	rm := os.Remove(l.path)
	l.f = nil
	if e != nil {
		return e
	}
	return rm
}

// Selector requires the caller to provide transport operations; no hardware is touched by construction.
type Selector struct {
	Inventory   Inventory
	DeviceID    string
	LockFactory func(string) (Locker, error)
	SendKey     func(context.Context, string, bool) error
	Rearm       func(context.Context) error
	Verify      func(context.Context, Target) error
}

func (s Selector) Select(parent context.Context, name string, opts SelectOptions) (SelectionRecord, error) {
	t, e := s.Inventory.Resolve(name)
	rec := SelectionRecord{Target: t, State: Unknown, At: time.Now()}
	if e != nil {
		return rec, e
	}
	if s.SendKey == nil || s.Verify == nil {
		return rec, fmt.Errorf("selector requires key sender and verifier")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()
	if s.LockFactory == nil {
		return rec, fmt.Errorf("selector requires lock factory")
	}
	lock, e := s.LockFactory(s.DeviceID)
	if e != nil {
		return rec, e
	}
	if e = lock.Acquire(ctx); e != nil {
		return rec, e
	}
	defer lock.Release()
	if opts.Rearm && s.Rearm != nil {
		if e = s.Rearm(ctx); e != nil {
			return rec, fmt.Errorf("rearm failed: %w", e)
		}
	}
	rec.State = SelectedUnverified
	hold := opts.Hold
	if hold <= 0 {
		hold = HoldMS
	}
	gap := opts.Gap
	if gap <= 0 {
		gap = GapMS
	}
	sleep := opts.Sleep
	for _, k := range []string{"ControlRight", "ControlRight", fmt.Sprintf("Digit%d", t.Port), "Enter"} {
		if e = s.SendKey(ctx, k, true); e != nil {
			return rec, fmt.Errorf("selection failed: %w", e)
		}
		if err := sleepWithContext(ctx, hold, sleep); err != nil {
			_ = s.SendKey(context.WithoutCancel(ctx), k, false)
			return rec, err
		}
		if e = s.SendKey(ctx, k, false); e != nil {
			cleanup, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			_ = s.SendKey(cleanup, k, false)
			cancelCleanup()
			return rec, fmt.Errorf("selection failed: %w", e)
		}
		if err := sleepWithContext(ctx, gap, sleep); err != nil {
			return rec, err
		}
	}
	if opts.Settle > 0 {
		if err := sleepWithContext(ctx, opts.Settle, sleep); err != nil {
			return rec, err
		}
	}
	if e = s.Verify(ctx, t); e != nil {
		rec.State = VerifyFailed
		rec.Detail = e.Error()
		return rec, fmt.Errorf("selection not verified: %w", e)
	}
	rec.State = Verified
	return rec, nil
}

func LockPath(dir, device string) string {
	h := sha256.Sum256([]byte(device))
	return filepath.Join(dir, hex.EncodeToString(h[:])+".lock")
}

func sleepWithContext(ctx context.Context, d time.Duration, sleep func(context.Context, time.Duration) error) error {
	if d <= 0 {
		return nil
	}
	if sleep != nil {
		return sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
