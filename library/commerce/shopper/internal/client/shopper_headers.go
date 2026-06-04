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

package client

import (
	"os"
	"strconv"
	"strings"
)

// Store identifies one Shopper storefront by its API store/cluster id pair.
// Values come from GET /features/stores (the `number` and `cluster_id` fields).
type Store struct {
	StoreID   string
	ClusterID string
}

// shopperStores maps the human-facing store name (the storefront subdomain) to
// its API id pair. Keep in sync with GET /features/stores.
var shopperStores = map[string]Store{
	"programada": {StoreID: "1", ClusterID: "1"}, // Compra Programada (mensal)
	"fresh":      {StoreID: "2", ClusterID: "1"}, // Programada Fresh
	"unica":      {StoreID: "3", ClusterID: "1"}, // Compra Única (pontual)
	"pet":        {StoreID: "5", ClusterID: "3"}, // Pet.Shopper
}

// storeAliases lets callers use the friendlier label the app shows.
var storeAliases = map[string]string{
	"mensal":  "programada",
	"monthly": "programada",
	"pontual": "unica",
	"única":   "unica",
}

// StoreNames returns the canonical selectable store names (for help text and
// flag validation) in display order.
func StoreNames() []string {
	return []string{"programada", "fresh", "unica", "pet"}
}

// SpendStoreNames returns every storefront to report on by default, grouped
// like the Shopper store picker: the three recurring stores (programada,
// fresh, pet) followed by the one-off store (unica). Reporting all four keeps
// the tool complete for any account, not just the two grocery stores.
func SpendStoreNames() []string {
	return []string{"programada", "fresh", "pet", "unica"}
}

// ResolveStore turns a user-supplied store selector into its header id pair.
// It accepts a canonical name (programada/fresh/unica/pet), a known alias
// (mensal, pontual, ...), or a raw numeric store id. The bool is false when the
// selector matches nothing, so callers can surface a clear error instead of
// silently querying the wrong store.
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
	// Raw numeric store id: keep the matching cluster when we know it,
	// otherwise default cluster 1 (every storefront except pet is cluster 1).
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
		"app-os-x-version": "web:1002",
		"x-store-id":       store.StoreID,
		"x-cluster-id":     store.ClusterID,
	}
}

// init registers the Shopper required headers as default Config.Headers so
// every client.New() call includes them without any per-command overhead.
// The init hook runs exactly once per process and merges into Config.Headers
// after config.Load(), so explicit env/config overrides still win.
func init() {
	// Inject defaults via the global default-headers hook.
	// We store in a package-level variable so New() can merge them.
	shopperDefaultHeaders = ShopperRequiredHeaders()
}

// shopperDefaultHeaders holds the injected Shopper-specific headers.
// Merged into every new Client by patchShopperHeaders().
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
		// Don't override headers the user set explicitly in config.
		if _, exists := c.Config.Headers[k]; !exists {
			c.Config.Headers[k] = v
		}
	}
}

// SetStoreHeaders forces the active store on a client's Config.Headers,
// overriding any default/env value. Used by the --store global flag so an
// explicit per-invocation selection always wins. Safe to call before or after
// PatchShopperHeaders.
func SetStoreHeaders(c *Client, st Store) {
	if c == nil || c.Config == nil {
		return
	}
	if c.Config.Headers == nil {
		c.Config.Headers = make(map[string]string)
	}
	c.Config.Headers["x-store-id"] = st.StoreID
	c.Config.Headers["x-cluster-id"] = st.ClusterID
}
