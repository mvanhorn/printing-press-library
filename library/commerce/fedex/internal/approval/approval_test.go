// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package approval

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testRequest() map[string]any {
	return map[string]any{
		"accountNumber":     map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND"},
	}
}

func testMutation() Mutation {
	return Mutation{
		Action:  "create_label",
		Origin:  "https://apis-sandbox.fedex.com:443",
		Method:  "POST",
		Path:    "/ship/v1/shipments",
		Request: testRequest(),
	}
}

func TestStoreCreateAndConsumeBindsCanonicalWireOperation(t *testing.T) {
	now := time.Date(2026, 9, 2, 20, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir(), 10*time.Minute)
	store.now = func() time.Time { return now }
	mutation := testMutation()
	record, err := store.Create(mutation, ReviewSummary{AccountSuffix: "6789", ServiceType: "FEDEX_GROUND"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if record.ID == "" || record.ConfirmationDigest == "" || record.Status != StatusPending {
		t.Fatalf("invalid pending record: %#v", record)
	}

	mutation.Request = map[string]any{
		"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND"},
		"accountNumber":     map[string]any{"value": "123456789"},
	}
	consumed, permit, err := store.Consume(record.ID, record.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if consumed.Status != StatusExecuting || consumed.ConsumedAt == nil || !permit.Allows(mutation.Method, mutation.Path, mutation.Request) {
		t.Fatalf("invalid consumed record or permit: %#v", consumed)
	}
	if permit.Allows("PUT", mutation.Path, mutation.Request) || permit.Allows(mutation.Method, "/pickup/v1/pickups", mutation.Request) {
		t.Fatal("permit authorized a different wire operation")
	}
	if _, _, err := store.Consume(record.ID, record.ConfirmationDigest, mutation); !errors.Is(err, ErrAlreadyConsumed) {
		t.Fatalf("second Consume error=%v, want ErrAlreadyConsumed", err)
	}
}

func TestStoreRejectsModifiedExpiredForgedAndWrongOrigin(t *testing.T) {
	now := time.Date(2026, 9, 2, 20, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir(), time.Minute)
	store.now = func() time.Time { return now }
	mutation := testMutation()
	mutation.Action = "schedule_pickup"
	mutation.Method = "POST"
	mutation.Path = "/pickup/v1/pickups"
	record, err := store.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	modified := mutation
	modified.Request = testRequest()
	modified.Request.(map[string]any)["packageCount"] = 2
	if _, _, err := store.Consume(record.ID, record.ConfirmationDigest, modified); !errors.Is(err, ErrRequestMismatch) {
		t.Fatalf("modified request error=%v, want ErrRequestMismatch", err)
	}
	wrongOrigin := mutation
	wrongOrigin.Origin = "https://apis.fedex.com:443"
	if _, _, err := store.Consume(record.ID, record.ConfirmationDigest, wrongOrigin); !errors.Is(err, ErrRequestMismatch) {
		t.Fatalf("wrong origin error=%v, want ErrRequestMismatch", err)
	}
	if _, _, err := store.Consume(record.ID, "sha256:forged", mutation); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("forged digest error=%v, want ErrDigestMismatch", err)
	}
	now = now.Add(2 * time.Minute)
	if _, _, err := store.Consume(record.ID, record.ConfirmationDigest, mutation); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired request error=%v, want ErrExpired", err)
	}
}

func TestStorePersistsNoRequestOrLabelDataAndUsesPrivateModes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fedex", "pending")
	store := NewStore(root, 10*time.Minute)
	record, err := store.Create(testMutation(), ReviewSummary{AccountSuffix: "6789"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got := mustMode(t, root); got != 0o700 {
		t.Fatalf("pending dir mode=%#o, want 0700", got)
	}
	path := filepath.Join(root, record.ID+".json")
	if got := mustMode(t, path); got != 0o600 {
		t.Fatalf("pending record mode=%#o, want 0600", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"123456789", "requestedShipment", "FEDEX_GROUND"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("pending record contains request data %q: %s", forbidden, data)
		}
	}
}

func TestStoreConsumeIsAtomicAcrossConcurrentCallers(t *testing.T) {
	store := NewStore(t.TempDir(), 10*time.Minute)
	mutation := testMutation()
	mutation.Action = "cancel_shipment"
	mutation.Method = "PUT"
	mutation.Path = "/ship/v1/shipments/cancel"
	record, err := store.Create(mutation, ReviewSummary{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	const callers = 8
	var wg sync.WaitGroup
	wg.Add(callers)
	results := make(chan error, callers)
	for range callers {
		go func() {
			defer wg.Done()
			_, _, err := store.Consume(record.ID, record.ConfirmationDigest, mutation)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrAlreadyConsumed) && !errors.Is(err, ErrOperationBusy) {
			t.Fatalf("unexpected concurrent consume error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumers=%d, want 1", successes)
	}
}

func TestNormalizeOriginBindsExactOrigin(t *testing.T) {
	got, err := NormalizeOrigin("HTTPS://Apis-Sandbox.FedEx.com/")
	if err != nil || got != "https://apis-sandbox.fedex.com:443" {
		t.Fatalf("NormalizeOrigin=%q err=%v", got, err)
	}
	customA, _ := NormalizeOrigin("http://127.0.0.1:8080")
	customB, _ := NormalizeOrigin("http://127.0.0.1:8081")
	if customA == customB {
		t.Fatal("different custom origins normalized identically")
	}
}

func mustMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
