package sync

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	sfmock "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/testdata/salesforce-mock"
)

func TestSharingCrossCheckDropsRestrictedContactsAndAudits(t *testing.T) {
	server := sfmock.Start(t)
	server.SetFailMode(sfmock.FailSharingRestricted)
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

	records := []json.RawMessage{
		json.RawMessage(`{"attributes":{"type":"Contact"},"Id":"003ACME9001","AccountId":"001RESTRICTED"}`),
		json.RawMessage(`{"attributes":{"type":"Contact"},"Id":"003RESTRICTED001","AccountId":"001RESTRICTED"}`),
	}
	visible, err := FilterVisibleRecords(c, db, "001RESTRICTED", "Contact", records, NewGate())
	if err != nil {
		t.Fatalf("filter visible records: %v", err)
	}
	if len(visible) != 1 {
		t.Fatalf("visible len = %d, want 1", len(visible))
	}
	if got := ExtractRecordID(visible[0]); got != "003ACME9001" {
		t.Fatalf("visible id = %q, want visible contact", got)
	}

	rows, err := db.Query(`SELECT sobject, sobject_id, reason, account_id FROM sharing_drop_audit`)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("expected sharing_drop_audit row")
	}
	var sobject, id, reason, accountID string
	if err := rows.Scan(&sobject, &id, &reason, &accountID); err != nil {
		t.Fatalf("scan audit: %v", err)
	}
	if sobject != "Contact" || id != "003RESTRICTED001" || reason != "ui_api_403" || accountID != "001RESTRICTED" {
		t.Fatalf("audit row = %s %s %s %s", sobject, id, reason, accountID)
	}
	if rows.Next() {
		t.Fatalf("expected one audit row")
	}
}

func TestSharingCrossCheckRateLimitReturnsPartial(t *testing.T) {
	client := &fakeHeaderGetClient{
		responses: map[string]json.RawMessage{
			"/services/data/v63.0/ui-api/records/003ACME0001": json.RawMessage(`{"id":"003ACME0001"}`),
		},
		headers: http.Header{"Sforce-Limit-Info": []string{"api-usage=81000/100000"}},
	}
	_, err := FilterVisibleRecords(client, nil, "001ACME0001", "Contact", []json.RawMessage{
		json.RawMessage(`{"Id":"003ACME0001"}`),
		json.RawMessage(`{"Id":"003ACME0002"}`),
	}, NewGate())
	if !IsPartialSync(err) {
		t.Fatalf("err = %v, want partial sync", err)
	}
}
