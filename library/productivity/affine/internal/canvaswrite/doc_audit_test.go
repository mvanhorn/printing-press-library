package canvaswrite

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

func TestAuditDocAllowsSnapshotFileWithoutDocFlags(t *testing.T) {
	path := writeSnapshot(t)

	result, err := AuditDoc(&config.Config{}, DocAuditOptions{
		SnapshotFile: path,
		Keywords:     []string{"audit"},
	})
	if err != nil {
		t.Fatalf("AuditDoc returned error: %v", err)
	}
	if result.BlockCount != 1 {
		t.Fatalf("BlockCount = %d, want 1", result.BlockCount)
	}
	if result.KeywordHits["audit"] != 1 {
		t.Fatalf("KeywordHits[audit] = %d, want 1", result.KeywordHits["audit"])
	}
}

func TestCheckDocIntegrityAllowsSnapshotFileWithoutDocFlags(t *testing.T) {
	path := writeSnapshot(t)

	result, err := CheckDocIntegrity(&config.Config{}, DocIntegrityOptions{
		SnapshotFile: path,
	})
	if err != nil {
		t.Fatalf("CheckDocIntegrity returned error: %v", err)
	}
	if result.BlockCount != 1 {
		t.Fatalf("BlockCount = %d, want 1", result.BlockCount)
	}
}

func writeSnapshot(t *testing.T) string {
	t.Helper()

	engine, err := yjs.NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	docID, err := engine.NewDoc()
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.CreateFormattedBlock(docID, "p1", "affine:paragraph", "text", "Snapshot audit"); err != nil {
		t.Fatal(err)
	}
	update, err := engine.EncodeStateAsUpdate(docID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(update)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/snapshot.bin"
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
