// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestPlatformMigrationIsVerifiedIdempotentAndQuarantinesLegacyState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")
	legacyDB := filepath.Join(t.TempDir(), "legacy.db")
	if err := os.WriteFile(legacyDB, []byte("synthetic legacy fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(t.TempDir(), "migration-receipt.json")
	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = platformCLITestRegistration(platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-a"}})
	platformResolverFactory = func() platform.CredentialResolver {
		return platformCLIResolver{value: []byte("migration-secret-never-print")}
	}
	args := []string{"client", "migrate", "tenant-a", "--credential-ref", "op://Test Vault/Test Item/token", "--expected-account-id", "acct-a", "--legacy-db", legacyDB, "--receipt-file", receiptPath, "--json"}
	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"status": "quarantined"`) || !strings.Contains(output.String(), `"profile_written": true`) {
		t.Fatalf("migration output = %s", output.String())
	}
	if _, err := os.Stat(legacyDB); err != nil {
		t.Fatalf("legacy fixture was not preserved: %v", err)
	}
	profile, err := platform.LoadProfile("tenant-a")
	if err != nil || profile.Sources["test-source"].ExpectedAccountID != "acct-a" {
		t.Fatalf("migrated profile = %#v err=%v", profile, err)
	}
	receiptData, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt platform.Receipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "success" || receipt.Client.Gate != platform.GateVerified || strings.Contains(string(receiptData), "migration-secret-never-print") {
		t.Fatalf("migration receipt = %s", receiptData)
	}
	if receipt.RateLimit.Requests != 1 || !strings.Contains(string(receiptData), `"wait_ms"`) {
		t.Fatalf("migration receipt did not record the live gate rate metadata: %s", receiptData)
	}
	repeat := RootCmd()
	output.Reset()
	repeat.SetOut(&output)
	repeat.SetErr(&output)
	repeat.SetArgs(args)
	if err := repeat.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"already_migrated": true`) || !strings.Contains(output.String(), `"profile_written": false`) {
		t.Fatalf("idempotent migration output = %s", output.String())
	}
	conflict := RootCmd()
	conflict.SetArgs([]string{"client", "migrate", "tenant-a", "--credential-ref", "op://Test Vault/Test Item/token", "--expected-account-id", "acct-b"})
	if err := conflict.Execute(); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("conflicting migration error = %v", err)
	}
}

func TestPlatformMigrationWrongTenantCannotCreateProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")

	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = platformCLITestRegistration(platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-b"}})
	platformResolverFactory = func() platform.CredentialResolver {
		return platformCLIResolver{value: []byte("wrong-tenant-credential")}
	}

	root := RootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"client", "migrate", "tenant-rejected", "--credential-ref", "op://Test Vault/Test Item/token", "--expected-account-id", "acct-a", "--json"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "tenant gate mismatch") {
		t.Fatalf("wrong-tenant migration error = %v", err)
	}
	if _, err := platform.LoadProfile("tenant-rejected"); !os.IsNotExist(err) {
		t.Fatalf("wrong-tenant migration created a profile: %v", err)
	}
	paths, err := platform.PathsFor("tenant-rejected", "klaviyo-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.DataFile); !os.IsNotExist(err) {
		t.Fatalf("wrong-tenant migration created a database: %v", err)
	}
}

func TestPlatformMigrationAdoptsOnlyVerifiedTenantDatabase(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")

	legacyDB := filepath.Join(t.TempDir(), "verified-legacy.db")
	db, err := sql.Open("sqlite", legacyDB)
	if err != nil {
		t.Fatal(err)
	}
	legacySession := &platform.Session{
		ProfileName: "tenant-adopt", Source: "test-source", GateOutcome: platform.GateVerified,
		ExpectedIdentity: map[string]string{"account_id": "acct-a"}, ObservedIdentity: map[string]string{"account_id": "acct-a"},
		CredentialFingerprint: "legacy-fingerprint", GateCheckedAt: time.Unix(1, 0).UTC(),
	}
	if err := platform.InitializeTenantMetadata(context.Background(), db, legacySession, "synthetic-legacy-fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE migration_fixture (value TEXT NOT NULL); INSERT INTO migration_fixture VALUES ('preserved')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = platformCLITestRegistration(platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-a"}})
	platformResolverFactory = func() platform.CredentialResolver { return platformCLIResolver{value: []byte("new-credential")} }

	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	migrationArgs := []string{"client", "migrate", "tenant-adopt", "--credential-ref", "op://Test Vault/Test Item/token", "--expected-account-id", "acct-a", "--legacy-db", legacyDB, "--json"}
	root.SetArgs(migrationArgs)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	statusAdopted := strings.Contains(output.String(), `"status": "adopted"`) || strings.Contains(output.String(), `"status":"adopted"`)
	copied := strings.Contains(output.String(), `"copied": true`) || strings.Contains(output.String(), `"copied":true`)
	if !statusAdopted || !copied {
		t.Fatalf("verified migration output = %s", output.String())
	}
	cleanup := RootCmd()
	output.Reset()
	cleanup.SetOut(&output)
	cleanup.SetErr(&output)
	cleanup.SetArgs(append(append([]string(nil), migrationArgs...), "--cleanup-legacy"))
	if err := cleanup.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacyDB); !os.IsNotExist(err) {
		t.Fatalf("explicit cleanup left the legacy database at its active path: %v", err)
	}
	backupPath := legacyDB + ".printing-press-backup"
	backupInfo, err := os.Stat(backupPath)
	if err != nil || backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("recoverable backup info=%v err=%v", backupInfo, err)
	}
	repeatCleanup := RootCmd()
	repeatCleanup.SetOut(&bytes.Buffer{})
	repeatCleanup.SetErr(&bytes.Buffer{})
	repeatCleanup.SetArgs(append(append([]string(nil), migrationArgs...), "--cleanup-legacy"))
	if err := repeatCleanup.Execute(); err != nil {
		t.Fatalf("idempotent cleanup = %v", err)
	}
	paths, err := platform.PathsFor("tenant-adopt", "klaviyo-pp-cli", "test-source")
	if err != nil {
		t.Fatal(err)
	}
	migrated, err := sql.Open("sqlite", "file:"+paths.DataFile+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	var value string
	if err := migrated.QueryRow(`SELECT value FROM migration_fixture`).Scan(&value); err != nil || value != "preserved" {
		t.Fatalf("migrated fixture value=%q err=%v", value, err)
	}
	if err := platform.VerifyTenantMetadataReadOnly(context.Background(), migrated, &platform.Session{
		ProfileName: "tenant-adopt", Source: "test-source", GateOutcome: platform.GateVerified,
		ExpectedIdentity: map[string]string{"account_id": "acct-a"}, ObservedIdentity: map[string]string{"account_id": "acct-a"},
		CredentialFingerprint: "ignored-for-read-only-verification", GateCheckedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("migrated metadata = %v", err)
	}
}
