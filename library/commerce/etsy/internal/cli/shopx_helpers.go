// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared read-side helpers for the Slice 3 "shop transcendence" commands
// (listings audit/expiring/dead-weight, inventory audit, money fees/reconcile,
// reviews digest, shop attention). Everything here reads the generic
// `resources` mirror (decoding each row's JSON `data`) because that is the one
// representation every sync path writes; the typed tables are projections of
// it and are incomplete for dependent resources (see build-log-s3.md).

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

// shopxNow is the clock; tests override it for deterministic windows.
var shopxNow = time.Now

// Resource-type groups: flat sync name first, then parent-scoped dependent names.
var (
	shopxListingTypes     = []string{"listings", "listings-featured"}
	shopxImageTypes       = []string{"listing_images"}
	shopxInventoryTypes   = []string{"listings_inventory", "listing_inventory"}
	shopxLedgerTypes      = []string{"ledger_entries"}
	shopxReceiptTypes     = []string{"receipts"}
	shopxPaymentTypes     = []string{"shops_receipts_payments", "payments"}
	shopxTransactionTypes = []string{"transactions", "listings_transactions"}
	shopxReviewTypes      = []string{"reviews", "listings_reviews"}
	shopxOperatorTypes    = []string{"economic_operators"}
)

// shopxOpenDB opens the local store read-only. A missing database file (never
// synced) returns (nil, nil): callers treat nil as an empty store.
func shopxOpenDB(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("etsy-pp-cli")
	}
	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}
	if !shopxFileExists(dbPath) {
		return nil, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	return db, nil
}

const shopxNoteShopUnset = "ETSY_SHOP_ID not set; shop reports need your shop id"

func shopxShopID(cmd *cobra.Command) string {
	if shop := strings.TrimSpace(cliutil.EnvOverride("ETSY_SHOP_ID")); shop != "" {
		return shop
	}
	path := ""
	if cmd != nil {
		if flag := cmd.Flag("config"); flag != nil {
			path = flag.Value.String()
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return ""
	}
	shop := strings.TrimSpace(cfg.TemplateVars["shop_id"])
	if shop == "shop_id_placeholder" {
		return ""
	}
	return shop
}

func shopxShopMatches(row map[string]any, shop string) bool {
	v, exists := row["shop_id"]
	if !exists || v == nil {
		return false
	}
	a, aok := shopxInt(v)
	b, bok := shopxInt(shop)
	if aok && bok {
		return a == b
	}
	return shopxStr(v) == strings.TrimSpace(shop)
}

func shopxForeignNote(n int) string {
	if n > 0 {
		return fmt.Sprintf("%d listing(s) from other shops ignored (stale marketplace rows in the local store)", n)
	}
	return ""
}

var shopxMemo map[string][]map[string]any

func shopxWithMemo(fn func()) {
	shopxMemo = make(map[string][]map[string]any)
	defer func() { shopxMemo = nil }()
	fn()
}

func shopxPrintNotes(cmd *cobra.Command, notes []string) {
	for _, note := range notes {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: "+note)
	}
}

// shopxRows drains every row of the given resource types (in order) into
// decoded JSON objects. A nil store or a missing table yields no rows.
func shopxRows(db *store.Store, types ...string) []map[string]any {
	key := fmt.Sprintf("%p", db) + "\x00" + strings.Join(types, "\x00")
	if shopxMemo != nil {
		if rows, ok := shopxMemo[key]; ok {
			return rows
		}
	}
	rows := make([]map[string]any, 0)
	if shopxMemo != nil {
		defer func() { shopxMemo[key] = rows }()
	}
	if db == nil {
		return rows
	}
	for _, rt := range types {
		_ = db.ListScan(rt, func(_ string, data json.RawMessage) bool {
			obj, err := store.DecodeJSONObject(data)
			if err == nil && obj != nil {
				rows = append(rows, obj)
			}
			return true
		})
	}
	return rows
}

func shopxInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return int64(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		if f, err := n.Float64(); err == nil {
			return int64(f), true
		}
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), true
		}
	}
	return 0, false
}

func shopxStr(v any) string {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case nil:
		return ""
	case float64:
		if s == math.Trunc(s) {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case json.Number:
		return s.String()
	case bool:
		return strconv.FormatBool(s)
	}
	return ""
}

// shopxBool reports (value, present). Strings "true"/"false"/"1"/"0" and 0/1
// numbers count as present booleans.
func shopxBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case float64:
		return b != 0, true
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "1":
			return true, true
		case "false", "0":
			return false, true
		}
	}
	return false, false
}

func shopxList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func shopxMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// shopxMoney decodes an Etsy Money object {amount, divisor, currency_code}.
// A bare number is treated as minor units with divisor 1.
func shopxMoney(v any) (amount, divisor int64, currency string, ok bool) {
	if m := shopxMap(v); m != nil {
		a, aok := shopxInt(m["amount"])
		if !aok {
			return 0, 0, "", false
		}
		d, dok := shopxInt(m["divisor"])
		if !dok || d <= 0 {
			d = 100
		}
		return a, d, shopxStr(m["currency_code"]), true
	}
	if a, aok := shopxInt(v); aok {
		return a, 1, "", true
	}
	return 0, 0, "", false
}

// shopxTS decodes an Etsy unix timestamp (seconds; milliseconds tolerated).
func shopxTS(v any) (time.Time, bool) {
	n, ok := shopxInt(v)
	if !ok || n <= 0 {
		return time.Time{}, false
	}
	if n > 1e12 {
		return time.UnixMilli(n).UTC(), true
	}
	return time.Unix(n, 0).UTC(), true
}

// shopxFirstTS returns the first decodable timestamp among keys.
func shopxFirstTS(m map[string]any, keys ...string) (time.Time, bool) {
	for _, k := range keys {
		if t, ok := shopxTS(m[k]); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

func shopxFirstInt(m map[string]any, keys ...string) (int64, bool) {
	for _, k := range keys {
		if n, ok := shopxInt(m[k]); ok {
			return n, true
		}
	}
	return 0, false
}

// shopxParseWindow parses a --since/--within style duration (7d, 4w, 36h).
func shopxParseWindow(name, val string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(val)
	if err != nil || d <= 0 {
		return 0, usageErr(fmt.Errorf("invalid --%s %q: use a positive duration such as 30d, 12w or 48h", name, val))
	}
	return d, nil
}

// shopxListingID returns the listing id of a listing row.
func shopxListingID(m map[string]any) int64 {
	if n, ok := shopxFirstInt(m, "listing_id", "id"); ok {
		return n
	}
	return 0
}

// shopxLinkedListingID returns the listing a dependent row (image, inventory,
// transaction, review) belongs to: listing_id, falling back to the stamped
// parent_id.
func shopxLinkedListingID(m map[string]any) int64 {
	if n, ok := shopxFirstInt(m, "listing_id", "parent_id"); ok {
		return n
	}
	return 0
}

// shopxListingsByID scopes all listing readers and keeps the freshest duplicate.
func shopxListingsByID(db *store.Store, shop string) (rows []map[string]any, byID map[int64]map[string]any, foreignDropped int) {
	rows = make([]map[string]any, 0)
	byID = make(map[int64]map[string]any)
	if shop == "" {
		return
	}
	foreign := make(map[int64]bool)
	for _, r := range shopxRows(db, shopxListingTypes...) {
		id := shopxListingID(r)
		if !shopxShopMatches(r, shop) {
			foreign[id] = true
			continue
		}
		if id == 0 {
			continue
		}
		if old, exists := byID[id]; exists {
			ts, ok := shopxFirstTS(r, "last_modified_timestamp", "updated_timestamp", "last_modified")
			prev, had := shopxFirstTS(old, "last_modified_timestamp", "updated_timestamp", "last_modified")
			if !ok || (had && !ts.After(prev)) {
				continue
			}
		}
		byID[id] = r
	}
	for _, r := range byID {
		rows = append(rows, r)
	}
	shopxSortByListingID(rows)
	return rows, byID, len(foreign)
}

// shopxIsActive is the single active rule: only explicit state "active", never NULL/missing.
func shopxIsActive(m map[string]any) bool {
	return strings.EqualFold(shopxStr(m["state"]), "active")
}

func shopxActiveListings(db *store.Store, shop string) ([]map[string]any, int) {
	all, _, dropped := shopxListingsByID(db, shop)
	out := make([]map[string]any, 0, len(all))
	for _, l := range all {
		if shopxIsActive(l) {
			out = append(out, l)
		}
	}
	return out, dropped
}

func shopxSortByListingID(rows []map[string]any) {
	type keyed struct {
		id  int64
		row map[string]any
	}
	keys := make([]keyed, len(rows))
	for i, row := range rows {
		keys[i] = keyed{shopxListingID(row), row}
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].id < keys[j].id })
	for i, key := range keys {
		rows[i] = key.row
	}
}

// shopxFamilySynced reports whether any resource in the family has a valid sync.
func shopxFamilySynced(db *store.Store, types []string) bool {
	for _, age := range shopxSyncAges(db, types) {
		if age != nil {
			return true
		}
	}
	return false
}

// shopxFamilyHasData distinguishes an unknown family from a synced empty one.
func shopxFamilyHasData(db *store.Store, types []string) bool {
	if shopxFamilySynced(db, types) {
		return true
	}
	if db == nil {
		return false
	}
	for _, typ := range types {
		var one int
		if db.DB().QueryRow("SELECT 1 FROM resources WHERE resource_type = ? LIMIT 1", typ).Scan(&one) == nil {
			return true
		}
	}
	return false
}

// shopxSyncAges returns seconds since each resource last synced; a resource
// that never synced (or has no sync_state row / table) maps to nil (unknown),
// never to 0.
func shopxSyncAges(db *store.Store, resources []string) map[string]*int64 {
	out := make(map[string]*int64, len(resources))
	for _, r := range resources {
		out[r] = nil
	}
	if db == nil {
		return out
	}
	now := shopxNow()
	for _, r := range resources {
		var ts sql.NullTime
		err := db.DB().QueryRow(`SELECT last_synced_at FROM sync_state WHERE resource_type = ?`, r).Scan(&ts)
		if err != nil || !ts.Valid || ts.Time.IsZero() || ts.Time.Year() < 2000 {
			continue
		}
		age := int64(now.Sub(ts.Time).Seconds())
		if age < 0 {
			age = 0
		}
		out[r] = &age
	}
	return out
}

var shopxZeroDecimal = map[string]bool{"JPY": true, "KRW": true, "VND": true, "CLP": true, "ISK": true, "UGX": true}

// shopxFormatMinor renders minor units as a decimal string for the currency.
func shopxFormatMinor(amount int64, currency string) string {
	if shopxZeroDecimal[strings.ToUpper(currency)] {
		return strconv.FormatInt(amount, 10)
	}
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	return fmt.Sprintf("%s%d.%02d", sign, amount/100, amount%100)
}

// shopxFamilyTypes resolves a command's family name to its stored resource types.
func shopxFamilyTypes(name string) []string {
	switch name {
	case "listings":
		return shopxListingTypes
	case "listings_inventory":
		return shopxInventoryTypes
	case "ledger_entries":
		return shopxLedgerTypes
	case "receipts":
		return shopxReceiptTypes
	case "transactions":
		return shopxTransactionTypes
	case "reviews":
		return shopxReviewTypes
	case "listing_images":
		return shopxImageTypes
	default:
		return []string{name}
	}
}

func shopxFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// shopxReceiptNeedsShipping uses the first present payment/shipping flag.
func shopxReceiptNeedsShipping(m map[string]any) bool {
	paid, _ := s2FirstTrue(m, "is_paid", "was_paid")
	shipped, _ := s2FirstTrue(m, "is_shipped", "was_shipped")
	status := strings.NewReplacer("_", " ", "-", " ").Replace(strings.ToLower(shopxStr(m["status"])))
	return paid && !shipped && status != "canceled" && status != "cancelled" && status != "fully refunded"
}

// EtsyWritableResourceTypes lists every resource type sync or write-through can write into the local store
// (flat + dependent sync names, sync-owned partitions, embedded/typed families, marketplace_listings). Sorted, deduped.
// Used by the MCP buyer-PII classification test; a new type must be classified there.
func EtsyWritableResourceTypes() []string {
	etsyNames := append([]string{}, knownSyncResourceNames()...)
	for _, etsyDep := range dependentResourceDefs() {
		etsyNames = append(etsyNames, etsyDep.Name)
	}
	for etsyName := range syncOwnedPartitions {
		etsyNames = append(etsyNames, etsyName)
	}
	etsyNames = append(etsyNames, "app", "users", "marketplace_listings", "listings_inventory", "listing_images", "listing_files", "listing_videos", "listing_translations", "listing_variation_images", "holiday_preferences", "processing_profiles", "reviews")
	etsySet := make(map[string]bool)
	for _, etsyName := range etsyNames {
		etsySet[etsyName] = true
		etsySet[localCanonicalResourceType(etsyName)] = true
	}
	etsyOut := make([]string, 0, len(etsySet))
	for etsyName := range etsySet {
		etsyOut = append(etsyOut, etsyName)
	}
	sort.Strings(etsyOut)
	return etsyOut
}
