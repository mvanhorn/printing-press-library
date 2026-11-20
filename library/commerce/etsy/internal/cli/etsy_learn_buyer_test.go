package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyLearnBuyerFixture(t *testing.T) (string, *store.Store) {
	t.Helper()
	// Building the root command registers the CLI query synonyms (order -> receipt) that
	// store.UpsertLearning folds with; seeding before that keys learnings differently from recall.
	RootCmd()
	etsyDB := filepath.Join(withTempLearnHome(t), "data.db")
	etsyStore, etsyErr := store.OpenWithContext(context.Background(), etsyDB)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(func() { etsyStore.Close() })
	etsyReceipt := []byte(`{"receipt_id":555,"name":"QZXBUYERNAME","buyer_email":"qzx@buyer.invalid"}`)
	if etsyErr := etsyStore.UpsertReceipts(etsyReceipt); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if etsyErr := etsyStore.Upsert("receipts", "555", etsyReceipt); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if etsyErr := etsyStore.Upsert("listings", "1000000001", []byte(`{"title":"Juniper Ceramics linen apron","name":"Juniper Ceramics"}`)); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	return etsyDB, etsyStore
}

func TestEtsyTeachRefusesBuyerTypeUnderMCP(t *testing.T) {
	etsyDB, etsyStore := etsyLearnBuyerFixture(t)
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
	for _, etsyArgs := range [][]string{
		{"teach", "--query", "open order zz", "--resource-type", "receipts", "--resource", "555"},
		{"teach-pattern", "--query-template", "orders for {entity}", "--resource-template", "R-{entity:buyer}", "--resource-type", "receipts", "--entity-kind", "buyer", "--strategy", "substitute"},
	} {
		_, _, etsyErr := runRootArgs(t, append(etsyArgs, "--db", etsyDB)...)
		if etsyErr == nil || ExitCode(etsyErr) != 2 || !strings.Contains(etsyErr.Error(), "ETSY_MCP_ALLOW_BUYER_PII") {
			t.Fatalf("expected buyer refusal, got %v", etsyErr)
		}
	}
	var etsyCount int
	if etsyErr := etsyStore.DB().QueryRow("SELECT COUNT(*) FROM search_learnings").Scan(&etsyCount); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if etsyCount != 0 {
		t.Fatalf("stored %d refused learnings", etsyCount)
	}
	if _, _, etsyErr := runRootArgs(t, "teach", "--query", "find apron", "--resource-type", "listings", "--resource", "1000000001", "--db", etsyDB); etsyErr != nil {
		t.Fatal(etsyErr)
	}
}

func TestEtsyRecallDropsBuyerEntities(t *testing.T) {
	etsyDB, etsyStore := etsyLearnBuyerFixture(t)
	for _, etsyInput := range []store.UpsertLearningInput{
		{Query: "open order zz", QueryEntities: []string{}, ResourceID: "555", ResourceType: "receipts", Source: store.LearningSourceTaught},
		{Query: "find linen apron", QueryEntities: []string{}, ResourceID: "1000000001", ResourceType: "listings", Source: store.LearningSourceTaught},
	} {
		if _, _, etsyErr := etsyStore.UpsertLearning(context.Background(), etsyInput); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	for _, etsyCase := range []struct {
		etsyName, etsyMCP, etsyOpt string
		etsyPresent                bool
	}{
		{"mcp restricted", "1", "", false}, {"mcp opted in", "1", "1", true}, {"plain cli", "", "", true},
	} {
		t.Run(etsyCase.etsyName, func(t *testing.T) {
			t.Setenv(cliutil.MCPChildEnvVar, etsyCase.etsyMCP)
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", etsyCase.etsyOpt)
			etsyOut, _, etsyErr := runRootArgs(t, "recall", "open order zz", "--agent", "--db", etsyDB)
			if etsyErr != nil {
				t.Fatal(etsyErr)
			}
			var etsyEnv recallEnvelope
			unmarshalAgentResults(t, etsyOut, &etsyEnv)
			if !etsyEnv.Found || len(etsyEnv.Results) != 1 || etsyEnv.Results[0].ResourceID != "555" {
				t.Fatalf("missing receipt hit: %s", etsyOut)
			}
			if strings.Contains(strings.ToLower(etsyOut), "qzxbuyername") != etsyCase.etsyPresent {
				t.Fatalf("unexpected buyer entity visibility: %s", etsyOut)
			}
			if !etsyCase.etsyPresent {
				if len(etsyEnv.Results[0].ResourceEntities) != 0 || strings.Contains(etsyOut, "qzx@buyer.invalid") || strings.Contains(etsyOut, "resource_not_in_store") {
					t.Fatalf("buyer details or false missing warning: %s", etsyOut)
				}
			}
			etsyOut, _, etsyErr = runRootArgs(t, "recall", "find linen apron", "--agent", "--db", etsyDB)
			if etsyErr != nil {
				t.Fatal(etsyErr)
			}
			unmarshalAgentResults(t, etsyOut, &etsyEnv)
			if !etsyEnv.Found || len(etsyEnv.Results) != 1 || len(etsyEnv.Results[0].ResourceEntities) == 0 {
				t.Fatalf("listing entities missing: %s", etsyOut)
			}
		})
	}
}

func TestEtsyRecallPurgesLegacyBuyerLookups(t *testing.T) {
	etsyDB, etsyStore := etsyLearnBuyerFixture(t)
	for _, etsyKind := range []string{"receipts", "listings"} {
		if etsyErr := lookups.Upsert(etsyStore.DB(), etsyKind, "legacy fixture", "555", "synced"); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	if _, _, etsyErr := runRootArgs(t, "recall", "anything here", "--db", etsyDB); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	for etsyKind, etsyWant := range map[string]int{"receipts": 0, "listings": 1} {
		var etsyCount int
		if etsyErr := etsyStore.DB().QueryRow("SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND source = 'synced'", etsyKind).Scan(&etsyCount); etsyErr != nil {
			t.Fatal(etsyErr)
		}
		if etsyCount != etsyWant {
			t.Fatalf("%s lookup count = %d, want %d", etsyKind, etsyCount, etsyWant)
		}
	}
}
