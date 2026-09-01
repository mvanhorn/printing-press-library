package sequence

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// winEnv builds a fake Windows environment lookup.
func winEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// TestWindowsDefaultLockDirIsMachineWide is the regression test for the Windows
// half of cross-user serialization. os.TempDir() honors TMP/TEMP, which for an
// interactive logon is the per-user %LOCALAPPDATA%\Temp — two accounts driving
// one KVM would never contend on the same lock file.
//
// This runs on every host (not just Windows) by exercising the pure core, so
// the guarantee is genuinely verified rather than skipped in CI.
func TestWindowsDefaultLockDirIsMachineWide(t *testing.T) {
	alice := defaultSharedLockDirFor("windows", winEnv(map[string]string{
		"ProgramData":  `C:\ProgramData`,
		"SystemRoot":   `C:\Windows`,
		"TMP":          `C:\Users\alice\AppData\Local\Temp`,
		"TEMP":         `C:\Users\alice\AppData\Local\Temp`,
		"LOCALAPPDATA": `C:\Users\alice\AppData\Local`,
		"USERPROFILE":  `C:\Users\alice`,
	}))
	bob := defaultSharedLockDirFor("windows", winEnv(map[string]string{
		"ProgramData":  `C:\ProgramData`,
		"SystemRoot":   `C:\Windows`,
		"TMP":          `C:\Users\bob\AppData\Local\Temp`,
		"TEMP":         `C:\Users\bob\AppData\Local\Temp`,
		"LOCALAPPDATA": `C:\Users\bob\AppData\Local`,
		"USERPROFILE":  `C:\Users\bob`,
	}))

	if alice != bob {
		t.Fatalf("Windows lock dir differs per user (%q vs %q); the two accounts would never contend on one lock", alice, bob)
	}
	for _, marker := range []string{`\AppData\`, `\Users\`} {
		if strings.Contains(alice, marker) {
			t.Fatalf("Windows lock dir %q is inside a per-user location (%s)", alice, marker)
		}
	}
	// filepath.Join uses the host separator, so compare on normalized slashes:
	// the property under test is the machine-wide *location*, not the separator.
	norm := func(s string) string { return strings.ReplaceAll(s, `\`, "/") }
	if want := `C:/ProgramData/kvmctl/locks`; norm(alice) != want {
		t.Fatalf("Windows lock dir = %q, want the machine-wide %q", alice, want)
	}
}

// TestWindowsDefaultLockDirFallbacks covers hosts where ProgramData is absent
// (service accounts, stripped environments). The fallbacks must still be
// machine-wide.
func TestWindowsDefaultLockDirFallbacks(t *testing.T) {
	noProgramData := defaultSharedLockDirFor("windows", winEnv(map[string]string{
		"SystemRoot": `C:\Windows`,
		"TMP":        `C:\Users\carol\AppData\Local\Temp`,
	}))
	if want := "C:/Windows/Temp/kvmctl-locks"; strings.ReplaceAll(noProgramData, `\`, "/") != want {
		t.Fatalf("SystemRoot fallback = %q, want %q", noProgramData, want)
	}
	if strings.Contains(noProgramData, `\Users\`) {
		t.Fatalf("SystemRoot fallback %q is per-user", noProgramData)
	}

	bare := defaultSharedLockDirFor("windows", winEnv(map[string]string{
		"TMP": `C:\Users\dave\AppData\Local\Temp`,
	}))
	if want := "C:/ProgramData/kvmctl/locks"; strings.ReplaceAll(bare, `\`, "/") != want {
		t.Fatalf("bare fallback = %q, want %q", bare, want)
	}
	if strings.Contains(bare, `\Users\`) {
		t.Fatalf("bare fallback %q is per-user", bare)
	}
}

// TestWindowsDefaultLockDirIgnoresTempVars pins the specific bug: changing
// TMP/TEMP (what os.TempDir reads) must not move the lock.
func TestWindowsDefaultLockDirIgnoresTempVars(t *testing.T) {
	base := map[string]string{"ProgramData": `C:\ProgramData`, "SystemRoot": `C:\Windows`}
	withTemp := map[string]string{
		"ProgramData": `C:\ProgramData`,
		"SystemRoot":  `C:\Windows`,
		"TMP":         `D:\somewhere\else`,
		"TEMP":        `D:\somewhere\else`,
	}
	if a, b := defaultSharedLockDirFor("windows", winEnv(base)), defaultSharedLockDirFor("windows", winEnv(withTemp)); a != b {
		t.Fatalf("TMP/TEMP changed the lock dir (%q vs %q); it must be independent of the per-user temp path", a, b)
	}
}

func TestUnixDefaultLockDirIsMachineWide(t *testing.T) {
	got := defaultSharedLockDirFor("linux", winEnv(map[string]string{"HOME": "/home/alice"}))
	if got != "/tmp/kvmctl-locks" {
		t.Fatalf("unix lock dir = %q, want /tmp/kvmctl-locks", got)
	}
	if strings.Contains(got, "/home/") {
		t.Fatalf("unix lock dir %q is per-user", got)
	}
}

// TestDefaultSharedLockDirIsStable asserts the invariant that matters on every
// platform: the default is a constant, not a function of the calling user's
// environment or privileges.
func TestDefaultSharedLockDirIsStable(t *testing.T) {
	first := defaultSharedLockDir()
	if first == "" {
		t.Fatal("defaultSharedLockDir returned an empty path")
	}
	if runtime.GOOS != "windows" && !filepath.IsAbs(first) {
		t.Fatalf("defaultSharedLockDir = %q, which is not absolute", first)
	}
	for i := 0; i < 5; i++ {
		if again := defaultSharedLockDir(); again != first {
			t.Fatalf("defaultSharedLockDir returned %q then %q; it must be deterministic", first, again)
		}
	}
}

// TestDefaultSharedLockDirUnixIsNotPerUser guards the live Unix default against
// drifting to os.UserCacheDir or $HOME.
func TestDefaultSharedLockDirUnixIsNotPerUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-specific default")
	}
	got := defaultSharedLockDir()
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if strings.HasPrefix(got, strings.TrimRight(home, string(os.PathSeparator))+string(os.PathSeparator)) {
			t.Fatalf("default lock dir %q is inside the user's home", got)
		}
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" && strings.HasPrefix(got, cache) {
		t.Fatalf("default lock dir %q is inside the per-user cache dir", got)
	}
}
