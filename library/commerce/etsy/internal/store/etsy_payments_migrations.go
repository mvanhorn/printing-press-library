// Hand-written (not generated): lazy payment-check ledger.
package store

import (
	"database/sql"
	"strings"
	"sync"
	"time"
)

const EtsyPaymentChecksTable = "etsy_payment_checks"

// etsy-ext: Serialize the legacy denial migration once per database handle.
var etsyPaymentCheckMigrations sync.Map

type etsyPaymentCheckMigration struct {
	sync.Mutex
	done bool
}

// EnsurePaymentChecksSchema lazily creates the payment-check ledger.
func EnsurePaymentChecksSchema(db *sql.DB) error {
	state, _ := etsyPaymentCheckMigrations.LoadOrStore(db, &etsyPaymentCheckMigration{})
	migration := state.(*etsyPaymentCheckMigration)
	migration.Lock()
	defer migration.Unlock()
	if migration.done {
		_, err := db.Exec(`UPDATE etsy_payment_checks SET ever_ok=1 WHERE (status='ok' OR (status IS NULL AND receipt_updated IS NOT NULL)) AND COALESCE(ever_ok,0)=0`)
		return err
	}
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS etsy_payment_checks(receipt_id TEXT PRIMARY KEY, receipt_updated INTEGER, checked_at INTEGER NOT NULL, status TEXT)`)
	if err != nil {
		return err
	}
	hasStatus, err := etsyPaymentChecksHasStatus(db)
	if err != nil {
		return err
	}
	if !hasStatus {
		if _, err = db.Exec(`ALTER TABLE etsy_payment_checks ADD COLUMN status TEXT`); err != nil {
			// Another connection may have completed the same upgrade.
			if hasStatus, checkErr := etsyPaymentChecksHasStatus(db); checkErr != nil || !hasStatus {
				return err
			}
		}
	}
	// Preserve lifetime success independently of the latest response.
	var hasEver int
	if err = db.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='ever_ok'`).Scan(&hasEver); err != nil {
		return err
	}
	if hasEver == 0 {
		if _, err = db.Exec(`ALTER TABLE etsy_payment_checks ADD COLUMN ever_ok INTEGER`); err != nil {
			if checkErr := db.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='ever_ok'`).Scan(&hasEver); checkErr != nil || hasEver == 0 {
				return err
			}
		}
	}
	if _, err = db.Exec(`UPDATE etsy_payment_checks SET ever_ok=1 WHERE (status='ok' OR (status IS NULL AND receipt_updated IS NOT NULL)) AND COALESCE(ever_ok,0)=0`); err != nil {
		return err
	}
	// etsy-ext: Legacy denials mixed 403 and 404; re-arm once, preserving new not_found checks.
	if _, err = db.Exec(`UPDATE etsy_payment_checks SET status=NULL, receipt_updated=NULL WHERE status='denied'`); err != nil {
		return err
	}
	// etsy-ext: Preserve lifetime success independently of prunable receipt rows.
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS etsy_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TRIGGER IF NOT EXISTS etsy_payment_success_insert AFTER INSERT ON etsy_payment_checks WHEN NEW.ever_ok=1 BEGIN INSERT INTO etsy_meta(key,value) VALUES('payments_ever_succeeded','1') ON CONFLICT(key) DO UPDATE SET value='1'; END;
 CREATE TRIGGER IF NOT EXISTS etsy_payment_success_update AFTER UPDATE OF ever_ok ON etsy_payment_checks WHEN NEW.ever_ok=1 BEGIN INSERT INTO etsy_meta(key,value) VALUES('payments_ever_succeeded','1') ON CONFLICT(key) DO UPDATE SET value='1'; END;
 INSERT OR IGNORE INTO etsy_meta(key,value) SELECT 'payments_ever_succeeded','1' WHERE EXISTS(SELECT 1 FROM etsy_payment_checks WHERE ever_ok=1)`); err != nil {
		return err
	}
	migration.done = true
	return nil
}

func etsyPaymentChecksHasStatus(db *sql.DB) (bool, error) {
	var etsyCount int
	err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='status'`).Scan(&etsyCount)
	return etsyCount > 0, err
}

type PaymentCheck struct {
	ReceiptUpdated sql.NullInt64
	CheckedAt      int64
	Denied         bool
	Forbidden      bool
	Successful     bool
	EverOK         bool
}

// ReadPaymentChecks never creates the table, including on a fresh store.
func (s *Store) ReadPaymentChecks() (map[string]PaymentCheck, error) {
	return s.readPaymentChecks("")
}

// PaymentCheckSucceeded reads only this receipt, including legacy stamped successes.
func (s *Store) PaymentCheckSucceeded(receiptID string) (bool, error) {
	checks, err := s.readPaymentChecks(receiptID)
	return checks[receiptID].EverOK, err
}

func (s *Store) readPaymentChecks(receiptID string) (map[string]PaymentCheck, error) {
	checks := map[string]PaymentCheck{}
	var name string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, EtsyPaymentChecksTable).Scan(&name)
	if err == sql.ErrNoRows {
		return checks, nil
	}
	if err != nil {
		return nil, err
	}
	etsyHasStatus, err := etsyPaymentChecksHasStatus(s.db)
	if err != nil {
		return nil, err
	}
	etsyQuery := `SELECT receipt_id, receipt_updated, checked_at, NULL FROM etsy_payment_checks`
	if etsyHasStatus {
		etsyQuery = `SELECT receipt_id, receipt_updated, checked_at, status FROM etsy_payment_checks`
	}
	var hasEver int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='ever_ok'`).Scan(&hasEver); err != nil {
		return nil, err
	}
	if hasEver > 0 {
		etsyQuery = strings.Replace(etsyQuery, " FROM", ", COALESCE(ever_ok,0) FROM", 1)
	} else {
		etsyQuery = strings.Replace(etsyQuery, " FROM", ", 0 FROM", 1)
	}
	var args []any
	if receiptID != "" {
		etsyQuery += " WHERE receipt_id=?"
		args = append(args, receiptID)
	}
	rows, err := s.db.Query(etsyQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var check PaymentCheck
		var etsyStatus sql.NullString
		if err := rows.Scan(&id, &check.ReceiptUpdated, &check.CheckedAt, &etsyStatus, &check.EverOK); err != nil {
			return nil, err
		}
		check.Successful = etsyStatus.String == "ok" || (!etsyStatus.Valid && check.ReceiptUpdated.Valid)
		check.EverOK = check.EverOK || check.Successful
		check.Denied = etsyStatus.String == "not_found"
		check.Forbidden = etsyStatus.String == "forbidden"
		// etsy-ext: Re-arm ambiguous legacy denials without mutating on read.
		if etsyStatus.String == "denied" {
			check.ReceiptUpdated = sql.NullInt64{}
		}
		checks[id] = check
	}
	return checks, rows.Err()
}

// RecordPaymentCheck records the receipt stamp observed by a successful walk.
func (s *Store) RecordPaymentCheck(receiptID string, receiptUpdated sql.NullInt64) error {
	return s.etsyRecordPaymentCheck(receiptID, receiptUpdated, "ok")
}

// RecordPaymentDenied records a permanent per-receipt 404.
func (s *Store) RecordPaymentDenied(receiptID string, receiptUpdated sql.NullInt64) error {
	return s.etsyRecordPaymentCheck(receiptID, receiptUpdated, "not_found")
}

func (s *Store) etsyRecordPaymentCheck(receiptID string, receiptUpdated sql.NullInt64, etsyStatus any) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if err := EnsurePaymentChecksSchema(s.db); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at,status,ever_ok) VALUES(?,?,?,?,?) ON CONFLICT(receipt_id) DO UPDATE SET receipt_updated=excluded.receipt_updated, checked_at=excluded.checked_at, status=excluded.status, ever_ok=MAX(COALESCE(etsy_payment_checks.ever_ok,0),excluded.ever_ok)`, receiptID, receiptUpdated, time.Now().Unix(), etsyStatus, etsyStatus == "ok")
	return err
}

// etsy-ext: RecordPaymentForbidden records a retryable authorization failure.
func (s *Store) RecordPaymentForbidden(receiptID string, receiptUpdated sql.NullInt64) error {
	return s.etsyRecordPaymentCheck(receiptID, receiptUpdated, "forbidden")
}

// etsy-ext: RecordPaymentCheckRearming atomically re-arms forbidden checks when a forbidden probe succeeds.
func (s *Store) RecordPaymentCheckRearming(receiptID string, receiptUpdated sql.NullInt64) (rearmed int64, err error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if etsyErr := EnsurePaymentChecksSchema(s.db); etsyErr != nil {
		return 0, etsyErr
	}
	etsyTx, etsyErr := s.db.Begin()
	if etsyErr != nil {
		return 0, etsyErr
	}
	defer etsyTx.Rollback()
	var etsyStatus sql.NullString
	etsyErr = etsyTx.QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id=?`, receiptID).Scan(&etsyStatus)
	if etsyErr != nil && etsyErr != sql.ErrNoRows {
		return 0, etsyErr
	}
	var etsyRearmed int64
	if etsyStatus.String == "forbidden" {
		etsyResult, etsyErr := etsyTx.Exec(`UPDATE etsy_payment_checks SET status=NULL, receipt_updated=NULL WHERE status='forbidden'`)
		if etsyErr != nil {
			return 0, etsyErr
		}
		etsyRearmed, etsyErr = etsyResult.RowsAffected()
		if etsyErr != nil {
			return 0, etsyErr
		}
	}
	_, etsyErr = etsyTx.Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at,status,ever_ok) VALUES(?,?,?,'ok',1) ON CONFLICT(receipt_id) DO UPDATE SET receipt_updated=excluded.receipt_updated, checked_at=excluded.checked_at, status=excluded.status, ever_ok=1`, receiptID, receiptUpdated, time.Now().Unix())
	if etsyErr != nil {
		return 0, etsyErr
	}
	if etsyErr = etsyTx.Commit(); etsyErr != nil {
		return 0, etsyErr
	}
	return etsyRearmed, nil
}

// etsy-ext: CountPaymentForbidden counts authorization failures without creating or upgrading the ledger.
func (s *Store) CountPaymentForbidden() (int, error) {
	var etsyName string
	etsyErr := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, EtsyPaymentChecksTable).Scan(&etsyName)
	if etsyErr == sql.ErrNoRows {
		return 0, nil
	}
	if etsyErr != nil {
		return 0, etsyErr
	}
	etsyHasStatus, etsyErr := etsyPaymentChecksHasStatus(s.db)
	if etsyErr != nil || !etsyHasStatus {
		return 0, etsyErr
	}
	var etsyCount int
	etsyErr = s.db.QueryRow(`SELECT count(*) FROM etsy_payment_checks WHERE status='forbidden'`).Scan(&etsyCount)
	return etsyCount, etsyErr
}

// ClearPaymentSuccess classifies a current forbidden response as receipt-level.
func (s *Store) ClearPaymentSuccess(receiptID string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`UPDATE etsy_payment_checks SET ever_ok=0 WHERE receipt_id=? AND status='forbidden'`, receiptID)
	return err
}
