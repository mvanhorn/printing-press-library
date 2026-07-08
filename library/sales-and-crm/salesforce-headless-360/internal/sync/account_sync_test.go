package sync

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	sfmock "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/testdata/salesforce-mock"
)

func TestSyncAccountRateLimitReturnsPartialAndPreservesCursor(t *testing.T) {
	server := sfmock.Start(t)
	server.SetFailMode(sfmock.FailRateLimit)
	c := client.New(&config.Config{
		BaseURL:               server.URL,
		SalesforceInstanceUrl: server.URL,
		AccessToken:           "test-token",
	}, 5*time.Second, 0)

	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()
	if err := db.SaveSyncState("accounts", "resume-cursor", 12); err != nil {
		t.Fatalf("seed sync state: %v", err)
	}

	_, err = SyncAccount(c, db, "001ACME0001", time.Time{}, NoopFilter(), NewGate())
	if !IsPartialSync(err) || !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("err = %v, want partial budget error", err)
	}
	cursor, _, count, err := db.GetSyncState("accounts")
	if err != nil {
		t.Fatalf("get sync state: %v", err)
	}
	if cursor != "resume-cursor" || count != 12 {
		t.Fatalf("sync state = cursor %q count %d, want preserved cursor/count", cursor, count)
	}
}
