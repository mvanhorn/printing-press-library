// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/cliutil"
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

func TestVersionCommandDeliversFileThroughExecute(t *testing.T) {
	home := t.TempDir()
	destination := filepath.Join(home, "version.txt")
	restoreHome, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restoreHome)

	previousArgs := os.Args
	os.Args = []string{"sarvam-pp-cli", "--home", home, "--no-learn", "--deliver", "file:" + destination, "version"}
	t.Cleanup(func() { os.Args = previousArgs })

	if err := Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("sarvam-pp-cli %s\n", version); string(got) != want {
		t.Fatalf("delivered version = %q, want %q", got, want)
	}
}
