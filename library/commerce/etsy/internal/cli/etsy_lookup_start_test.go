package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyIsolateLookupStart(t *testing.T) string {
	t.Helper()
	etsyRestore, etsyErr := cliutil.SetHomeOverride("")
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(etsyRestore)
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	etsyPath, etsyErr := MCPStorePath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	return etsyPath
}

func TestEtsyPurgeBuyerLookupsAtStart(t *testing.T) {
	etsyPath := etsyIsolateLookupStart(t)
	etsyDB, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { _ = etsyDB.Close() })
	for _, etsyKind := range []string{"receipts", "listings"} {
		if etsyErr := lookups.Upsert(etsyDB.DB(), etsyKind, "legacy fixture", "555", "synced"); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	// Use a distinct canonical value because source is not part of the primary key.
	if etsyErr := lookups.Upsert(etsyDB.DB(), "receipts", "taught fixture", "555", "taught"); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if etsyErr := etsyDB.Close(); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	EtsyPurgeBuyerLookupsAtStart()
	etsyReopened, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyReopened.Close()
	for _, etsyCase := range []struct {
		etsyKind, etsySource string
		etsyWant             int
	}{
		{"receipts", "synced", 0},
		{"listings", "synced", 1},
		{"receipts", "taught", 1},
	} {
		var etsyCount int
		if etsyErr := etsyReopened.DB().QueryRow("SELECT count(*) FROM entity_lookups WHERE kind = ? AND source = ?", etsyCase.etsyKind, etsyCase.etsySource).Scan(&etsyCount); etsyErr != nil {
			t.Fatal(etsyErr)
		}
		if etsyCount != etsyCase.etsyWant {
			t.Errorf("kind=%s source=%s: got %d rows, want %d", etsyCase.etsyKind, etsyCase.etsySource, etsyCount, etsyCase.etsyWant)
		}
	}
}

func TestEtsyPurgeBuyerLookupsAtStartNoStore(t *testing.T) {
	etsyPath := etsyIsolateLookupStart(t)
	if _, etsyErr := os.Stat(etsyPath); !os.IsNotExist(etsyErr) {
		t.Fatalf("expected absent store before startup, got %v", etsyErr)
	}
	EtsyPurgeBuyerLookupsAtStart()
	if _, etsyErr := os.Stat(etsyPath); !os.IsNotExist(etsyErr) {
		t.Fatalf("startup created store or stat failed: %v", etsyErr)
	}
}

func etsyCheckLookupStartRows(t *testing.T, etsyDB *store.Store, etsyBuyerCount int) {
	t.Helper()
	for _, etsyCase := range []struct {
		etsyKind, etsySource string
		etsyWant             int
	}{
		{"listings", "synced", 1},
		{"receipts", "taught", 1},
		{"receipts", "synced", etsyBuyerCount},
		{"users", "synced", etsyBuyerCount},
	} {
		var etsyCount int
		if etsyErr := etsyDB.DB().QueryRow("SELECT count(*) FROM entity_lookups WHERE kind = ? AND source = ?", etsyCase.etsyKind, etsyCase.etsySource).Scan(&etsyCount); etsyErr != nil {
			t.Fatal(etsyErr)
		}
		if etsyCount != etsyCase.etsyWant {
			t.Errorf("kind=%s source=%s: got %d rows, want %d", etsyCase.etsyKind, etsyCase.etsySource, etsyCount, etsyCase.etsyWant)
		}
	}
}

func etsySeedLookupStartRows(t *testing.T, etsyPath string, etsyBuyers bool) *store.Store {
	t.Helper()
	etsyDB, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { _ = etsyDB.Close() })
	for _, etsyCase := range []struct{ etsyKind, etsySource string }{
		{"listings", "synced"}, {"receipts", "taught"}, {"receipts", "synced"}, {"users", "synced"},
	} {
		if !etsyBuyers && etsyCase.etsySource == "synced" && etsyCase.etsyKind != "listings" {
			continue
		}
		if etsyErr := lookups.Upsert(etsyDB.DB(), etsyCase.etsyKind, etsyCase.etsySource+" fixture", "555", etsyCase.etsySource); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	return etsyDB
}

func TestEtsyPurgeBuyerLookupsAtStartNoBuyerRowsNeverOpensReadWrite(t *testing.T) {
	etsyPath := etsyIsolateLookupStart(t)
	etsyDB := etsySeedLookupStartRows(t, etsyPath, false)
	etsyOriginal := etsyPurgeOpenRW
	t.Cleanup(func() { etsyPurgeOpenRW = etsyOriginal })
	etsyCalls := 0
	etsyPurgeOpenRW = func(etsyCtx context.Context, etsyPath string) (*store.Store, error) {
		etsyCalls++
		return nil, fmt.Errorf("unexpected read-write open")
	}
	EtsyPurgeBuyerLookupsAtStart()
	if etsyCalls != 0 {
		t.Errorf("no-buyer-rows: read-write opens = %d, want 0", etsyCalls)
	}
	etsyCheckLookupStartRows(t, etsyDB, 0)
}

func TestEtsyPurgeBuyerLookupsAtStartBuyerRowsOpensReadWriteOnce(t *testing.T) {
	etsyPath := etsyIsolateLookupStart(t)
	etsyDB := etsySeedLookupStartRows(t, etsyPath, true)
	etsyCheckLookupStartRows(t, etsyDB, 1)
	etsyOriginal := etsyPurgeOpenRW
	t.Cleanup(func() { etsyPurgeOpenRW = etsyOriginal })
	etsyCalls := 0
	etsyPurgeOpenRW = func(etsyCtx context.Context, etsyPath string) (*store.Store, error) {
		etsyCalls++
		return store.OpenWithContext(etsyCtx, etsyPath)
	}
	EtsyPurgeBuyerLookupsAtStart()
	if etsyCalls != 1 {
		t.Errorf("read-write opens = %d, want 1", etsyCalls)
	}
	etsyCheckLookupStartRows(t, etsyDB, 0)
}

func TestEtsyPurgeBuyerLookupsAtStartMissingTableIsSilent(t *testing.T) {
	etsyPath := etsyIsolateLookupStart(t)
	etsyDB := etsySeedLookupStartRows(t, etsyPath, false)
	if _, etsyErr := etsyDB.DB().Exec("DROP TABLE entity_lookups"); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyOriginal := etsyPurgeOpenRW
	t.Cleanup(func() { etsyPurgeOpenRW = etsyOriginal })
	etsyCalls := 0
	etsyPurgeOpenRW = func(etsyCtx context.Context, etsyPath string) (*store.Store, error) {
		etsyCalls++
		return nil, fmt.Errorf("unexpected read-write open")
	}
	etsyRead, etsyWrite, etsyErr := os.Pipe()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyRead.Close()
	defer etsyWrite.Close()
	etsyStderr := os.Stderr
	t.Cleanup(func() { os.Stderr = etsyStderr })
	os.Stderr = etsyWrite
	EtsyPurgeBuyerLookupsAtStart()
	os.Stderr = etsyStderr
	_ = etsyWrite.Close()
	etsyOutput, etsyErr := io.ReadAll(etsyRead)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if len(etsyOutput) != 0 {
		t.Errorf("startup stderr = %q, want empty", etsyOutput)
	}
	if etsyCalls != 0 {
		t.Errorf("missing-table: read-write opens = %d, want 0", etsyCalls)
	}
	var etsyCount int
	if etsyErr := etsyDB.DB().QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'entity_lookups'").Scan(&etsyCount); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if etsyCount != 0 {
		t.Errorf("entity_lookups recreated: got %d tables", etsyCount)
	}
}
