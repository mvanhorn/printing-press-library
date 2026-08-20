package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/zohotools"
)

// openZohoStore opens the local SQLite store at the canonical path and
// ensures the merchant_memory and receipt_hashes tables exist. Returns
// the live *store.Store; the caller must Close it.
func openZohoStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("zoho-expense-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := zohotools.EnsureSchema(s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// resolveExpenseCategory turns either a category id or a category name
// into a (category_id, category_name) pair using the local store. Returns
// the input verbatim when no local mapping is found (so unsynced caches
// don't block tagging — Zoho will reject bad IDs with a clear error).
func resolveExpenseCategory(db *sql.DB, idOrName string) (string, string) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return "", ""
	}
	// Try id first.
	var id, name string
	err := db.QueryRow(
		`SELECT COALESCE(category_id,''), COALESCE(name,'') FROM expense_categories WHERE category_id = ? OR id = ?`,
		idOrName, idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	err = db.QueryRow(
		`SELECT COALESCE(category_id,''), COALESCE(name,'') FROM expense_categories WHERE LOWER(name) = LOWER(?)`,
		idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	return idOrName, ""
}

// resolveProject resolves a project id-or-name against the local store.
func resolveProject(db *sql.DB, idOrName string) (string, string) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return "", ""
	}
	var id, name string
	err := db.QueryRow(
		`SELECT COALESCE(project_id,''), COALESCE(project_name,'') FROM projects WHERE project_id = ? OR id = ?`,
		idOrName, idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	err = db.QueryRow(
		`SELECT COALESCE(project_id,''), COALESCE(project_name,'') FROM projects WHERE LOWER(project_name) = LOWER(?)`,
		idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	return idOrName, ""
}

// resolveCustomer resolves a customer id-or-name against the local store.
func resolveCustomer(db *sql.DB, idOrName string) (string, string) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return "", ""
	}
	var id, name string
	err := db.QueryRow(
		`SELECT COALESCE(customer_id,''), COALESCE(contact_name,'') FROM customers WHERE customer_id = ? OR id = ?`,
		idOrName, idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	err = db.QueryRow(
		`SELECT COALESCE(customer_id,''), COALESCE(contact_name,'') FROM customers
		 WHERE LOWER(contact_name) = LOWER(?) OR LOWER(company_name) = LOWER(?)`,
		idOrName, idOrName,
	).Scan(&id, &name)
	if err == nil && id != "" {
		return id, name
	}
	return idOrName, ""
}

// resolveTax resolves a tax id-or-name against the local store.
func resolveTax(db *sql.DB, idOrName string) (string, string, float64) {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return "", "", 0
	}
	var id, name string
	var pct float64
	err := db.QueryRow(
		`SELECT COALESCE(tax_id,''), COALESCE(tax_name,''), COALESCE(tax_percentage,0) FROM taxes WHERE tax_id = ? OR id = ?`,
		idOrName, idOrName,
	).Scan(&id, &name, &pct)
	if err == nil && id != "" {
		return id, name, pct
	}
	err = db.QueryRow(
		`SELECT COALESCE(tax_id,''), COALESCE(tax_name,''), COALESCE(tax_percentage,0) FROM taxes WHERE LOWER(tax_name) = LOWER(?)`,
		idOrName,
	).Scan(&id, &name, &pct)
	if err == nil && id != "" {
		return id, name, pct
	}
	return idOrName, "", 0
}

// fetchExpenseObject loads /expenses/{id} via the client and unwraps the
// Zoho {"expense": {...}} envelope. Used by hand-authored commands that
// need the live expense for downstream math (gst-split, tag inspection).
func fetchExpenseObject(ctx context.Context, c *client.Client, expenseID string) (map[string]any, error) {
	raw, err := c.Get(ctx, "/expenses/"+expenseID, nil)
	if err != nil {
		return nil, err
	}
	var env struct {
		Expense map[string]any `json:"expense"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Expense != nil {
		return env.Expense, nil
	}
	var bare map[string]any
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare, nil
	}
	return nil, fmt.Errorf("unexpected response shape for expense %s", expenseID)
}

// expenseFromLocal loads an expense row from the local store, returning
// the parsed JSON object or nil when absent.
func expenseFromLocal(db *sql.DB, expenseID string) map[string]any {
	var data string
	err := db.QueryRow(
		`SELECT data FROM expenses WHERE expense_id = ? OR id = ?`, expenseID, expenseID,
	).Scan(&data)
	if err != nil {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil
	}
	return obj
}

// asFloat coerces a JSON-decoded value (float64, json.Number, string) to
// float64. Returns 0 when the value can't be coerced.
func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		var f float64
		fmt.Sscanf(x, "%f", &f)
		return f
	}
	return 0
}

// asStringOpt returns a string value from a json map, "" when missing.
func asStringOpt(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}
