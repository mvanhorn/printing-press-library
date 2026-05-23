package zohotools

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashFile computes a SHA256 hex digest of the file at path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LookupHash returns the expense_id associated with a content hash, or
// (false) when no prior upload matches.
func LookupHash(db *sql.DB, hash string) (string, bool, error) {
	var expenseID string
	err := db.QueryRow(
		`SELECT expense_id FROM receipt_hashes WHERE hash = ?`, hash,
	).Scan(&expenseID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return expenseID, true, nil
}

// RecordHash persists a content-hash → expense_id association. Used by the
// --force upload path (which skips reservation entirely) and as a fallback
// for callers that don't use the reserve/confirm pattern. New code should
// prefer ReserveHash + ConfirmHash to avoid the TOCTOU race that this
// upsert-late pattern has under parallel ingestion of duplicate-content
// files.
func RecordHash(db *sql.DB, hash, expenseID, originalFilename string) error {
	_, err := db.Exec(
		`INSERT INTO receipt_hashes (hash, expense_id, original_filename, uploaded_at)
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(hash) DO UPDATE SET expense_id = excluded.expense_id, original_filename = excluded.original_filename`,
		hash, expenseID, originalFilename,
	)
	if err != nil {
		return fmt.Errorf("record receipt hash: %w", err)
	}
	return nil
}

// ReserveHash atomically claims a hash slot before upload. Returns
// (true, "") when the caller has won the race and should proceed with
// upload (then call ConfirmHash on success or ReleaseHash on failure).
// Returns (false, expense_id) when an earlier upload already recorded a
// real expense_id — the caller should treat the file as a duplicate.
// Returns (false, "") when another goroutine has already reserved the
// slot but hasn't finished uploading yet — the caller should skip this
// file (one of the racing siblings will succeed and record the canonical
// expense_id; the others would create duplicate Zoho expenses).
//
// The reservation uses the sentinel empty-string expense_id to mark
// "claimed but not yet confirmed." ConfirmHash overwrites it with the
// real ID; ReleaseHash deletes the row (only when the expense_id is
// still the sentinel, so a confirmed row cannot be accidentally removed).
//
// This pattern closes the TOCTOU race where N goroutines all see
// LookupHash returning "not found" and all proceed to POST /expenses,
// creating N Zoho expenses where only one would be tracked.
func ReserveHash(db *sql.DB, hash, originalFilename string) (bool, string, error) {
	res, err := db.Exec(
		`INSERT INTO receipt_hashes (hash, expense_id, original_filename, uploaded_at)
		 VALUES (?, '', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(hash) DO NOTHING`,
		hash, originalFilename,
	)
	if err != nil {
		return false, "", fmt.Errorf("reserve receipt hash: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 1 {
		return true, "", nil
	}
	// A row already exists; read whether it's a real upload or a sibling reservation.
	var existingID string
	err = db.QueryRow(`SELECT expense_id FROM receipt_hashes WHERE hash = ?`, hash).Scan(&existingID)
	if err != nil {
		return false, "", fmt.Errorf("read existing reservation: %w", err)
	}
	return false, existingID, nil
}

// ConfirmHash writes the real expense_id into a row previously reserved
// via ReserveHash, replacing the empty-string sentinel. Safe to call even
// if the row's expense_id was already set (e.g., re-run of a partially
// completed batch).
func ConfirmHash(db *sql.DB, hash, expenseID string) error {
	_, err := db.Exec(
		`UPDATE receipt_hashes SET expense_id = ?, uploaded_at = CURRENT_TIMESTAMP WHERE hash = ?`,
		expenseID, hash,
	)
	if err != nil {
		return fmt.Errorf("confirm receipt hash: %w", err)
	}
	return nil
}

// ReleaseHash removes an unconfirmed reservation. Only deletes the row
// when the expense_id is still the empty sentinel — never wipes a
// confirmed record. Called on the upload-failure path so the next
// invocation can retry the same file.
func ReleaseHash(db *sql.DB, hash string) error {
	_, err := db.Exec(
		`DELETE FROM receipt_hashes WHERE hash = ? AND expense_id = ''`,
		hash,
	)
	if err != nil {
		return fmt.Errorf("release receipt hash: %w", err)
	}
	return nil
}
