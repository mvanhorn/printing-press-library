// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	_ "modernc.org/sqlite"
)

func TestKlaviyoProviderNativeLegacyMigrationRequiresExplicitEvidenceAdoption(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")

	legacyConfig, err := klaviyoAccountConfigPath("legacy-fixture")
	if err != nil {
		t.Fatal(err)
	}
	legacyConfigData := []byte("base_url = \"https://a.klaviyo.com\"\napi_key = \"synthetic-never-copy\"\n")
	if err := os.WriteFile(legacyConfig, legacyConfigData, 0o600); err != nil {
		t.Fatal(err)
	}
	legacyDB := filepath.Join(t.TempDir(), "klaviyo.db")
	db, err := sql.Open("sqlite", legacyDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE resources (id TEXT PRIMARY KEY, resource_type TEXT NOT NULL, data BLOB NOT NULL);
		INSERT INTO resources VALUES ('account', 'account', '{"id":"acct-fixture","attributes":{"contact_information":{"organization_name":"Fixture Org"}}}')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = &platformSourceRegistration{
		Source:                 "klaviyo",
		Adapter:                platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-fixture", OrgName: "Fixture Org"}},
		ValidateSourceProfile:  validateKlaviyoSourceProfile,
		PrepareLegacyMigration: prepareKlaviyoLegacyMigration,
		LegacyDatabaseEvidence: inspectKlaviyoLegacyDatabase,
	}
	platformResolverFactory = func() platform.CredentialResolver {
		return platformCLIResolver{value: []byte("resolved-synthetic-key")}
	}

	args := []string{"client", "migrate", "klaviyo-native", "--legacy-account", "legacy-fixture", "--legacy-db", legacyDB, "--credential-ref", "op://Test Vault/Klaviyo/key", "--expected-account-id", "acct-fixture", "--expected-org-name", "Fixture Org", "--json"}
	first := RootCmd()
	first.SetOut(&bytes.Buffer{})
	first.SetErr(&bytes.Buffer{})
	first.SetArgs(args)
	if err := first.Execute(); err == nil || !strings.Contains(err.Error(), "--adopt-verified-legacy-db") {
		t.Fatalf("migration without explicit evidence adoption error = %v", err)
	}
	if got, err := os.ReadFile(legacyConfig); err != nil || !bytes.Equal(got, legacyConfigData) {
		t.Fatalf("named legacy account changed before confirmation: err=%v", err)
	}
	if _, err := os.Stat(legacyDB); err != nil {
		t.Fatalf("legacy database was not preserved: %v", err)
	}
	paths, err := platform.PathsFor("klaviyo-native", "klaviyo-pp-cli", "klaviyo")
	if err != nil {
		t.Fatal(err)
	}
	if countKlaviyoMigrationReceipts(t, paths.ReceiptsDir) != 1 {
		t.Fatal("failed migration did not write its mandatory default receipt")
	}
	if _, err := os.Stat(paths.DataFile); !os.IsNotExist(err) {
		t.Fatalf("database was adopted without explicit confirmation: %v", err)
	}

	confirmedArgs := append(append([]string(nil), args...), "--adopt-verified-legacy-db")
	confirmed := RootCmd()
	confirmed.SetOut(&bytes.Buffer{})
	confirmed.SetErr(&bytes.Buffer{})
	confirmed.SetArgs(confirmedArgs)
	if err := confirmed.Execute(); err != nil {
		t.Fatal(err)
	}
	if countKlaviyoMigrationReceipts(t, paths.ReceiptsDir) != 2 {
		t.Fatal("successful migration did not write its mandatory default receipt")
	}
	migrated, err := sql.Open("sqlite", "file:"+paths.DataFile+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	session := &platform.Session{
		ProfileName: "klaviyo-native", Source: "klaviyo", GateOutcome: platform.GateVerified,
		ExpectedIdentity: map[string]string{"account_id": "acct-fixture", "org_name": "Fixture Org"},
		ObservedIdentity: map[string]string{"account_id": "acct-fixture", "org_name": "Fixture Org"},
	}
	if err := platform.VerifyTenantMetadataReadOnly(context.Background(), migrated, session); err != nil {
		t.Fatalf("adopted Klaviyo database metadata = %v", err)
	}

	cleanup := RootCmd()
	cleanup.SetOut(&bytes.Buffer{})
	cleanup.SetErr(&bytes.Buffer{})
	cleanup.SetArgs(append(confirmedArgs, "--cleanup-legacy"))
	if err := cleanup.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{legacyConfig, legacyDB} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("explicit cleanup left active legacy artifact %q: %v", path, err)
		}
		if info, err := os.Stat(path + ".printing-press-backup"); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("recoverable backup for %q info=%v err=%v", path, info, err)
		}
	}
}

func TestKlaviyoLegacyMigrationRejectsProfilePathCollision(t *testing.T) {
	paths := platform.Paths{ProfileFile: filepath.Join(t.TempDir(), "profile.toml")}
	state := &platform.LegacyMigrationState{Config: &platform.LegacyMigrationArtifact{Path: paths.ProfileFile}}
	if err := platform.ValidateLegacyMigrationPaths(&platform.Session{Paths: paths}, state); err == nil {
		t.Fatal("profile-owned legacy config path was accepted")
	}
}

func countKlaviyoMigrationReceipts(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}
