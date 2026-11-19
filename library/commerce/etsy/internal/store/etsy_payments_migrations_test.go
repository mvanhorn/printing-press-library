package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPaymentChecksSchema(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	checks, err := s.ReadPaymentChecks()
	if err != nil || len(checks) != 0 {
		t.Fatalf("fresh read %v %v", checks, err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, EtsyPaymentChecksTable).Scan(&n); err != nil || n != 0 {
		t.Fatalf("reader created table: %d %v", n, err)
	}
	before := time.Now().Unix()
	for _, stamp := range []sql.NullInt64{{Int64: 100, Valid: true}, {Int64: 200, Valid: true}, {}} {
		if err := s.RecordPaymentCheck("3000000001", stamp); err != nil {
			t.Fatal(err)
		}
		checks, err = s.ReadPaymentChecks()
		if err != nil || len(checks) != 1 || checks["3000000001"].ReceiptUpdated != stamp || checks["3000000001"].CheckedAt < before {
			t.Fatalf("round trip %v %v", checks, err)
		}
	}
}

func TestPaymentChecksDeniedRoundTrip(t *testing.T) {
	etsyStore, etsyErr := Open(filepath.Join(t.TempDir(), "data.db"))
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyStore.Close()
	etsyStamp := sql.NullInt64{Int64: 100, Valid: true}
	if etsyErr := etsyStore.RecordPaymentDenied("3000000001", etsyStamp); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	var status string
	if err := etsyStore.db.QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id='3000000001'`).Scan(&status); err != nil || status != "not_found" {
		t.Fatal(status, err)
	}
	etsyChecks, etsyErr := etsyStore.ReadPaymentChecks()
	if etsyErr != nil || !etsyChecks["3000000001"].Denied || etsyChecks["3000000001"].ReceiptUpdated != etsyStamp {
		t.Fatal(etsyChecks, etsyErr)
	}
	if etsyErr := etsyStore.RecordPaymentCheck("3000000001", etsyStamp); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyChecks, etsyErr = etsyStore.ReadPaymentChecks()
	if etsyErr != nil || etsyChecks["3000000001"].Denied {
		t.Fatal(etsyChecks, etsyErr)
	}
}
func TestPaymentChecksUpgradeAddsStatus(t *testing.T) {
	etsyStore, etsyErr := Open(filepath.Join(t.TempDir(), "data.db"))
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyStore.Close()
	_, etsyErr = etsyStore.db.Exec(`CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY, receipt_updated INTEGER, checked_at INTEGER);
 INSERT INTO etsy_payment_checks VALUES('3000000001',100,10),('3000000002',200,20)`)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyChecks, etsyErr := etsyStore.ReadPaymentChecks()
	if etsyErr != nil || len(etsyChecks) != 2 || etsyChecks["3000000001"].Denied || etsyChecks["3000000002"].Denied {
		t.Fatal(etsyChecks, etsyErr)
	}
	etsyHas, etsyErr := etsyPaymentChecksHasStatus(etsyStore.db)
	if etsyErr != nil || etsyHas {
		t.Fatal("read altered schema", etsyErr)
	}
	if etsyErr := etsyStore.RecordPaymentDenied("3000000001", sql.NullInt64{Int64: 100, Valid: true}); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	for etsyI := 0; etsyI < 3; etsyI++ {
		if etsyErr := EnsurePaymentChecksSchema(etsyStore.db); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	var etsyCount int
	if etsyErr := etsyStore.db.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='status'`).Scan(&etsyCount); etsyErr != nil || etsyCount != 1 {
		t.Fatal(etsyCount, etsyErr)
	}
	etsyChecks, etsyErr = etsyStore.ReadPaymentChecks()
	if etsyErr != nil || len(etsyChecks) != 2 || etsyChecks["3000000002"].ReceiptUpdated.Int64 != 200 || etsyChecks["3000000002"].CheckedAt != 20 {
		t.Fatal(etsyChecks, etsyErr)
	}
}
func TestPaymentChecksDDLNotNull(t *testing.T) {
	etsyStore, etsyErr := Open(filepath.Join(t.TempDir(), "data.db"))
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyStore.Close()
	if etsyErr := etsyStore.RecordPaymentCheck("3000000001", sql.NullInt64{}); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	for etsyName, etsyWant := range map[string]int{"checked_at": 1, "status": 0} {
		var etsyGot int
		if etsyErr := etsyStore.db.QueryRow(`SELECT "notnull" FROM pragma_table_info('etsy_payment_checks') WHERE name=?`, etsyName).Scan(&etsyGot); etsyErr != nil || etsyGot != etsyWant {
			t.Fatal(etsyName, etsyGot, etsyErr)
		}
	}
}

func TestEtsyPaymentLegacyDeniedMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.db.Exec(`CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY, receipt_updated INTEGER, checked_at INTEGER NOT NULL, status TEXT);
 INSERT INTO etsy_payment_checks VALUES('3000000001',100,10,'denied'),('3000000002',200,20,NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := s.ReadPaymentChecks()
	if err != nil || checks["3000000001"].Denied || checks["3000000001"].ReceiptUpdated.Valid {
		t.Fatal(checks, err)
	}
	var status sql.NullString
	var stamp sql.NullInt64
	assertRow := func(wantStatus sql.NullString, wantStamp sql.NullInt64) {
		t.Helper()
		if err := s.db.QueryRow(`SELECT status,receipt_updated FROM etsy_payment_checks WHERE receipt_id='3000000001'`).Scan(&status, &stamp); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || stamp != wantStamp {
			t.Fatal(status, stamp)
		}
	}
	assertRow(sql.NullString{String: "denied", Valid: true}, sql.NullInt64{Int64: 100, Valid: true})
	if err := s.RecordPaymentCheck("3000000003", sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	assertRow(sql.NullString{}, sql.NullInt64{})
	if err := EnsurePaymentChecksSchema(s.db); err != nil {
		t.Fatal(err)
	}
	assertRow(sql.NullString{}, sql.NullInt64{})
	checks, err = s.ReadPaymentChecks()
	if err != nil || checks["3000000002"].ReceiptUpdated != (sql.NullInt64{Int64: 200, Valid: true}) || checks["3000000002"].CheckedAt != 20 {
		t.Fatal(checks, err)
	}
	if err := s.RecordPaymentDenied("3000000001", sql.NullInt64{Int64: 100, Valid: true}); err != nil {
		t.Fatal(err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := EnsurePaymentChecksSchema(other.db); err != nil {
		t.Fatal(err)
	}
	assertRow(sql.NullString{String: "not_found", Valid: true}, sql.NullInt64{Int64: 100, Valid: true})
}

func etsyPaymentTestStore(etsyT *testing.T) *Store {
	etsyT.Helper()
	etsyStore, etsyErr := Open(filepath.Join(etsyT.TempDir(), "data.db"))
	if etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	etsyT.Cleanup(func() { etsyStore.Close() })
	return etsyStore
}

func etsyPaymentTestStamp(etsyValue int64) sql.NullInt64 {
	return sql.NullInt64{Int64: etsyValue, Valid: true}
}

func etsyPaymentTestChecks(etsyT *testing.T, etsyStore *Store) map[string]PaymentCheck {
	etsyT.Helper()
	etsyChecks, etsyErr := etsyStore.ReadPaymentChecks()
	if etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	return etsyChecks
}

func etsyPaymentTestCount(etsyT *testing.T, etsyStore *Store, etsyWant int) {
	etsyT.Helper()
	etsyCount, etsyErr := etsyStore.CountPaymentForbidden()
	if etsyErr != nil || etsyCount != etsyWant {
		etsyT.Fatalf("count=%d want=%d err=%v", etsyCount, etsyWant, etsyErr)
	}
}

func TestEtsyPaymentForbiddenRoundTrip(etsyT *testing.T) {
	etsyStore := etsyPaymentTestStore(etsyT)
	etsyBefore := time.Now().Unix()
	if etsyErr := etsyStore.RecordPaymentForbidden("3000000001", etsyPaymentTestStamp(100)); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	etsyCheck := etsyPaymentTestChecks(etsyT, etsyStore)["3000000001"]
	if !etsyCheck.Forbidden || etsyCheck.Denied || etsyCheck.ReceiptUpdated != etsyPaymentTestStamp(100) || etsyCheck.CheckedAt < etsyBefore {
		etsyT.Fatal(etsyCheck)
	}
	var etsyStatus string
	if etsyErr := etsyStore.db.QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id='3000000001'`).Scan(&etsyStatus); etsyErr != nil || etsyStatus != "forbidden" {
		etsyT.Fatal(etsyStatus, etsyErr)
	}
	etsyPaymentTestCount(etsyT, etsyStore, 1)
	if etsyErr := etsyStore.RecordPaymentDenied("3000000002", etsyPaymentTestStamp(200)); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	etsyCheck = etsyPaymentTestChecks(etsyT, etsyStore)["3000000002"]
	if !etsyCheck.Denied || etsyCheck.Forbidden {
		etsyT.Fatal(etsyCheck)
	}
	etsyPaymentTestCount(etsyT, etsyStore, 1)
}

func TestEtsyPaymentCheckRearmingOnProbeSuccess(etsyT *testing.T) {
	etsyStore := etsyPaymentTestStore(etsyT)
	for etsyID, etsyStamp := range map[string]int64{"3000000001": 100, "3000000002": 200, "3000000003": 300} {
		if etsyErr := etsyStore.RecordPaymentForbidden(etsyID, etsyPaymentTestStamp(etsyStamp)); etsyErr != nil {
			etsyT.Fatal(etsyErr)
		}
	}
	if etsyErr := etsyStore.RecordPaymentDenied("3000000004", etsyPaymentTestStamp(400)); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	if etsyErr := etsyStore.RecordPaymentCheck("3000000005", etsyPaymentTestStamp(50)); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	etsyBefore := etsyPaymentTestChecks(etsyT, etsyStore)
	etsyRearmed, etsyErr := etsyStore.RecordPaymentCheckRearming("3000000002", etsyPaymentTestStamp(777))
	if etsyErr != nil || etsyRearmed != 3 {
		etsyT.Fatal(etsyRearmed, etsyErr)
	}
	etsyChecks := etsyPaymentTestChecks(etsyT, etsyStore)
	for _, etsyID := range []string{"3000000001", "3000000003"} {
		etsyCheck := etsyChecks[etsyID]
		if etsyCheck.Forbidden || etsyCheck.Denied || etsyCheck.ReceiptUpdated.Valid || etsyCheck.CheckedAt != etsyBefore[etsyID].CheckedAt {
			etsyT.Fatal(etsyID, etsyCheck)
		}
	}
	etsyProbe := etsyChecks["3000000002"]
	if etsyProbe.Forbidden || etsyProbe.Denied || etsyProbe.ReceiptUpdated != etsyPaymentTestStamp(777) {
		etsyT.Fatal(etsyProbe)
	}
	if !etsyChecks["3000000004"].Denied || etsyChecks["3000000004"] != etsyBefore["3000000004"] || etsyChecks["3000000005"] != etsyBefore["3000000005"] {
		etsyT.Fatal(etsyChecks)
	}
	var etsyStatus sql.NullString
	if etsyErr := etsyStore.db.QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id='3000000002'`).Scan(&etsyStatus); etsyErr != nil || etsyStatus.String != "ok" {
		etsyT.Fatal(etsyStatus, etsyErr)
	}
	etsyPaymentTestCount(etsyT, etsyStore, 0)
}

func TestEtsyPaymentCheckRearmingNotForbiddenDoesNotRearm(etsyT *testing.T) {
	etsyStore := etsyPaymentTestStore(etsyT)
	if etsyErr := etsyStore.RecordPaymentForbidden("3000000001", etsyPaymentTestStamp(100)); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	etsyRearmed, etsyErr := etsyStore.RecordPaymentCheckRearming("3000000002", etsyPaymentTestStamp(200))
	if etsyErr != nil || etsyRearmed != 0 {
		etsyT.Fatal(etsyRearmed, etsyErr)
	}
	etsyChecks := etsyPaymentTestChecks(etsyT, etsyStore)
	if !etsyChecks["3000000001"].Forbidden || etsyChecks["3000000002"].ReceiptUpdated != etsyPaymentTestStamp(200) {
		etsyT.Fatal(etsyChecks)
	}
	var etsyStatus sql.NullString
	if etsyErr := etsyStore.db.QueryRow(`SELECT status FROM etsy_payment_checks WHERE receipt_id='3000000002'`).Scan(&etsyStatus); etsyErr != nil || etsyStatus.String != "ok" {
		etsyT.Fatal(etsyStatus, etsyErr)
	}
	etsyPaymentTestCount(etsyT, etsyStore, 1)
}

func TestEtsyPaymentCheckRearmingIsAtomic(etsyT *testing.T) {
	etsyStore := etsyPaymentTestStore(etsyT)
	for etsyID, etsyStamp := range map[string]int64{"3000000001": 100, "3000000002": 200} {
		if etsyErr := etsyStore.RecordPaymentForbidden(etsyID, etsyPaymentTestStamp(etsyStamp)); etsyErr != nil {
			etsyT.Fatal(etsyErr)
		}
	}
	etsyBefore := etsyPaymentTestChecks(etsyT, etsyStore)
	if _, etsyErr := etsyStore.db.Exec(`CREATE TRIGGER etsy_probe_fail BEFORE UPDATE ON etsy_payment_checks WHEN NEW.receipt_id='3000000001' AND NEW.status='ok' AND NEW.receipt_updated IS NOT NULL BEGIN SELECT RAISE(ABORT,'synthetic probe failure'); END;`); etsyErr != nil {
		etsyT.Fatal(etsyErr)
	}
	_, etsyErr := etsyStore.RecordPaymentCheckRearming("3000000001", etsyPaymentTestStamp(500))
	if etsyErr == nil || !strings.Contains(etsyErr.Error(), "synthetic probe failure") {
		etsyT.Fatal(etsyErr)
	}
	etsyAfter := etsyPaymentTestChecks(etsyT, etsyStore)
	for etsyID, etsyCheck := range etsyBefore {
		if !etsyAfter[etsyID].Forbidden || etsyAfter[etsyID] != etsyCheck {
			etsyT.Fatal(etsyID, etsyAfter[etsyID], etsyCheck)
		}
	}
	etsyPaymentTestCount(etsyT, etsyStore, 2)
}

func TestEtsyCountPaymentForbiddenFreshStore(etsyT *testing.T) {
	etsyStore := etsyPaymentTestStore(etsyT)
	etsyPaymentTestCount(etsyT, etsyStore, 0)
	var etsyCount int
	if etsyErr := etsyStore.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, EtsyPaymentChecksTable).Scan(&etsyCount); etsyErr != nil || etsyCount != 0 {
		etsyT.Fatal(etsyCount, etsyErr)
	}
}

func TestBatchAEverOKBackfillAndPreservation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.DB().Exec(`CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY,receipt_updated INTEGER,checked_at INTEGER,status TEXT);
 INSERT INTO etsy_payment_checks VALUES('legacy',100,1,NULL),('ok',NULL,1,'ok'),('no',100,1,'not_found')`)
	if err != nil {
		t.Fatal(err)
	}
	version, err := s.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if err = EnsurePaymentChecksSchema(s.DB()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"legacy", "ok"} {
		if err = s.RecordPaymentForbidden(id, sql.NullInt64{}); err != nil {
			t.Fatal(err)
		}
		yes, e := s.PaymentCheckSucceeded(id)
		if e != nil || !yes {
			t.Fatal(id, yes, e)
		}
	}
	if _, err = s.RecordPaymentCheckRearming("ok", sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	if err = s.RecordPaymentDenied("legacy", sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	checks, err := s.ReadPaymentChecks()
	if err != nil {
		t.Fatal(err)
	}
	if !checks["legacy"].EverOK || !checks["ok"].EverOK || checks["no"].EverOK {
		t.Fatal(checks)
	}
	after, err := s.SchemaVersion()
	if err != nil || after != version {
		t.Fatal(after, err)
	}
}

func TestPaymentChecksExistingColumnBackfill(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "checks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = EnsurePaymentChecksSchema(s.DB()); err != nil {
		t.Fatal(err)
	}
	_, err = s.DB().Exec(`INSERT INTO etsy_payment_checks(receipt_id,checked_at,status,ever_ok) VALUES ('synthetic',1,'ok',NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.RecordPaymentForbidden("synthetic", sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	ok, err := s.PaymentCheckSucceeded("synthetic")
	if err != nil || !ok {
		t.Fatalf("success=%v err=%v", ok, err)
	}
}
