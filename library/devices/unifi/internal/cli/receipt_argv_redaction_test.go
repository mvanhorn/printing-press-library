package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/platform"
)

// Proves the writer built for a real command applies the redaction, since the
// helper alone would pass even if nothing called it.
func TestReceiptWriterRedactsSecretArgv(t *testing.T) {
	prev := os.Args
	t.Cleanup(func() { os.Args = prev })
	os.Args = []string{"unifi-pp-cli", "sync", "--x-passphrase", "SMOKEVALUE1", "--token=SMOKEVALUE2", "--json"}

	flags := &rootFlags{platformSession: &platform.Session{Profile: &platform.Profile{Name: "tenant-a"}, CLI: "unifi-pp-cli"}, receiptFile: filepath.Join(t.TempDir(), "r.json")}
	cmd := &cobra.Command{Use: "unifi-pp-cli"}
	if err := initializePlatformReceipt(cmd, flags); err != nil {
		t.Fatalf("initializePlatformReceipt: %v", err)
	}
	joined := strings.Join(flags.receiptWriter.Receipt.Argv, " ")
	if strings.Contains(joined, "SMOKEVALUE") {
		t.Fatalf("secret survived in receipt argv: %q", joined)
	}
	if !strings.Contains(joined, "--json") {
		t.Fatalf("non-secret flag was lost: %q", joined)
	}
}
