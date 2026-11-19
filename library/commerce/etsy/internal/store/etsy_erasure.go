// Hand-written Etsy erasure policy; preserved across reprints.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type EtsyEraseOptions struct {
	ShopID                                                                 string
	PreScrubReviews                                                        map[string][]string
	BuyerIDs, Emails, ReceiptIDs, Unsuppress, UnsuppressReceipts, OwnerIDs []string
	ErasedAt                                                               int64
	OwnerRefusalUnavailable                                                bool
}

const etsySuppressionSchema = `CREATE TABLE IF NOT EXISTS etsy_erasure_suppression(buyer_user_id TEXT PRIMARY KEY, erased_at INTEGER)`

func containsKind(kinds []string, kind string) bool { return slices.Contains(kinds, kind) }

func etsyScalar(obj map[string]json.RawMessage, key string) string {
	raw := obj[key]
	var text string
	if json.Unmarshal(raw, &text) == nil {
		if strings.HasSuffix(key, "_id") {
			return etsyCanonicalID(text)
		}
		return text
	}
	if string(raw) == "null" {
		return ""
	}
	return string(raw)
}

// Both projections participate, including typed legacy rows whose JSON lost ids.
func etsyErasureRows(tx *sql.Tx, kind string) ([]struct{ id, data string }, error) {
	q := `SELECT id,data FROM resources WHERE resource_type=?`
	switch kind {
	case "receipts":
		q += ` UNION SELECT id,json_set(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id',COALESCE(receipt_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id')),'$.buyer_user_id',COALESCE(buyer_user_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id')),'$.buyer_email',COALESCE(buyer_email,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_email')),'$.created_timestamp',COALESCE(created_timestamp,create_timestamp,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.created_timestamp'),json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.create_timestamp')),'$.is_shipped',COALESCE(is_shipped,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.is_shipped'))) FROM receipts`
	case "transactions", "payments":
		identity := "payment_id"
		if kind == "transactions" {
			identity = "transaction_id"
		}
		q += ` UNION SELECT id,json_patch(CASE WHEN json_valid(data) THEN data ELSE '{}' END,json_object('receipt_id',COALESCE(receipt_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id')),'buyer_user_id',COALESCE(buyer_user_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id')),'` + identity + `',COALESCE("` + identity + `",json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.` + identity + `')))) FROM "` + kind + `"`
	case "users", "economic_operators":
		q += ` UNION SELECT id,data FROM "` + kind + `"`
	case "reviews":
		q += ` UNION SELECT id,json_patch(CASE WHEN json_valid(data) THEN data ELSE '{}' END,json_object('transaction_id',COALESCE(transaction_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.transaction_id')),'buyer_user_id',COALESCE(buyer_user_id,json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id')))) FROM reviews`
	}
	params := []any{kind}
	rows, err := tx.Query(q, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ id, data string }
	for rows.Next() {
		var r struct{ id, data string }
		if err = rows.Scan(&r.id, &r.data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func etsyCanonicalID(id string) string {
	n, err := strconv.ParseInt(id, 10, 64)
	if err == nil {
		return strconv.FormatInt(n, 10)
	}
	return id
}

func etsySelectErasure(tx *sql.Tx, o *EtsyEraseOptions) error {
	for _, q := range []string{`CREATE TEMP TABLE etsy_erase_buyers(id TEXT PRIMARY KEY)`} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	buyers := map[string]bool{}
	receipts := map[string]bool{}
	for _, id := range o.BuyerIDs {
		buyers[etsyCanonicalID(id)] = true
	}
	for _, id := range o.ReceiptIDs {
		receipts[etsyCanonicalID(id)] = true
	}
	// Resolve email/receipt selectors to buyers before choosing the purge units.
	all := map[string][]struct{ id, data string }{}
	for _, kind := range etsyBuyerResourceTypes {
		if kind == "user_addresses" {
			continue
		}
		rows, err := etsyErasureRows(tx, kind)
		if err != nil {
			return err
		}
		all[kind] = rows
		for _, r := range rows {
			var obj map[string]json.RawMessage
			if json.Unmarshal([]byte(r.data), &obj) != nil {
				continue
			}
			buyer := etsyScalar(obj, "buyer_user_id")
			if kind == "users" && buyer == "" {
				buyer = etsyScalar(obj, "user_id")
			}
			email := strings.ToLower(strings.TrimSpace(etsyScalar(obj, "buyer_email")))
			if email == "" {
				email = strings.ToLower(strings.TrimSpace(etsyScalar(obj, "email")))
			}
			if email == "" {
				email = strings.ToLower(strings.TrimSpace(etsyScalar(obj, "primary_email")))
			}
			for _, wanted := range o.Emails {
				if email == strings.ToLower(strings.TrimSpace(wanted)) && kind == "receipts" {
					id := etsyScalar(obj, "receipt_id")
					if id == "" {
						id = BareResourceID(r.id)
					}
					if EtsyValidateErasureID(id) == nil {
						receipts[id] = true
					}
				}
				if email == strings.ToLower(strings.TrimSpace(wanted)) && buyer != "" {
					if EtsyValidateErasureID(buyer) == nil {
						buyers[buyer] = true
					}
				}
			}
			if kind == "receipts" && (receipts[etsyScalar(obj, "receipt_id")] || receipts[BareResourceID(r.id)]) && buyer != "" {
				if EtsyValidateErasureID(buyer) == nil {
					buyers[buyer] = true
				}
			}
		}
	}
	owners := []string{}
	for _, id := range o.OwnerIDs {
		owners = append(owners, etsyCanonicalID(id))
	}
	rows, err := tx.Query(`SELECT CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.user_id') AS TEXT) FROM resources WHERE resource_type='users' AND json_valid(data) AND (id='me' OR json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.is_current_user')=1) UNION SELECT CAST(seller_user_id AS TEXT) FROM receipts WHERE seller_user_id IS NOT NULL UNION SELECT CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.seller_user_id') AS TEXT) FROM resources WHERE resource_type='receipts' AND json_valid(data) AND json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.seller_user_id') IS NOT NULL`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id sql.NullString
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if id.Valid {
			owners = append(owners, etsyCanonicalID(id.String))
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if o.ShopID != "" {
		rows, err = tx.Query(`SELECT CAST(user_id AS TEXT) FROM shops WHERE CAST(shop_id AS TEXT)=? UNION SELECT CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.user_id') AS TEXT) FROM resources WHERE resource_type='shops' AND json_valid(data) AND CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.shop_id') AS TEXT)=?`, o.ShopID, o.ShopID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var owner sql.NullString
			if err = rows.Scan(&owner); err != nil {
				rows.Close()
				return err
			}
			if owner.Valid {
				owners = append(owners, etsyCanonicalID(owner.String))
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	o.OwnerRefusalUnavailable = o.OwnerRefusalUnavailable || len(owners) == 0
	for id := range buyers {
		if err = EtsyValidateErasureID(id); err != nil {
			return err
		}
		if slices.Contains(owners, id) {
			return fmt.Errorf("refusing to erase the token owner's user id")
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_erase_buyers VALUES(?)`, id); err != nil {
			return err
		}
	}
	for _, id := range o.Unsuppress {
		if err = EtsyValidateErasureID(id); err != nil {
			return err
		}
	}
	for _, r := range all["receipts"] {
		var obj map[string]json.RawMessage
		_ = json.Unmarshal([]byte(r.data), &obj)
		if buyers[etsyScalar(obj, "buyer_user_id")] {
			id := etsyScalar(obj, "receipt_id")
			if id == "" {
				id = BareResourceID(r.id)
			}
			receipts[id] = true
		}
	}
	for id := range receipts {
		if err = EtsyValidateErasureID(id); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_receipts VALUES(?)`, id); err != nil {
			return err
		}
	}
	for _, kind := range []string{"transactions", "listings_transactions"} {
		for _, r := range all[kind] {
			var obj map[string]json.RawMessage
			_ = json.Unmarshal([]byte(r.data), &obj)
			if buyers[etsyScalar(obj, "buyer_user_id")] || receipts[etsyScalar(obj, "receipt_id")] {
				id := etsyScalar(obj, "transaction_id")
				if id == "" {
					id = BareResourceID(r.id)
				}
				if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_tx VALUES(?)`, id); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func EtsyValidateErasureID(id string) error {
	n, e := strconv.ParseInt(id, 10, 64)
	if e != nil || n <= 0 {
		return fmt.Errorf("erasure ids must be positive integers")
	}
	return nil
}

func etsyErasureHit(tx *sql.Tx, kind, id, key, data string) (bool, error) {
	var obj map[string]json.RawMessage
	_ = json.Unmarshal([]byte(data), &obj)
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM etsy_erase_buyers WHERE id=?`, func() string {
		if kind == "users" {
			return etsyScalar(obj, "user_id")
		}
		return etsyScalar(obj, "buyer_user_id")
	}()).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	receipt := etsyScalar(obj, "receipt_id")
	if receipt == "" {
		receipt = etsyScalar(obj, "receipts_id")
	}
	if receipt == "" && kind == "receipts" {
		receipt = BareResourceID(id)
	}
	if receipt == "" && (kind == "receipts_listings" || kind == "shops_receipts_payments") {
		receipt = etsyScalar(obj, "parent_id")
		if receipt == "" {
			receipt = resourceParentID(id)
		}
	}
	if err := tx.QueryRow(`SELECT count(*) FROM etsy_prune_receipts WHERE id=?`, receipt).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if kind == "reviews" || kind == "listings_reviews" || kind == "transactions" || kind == "listings_transactions" {
		if err := tx.QueryRow(`SELECT count(*) FROM etsy_prune_tx WHERE id=?`, key).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	// Legacy typed columns may be richer than their blob.
	if kind == "receipts" || kind == "transactions" || kind == "payments" || kind == "reviews" {
		q := `SELECT count(*) FROM "` + kind + `" WHERE id=? AND CAST(buyer_user_id AS TEXT) IN (SELECT id FROM etsy_erase_buyers)`
		if kind != "reviews" {
			q += ` OR (id=? AND CAST(receipt_id AS TEXT) IN (SELECT id FROM etsy_prune_receipts))`
			if err := tx.QueryRow(q, id, id).Scan(&n); err != nil {
				return false, err
			}
		} else {
			if err := tx.QueryRow(q, id).Scan(&n); err != nil {
				return false, err
			}
		}
	}
	return n > 0, nil
}

func etsyPersistErasure(tx *sql.Tx, o *EtsyEraseOptions) error {
	if _, err := tx.Exec(etsySuppressionSchema); err != nil {
		return err
	}
	at := o.ErasedAt
	if at == 0 {
		at = time.Now().Unix()
	}
	if _, err := tx.Exec(`INSERT INTO etsy_erasure_suppression SELECT id,? FROM etsy_erase_buyers WHERE true ON CONFLICT(buyer_user_id) DO UPDATE SET erased_at=MAX(erased_at,excluded.erased_at)`, at); err != nil {
		return err
	}
	// etsy-ext: retain a keyed owner hint for receipt-only unsuppression diagnostics.
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS etsy_erasure_buyer_receipts(receipt_id TEXT PRIMARY KEY, buyer_user_id TEXT NOT NULL)`); err != nil {
		return err
	}
	rows, err := etsyErasureRows(tx, "receipts")
	if err != nil {
		return err
	}
	for _, r := range rows {
		var m map[string]json.RawMessage
		_ = json.Unmarshal([]byte(r.data), &m)
		receipt, buyer := etsyScalar(m, "receipt_id"), etsyScalar(m, "buyer_user_id")
		if receipt == "" {
			receipt = BareResourceID(r.id)
		}
		if EtsyValidateErasureID(buyer) != nil {
			continue
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_erasure_buyer_receipts SELECT ?,? WHERE EXISTS(SELECT 1 FROM etsy_prune_receipts WHERE id=?) AND EXISTS(SELECT 1 FROM etsy_erase_buyers WHERE id=?)`, receipt, buyer, receipt, buyer); err != nil {
			return err
		}
	}
	// etsy-ext: receipt suppression survives email-only erasure and missing buyer ids.
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS etsy_erasure_receipts(id TEXT PRIMARY KEY, erased_at INTEGER)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO etsy_erasure_receipts SELECT p.id,? FROM etsy_prune_receipts p WHERE NOT EXISTS (SELECT 1 FROM receipts r WHERE CAST(r.receipt_id AS TEXT)=p.id AND r.buyer_user_id>0) AND NOT EXISTS (SELECT 1 FROM resources r WHERE r.resource_type='receipts' AND CASE WHEN json_valid(r.data) THEN CAST(json_extract(r.data,'$.receipt_id') AS TEXT) END=p.id AND CASE WHEN json_valid(r.data) THEN CAST(json_extract(r.data,'$.buyer_user_id') AS INTEGER) END>0) ON CONFLICT(id) DO UPDATE SET erased_at=MAX(erased_at,excluded.erased_at)`, at); err != nil {
		return err
	}
	// etsy-ext: remember guest transaction links before erasure removes their rows.
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS etsy_erasure_transactions(transaction_id TEXT PRIMARY KEY, receipt_id TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS etsy_erasure_transactions_receipt ON etsy_erasure_transactions(receipt_id)`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	for _, kind := range []string{"receipts", "transactions", "listings_transactions"} {
		rows, err := etsyErasureRows(tx, kind)
		if err != nil {
			return err
		}
		for _, r := range rows {
			var m map[string]json.RawMessage
			_ = json.Unmarshal([]byte(r.data), &m)
			receipt := etsyScalar(m, "receipt_id")
			if receipt == "" {
				receipt = etsyScalar(m, "receipts_id")
			}
			if receipt == "" {
				receipt = etsyScalar(m, "parent_id")
			}
			if kind == "receipts" && receipt == "" {
				receipt = BareResourceID(r.id)
			}
			transaction := etsyScalar(m, "transaction_id")
			if transaction == "" && kind != "receipts" {
				transaction = BareResourceID(r.id)
			}
			ids := []string{transaction}
			if kind == "receipts" {
				var embedded []map[string]json.RawMessage
				_ = json.Unmarshal(m["transactions"], &embedded)
				for _, item := range embedded {
					ids = append(ids, etsyScalar(item, "transaction_id"))
				}
			}
			for _, id := range ids {
				if id == "" {
					continue
				}
				if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_erasure_transactions SELECT ?,? WHERE EXISTS(SELECT 1 FROM etsy_prune_tx WHERE id=?) AND EXISTS(SELECT 1 FROM etsy_erasure_receipts WHERE id=?)`, id, receipt, id, receipt); err != nil {
					return err
				}
			}
		}
	}
	for _, id := range o.UnsuppressReceipts {
		if _, err := tx.Exec(`DELETE FROM etsy_erasure_transactions WHERE receipt_id=?`, etsyCanonicalID(id)); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM etsy_erasure_receipts WHERE id=?`, etsyCanonicalID(id)); err != nil {
			return err
		}
	}
	for _, id := range o.Unsuppress {
		if _, err := tx.Exec(`DELETE FROM etsy_erasure_suppression WHERE buyer_user_id=?`, etsyCanonicalID(id)); err != nil {
			return err
		}
	}
	return nil
}

func etsySuppressionDrop(tx *sql.Tx, kind string, data json.RawMessage) (bool, error) {
	if kind == "user_addresses" || !slices.Contains(etsyBuyerResourceTypes, kind) {
		return false, nil
	}
	// etsy-ext: parent receipt suppression also covers dependent imports.
	var m map[string]json.RawMessage
	_ = json.Unmarshal(data, &m)
	receipt := etsyScalar(m, "receipt_id")
	if receipt == "" {
		receipt = etsyScalar(m, "receipts_id")
	}
	if receipt == "" {
		receipt = etsyScalar(m, "parent_id")
	}
	if exists, e := etsyTableExists(tx, "etsy_erasure_receipts"); e != nil {
		return false, e
	} else if exists {
		var n int
		if e = tx.QueryRow(`SELECT count(*) FROM etsy_erasure_receipts WHERE id=?`, receipt).Scan(&n); e != nil {
			return false, e
		}
		if n > 0 {
			return true, nil
		}
	}
	// etsy-ext: guest reviews carry a transaction key without a receipt key.
	if kind == "reviews" || kind == "listings_reviews" || kind == "transactions" || kind == "listings_transactions" {
		if exists, err := etsyTableExists(tx, "etsy_erasure_transactions"); err != nil {
			return false, err
		} else if exists {
			var n int
			if err = tx.QueryRow(`SELECT count(*) FROM etsy_erasure_transactions WHERE transaction_id=?`, etsyScalar(m, "transaction_id")).Scan(&n); err != nil {
				return false, err
			}
			if n > 0 {
				return true, nil
			}
		}
	}
	exists, err := etsyTableExists(tx, "etsy_erasure_suppression")
	if err != nil || !exists {
		return false, err
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return false, nil
	}
	buyer := etsyScalar(obj, "buyer_user_id")
	if kind == "users" && buyer == "" {
		buyer = etsyScalar(obj, "user_id")
	}
	timestamp := int64(-1) // Missing age cannot prove a record postdates erasure.
	for _, key := range []string{"created_timestamp", "create_timestamp"} {
		if parsed, e := strconv.ParseInt(etsyScalar(obj, key), 10, 64); e == nil {
			timestamp = parsed
			break
		}
	}
	var n int
	err = tx.QueryRow(`SELECT count(*) FROM etsy_erasure_suppression WHERE buyer_user_id=? AND erased_at>=?`, buyer, timestamp).Scan(&n)
	return n > 0, err
}

func etsyDeleteResourceLearnings(tx *sql.Tx, ids map[string]bool) error {
	if len(ids) == 0 {
		return nil
	}
	for _, table := range []string{"search_learnings", "learn_recall_misses"} {
		exists, err := etsyTableExists(tx, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if table == "learn_recall_misses" {
			if _, err = tx.Exec(`DELETE FROM learn_recall_misses`); err != nil {
				return err
			}
			continue
		}
		rows, err := tx.Query(`SELECT id,COALESCE(resource_type,''),resource_id FROM search_learnings`)
		if err != nil {
			return err
		}
		var victims []int64
		for rows.Next() {
			var id int64
			var kind, resource string
			if err = rows.Scan(&id, &kind, &resource); err != nil {
				rows.Close()
				return err
			}
			var n int
			if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM etsy_prune_learning_ids WHERE kind=? AND id=? UNION ALL SELECT 1 FROM etsy_prune_victims WHERE kind=? AND id=?)", kind, BareResourceID(resource), kind, resource).Scan(&n); err != nil {
				rows.Close()
				return err
			}
			if slices.Contains(etsyBuyerResourceTypes, kind) && n > 0 {
				victims = append(victims, id)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range victims {
			if _, err = tx.Exec(`DELETE FROM search_learnings WHERE id=?`, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func etsyVacuum(ctx context.Context, conn *sql.Conn, steps map[string]string) (status string, err error) {
	// etsy-ext: hard errors outrank every incomplete reclamation status.
	defer func() {
		if err != nil {
			status = "error"
		}
	}()
	var priorTimeout int
	if err = conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&priorTimeout); err != nil {
		return "", err
	}
	defer conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA busy_timeout=%d", priorTimeout))
	rows, err := conn.QueryContext(ctx, `SELECT name,sql FROM sqlite_master WHERE type='table' AND sql LIKE 'CREATE VIRTUAL TABLE%'`)
	if err != nil {
		return "", err
	}
	var rebuild []string
	present := map[string]bool{}
	for rows.Next() {
		var name, ddl string
		if err = rows.Scan(&name, &ddl); err != nil {
			rows.Close()
			return "", err
		}
		present[name] = true
		if strings.Contains(strings.ToLower(ddl), "content=") || strings.Contains(strings.ToLower(ddl), "content =") {
			rebuild = append(rebuild, name)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	// etsy-ext: wait for FTS writers using the caller timeout; busy is incomplete, not failure.
	busyStep := false
	steps["optimize"] = "not_applicable"
	isBusy := func(e error) bool {
		text := strings.ToLower(e.Error())
		return strings.Contains(text, "busy") || strings.Contains(text, "locked")
	}
	for _, name := range []string{"resources_fts", "transactions_fts"} {
		if present[name] {
			if _, err = conn.ExecContext(ctx, `INSERT INTO `+quote(name)+`(`+quote(name)+`) VALUES('optimize')`); err != nil {
				if isBusy(err) {
					steps["optimize"] = "skipped_busy"
					busyStep = true
					continue
				}
				return "error", err
			}
			if steps["optimize"] != "skipped_busy" {
				steps["optimize"] = "completed"
			}
		}
	}
	for _, name := range rebuild {
		if _, err = conn.ExecContext(ctx, `INSERT INTO `+quote(name)+`(`+quote(name)+`) VALUES('rebuild')`); err != nil {
			if isBusy(err) {
				steps["rebuild"] = "skipped_busy"
				busyStep = true
				continue
			}
			return "error", err
		}
	}
	if steps["rebuild"] == "" {
		steps["rebuild"] = "completed"
		if len(rebuild) == 0 {
			steps["rebuild"] = "not_applicable"
		}
	}
	if _, err = conn.ExecContext(ctx, `PRAGMA busy_timeout=0`); err != nil {
		return "error", err
	}
	if _, err = conn.ExecContext(ctx, `VACUUM`); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "busy") || strings.Contains(strings.ToLower(err.Error()), "locked") {
			steps["vacuum"] = "skipped_busy"
			return "skipped_busy", nil
		}
		return "", err
	}
	steps["vacuum"] = "completed"
	var busy, log, checkpointed int
	if err = conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &checkpointed); err != nil {
		return "", err
	}
	if busy != 0 {
		steps["checkpoint"] = "checkpoint_busy"
		return "checkpoint_busy", nil
	}
	steps["checkpoint"] = "completed"
	if busyStep {
		return "optimize_busy", nil
	}
	return "completed", nil
}

// Embedded receipt transactions are links even when no standalone transaction
// was ever cached. Share this TEMP set with both age and buyer selection.
func etsyRememberSelectedTransactions(tx *sql.Tx, kind, key, data string) error {
	if kind == "transactions" || kind == "listings_transactions" {
		_, err := tx.Exec(`INSERT OR IGNORE INTO etsy_prune_tx VALUES(?)`, key)
		return err
	}
	if kind != "receipts" {
		return nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(data), &obj) != nil {
		return nil
	}
	var transactions []map[string]json.RawMessage
	if json.Unmarshal(obj["transactions"], &transactions) != nil {
		return nil
	}
	for _, transaction := range transactions {
		if id := etsyScalar(transaction, "transaction_id"); id != "" {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO etsy_prune_tx VALUES(?)`, id); err != nil {
				return err
			}
		}
	}
	return nil
}
