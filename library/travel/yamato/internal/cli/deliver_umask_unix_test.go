//go:build unix

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteDownloadUnderKeepsOwnerAccessWithUmask(t *testing.T) {
	if os.Getenv("YAMATO_DELIVERY_UMASK_TEST_HELPER") == "1" {
		defer syscall.Umask(syscall.Umask(0o400))
		dir := os.Getenv("YAMATO_DELIVERY_UMASK_TEST_DIR")
		if err := writeDownloadUnder(dir, "report.txt", []byte("delivered")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(dir, "report.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("delivery mode = %o; owner access and privacy require 0600", info.Mode().Perm())
		}
		return
	}
	// Process isolation keeps the restrictive umask away from parallel tests.
	cmd := exec.Command(os.Args[0], "-test.run=^TestWriteDownloadUnderKeepsOwnerAccessWithUmask$")
	cmd.Env = append(os.Environ(), "YAMATO_DELIVERY_UMASK_TEST_HELPER=1", "YAMATO_DELIVERY_UMASK_TEST_DIR="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restrictive-umask delivery: %v\n%s", err, out)
	}
}
