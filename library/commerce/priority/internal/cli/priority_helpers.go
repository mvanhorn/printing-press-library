// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the Priority-specific hand-written commands: tenant
// identity, $metadata fetch/cache, and JSON field extraction.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/priorityx"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// tenantKeyFromClient normalizes the service-root URL into a stable per-tenant
// cache key so one local store can serve several Priority instances.
func tenantKeyFromClient(c *client.Client) string {
	u := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(c.RequestBaseURL())), "/")
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return u
}

// fetchMetadataEDMX downloads the raw $metadata EDMX document.
func fetchMetadataEDMX(ctx context.Context, c *client.Client) ([]byte, error) {
	headers := map[string]string{client.BinaryResponseHeader: "true"}
	data, err := c.GetWithHeadersNoCache(ctx, "/$metadata", nil, headers)
	if err != nil {
		return nil, fmt.Errorf("fetching $metadata: %w", err)
	}
	return data, nil
}

// cacheMetadata replaces the pp_meta_* rows for a tenant with freshly parsed
// forms and stamps pp_meta_state. Runs in one write transaction.
func cacheMetadata(ctx context.Context, db *store.Store, tenant string, forms []priorityx.Form) (int, int, error) {
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin metadata cache tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, tbl := range []string{"pp_meta_forms", "pp_meta_fields", "pp_meta_subforms"} {
		// #nosec G202 -- tbl iterates a hardcoded table-name literal above; tenant is parameterized.
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+tbl+` WHERE tenant = ?`, tenant); err != nil {
			return 0, 0, fmt.Errorf("clearing %s: %w", tbl, err)
		}
	}
	fieldCount := 0
	for _, f := range forms {
		if _, err := tx.ExecContext(ctx, `INSERT INTO pp_meta_forms (tenant, form) VALUES (?, ?)`, tenant, f.Name); err != nil {
			return 0, 0, fmt.Errorf("inserting form %s: %w", f.Name, err)
		}
		for _, fl := range f.Fields {
			mand := 0
			if fl.Mandatory {
				mand = 1
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO pp_meta_fields (tenant, form, field, type, mandatory, description) VALUES (?, ?, ?, ?, ?, ?)`,
				tenant, f.Name, fl.Name, fl.Type, mand, fl.Description); err != nil {
				return 0, 0, fmt.Errorf("inserting field %s.%s: %w", f.Name, fl.Name, err)
			}
			fieldCount++
		}
		for _, sf := range f.Subforms {
			coll := 0
			if sf.Collection {
				coll = 1
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO pp_meta_subforms (tenant, form, subform, target, collection) VALUES (?, ?, ?, ?, ?)`,
				tenant, f.Name, sf.Name, sf.Target, coll); err != nil {
				return 0, 0, fmt.Errorf("inserting subform %s.%s: %w", f.Name, sf.Name, err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO pp_meta_state (tenant, refreshed_at, form_count, field_count) VALUES (?, ?, ?, ?)
		 ON CONFLICT(tenant) DO UPDATE SET refreshed_at = excluded.refreshed_at, form_count = excluded.form_count, field_count = excluded.field_count`,
		tenant, time.Now().UTC().Format(time.RFC3339), len(forms), fieldCount); err != nil {
		return 0, 0, fmt.Errorf("stamping pp_meta_state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("committing metadata cache: %w", err)
	}
	return len(forms), fieldCount, nil
}

// metadataCachePresent reports whether the tenant has a cached schema.
func metadataCachePresent(ctx context.Context, db *store.Store, tenant string) (bool, error) {
	var n int
	err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pp_meta_forms WHERE tenant = ?`, tenant).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// loadCachedForms rebuilds []priorityx.Form from the pp_meta_* cache using the
// drain-first pattern (scan each result set fully before the next query).
func loadCachedForms(ctx context.Context, db *store.Store, tenant string) ([]priorityx.Form, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT form, field, type, mandatory, COALESCE(description, '') FROM pp_meta_fields WHERE tenant = ? ORDER BY form, field`, tenant)
	if err != nil {
		return nil, fmt.Errorf("querying cached fields: %w", err)
	}
	type fieldRow struct {
		form string
		f    priorityx.Field
	}
	var fieldRows []fieldRow
	for rows.Next() {
		var fr fieldRow
		var mand int
		if err := rows.Scan(&fr.form, &fr.f.Name, &fr.f.Type, &mand, &fr.f.Description); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning cached field: %w", err)
		}
		fr.f.Mandatory = mand == 1
		fieldRows = append(fieldRows, fr)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	subRows, err := db.DB().QueryContext(ctx,
		`SELECT form, subform, COALESCE(target, ''), collection FROM pp_meta_subforms WHERE tenant = ? ORDER BY form, subform`, tenant)
	if err != nil {
		return nil, fmt.Errorf("querying cached subforms: %w", err)
	}
	type subRow struct {
		form string
		s    priorityx.Subform
	}
	var subRowsList []subRow
	for subRows.Next() {
		var sr subRow
		var coll int
		if err := subRows.Scan(&sr.form, &sr.s.Name, &sr.s.Target, &coll); err != nil {
			_ = subRows.Close()
			return nil, fmt.Errorf("scanning cached subform: %w", err)
		}
		sr.s.Collection = coll == 1
		subRowsList = append(subRowsList, sr)
	}
	if err := subRows.Err(); err != nil {
		_ = subRows.Close()
		return nil, err
	}
	if err := subRows.Close(); err != nil {
		return nil, err
	}

	formRows, err := db.DB().QueryContext(ctx, `SELECT form FROM pp_meta_forms WHERE tenant = ? ORDER BY form`, tenant)
	if err != nil {
		return nil, fmt.Errorf("querying cached forms: %w", err)
	}
	var names []string
	for formRows.Next() {
		var n string
		if err := formRows.Scan(&n); err != nil {
			_ = formRows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	if err := formRows.Err(); err != nil {
		_ = formRows.Close()
		return nil, err
	}
	if err := formRows.Close(); err != nil {
		return nil, err
	}

	byName := map[string]*priorityx.Form{}
	var forms []priorityx.Form
	for _, n := range names {
		forms = append(forms, priorityx.Form{Name: n})
	}
	for i := range forms {
		byName[forms[i].Name] = &forms[i]
	}
	for _, fr := range fieldRows {
		if f, ok := byName[fr.form]; ok {
			f.Fields = append(f.Fields, fr.f)
		}
	}
	for _, sr := range subRowsList {
		if f, ok := byName[sr.form]; ok {
			f.Subforms = append(f.Subforms, sr.s)
		}
	}
	return forms, nil
}

// refreshMetadataForTenant fetches, parses, and caches $metadata. Returns
// (formCount, fieldCount).
func refreshMetadataForTenant(ctx context.Context, c *client.Client, db *store.Store) (int, int, error) {
	raw, err := fetchMetadataEDMX(ctx, c)
	if err != nil {
		return 0, 0, err
	}
	forms, err := priorityx.ParseEDMX(raw)
	if err != nil {
		return 0, 0, err
	}
	return cacheMetadata(ctx, db, tenantKeyFromClient(c), forms)
}

// jsonNumField pulls a numeric field from a raw JSON object, accepting both
// JSON numbers and numeric strings (Priority emits both across tenants).
func jsonNumField(obj map[string]json.RawMessage, key string) (float64, bool) {
	raw, ok := obj[key]
	if !ok {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		if err := json.Unmarshal([]byte(s), &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// jsonStrField pulls a string field from a raw JSON object.
func jsonStrField(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// nullStr converts sql.NullString to its zero-value-safe string.
func nullStr(v sql.NullString) string { return v.String }
