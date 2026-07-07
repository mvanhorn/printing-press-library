// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package pexels

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/pexels/internal/cliutil"
)

// RateSnapshot is the persisted form of the most recently observed rate-limit
// headers, used by `quota forecast` to estimate whether a bulk pull fits.
type RateSnapshot struct {
	Limit     int64     `json:"limit"`
	Remaining int64     `json:"remaining"`
	Reset     int64     `json:"reset"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Known reports whether the snapshot carries a real limit value (vs. an
// all-zero snapshot that should be treated as "quota unknown").
func (s RateSnapshot) Known() bool { return s.Limit > 0 }

// ErrNoRateLedger is returned by LoadRate when no ledger file exists yet, so
// callers can distinguish "never observed a quota" from a read failure.
var ErrNoRateLedger = errors.New("no rate ledger recorded yet")

func rateLedgerPath() (string, error) {
	dir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pexels-pp-cli", "rate-ledger.json"), nil
}

// SaveRate persists ri to the rate ledger. It only writes when ri.Known is
// true (no point recording an all-zero snapshot from a header-less response).
func SaveRate(ri RateInfo) error {
	if !ri.Known {
		return nil
	}
	path, err := rateLedgerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	snap := RateSnapshot{
		Limit:     ri.Limit,
		Remaining: ri.Remaining,
		Reset:     ri.Reset,
		UpdatedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadRate reads the persisted rate snapshot. It returns ErrNoRateLedger when
// the ledger file does not exist.
func LoadRate() (RateSnapshot, error) {
	path, err := rateLedgerPath()
	if err != nil {
		return RateSnapshot{}, err
	}
	// #nosec G304 -- path comes from rateLedgerPath(), derived from the CLI's
	// own config dir; it is not influenced by user input.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RateSnapshot{}, ErrNoRateLedger
		}
		return RateSnapshot{}, err
	}
	var snap RateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return RateSnapshot{}, err
	}
	return snap, nil
}
