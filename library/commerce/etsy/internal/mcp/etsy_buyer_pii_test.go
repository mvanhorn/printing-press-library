package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestEtsyBuyerPIIBlocksEveryBuyerType(t *testing.T) {
	resetMCPPathEnv(t)
	path, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{}
	for _, typ := range etsyBuyerResourceTypes {
		fixtures[typ] = `{"name":"Test Buyer","city":"Testville","title":"Test Buyer","buyer_user_id":920001}`
	}
	fixtures["receipts"] = `{"receipt_id":910001,"name":"Test Buyer","city":"Testville","payment_email":"synthetic@example.invalid","message_from_seller":"seller note","transactions":[{"transaction_id":930001,"variations":[{"formatted_name":"Personalization","formatted_value":"Monogram AB"}]}]}`
	fixtures["transactions"] = `{"transaction_id":930001,"name":"Test Buyer"}`
	fixtures["user_addresses"] = `{"user_address_id":940001,"name":"Test Buyer"}`
	fixtures["shops_receipts_payments"] = `{"payment_id":950001,"name":"Test Buyer"}`
	fixtures["users"] = `{"user_id":920001,"primary_email":"synthetic@example.invalid","first_name":"Test Buyer"}`
	for _, typ := range []string{"reviews", "listings_reviews"} {
		fixtures[typ] = `{"buyer_user_id":920001,"review":"linen apron arrived"}`
	}
	fixtures["receipts_listings"] = `{"listing_id":1000000009,"title":"Juniper receipt-linked","parent_id":"910001"}`
	for _, typ := range []string{"listings", "shops", "listings_inventory", "shipping_profiles"} {
		fixtures[typ] = `{"listing_id":1000000001,"shop_id":9000001,"title":"Juniper Ceramics mug"}`
	}
	for typ, data := range fixtures {
		if err := db.Upsert(typ, typ, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.UpsertTransactions([]byte(fixtures["transactions"])); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertUserAddresses([]byte(fixtures["user_addresses"])); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPayments([]byte(fixtures["shops_receipts_payments"])); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertReceipts([]byte(fixtures["receipts"])); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	invoke := func(sql bool, query string) string {
		t.Helper()
		req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": query}}}
		var result *mcplib.CallToolResult
		var err error
		if sql {
			result, err = handleSQL(context.Background(), req)
		} else {
			result, err = handleSearch(context.Background(), req)
		}
		if err != nil {
			t.Fatal(err)
		}
		return mcpTextContent(t, result)
	}
	for _, opt := range []string{"", "0", "no", "1", "true", "yes", "TrUe", "YES"} {
		t.Setenv(BuyerPIIEnvVar, opt)
		text := invoke(true, "SELECT data FROM resources")
		if !strings.Contains(text, "Juniper Ceramics mug") {
			t.Fatalf("listing lost: %s", text)
		}
		if strings.Contains(text, "Test Buyer") != (buyerPIIAllowed()) {
			t.Fatalf("SQL privacy with opt=%q: %s", opt, text)
		}
		text = invoke(false, "Test")
		if strings.Contains(text, "Test Buyer") != (buyerPIIAllowed()) {
			t.Fatalf("search privacy opt=%q: %s", opt, text)
		}
		if !buyerPIIAllowed() {
			for _, secret := range []string{"Test Buyer", "Monogram AB", "synthetic@example.invalid", "seller note", "Juniper receipt-linked", "linen apron arrived"} {
				if strings.Contains(invoke(true, "SELECT data FROM resources"), secret) {
					t.Fatalf("SQL leaked %s", secret)
				}
			}
			for _, query := range []string{"Monogram", "Test", "Juniper receipt-linked", "arrived", "synthetic"} {
				output := invoke(false, query)
				for _, secret := range []string{"Test Buyer", "Monogram AB", "synthetic@example.invalid", "seller note", "Juniper receipt-linked", "linen apron arrived"} {
					if strings.Contains(output, secret) {
						t.Fatalf("search leaked %s: %s", secret, output)
					}
				}
			}
		}
		text = invoke(false, "Juniper Ceramics")
		if !strings.Contains(text, "Juniper Ceramics mug") {
			t.Fatalf("listing search lost: %s", text)
		}
	}
	t.Setenv(BuyerPIIEnvVar, "")
	for _, q := range []string{`SELECT * FROM users`, `SELECT * FROM reviews`, `SELECT * FROM economic_operators`, `SELECT * FROM etsy_payment_checks`, `SELECT * FROM receipts`, `SELECT data FROM main.resources`, `SELECT data FROM "main".resources`, `SELECT data FROM [main].resources`, `SELECT data FROM /*comment*/ main.resources`, `SELECT * FROM resources_fts`, `SELECT * FROM transactions`, `SELECT * FROM user_addresses`, `SELECT * FROM shops_receipts_payments`, `SELECT * FROM sqlite_master`, `SELECT * FROM sqlite_schema`, `SELECT data FROM payments`, `SELECT data FROM receipts_fts`, `SELECT data FROM user_addresses_fts`} {
		text := invoke(true, q)
		if !strings.Contains(text, BuyerPIIEnvVar) || strings.Contains(text, "Test Buyer") {
			t.Fatalf("unguarded %q: %s", q, text)
		}
	}
	if text := invoke(true, "SELECT count(*) FROM market_reviews"); strings.Contains(text, BuyerPIIEnvVar) {
		t.Fatalf("aggregate blocked: %s", text)
	}

	t.Setenv(BuyerPIIEnvVar, "1")
	if text := invoke(true, `SELECT data FROM receipts`); !strings.Contains(text, "Test Buyer") {
		t.Fatalf("typed opt-in lost: %s", text)
	}
}

func TestEtsyBuyerSearchAllowlist(t *testing.T) {
	for _, etsyCase := range []string{"different blobs", "shared blob", "unknown type", "unmatched blob", "missing store", "chunk boundary"} {
		t.Run(etsyCase, func(t *testing.T) {
			resetMCPPathEnv(t)
			t.Setenv(BuyerPIIEnvVar, "")
			path, err := mcpDBPath()
			if err != nil {
				t.Fatal(err)
			}
			good := json.RawMessage(`{"title":"Juniper shared words","listing_id":1000000001}`)
			bad := json.RawMessage(`{"title":"Juniper shared words","name":"Test Buyer"}`)
			expected := 1
			inputs := []json.RawMessage{good, bad}
			if etsyCase != "missing store" {
				db, err := store.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Upsert("listings", "1000000001", good); err != nil {
					t.Fatal(err)
				}
				typ := "receipts"
				if etsyCase == "unknown type" {
					typ = "future_buyer_type"
				}
				if etsyCase == "shared blob" {
					bad = good
					inputs[1] = bad
					expected = 0
				}
				if etsyCase != "unmatched blob" {
					if err := db.Upsert(typ, "910001", bad); err != nil {
						t.Fatal(err)
					}
				}
				if etsyCase == "chunk boundary" {
					inputs = nil
					for i := 0; i < 101; i++ {
						inputs = append(inputs, good)
					}
					inputs = append(inputs, bad)
					expected = 101
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				expected = 0
			}
			out, changed := redactBuyerSearch(inputs)
			if out == nil || len(out) != expected || !changed {
				t.Fatalf("got %s changed=%v; expected %d", fmt.Sprint(out), changed, expected)
			}
			if expected > 0 {
				out, changed = redactBuyerSearch([]json.RawMessage{good})
				if len(out) != 1 || changed {
					t.Fatal("unchanged allowed result marked changed")
				}
			}
		})
	}
}

func TestEtsyEntityLookupsViewGuard(t *testing.T) {
	resetMCPPathEnv(t)
	etsyPath, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	etsyDB, err := store.Open(etsyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := etsyDB.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES ('receipts','Quinby Testbuyer','Quinby Testbuyer','synced'), ('users','Test Buyer','Test Buyer','synced'), ('listings','Juniper Ceramics','Juniper Ceramics','synced')`); err != nil {
		t.Fatal(err)
	}
	if err := etsyDB.Close(); err != nil {
		t.Fatal(err)
	}
	for _, etsyOpt := range []string{"", "1"} {
		t.Setenv(BuyerPIIEnvVar, etsyOpt)
		for etsyTable := range derivedGuardedTables {
			for _, etsyProjection := range []string{"kind", "kind, canonical"} {
				etsyReq := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": "SELECT " + etsyProjection + " FROM " + etsyTable}}}
				etsyResult, err := handleSQL(context.Background(), etsyReq)
				if err != nil {
					t.Fatal(err)
				}
				etsyText := mcpTextContent(t, etsyResult)
				if !strings.Contains(etsyText, "listings") || (etsyProjection != "kind" && !strings.Contains(etsyText, "Juniper Ceramics")) {
					t.Fatalf("PROBE_ENTITY_LOOKUPS_VIEW_LEAK: listing missing: %s", etsyText)
				}
				for _, etsyBuyer := range []string{"receipts", "users"} {
					if strings.Contains(etsyText, etsyBuyer) != (etsyOpt == "1") {
						t.Fatalf("PROBE_ENTITY_LOOKUPS_VIEW_LEAK: opt=%q: %s", etsyOpt, etsyText)
					}
				}
				if etsyProjection != "kind" && strings.Contains(etsyText, "Quinby Testbuyer") != (etsyOpt == "1") {
					t.Fatalf("PROBE_ENTITY_LOOKUPS_VIEW_LEAK: opt=%q: %s", etsyOpt, etsyText)
				}
			}
		}
	}
}

func TestEtsySearchAllowlistInsideSQL(t *testing.T) {
	resetMCPPathEnv(t)
	t.Setenv(BuyerPIIEnvVar, "")
	path, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for etsyIndex := 1; etsyIndex <= 60; etsyIndex++ {
		etsyData := fmt.Sprintf(`{"receipt_id":%d,"name":"Test Buyer","message":"linenapron order %d"}`, 910000+etsyIndex, etsyIndex)
		if err := db.Upsert("receipts", fmt.Sprint(910000+etsyIndex), []byte(etsyData)); err != nil {
			t.Fatal(err)
		}
	}
	for etsyIndex := 1; etsyIndex <= 5; etsyIndex++ {
		etsyData := fmt.Sprintf(`{"listing_id":%d,"title":"Juniper Ceramics linenapron %d"}`, 1000000000+etsyIndex, etsyIndex)
		if err := db.Upsert("listings", fmt.Sprint(1000000000+etsyIndex), []byte(etsyData)); err != nil {
			t.Fatal(err)
		}
	}
	etsyEnvelope, _ := etsySearchProbe(t, "linenapron", 50)
	if etsyEnvelope["count"] != float64(5) {
		t.Fatalf("PROBE_SEARCH_LIMIT_STARVED: expected five listings: %v", etsyEnvelope)
	}
	for _, etsyRow := range etsyEnvelope["results"].([]any) {
		if _, etsyOK := etsyRow.(map[string]any)["listing_id"]; !etsyOK {
			t.Fatalf("PROBE_SEARCH_LIMIT_STARVED: non-listing result: %v", etsyRow)
		}
	}
}

func TestEtsySearchEnvelopeIsNotAnOracle(t *testing.T) {
	resetMCPPathEnv(t)
	t.Setenv(BuyerPIIEnvVar, "")
	path, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Upsert("receipts", "910001", []byte(`{"receipt_id":910001,"name":"Test Buyer","message":"quinbytoken"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("listings", "1000000001", []byte(`{"listing_id":1000000001,"title":"Juniper Ceramics juniperonly"}`)); err != nil {
		t.Fatal(err)
	}
	etsyMatch, etsyMatchText := etsySearchProbe(t, "quinbytoken", 50)
	etsyMiss, etsyMissText := etsySearchProbe(t, "nomatchtokenzz", 50)
	if etsyMatch["buyer_pii"] != "filtered" || etsyMiss["buyer_pii"] != "filtered" || etsyMatch["note"] == nil || etsyMatch["note"] == "" || etsyMatch["note"] != etsyMiss["note"] {
		t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: inconsistent filtering disclosure: %v / %v", etsyMatch, etsyMiss)
	}
	for _, etsyText := range []string{etsyMatchText, etsyMissText} {
		if strings.Contains(etsyText, "quinbytoken") || strings.Contains(etsyText, "Test Buyer") {
			t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: buyer data leaked: %s", etsyText)
		}
	}
	for _, etsyEnvelope := range []map[string]any{etsyMatch, etsyMiss} {
		for _, etsyKey := range []string{"results", "count", "next_step"} {
			delete(etsyEnvelope, etsyKey)
		}
	}
	if len(etsyMatch) != len(etsyMiss) {
		t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: different key sets: %v / %v", etsyMatch, etsyMiss)
	}
	for etsyKey := range etsyMatch {
		if _, etsyOK := etsyMiss[etsyKey]; !etsyOK {
			t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: missing key %s", etsyKey)
		}
	}
	t.Setenv(BuyerPIIEnvVar, "1")
	etsyOptIn, etsyOptInText := etsySearchProbe(t, "quinbytoken", 50)
	if etsyOptIn["count"] != float64(1) || !strings.Contains(etsyOptInText, "Test Buyer") || !strings.Contains(etsyOptInText, "quinbytoken") {
		t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: opt-in receipt missing: %s", etsyOptInText)
	}
	for _, etsyKey := range []string{"buyer_pii", "note"} {
		if _, etsyOK := etsyOptIn[etsyKey]; etsyOK {
			t.Fatalf("PROBE_SEARCH_ENVELOPE_ORACLE: opt-in has %s", etsyKey)
		}
	}
}

func etsySearchProbe(t *testing.T, etsyQuery string, etsyLimit int) (map[string]any, string) {
	t.Helper()
	etsyRequest := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": etsyQuery, "limit": float64(etsyLimit)}}}
	etsyResult, err := handleSearch(context.Background(), etsyRequest)
	if err != nil {
		t.Fatal(err)
	}
	etsyText := mcpTextContent(t, etsyResult)
	var etsyEnvelope map[string]any
	if err := json.Unmarshal([]byte(etsyText), &etsyEnvelope); err != nil {
		t.Fatalf("decoding search envelope: %v: %s", err, etsyText)
	}
	return etsyEnvelope, etsyText
}
