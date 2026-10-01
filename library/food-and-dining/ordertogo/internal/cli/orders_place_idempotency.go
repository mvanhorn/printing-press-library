// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// pendingPlace records the __requestid reserved for an order before the POST
// fires. The record survives until a confirmed response clears it. An unknown
// outcome blocks further checkout until the customer inspects recent orders.
type pendingPlace struct {
	RequestID        string    `json:"request_id"`
	CartFingerprint  string    `json:"cart_fingerprint"`
	At               time.Time `json:"at"`
	ConfirmedOrderID int       `json:"confirmed_order_id,omitempty"`
}

// placementReservation serializes every checkout on this installation. An
// exclusive advisory lock is held from reservation through the POST and result
// handling, so a changed cart cannot bypass an earlier uncertain attempt.
type placementReservation struct {
	RequestID string

	recordPath  string
	fingerprint string
	at          time.Time
	releaseLock func()
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

func confirmedPlaceRecordPath() string {
	return defaultConfigDirFile("confirmed-place.json")
}

// reservePlacement acquires the installation-wide checkout reservation. It fails
// closed on every persistence or locking error: an order must never POST
// without a durable idempotency record, and an unknown outcome must block
// another checkout even if the cart or payment details change.
func reservePlacement(fingerprint string) (*placementReservation, error) {
	return reservePlacementWithAckAndWriter(fingerprint, 0, writeFileDurable)
}

func reservePlacementAcknowledging(fingerprint string, orderID int) (*placementReservation, error) {
	return reservePlacementWithAckAndWriter(fingerprint, orderID, writeFileDurable)
}

// checkPendingPlacement reports an earlier unknown outcome before token
// refresh. This is read-only: a new reservation is created only after auth
// succeeds. reservePlacement repeats the check under the checkout lock.
func checkPendingPlacement(fingerprint string, acknowledgedOrderID int) error {
	_, err := pendingPlacementError(pendingPlaceRecordPath(), fingerprint, acknowledgedOrderID)
	return err
}

func loadPlacementRecord(recordPath string) (pendingPlace, bool, error) {
	data, err := readPendingPlacement(recordPath)
	if os.IsNotExist(err) {
		return pendingPlace{}, false, nil
	}
	if err != nil {
		return pendingPlace{}, false, fmt.Errorf("cannot read checkout idempotency record: %w", err)
	}
	var p pendingPlace
	if json.Unmarshal(data, &p) != nil || p.RequestID == "" || p.CartFingerprint == "" {
		return pendingPlace{}, false, fmt.Errorf("checkout idempotency record %s is corrupt; inspect your recent orders, then clear the reservation to explicitly start a new attempt", pendingPlacementLocation(recordPath))
	}
	return p, true, nil
}

// A confirmed receipt survives until the customer explicitly acknowledges its
// order ID. This protects against output, local-history, or delivery failure
// after the provider charged the card. The original pending record stays too.
func pendingPlacementError(recordPath, fingerprint string, acknowledgedOrderID int) (bool, error) {
	confirmed, hasConfirmed, err := loadPlacementRecord(confirmedPlaceRecordPath())
	if err != nil {
		return false, err
	}
	if hasConfirmed {
		if confirmed.ConfirmedOrderID <= 0 {
			return false, fmt.Errorf("confirmed checkout receipt %s is corrupt; inspect recent orders before clearing it", pendingPlacementLocation(confirmedPlaceRecordPath()))
		}
		pending, hasPending, err := loadPlacementRecord(recordPath)
		if err != nil {
			return false, err
		}
		if hasPending && pending.RequestID != confirmed.RequestID {
			return false, fmt.Errorf("confirmed checkout receipt and pending reservation do not match; inspect recent orders before another checkout")
		}
		if acknowledgedOrderID == confirmed.ConfirmedOrderID {
			return true, nil
		}
		return false, fmt.Errorf("previous checkout was confirmed as order %d; inspect recent orders, then pass --ack-last-order %d to place another order", confirmed.ConfirmedOrderID, confirmed.ConfirmedOrderID)
	}
	p, hasPending, err := loadPlacementRecord(recordPath)
	if err != nil {
		return false, err
	}
	if !hasPending {
		if acknowledgedOrderID > 0 {
			return false, fmt.Errorf("no confirmed order to acknowledge with --ack-last-order")
		}
		return false, nil
	}
	if p.CartFingerprint != fingerprint {
		return false, fmt.Errorf("a previous checkout with different details has an unknown outcome; inspect recent orders before starting another checkout, then clear the reservation at %s only after confirming the outcome", pendingPlacementLocation(recordPath))
	}
	return false, fmt.Errorf("a previous checkout has an unknown outcome; inspect recent orders before starting another checkout, then clear the reservation at %s only after confirming the outcome", pendingPlacementLocation(recordPath))
}

func reservePlacementWithWriter(fingerprint string, writeRecord func(string, pendingPlace) error) (*placementReservation, error) {
	return reservePlacementWithAckAndWriter(fingerprint, 0, writeRecord)
}

func reservePlacementWithAckAndWriter(fingerprint string, acknowledgedOrderID int, writeRecord func(string, pendingPlace) error) (*placementReservation, error) {
	recordPath := pendingPlaceRecordPath()
	releaseLock, err := acquirePlacementLock(recordPath)
	if err != nil {
		return nil, fmt.Errorf("cannot acquire checkout lock: %w", err)
	}
	res := &placementReservation{recordPath: recordPath, releaseLock: releaseLock}

	acknowledged, err := pendingPlacementError(recordPath, fingerprint, acknowledgedOrderID)
	if err != nil {
		res.Release()
		return nil, err
	}
	if acknowledged {
		// Keep the confirmed receipt until the pending record is durably
		// cleared. A failed clear never reaches another checkout POST.
		if err := clearPendingPlacement(recordPath); err != nil {
			res.Release()
			return nil, fmt.Errorf("clearing acknowledged checkout reservation: %w", err)
		}
		if err := clearPendingPlacement(confirmedPlaceRecordPath()); err != nil {
			res.Release()
			return nil, fmt.Errorf("clearing acknowledged checkout receipt: %w", err)
		}
	}

	id := newRequestID()
	now := time.Now()
	if err := writeRecord(recordPath, pendingPlace{RequestID: id, CartFingerprint: fingerprint, At: now}); err != nil {
		// The POST has not started, so a partially written reservation is safe
		// to remove. If cleanup also fails, checkout still stays blocked.
		cleanupErr := clearPendingPlacement(recordPath)
		res.Release()
		if cleanupErr != nil {
			return nil, fmt.Errorf("cannot durably record the checkout idempotency id; refusing to place the order: %w; removing the unsubmitted reservation also failed: %v", err, cleanupErr)
		}
		return nil, fmt.Errorf("cannot durably record the checkout idempotency id; refusing to place the order (a lost response could otherwise be charged twice): %w", err)
	}
	res.RequestID = id
	res.fingerprint = fingerprint
	res.at = now
	return res, nil
}

// writeFileDurable persists a reservation using the platform's crash-safe
// store, then verifies it can be read back. Any failure prevents the POST.
func writeFileDurable(path string, record pendingPlace) error {
	return writePendingPlacement(path, record)
}

// MarkConfirmed writes a separate durable receipt before any local history or
// output work. If that work fails, a retry sees the confirmed order ID instead
// of sending another paid POST. The original pending record is not removed.
func (r *placementReservation) MarkConfirmed(orderID int) string {
	if orderID <= 0 {
		return "order placed, but the confirmed order ID is invalid; inspect recent orders before another checkout"
	}
	receipt := pendingPlace{RequestID: r.RequestID, CartFingerprint: r.fingerprint, At: r.at, ConfirmedOrderID: orderID}
	if err := writeFileDurable(confirmedPlaceRecordPath(), receipt); err != nil {
		return fmt.Sprintf("order placed, but saving a durable checkout receipt failed (%v); inspect recent orders, then clear the reservation at %s before placing another order", err, pendingPlacementLocation(r.recordPath))
	}
	return ""
}

// Release drops the checkout lock. Safe to call multiple times.
func (r *placementReservation) Release() {
	if r.releaseLock != nil {
		r.releaseLock()
		r.releaseLock = nil
	}
}
