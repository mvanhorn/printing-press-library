// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/cliutil/testenv"
)

func TestVersionCommandUsesConfiguredWriter(t *testing.T) {
	cmd := newVersionCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), fmt.Sprintf("version %s\n", version); got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestVersionDeliveryCaptureThroughRoot(t *testing.T) {
	testenv.Isolate(t, cliutil.ConfigDir)
	t.Setenv(mcpBoundProfileEnv, "")
	flags := &rootFlags{}
	root := newRootCmd(flags)
	root.SetArgs([]string{"version", "--deliver", "file:" + filepath.Join(t.TempDir(), "version.txt")})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if flags.deliverBuf == nil {
		t.Fatal("root did not configure delivery capture")
	}
	if got, want := flags.deliverBuf.String(), fmt.Sprintf("mcpmarket-pp-cli %s\n", version); got != want {
		t.Fatalf("captured version = %q, want %q", got, want)
	}
}

func TestVersionDeliveryWritesFile(t *testing.T) {
	testenv.Isolate(t, cliutil.ConfigDir)
	t.Setenv(mcpBoundProfileEnv, "")
	deliveryPath := filepath.Join(t.TempDir(), "version.txt")
	previousArgs := os.Args
	os.Args = []string{"mcpmarket-pp-cli", "version", "--deliver", "file:" + deliveryPath}
	defer func() { os.Args = previousArgs }()
	if err := Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(deliveryPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), fmt.Sprintf("mcpmarket-pp-cli %s\n", version); got != want {
		t.Fatalf("delivered version = %q, want %q", got, want)
	}
}
