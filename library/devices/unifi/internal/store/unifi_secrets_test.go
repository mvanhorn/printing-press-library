// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
)

func secretsEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	return filepath.Join(home, "data", "m.db")
}

func filesWithout(t *testing.T, dir, secret string) {
	t.Helper()
	checked := 0
	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("%s still holds the secret", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no files checked")
	}
}

// Old rows must not survive, including in freed pages, the WAL and the FTS index, or
// upgrading would leave every passphrase already synced sitting in the file.
func TestMigrationSealsExistingSecrets(t *testing.T) {
	dbPath := secretsEnv(t)
	const psk = "legacy-plaintext-psk-Hx2"
	ctx := context.Background()
	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"id":"w1","name":"guest","securityConfiguration":{"passphrase":"` + psk + `"}}`
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO resources (id, resource_type, data, synced_at, updated_at) VALUES ('w1','network_integration_v1_sites_wifi_broadcasts',?,'t','t')`, []any{raw}},
		{`INSERT INTO resources_fts (rowid, id, resource_type, content) VALUES (42,'w1','network_integration_v1_sites_wifi_broadcasts',?)`, []any{"guest " + psk}},
		{`INSERT INTO "network" ("id","data","synced_at") VALUES ('w1',?,'t')`, []any{raw}},
		{`INSERT INTO unifi_snapshots (id, console_id, taken_at) VALUES (1,'C','2026-01-01T00:00:00Z')`, nil},
		{`INSERT INTO unifi_snapshot_rows (snapshot_id, resource_type, entity_id, data) VALUES (1,'wifi','w1',?)`, []any{raw}},
		{`INSERT INTO unifi_change_journal (id, console_id, created_at, updated_at, state, kind, method, path, before_image) VALUES ('c1','C','t','t','verified','update','PUT','/x',?)`, []any{raw}},
	} {
		if _, err := db.DB().ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.query, err)
		}
	}
	if _, err := db.DB().ExecContext(ctx, `PRAGMA user_version = 14`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var data string
	if err := db.DB().QueryRowContext(ctx, `SELECT data FROM unifi_snapshot_rows`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "[secret:v1:") {
		t.Fatalf("no digest marker: %s", data)
	}
	var matches int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM resources_fts WHERE content LIKE ?`, "%"+psk+"%").Scan(&matches); err != nil {
		t.Fatal(err)
	}
	if matches != 0 {
		t.Fatal("FTS still indexes the secret")
	}
	if rows, err := db.Search(psk, 10); err == nil && len(rows) != 0 {
		t.Fatalf("search matched secret text: %s", rows[0])
	}
	db.Close()
	filesWithout(t, filepath.Dir(dbPath), psk)
}

// The read-only path is what MCP sql and search use, and it runs no migrations, so an
// older file would otherwise serve plaintext until something opened it for writing.
func TestReadOnlyOpenDoesNotServeUnmigratedSecrets(t *testing.T) {
	dbPath := secretsEnv(t)
	const psk = "legacy-plaintext-psk-Ro5"
	ctx := context.Background()
	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `INSERT INTO resources (id, resource_type, data, synced_at, updated_at) VALUES ('w1','x',?,'t','t')`, `{"id":"w1","passphrase":"`+psk+`"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().ExecContext(ctx, `PRAGMA user_version = 14`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	ro, err := OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	var data string
	if err := ro.DB().QueryRow(`SELECT data FROM resources`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(data, psk) {
		t.Fatalf("read-only open served plaintext: %s", data)
	}
}

// The salt lives outside the data directory, so a copied database cannot be attacked
// offline, and it differs per install, so digests cannot be compared across installs.
func TestSaltIsPrivatePerInstallAndOutsideTheDB(t *testing.T) {
	dbPath := secretsEnv(t)
	ctx := context.Background()
	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := json.Marshal(map[string]any{"id": "w1", "passphrase": "same-psk-Bv1"})
	if _, _, err := db.UpsertBatch("network", []json.RawMessage{item}); err != nil {
		t.Fatal(err)
	}
	stored, err := db.Get("network", "w1")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	state, err := cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(state, "mirror-secret-salt"))
	if len(matches) != 1 {
		t.Fatalf("salt not in the state dir %s", state)
	}
	info, err := os.Stat(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("salt mode %o", info.Mode().Perm())
	}
	if rel, err := filepath.Rel(filepath.Dir(dbPath), matches[0]); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("salt sits beside the database: %s", matches[0])
	}
	salt, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	filesWithout(t, filepath.Dir(dbPath), strings.TrimSpace(string(salt)))
	second := secretsEnv(t)
	db2, err := OpenWithContext(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if _, _, err := db2.UpsertBatch("network", []json.RawMessage{item}); err != nil {
		t.Fatal(err)
	}
	other, err := db2.Get("network", "w1")
	if err != nil {
		t.Fatal(err)
	}
	if digestOf(t, stored) == digestOf(t, other) {
		t.Fatal("two installs produced the same digest")
	}
}

func digestOf(t *testing.T, row json.RawMessage) string {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(row, &obj); err != nil {
		t.Fatal(err)
	}
	value, _ := obj["passphrase"].(string)
	if !strings.HasPrefix(value, "[secret:v1:") {
		t.Fatalf("not sealed: %s", row)
	}
	return value
}

// Every writer has to seal, not just the batch path sync happens to use today.
func TestEveryUpsertEntryPointSeals(t *testing.T) {
	dbPath := secretsEnv(t)
	const psk = "entrypoint-psk-Kj8"
	ctx := context.Background()
	db, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	item := json.RawMessage(`{"id":"e1","passphrase":"` + psk + `","name":"n"}`)
	if err := db.Upsert("network", "e1", item); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertNetwork(item); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertProtect(item); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertHosts(item); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertJournalEntry(ctx, JournalEntry{ID: "j1", ConsoleID: "C", CreatedAt: "t", UpdatedAt: "t", State: "pending", Kind: "update", Method: "PUT", Path: "/x", BeforeImage: item}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SaveSnapshot(ctx, "C", time.Now(), []SnapshotRow{{ResourceType: "wifi", EntityID: "e1", Data: item}}, nil, nil); err != nil {
		t.Fatal(err)
	}
	db.Close()
	filesWithout(t, filepath.Dir(dbPath), psk)
}

// Drift compares digests, so a passphrase that did not change must produce the same bytes
// on every sync, through a re-seal, and against a row sealed by the migration. Otherwise
// every sync, and the first sync after upgrading, would report a false secret change.
func TestSealingIsStableAcrossSyncs(t *testing.T) {
	secretsEnv(t)
	row := []byte(`{"id":"w1","passphrase":"same-psk-Sx4","nested":[{"presharedKeys":[{"passphrase":"p2"}]}]}`)
	first := sealStoredJSON(row)
	if bytes.Contains(first, []byte("same-psk-Sx4")) || bytes.Contains(first, []byte("p2")) {
		t.Fatalf("not sealed: %s", first)
	}
	if again := sealStoredJSON(row); !bytes.Equal(first, again) {
		t.Fatalf("two syncs differ:\n%s\n%s", first, again)
	}
	if resealed := sealStoredJSON(first); !bytes.Equal(first, resealed) {
		t.Fatalf("sealing is not idempotent:\n%s\n%s", first, resealed)
	}
	changed := sealStoredJSON([]byte(`{"id":"w1","passphrase":"rotated-Sx5","nested":[{"presharedKeys":[{"passphrase":"p2"}]}]}`))
	if bytes.Equal(first, changed) {
		t.Fatal("a rotated secret produced the same digest")
	}
	clean := []byte(`{"name":"a","big":12345678901234567890,"z":1,"b":2}`)
	if got := sealStoredJSON(clean); !bytes.Equal(got, clean) {
		t.Fatalf("a row with no secret was rewritten: %s", got)
	}
}
