// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestKlaviyoSchemaRepairLegacyTypedTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("testdata/klaviyo_legacy_typed_tables.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	owner := klaviyoStoreSession(time.Unix(1, 0).UTC())
	if err := platform.InitializeTenantMetadata(context.Background(), db, owner, "legacy-fixture"); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO campaign_messages(rowid,id,campaigns_id,data,synced_at,created,updated) VALUES (41,'old','parent','{"keep":true}','2026-01-01','2025-01-01','2025-12-31')`,
		`INSERT INTO lists(rowid,id,profiles_id,data,synced_at,created,name,opt_in_process,updated) VALUES (42,'old','parent','{"keep":true}','2026-01-01','2025-01-01','Legacy list','double_opt_in','2025-12-31')`,
		`INSERT INTO metrics(rowid,id,custom_metrics_id,data,synced_at,created,name,updated) VALUES (43,'old','parent','{"keep":true}','2026-01-01','2025-01-01','Legacy metric','2025-12-31')`,
		`INSERT INTO profiles(rowid,id,segments_id,data,synced_at,type) VALUES (44,'old','parent','{"keep":true}','2026-01-01','profile')`,
		`INSERT INTO tags(rowid,id,segments_id,data,synced_at,name) VALUES (45,'old','parent','{"keep":true}','2026-01-01','Legacy tag')`,
		`CREATE TABLE resources (id TEXT NOT NULL,resource_type TEXT NOT NULL,data JSON NOT NULL,synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(resource_type,id))`,
		`INSERT INTO resources VALUES ('old','metrics','{"generic":"unchanged"}','2026-01-01','2026-01-02')`,
		`CREATE TABLE sync_state (resource_type TEXT PRIMARY KEY,last_cursor TEXT,last_synced_at DATETIME,total_count INTEGER DEFAULT 0,last_attempt_complete INTEGER NOT NULL DEFAULT 0,scope_key TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO sync_state VALUES ('metrics','saved-cursor','2026-01-03',200,1,'tenant-scope')`,
		fmt.Sprintf(`PRAGMA user_version = %d`, StoreSchemaVersion),
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	for attempt := range 2 {
		s, err := OpenVerifiedWithContext(context.Background(), dbPath, owner)
		if err != nil {
			t.Fatalf("open %d: %v", attempt, err)
		}
		func() {
			defer s.Close()
			for i, tc := range []struct {
				table, resource, obsolete, extraSQL string
			}{
				{"campaign_messages", "campaign-messages", "campaigns_id", `created='2025-01-01' AND updated='2025-12-31'`},
				{"lists", "lists", "profiles_id", `created='2025-01-01' AND name='Legacy list' AND opt_in_process='double_opt_in' AND updated='2025-12-31'`},
				{"metrics", "metrics", "custom_metrics_id", `created='2025-01-01' AND name='Legacy metric' AND updated='2025-12-31'`},
				{"profiles", "profiles", "segments_id", `type='profile'`},
				{"tags", "tags", "segments_id", `name='Legacy tag'`},
			} {
				var kept int
				query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE rowid=? AND id='old' AND data='{"keep":true}' AND date(synced_at)='2026-01-01' AND %s`, tc.table, tc.extraSQL)
				if err := s.DB().QueryRow(query, 41+i).Scan(&kept); err != nil || kept != 1 {
					t.Fatalf("%s shared values: count=%d err=%v", tc.table, kept, err)
				}
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_xinfo(?) WHERE name=?`, tc.table, tc.obsolete).Scan(&kept); err != nil || kept != 0 {
					t.Fatalf("%s obsolete column: count=%d err=%v", tc.table, kept, err)
				}
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, "idx_"+tc.table+"_"+tc.obsolete).Scan(&kept); err != nil || kept != 0 {
					t.Fatalf("%s obsolete index: count=%d err=%v", tc.table, kept, err)
				}
				item := json.RawMessage(fmt.Sprintf(`{"id":"new-%d","attributes":{"name":"Current item"}}`, attempt))
				stored, extractFailures, typedFailures, err := s.UpsertBatchDetailed(tc.resource, []json.RawMessage{item})
				if err != nil || stored != 1 || extractFailures != 0 || typedFailures != 0 {
					t.Fatalf("%s upsert: stored=%d extract=%d typed=%d err=%v", tc.table, stored, extractFailures, typedFailures, err)
				}
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM `+tc.table+` WHERE id=?`, fmt.Sprintf("new-%d", attempt)).Scan(&kept); err != nil || kept != 1 {
					t.Fatalf("%s new typed row: count=%d err=%v", tc.table, kept, err)
				}
			}
			var kept int
			for _, query := range []string{
				`SELECT COUNT(*) FROM resources WHERE id='old' AND resource_type='metrics' AND data='{"generic":"unchanged"}' AND date(synced_at)='2026-01-01' AND date(updated_at)='2026-01-02'`,
				`SELECT COUNT(*) FROM sync_state WHERE resource_type='metrics' AND last_cursor='saved-cursor' AND date(last_synced_at)='2026-01-03' AND total_count=200 AND last_attempt_complete=1 AND scope_key='tenant-scope'`,
			} {
				if err := s.DB().QueryRow(query).Scan(&kept); err != nil || kept != 1 {
					t.Fatalf("generic row/checkpoint changed: count=%d err=%v", kept, err)
				}
			}
			if err := platform.VerifyTenantMetadataReadOnly(context.Background(), s.DB(), owner); err != nil {
				t.Fatalf("tenant metadata changed: %v", err)
			}
		}()
	}
}

func TestKlaviyoSchemaRepairDetection(t *testing.T) {
	for _, tc := range []struct {
		name, declaration string
		wantRepair        bool
	}{
		{"obsolete required column", `TEXT NOT NULL`, true},
		{"obsolete nullable column", `TEXT`, false},
		{"obsolete column with default", `TEXT NOT NULL DEFAULT 'legacy'`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "data.db")
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`CREATE TABLE lists_profiles (id TEXT PRIMARY KEY, lists_id TEXT NOT NULL, data JSON NOT NULL, synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				bare_id TEXT GENERATED ALWAYS AS (substr(id,1,coalesce(nullif(instr(id,char(0)),0)-1,length(id)))) VIRTUAL, legacy_scope ` + tc.declaration + `);
				CREATE INDEX idx_lists_profiles_legacy_scope ON lists_profiles(legacy_scope);
				INSERT INTO lists_profiles(rowid,id,lists_id,data,legacy_scope) VALUES (71,'profile','list','{}','legacy')`)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var columns int
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_xinfo('lists_profiles') WHERE name='legacy_scope'`).Scan(&columns); err != nil {
				t.Fatal(err)
			}
			if (columns == 0) != tc.wantRepair {
				t.Fatalf("legacy_scope columns=%d wantRepair=%v", columns, tc.wantRepair)
			}
			var bareID, parent string
			if err := s.DB().QueryRow(`SELECT bare_id,lists_id FROM lists_profiles WHERE rowid=71`).Scan(&bareID, &parent); err != nil || bareID != "profile" || parent != "list" {
				t.Fatalf("generated/shared values: bare=%q parent=%q err=%v", bareID, parent, err)
			}
			for _, index := range []string{"idx_lists_profiles_lists_id", "idx_lists_profiles_parent_id", "idx_lists_profiles_bare_id"} {
				var count int
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&count); err != nil || count != 1 {
					t.Fatalf("current index %s: count=%d err=%v", index, count, err)
				}
			}
			if _, err := s.DB().Exec(`INSERT INTO lists_profiles(id,data) VALUES ('missing-parent','{}')`); err == nil {
				t.Fatal("current required parent constraint was removed")
			}
		})
	}
}

func TestKlaviyoSchemaRepairRollsBack(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE lists(id TEXT PRIMARY KEY, profiles_id TEXT NOT NULL, data JSON NOT NULL);
		CREATE INDEX idx_lists_profiles_id ON lists(profiles_id);
		INSERT INTO lists VALUES ('keep','parent','{}');
		CREATE TABLE metrics(id TEXT PRIMARY KEY, custom_metrics_id TEXT NOT NULL, data JSON);
		INSERT INTO metrics VALUES ('invalid','parent',NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	if s, err := Open(dbPath); err == nil {
		s.Close()
		t.Fatal("migration accepted NULL data for a current NOT NULL column")
	} else if !strings.Contains(err.Error(), "NOT NULL constraint failed") {
		t.Fatalf("unexpected migration error: %v", err)
	}
	var count int
	for _, query := range []string{
		`SELECT COUNT(*) FROM lists WHERE id='keep' AND profiles_id='parent' AND data='{}'`,
		`SELECT COUNT(*) FROM metrics WHERE id='invalid' AND custom_metrics_id='parent' AND data IS NULL`,
		`SELECT COUNT(*) FROM sqlite_master WHERE name='idx_lists_profiles_id'`,
	} {
		if err := db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("rollback lost old schema/data: count=%d err=%v", count, err)
		}
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '__pp_schema_repair_%'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback left replacement tables: count=%d err=%v", count, err)
	}
}
