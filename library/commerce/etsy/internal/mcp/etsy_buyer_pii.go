package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"os"
	"regexp"
	"strings"
)

const BuyerPIIEnvVar = "ETSY_MCP_ALLOW_BUYER_PII"

// etsyNonBuyerResourceTypes is the allowlist. Anything NOT listed is treated as buyer-bearing (fail closed).
var etsyNonBuyerResourceTypes = []string{"app", "buyer_taxonomy", "holiday_preferences", "ledger_entries", "listing_files", "listing_images", "listing_inventory", "listing_translations", "listing_variation_images", "listing_videos", "listings", "listings-active", "listings-batch", "listings-batch-shipping", "listings-featured", "listings_active", "listings_batch", "listings_batch_shipping", "listings_featured", "listings_inventory", "marketplace_listings", "processing_profiles", "production_partners", "return_policies", "seller_taxonomy", "shipping_profiles", "shipping_profiles-shipping-carriers", "shipping_profiles_destinations", "shipping_profiles_shipping_carriers", "shop_sections", "shops"}

// etsyBuyerResourceTypes shares the canonical storage policy list.
var etsyBuyerResourceTypes = store.EtsyBuyerTypes()

func buyerPIIAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(BuyerPIIEnvVar))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// Conservative denial supplements the connection-local resources view: direct
// typed/FTS reads, schema qualification, and schema introspection are refused.
// This intentionally also refuses those words inside SQL literals/comments.
// etsy_payment_checks (lazy ledger of receipt ids) links to buyer receipts, so it is buyer-side.
// payments is the typed alias for shops_receipts_payments and is denied too.
// mode=ro permits TEMP views; no persistent database state is changed. Fail
// closed if installing the view fails, rather than executing an unguarded query.
var buyerSQLBypass = etsyBuyerSQLBypass()

func etsyBuyerSQLBypass() *regexp.Regexp {
	terms := []string{`\betsy_(erasure|retention)_\w*`, `resources_fts`, `sqlite_`, `\bsearch_learnings\b`, `\blearn_candidates\b`, `\blearning_playbooks\b`, `\bsearch_patterns\b`, `\blearn_recall_misses\b`, `\bmain\b`, `\btemp\b`}
	for _, typ := range store.EtsyBuyerTypes() {
		// Preserve suffix-table denial for the legacy substring guards, while
		// reviews must not also refuse the non-buyer market_reviews table.
		term := regexp.QuoteMeta(typ)
		switch typ {
		case "receipts", "transactions", "user_addresses", "shops_receipts_payments", "payments":
		default:
			term = `\b` + term + `\b`
		}
		terms = append(terms, term)
	}
	return regexp.MustCompile(`(?i)(` + strings.Join(terms, "|") + `)`)
}

func guardBuyerSQL(ctx context.Context, conn *sql.Conn, query string) error {
	if buyerPIIAllowed() {
		return nil
	}
	if buyerSQLBypass.MatchString(query) {
		return fmt.Errorf("buyer PII SQL access is restricted; explicit opt-in requires %s=1", BuyerPIIEnvVar)
	}
	literals := etsyAllowlistLiterals()
	_, err := conn.ExecContext(ctx, `CREATE TEMP VIEW resources AS SELECT * FROM main.resources WHERE resource_type IN (`+literals+`)`) // #nosec G202 -- literals come from the compile-time etsyNonBuyerResourceTypes allowlist, quote-escaped; CREATE VIEW cannot bind args.
	if err != nil {
		return fmt.Errorf("installing buyer PII guard (opt-in %s): %w", BuyerPIIEnvVar, err)
	}
	_, err = conn.ExecContext(ctx, `CREATE TEMP VIEW entity_lookups AS SELECT * FROM main.entity_lookups WHERE kind IN (`+literals+`)`) // #nosec G202 -- literals come from the compile-time etsyNonBuyerResourceTypes allowlist, quote-escaped; CREATE VIEW cannot bind args.
	if err != nil {
		return fmt.Errorf("installing buyer PII guard (opt-in %s): %w", BuyerPIIEnvVar, err)
	}
	return nil
}

func etsyAllowlistLiterals() string {
	literals := make([]string, len(etsyNonBuyerResourceTypes))
	for i, typ := range etsyNonBuyerResourceTypes {
		literals[i] = "'" + strings.ReplaceAll(typ, "'", "''") + "'"
	}
	return strings.Join(literals, ",")
}

// etsyScopedSearch applies the allowlist before the search LIMIT on one connection.
func etsyScopedSearch(ctx context.Context, db *store.Store, query string, limit int) ([]json.RawMessage, error) {
	if buyerPIIAllowed() {
		return db.SearchContext(ctx, query, limit)
	}
	if db.Path() == "" {
		return nil, fmt.Errorf("opening buyer PII search store: empty database path")
	}
	priv, err := store.OpenReadOnlyContext(ctx, db.Path())
	if err != nil {
		return nil, fmt.Errorf("opening buyer PII search store: %w", err)
	}
	defer priv.Close()
	sqlDB := priv.DB()
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if _, err := sqlDB.ExecContext(ctx, `CREATE TEMP VIEW resources AS SELECT * FROM main.resources WHERE resource_type IN (`+etsyAllowlistLiterals()+`)`); err != nil { // #nosec G202 -- literals come from the compile-time etsyNonBuyerResourceTypes allowlist, quote-escaped; CREATE VIEW cannot bind args.
		return nil, fmt.Errorf("installing buyer PII search guard: %w", err)
	}
	return priv.SearchContext(ctx, query, limit)
}

func redactBuyerSearch(results []json.RawMessage) ([]json.RawMessage, bool) {
	if buyerPIIAllowed() {
		return results, false
	}
	empty := make([]json.RawMessage, 0)
	if len(results) == 0 {
		return empty, false
	}
	path, err := mcpDBPath()
	if err != nil {
		return empty, true
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return empty, true
	}
	defer db.Close()
	allowed := make(map[string]bool, len(etsyNonBuyerResourceTypes))
	for _, typ := range etsyNonBuyerResourceTypes {
		allowed[typ] = true
	}
	matched := make(map[string]bool)
	denied := make(map[string]bool)
	for start := 0; start < len(results); start += 50 {
		end := start + 50
		if end > len(results) {
			end = len(results)
		}
		args := make([]any, end-start)
		marks := make([]string, end-start)
		for i, raw := range results[start:end] {
			args[i] = string(raw)
			marks[i] = "?"
		}
		rows, err := db.DB().Query("SELECT data, resource_type FROM resources WHERE data IN ("+strings.Join(marks, ",")+")", args...)
		if err != nil {
			return empty, true
		}
		for rows.Next() {
			var data, typ string
			if err := rows.Scan(&data, &typ); err != nil {
				_ = rows.Close()
				return empty, true
			}
			matched[data] = true
			if !allowed[typ] {
				denied[data] = true
			}
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil || closeErr != nil {
			return empty, true
		}
	}
	out := make([]json.RawMessage, 0, len(results))
	for _, raw := range results {
		key := string(raw)
		if matched[key] && !denied[key] {
			out = append(out, raw)
		}
	}
	return out, len(out) != len(results)
}
