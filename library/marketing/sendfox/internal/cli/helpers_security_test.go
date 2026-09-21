package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHumanFriendlyRedirectedOutputDoesNotContainANSI(t *testing.T) {
	withTempLearnHome(t)
	t.Setenv("SENDFOX_API_TOKEN", "")
	t.Setenv("SENDFOX_BEARER_AUTH", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")

	stdout, _, err := runRootArgs(t, "auth", "status", "--human-friendly")
	if err == nil {
		t.Fatal("auth status without credentials unexpectedly succeeded")
	}
	if !strings.Contains(stdout, "Not authenticated") {
		t.Fatalf("auth output missing status: %q", stdout)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("redirected human-friendly output contains ANSI escapes: %q", stdout)
	}
}

func TestDefaultDBPathInDirNeverSharesLegacyDatabaseAcrossCredentialScopes(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "data.db")
	if err := os.WriteFile(legacyPath, []byte("legacy"), 0o600); err != nil {
		t.Fatalf("write legacy database: %v", err)
	}
	t.Cleanup(func() { setDefaultDBScopeCredential("") })

	setDefaultDBScopeCredential("token-for-account-a")
	accountAPath := defaultDBPathInDir(dir)
	if accountAPath == legacyPath {
		t.Fatal("credential-scoped storage reused the legacy unscoped database")
	}

	setDefaultDBScopeCredential("token-for-account-b")
	accountBPath := defaultDBPathInDir(dir)
	if accountBPath == legacyPath {
		t.Fatal("second credential scope reused the legacy unscoped database")
	}
	if accountAPath == accountBPath {
		t.Fatalf("different credentials resolved to the same database: %s", accountAPath)
	}

	setDefaultDBScopeCredential("")
	if got := defaultDBPathInDir(dir); got != legacyPath {
		t.Fatalf("unscoped storage path = %s, want %s", got, legacyPath)
	}
}
