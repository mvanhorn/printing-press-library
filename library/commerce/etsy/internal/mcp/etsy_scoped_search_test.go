package mcp

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyScopedSearchFixture(t *testing.T) *store.Store {
	t.Helper()
	resetMCPPathEnv(t)
	etsyPath, etsyErr := mcpDBPath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyWriter, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { etsyWriter.Close() })
	for etsyType, etsyData := range map[string]string{
		"listings": `{"title":"Juniper Ceramics linen apron"}`,
		"receipts": `{"name":"QZXBUYERNAME linen apron"}`,
	} {
		if etsyErr := etsyWriter.Upsert(etsyType, etsyType, json.RawMessage(etsyData)); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	if etsyErr := etsyWriter.Close(); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyDB, etsyErr := store.OpenReadOnly(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { etsyDB.Close() })
	etsyDB.DB().SetMaxOpenConns(0)
	return etsyDB
}

func TestEtsyScopedSearchLeavesCallerPoolAlone(t *testing.T) {
	etsyDB := etsyScopedSearchFixture(t)
	t.Setenv(BuyerPIIEnvVar, "")
	if etsyErr := os.Unsetenv(BuyerPIIEnvVar); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyBefore := etsyDB.DB().Stats().MaxOpenConnections
	if etsyBefore != 0 {
		t.Fatalf("expected unlimited caller pool, got %d", etsyBefore)
	}
	for etsyAttempt := 0; etsyAttempt < 2; etsyAttempt++ {
		etsyResults, etsyErr := etsyScopedSearch(context.Background(), etsyDB, "linen apron", 10)
		if etsyErr != nil {
			t.Fatal(etsyErr)
		}
		etsyText := string(etsyJoinedSearchResults(etsyResults))
		if len(etsyResults) != 1 || !strings.Contains(etsyText, "Juniper Ceramics") || strings.Contains(etsyText, "QZXBUYERNAME") {
			t.Fatalf("unexpected guarded search: %s", etsyText)
		}
		if etsyAfter := etsyDB.DB().Stats().MaxOpenConnections; etsyAfter != etsyBefore {
			t.Fatalf("caller pool changed from %d to %d", etsyBefore, etsyAfter)
		}
		var etsyViews int
		if etsyErr := etsyDB.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_temp_master WHERE name='resources'`).Scan(&etsyViews); etsyErr != nil {
			t.Fatal(etsyErr)
		}
		if etsyViews != 0 {
			t.Fatalf("caller has %d resources temp views", etsyViews)
		}
	}
}

func etsyJoinedSearchResults(etsyResults []json.RawMessage) []byte {
	var etsyText []byte
	for _, etsyResult := range etsyResults {
		etsyText = append(etsyText, etsyResult...)
	}
	return etsyText
}

func TestEtsyScopedSearchOptInSeesBuyerRows(t *testing.T) {
	etsyDB := etsyScopedSearchFixture(t)
	t.Setenv(BuyerPIIEnvVar, "1")
	etsyBefore := etsyDB.DB().Stats().MaxOpenConnections
	etsyResults, etsyErr := etsyScopedSearch(context.Background(), etsyDB, "linen apron", 10)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyText := string(etsyJoinedSearchResults(etsyResults))
	if len(etsyResults) != 2 || !strings.Contains(etsyText, "Juniper Ceramics") || !strings.Contains(etsyText, "QZXBUYERNAME") {
		t.Fatalf("unexpected opt-in search: %s", etsyText)
	}
	if etsyAfter := etsyDB.DB().Stats().MaxOpenConnections; etsyAfter != etsyBefore {
		t.Fatalf("caller pool changed from %d to %d", etsyBefore, etsyAfter)
	}
}

func TestEtsyScopedSearchEmptyPathFailsClosed(t *testing.T) {
	t.Setenv(BuyerPIIEnvVar, "")
	etsyResults, etsyErr := etsyScopedSearch(context.Background(), &store.Store{}, "linen apron", 10)
	if etsyErr == nil || !strings.Contains(etsyErr.Error(), "empty database path") || len(etsyResults) != 0 {
		t.Fatalf("expected closed empty-path failure, got %s, %v", etsyResults, etsyErr)
	}
}
