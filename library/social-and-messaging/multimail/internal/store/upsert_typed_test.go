// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestUpsertTyped_ColumnOrdering is a table-driven test covering ALL typed
// upsert functions. It verifies that the SQL binding arguments match the
// declared column order — specifically that:
//   - FK columns contain the FK value (not a JSON blob)
//   - data columns contain valid JSON (not a timestamp or FK value)
//   - synced_at columns parse as a timestamp (not a JSON blob or FK value)
//
// This test was written to catch the argument-order bug in 9 upsert functions
// (verify, not_spam, report_spam, mailboxes_emails, reply, request_upgrade,
// send, threads, upgrade) where the SQL declared columns as (id, <fk>, data,
// synced_at) but the Go arguments were passed as (id, data, synced_at, fk).
func TestUpsertTyped_ColumnOrdering(t *testing.T) {
	type testCase struct {
		name       string               // human-readable name
		upsert     func(s *Store) error // calls the exported Upsert method
		table      string               // typed table name
		fkColumn   string               // FK or extra column to verify (empty if none to check)
		fkValue    string               // expected FK value
		hasDataCol bool                  // whether to verify the data column
	}

	const testID = "test-upsert-001"
	const testFK = "test-fk-123"

	cases := []testCase{
		{
			name: "account",
			upsert: func(s *Store) error {
				return s.UpsertAccount(json.RawMessage(fmt.Sprintf(`{"id": %q, "name": %q}`, testID, testFK)))
			},
			table:      "account",
			fkColumn:   "name",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "admin",
			upsert: func(s *Store) error {
				return s.UpsertAdmin(json.RawMessage(fmt.Sprintf(`{"id": %q, "api_key": %q}`, testID, testFK)))
			},
			table:      "admin",
			fkColumn:   "api_key",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "billing",
			upsert: func(s *Store) error {
				return s.UpsertBilling(json.RawMessage(fmt.Sprintf(`{"id": %q, "api_key": %q}`, testID, testFK)))
			},
			table:      "billing",
			fkColumn:   "api_key",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "domains",
			upsert: func(s *Store) error {
				return s.UpsertDomains(json.RawMessage(fmt.Sprintf(`{"id": %q, "domain": %q}`, testID, testFK)))
			},
			table:      "domains",
			fkColumn:   "domain",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "verify",
			upsert: func(s *Store) error {
				return s.UpsertVerify(json.RawMessage(fmt.Sprintf(`{"id": %q, "domains_id": %q}`, testID, testFK)))
			},
			table:      "verify",
			fkColumn:   "domains_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "not_spam",
			upsert: func(s *Store) error {
				return s.UpsertNotSpam(json.RawMessage(fmt.Sprintf(`{"id": %q, "emails_id": %q}`, testID, testFK)))
			},
			table:      "not_spam",
			fkColumn:   "emails_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "report_spam",
			upsert: func(s *Store) error {
				return s.UpsertReportSpam(json.RawMessage(fmt.Sprintf(`{"id": %q, "emails_id": %q}`, testID, testFK)))
			},
			table:      "report_spam",
			fkColumn:   "emails_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "mailboxes_emails",
			upsert: func(s *Store) error {
				return s.UpsertMailboxesEmails(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q, "parent_id": "parent-abc"}`, testID, testFK)))
			},
			table:      "mailboxes_emails",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "reply",
			upsert: func(s *Store) error {
				return s.UpsertReply(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q}`, testID, testFK)))
			},
			table:      "reply",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "request_upgrade",
			upsert: func(s *Store) error {
				return s.UpsertRequestUpgrade(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q}`, testID, testFK)))
			},
			table:      "request_upgrade",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "send",
			upsert: func(s *Store) error {
				return s.UpsertSend(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q}`, testID, testFK)))
			},
			table:      "send",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "threads",
			upsert: func(s *Store) error {
				return s.UpsertThreads(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q}`, testID, testFK)))
			},
			table:      "threads",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "upgrade",
			upsert: func(s *Store) error {
				return s.UpsertUpgrade(json.RawMessage(fmt.Sprintf(`{"id": %q, "mailboxes_id": %q}`, testID, testFK)))
			},
			table:      "upgrade",
			fkColumn:   "mailboxes_id",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "operator",
			upsert: func(s *Store) error {
				return s.UpsertOperator(json.RawMessage(fmt.Sprintf(`{"id": %q, "max_tier": %q}`, testID, testFK)))
			},
			table:      "operator",
			fkColumn:   "max_tier",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "slug_check",
			upsert: func(s *Store) error {
				return s.UpsertSlugCheck(json.RawMessage(fmt.Sprintf(`{"id": %q, "slug": %q}`, testID, testFK)))
			},
			table:      "slug_check",
			fkColumn:   "slug",
			fkValue:    testFK,
			hasDataCol: true,
		},
		{
			name: "webhooks",
			upsert: func(s *Store) error {
				return s.UpsertWebhooks(json.RawMessage(fmt.Sprintf(`{"id": %q, "url": %q}`, testID, testFK)))
			},
			table:      "webhooks",
			fkColumn:   "url",
			fkValue:    testFK,
			hasDataCol: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "data.db")
			s, err := Open(dbPath)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer s.Close()

			if err := tc.upsert(s); err != nil {
				t.Fatalf("Upsert%s: %v", tc.name, err)
			}

			db := s.DB()

			// 1. Verify the FK/extra column contains the expected FK value,
			//    not a JSON blob or timestamp.
			if tc.fkColumn != "" {
				var fkVal string
				q := fmt.Sprintf(`SELECT "%s" FROM "%s" WHERE id = ?`, tc.fkColumn, tc.table)
				if err := db.QueryRow(q, testID).Scan(&fkVal); err != nil {
					t.Fatalf("SELECT %s: %v", tc.fkColumn, err)
				}
				if fkVal != tc.fkValue {
					t.Fatalf("%s column = %q, want %q (argument order bug: FK column got wrong value)", tc.fkColumn, fkVal, tc.fkValue)
				}
			}

			// 2. Verify the data column contains valid JSON.
			if tc.hasDataCol {
				var dataVal string
				q := fmt.Sprintf(`SELECT data FROM "%s" WHERE id = ?`, tc.table)
				if err := db.QueryRow(q, testID).Scan(&dataVal); err != nil {
					t.Fatalf("SELECT data: %v", err)
				}
				if !json.Valid([]byte(dataVal)) {
					t.Fatalf("data column is not valid JSON: %q (argument order bug: data column got wrong value)", dataVal)
				}
			}

			// 3. Verify the synced_at column parses as a timestamp.
			{
				var syncedAt string
				q := fmt.Sprintf(`SELECT synced_at FROM "%s" WHERE id = ?`, tc.table)
				if err := db.QueryRow(q, testID).Scan(&syncedAt); err != nil {
					t.Fatalf("SELECT synced_at: %v", err)
				}
				// SQLite stores time.Time via the Go driver in several
				// possible formats. Try the common ones.
				parsed := false
				for _, layout := range []string{
					time.RFC3339Nano,
					time.RFC3339,
					"2006-01-02 15:04:05.999999999-07:00",
					"2006-01-02 15:04:05-07:00",
					"2006-01-02T15:04:05.999999999Z07:00",
					"2006-01-02T15:04:05Z07:00",
				} {
					if _, err := time.Parse(layout, syncedAt); err == nil {
						parsed = true
						break
					}
				}
				if !parsed {
					t.Fatalf("synced_at column does not parse as a timestamp: %q (argument order bug: synced_at got wrong value)", syncedAt)
				}
			}
		})
	}
}

// TestUpsertTyped_MailboxesEmails_ParentID verifies that the parent_id column
// is correctly populated for the 9th FK-bearing function (library-only).
func TestUpsertTyped_MailboxesEmails_ParentID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const testID = "me-parent-test-001"
	const testMbxID = "mbx-999"
	const testParentID = "parent-xyz"

	if err := s.UpsertMailboxesEmails(json.RawMessage(fmt.Sprintf(
		`{"id": %q, "mailboxes_id": %q, "parent_id": %q}`, testID, testMbxID, testParentID,
	))); err != nil {
		t.Fatalf("UpsertMailboxesEmails: %v", err)
	}

	db := s.DB()

	// Verify mailboxes_id
	var mbxVal string
	if err := db.QueryRow(`SELECT mailboxes_id FROM mailboxes_emails WHERE id = ?`, testID).Scan(&mbxVal); err != nil {
		t.Fatalf("SELECT mailboxes_id: %v", err)
	}
	if mbxVal != testMbxID {
		t.Fatalf("mailboxes_id = %q, want %q", mbxVal, testMbxID)
	}

	// Verify parent_id
	var parentVal string
	if err := db.QueryRow(`SELECT parent_id FROM mailboxes_emails WHERE id = ?`, testID).Scan(&parentVal); err != nil {
		t.Fatalf("SELECT parent_id: %v", err)
	}
	if parentVal != testParentID {
		t.Fatalf("parent_id = %q, want %q", parentVal, testParentID)
	}

	// Verify data is valid JSON
	var dataVal string
	if err := db.QueryRow(`SELECT data FROM mailboxes_emails WHERE id = ?`, testID).Scan(&dataVal); err != nil {
		t.Fatalf("SELECT data: %v", err)
	}
	if !json.Valid([]byte(dataVal)) {
		t.Fatalf("data column is not valid JSON: %q", dataVal)
	}
}
