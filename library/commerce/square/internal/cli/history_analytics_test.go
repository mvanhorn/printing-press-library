package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/square/internal/store"
)

func TestLoadResourceHistoryAndMeaningfulFieldChanges(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Upsert("inventory", "VAR_1", json.RawMessage(`{"id":"VAR_1","quantity":"2","updated_at":"2026-08-01T00:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("inventory", "VAR_1", json.RawMessage(`{"id":"VAR_1","quantity":"5","updated_at":"2026-08-02T00:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	history, err := loadResourceHistory(context.Background(), db, []string{"inventory"}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %#v, want two versions", history)
	}
	changes := meaningfulFieldChanges(history[0].Data, history[1].Data)
	if len(changes) != 1 || changes[0].Field != "quantity" {
		t.Fatalf("changes = %#v, want only quantity (timestamps are non-business metadata)", changes)
	}
}

func TestLoadWebhookDeliveriesUsesReceiptLog(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "webhooks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	if _, err := db.RecordWebhookDelivery(store.WebhookDelivery{EventID: "evt-1", EventType: "payment.updated", OccurredAt: now.Add(-time.Second), ReceivedAt: now, Payload: json.RawMessage(`{"event_id":"evt-1"}`)}); err != nil {
		t.Fatal(err)
	}
	deliveries, err := loadWebhookDeliveriesSince(db, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 1 || deliveries[0].EventID != "evt-1" || deliveries[0].OccurredAt.IsZero() {
		t.Fatalf("deliveries = %#v", deliveries)
	}
}

func TestLoadResourceHistoryFallsBackForPreHistoryDatabase(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Upsert("catalog", "ITEM_1", json.RawMessage(`{"id":"ITEM_1","price":100}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`DROP TABLE resource_history`); err != nil {
		t.Fatal(err)
	}
	history, err := loadResourceHistory(context.Background(), db, []string{"catalog"}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ResourceID != "ITEM_1" {
		t.Fatalf("fallback history = %#v, want current catalog snapshot", history)
	}
}

func TestWebhookSecretRedactionUsesExactKeysRecursively(t *testing.T) {
	body, err := decodeWebhookBody([]byte(`{"event_id":"evt-1","data":{"access_token":"x","signature_key":"x","signing_key":"x","signing_secret":"x","secret_key":"x","refresh_token":"x","oauth_token":"x","bearer_token":"x","token_count":3},"authorization":"Bearer secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	redacted := redactWebhookSecrets(body).(map[string]any)
	if redacted["authorization"] != "<redacted>" {
		t.Fatalf("authorization was not redacted: %#v", redacted)
	}
	data := redacted["data"].(map[string]any)
	for _, key := range []string{"access_token", "signature_key", "signing_key", "signing_secret", "secret_key", "refresh_token", "oauth_token", "bearer_token"} {
		if data[key] != "<redacted>" {
			t.Fatalf("nested secret %s was not redacted: %#v", key, data)
		}
	}
	if data["token_count"].(json.Number).String() != "3" {
		t.Fatalf("non-secret similarly named key was changed: %#v", data)
	}
}

func TestInventoryHistoryRejectsMetadataRows(t *testing.T) {
	metadata := resourceHistoryRecord{ResourceType: "inventory", Data: map[string]any{"reasons": []any{"SOLD"}}}
	real := resourceHistoryRecord{ResourceType: "inventory", Data: map[string]any{"catalog_object_id": "VAR_1", "quantity": "2"}}
	if isRealInventoryHistory(metadata) {
		t.Fatal("adjustment-reason metadata was treated as inventory state")
	}
	if !isRealInventoryHistory(real) {
		t.Fatal("inventory count was not recognized")
	}
}

func TestWebhookIngestRejectsImplausibleFutureReceiptTime(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "webhook.json")
	if err := os.WriteFile(bodyPath, []byte(`{"event_id":"evt-1","type":"payment.updated"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelWebhookIngestCmd(&rootFlags{})
	future := time.Now().UTC().Add(maxWebhookFutureSkew + time.Hour).Format(time.RFC3339)
	cmd.SetArgs([]string{"--body", bodyPath, "--received-at", future})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "in the future") {
		t.Fatalf("error = %v, want future timestamp rejection", err)
	}
}
