// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pendingPlace records the __requestid reserved for an order before the POST
// fires. The record survives until a confirmed response clears it. An unknown
// outcome blocks further checkout until the customer inspects recent orders.
type pendingPlace struct {
	RequestID       string    `json:"request_id"`
	CartFingerprint string    `json:"cart_fingerprint"`
	At              time.Time `json:"at"`
}

// placementReservation serializes every checkout on this installation. An
// exclusive advisory lock is held from reservation through the POST and result
// handling, so a changed cart cannot bypass an earlier uncertain attempt.
type placementReservation struct {
	RequestID string

	recordPath string
	lockFile   *os.File
}

// cartFingerprint hashes the entire submitted order, including billing and
// context fields, without writing those private values to disk. A changed
// request body is refused while an earlier checkout remains unresolved.
func cartFingerprint(body postOrderBody) (string, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode checkout for idempotency: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func pendingPlaceRecordPath() string {
	return defaultConfigDirFile("pending-place.json")
}

// reservePlacement acquires the installation-wide checkout reservation. It fails
// closed on every persistence or locking error: an order must never POST
// without a durable idempotency record, and an unknown outcome must block
// another checkout even if the cart or payment details change.
func reservePlacement(fingerprint string) (*placementReservation, error) {
	return reservePlacementWithWriter(fingerprint, writeFileDurable)
}

func reservePlacementWithWriter(fingerprint string, writeRecord func(string, pendingPlace) error) (*placementReservation, error) {
	if err := checkPlacementDurability(); err != nil {
		return nil, err
	}
	recordPath := pendingPlaceRecordPath()
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o700); err != nil {
		return nil, fmt.Errorf("cannot create config directory for the checkout idempotency record: %w", err)
	}

	lockPath := recordPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open checkout lock file: %w", err)
	}
	if err := lockPlacementFile(lockFile); err != nil {
		_ = lockFile.Close()
		return nil, fmt.Errorf("another checkout is already in progress (checkout lock held); wait for it to finish")
	}
	res := &placementReservation{recordPath: recordPath, lockFile: lockFile}

	if data, err := os.ReadFile(recordPath); err == nil {
		var p pendingPlace
		if json.Unmarshal(data, &p) != nil || p.RequestID == "" || p.CartFingerprint == "" {
			res.Release()
			return nil, fmt.Errorf("checkout idempotency record %s is corrupt; inspect your recent orders, then delete the file to explicitly start a new attempt", recordPath)
		}
		if p.CartFingerprint != fingerprint {
			res.Release()
			return nil, fmt.Errorf("a previous checkout with different details has an unknown outcome; inspect recent orders before starting another checkout, then delete %s only after confirming the outcome", recordPath)
		}
		res.Release()
		return nil, fmt.Errorf("a previous checkout has an unknown outcome; inspect recent orders before starting another checkout, then delete %s only after confirming the outcome", recordPath)
	} else if !os.IsNotExist(err) {
		res.Release()
		return nil, fmt.Errorf("cannot read checkout idempotency record: %w", err)
	}

	id := newRequestID()
	if err := writeRecord(recordPath, pendingPlace{RequestID: id, CartFingerprint: fingerprint, At: time.Now()}); err != nil {
		res.Release()
		return nil, fmt.Errorf("cannot durably record the checkout idempotency id; refusing to place the order (a lost response could otherwise be charged twice): %w", err)
	}
	res.RequestID = id
	return res, nil
}

// writeFileDurable writes JSON via same-directory temp file + fsync + atomic
// rename + directory fsync, then reads the result back. Any failure means the
// record cannot be trusted to survive, so the caller must not POST.
func writeFileDurable(path string, record pendingPlace) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pending-place-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// The rename itself must survive a crash: sync the directory and treat
	// any failure as not-durable (the caller then refuses to POST).
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("syncing record directory: %w", err)
	}
	readBack, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verifying written record: %w", err)
	}
	var verify pendingPlace
	if json.Unmarshal(readBack, &verify) != nil || verify.RequestID != record.RequestID {
		return fmt.Errorf("written record did not read back intact")
	}
	return nil
}

// ConfirmSuccess removes the record after a confirmed order response so the
// next intentional identical cart gets a fresh id. The removal is made
// crash-durable (directory fsync) — a resurrected record would make a later
// intentional reorder reuse the confirmed id and be silently deduped away.
// A failure leaves checkout blocked for manual inspection, so it degrades to
// a warning string instead of hiding the confirmed order from the caller.
func (r *placementReservation) ConfirmSuccess() string {
	warnf := func(err error) string {
		return fmt.Sprintf("order placed, but durably clearing the checkout idempotency record failed (%v); inspect recent orders, then delete %s before placing another order", err, r.recordPath)
	}
	if err := os.Remove(r.recordPath); err != nil && !os.IsNotExist(err) {
		return warnf(err)
	}
	if err := syncDir(filepath.Dir(r.recordPath)); err != nil {
		return warnf(err)
	}
	return ""
}

// Release drops the checkout lock. Safe to call multiple times.
func (r *placementReservation) Release() {
	if r.lockFile != nil {
		unlockPlacementFile(r.lockFile)
		_ = r.lockFile.Close()
		r.lockFile = nil
	}
}
