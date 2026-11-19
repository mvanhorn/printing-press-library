// Hand-written Etsy retention policy; preserved across reprints.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
)

type EtsyPruneOptions struct {
	Context                              context.Context
	Cutoff                               int64
	Vacuum                               bool
	ExplicitVacuum                       bool
	Erase                                *EtsyEraseOptions
	CleanupLearn                         func(*sql.Tx, map[string]bool) error
	Receipts, Reviews, ClearFloor, Apply bool
}
type EtsyPruneResult struct {
	ReviewsUnlinkableAfterPrune int                 `json:"reviews_unlinkable_after_prune"`
	ReadmittedReceipts          int                 `json:"readmitted_receipts"`
	SuppressedBuyerReceipt      bool                `json:"suppressed_buyer_receipt"`
	KeptUnknownShipped          int                 `json:"kept_unknown_shipped"`
	UnsuppressedReceipts        int                 `json:"unsuppressed_receipts"`
	Committed                   bool                `json:"-"`
	VacuumSteps                 map[string]string   `json:"vacuum_steps,omitempty"`
	KeptUnlinkableReviews       int                 `json:"kept_unlinkable_reviews"`
	Vacuum                      string              `json:"vacuum,omitempty"`
	ReviewIDs                   map[string][]string `json:"-"`
	BuyerIDs                    []string            `json:"-"`
	ErasedIDs                   map[string]bool     `json:"-"`
	Counts                      map[string]int      `json:"counts"`
	KeptNoTimestamp             int                 `json:"kept_no_timestamp"`
	KeptUnshipped               int                 `json:"kept_unshipped"`
	Floor                       map[string]int64    `json:"floor"`
}

const etsyRetentionSchema = `CREATE TABLE IF NOT EXISTS etsy_retention_floor(scope TEXT PRIMARY KEY, cutoff INTEGER)`

func etsyTableExists(tx *sql.Tx, name string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n > 0, err
}

// EtsyPrune selects once into TEMP tables. Preview and apply read exactly those
// tables, and all projections, FTS and retention metadata commit together.
func (s *Store) EtsyPrune(o EtsyPruneOptions) (result EtsyPruneResult, err error) {
	result = EtsyPruneResult{Counts: map[string]int{}, Floor: map[string]int64{}, ReviewIDs: map[string][]string{}}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	ctx := o.Context
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	if o.Apply {
		var priorSecure int
		if err = conn.QueryRowContext(ctx, `PRAGMA secure_delete`).Scan(&priorSecure); err != nil {
			return result, err
		}
		defer conn.ExecContext(context.Background(), "PRAGMA secure_delete="+strconv.Itoa(priorSecure))
		if _, err = conn.ExecContext(ctx, `PRAGMA secure_delete=ON`); err != nil {
			return result, err
		}
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	for _, q := range []string{`CREATE TEMP TABLE etsy_prune_receipts(id TEXT PRIMARY KEY)`, `CREATE TEMP TABLE etsy_prune_victims(kind TEXT,id TEXT,PRIMARY KEY(kind,id))`, `CREATE TEMP TABLE etsy_prune_learning_ids(kind TEXT,id TEXT,PRIMARY KEY(kind,id))`, `CREATE TEMP TABLE etsy_prune_tx(id TEXT PRIMARY KEY)`} {
		if _, err = tx.Exec(q); err != nil {
			return result, err
		}
	}
	// etsy-ext: snapshot stored parent identities once; anti-join finds orphans.
	for _, q := range []string{
		"CREATE TEMP TABLE etsy_stored_receipts(id TEXT PRIMARY KEY)",
		"INSERT OR IGNORE INTO etsy_stored_receipts SELECT COALESCE(CAST(receipt_id AS TEXT),CASE WHEN instr(id,char(0))>0 THEN substr(id,1,instr(id,char(0))-1) ELSE id END) FROM receipts",
		"INSERT OR IGNORE INTO etsy_stored_receipts SELECT COALESCE(CASE WHEN json_valid(data) THEN CAST(json_extract(data,'$.receipt_id') AS TEXT) END,CASE WHEN instr(id,char(0))>0 THEN substr(id,1,instr(id,char(0))-1) ELSE id END) FROM resources WHERE resource_type='receipts'",
		"CREATE TEMP TABLE etsy_orphan_parents(id TEXT PRIMARY KEY)",
		"INSERT OR IGNORE INTO etsy_orphan_parents SELECT DISTINCT CAST(COALESCE(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id'),json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipts_id'),json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.parent_id')) AS TEXT) FROM resources r WHERE resource_type IN ('transactions','listings_transactions','payments','shops_receipts_payments') AND NOT EXISTS(SELECT 1 FROM etsy_stored_receipts p WHERE p.id=CAST(COALESCE(json_extract(CASE WHEN json_valid(r.data) THEN r.data ELSE '{}' END,'$.receipt_id'),json_extract(CASE WHEN json_valid(r.data) THEN r.data ELSE '{}' END,'$.receipts_id'),json_extract(CASE WHEN json_valid(r.data) THEN r.data ELSE '{}' END,'$.parent_id')) AS TEXT))",
		"INSERT OR IGNORE INTO etsy_orphan_parents SELECT CAST(r.receipt_id AS TEXT) FROM transactions r LEFT JOIN etsy_stored_receipts p ON p.id=CAST(r.receipt_id AS TEXT) WHERE p.id IS NULL",
		"INSERT OR IGNORE INTO etsy_orphan_parents SELECT CAST(r.receipt_id AS TEXT) FROM payments r LEFT JOIN etsy_stored_receipts p ON p.id=CAST(r.receipt_id AS TEXT) WHERE p.id IS NULL",
	} {
		if _, err = tx.Exec(q); err != nil {
			return result, err
		}
	}
	// etsy-ext: materialize anti-join results once, without per-row parent probes.
	orphanParents := map[string]bool{}
	orphanRows, e := tx.Query("SELECT id FROM etsy_orphan_parents")
	if e != nil {
		return result, e
	}
	for orphanRows.Next() {
		var id sql.NullString
		if e = orphanRows.Scan(&id); e != nil {
			orphanRows.Close()
			return result, e
		}
		orphanParents[id.String] = true
	}
	e = orphanRows.Err()
	orphanRows.Close()
	if e != nil {
		return result, e
	}
	defer tx.Exec("DROP TABLE IF EXISTS etsy_orphan_parents")
	defer tx.Exec("DROP TABLE IF EXISTS etsy_stored_receipts")
	if o.Erase != nil {
		if err = etsySelectErasure(tx, o.Erase); err != nil {
			return result, err
		}
	}
	orphanSelected, orphanVeto := map[string]bool{}, map[string]bool{}
	if !o.ClearFloor {
		if o.Receipts && o.Erase == nil {
			// UNION includes typed-only and generic-only legacy rows. JSON is guarded
			// before extraction; identifiers stay strings, including numeric API ids.
			rows, e := tx.Query(`SELECT id, CAST(receipt_id AS TEXT), COALESCE(created_timestamp,create_timestamp,CASE WHEN json_valid(data) THEN json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.create_timestamp') END),COALESCE(is_shipped,CASE WHEN json_valid(data) THEN json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.is_shipped') END) FROM receipts UNION SELECT id,CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id') AS TEXT),COALESCE(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.created_timestamp'),json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.create_timestamp')),json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.is_shipped') FROM resources WHERE resource_type='receipts' AND json_valid(data)`)
			if e != nil {
				return result, e
			}
			type receipt struct {
				id      string
				age     sql.NullInt64
				shipped sql.NullBool
			}
			var all []receipt
			for rows.Next() {
				var r receipt
				var api sql.NullString
				if e = rows.Scan(&r.id, &api, &r.age, &r.shipped); e != nil {
					rows.Close()
					return result, e
				}
				if api.Valid {
					r.id = api.String
				} else {
					r.id = BareResourceID(r.id)
				}
				all = append(all, r)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return result, e
			}
			// Any unshipped representation vetoes pruning the entire receipt.
			veto := map[string]bool{}
			selected := map[string]bool{}
			missing := map[string]bool{}
			shipped := map[string]bool{}
			for _, r := range all {
				if r.shipped.Valid && !r.shipped.Bool {
					veto[r.id] = true
				}
				if r.shipped.Valid && r.shipped.Bool {
					shipped[r.id] = true
				}
				if !r.age.Valid {
					missing[r.id] = true
				}
				if r.age.Valid && r.age.Int64 < o.Cutoff {
					selected[r.id] = true
				}
			}
			for id := range missing {
				if !selected[id] && !veto[id] {
					result.KeptNoTimestamp++
				}
			}
			for id := range selected {
				if veto[id] {
					result.KeptUnshipped++
					continue
				}
				if !shipped[id] {
					result.KeptUnknownShipped++
					continue
				}
				if !veto[id] {
					if _, err = tx.Exec(`INSERT INTO etsy_prune_receipts VALUES(?)`, id); err != nil {
						return result, err
					}
				}
			}
		}
		types := []string{}
		if o.Receipts {
			types = append(types, "receipts", "transactions", "payments", "shops_receipts_payments", "listings_transactions", "receipts_listings", "etsy_payment_checks")
		}
		if o.Reviews {
			types = append(types, "reviews", "listings_reviews")
		}
		if o.Erase != nil {
			for _, kind := range EtsyBuyerTypes() {
				if kind != "user_addresses" && !containsKind(types, kind) {
					types = append(types, kind)
				}
			}
		}
		for _, kind := range types {
			typed := ""
			if kind == "receipts" || kind == "transactions" || kind == "payments" || kind == "reviews" || (o.Erase != nil && (kind == "users" || kind == "economic_operators")) {
				typed = kind
			}
			query := `SELECT id,data FROM resources WHERE resource_type=?`
			args := []any{kind}
			if typed != "" {
				query += ` UNION SELECT id,data FROM "` + typed + `"`
			}
			rows, e := tx.Query(query, args...)
			if e != nil {
				return result, e
			}
			type row struct{ id, data, key string }
			var candidates []row
			for rows.Next() {
				var r row
				if e = rows.Scan(&r.id, &r.data); e != nil {
					rows.Close()
					return result, e
				}
				candidates = append(candidates, r)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return result, e
			}
			selectedKeys := map[string]bool{}
			noTimestamp := map[string]bool{}
			for i := range candidates {
				r := &candidates[i]
				r.key = BareResourceID(r.id)
				field := ""
				switch kind {
				case "receipts", "etsy_payment_checks":
					field = "receipt_id"
				case "transactions", "listings_transactions", "reviews", "listings_reviews":
					field = "transaction_id"
				case "payments", "shops_receipts_payments":
					field = "payment_id"
				case "receipts_listings":
					// Listing identities repeat across receipts; preserve the full scoped key.
					r.key = r.id
				}
				if field != "" {
					var identity sql.NullString
					identitySQL := `SELECT CASE WHEN json_valid(?) THEN CAST(json_extract(?,?) AS TEXT) END`
					identityArgs := []any{r.data, r.data, "$." + field}
					if typed != "" {
						identitySQL = `SELECT COALESCE((` + identitySQL + `),(SELECT CAST("` + field + `" AS TEXT) FROM "` + typed + `" WHERE id=?))`
						identityArgs = append(identityArgs, r.id)
					}
					if e = tx.QueryRow(identitySQL, identityArgs...).Scan(&identity); e != nil {
						return result, e
					}
					if identity.Valid {
						r.key = identity.String
					}
				}
			}
			for _, r := range candidates {
				var hit bool
				if o.Erase != nil {
					hit, e = etsyErasureHit(tx, kind, r.id, r.key, r.data)
					if e != nil {
						return result, e
					}
					if !hit && (kind == "reviews" || kind == "listings_reviews") {
						var buyer, transaction sql.NullString
						if e = tx.QueryRow(`SELECT CASE WHEN json_valid(?) THEN CAST(json_extract(?,'$.buyer_user_id') AS TEXT) END, CASE WHEN json_valid(?) THEN CAST(json_extract(?,'$.transaction_id') AS TEXT) END`, r.data, r.data, r.data, r.data).Scan(&buyer, &transaction); e != nil {
							return result, e
						}
						var linked int
						if e = tx.QueryRow(`SELECT (SELECT count(*) FROM transactions WHERE CAST(transaction_id AS TEXT)=?) + (SELECT count(*) FROM resources WHERE resource_type IN ('transactions','listings_transactions') AND CASE WHEN json_valid(data) THEN CAST(json_extract(data,'$.transaction_id') AS TEXT) END=?)`, transaction.String, transaction.String).Scan(&linked); e != nil {
							return result, e
						}
						if !buyer.Valid && linked == 0 {
							result.KeptUnlinkableReviews++
						}
					}
				} else if kind == "reviews" || kind == "listings_reviews" {
					var age sql.NullInt64
					// Typed timestamp columns are authoritative fallback when legacy JSON lacks age.
					q := `SELECT CASE WHEN json_valid(?) THEN COALESCE(json_extract(?,'$.created_timestamp'),json_extract(?,'$.create_timestamp')) END`
					a := []any{r.data, r.data, r.data}
					if kind == "reviews" {
						q = `SELECT COALESCE((SELECT COALESCE(created_timestamp,create_timestamp) FROM reviews WHERE id=?),(` + q + `))`
						a = append([]any{r.id}, a...)
					}
					if e = tx.QueryRow(q, a...).Scan(&age); e != nil {
						return result, e
					}
					if !age.Valid {
						noTimestamp[r.key] = true
					} else {
						hit = age.Int64 < o.Cutoff
					}
				} else {
					// String-bound identity comparison; bare-id membership happens in Go
					// because SQLite substr/instr truncate NUL-composite storage keys.
					var ref sql.NullString
					e = tx.QueryRow(`SELECT CASE WHEN json_valid(?) THEN CAST(COALESCE(json_extract(?,'$.receipt_id'),json_extract(?,'$.receipts_id'),json_extract(?,'$.parent_id')) AS TEXT) END`, r.data, r.data, r.data, r.data).Scan(&ref)
					if e != nil {
						return result, e
					}
					if !ref.Valid && (kind == "transactions" || kind == "payments") {
						if e = tx.QueryRow(`SELECT CAST(receipt_id AS TEXT) FROM "`+kind+`" WHERE id=?`, r.id).Scan(&ref); e != nil && e != sql.ErrNoRows {
							return result, e
						}
					}
					id := ref.String
					if kind == "receipts" && !ref.Valid {
						id = BareResourceID(r.id)
					}
					if (kind == "receipts_listings" || kind == "shops_receipts_payments") && !ref.Valid {
						var parent sql.NullString
						if e = tx.QueryRow(`SELECT CASE WHEN json_valid(?) THEN CAST(json_extract(?,'$.parent_id') AS TEXT) END`, r.data, r.data).Scan(&parent); e != nil {
							return result, e
						}
						id = parent.String
						if !parent.Valid {
							id = resourceParentID(r.id)
						}
					}
					var n int
					if e = tx.QueryRow(`SELECT count(*) FROM etsy_prune_receipts WHERE id=?`, id).Scan(&n); e != nil {
						return result, e
					}
					hit = n > 0
					if !hit && typed != "" && kind != "receipts" {
						if e = tx.QueryRow(`SELECT count(*) FROM "`+typed+`" WHERE id=? AND CAST(receipt_id AS TEXT) IN (SELECT id FROM etsy_prune_receipts)`, r.id).Scan(&n); e != nil {
							return result, e
						}
						hit = n > 0
					}
					// etsy-ext: orphan purchase rows use their own age, associations remain parent-only.
					if !hit && (kind == "transactions" || kind == "listings_transactions" || kind == "payments" || kind == "shops_receipts_payments") {
						if orphanParents[id] || id == "" {
							var m map[string]json.RawMessage
							_ = json.Unmarshal([]byte(r.data), &m)
							age, ageErr := strconv.ParseInt(etsyScalar(m, "created_timestamp"), 10, 64)
							if ageErr != nil {
								age, ageErr = strconv.ParseInt(etsyScalar(m, "create_timestamp"), 10, 64)
							}
							hasAge := ageErr == nil
							if typed != "" && !hasAge {
								var legacy sql.NullInt64
								if e = tx.QueryRow("SELECT COALESCE(created_timestamp,create_timestamp) FROM "+typed+" WHERE id=?", r.id).Scan(&legacy); e != nil && e != sql.ErrNoRows {
									return result, e
								}
								if legacy.Valid {
									age = legacy.Int64
									hasAge = true
								}
							}
							hit = hasAge && age < o.Cutoff
							// etsy-ext: an orphan with an unknown/unshipped timestamp stays.
							if _, exists := m["shipped_timestamp"]; exists && (etsyScalar(m, "shipped_timestamp") == "" || etsyScalar(m, "shipped_timestamp") == "0") {
								orphanVeto[id] = true
								hit = false
							}
							if _, exists := m["shipped_timestamp"]; !exists && typed != "" {
								var shipped sql.NullInt64
								if e = tx.QueryRow("SELECT shipped_timestamp FROM "+typed+" WHERE id=?", r.id).Scan(&shipped); e != nil && e != sql.ErrNoRows {
									return result, e
								}
								if e == nil && (!shipped.Valid || shipped.Int64 == 0) {
									orphanVeto[id] = true
									hit = false
								}
							}

							if hit && id != "" {
								orphanSelected[id] = true
							}
						}
					}
				}
				if hit {
					selectedKeys[r.key] = true
				}
			}
			// Include every stored form of selected entities, even if only one
			// projection retained its receipt reference. Compare bare keys in Go.
			for _, r := range candidates {
				if selectedKeys[r.key] {
					if err = etsyRememberSelectedTransactions(tx, kind, r.key, r.data); err != nil {
						return result, err
					}
					if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_learning_ids SELECT ?,? WHERE ? != 'receipts_listings'`, kind, r.key, kind); err != nil {
						return result, err
					}
					if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_victims VALUES(?,?)`, kind, r.id); err != nil {
						return result, err
					}
				}
			}
			for key := range noTimestamp {
				if !selectedKeys[key] {
					result.KeptNoTimestamp++
				}
			}
		}
		exists, e := etsyTableExists(tx, "etsy_payment_checks")
		if e != nil {
			return result, e
		}
		if exists && o.Receipts {
			if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_victims SELECT 'etsy_payment_checks',receipt_id FROM etsy_payment_checks WHERE receipt_id IN (SELECT id FROM etsy_prune_receipts)`); err != nil {
				return result, err
			}
		}
	}
	// etsy-ext: count selected receipts that cannot be suppressed by buyer or receipt.
	countUnsuppressed := func() error {
		exists, e := etsyTableExists(tx, "etsy_erasure_receipts")
		if e != nil {
			return e
		}
		bare := "CASE WHEN instr(id,char(0))>0 THEN substr(id,1,instr(id,char(0))-1) ELSE id END"
		q := "SELECT count(*) FROM etsy_prune_receipts p WHERE p.id NOT IN (SELECT COALESCE(CAST(receipt_id AS TEXT)," + bare + ") FROM receipts WHERE buyer_user_id>0 UNION SELECT COALESCE(CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.receipt_id') AS TEXT)," + bare + ") FROM resources WHERE resource_type='receipts' AND CAST(json_extract(CASE WHEN json_valid(data) THEN data ELSE '{}' END,'$.buyer_user_id') AS INTEGER)>0)"
		if exists {
			q += " AND NOT EXISTS(SELECT 1 FROM etsy_erasure_receipts e WHERE e.id=p.id)"
		}
		return tx.QueryRow(q).Scan(&result.UnsuppressedReceipts)
	}
	if err = countUnsuppressed(); err != nil {
		return result, err
	}
	// Writable opens scrub legacy review buyer columns. Preserve review victims
	// selected read-only before that scrub, without retaining buyer data itself.
	if o.Erase != nil {
		for kind, ids := range o.Erase.PreScrubReviews {
			if kind != "reviews" && kind != "listings_reviews" {
				continue
			}
			for _, id := range ids {
				if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_prune_victims VALUES(?,?)`, kind, id); err != nil {
					return result, err
				}
			}
		}
	}
	rows, err := tx.Query(`SELECT kind,id FROM etsy_prune_victims ORDER BY CASE kind WHEN 'reviews' THEN 0 WHEN 'listings_reviews' THEN 0 WHEN 'transactions' THEN 1 WHEN 'listings_transactions' THEN 1 WHEN 'payments' THEN 2 WHEN 'etsy_payment_checks' THEN 2 WHEN 'receipts' THEN 4 ELSE 3 END,id`)
	if err != nil {
		return result, err
	}
	type victim struct{ kind, id string }
	var victims []victim
	for rows.Next() {
		var v victim
		if err = rows.Scan(&v.kind, &v.id); err != nil {
			rows.Close()
			return result, err
		}
		victims = append(victims, v)
		result.Counts[v.kind]++
		if v.kind == "reviews" || v.kind == "listings_reviews" {
			result.ReviewIDs[v.kind] = append(result.ReviewIDs[v.kind], v.id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.ErasedIDs = map[string]bool{}
	for _, v := range victims {
		if v.kind == "receipts_listings" {
			continue
		}
		result.ErasedIDs[v.id] = true
		result.ErasedIDs[BareResourceID(v.id)] = true
	}
	rows, err = tx.Query(`SELECT id FROM etsy_prune_learning_ids UNION SELECT id FROM etsy_prune_tx`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return result, err
		}
		result.ErasedIDs[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if o.Erase != nil {
		rows, err = tx.Query(`SELECT id FROM etsy_erase_buyers`)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return result, err
			}
			result.BuyerIDs = append(result.BuyerIDs, id)
			if len(victims) > 0 {
				result.ErasedIDs[id] = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
	}
	// etsy-ext: preview the privacy consequences before deleting transaction links.
	if o.Receipts && o.Erase == nil && !o.ClearFloor && !o.Reviews {
		seen := map[string]bool{}
		survivingLinks := map[string]bool{}
		for _, kind := range []string{"transactions", "listings_transactions"} {
			rows, e := etsyErasureRows(tx, kind)
			if e != nil {
				return result, e
			}
			for _, r := range rows {
				var selected int
				if e = tx.QueryRow(`SELECT count(*) FROM etsy_prune_victims WHERE kind=? AND id=?`, kind, r.id).Scan(&selected); e != nil {
					return result, e
				}
				if selected == 0 {
					var m map[string]json.RawMessage
					_ = json.Unmarshal([]byte(r.data), &m)
					survivingLinks[etsyScalar(m, "transaction_id")] = true
				}
			}
		}
		for _, kind := range []string{"reviews", "listings_reviews"} {
			rows, e := etsyErasureRows(tx, kind)
			if e != nil {
				return result, e
			}
			for _, r := range rows {
				var m map[string]json.RawMessage
				_ = json.Unmarshal([]byte(r.data), &m)
				id := etsyScalar(m, "transaction_id")
				if id == "" || survivingLinks[id] || EtsyValidateErasureID(etsyScalar(m, "buyer_user_id")) == nil {
					continue
				}
				var n int
				if e = tx.QueryRow(`SELECT count(*) FROM etsy_prune_tx WHERE id=?`, id).Scan(&n); e != nil {
					return result, e
				}
				if n > 0 {
					seen[id] = true
				}
			}
		}
		result.ReviewsUnlinkableAfterPrune = len(seen)
	}
	if o.ClearFloor && o.Receipts {
		if exists, e := etsyTableExists(tx, "etsy_retention_receipts"); e != nil {
			return result, e
		} else if exists {
			if e = tx.QueryRow(`SELECT count(*) FROM etsy_retention_receipts`).Scan(&result.ReadmittedReceipts); e != nil {
				return result, e
			}
		}
	}
	if o.Erase != nil {
		for _, id := range o.Erase.UnsuppressReceipts {
			if exists, e := etsyTableExists(tx, "etsy_erasure_buyer_receipts"); e != nil {
				return result, e
			} else if exists {
				var n int
				if e = tx.QueryRow(`SELECT count(*) FROM etsy_erasure_buyer_receipts r JOIN etsy_erasure_suppression b ON b.buyer_user_id=r.buyer_user_id WHERE r.receipt_id=?`, etsyCanonicalID(id)).Scan(&n); e != nil {
					return result, e
				}
				result.SuppressedBuyerReceipt = result.SuppressedBuyerReceipt || n > 0
			}
		}
	}
	if o.Apply {
		if o.CleanupLearn != nil {
			if err = o.CleanupLearn(tx, result.ErasedIDs); err != nil {
				return result, err
			}
		}
		if err = etsyDeleteResourceLearnings(tx, result.ErasedIDs); err != nil {
			return result, err
		}
		if o.Erase != nil {
			if err = etsyPersistErasure(tx, o.Erase); err != nil {
				return result, err
			}
		}
		if err = countUnsuppressed(); err != nil {
			return result, err
		}
		if _, err = tx.Exec(etsyRetentionSchema); err != nil {
			return result, err
		}
		if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS etsy_retention_receipts(id TEXT PRIMARY KEY)`); err != nil {
			return result, err
		}
		if err = etsyRememberPaymentSuccess(tx); err != nil {
			return result, err
		}
		// etsy-ext: remember selected orphan parents unless any projection vetoed them.
		if o.Receipts && o.Erase == nil && !o.ClearFloor {
			for id := range orphanSelected {
				if !orphanVeto[id] {
					if _, err = tx.Exec(`INSERT OR IGNORE INTO etsy_retention_receipts VALUES(?)`, id); err != nil {
						return result, err
					}
				}
			}
		}
		for _, v := range victims {
			switch v.kind {
			case "receipts", "transactions", "payments", "reviews", "users", "economic_operators":
				_, err = tx.Exec(`DELETE FROM "`+v.kind+`" WHERE id=?`, v.id)
			case "etsy_payment_checks":
				var exists bool
				exists, err = etsyTableExists(tx, "etsy_payment_checks")
				if err == nil && exists {
					_, err = tx.Exec(`DELETE FROM etsy_payment_checks WHERE receipt_id=?`, v.id)
				}
			}
			if err != nil {
				return result, err
			}
			if _, err = tx.Exec(`DELETE FROM resources WHERE resource_type=? AND id=?`, v.kind, v.id); err != nil {
				return result, err
			}
			for _, id := range []string{v.id, BareResourceID(v.id)} {
				if _, err = tx.Exec(`DELETE FROM resources_fts WHERE rowid=?`, ftsRowID(v.kind, id)); err != nil {
					return result, err
				}
			}
		}
		if o.Receipts && o.Erase == nil {
			if o.ClearFloor {
				_, err = tx.Exec(`DELETE FROM etsy_retention_receipts`)
			} else {
				_, err = tx.Exec(`INSERT OR IGNORE INTO etsy_retention_receipts SELECT id FROM etsy_prune_receipts`)
			}
			if err != nil {
				return result, err
			}
		}
	}
	exists, err := etsyTableExists(tx, "etsy_retention_floor")
	if err != nil {
		return result, err
	}
	if exists {
		rows, err = tx.Query(`SELECT scope,cutoff FROM etsy_retention_floor`)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var scope string
			var cutoff int64
			if err = rows.Scan(&scope, &cutoff); err != nil {
				rows.Close()
				return result, err
			}
			result.Floor[scope] = cutoff
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
	}
	for _, scope := range []string{"receipts", "reviews"} {
		if o.Erase != nil {
			continue
		}
		if (scope == "receipts" && !o.Receipts) || (scope == "reviews" && !o.Reviews) {
			continue
		}
		if o.ClearFloor {
			delete(result.Floor, scope)
			if o.Apply {
				_, err = tx.Exec(`DELETE FROM etsy_retention_floor WHERE scope=?`, scope)
			}
		} else {
			cutoff := o.Cutoff
			if result.Floor[scope] > cutoff {
				cutoff = result.Floor[scope]
			}
			result.Floor[scope] = cutoff
			if o.Apply {
				_, err = tx.Exec(`INSERT INTO etsy_retention_floor VALUES(?,?) ON CONFLICT(scope) DO UPDATE SET cutoff=MAX(cutoff,excluded.cutoff)`, scope, cutoff)
			}
		}
		if err != nil {
			return result, err
		}
	}
	if _, err = tx.Exec(`DROP TABLE etsy_orphan_parents; DROP TABLE etsy_stored_receipts; DROP TABLE etsy_prune_tx`); err != nil {
		return result, err
	}
	if _, err = tx.Exec(`DROP TABLE etsy_prune_learning_ids`); err != nil {
		return result, err
	}
	if _, err = tx.Exec(`DROP TABLE etsy_prune_victims`); err != nil {
		return result, err
	}
	if _, err = tx.Exec(`DROP TABLE etsy_prune_receipts`); err != nil {
		return result, err
	}
	if o.Erase != nil {
		for _, name := range []string{"etsy_erase_buyers"} {
			if _, err = tx.Exec(`DROP TABLE ` + name); err != nil {
				return result, err
			}
		}
	}
	if o.Apply {
		err = tx.Commit()
		result.Committed = err == nil
		if err == nil && o.Vacuum && (o.Erase == nil || o.ExplicitVacuum || len(victims) > 0) {
			result.VacuumSteps = map[string]string{}
			result.Vacuum, err = etsyVacuum(ctx, conn, result.VacuumSteps)
		}
	}
	return result, err
}

func resourceParentID(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == 0 {
			return id[i+1:]
		}
	}
	return ""
}

// etsy-ext: write policy rejects incoming rows only; stored updates await prune.
func etsyRetentionDrop(tx *sql.Tx, kind string, data json.RawMessage, storageIDs ...string) (bool, error) {
	id := ""
	if len(storageIDs) > 0 {
		id = storageIDs[0]
	}
	return etsyRetentionDecision(tx, kind, data, id, true)
}

func etsyRetentionDecision(tx *sql.Tx, kind string, data json.RawMessage, storageID string, remember bool) (bool, error) {
	if drop, err := etsySuppressionDrop(tx, kind, data); drop || err != nil {
		return drop, err
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return false, nil
	}
	scope := ""
	switch kind {
	case "receipts", "transactions", "listings_transactions", "payments", "shops_receipts_payments", "receipts_listings", "etsy_payment_checks":
		scope = "receipts"
	case "reviews", "listings_reviews":
		scope = "reviews"
	default:
		return false, nil
	}
	if scope == "receipts" {
		id := etsyScalar(m, "receipt_id")
		if id == "" {
			id = etsyScalar(m, "receipts_id")
		}
		if id == "" {
			id = etsyScalar(m, "parent_id")
		}
		if exists, err := etsyTableExists(tx, "etsy_retention_receipts"); err != nil {
			return false, err
		} else if exists {
			var n int
			if err = tx.QueryRow("SELECT count(*) FROM etsy_retention_receipts WHERE id=?", id).Scan(&n); err != nil {
				return false, err
			}
			if n > 0 {
				return true, nil
			}
		}
		if kind != "receipts" {
			return false, nil
		}
		if v := string(m["is_shipped"]); v != "true" && v != "1" {
			return false, nil
		}
	}
	if exists, err := etsyTableExists(tx, "etsy_retention_floor"); err != nil || !exists {
		return false, err
	}
	var cutoff int64
	if err := tx.QueryRow("SELECT cutoff FROM etsy_retention_floor WHERE scope=?", scope).Scan(&cutoff); err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return false, nil
	}
	id := ExtractResourceID(kind, obj)
	if kind == "reviews" || kind == "listings_reviews" {
		id = etsyScalar(m, "transaction_id")
	}
	if id == "" {
		id = storageID
	}
	if id == "" {
		id = ResolveStorageID(kind, obj)
	}
	if kind == "receipts" {
		var n int
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM receipts WHERE CAST(receipt_id AS TEXT)=? UNION ALL SELECT 1 FROM resources WHERE resource_type='receipts' AND CASE WHEN json_valid(data) THEN CAST(json_extract(data,'$.receipt_id') AS TEXT) END=?)", id, id).Scan(&n); err != nil {
			return false, err
		} else if n > 0 {
			return false, nil
		}
	}
	// PK ranges cover both bare and parent-scoped copies without decoding a table.
	var n int
	q := "SELECT EXISTS(SELECT 1 FROM resources WHERE resource_type=? AND id=? UNION ALL SELECT 1 FROM resources WHERE resource_type=? AND id>=? AND id<?)"
	if err = tx.QueryRow(q, kind, id, kind, id+string(rune(0)), id+string(rune(1))).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if kind == "receipts" || kind == "reviews" {
		if err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM "+kind+" WHERE id=? OR (id>=? AND id<?))", id, id+string(rune(0)), id+string(rune(1))).Scan(&n); err != nil {
			return false, err
		}
		if n > 0 {
			return false, nil
		}
	}
	for _, key := range []string{"created_timestamp", "create_timestamp"} {
		if age, err := strconv.ParseInt(etsyScalar(m, key), 10, 64); err == nil {
			if age < cutoff && kind == "receipts" && remember && id != "" {
				// etsy-ext: rejected receipts must keep later dependents out too.
				if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS etsy_retention_receipts(id TEXT PRIMARY KEY)`); err != nil {
					return false, err
				}
				if _, err := tx.Exec(`INSERT OR IGNORE INTO etsy_retention_receipts VALUES(?)`, id); err != nil {
					return false, err
				}
			}
			return age < cutoff, nil
		}
	}
	return false, nil
}

func etsyRememberPaymentSuccess(tx *sql.Tx) error {
	exists, err := etsyTableExists(tx, "etsy_payment_checks")
	if err != nil || !exists {
		return err
	}
	var hasEver int
	if err = tx.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='ever_ok'`).Scan(&hasEver); err != nil {
		return err
	}
	var hasStatus int
	if err = tx.QueryRow(`SELECT count(*) FROM pragma_table_info('etsy_payment_checks') WHERE name='status'`).Scan(&hasStatus); err != nil {
		return err
	}
	test := `receipt_updated IS NOT NULL`
	if hasStatus > 0 {
		test = `status='ok' OR (status IS NULL AND receipt_updated IS NOT NULL)`
	}
	if hasEver > 0 {
		test += ` OR ever_ok=1`
	}
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS etsy_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL)`); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT OR IGNORE INTO etsy_meta(key,value) SELECT 'payments_ever_succeeded','1' WHERE EXISTS(SELECT 1 FROM etsy_payment_checks WHERE ` + test + `)`)
	return err
}

func (s *Store) PaymentsEverSucceeded() (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='etsy_meta'`).Scan(&n); err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	err := s.db.QueryRow(`SELECT count(*) FROM etsy_meta WHERE key='payments_ever_succeeded' AND value='1'`).Scan(&n)
	return n > 0, err
}

// FilterEtsyRetentionIncoming lets sync distinguish policy exclusions from
// failed storage. The post-merge write guard remains authoritative for every
// writer, including callers that do not use this progress-reporting helper.
func (s *Store) FilterEtsyRetentionIncoming(kind string, items []json.RawMessage) ([]json.RawMessage, int, error) {
	if !containsKind(EtsyBuyerTypes(), kind) {
		return items, 0, nil
	}
	var active int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('etsy_retention_floor','etsy_erasure_suppression')`).Scan(&active); err != nil {
		return nil, 0, err
	}
	if active == 0 {
		return items, 0, nil
	}
	// etsy-ext: progress filtering is read-only; the upsert guard records exclusions.
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	kept := make([]json.RawMessage, 0, len(items))
	dropped := 0
	for _, item := range items {
		merged := item
		if obj, e := DecodeJSONObject(item); e == nil {
			id := ExtractResourceID(kind, obj)
			if id == "" {
				id = ResolveStorageID(kind, obj)
			}
			if id != "" {
				merged = s.mergeIncomingResourceData(tx, kind, resourceStorageID(kind, id, obj), item)
			}
		}
		drop, e := etsyRetentionDecision(tx, kind, merged, "", false)
		if e != nil {
			return nil, 0, e
		}
		if drop {
			dropped++
		} else {
			kept = append(kept, item)
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, 0, err
	}
	return kept, dropped, nil
}

func etsyIdentityFields(kind string) []string {
	switch kind {
	case "receipts":
		return []string{"receipt_id"}
	case "transactions", "listings_transactions", "reviews", "listings_reviews":
		return []string{"transaction_id"}
	case "payments", "shops_receipts_payments":
		return []string{"payment_id"}
	case "users":
		return []string{"user_id"}
	case "economic_operators":
		return []string{"economic_operator_id"}
	}
	return nil
}
