// Package mock provides a pure-Go reference UCP merchant for end-to-end testing.
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type product struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Price int    `json:"price"`
	SKU   string `json:"sku"`
	URL   string `json:"url"`
}

var catalog = []product{
	{ID: "p1", Title: "Mock French Press", Price: 3500, SKU: "FP-001", URL: "/products/french-press"},
	{ID: "p2", Title: "Mock Coffee Beans 1lb", Price: 1495, SKU: "CB-1LB", URL: "/products/coffee-beans-1lb"},
	{ID: "p3", Title: "Mock Coffee Grinder", Price: 4999, SKU: "CG-100", URL: "/products/coffee-grinder"},
	{ID: "p4", Title: "Mock Coffee Mug 12oz", Price: 1200, SKU: "MG-12", URL: "/products/coffee-mug-12oz"},
	{ID: "p5", Title: "Mock Pour-Over Kit", Price: 2795, SKU: "PO-KIT", URL: "/products/pour-over-kit"},
}

// Start starts the mock UCP merchant HTTP server on addr and returns it.
func Start(addr string) (*http.Server, error) {
	mux := http.NewServeMux()

	// Build base URL from addr for the manifest endpoints
	baseURL := "http://" + addr

	mux.HandleFunc("/.well-known/ucp", func(w http.ResponseWriter, r *http.Request) {
		manifest := map[string]interface{}{
			"ucp": map[string]interface{}{
				"version": "2026-04-08",
				"services": map[string]interface{}{
					"dev.ucp.shopping.catalog.search": []map[string]interface{}{
						{
							"version":   "2026-04-08",
							"transport": "rest",
							"endpoint":  baseURL + "/catalog/search",
						},
					},
					"dev.ucp.shopping.checkout": []map[string]interface{}{
						{
							"version":   "2026-04-08",
							"transport": "rest",
							"endpoint":  baseURL + "/checkout-sessions",
						},
					},
				},
				"capabilities": map[string]interface{}{
					"dev.ucp.shopping.catalog.search": []map[string]interface{}{
						{"version": "2026-04-08"},
					},
					"dev.ucp.shopping.checkout": []map[string]interface{}{
						{"version": "2026-04-08"},
					},
					"dev.ucp.shopping.cart": []map[string]interface{}{
						{"version": "2026-04-08"},
					},
				},
				"payment_handlers": map[string]interface{}{
					"com.google.pay": []map[string]interface{}{
						{
							"id":      "com.google.pay",
							"version": "2026-04-08",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	})

	mux.HandleFunc("/catalog/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.ToLower(r.URL.Query().Get("q"))
		var results []product
		for _, p := range catalog {
			if q == "" || strings.Contains(strings.ToLower(p.Title), q) {
				results = append(results, p)
			}
		}
		if results == nil {
			results = []product{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
	})

	mux.HandleFunc("/checkout-sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Compute total from line_items if present
		total := 0
		if liRaw, ok := body["line_items"]; ok {
			var lineItems []map[string]json.RawMessage
			if json.Unmarshal(liRaw, &lineItems) == nil {
				for _, li := range lineItems {
					qty := 1
					if qRaw, ok := li["quantity"]; ok {
						var q float64
						if json.Unmarshal(qRaw, &q) == nil {
							qty = int(q)
						}
					}
					if itemRaw, ok := li["item"]; ok {
						var item map[string]json.RawMessage
						if json.Unmarshal(itemRaw, &item) == nil {
							if pRaw, ok := item["price"]; ok {
								var p float64
								if json.Unmarshal(pRaw, &p) == nil {
									total += int(p) * qty
								}
							}
						}
					}
				}
			}
		}

		session := map[string]interface{}{
			"id":         fmt.Sprintf("sess_%s", uuid.New().String()[:8]),
			"status":     "ready_for_complete",
			"line_items": body["line_items"],
			"totals": []map[string]interface{}{
				{"type": "subtotal", "amount": total},
				{"type": "total", "amount": total},
			},
		}
		if buyerRaw, ok := body["buyer"]; ok {
			session["buyer"] = buyerRaw
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(session)
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	ln, err := listenTCP(addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	go func() { _ = srv.Serve(ln) }()
	return srv, nil
}
