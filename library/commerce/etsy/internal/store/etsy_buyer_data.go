// Hand-written Etsy buyer-data policy; preserved across reprints.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// etsy-ext: keep the canonical slice private; callers receive a defensive copy.
// etsyBuyerResourceTypes is the canonical sorted list of known buyer-bearing
// types, including resources-only dependents. Unknown types remain restricted
// by MCP's separate fail-closed non-buyer allowlist. Treat this list as read-only.
var etsyBuyerResourceTypes = []string{
	"economic_operators", "etsy_payment_checks", "listings_reviews",
	"listings_transactions", "payments", "receipts", "receipts_listings",
	"reviews", "shops_receipts_payments", "transactions", "user_addresses", "users",
}

// etsyStripReviewBuyerFields runs after merging so richer legacy JSON cannot
// restore these fields. Decode raw values to preserve numeric precision.
func etsyStripReviewBuyerFields(resourceType string, item json.RawMessage) json.RawMessage {
	if !slices.Contains(etsyBuyerResourceTypes, resourceType) || (resourceType != "reviews" && resourceType != "listings_reviews") {
		return item
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(item, &obj) != nil || obj == nil {
		return item
	}
	delete(obj, "buyer_user_id")
	delete(obj, "image_url_fullxfull")
	stripped, err := json.Marshal(obj)
	if err != nil {
		return item
	}
	return stripped
}

func EtsyBuyerTypes() []string { return slices.Clone(etsyBuyerResourceTypes) }

const etsyReviewScrubMarker = "review_buyer_fields_scrub_v1"

// etsyScrubReviewBuyerFields is deliberately outside schema user_version. A
// pinned connection keeps secure_delete and BEGIN IMMEDIATE on the same handle.
// The marker and both projections commit together; failures remain retryable.
func (s *Store) etsyScrubReviewBuyerFields(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var exists, done int
	if err = conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name='etsy_meta' AND type='table'`).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		if err = conn.QueryRowContext(ctx, `SELECT count(*) FROM etsy_meta WHERE key=?`, etsyReviewScrubMarker).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			return nil
		}
	}
	var prior int
	if err = conn.QueryRowContext(ctx, `PRAGMA secure_delete`).Scan(&prior); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA secure_delete=%d", prior))
	if _, err = conn.ExecContext(ctx, `PRAGMA secure_delete=ON`); err != nil {
		return err
	}
	if err = execWithBusyRetry(ctx, conn, "BEGIN IMMEDIATE", "begin review scrub", time.Now().Add(5*time.Second)); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `ROLLBACK`)
	if _, err = conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS etsy_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	done = 0
	if err = conn.QueryRowContext(ctx, `SELECT count(*) FROM etsy_meta WHERE key=?`, etsyReviewScrubMarker).Scan(&done); err != nil {
		return err
	}
	if done == 0 {
		// CASE guards malformed legacy JSON independently of predicate order; JSON null
		// still has a json_type and must be removed too. No id filter excludes
		// parameter-fingerprint rows.
		if _, err = conn.ExecContext(ctx, `UPDATE resources SET data=json_remove(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id','$.image_url_fullxfull')
			WHERE resource_type IN ('reviews','listings_reviews') AND json_valid(data)
			AND (json_type(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id') IS NOT NULL OR json_type(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.image_url_fullxfull') IS NOT NULL)`); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, `UPDATE reviews SET data=json_remove(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id','$.image_url_fullxfull'), buyer_user_id=NULL, image_url_fullxfull=NULL
			WHERE json_valid(data) AND (json_type(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id') IS NOT NULL OR json_type(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.image_url_fullxfull') IS NOT NULL)`); err != nil {
			return err
		}
		// Columns may survive even when the old blob omitted the fields (or
		// is malformed). Clear them without invoking JSON functions.
		if _, err = conn.ExecContext(ctx, `UPDATE reviews SET buyer_user_id=NULL, image_url_fullxfull=NULL WHERE buyer_user_id IS NOT NULL OR image_url_fullxfull IS NOT NULL`); err != nil {
			return err
		}
		if _, err = conn.ExecContext(ctx, `INSERT INTO etsy_meta(key,value) VALUES(?,'done')`, etsyReviewScrubMarker); err != nil {
			return err
		}
	}
	err = execWithBusyRetry(ctx, conn, "COMMIT", "commit review scrub", time.Now().Add(5*time.Second))
	return err
}
