// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: adds the required Shopper API headers to every authenticated request.
// These headers are mandatory — the real siteapi.shopper.com.br returns 401/empty
// without app-os-x-version, x-store-id, and x-cluster-id.
//
// The store the API answers for is selected ENTIRELY by the x-store-id /
// x-cluster-id request headers — NOT by POST /features/stores/select (that call
// is a no-op for subsequent reads). Because every CLI invocation is a fresh
// process, the active store must be chosen per-request via these headers. This
// file is the single source of truth for that mapping; the --store global flag
// and the SHOPPER_STORE env var both route through ResolveStore.
//
// Store map confirmed live via GET /features/stores (2026-08-03):
//   programada (mensal):  store_id=1, cluster_id=1, with_recurrence=true
//   fresh:                store_id=2, cluster_id=1, with_recurrence=true
//   unica (pontual):      store_id=3, cluster_id=3, with_recurrence=false
//   pet:                  store_id=5, cluster_id=3, with_recurrence=true
//   now:                  store_id=6, cluster_id=11, with_recurrence=false, ultra_fast=true
//   now-bebidas:          store_id=8, cluster_id=11, with_recurrence=false, ultra_fast=true

package client

import (
	"os"
	"strconv"
	"strings"
)

// Store identifies one Shopper storefront by its API store/cluster id pair.
// Values come from GET /features/stores (the `number` and `cluster_id` fields).
type Store struct {
	StoreID        string
	ClusterID      string
	Subdomain      string
	WithRecurrence bool
	UltraFast      bool
}

// shopperStores maps the human-facing store name (the storefront subdomain) to
// its API id pair. Keep in sync with GET /features/stores.
var shopperStores = map[string]Store{
	"programada":  {StoreID: "1", ClusterID: "1", Subdomain: "programada", WithRecurrence: true, UltraFast: false},
	"fresh":       {StoreID: "2", ClusterID: "1", Subdomain: "fresh", WithRecurrence: true, UltraFast: false},
	"unica":       {StoreID: "3", ClusterID: "3", Subdomain: "unica", WithRecurrence: false, UltraFast: false},
	"pet":         {StoreID: "5", ClusterID: "3", Subdomain: "pet", WithRecurrence: true, UltraFast: false},
	"now":         {StoreID: "6", ClusterID: "11", Subdomain: "now", WithRecurrence: false, UltraFast: true},
	"now-bebidas": {StoreID: "8", ClusterID: "11", Subdomain: "now-bebidas", WithRecurrence: false, UltraFast: true},
}

// storeAliases lets callers use the friendlier label the app shows.
var storeAliases = map[string]string{
	"mensal":      "programada",
	"monthly":     "programada",
	"pontual":     "unica",
	"única":       "unica",
	"bebidas":     "now-bebidas",
	"now bebidas": "now-bebidas",
	"nowbebidas":  "now-bebidas",
}

// StoreNames returns the canonical selectable store names (for help text and
// flag validation) in display order.
func StoreNames() []string {
	return []string{"programada", "fresh", "unica", "pet", "now", "now-bebidas"}
}

// SubscriptionStoreNames returns stores that operate on a recurring-basket model.
func SubscriptionStoreNames() []string {
	return []string{"programada", "fresh", "pet"}
}

// SpendStoreNames returns every storefront to report on by default.
// Ordered: subscription stores first, then one-off, then ultra-fast.
func SpendStoreNames() []string {
	return []string{"programada", "fresh", "pet", "unica", "now", "now-bebidas"}
}

// StorefrontURL returns the base web URL for the given store (subdomain.shopper.com.br).
// Used by browser-handoff commands to open the correct storefront.
func StorefrontURL(sel string) string {
	s := strings.ToLower(strings.TrimSpace(sel))
	if canon, ok := storeAliases[s]; ok {
		s = canon
	}
	if st, ok := shopperStores[s]; ok {
		return "https://" + st.Subdomain + ".shopper.com.br"
	}
	// Default to programada subdomain when store is unknown.
	return "https://programada.shopper.com.br"
}

// ResolveStore turns a user-supplied store selector into its header id pair.
// It accepts a canonical name, a known alias, or a raw numeric store id.
// The bool is false when the selector matches nothing.
func ResolveStore(sel string) (Store, bool) {
	s := strings.ToLower(strings.TrimSpace(sel))
	if s == "" {
		return Store{}, false
	}
	if canon, ok := storeAliases[s]; ok {
		s = canon
	}
	if st, ok := shopperStores[s]; ok {
		return st, true
	}
	// Raw numeric store id.
	if _, err := strconv.Atoi(s); err == nil {
		for _, st := range shopperStores {
			if st.StoreID == s {
				return st, true
			}
		}
		return Store{StoreID: s, ClusterID: "1"}, true
	}
	return Store{}, false
}

// ShopperRequiredHeaders returns the default required headers for every
// Shopper API call. The active store defaults to Programada (store 1, cluster
// 1) and can be overridden, in increasing precedence, by:
//
//   - SHOPPER_STORE      a store name/alias/id (e.g. "fresh") — resolved here
//   - SHOPPER_STORE_ID   raw x-store-id   (default "1")
//   - SHOPPER_CLUSTER_ID raw x-cluster-id (default "1")
//
// The --store global flag overrides all of the above at the request layer via
// Config.Headers (see rootFlags.newClient), since explicit flags beat env.
func ShopperRequiredHeaders() map[string]string {
	store := Store{StoreID: "1", ClusterID: "1"}
	if name := os.Getenv("SHOPPER_STORE"); name != "" {
		if resolved, ok := ResolveStore(name); ok {
			store = resolved
		}
	}
	if v := os.Getenv("SHOPPER_STORE_ID"); v != "" {
		store.StoreID = v
	}
	if v := os.Getenv("SHOPPER_CLUSTER_ID"); v != "" {
		store.ClusterID = v
	}
	return map[string]string{
		"app-os-x-version":             "web:1002",
		"x-store-id":                   store.StoreID,
		"x-cluster-id":                 store.ClusterID,
		// Cache-key sentinel: canonicalRepresentationHeaders includes headers
		// whose normalized name contains "api-version", making responses from
		// different storefronts hash to different cache files.
		"x-shopper-store-api-version":  "store=" + store.StoreID + "/cluster=" + store.ClusterID,
	}
}

// init registers the Shopper required headers as default Config.Headers so
// every client.New() call includes them without any per-command overhead.
func init() {
	shopperDefaultHeaders = ShopperRequiredHeaders()
}

// shopperDefaultHeaders holds the injected Shopper-specific headers.
var shopperDefaultHeaders map[string]string

// PatchShopperHeaders merges Shopper-required headers into c.Config.Headers.
// Called automatically by New() so existing generated commands pick them up
// without modification.
func PatchShopperHeaders(c *Client) {
	if c == nil || c.Config == nil {
		return
	}
	if c.Config.Headers == nil {
		c.Config.Headers = make(map[string]string)
	}
	for k, v := range shopperDefaultHeaders {
		if _, exists := c.Config.Headers[k]; !exists {
			c.Config.Headers[k] = v
		}
	}
}

// SetStoreHeaders forces the active store on a client's Config.Headers,
// overriding any default/env value. Used by the --store global flag.
// Also sets x-shopper-store-ver so the generated cache key (which includes
// headers containing "api-version") is store-scoped — preventing responses
// from different storefronts from colliding in the local response cache.
func SetStoreHeaders(c *Client, st Store) {
	if c == nil || c.Config == nil {
		return
	}
	if c.Config.Headers == nil {
		c.Config.Headers = make(map[string]string)
	}
	c.Config.Headers["x-store-id"] = st.StoreID
	c.Config.Headers["x-cluster-id"] = st.ClusterID
	// Cache-key disambiguation: canonicalRepresentationHeaders includes any
	// header whose normalized name contains "api-version". This sentinel makes
	// the cache key unique per storefront without affecting API routing.
	c.Config.Headers["x-shopper-store-api-version"] = "store=" + st.StoreID + "/cluster=" + st.ClusterID
}
