package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTrackedResourcesRetainChangedVersionsAndDedupeIdenticalJSON(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := json.RawMessage(`{"id":"ITEM_1","price":100,"updated_at":"2026-08-01T00:00:00Z"}`)
	// Same JSON object, different key order: this must not create a version.
	same := json.RawMessage(`{"updated_at":"2026-08-01T00:00:00Z","price":100,"id":"ITEM_1"}`)
	changed := json.RawMessage(`{"id":"ITEM_1","price":125,"updated_at":"2026-08-02T00:00:00Z"}`)
	for _, raw := range []json.RawMessage{first, same, changed, changed} {
		if err := db.Upsert("catalog", "ITEM_1", raw); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history WHERE resource_type='catalog' AND resource_id='ITEM_1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("history count = %d, want 2 distinct versions", count)
	}
}

func TestHistoryCanonicalizationPreservesLargeIntegers(t *testing.T) {
	first, err := canonicalHistoryJSON(json.RawMessage(`{"amount":9007199254740992}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalHistoryJSON(json.RawMessage(`{"amount":9007199254740993}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatalf("distinct large integers collapsed: %s", first)
	}
	if string(second) != `{"amount":9007199254740993}` {
		t.Fatalf("large integer changed: %s", second)
	}
}

func TestHistoryCanonicalizationRejectsTrailingJSON(t *testing.T) {
	if _, err := canonicalHistoryJSON(json.RawMessage(`{"id":1}{"id":2}`)); err == nil {
		t.Fatal("trailing JSON document was accepted")
	}
}

func TestHistoryCapsVersionsAndRecordsReconcileTombstone(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < resourceHistoryMaxVersions+5; i++ {
		raw, _ := json.Marshal(map[string]any{"id": "ITEM_1", "location_id": "L1", "price": i})
		if err := db.Upsert("catalog", "ITEM_1", raw); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history WHERE resource_type='catalog' AND resource_id='ITEM_1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != resourceHistoryMaxVersions {
		t.Fatalf("history count = %d, want cap %d", count, resourceHistoryMaxVersions)
	}
	deleted, err := db.ReconcilePartition("catalog", "$.location_id", "L1", nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	var latest []byte
	if err := db.DB().QueryRow(`SELECT data FROM resource_history WHERE resource_type='catalog' AND resource_id='ITEM_1' ORDER BY sequence DESC LIMIT 1`).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if string(latest) != `{"_deleted":true,"id":"ITEM_1"}` {
		t.Fatalf("latest history = %s, want tombstone", latest)
	}
}

func TestWebhookDeliveryLogRetainsDuplicateObservations(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "webhooks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		if _, err := db.RecordWebhookDelivery(WebhookDelivery{EventID: "evt-1", EventType: "payment.updated", ReceivedAt: now.Add(time.Duration(i) * time.Second), Payload: json.RawMessage(`{"event_id":"evt-1"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM webhook_deliveries WHERE event_id='evt-1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("delivery count = %d, want both duplicate observations", count)
	}
}

func TestSchemaV10UpgradesWebhookDeliveryLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`DROP TABLE webhook_deliveries`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`PRAGMA user_version = 10`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='webhook_deliveries'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "webhook_deliveries" {
		t.Fatalf("upgraded table = %q", name)
	}
}

func TestIdenticalUpsertCompactsExpiredHistoryBeforeDedupe(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "compact.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw := json.RawMessage(`{"id":"ITEM_1","type":"ITEM"}`)
	if err := db.Upsert("catalog", "ITEM_1", raw); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-resourceHistoryRetention - time.Hour).Format(time.RFC3339)
	oldTime, _ := time.Parse(time.RFC3339, old)
	if _, err := db.DB().Exec(`UPDATE resource_history SET observed_at=?, observed_at_ns=? WHERE resource_type='catalog' AND resource_id='ITEM_1'`, old, oldTime.UnixNano()); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("catalog", "ITEM_1", raw); err != nil {
		t.Fatal(err)
	}
	var count int
	var observedRaw string
	if err := db.DB().QueryRow(`SELECT COUNT(*), MAX(observed_at) FROM resource_history WHERE resource_type='catalog' AND resource_id='ITEM_1'`).Scan(&count, &observedRaw); err != nil {
		t.Fatal(err)
	}
	observed, err := time.Parse(time.RFC3339Nano, observedRaw)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || observed.Before(time.Now().UTC().Add(-time.Minute)) {
		t.Fatalf("count=%d observed=%v, want one refreshed recent version", count, observed)
	}
}

func TestWebhookSubscriptionSecretsNeverReachCacheHistoryOrFTS(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "redaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw := json.RawMessage(`{"id":"sub-1","notification_url":"https://example.test/hook","signature_key":"literal-signature-secret","nested":{"signing_secret":"literal-signing-secret","refresh_token":"literal-refresh-secret"}}`)
	if err := db.Upsert("webhooks-subscriptions", "sub-1", raw); err != nil {
		t.Fatal(err)
	}
	current, err := db.Get("webhooks-subscriptions", "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	var history, fts string
	if err := db.DB().QueryRow(`SELECT data FROM resource_history WHERE resource_type='webhooks-subscriptions' AND resource_id='sub-1' ORDER BY sequence DESC LIMIT 1`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRow(`SELECT content FROM resources_fts WHERE resource_type='webhooks-subscriptions' AND id='sub-1'`).Scan(&fts); err != nil {
		t.Fatal(err)
	}
	combined := string(current) + history + fts
	for _, secret := range []string{"literal-signature-secret", "literal-signing-secret", "literal-refresh-secret"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("secret %q reached persistent cache/index: %s", secret, combined)
		}
	}
	if !strings.Contains(string(current), "redacted") || !strings.Contains(history, "redacted") {
		t.Fatalf("redaction marker missing: current=%s history=%s", current, history)
	}
}

func TestOpenFullyConvergesExpiredWebhookRetentionInBatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retention.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-webhookDeliveryRetention - time.Hour)
	for i := 0; i < 1205; i++ {
		if _, err := tx.Exec(`INSERT INTO webhook_deliveries(event_id, received_at, received_at_source, received_at_ns, payload) VALUES(?,?,?,?,?)`, fmt.Sprintf("evt-%d", i), old.Format(time.RFC3339Nano), "provided", old.UnixNano(), `{}`); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM webhook_deliveries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired webhook rows remain after bounded convergence: %d", count)
	}
}

func TestUpgradeScrubsSubscriptionBeforeV10HistoryBackfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-secrets.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"id":"sub-legacy","signature_key":"literal-upgrade-signature","nested":{"secret_key":"literal-upgrade-secret"}}`
	if _, err := db.DB().Exec(`INSERT OR REPLACE INTO resources(id, resource_type, data) VALUES('sub-legacy','webhooks-subscriptions',?)`, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`DELETE FROM resource_history WHERE resource_type='webhooks-subscriptions' AND resource_id='sub-legacy'`); err != nil {
		t.Fatal(err)
	}
	rowID := ftsRowID("webhooks-subscriptions", "sub-legacy")
	if _, err := db.DB().Exec(`DELETE FROM resources_fts WHERE rowid=?`, rowID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO resources_fts(rowid,id,resource_type,content) VALUES(?,'sub-legacy','webhooks-subscriptions',?)`, rowID, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`PRAGMA user_version = 9`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	current, err := db.Get("webhooks-subscriptions", "sub-legacy")
	if err != nil {
		t.Fatal(err)
	}
	var history, fts string
	if err := db.DB().QueryRow(`SELECT data FROM resource_history WHERE resource_type='webhooks-subscriptions' AND resource_id='sub-legacy' ORDER BY sequence DESC LIMIT 1`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRow(`SELECT content FROM resources_fts WHERE rowid=?`, rowID).Scan(&fts); err != nil {
		t.Fatal(err)
	}
	combined := string(current) + history + fts
	for _, secret := range []string{"literal-upgrade-signature", "literal-upgrade-secret"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("legacy secret %q survived upgrade scrub: %s", secret, combined)
		}
	}
	if !strings.Contains(combined, "redacted") {
		t.Fatalf("upgrade did not preserve redaction markers: %s", combined)
	}
}

func TestOpenFullyConvergesStaleResourceHistoryInBatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history-retention.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-resourceHistoryRetention - time.Hour)
	for i := 0; i < 1205; i++ {
		if _, err := tx.Exec(`INSERT INTO resource_history(resource_type,resource_id,data,observed_at,observed_at_ns) VALUES('catalog',?,?,?,?)`, fmt.Sprintf("old-%d", i), `{}`, old.Format(time.RFC3339Nano), old.UnixNano()); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history WHERE resource_type='catalog' AND resource_id LIKE 'old-%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale resource history remains after bounded convergence: %d", count)
	}
}

func TestCurrentSchemaOpenDoesNotRepeatV10HistoryBackfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gated.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`INSERT INTO resources(id, resource_type, data) VALUES ('ITEM_1','catalog','{"id":"ITEM_1"}')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history WHERE resource_type='catalog' AND resource_id='ITEM_1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("current-schema open reran v10 data backfill: count=%d", count)
	}
}

func TestUntrackedResourcesDoNotGrowHistory(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Upsert("payments", "PAY_1", json.RawMessage(`{"id":"PAY_1"}`)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resource_history`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("history count = %d, want 0 for untracked resource", count)
	}
}
