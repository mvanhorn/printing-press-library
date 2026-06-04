package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopify/internal/store"
)

type novelSeed struct {
	DBPath string
	Now    time.Time
}

func seedNovelReportDB(t *testing.T) novelSeed {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Hour)
	dbPath := filepath.Join(t.TempDir(), "shopify.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed db: %v", err)
	}
	defer s.Close()
	db := s.DB()

	insertOrder := func(id, name string, created time.Time, status, source, customerID, email string, total, refund float64, discounted bool, tags []string, items string, country, province string, shipping float64) {
		t.Helper()
		discounts := "[]"
		if discounted {
			discounts = `[{"__typename":"DiscountCodeApplication","targetType":"LINE_ITEM"}]`
		}
		data := fmt.Sprintf(`{"id":%q,"name":%q,"totalPriceSet":{"shopMoney":{"amount":"%.2f","currencyCode":"USD"}},"totalRefundedSet":{"shopMoney":{"amount":"%.2f","currencyCode":"USD"}},"customer":{"id":%q,"email":%q},"shippingAddress":{"countryCode":%q,"provinceCode":%q,"city":"Testville"},"shippingLines":{"nodes":[{"title":"Standard","originalPriceSet":{"shopMoney":{"amount":"%.2f","currencyCode":"USD"}}}]},"tags":%s,"discountApplications":{"nodes":%s},"lineItems":{"nodes":%s}}`, id, name, total, refund, customerID, email, country, province, shipping, mustMarshalString(t, tags), discounts, items)
		if _, err := db.Exec(`INSERT INTO orders (id,data,name,created_at,processed_at,display_financial_status,currency_code,source_name,note) VALUES (?,?,?,?,?,?,?,?,?)`, id, data, name, ts(created), ts(created), status, "USD", source, ""); err != nil {
			t.Fatalf("insert order %s: %v", id, err)
		}
	}

	insertOrder("o-old-c1", "#1000", now.AddDate(0, 0, -400), "PAID", "web", "gid://shopify/Customer/1", "c1@example.com", 70, 0, false, nil, `[{"id":"li-old","title":"Legacy A","quantity":1,"originalUnitPriceSet":{"shopMoney":{"amount":"70.00"}},"variant":{"id":"v-old","sku":"SKU-A","product":{"id":"pA","title":"Widget A","handle":"widget-a"}}}]`, "US", "CA", 7)
	insertOrder("o-cur-a", "#1001", now.AddDate(0, 0, -2), "PAID", "web", "gid://shopify/Customer/1", "c1@example.com", 100, 0, true, []string{"Klaviyo", "VIP"}, `[{"id":"li-1","title":"Widget A","quantity":2,"originalUnitPriceSet":{"shopMoney":{"amount":"20.00"}},"variant":{"id":"vA","sku":"SKU-A","product":{"id":"pA","title":"Widget A","handle":"widget-a"}}},{"id":"li-2","title":"Widget B","quantity":1,"originalUnitPriceSet":{"shopMoney":{"amount":"60.00"}},"variant":{"id":"vB","sku":"SKU-B","product":{"id":"pB","title":"Widget B","handle":"widget-b"}}}]`, "US", "CA", 25)
	insertOrder("o-cur-refund", "#1002", now.AddDate(0, 0, -1), "PARTIALLY_REFUNDED", "web", "gid://shopify/Customer/1", "c1@example.com", 50, 10, false, nil, `[{"id":"li-3","title":"Widget A","quantity":1,"originalUnitPriceSet":{"shopMoney":{"amount":"50.00"}},"variant":{"id":"vA","sku":"SKU-A","product":{"id":"pA","title":"Widget A","handle":"widget-a"}}}]`, "US", "CA", 0)
	insertOrder("o-prev", "#1003", now.AddDate(0, 0, -10), "PAID", "pos", "gid://shopify/Customer/2", "c2@example.com", 80, 0, false, nil, `[{"id":"li-4","title":"Widget B","quantity":2,"originalUnitPriceSet":{"shopMoney":{"amount":"40.00"}},"variant":{"id":"vB","sku":"SKU-B","product":{"id":"pB","title":"Widget B","handle":"widget-b"}}}]`, "US", "NY", 5)
	insertOrder("o-web-c2", "#1004", now.AddDate(0, 0, -20), "PAID", "pos", "gid://shopify/Customer/2", "c2@example.com", 200, 0, false, nil, `[{"id":"li-5","title":"Widget B","quantity":2,"originalUnitPriceSet":{"shopMoney":{"amount":"100.00"}},"variant":{"id":"vB","sku":"SKU-B","product":{"id":"pB","title":"Widget B","handle":"widget-b"}}}]`, "US", "NY", 60)
	insertOrder("o-email-c3", "#1005", now.AddDate(0, 0, -40), "PAID", "email", "gid://shopify/Customer/3", "c3@example.com", 30, 0, false, []string{"newsletter"}, `[{"id":"li-6","title":"Widget C","quantity":1,"originalUnitPriceSet":{"shopMoney":{"amount":"30.00"}},"variant":{"id":"vC","sku":"SKU-C","product":{"id":"pC","title":"Widget C","handle":"widget-c"}}}]`, "CA", "ON", 10)

	mustExec(t, db, `INSERT INTO customers (id,data,email,first_name,last_name,number_of_orders,created_at,updated_at,note) VALUES (?,?,?,?,?,?,?,?,?)`, "gid://shopify/Customer/1", `{"id":"gid://shopify/Customer/1","email":"c1@example.com"}`, "c1@example.com", "C", "One", 3, ts(now.AddDate(0, 0, -400)), ts(now), "")
	mustExec(t, db, `INSERT INTO products (id,data,title,handle,vendor,product_type,status,updated_at) VALUES (?,?,?,?,?,?,?,?)`, "pA", `{"id":"pA","title":"Widget A"}`, "Widget A", "widget-a", "Acme", "Widget", "ACTIVE", ts(now))
	mustExec(t, db, `INSERT INTO inventory_items (id,data,sku,tracked,updated_at) VALUES (?,?,?,?,?)`, "inv-a", `{"id":"inv-a","sku":"SKU-A","tracked":true,"inventoryLevels":{"nodes":[{"quantities":[{"name":"available","quantity":10}]}]}}`, "SKU-A", 1, ts(now))
	mustExec(t, db, `INSERT INTO inventory_items (id,data,sku,tracked,updated_at) VALUES (?,?,?,?,?)`, "inv-z", `{"id":"inv-z","sku":"SKU-Z","tracked":true,"inventoryLevels":{"nodes":[{"quantities":[{"name":"available","quantity":5}]}]}}`, "SKU-Z", 1, ts(now))
	mustExec(t, db, `INSERT INTO fulfillment_orders (id,data,status,request_status,fulfill_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, "fo-1", `{"id":"fo-1","status":"CLOSED"}`, "CLOSED", "FULFILLED", ts(now.AddDate(0, 0, -1)), ts(now.AddDate(0, 0, -2)), ts(now))
	mustExec(t, db, `INSERT INTO fulfillment_orders (id,data,status,request_status,fulfill_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, "fo-risk", `{"id":"fo-risk","status":"OPEN","assignedLocation":{"name":"Warehouse"}}`, "OPEN", "UNSUBMITTED", "", ts(now.AddDate(0, 0, -3)), ts(now))
	mustExec(t, db, `INSERT INTO abandoned_checkouts (id,data,name,created_at,updated_at,completed_at,abandoned_checkout_url,note) VALUES (?,?,?,?,?,?,?,?)`, "ac-1", `{"id":"ac-1","totalPriceSet":{"presentmentMoney":{"amount":"120.00","currencyCode":"USD"}}}`, "AC1", ts(now.AddDate(0, 0, -1)), ts(now), "", "", "")
	mustExec(t, db, `INSERT INTO abandoned_checkouts (id,data,name,created_at,updated_at,completed_at,abandoned_checkout_url,note) VALUES (?,?,?,?,?,?,?,?)`, "ac-2", `{"id":"ac-2","totalPriceSet":{"presentmentMoney":{"amount":"80.00","currencyCode":"USD"}}}`, "AC2", ts(now.AddDate(0, 0, -2)), ts(now), ts(now.AddDate(0, 0, -1)), "", "")

	return novelSeed{DBPath: dbPath, Now: now}
}

func runNovelReport(t *testing.T, dbPath string, args ...string) any {
	t.Helper()
	full := append([]string{"report", "--db", dbPath}, args...)
	return runNovelCommand(t, dbPath, full...)
}

func runNovelCommand(t *testing.T, dbPath string, args ...string) any {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if len(args) > 0 {
		switch args[0] {
		case "report", "growth", "ops", "merchandising", "store":
			withDB := []string{args[0], "--db", dbPath}
			withDB = append(withDB, args[1:]...)
			args = withDB
		}
	}
	full := append([]string{"--json"}, args...)
	cmd.SetArgs(full)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr=%s\nstdout=%s", full, err, errOut.String(), out.String())
	}
	var got any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %v output: %v\n%s", full, err, out.String())
	}
	return got
}

func mustMarshalString(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func obj(v any) map[string]any { return v.(map[string]any) }
func arr(v any) []any          { return v.([]any) }

func assertFloat(t *testing.T, got any, want float64) {
	t.Helper()
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("value %v (%T) is not float64", got, got)
	}
	if math.Abs(f-want) > 0.01 {
		t.Fatalf("got %.4f, want %.4f", f, want)
	}
}

func findRow(t *testing.T, rows []any, key string, want any) map[string]any {
	t.Helper()
	for _, row := range rows {
		m := obj(row)
		if m[key] == want {
			return m
		}
	}
	t.Fatalf("no row with %s=%v in %#v", key, want, rows)
	return nil
}
