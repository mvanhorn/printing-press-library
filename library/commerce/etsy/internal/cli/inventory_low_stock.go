// Hand-authored (Slice 2): inventory low-stock. Reads the local store only.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

var etsyLowStockCallTimeout = 30 * time.Second
var etsyLowStockMCPCeiling = 10 * time.Minute
var etsyLowStockMCPMaxCalls = etsyMCPMaxCalls
var errEtsyLowStockCallCap = errors.New("inventory refresh stopped: MCP call cap reached")

type s2LowStockRow struct {
	ListingID  int64  `json:"listing_id"`
	Title      string `json:"title,omitempty"`
	ProductID  string `json:"product_id,omitempty"`
	SKU        string `json:"sku,omitempty"`
	OfferingID string `json:"offering_id,omitempty"`
	Quantity   int64  `json:"quantity"`
}

func newS2InventoryLowStockCmd(flags *rootFlags) *cobra.Command {
	var threshold, limit, refreshLimit int
	var dbPath string
	var refresh, ignoreBudget bool
	cmd := &cobra.Command{
		Use:   "low-stock",
		Short: "List offerings at or below a quantity threshold from the local store (no API calls).",
		Long: strings.Trim(`
Lists every live offering whose quantity is at or below --threshold, lowest
first. Deleted products, deleted offerings and disabled offerings are skipped.
Reads the local store only, so there are no API calls unless you pass
--refresh, which re-reads inventory in batches of 100 listings (one request
each), budget-checked and scoped to your configured shop; use it when 'sync' has not stored
inventory. For structural data problems (duplicate SKUs, zero-quantity
variations) use 'inventory audit'.
`, "\n"),
		Example: strings.Trim(`
  etsy-pp-cli inventory low-stock --threshold 3
  etsy-pp-cli inventory low-stock --threshold 0 --json
  etsy-pp-cli inventory low-stock --threshold 3 --refresh
`, "\n"),
		Annotations: map[string]string{"mcp:read-only-unless-flags": "refresh", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsMCPChildEnv() {
				if ignoreBudget {
					return usageErr(fmt.Errorf("--ignore-budget is not available over MCP"))
				}
				if refresh {
					etsyMCPClamp(cmd, "refresh-limit", &refreshLimit, 5000, true)
				}
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inventory low-stock")
			}
			if threshold < 0 {
				return usageErr(fmt.Errorf("--threshold must be 0 or more"))
			}
			if refresh && cmd.Context().Err() != nil {
				return fmt.Errorf("inventory refresh interrupted: %w after 0 of 0 listings", cmd.Context().Err())
			}
			if dbPath == "" {
				dbPath = defaultDBPath("etsy-pp-cli")
			}
			rows := make([]s2LowStockRow, 0)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: etsy-pp-cli sync --resources listings --db %s\n", dbPath, dbPath)
				}
				return s2PrintLowStock(cmd, flags, rows, []string{shopxNoteNoStore})
			}
			var db *store.Store
			var err error
			if refresh {
				db, err = store.OpenWithContext(cmd.Context(), dbPath)
			} else {
				db, err = store.OpenReadOnly(dbPath)
			}
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if refresh {
				if err := s2RefreshInventory(cmd, flags, db, refreshLimit, ignoreBudget); err != nil {
					return err
				}
			}
			shopxEmitHints(cmd, db, "listings_inventory")
			rows, notes, readErr := s2ScopedLowStockRows(db, shopxShopID(cmd), int64(threshold))
			err = readErr
			if err != nil {
				return err
			}
			if len(rows) == 0 && !refresh && s2CountRows(db, `SELECT COUNT(*) FROM listings`) > 0 && s2CountRows(db, `SELECT COUNT(*) FROM resources WHERE resource_type IN ('listings_inventory','listing_inventory')`) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: listings are synced but no inventory is stored locally (sync does not cache per-listing inventory); run with --refresh to read and cache it")
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return s2PrintLowStock(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().IntVar(&threshold, "threshold", 3, "Report offerings with quantity at or below this")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum rows to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh your configured shop's inventory in batches of 100 listings (budget-checked)")
	cmd.Flags().IntVar(&refreshLimit, "refresh-limit", 0, "With --refresh, refresh at most this many listings (0 = all)")
	cmd.Flags().BoolVar(&ignoreBudget, "ignore-budget", false, "With --refresh, run even if the calls exceed the remaining daily Etsy budget")
	return cmd
}

func s2CountRows(db *store.Store, q string) int {
	var n int
	if err := db.DB().QueryRow(q).Scan(&n); err != nil {
		return 0
	}
	return n
}

// s2RefreshInventory reads the inventory of every synced listing live and
// caches each product as a generic listings_inventory row keyed like sync does (product_id NUL listing_id).
// Fresh inventory replaces the listing's previously cached inventory rows.
func s2RefreshInventory(cmd *cobra.Command, flags *rootFlags, db *store.Store, limit int, ignoreBudget bool) error {
	shop := bulkShopIDForCmd(cmd, "")
	if shop == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: ETSY_SHOP_ID not set; shop reports need your shop id")
		return nil
	}
	rs, err := db.DB().Query(`SELECT COALESCE(listing_id, CAST(id AS INTEGER)) FROM listings WHERE state = 'active' AND CAST(json_extract(data,'$.shop_id') AS TEXT) = ? ORDER BY 1`, shop)
	if err != nil {
		return err
	}
	ids := make([]int64, 0)
	for rs.Next() {
		var id int64
		if rs.Scan(&id) == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	_ = rs.Close()
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	if cliutil.IsDogfoodEnv() && len(ids) > 3 {
		ids = ids[:3]
	}
	if len(ids) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: no synced listings to refresh; run 'etsy-pp-cli sync --resources listings' first")
		return nil
	}
	planned := (len(ids) + 99) / 100
	if err := checkBudget(cmd, planned, ignoreBudget); err != nil {
		return err
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	// read-before-write / snapshot / refresh reads must see live data, never the 5-minute response cache
	c.NoCache = true
	parent := cmd.Context()
	ctx := parent
	if cliutil.IsMCPChildEnv() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(parent, etsyLowStockMCPCeiling)
		defer cancel()
	}
	thr := &bulkThrottle{interval: bulkMinInterval}
	failed, stored := 0, 0
	etsyCalls := 0
	save := func(idStr string, obj map[string]any) error {
		rows := []json.RawMessage{}
		products, _ := obj["products"].([]any)
		for _, value := range products {
			product, ok := value.(map[string]any)
			if !ok || s2Str(product["product_id"]) == "" {
				continue
			}
			product["listing_id"], product["parent_id"] = idStr, idStr
			for _, etsyKey := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
				if etsyValue, etsyOK := obj[etsyKey]; etsyOK {
					product[etsyKey] = etsyValue
				}
			}
			data, err := json.Marshal(product)
			if err != nil {
				return err
			}
			rows = append(rows, data)
		}
		if _, _, _, _, err := db.ReplaceParentPartition("listings_inventory", "", []string{idStr}, rows); err != nil {
			return err
		}

		stored++
		return nil
	}
	get := func(path string, params map[string]string) (json.RawMessage, error) {
		if err := thr.wait(ctx); err != nil {
			return nil, err
		}
		if cliutil.IsMCPChildEnv() && etsyCalls >= etsyLowStockMCPMaxCalls {
			return nil, errEtsyLowStockCallCap
		}
		etsyCalls++
		callCtx, cancel := context.WithTimeout(ctx, etsyLowStockCallTimeout)
		defer cancel()
		return c.Get(callCtx, path, params)
	}
	interrupted := func() error {
		etsyHint := ""
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			if parent.Err() == nil && cliutil.IsMCPChildEnv() {
				etsyHint = fmt.Sprintf("; the MCP run ceiling (%s) was reached — lower --refresh-limit and re-run", etsyLowStockMCPCeiling)
			} else {
				etsyHint = fmt.Sprintf("; a deadline on the calling context was reached — lower --refresh-limit and re-run (each API call also has a %s deadline)", etsyLowStockCallTimeout)
			}
		}
		return fmt.Errorf("inventory refresh interrupted: %w after %d of %d listings%s", ctx.Err(), stored+failed, len(ids), etsyHint)
	}
	var refreshChunk func([]string, *int, bool) error
	refreshChunk = func(chunk []string, etsySplitBudget *int, etsyTopLevel bool) error {
		if ctx.Err() != nil {
			return interrupted()
		}
		if !etsyTopLevel {
			if *etsySplitBudget <= 0 {
				failed += len(chunk)
				return nil
			}
			*etsySplitBudget--
		}
		pending := map[string]bool{}
		for _, id := range chunk {
			pending[id] = true
		}
		raw, err := get("/listings/batch/inventory", map[string]string{"listing_ids": strings.Join(chunk, ",")})
		if ctx.Err() != nil {
			return interrupted()
		}
		if errors.Is(err, errEtsyLowStockCallCap) {
			return fmt.Errorf("%w after %d calls (%d of %d listings processed); re-run with a smaller --refresh-limit", errEtsyLowStockCallCap, etsyCalls, stored+failed, len(ids))
		}
		if err != nil && (bulkAPIStatus(err) == 400 || bulkAPIStatus(err) == 404) {
			if len(chunk) == 1 {
				failed += len(chunk)
				return nil
			}
			if err := checkBudget(cmd, 2*len(chunk)-2, ignoreBudget); err != nil {
				return err
			}
			mid := len(chunk) / 2
			if err := refreshChunk(chunk[:mid], etsySplitBudget, false); err != nil {
				return err
			}
			return refreshChunk(chunk[mid:], etsySplitBudget, false)
		}
		if err != nil {
			if bulkFatal(err) || bulkIsDailyQuota(err) {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			failed += len(chunk)
			return nil
		}
		obj, err := s2DecodeObject(raw)
		if err != nil {
			failed += len(chunk)
			return nil
		}
		results, _ := obj["results"].([]any)
		for _, value := range results {
			result, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id := s2Str(result["listing_id"])
			if !pending[id] {
				continue
			}
			inventory, ok := result["inventory"].(map[string]any)
			if !ok {
				inventory = result
			}
			if _, ok := inventory["products"].([]any); !ok {
				continue
			}
			if err := save(id, inventory); err != nil {
				return err
			}
			delete(pending, id)
		}
		failed += len(pending)
		return nil
	}
	for start := 0; start < len(ids); start += 100 {
		end := start + 100
		if end > len(ids) {
			end = len(ids)
		}
		chunk := []string{}
		for _, id := range ids[start:end] {
			chunk = append(chunk, strconv.FormatInt(id, 10))
		}
		etsySplitBudget := len(chunk) + 8
		if err := refreshChunk(chunk, &etsySplitBudget, true); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return interrupted()
	}

	if failed > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d inventory reads failed; results cover the other %d listings\n", failed, len(ids), stored)
	}
	return nil
}

func s2PrintLowStock(cmd *cobra.Command, flags *rootFlags, rows []s2LowStockRow, notes []string) error {
	if rows == nil {
		rows = []s2LowStockRow{}
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		shopxPrintNotes(cmd, notes)
		return s2PrintRows(cmd, flags, rows)
	}
	return shopxEmit(cmd, flags, rows, notes)
}

func s2PrintRows(cmd *cobra.Command, flags *rootFlags, rows []s2LowStockRow) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no offerings at or below the threshold")
			return nil
		}
		tw := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, "LISTING\tQTY\tSKU\tTITLE")
		for _, r := range rows {
			fmt.Fprintf(tw, "%d\t%d\t%s\t%s\n", r.ListingID, r.Quantity, r.SKU, truncate(r.Title, 48))
		}
		return tw.Flush()
	}
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}

// Sync stamps parent identifiers as strings; legacy inventory uses JSON numbers.
func s2InventoryID(value any) int64 {
	id, _ := strconv.ParseInt(s2Str(value), 10, 64)
	return id
}

type s2InvProduct struct {
	ListingID int64
	Product   map[string]any
}

// s2InventoryProducts normalizes whole inventories and individual product rows.
func s2InventoryProducts(db *store.Store, include ...func(int64) bool) ([]s2InvProduct, error) {
	rs, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type IN ('listings_inventory','listing_inventory')
		UNION ALL SELECT id, data FROM listing_inventory`)
	if err != nil {
		if syncHintMissingTable(err) {
			return []s2InvProduct{}, nil
		}
		return nil, err
	}
	defer rs.Close()
	out := make([]s2InvProduct, 0)
	seen := map[string]bool{}
	for rs.Next() {
		var rowID string
		var data []byte
		if err := rs.Scan(&rowID, &data); err != nil {
			return nil, err
		}
		inv, err := s2DecodeObject(data)
		if err != nil {
			continue
		}
		lid := s2InventoryID(inv["listing_id"])
		if lid == 0 {
			lid = s2InventoryID(inv["parent_id"])
		}
		if lid == 0 {
			lid, _ = strconv.ParseInt(rowID, 10, 64)
		}
		if len(include) > 0 && !include[0](lid) {
			continue
		}
		prods, legacy := inv["products"].([]any)
		if !legacy {
			prods = []any{inv}
		}
		for _, value := range prods {
			prod, ok := value.(map[string]any)
			if !ok {
				continue
			}
			key := strconv.FormatInt(lid, 10) + ":" + s2Str(prod["product_id"])
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, s2InvProduct{ListingID: lid, Product: prod})
		}
	}
	return out, rs.Err()
}

// s2LowStockRows scans synced inventory rows (the JSON payload, since the
// inventory response is nested products[].offerings[]) and joins titles.
func s2LowStockRows(db *store.Store, threshold int64, include ...func(int64) bool) ([]s2LowStockRow, error) {
	titles := map[int64]string{}
	if tr, err := db.DB().Query(`SELECT COALESCE(listing_id, CAST(id AS INTEGER)), COALESCE(title,'') FROM listings`); err == nil {
		for tr.Next() {
			var id int64
			var t string
			if tr.Scan(&id, &t) == nil {
				titles[id] = t
			}
		}
		_ = tr.Close()
	}
	products, err := s2InventoryProducts(db, include...)
	if err != nil {
		return nil, err
	}
	out := make([]s2LowStockRow, 0)
	for _, item := range products {
		lid, prod := item.ListingID, item.Product
		if s2Bool(prod["is_deleted"]) {
			continue
		}
		offs, _ := prod["offerings"].([]any)
		for _, oi := range offs {
			off, ok := oi.(map[string]any)
			if !ok || s2Bool(off["is_deleted"]) {
				continue
			}
			if en, present := off["is_enabled"]; present && !s2Bool(en) {
				continue
			}
			qv, present := off["quantity"]
			if !present || qv == nil {
				continue
			}
			if qn := s2Int(qv); qn > threshold {
				continue
			}
			qn := s2Int(qv)
			sku, _ := prod["sku"].(string)
			out = append(out, s2LowStockRow{
				ListingID: lid, Title: titles[lid], ProductID: s2Str(prod["product_id"]),
				SKU: sku, OfferingID: s2Str(off["offering_id"]), Quantity: qn,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Quantity != out[j].Quantity {
			return out[i].Quantity < out[j].Quantity
		}
		if out[i].ListingID != out[j].ListingID {
			return out[i].ListingID < out[j].ListingID
		}
		return out[i].OfferingID < out[j].OfferingID
	})
	return out, nil
}

// s2StoredInventory returns the synced inventory JSON for one listing ("" if none).
func s2StoredInventory(db *store.Store, listingID int64) string {
	rs, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type IN ('listings_inventory','listing_inventory')
		UNION ALL SELECT id, data FROM listing_inventory`)
	if err != nil {
		return ""
	}
	defer rs.Close()
	for rs.Next() {
		var id string
		var data []byte
		if rs.Scan(&id, &data) != nil {
			continue
		}
		m, err := s2DecodeObject(data)
		if err != nil {
			continue
		}
		lid := s2InventoryID(m["listing_id"])
		if lid == 0 {
			lid = s2InventoryID(m["parent_id"])
		}
		if lid == 0 {
			lid, _ = strconv.ParseInt(id, 10, 64)
		}
		if _, legacy := m["products"].([]any); lid == listingID && legacy {
			return string(data)
		}
	}
	_ = rs.Close()
	products, err := s2InventoryProducts(db)
	if err != nil {
		return ""
	}
	var selected []map[string]any
	for _, product := range products {
		if product.ListingID == listingID {
			selected = append(selected, product.Product)
		}
	}
	if len(selected) == 0 {
		return ""
	}
	inventory := map[string]any{"listing_id": listingID, "products": selected}
	for _, product := range selected {
		for _, key := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
			if existing, ok := inventory[key].([]any); ok && len(existing) > 0 {
				continue
			}
			if value, ok := product[key].([]any); ok && len(value) > 0 {
				inventory[key] = value
			}
		}
	}
	data, err := json.Marshal(inventory)
	if err != nil {
		return ""
	}
	return string(data)
}

// Scope the reporting reader only; snapshots and write read-before-write retain their lookup behavior.
func s2ScopedLowStockRows(db *store.Store, shop string, threshold int64) ([]s2LowStockRow, []string, error) {
	if shop == "" {
		return []s2LowStockRow{}, []string{shopxNoteShopUnset}, nil
	}
	_, allowed, _ := shopxListingsByID(db, shop)
	dropped := map[int64]bool{}
	rows, err := s2LowStockRows(db, threshold, func(id int64) bool {
		if _, ok := allowed[id]; !ok {
			dropped[id] = true
			return false
		}
		return true
	})
	notes := []string{}
	if len(dropped) > 0 {
		notes = append(notes, shopxForeignNote(len(dropped)))
	}
	return rows, notes, err
}
