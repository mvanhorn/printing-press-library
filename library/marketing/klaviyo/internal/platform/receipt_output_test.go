// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func receiptOutputTestSession(t *testing.T) *Session {
	t.Helper()
	setPlatformRoots(t)
	paths, err := PathsFor("synthetic", "sample-pp-cli", "sample")
	if err != nil {
		t.Fatal(err)
	}
	return &Session{
		Profile: &Profile{Name: "synthetic"}, ProfileName: "synthetic", CLI: "sample", Source: "sample",
		GateOutcome: GateVerified, ExpectedIdentity: map[string]string{"account_id": "one"},
		ObservedIdentity: map[string]string{"account_id": "one"}, Paths: paths,
	}
}

func outputDigestMap(outputs []OutputDigest) map[string]OutputDigest {
	result := make(map[string]OutputDigest, len(outputs))
	for _, output := range outputs {
		result[output.Name] = output
	}
	return result
}

func expectedOutputDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func readReceiptFixture(t *testing.T, path string) Receipt {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatalf("receipt is not valid JSON: %v: %s", err, data)
	}
	return receipt
}

func TestReceiptOutputHashFixturesCoverStdoutEmptyDeliveryAndFailure(t *testing.T) {
	session := receiptOutputTestSession(t)
	root := t.TempDir()
	deliveredPath := filepath.Join(root, "delivered.json")
	delivered := []byte("{\"delivered\":true}\n")
	if err := os.WriteFile(deliveredPath, delivered, 0o600); err != nil {
		t.Fatal(err)
	}

	successPath := filepath.Join(root, "success.json")
	writer, err := NewReceiptWriter(session, ReceiptOptions{CLI: ReceiptCLI{Name: "sample-pp-cli", Version: "test"}, ReceiptFile: successPath})
	if err != nil {
		t.Fatal(err)
	}
	stdout := []byte("{\"ok\":true}\n")
	if _, err := writer.CaptureWriter("stdout", io.Discard).Write(stdout); err != nil {
		t.Fatal(err)
	}
	writer.CaptureWriter("stderr", io.Discard)
	if err := writer.RecordFileOutput("delivered_file", deliveredPath); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(context.Background(), "success", 0, nil, OutputMetadata{}, RateLimitMetadata{}); err != nil {
		t.Fatal(err)
	}
	success := readReceiptFixture(t, successPath)
	digests := outputDigestMap(success.Outputs)
	for name, expected := range map[string][]byte{"stdout": stdout, "stderr": {}, "delivered_file": delivered} {
		actual, ok := digests[name]
		if !ok || actual.Bytes != int64(len(expected)) || actual.SHA256 != expectedOutputDigest(expected) {
			t.Errorf("%s digest = %+v, want bytes=%d sha256=%s", name, actual, len(expected), expectedOutputDigest(expected))
		}
	}

	failurePath := filepath.Join(root, "failure.json")
	failureWriter, err := NewReceiptWriter(session, ReceiptOptions{CLI: ReceiptCLI{Name: "sample-pp-cli", Version: "test"}, ReceiptFile: failurePath})
	if err != nil {
		t.Fatal(err)
	}
	failureWriter.CaptureWriter("stdout", io.Discard)
	diagnostic := []byte("Error: synthetic failure\n")
	if _, err := failureWriter.CaptureWriter("stderr", io.Discard).Write(diagnostic); err != nil {
		t.Fatal(err)
	}
	if err := failureWriter.Finalize(context.Background(), "error", 7, nil, OutputMetadata{}, RateLimitMetadata{}); err != nil {
		t.Fatal(err)
	}
	failure := readReceiptFixture(t, failurePath)
	failureDigests := outputDigestMap(failure.Outputs)
	if failure.Status != "error" || failure.ExitCode != 7 || failureDigests["stderr"].SHA256 != expectedOutputDigest(diagnostic) || failureDigests["stderr"].Bytes != int64(len(diagnostic)) {
		t.Fatalf("failure receipt = %+v", failure)
	}
}

func TestReceiptAtomicWriteLeavesNoValidLookingPartial(t *testing.T) {
	session := receiptOutputTestSession(t)
	root := t.TempDir()
	blockedDestination := filepath.Join(root, "receipt.json")
	if err := os.Mkdir(blockedDestination, 0o700); err != nil {
		t.Fatal(err)
	}
	writer, err := NewReceiptWriter(session, ReceiptOptions{CLI: ReceiptCLI{Name: "sample-pp-cli", Version: "test"}, ReceiptFile: blockedDestination})
	if err != nil {
		t.Fatal(err)
	}
	writer.CaptureWriter("stdout", io.Discard)
	if err := writer.Finalize(context.Background(), "success", 0, nil, OutputMetadata{}, RateLimitMetadata{}); err == nil {
		t.Fatal("receipt publication unexpectedly replaced a directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "receipt.json" && entry.IsDir() {
			continue
		}
		if entry.Type().IsRegular() || filepath.Ext(entry.Name()) == ".json" {
			t.Fatalf("failed atomic publication left a valid-looking artifact: %s", entry.Name())
		}
	}
}

func TestReceiptAuditIndexRemainsValidUnderConcurrentWriters(t *testing.T) {
	session := receiptOutputTestSession(t)
	root := t.TempDir()
	auditDir := filepath.Join(root, "audit")
	const writers = 24
	errorsCh := make(chan error, writers)
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			writer, err := NewReceiptWriter(session, ReceiptOptions{
				CLI:         ReceiptCLI{Name: "sample-pp-cli", Version: "test"},
				ReceiptFile: filepath.Join(root, "primary", string(rune('a'+index))+".json"), AuditDir: auditDir,
			})
			if err != nil {
				errorsCh <- err
				return
			}
			payload := []byte{byte(index)}
			if _, err := writer.CaptureWriter("stdout", io.Discard).Write(payload); err != nil {
				errorsCh <- err
				return
			}
			if err := writer.Finalize(context.Background(), "success", 0, nil, OutputMetadata{}, RateLimitMetadata{}); err != nil {
				errorsCh <- err
			}
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	indexFile, err := os.Open(filepath.Join(auditDir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer indexFile.Close()
	seen := map[string]bool{}
	scanner := bufio.NewScanner(indexFile)
	for scanner.Scan() {
		var entry struct {
			RunID   string `json:"run_id"`
			Receipt string `json:"receipt"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid concurrent index line: %v: %s", err, scanner.Bytes())
		}
		if entry.RunID == "" || seen[entry.RunID] {
			t.Fatalf("missing or duplicate run ID: %q", entry.RunID)
		}
		seen[entry.RunID] = true
		info, err := os.Stat(filepath.Join(auditDir, filepath.FromSlash(entry.Receipt)))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("indexed receipt mode = %v err=%v", info, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != writers {
		t.Fatalf("audit index contains %d entries, want %d", len(seen), writers)
	}
	if info, err := os.Stat(filepath.Join(auditDir, "index.jsonl")); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("audit index mode = %v err=%v", info, err)
	}
	if matches, err := filepath.Glob(filepath.Join(auditDir, ".tmp-*")); err != nil || len(matches) != 0 {
		t.Fatalf("audit index left temporary artifacts: matches=%v err=%v", matches, err)
	}
}

func TestReceiptFileOutputRejectsDirectories(t *testing.T) {
	session := receiptOutputTestSession(t)
	writer, err := NewReceiptWriter(session, ReceiptOptions{CLI: ReceiptCLI{Name: "sample-pp-cli", Version: "test"}, ReceiptFile: filepath.Join(t.TempDir(), "receipt.json")})
	if err != nil {
		t.Fatal(err)
	}
	err = writer.RecordFileOutput("delivered_file", t.TempDir())
	if err == nil || !errors.Is(err, ErrInvalidOutputFile) {
		t.Fatalf("directory delivery error = %v", err)
	}
}

func TestOutputCaptureHashesOnlySuccessfullyWrittenBytes(t *testing.T) {
	capture := &outputCapture{destination: shortWriter{}, hash: sha256.New()}
	n, err := capture.Write([]byte("abcdef"))
	if n != 3 || err == nil {
		t.Fatalf("short write = %d, %v", n, err)
	}
	digest, count := capture.snapshot()
	if count != 3 || digest != expectedOutputDigest([]byte("abc")) {
		t.Fatalf("capture = bytes=%d sha256=%s", count, digest)
	}
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) {
	return min(3, len(data)), io.ErrShortWrite
}

func TestReceiptOutputFixtureDoesNotEmbedPayload(t *testing.T) {
	session := receiptOutputTestSession(t)
	path := filepath.Join(t.TempDir(), "receipt.json")
	writer, err := NewReceiptWriter(session, ReceiptOptions{CLI: ReceiptCLI{Name: "sample-pp-cli", Version: "test"}, ReceiptFile: path})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("private synthetic output")
	if _, err := writer.CaptureWriter("stdout", io.Discard).Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Finalize(context.Background(), "success", 0, nil, OutputMetadata{}, RateLimitMetadata{}); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(receipt, payload) {
		t.Fatal("receipt embedded captured output payload")
	}
}
