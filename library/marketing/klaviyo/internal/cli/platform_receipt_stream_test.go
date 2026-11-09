// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func TestPlatformReceiptCapturesDirectFinalStdoutAndStderr(t *testing.T) {
	session := platformCommandTestSession(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	flags := &rootFlags{
		platformSession: session, receiptEnabled: true, receiptFile: receiptPath,
		platformMetadataEmitted: true,
	}
	root := &cobra.Command{Use: "fixture-pp-cli"}
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	if err := initializePlatformReceipt(root, flags); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if flags.receiptWriter != nil {
			_ = flags.receiptWriter.CloseProcessOutputs()
		}
	})

	stdoutBytes := []byte("direct stdout\n")
	stderrBytes := []byte("direct stderr\n")
	if _, err := os.Stdout.Write(stdoutBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stderr.Write(stderrBytes); err != nil {
		t.Fatal(err)
	}
	var commandErr error
	finalizePlatformInvocation(flags, &commandErr)
	if commandErr != nil {
		t.Fatal(commandErr)
	}
	if !bytes.Equal(stdout.Bytes(), stdoutBytes) || !bytes.Equal(stderr.Bytes(), stderrBytes) {
		t.Fatalf("destinations stdout=%q stderr=%q", stdout.Bytes(), stderr.Bytes())
	}

	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt platform.Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	digests := map[string]platform.OutputDigest{}
	for _, output := range receipt.Outputs {
		digests[output.Name] = output
	}
	for name, expected := range map[string][]byte{"stdout": stdoutBytes, "stderr": stderrBytes} {
		digest := sha256.Sum256(expected)
		if digests[name].SHA256 != hex.EncodeToString(digest[:]) || digests[name].Bytes != int64(len(expected)) {
			t.Errorf("%s digest = %+v", name, digests[name])
		}
	}
}

func TestPlatformReceiptFailureDiagnosticUsesCapturedStderr(t *testing.T) {
	session := platformCommandTestSession(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	flags := &rootFlags{
		platformSession: session, receiptEnabled: true, receiptFile: receiptPath,
		platformMetadataEmitted: true,
	}
	root := &cobra.Command{Use: "fixture-pp-cli"}
	var stderr bytes.Buffer
	root.SetOut(bytes.NewBuffer(nil))
	root.SetErr(&stderr)
	if err := initializePlatformReceipt(root, flags); err != nil {
		t.Fatal(err)
	}
	diagnostic := "Error: synthetic command failure\n"
	if _, err := fmt.Fprint(root.ErrOrStderr(), diagnostic); err != nil {
		t.Fatal(err)
	}
	commandErr := fmt.Errorf("synthetic command failure")
	finalizePlatformInvocation(flags, &commandErr)
	if !bytes.Equal(stderr.Bytes(), []byte(diagnostic)) {
		t.Fatalf("stderr = %q", stderr.Bytes())
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt platform.Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(diagnostic))
	for _, output := range receipt.Outputs {
		if output.Name == "stderr" && output.SHA256 == hex.EncodeToString(digest[:]) && output.Bytes == int64(len(diagnostic)) {
			return
		}
	}
	t.Fatalf("failure diagnostic digest missing: %+v", receipt.Outputs)
}
