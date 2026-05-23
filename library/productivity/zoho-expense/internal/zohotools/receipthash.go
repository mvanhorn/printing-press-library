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

// RecordHash persists a content-hash → expense_id association.
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
