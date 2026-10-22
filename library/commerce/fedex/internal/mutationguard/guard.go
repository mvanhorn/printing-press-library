// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Package mutationguard keeps a small local ledger of FedEx write requests so
// a timeout, agent retry, or double-submit cannot silently buy a second label
// or book a second pickup. Entries are keyed by a digest of the origin, method,
// path, and canonical request body; the body itself is never stored.
package mutationguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/secureio"
)

const (
	StatusExecuting      = "executing"
	StatusSucceeded      = "succeeded"
	StatusRejected       = "rejected"
	StatusOutcomeUnknown = "outcome_unknown"

	// DefaultWindow is how long an identical write stays blocked after it
	// started, succeeded, or ended with an unknown outcome.
	DefaultWindow = 24 * time.Hour

	// staleLockAge lets a crashed process's lock file be reclaimed.
	staleLockAge = 2 * time.Minute
)

// DuplicateError reports that an identical write is already recorded.
type DuplicateError struct {
	Status    string
	StartedAt time.Time
}

func (e *DuplicateError) Error() string {
	var reason string
	switch e.Status {
	case StatusExecuting:
		reason = "an identical FedEx write is still in flight or was interrupted"
	case StatusOutcomeUnknown:
		reason = "an identical FedEx write ended with an unknown outcome; check FedEx before sending it again"
	default:
		reason = "an identical FedEx write already succeeded"
	}
	return fmt.Sprintf("duplicate FedEx write refused: %s (first sent %s). Re-send with --allow-duplicate (CLI) or allow_duplicate=true (MCP) only if you really want a second one", reason, e.StartedAt.UTC().Format(time.RFC3339))
}

// ErrBusy means another process holds the lock for the same request digest.
var ErrBusy = errors.New("an identical FedEx write is being sent by another process")

type record struct {
	Digest      string     `json:"digest"`
	Method      string     `json:"method"`
	Path        string     `json:"path"`
	Origin      string     `json:"origin"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// Blocking* remembers the most recent attempt that still blocks a resend
	// (succeeded, executing, or outcome_unknown) so a later forced attempt
	// that FedEx rejects cannot erase it.
	BlockingStatus string     `json:"blocking_status,omitempty"`
	BlockingSince  *time.Time `json:"blocking_since,omitempty"`
}

func (r *record) blocking(now time.Time, window time.Duration) (string, time.Time, bool) {
	if r.Status != StatusRejected && now.Sub(r.StartedAt) < window {
		return r.Status, r.StartedAt, true
	}
	if r.BlockingStatus != "" && r.BlockingSince != nil && now.Sub(*r.BlockingSince) < window {
		return r.BlockingStatus, *r.BlockingSince, true
	}
	return "", time.Time{}, false
}

// Ledger is a directory of one small JSON record per request digest.
type Ledger struct {
	Dir    string
	Window time.Duration
	Now    func() time.Time
}

// DefaultDir returns the private directory used for the write ledger.
func DefaultDir() (string, error) {
	if root := strings.TrimSpace(os.Getenv("FEDEX_DATA_DIR")); root != "" {
		return filepath.Join(root, "mutations"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("resolve home directory for the FedEx write ledger: %w", err)
	}
	return filepath.Join(home, ".local", "share", "fedex-pp-cli", "mutations"), nil
}

// Digest canonicalizes body (sorted keys, preserved number text) and hashes
// it together with the request target.
func Digest(origin, method, path string, body []byte) (string, error) {
	canonicalBody := json.RawMessage("null")
	if len(bytes.TrimSpace(body)) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			return "", fmt.Errorf("canonicalizing FedEx write body: %w", err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("canonicalizing FedEx write body: %w", err)
		}
		canonicalBody = encoded
	}
	path, _, _ = strings.Cut(path, "?")
	canonical, err := json.Marshal(struct {
		Version int             `json:"v"`
		Origin  string          `json:"origin"`
		Method  string          `json:"method"`
		Path    string          `json:"path"`
		Body    json.RawMessage `json:"body"`
	}{1, strings.ToLower(strings.TrimRight(origin, "/")), strings.ToUpper(method), path, canonicalBody})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// Begin records an executing write. Unless force is set, it refuses when an
// identical write is executing, has an unknown outcome, or succeeded within
// the window. A rejected write never blocks a corrected or repeated attempt.
func (l *Ledger) Begin(origin, method, path string, body []byte, force bool) (string, error) {
	digest, err := Digest(origin, method, path, body)
	if err != nil {
		return "", err
	}
	if err := l.ensureDir(); err != nil {
		return "", err
	}
	unlock, err := l.lock(digest)
	if err != nil {
		return "", err
	}
	defer unlock()

	now := l.now()
	existing, err := l.read(digest)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	rec := &record{Digest: digest, Method: strings.ToUpper(method), Path: path, Origin: origin, Status: StatusExecuting, StartedAt: now}
	if existing != nil {
		if status, since, ok := existing.blocking(now, l.window()); ok {
			if !force {
				return "", &DuplicateError{Status: status, StartedAt: since}
			}
			rec.BlockingStatus = status
			rec.BlockingSince = &since
		}
	}
	if err := l.write(rec); err != nil {
		return "", err
	}
	return digest, nil
}

// Complete stores the final status for a digest returned by Begin.
func (l *Ledger) Complete(digest, status string) error {
	switch status {
	case StatusSucceeded, StatusRejected, StatusOutcomeUnknown:
	default:
		return fmt.Errorf("invalid FedEx write ledger status %q", status)
	}
	if !validDigest(digest) {
		return fmt.Errorf("invalid FedEx write ledger digest")
	}
	unlock, err := l.lock(digest)
	if err != nil {
		return err
	}
	defer unlock()
	rec, err := l.read(digest)
	if err != nil {
		return fmt.Errorf("reading FedEx write ledger: %w", err)
	}
	now := l.now()
	rec.Status = status
	rec.CompletedAt = &now
	if status != StatusRejected {
		// This attempt now blocks resends on its own.
		rec.BlockingStatus = ""
		rec.BlockingSince = nil
	}
	return l.write(rec)
}

func (l *Ledger) window() time.Duration {
	if l.Window > 0 {
		return l.Window
	}
	return DefaultWindow
}

func (l *Ledger) now() time.Time {
	if l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}

func (l *Ledger) ensureDir() error {
	if strings.TrimSpace(l.Dir) == "" {
		return fmt.Errorf("FedEx write ledger directory is required")
	}
	if err := secureio.EnsurePrivateDir(l.Dir); err != nil {
		return fmt.Errorf("securing FedEx write ledger: %w", err)
	}
	return nil
}

func (l *Ledger) recordPath(digest string) string {
	return filepath.Join(l.Dir, digest+".json")
}

func (l *Ledger) read(digest string) (*record, error) {
	data, err := secureio.ReadFile(l.recordPath(digest))
	if err != nil {
		return nil, err
	}
	var rec record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("decoding FedEx write ledger entry: %w", err)
	}
	return &rec, nil
}

func (l *Ledger) write(rec *record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := secureio.WriteFileAtomic(l.recordPath(rec.Digest), append(data, '\n')); err != nil {
		return fmt.Errorf("writing FedEx write ledger: %w", err)
	}
	return nil
}

func (l *Ledger) lock(digest string) (func(), error) {
	path := filepath.Join(l.Dir, digest+".lock")
	for attempt := 0; attempt < 2; attempt++ {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, secureio.PrivateFileMode)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("locking FedEx write ledger: %w", closeErr)
			}
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("locking FedEx write ledger: %w", err)
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || time.Since(info.ModTime()) < staleLockAge || !info.Mode().IsRegular() {
			return nil, ErrBusy
		}
		_ = os.Remove(path)
	}
	return nil, ErrBusy
}

func validDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
