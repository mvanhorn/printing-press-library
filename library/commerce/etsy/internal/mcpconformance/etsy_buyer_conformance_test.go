package mcpconformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

var etsyBin string

// Keep the one built binary alive across both top-level tests; a test-owned
// TempDir would be removed before the opt-in arm runs.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("PP_MCP_MAX_BYTES"); err != nil {
		panic(err)
	}
	bound.MaxBytes = bound.Budget()
	etsyDir, etsyErr := os.MkdirTemp("", "etsy-conformance-")
	if etsyErr != nil {
		fmt.Fprintln(os.Stderr, etsyErr)
		os.Exit(1)
	}
	etsyBin = filepath.Join(etsyDir, "etsy-pp-cli")
	etsyCmd := exec.Command("go", "build", "-o", etsyBin, "../../cmd/etsy-pp-cli")
	etsyCmd.Dir = "."
	etsyCmd.Env = os.Environ()
	etsyOutput, etsyErr := etsyCmd.CombinedOutput()
	etsyCode := 1
	if etsyErr != nil {
		fmt.Fprintf(os.Stderr, "build companion: %v: %s", etsyErr, etsyOutput)
	} else {
		etsyCode = m.Run()
	}
	os.RemoveAll(etsyDir)
	if etsyMCPBinPath != "" {
		os.RemoveAll(filepath.Dir(etsyMCPBinPath))
	}
	os.Exit(etsyCode)
}

const etsyShopID = "9000001"

const etsyLastPageSentinel = "QZXLASTPAGEBUYER"

var etsySentinels = []string{etsyLastPageSentinel, "QZXBUYERNAME", "qzx@buyer.invalid", "QZXPERSONAL", "QZXADDR", "QZXMSG"}
var etsyBuyerTypes = store.EtsyBuyerTypes()

func etsyUnset(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func etsySetup(t *testing.T) *atomic.Int64 {
	t.Helper()
	t.Setenv("ETSY_CLI_PATH", etsyBin)
	etsyExe, etsyErr := os.Executable()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if _, etsyErr := os.Stat(filepath.Join(filepath.Dir(etsyExe), "etsy-pp-cli")); !os.IsNotExist(etsyErr) {
		t.Fatal("sibling companion could shadow ETSY_CLI_PATH")
	}
	etsyRestore, etsyErr := cliutil.SetHomeOverride("")
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(etsyRestore)
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	for _, etsyKey := range []string{"ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_SHOP_ID", "ETSY_OAUTH2", "PRINTING_PRESS_VERIFY", mcp.BuyerPIIEnvVar} {
		etsyUnset(t, etsyKey)
	}
	t.Setenv("ETSY_NO_LEARN", "false")
	// Shop reports short-circuit without a shop id, which would make every one of them
	// emit nothing whether or not it guards buyer data.
	t.Setenv("ETSY_SHOP_ID", etsyShopID)
	etsyCalls := new(atomic.Int64)
	etsyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		etsyCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(etsyServer.Close)
	t.Setenv("ETSY_BASE_URL", etsyServer.URL)
	etsySeedBuyerStore(t)
	return etsyCalls
}

func etsySeedBuyerStore(t *testing.T) {
	t.Helper()
	// Registers the CLI query synonyms so seeded learnings are keyed the way recall folds them.
	cli.RootCmd()
	etsyPath, etsyErr := cli.MCPStorePath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyDB, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyDB.Close()
	etsyCheck := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	etsyData := json.RawMessage(`{"receipt_id":910001,"transaction_id":930001,"user_address_id":940001,"payment_id":950001,"user_id":920001,"review_id":960001,"listing_id":1000000001,"economic_operator_id":970001,"shop_id":9000001,"name":"QZXBUYERNAME","first_name":"QZXBUYERNAME","primary_email":"qzx@buyer.invalid","email":"qzx@buyer.invalid","payment_email":"qzx@buyer.invalid","address":"QZXADDR","first_line":"QZXADDR","message":"QZXMSG","message_from_buyer":"QZXMSG","review":"QZXMSG","personalization":"QZXPERSONAL","title":"QZXPERSONAL"}`)
	for _, etsyType := range etsyBuyerTypes {
		etsyCheck(etsyDB.Upsert(etsyType, "910001", etsyData))
	}
	for _, etsyUpsert := range []func(json.RawMessage) error{etsyDB.UpsertReceipts, etsyDB.UpsertTransactions, etsyDB.UpsertUserAddresses, etsyDB.UpsertPayments, etsyDB.UpsertReviews, etsyDB.UpsertUsers, etsyDB.UpsertEconomicOperators} {
		etsyCheck(etsyUpsert(etsyData))
	}
	for _, etsyLearning := range []store.UpsertLearningInput{
		{Query: "open order zz", QueryEntities: []string{"zz"}, ResourceID: "910001", ResourceType: "receipts", Source: store.LearningSourceTaught},
		{Query: "open user zz", QueryEntities: []string{"zz"}, ResourceID: "920001", ResourceType: "users", Source: store.LearningSourceTaught},
		{Query: "linen apron", ResourceID: "1000000001", ResourceType: "listings", Source: store.LearningSourceTaught},
		// CLI-taught buyer learnings whose query text itself carries the buyer name (the CLI
		// accepts these; only the MCP teach path refuses them). Learnings list, candidates,
		// playbooks and the teach log are read by tools, so each needs a sentinel of its own.
		{Query: "orders for QZXBUYERNAME", ResourceID: "910002", ResourceType: "receipts", Source: store.LearningSourceTaught, Notes: "QZXMSG"},
		{Query: "open user QZXBUYERNAME", ResourceID: "920002", ResourceType: "users", Source: store.LearningSourceTaught},
	} {
		_, _, etsyErr = etsyDB.UpsertLearning(context.Background(), etsyLearning)
		etsyCheck(etsyErr)
	}
	// Two open candidates carry the sentinel in their family and payload. The ids are fixed so the
	// confirm and reject probes can name them; a fresh store hands out 1 and 2 in insert order.
	for etsyIdx, etsySig := range []string{"playbook_candidate|qzxbuyername", "playbook_candidate|qzxbuyername users"} {
		etsyRow, _, etsyDeriveErr := etsyDB.DeriveCandidate(store.CandidateClassPlaybookCandidate,
			`{"query_family":"qzxbuyername","notes_text":"QZXMSG candidate note"}`, etsySig, "qzxbuyername", "")
		etsyCheck(etsyDeriveErr)
		if etsyRow.ID != int64(etsyIdx+1) {
			t.Fatalf("candidate %q got id %d, want %d: the confirm/reject probe ids are stale", etsySig, etsyRow.ID, etsyIdx+1)
		}
	}
	_, _, etsyErr = etsyDB.UpsertPlaybook(store.UpsertPlaybookInput{QueryFamily: "qzxbuyername", NotesText: "QZXMSG buyer playbook note"})
	etsyCheck(etsyErr)
	// teach.log warnings carry query/resource entities and no resource type.
	etsyCheck(learn.AppendTeachLogWarning("teach", "synthetic query", learn.Warning{Code: "no_entity_overlap", Resource: "910002", Detail: "resource entities [QZXBUYERNAME]"}))
	etsyCheck(learn.AppendTeachLogWarning("teach", "synthetic query", learn.Warning{Code: "no_entity_overlap", Resource: "1000000001", Detail: "resource entities [linen apron]"}))
	// A taught (not synced) buyer lookup survives the synced-lookup purge, and a pattern keyed on it
	// lets recall substitute the sentinel into a resource id for the non-sentinel query "open order zz".
	etsyCheck(lookups.Upsert(etsyDB.DB(), "receipts", "zz", "QZXPERSONAL", "taught"))
	_, _, etsyErr = patterns.Upsert(etsyDB.DB(), patterns.Pattern{QueryTemplate: "open order {entity}", ResourceTemplate: "{entity:receipts}", ResourceType: "receipts", Strategy: patterns.StrategySubstitute, EntityKind: "receipts", Confidence: 2, Source: "taught"})
	etsyCheck(etsyErr)
	etsyCheck(store.EnsurePaymentChecksSchema(etsyDB.DB()))
	for _, etsyID := range []string{"910001", "QZXPERSONAL-910001"} {
		_, etsyErr = etsyDB.DB().Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at) VALUES (?,0,0)`, etsyID)
		etsyCheck(etsyErr)
	}
	etsyNow := time.Now()
	// Passes the report filters: paid, not shipped, not canceled, created recently, in this shop.
	etsyReceipt := json.RawMessage(fmt.Sprintf(`{"receipt_id":910002,"shop_id":%s,"is_paid":true,"is_shipped":false,"status":"paid","create_timestamp":%d,"country_iso":"US","grandtotal":{"amount":4200,"divisor":100,"currency_code":"USD"},"transactions":[{"transaction_id":930002}],"name":"QZXBUYERNAME","buyer_email":"qzx@buyer.invalid","buyer_user_id":920002,"first_line":"QZXADDR","city":"QZXADDR","message_from_buyer":"QZXMSG","personalization":"QZXPERSONAL"}`, etsyShopID, etsyNow.Add(-72*time.Hour).Unix()))
	etsyCheck(etsyDB.Upsert("receipts", "910002", etsyReceipt))
	etsyCheck(etsyDB.UpsertReceipts(etsyReceipt))
	// Enough bounded synthetic orders to exercise continuation in both arms.
	for i := 2; i <= 25; i++ {
		var row map[string]any
		etsyCheck(json.Unmarshal(etsyReceipt, &row))
		row["receipt_id"] = 1000000000 + i
		// orders_unshipped sorts oldest first. Only the newest receipt has
		// this sentinel, so a walk stopped after page 1 cannot find it.
		row["create_timestamp"] = etsyNow.Add(time.Duration(-72+i) * time.Hour).Unix()
		if i == 25 {
			row["name"] = etsyLastPageSentinel
		}
		raw, err := json.Marshal(row)
		etsyCheck(err)
		etsyCheck(etsyDB.Upsert("receipts", fmt.Sprint(1000000000+i), raw))
		etsyCheck(etsyDB.UpsertReceipts(raw))
	}
	// A low-rated review with text inside the 30d digest window.
	etsyReview := json.RawMessage(fmt.Sprintf(`{"shop_id":%s,"listing_id":1000000001,"transaction_id":930002,"rating":2,"review":"QZXMSG","create_timestamp":%d,"language":"en"}`, etsyShopID, etsyNow.Add(-120*time.Hour).Unix()))
	etsyCheck(etsyDB.Upsert("reviews", "930002", etsyReview))
	etsyCheck(etsyDB.UpsertReviews(etsyReview))
	for _, etsySyncType := range []string{"receipts", "reviews", "listings", "transactions", "listings_inventory", "shops_receipts_payments", "ledger_entries"} {
		etsyCheck(etsyDB.SaveSyncState(etsySyncType, "", 1))
	}
	etsyListing := json.RawMessage(fmt.Sprintf(`{"listing_id":1000000001,"shop_id":%s,"title":"Juniper Ceramics linen apron","state":"active","quantity":3,"price":{"amount":2500,"divisor":100,"currency_code":"USD"},"ending_timestamp":%d}`, etsyShopID, etsyNow.Add(240*time.Hour).Unix()))
	etsyCheck(etsyDB.Upsert("listings", "1000000001", etsyListing))
	etsyCheck(etsyDB.UpsertListings(etsyListing))
}

// etsyReseedLookups restores the derived buyer lookups before each probe: recall purges
// them, and a purged table would turn the entity_lookups sentinel into a dead one.
func etsyReseedLookups(t *testing.T) {
	t.Helper()
	etsyPath, etsyErr := cli.MCPStorePath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyDB, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyDB.Close()
	for _, etsyKind := range []string{"receipts", "users", "buyer_names"} {
		if etsyErr = lookups.Upsert(etsyDB.DB(), etsyKind, "QZXBUYERNAME", "QZXPERSONAL", "synced"); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
}

type etsyTool struct {
	Name        string `json:"name"`
	InputSchema struct {
		Required   []string       `json:"required"`
		Properties map[string]any `json:"properties"`
	} `json:"inputSchema"`
}

func etsyRPC(t *testing.T, s *server.MCPServer, method string, params any) json.RawMessage {
	t.Helper()
	etsyRaw, etsyErr := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyCtx, etsyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer etsyCancel()
	etsyResult, etsyErr := json.Marshal(s.HandleMessage(etsyCtx, etsyRaw))
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	return etsyResult
}

func etsyTools(t *testing.T) (*server.MCPServer, map[string]etsyTool) {
	t.Helper()
	etsyServer := server.NewMCPServer("etsy", "test")
	mcp.RegisterTools(etsyServer)
	var etsyResponse struct {
		Result struct {
			Tools []etsyTool `json:"tools"`
		} `json:"result"`
	}
	if etsyErr := json.Unmarshal(etsyRPC(t, etsyServer, "tools/list", map[string]any{}), &etsyResponse); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if len(etsyResponse.Result.Tools) == 0 || len(etsyResponse.Result.Tools) != len(etsyServer.ListTools()) {
		t.Fatal("tools/list disagrees with registration")
	}
	etsyOut := map[string]etsyTool{}
	for _, etsyEntry := range etsyResponse.Result.Tools {
		etsyOut[etsyEntry.Name] = etsyEntry
	}
	return etsyServer, etsyOut
}

// The caller passes the union when auditing stale skips across registration modes.
func etsyPartitionTools(registered []string, argTable map[string][]map[string]any, skips map[string]string, noArgOK func(string) bool) (run, skipped, problems []string) {
	etsySeen := map[string]bool{}
	for _, etsyName := range registered {
		etsySeen[etsyName] = true
		_, etsyArgs := argTable[etsyName]
		etsyReason, etsySkip := skips[etsyName]
		if etsySkip {
			skipped = append(skipped, etsyName)
			if strings.TrimSpace(etsyReason) == "" {
				problems = append(problems, "empty skip reason: "+etsyName)
			}
			if etsyArgs {
				problems = append(problems, "both run and skipped: "+etsyName)
			}
			continue
		}
		if etsyArgs || noArgOK(etsyName) {
			run = append(run, etsyName)
		} else {
			problems = append(problems, "uncovered tool: "+etsyName)
		}
	}
	for etsyName := range skips {
		if !etsySeen[etsyName] {
			problems = append(problems, "stale skip: "+etsyName)
		}
	}
	sort.Strings(run)
	sort.Strings(skipped)
	sort.Strings(problems)
	return
}

func TestEtsyConformancePartitionRules(t *testing.T) {
	for _, etsyCase := range []struct {
		name  string
		names []string
		args  map[string][]map[string]any
		skips map[string]string
		bad   bool
	}{
		{"uncovered", []string{"a"}, nil, nil, true},
		{"stale", nil, nil, map[string]string{"a": "Live endpoint has no offline fixture."}, true},
		{"overlap", []string{"a"}, map[string][]map[string]any{"a": {{}}}, map[string]string{"a": "Live endpoint has no offline fixture."}, true},
		{"empty", []string{"a"}, nil, map[string]string{"a": " "}, true},
		{"clean", []string{"a", "b", "c"}, map[string][]map[string]any{"a": {{}}}, map[string]string{"b": "Live endpoint has no offline fixture."}, false},
	} {
		t.Run(etsyCase.name, func(t *testing.T) {
			_, _, etsyProblems := etsyPartitionTools(etsyCase.names, etsyCase.args, etsyCase.skips, func(n string) bool { return n == "c" })
			if (len(etsyProblems) > 0) != etsyCase.bad {
				t.Fatalf("problems=%v", etsyProblems)
			}
		})
	}
}

var etsyToolArgs = map[string][]map[string]any{
	"etsy_search": {{"query": "receipts"}},
	"etsy_get":    {{"endpoint_id": "listings.get"}},
	// Confirm and reject name the seeded sentinel candidates (ids 1 and 2) so each can echo one.
	"learnings_confirm":  {{"id": "1"}},
	"learnings_reject":   {{"id": "2"}},
	"learnings_list":     {{}, {"warnings": true}},
	"learnings_forget":   {{"query": "absent synthetic learning", "all": true}},
	"teach_playbook":     {{"query": "linen apron", "notes": "Synthetic conformance note."}},
	"playbook_amend":     {{"query": "linen apron", "add-note": "Synthetic conformance note."}},
	"listings_snapshot":  {{"list": true, "bodies": true}},
	"listings_bulk_edit": {{"all": true, "field": "title", "set": []string{"Synthetic apron"}}},

	"sql": {{"query": "SELECT data FROM resources"}, {"query": "SELECT * FROM entity_lookups"}, {"query": "SELECT * FROM receipts"}, {"query": "SELECT * FROM etsy_payment_checks"}, {"query": "SELECT name FROM sqlite_master"},
		// The learn tables hold taught buyer text and recall query text; the SQL guard must cover them too.
		{"query": "SELECT * FROM search_learnings"}, {"query": "SELECT * FROM learn_candidates"}, {"query": "SELECT * FROM learning_playbooks"}, {"query": "SELECT * FROM search_patterns"}, {"query": "SELECT * FROM learn_recall_misses"}},
	"search":        {{"query": "QZXBUYERNAME"}, {"query": "qzx"}, {"query": "linen apron"}},
	"recall":        {{"query": "open order zz"}, {"query": "open order zz", "debug-mismatches": true}, {"query": "open user zz"}},
	"teach":         {{"agent": true, "query": "open order zz", "resource-type": "receipts", "resource": []string{"910001"}}, {"agent": true, "query": "open user zz", "resource-type": "users", "resource": []string{"920001"}}},
	"teach_pattern": {{"query-template": "open order {entity}", "resource-template": "{entity:receipts}", "resource-type": "receipts", "entity-kind": "receipts"}},
	// The probe input carries no sentinel: teach_lookup echoes its own arguments, and only
	// the seeded store rows (which hold the sentinels) may be a leak source.
	"teach_lookup": {{"kind": "receipts", "canonical": "Juniper Ceramics", "value": "x"}},

	// Each buyer-gated flag is passed explicitly: a bare call never reaches the guard.
	"orders_unshipped": {{}, {"full": true}},
	"reviews_digest":   {{}, {"text": true}},
	"analytics":        {{}, {"type": "receipts"}, {"type": "receipts", "group-by": "name"}, {"type": "users", "group-by": "name"}},
}
var etsySkips = map[string]string{
	"etsy_read":         "Live endpoint reads have no offline fixture.",
	"etsy_execute":      "Endpoint execution writes to the live API.",
	"sync":              "Sync fetches the live API; the sentinel-serving sync fakes (syncCLIServer, newMktFakeEtsy) are unexported helpers in package cli and cannot be imported here.",
	"export":            "Export fetches the resource from the live API.",
	"market_snapshot":   "Market snapshot fetches live search results and reviews.",
	"listings_copy":     "Listing copy reads live listing details before preparing writes.",
	"inventory_set_qty": "Quantity updates read live inventory before replacing it.",
	"import":            "Import issues live POST requests and its file input is unavailable over MCP.",
}

// etsyWriteGuarded tools have no buyer-reading output: their buyer guard refuses the WRITE.
// With the opt-in on, every probe must reach the implementation and record.
var etsyWriteGuarded = map[string]bool{"teach": true, "teach_pattern": true}

// etsyNonBuyerReading lists every walked tool that emits no buyer sentinel even with the
// buyer opt-in on, with the reason. The opt-in arm fails when a walked tool is in neither
// this map nor the set that emitted a sentinel, and when an entry here does emit one.
//
// Every entry was audited against the source (round 9). Each reason names what the tool reads
// and why a sentinel cannot reach its output. The seed holds a sentinel in EVERY buyer-bearing
// table and learn table a tool reads (receipts, transactions, users, reviews, user_addresses,
// economic_operators, payments, etsy_payment_checks, entity_lookups, search_learnings,
// learn_candidates, learning_playbooks, teach.log), so an entry never rests on "the table was
// empty". A tool that reads learnings, candidates, playbooks or lookups and emits their text is
// NOT here: it must fire (learnings_list, learnings_candidates, learnings_confirm,
// learnings_reject, playbook_list). A new entry needs a source-checked reason, not prose.
var etsyNonBuyerReading = map[string]string{
	// agent_context.go reads no store: static command tree, auth and config metadata.
	"context": "Prints static CLI and auth metadata (agent_context.go); opens no store.",
	// internal/mcp/code_orch.go: registry-only lookup over the embedded endpoint catalog.
	"etsy_get": "Describes one endpoint from the embedded catalog; opens no store.",
	// internal/mcp/code_orch.go: ranks the embedded endpoint catalog by the caller's query text.
	"etsy_search": "Searches the embedded endpoint catalog; opens no store.",
	// inventory_audit.go: shopxListingsByID (listings) and shopxInventoryTypes rows, projected to
	// listing id, sku-level stock and price spread fields.
	"inventory_audit": "Reads listings and listing-inventory rows; projects listing ids, stock and price fields.",
	// inventory_low_stock.go: SELECTs on listings (id, title), listing_inventory and
	// resources(listings_inventory) only.
	"inventory_low_stock": "Reads listings and listing inventory only.",
	// learnings_forget: forgetLearningsRows deletes by the caller's own query; its output is
	// {deleted, query} where query is the caller's argument. It reads no stored text (the
	// delete count is an existence signal for a query the caller already supplied).
	"learnings_forget": "Deletes learnings matching the caller's own query; echoes only that query and a count.",
	// learnings_stats.go: LearnEventStats over learn_events, whose rows hold an event type, a
	// family HASH and ids; counters and rates only, no query text.
	"learnings_stats": "Prints learn_events counters and rates; the table stores family hashes, never text.",
	// listings_audit.go: listings, listing_images, and economic_operators rows read for the
	// operator id only (op["economic_operator_id"]); findings carry listing ids, titles, rule names.
	"listings_audit": "Reads listings, images and economic-operator ids; projects listing ids, titles and rule findings.",
	// listings_bulk_edit.go: listings, listing_images (alt text) and bulk_edit_items (op_key,
	// listing_id, kind, status); previews edits without confirmation over MCP.
	"listings_bulk_edit": "Reads listings, images and bulk_edit_items; previews edits without confirmation over MCP.",
	// listings_dead_weight.go: transactions rows are read for listing id and timestamps only;
	// shopxDeadWeightRow (listing_id, title from listings, favorers, views, last_sale_at).
	"listings_dead_weight": "Reads listings and transaction listing ids and timestamps; the row type carries no buyer field.",
	// listings_expiring.go: listings only.
	"listings_expiring": "Reads listings only.",
	// listings_snapshot.go: listing_snapshots and listings bodies; seller-authored listing JSON.
	"listings_snapshot": "Reads listing snapshots and listing bodies only.",
	// market_*.go: mktOpenRead over market_snapshots/market_results/market_shops/market_reviews.
	// DDL (etsy_market_migrations.go): market_reviews holds only snapshot_id, listing_id, created_ts
	// and rating; no review text or buyer column exists. Other shops' public listing metadata.
	"market_demand": "Reads market_* snapshot tables: other shops' public listings; market_reviews has only a timestamp and rating column.",
	"market_gaps":   "Reads market_* snapshot tables: other shops' public listings; market_reviews has only a timestamp and rating column.",
	"market_price":  "Reads market_* snapshot tables: other shops' public listings; market_reviews has only a timestamp and rating column.",
	"market_shops":  "Reads market_* snapshot tables: other shops' public listings and shop counters; no buyer column exists.",
	"market_tags":   "Reads market_* snapshot tables: other shops' public listing tags; no buyer column exists.",
	"market_trend":  "Reads market_* snapshot tables: other shops' public listings; market_reviews has only a timestamp and rating column.",
	// money_fees.go / money_summary.go: ledger_entries only (amount, currency, ledger_type,
	// reference_type, timestamps); ledger_entries is on the MCP non-buyer allowlist.
	"money_fees": "Reads ledger_entries only.",
	// money_reconcile.go: reads receipts, payments, ledger_entries and transactions, which hold
	// sentinels in the seed; every result type (shopxReconcileReceipt/Payment/Reference/Gap) is
	// ids, amount_minor and currency, so no name, email, address or message can be projected.
	"money_reconcile": "Reads receipts, payments, ledger and transactions; result types carry ids and amounts only.",
	"money_summary":   "Reads ledger_entries only.",
	// channel_workflow.go (workflow status): Store.Status is SELECT resource_type, COUNT(*) FROM
	// resources GROUP BY resource_type; the output keys are type names and counts.
	"workflow_status": "Prints per-resource-type row counts (Store.Status); no row content.",
	// shop_attention.go: shopxAttentionResult is counts, a map of audit finding counts and sync
	// ages; it reads receipts for counts and projects no row content.
	"shop_attention": "Reads receipts, listings and ledger for counts and sync ages; projects no row content.",
	// teach_lookup: lookups.Upsert from the caller's own --kind/--canonical/--value, and the
	// output echoes those arguments. The probe carries no sentinel, so only seeded rows could
	// leak, and the command reads none.
	"teach_lookup": "Writes an entity lookup from its own arguments and echoes them; reads no stored rows.",
	// teach_playbook.go (teach-playbook): UpsertPlaybook, output echoes the family derived from
	// the caller's own query and has_playbook/has_notes booleans.
	"teach_playbook": "Writes a playbook note from its own arguments; echoes the family derived from the caller's query.",
	// playbook amend (teach_playbook.go): AppendPlaybookNotes from the caller's own arguments;
	// silent on success and its JSON echo is the caller's own query.
	"playbook_amend": "Appends a note from its own arguments; reads no stored text.",
}

// etsyEachProbeFires names tools whose probes read DIFFERENT stores, so each probe must expose its
// own sentinel with the opt-in on. Without this, one probe firing would hide a dead seed behind
// another (learnings_list reads search_learnings by default and teach.log with --warnings).
var etsyEachProbeFires = map[string]bool{"learnings_list": true}

// etsyFloors is the least number of tools each registration mode must exercise, so a
// registration regression that empties a mode cannot pass silently.
var etsyFloors = map[string]int{"readOnly": 20, "readWrite": 20}

func etsyFloorProblem(mode string, exercised int) string {
	if floor, ok := etsyFloors[mode]; ok && exercised < floor {
		return fmt.Sprintf("mode=%s exercised %d tools, fewer than the floor of %d", mode, exercised, floor)
	}
	return ""
}

func TestEtsyConformanceFloors(t *testing.T) {
	for _, etsyCase := range []struct {
		mode string
		n    int
		bad  bool
	}{{"readOnly", 19, true}, {"readOnly", 20, false}, {"readWrite", 19, true}, {"readWrite", 20, false}} {
		if got := etsyFloorProblem(etsyCase.mode, etsyCase.n) != ""; got != etsyCase.bad {
			t.Errorf("floor mode=%s n=%d: problem=%v want %v", etsyCase.mode, etsyCase.n, got, etsyCase.bad)
		}
	}
}

// etsyRecorded reports whether a teach-style tool output says the learning was recorded.
func etsyRecorded(text string) bool {
	return strings.Contains(text, `"recorded":true`) || strings.Contains(text, `"recorded": true`)
}

// etsyIsBuyerRefusal accepts only a failure attributable to the buyer guard.
//   - A failure whose text names the opt-in env var is the guard's own message.
//   - teach and teach-pattern set cmd.SilenceErrors, so over MCP their refusal carries no text
//     at all (the plan expected the marker in the error text; the code silences it). A silent
//     failure then counts only when the IDENTICAL probe records with the opt-in on, so the only
//     difference between the failing and the recording run is the guard.
//   - Any other failure (bad argument, missing store, a message without the marker) is unrelated
//     and must not pass as a refusal, however IsError reads.
func etsyIsBuyerRefusal(isError bool, text string, optInControlRecorded bool) bool {
	if !isError {
		return false
	}
	if strings.Contains(text, mcp.BuyerPIIEnvVar) {
		return true
	}
	return etsySilentFailure(text) && optInControlRecorded
}

var etsyExitOnly = regexp.MustCompile(`^cli \S+: exit status \d+$`)

// etsySilentFailure is true for a failure that carries no message beyond the shell-out
// wrapper's "cli <path>: exit status N".
func etsySilentFailure(text string) bool {
	etsyTrimmed := strings.TrimSpace(text)
	return etsyTrimmed == "" || etsyExitOnly.MatchString(etsyTrimmed)
}

func TestEtsyBuyerRefusalRequiresMarker(t *testing.T) {
	for _, etsyCase := range []struct {
		name    string
		isError bool
		text    string
		control bool
		want    bool
	}{
		{"refusal message", true, "teaching a receipts learning exposes buyer data over MCP; explicit opt-in requires ETSY_MCP_ALLOW_BUYER_PII=1", false, true},
		{"unrelated failure with message", true, "teach: missing --resource", true, false},
		{"silent failure the opt-in control also fails", true, "", false, false},
		{"exit-only failure the opt-in control also fails", true, "cli /tmp/etsy-pp-cli: exit status 2", false, false},
		{"silent refusal the opt-in control records", true, "", true, true},
		{"exit-only refusal the opt-in control records", true, "\ncli /tmp/etsy-pp-cli: exit status 2", true, true},
		{"marker without error", false, "ETSY_MCP_ALLOW_BUYER_PII", true, false},
		{"accepted probe", false, `{"recorded":true}`, true, false},
	} {
		if got := etsyIsBuyerRefusal(etsyCase.isError, etsyCase.text, etsyCase.control); got != etsyCase.want {
			t.Errorf("%s: refusal=%v want %v", etsyCase.name, got, etsyCase.want)
		}
	}
}

type etsyProbe struct {
	tool    string
	args    map[string]any
	text    string
	isError bool
	// controlRecorded is set for write-guarded tools in the default arm: the identical probe,
	// rerun with the opt-in on, recorded.
	controlRecorded bool
	nextCursor      string
}

func etsyCallPage(t *testing.T, s *server.MCPServer, tool etsyTool, args map[string]any) etsyProbe {
	t.Helper()
	for _, etsyRequired := range tool.InputSchema.Required {
		if _, etsyOK := args[etsyRequired]; !etsyOK {
			t.Fatalf("%s missing required argument %s", tool.Name, etsyRequired)
		}
	}
	for etsyKey := range args {
		if _, etsyOK := tool.InputSchema.Properties[etsyKey]; !etsyOK {
			t.Fatalf("%s has no schema property %s", tool.Name, etsyKey)
		}
	}
	var etsyResponse struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	etsyReseedLookups(t)
	etsyRaw := etsyRPC(t, s, "tools/call", map[string]any{"name": tool.Name, "arguments": args})
	if etsyErr := json.Unmarshal(etsyRaw, &etsyResponse); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyText := string(etsyResponse.Error)
	var nextCursor string
	for _, etsyPart := range etsyResponse.Result.Content {
		etsyText += "\n" + etsyPart.Text
		var page struct {
			NextCursor string `json:"next_cursor"`
		}
		if json.Unmarshal([]byte(etsyPart.Text), &page) == nil && page.NextCursor != "" {
			nextCursor = page.NextCursor
		}
	}
	return etsyProbe{nextCursor: nextCursor, tool: tool.Name, args: args, text: etsyText, isError: etsyResponse.Result.IsError || len(etsyResponse.Error) > 0}
}

// etsyCall scans the full logical response; aggregate pages so each probe's
// no-leak and could-have-fired assertions apply to every returned byte.
func etsyCall(t *testing.T, s *server.MCPServer, tool etsyTool, args map[string]any) etsyProbe {
	t.Helper()
	forwarded := map[string]any{}
	for key, value := range args {
		forwarded[key] = value
	}
	result := etsyCallPage(t, s, tool, forwarded)
	result.args = args
	firstText, lastText := result.text, result.text
	pages := 1
	cursor := result.nextCursor
	for page := 1; cursor != ""; page++ {
		if page >= 200 {
			t.Fatalf("%s paging exceeded 200 pages", tool.Name)
		}
		if _, ok := tool.InputSchema.Properties["cursor"]; !ok {
			t.Fatalf("%s emitted an unusable cursor", tool.Name)
		}
		forwarded["cursor"] = cursor
		next := etsyCallPage(t, s, tool, forwarded)
		if next.isError {
			t.Fatalf("%s page %d failed: %s", tool.Name, page, next.text)
		}
		t.Logf("PAGED tool=%s page=%d", tool.Name, page+1)
		pages++
		lastText = next.text
		result.text += "\n" + next.text
		cursor = next.nextCursor
	}
	if tool.Name == "orders_unshipped" && !result.isError {
		if pages < 2 {
			t.Fatalf("orders_unshipped must scan at least 2 pages, got %d", pages)
		}
		wantSentinel := os.Getenv(mcp.BuyerPIIEnvVar) == "1" && args["full"] == true
		if strings.Contains(result.text, etsyLastPageSentinel) != wantSentinel {
			t.Fatalf("last-page sentinel present=%v want=%v", strings.Contains(result.text, etsyLastPageSentinel), wantSentinel)
		}
		if wantSentinel && (strings.Contains(firstText, etsyLastPageSentinel) || !strings.Contains(lastText, etsyLastPageSentinel)) {
			t.Fatal("opt-in sentinel must occur only on the last page")
		}
	}
	return result
}

// etsyWalkMode registers the tools in one mode and runs every probe of every non-skipped tool.
func etsyWalkMode(t *testing.T, etsyMode string) (probes []etsyProbe, run, skipped []string, skipReasons map[string]string) {
	t.Setenv("PP_MCP_MAX_BYTES", "4000")
	t.Helper()
	etsyRO := ""
	if etsyMode == "readOnly" {
		etsyRO = "1"
	}
	t.Setenv(cliutil.MCPReadOnlyEnvVar, etsyRO)
	etsyServer, etsyRegistered := etsyTools(t)
	etsyNames := []string{}
	skipReasons = map[string]string{}
	for etsyName := range etsyRegistered {
		etsyNames = append(etsyNames, etsyName)
		if etsyReason, etsyOK := etsySkips[etsyName]; etsyOK {
			skipReasons[etsyName] = etsyReason
		}
	}
	etsyRun, etsySkipped, etsyProblems := etsyPartitionTools(etsyNames, etsyToolArgs, skipReasons, func(n string) bool { return len(etsyRegistered[n].InputSchema.Required) == 0 })
	if len(etsyProblems) > 0 {
		t.Fatal(etsyProblems)
	}
	etsyRank := func(n string) int {
		switch n {
		case "teach", "teach_pattern", "teach_lookup", "teach_playbook":
			return 0
		case "recall":
			return 1
		}
		return 2
	}
	sort.Slice(etsyRun, func(i, j int) bool {
		if etsyRank(etsyRun[i]) != etsyRank(etsyRun[j]) {
			return etsyRank(etsyRun[i]) < etsyRank(etsyRun[j])
		}
		return etsyRun[i] < etsyRun[j]
	})
	for _, etsyName := range etsyRun {
		etsyProbes, etsyOK := etsyToolArgs[etsyName]
		if !etsyOK {
			etsyProbes = []map[string]any{{}}
		}
		for _, etsyArgs := range etsyProbes {
			etsyResult := etsyCall(t, etsyServer, etsyRegistered[etsyName], etsyArgs)
			if etsyWriteGuarded[etsyName] && os.Getenv(mcp.BuyerPIIEnvVar) != "1" {
				t.Setenv(mcp.BuyerPIIEnvVar, "1")
				etsyResult.controlRecorded = etsyRecorded(etsyCall(t, etsyServer, etsyRegistered[etsyName], etsyArgs).text)
				t.Setenv(mcp.BuyerPIIEnvVar, "")
			}
			probes = append(probes, etsyResult)
		}
	}
	return probes, etsyRun, etsySkipped, skipReasons
}

func etsyLeakedSentinels(text string) []string {
	etsyLower := strings.ToLower(text)
	var etsyHits []string
	for _, etsySentinel := range etsySentinels {
		if strings.Contains(etsyLower, strings.ToLower(etsySentinel)) {
			etsyHits = append(etsyHits, etsySentinel)
		}
	}
	return etsyHits
}

func etsyRegisteredInRun(run []string, name string) bool {
	for _, etsyName := range run {
		if etsyName == name {
			return true
		}
	}
	return false
}

func etsyUnionRegistered(t *testing.T) map[string]etsyTool {
	t.Helper()
	etsyUnion := map[string]etsyTool{}
	for _, etsyRO := range []string{"1", ""} {
		t.Setenv(cliutil.MCPReadOnlyEnvVar, etsyRO)
		_, etsyModeTools := etsyTools(t)
		for etsyName, etsyEntry := range etsyModeTools {
			etsyUnion[etsyName] = etsyEntry
		}
	}
	return etsyUnion
}

func TestEtsyMCPBuyerConformance(t *testing.T) {
	etsySetup(t)
	etsyUnionTools := etsyUnionRegistered(t)
	etsyUnion := []string{}
	for etsyName := range etsyUnionTools {
		etsyUnion = append(etsyUnion, etsyName)
	}
	_, _, etsyProblems := etsyPartitionTools(etsyUnion, etsyToolArgs, etsySkips, func(n string) bool { return len(etsyUnionTools[n].InputSchema.Required) == 0 })
	if len(etsyProblems) > 0 {
		t.Fatal(etsyProblems)
	}
	for _, etsyMode := range []string{"readOnly", "readWrite"} {
		t.Run(etsyMode, func(t *testing.T) {
			etsyCalls := etsySetup(t)
			etsyProbes, etsyRun, etsySkipped, etsyReasons := etsyWalkMode(t, etsyMode)
			for _, etsyProbe := range etsyProbes {
				t.Logf("OUT %s %s %v isError=%v", etsyMode, etsyProbe.tool, etsyProbe.args, etsyProbe.isError)
				if etsyProbe.tool == "recall" && etsyProbe.args["query"] == "open order zz" && !strings.Contains(etsyProbe.text, "910001") {
					t.Errorf("recall mode=%s lost seeded receipt learning", etsyMode)
				}
				if (etsyProbe.tool == "sql" && etsyProbe.args["query"] == "SELECT data FROM resources") || (etsyProbe.tool == "search" && etsyProbe.args["query"] == "linen apron") {
					if !strings.Contains(etsyProbe.text, "Juniper Ceramics linen apron") {
						t.Errorf("%s mode=%s lost non-buyer control", etsyProbe.tool, etsyMode)
					}
				}
				if etsyProbe.tool == "teach" || etsyProbe.tool == "teach_pattern" {
					if !etsyIsBuyerRefusal(etsyProbe.isError, etsyProbe.text, etsyProbe.controlRecorded) {
						t.Errorf("%s mode=%s did not refuse buyer-bearing learning arguments %v (no guard-attributable refusal; opt-in control recorded=%v)", etsyProbe.tool, etsyMode, etsyProbe.args, etsyProbe.controlRecorded)
					}
				}
				for _, etsySentinel := range etsyLeakedSentinels(etsyProbe.text) {
					t.Errorf("buyer leak tool=%s mode=%s args=%v sentinel=%s", etsyProbe.tool, etsyMode, etsyProbe.args, etsySentinel)
				}
			}
			for _, etsyName := range etsySkipped {
				t.Logf("TABLE mode=%s tool=%s skip reason=%s", etsyMode, etsyName, etsyReasons[etsyName])
			}
			t.Logf("CONFORMANCE mode=%s run=%d skipped=%d", etsyMode, len(etsyRun), len(etsySkipped))
			if etsyProblem := etsyFloorProblem(etsyMode, len(etsyRun)); etsyProblem != "" {
				t.Error(etsyProblem)
			}
			if etsyCalls.Load() != 0 {
				t.Errorf("offline probes attempted %d HTTP calls", etsyCalls.Load())
			}
		})
	}
}

// TestEtsyMCPBuyerConformanceOptInSeesSentinels is the "could have fired" arm. With the buyer
// opt-in on, the default arm's guards are lifted, so a tool that still emits no sentinel proves
// nothing when the default arm passes. Every walked tool must therefore emit a sentinel or carry
// an honest nonBuyerReading entry, and an entry for a tool that does emit is stale.
func TestEtsyMCPBuyerConformanceOptInSeesSentinels(t *testing.T) {
	etsySetup(t)
	etsyUnionTools := etsyUnionRegistered(t)
	for etsyName, etsyReason := range etsyNonBuyerReading {
		if _, etsyOK := etsyUnionTools[etsyName]; !etsyOK {
			t.Errorf("stale nonBuyerReading entry for unregistered tool %s", etsyName)
		}
		if strings.TrimSpace(etsyReason) == "" {
			t.Errorf("empty nonBuyerReading reason for %s", etsyName)
		}
		if _, etsyOK := etsySkips[etsyName]; etsyOK {
			t.Errorf("tool %s is both skipped and in nonBuyerReading", etsyName)
		}
	}
	for _, etsyMode := range []string{"readOnly", "readWrite"} {
		t.Run(etsyMode, func(t *testing.T) {
			etsyCalls := etsySetup(t)
			t.Setenv(mcp.BuyerPIIEnvVar, "1")
			etsyProbes, etsyRun, etsySkipped, _ := etsyWalkMode(t, etsyMode)
			etsyFired := map[string][]string{}
			etsyRecordedN := map[string]int{}
			for _, etsyProbe := range etsyProbes {
				for _, etsySentinel := range etsyLeakedSentinels(etsyProbe.text) {
					etsyFired[etsyProbe.tool] = append(etsyFired[etsyProbe.tool], etsySentinel)
				}
				if etsyWriteGuarded[etsyProbe.tool] {
					if etsyRecorded(etsyProbe.text) {
						etsyRecordedN[etsyProbe.tool]++
					} else {
						t.Errorf("opt-in %s mode=%s did not record probe %v: %q", etsyProbe.tool, etsyMode, etsyProbe.args, etsyProbe.text)
					}
				}
			}
			for _, etsyProbe := range etsyProbes {
				if etsyEachProbeFires[etsyProbe.tool] && len(etsyLeakedSentinels(etsyProbe.text)) == 0 {
					t.Errorf("opt-in %s mode=%s probe %v exposed no seeded buyer sentinel: its seed is dead", etsyProbe.tool, etsyMode, etsyProbe.args)
				}
			}
			nFired, nReading, nGuard := 0, 0, 0
			for _, etsyName := range etsyRun {
				_, etsyIsReading := etsyNonBuyerReading[etsyName]
				switch {
				case etsyWriteGuarded[etsyName]:
					nGuard++
					if etsyRecordedN[etsyName] != len(etsyToolArgs[etsyName]) {
						t.Errorf("opt-in %s mode=%s recorded %d of %d probes", etsyName, etsyMode, etsyRecordedN[etsyName], len(etsyToolArgs[etsyName]))
					}
					t.Logf("FIRED mode=%s tool=%s => writeGuard(recorded %d/%d)", etsyMode, etsyName, etsyRecordedN[etsyName], len(etsyToolArgs[etsyName]))
				case len(etsyFired[etsyName]) > 0:
					nFired++
					if etsyIsReading {
						t.Errorf("stale nonBuyerReading entry tool=%s mode=%s: it emitted a buyer sentinel with the opt-in", etsyName, etsyMode)
					}
					t.Logf("FIRED mode=%s tool=%s => fired", etsyMode, etsyName)
				case etsyIsReading:
					nReading++
					t.Logf("FIRED mode=%s tool=%s => nonBuyerReading(%s)", etsyMode, etsyName, etsyNonBuyerReading[etsyName])
				default:
					t.Errorf("tool=%s mode=%s could not fire: no sentinel with the opt-in and no nonBuyerReading reason", etsyName, etsyMode)
				}
			}
			for _, etsyName := range etsySkipped {
				t.Logf("FIRED mode=%s tool=%s => skipped(%s)", etsyMode, etsyName, etsySkips[etsyName])
			}
			t.Logf("FIRED-COUNTS mode=%s run=%d fired=%d nonBuyerReading=%d writeGuard=%d skipped=%d", etsyMode, len(etsyRun), nFired, nReading, nGuard, len(etsySkipped))
			// The readers whose buyer guards the default arm relies on must demonstrably fire.
			for _, etsyName := range []string{"sql", "search", "recall", "orders_unshipped", "reviews_digest", "analytics", "learnings_list", "learnings_candidates", "learnings_confirm", "learnings_reject", "playbook_list"} {
				// confirm and reject are local writes: registered in readWrite mode only.
				if !etsyRegisteredInRun(etsyRun, etsyName) {
					if etsyMode == "readWrite" || etsyName != "learnings_confirm" && etsyName != "learnings_reject" {
						t.Errorf("opt-in %s mode=%s was not walked", etsyName, etsyMode)
					}
					continue
				}
				if len(etsyFired[etsyName]) == 0 {
					t.Errorf("opt-in %s mode=%s failed to expose any seeded buyer sentinel", etsyName, etsyMode)
				}
			}
			if etsyCalls.Load() != 0 {
				t.Errorf("offline probes attempted %d HTTP calls", etsyCalls.Load())
			}
		})
	}
}
