package cli

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
)

func TestCredentialSavePathReportsActualProfileFile(t *testing.T) {
	t.Setenv("GOVERNMENT_DATA_HOME", t.TempDir())
	t.Setenv("DATA_GOV_API_KEY", "")
	cfg, err := config.Load(filepath.Join(t.TempDir(), "profile.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveCredential("PROFILE-TEST-KEY"); err != nil {
		t.Fatal(err)
	}
	want, err := cliutil.CredentialsFilePathForConfig(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := credentialSavePath(cfg); got != want {
		t.Fatalf("reported %q; actual profile file %q", got, want)
	}
	global, err := cliutil.CredentialsFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if want == global {
		t.Fatal("test did not isolate profile and global locations")
	}
}
