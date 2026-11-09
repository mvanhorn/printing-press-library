// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

func TestPlatformReceiptDeliveryPreservesStdoutAndHashesFinalFile(t *testing.T) {
	session := platformCommandTestSession(t)
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	deliveredPath := filepath.Join(t.TempDir(), "delivered.json")
	writer, err := platform.NewReceiptWriter(session, platform.ReceiptOptions{
		CLI: platform.ReceiptCLI{Name: "fixture-pp-cli", Version: "test"}, ReceiptFile: receiptPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{deliverSpec: "file:" + deliveredPath, receiptWriter: writer}
	root := &cobra.Command{Use: "fixture-pp-cli"}
	child := &cobra.Command{Use: "report"}
	root.AddCommand(child)
	var stdout bytes.Buffer
	root.SetOut(writer.CaptureWriter("stdout", &stdout))
	root.SetErr(writer.CaptureWriter("stderr", bytes.NewBuffer(nil)))
	if err := configureDelivery(child, flags); err != nil {
		t.Fatal(err)
	}
	payload := []byte("{\"result\":true}\n")
	if _, err := child.OutOrStdout().Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := finalizeDelivery(flags); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(context.Background(), "success", 0, nil, platform.OutputMetadata{}, platform.RateLimitMetadata{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("stdout = %q, want %q", stdout.Bytes(), payload)
	}
	delivered, err := os.ReadFile(deliveredPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(delivered, payload) {
		t.Fatalf("delivered file = %q, want %q", delivered, payload)
	}
	receiptData, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt platform.Receipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		t.Fatal(err)
	}
	digests := map[string]platform.OutputDigest{}
	for _, output := range receipt.Outputs {
		digests[output.Name] = output
	}
	expected := sha256.Sum256(payload)
	wantDigest := hex.EncodeToString(expected[:])
	for _, name := range []string{"stdout", "delivered_file"} {
		if digests[name].SHA256 != wantDigest || digests[name].Bytes != int64(len(payload)) {
			t.Errorf("%s digest = %+v, want bytes=%d sha256=%s", name, digests[name], len(payload), wantDigest)
		}
	}
}
