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

// pendingPlace records the __requestid reserved for a cart before the POST
// fires. postmicmeshorder dedups on __requestid, so a retry of the same cart
// must present the same id — a fresh id re-places (and re-charges) an order
// whose first response was lost. The record survives until a confirmed order
// response clears it; it is deliberately never auto-expired, because expiring
// an uncertain attempt back into a fresh id recreates the double-charge path.
type pendingPlace struct {
	RequestID       string    `json:"request_id"`
	CartFingerprint string    `json:"cart_fingerprint"`
	At              time.Time `json:"at"`
}

// placementReservation serializes one checkout attempt per cart fingerprint:
// an exclusive advisory lock is held from reservation through the POST and
// result handling, so overlapping invocations for the same cart cannot race
// the record, and invocations for different carts never share state.
type placementReservation struct {
	RequestID string
	Reused    bool

	fingerprint string
	recordPath  string
	lockFile    *os.File
}

// cartFingerprint identifies a checkout by everything charge-relevant the
// client controls: restaurant, items (with options), money, and the payment
// identity the order will bill.
func cartFingerprint(slug string, restID int, items []cartItem, subtotal, tax, tip float64, stripeCustomer, stripeCard string) string {
	payload, _ := json.Marshal(struct {
		Slug           string     `json:"slug"`
		RestID         int        `json:"rest_id"`
		Items          []cartItem `json:"items"`
		Subtotal       float64    `json:"subtotal"`
		Tax            float64    `json:"tax"`
		Tip            float64    `json:"tip"`
		StripeCustomer string     `json:"stripe_customer"`
		StripeCard     string     `json:"stripe_card"`
	}{slug, restID, items, subtotal, tax, tip, stripeCustomer, stripeCard})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func pendingPlaceRecordPath(fingerprint string) string {
	return defaultConfigDirFile("pending-place-" + fingerprint + ".json")
}

// reservePlacement acquires the per-cart checkout reservation. It fails
// closed on every persistence or locking error: an order must never POST
// without a durable idempotency record, and two overlapping invocations for
// the same cart must never both POST.
func reservePlacement(fingerprint string) (*placementReservation, error) {
	recordPath := pendingPlaceRecordPath(fingerprint)
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
		return nil, fmt.Errorf("another placement for this exact cart is already in progress (checkout lock held); wait for it to finish")
	}
	res := &placementReservation{fingerprint: fingerprint, recordPath: recordPath, lockFile: lockFile}

	if data, err := os.ReadFile(recordPath); err == nil {
		var p pendingPlace
		if json.Unmarshal(data, &p) != nil || p.RequestID == "" || p.CartFingerprint != fingerprint {
			res.Release()
			return nil, fmt.Errorf("checkout idempotency record %s is corrupt; inspect your recent orders, then delete the file to explicitly start a new attempt", recordPath)
		}
		res.RequestID = p.RequestID
		res.Reused = true
		return res, nil
	} else if !os.IsNotExist(err) {
		res.Release()
		return nil, fmt.Errorf("cannot read checkout idempotency record: %w", err)
	}

	id := newRequestID()
	if err := writeFileDurable(recordPath, pendingPlace{RequestID: id, CartFingerprint: fingerprint, At: time.Now()}); err != nil {
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
// Failure here is not money-losing (a reused id for an already-confirmed
// order is absorbed by the server's dedup), so it degrades to a warning
// string instead of an error.
func (r *placementReservation) ConfirmSuccess() string {
	warnf := func(err error) string {
		return fmt.Sprintf("order placed, but durably clearing the checkout idempotency record failed (%v); delete %s before intentionally reordering the identical cart", err, r.recordPath)
	}
	if err := os.Remove(r.recordPath); err != nil && !os.IsNotExist(err) {
		return warnf(err)
	}
	if err := syncDir(filepath.Dir(r.recordPath)); err != nil {
		return warnf(err)
	}
	return ""
}

// Release drops the per-cart lock. Safe to call multiple times.
func (r *placementReservation) Release() {
	if r.lockFile != nil {
		unlockPlacementFile(r.lockFile)
		_ = r.lockFile.Close()
		r.lockFile = nil
	}
}
