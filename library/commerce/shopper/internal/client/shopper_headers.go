// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: adds the required Shopper API headers to every authenticated request.
// These headers are mandatory — the real siteapi.shopper.com.br returns 401/empty
// without app-os-x-version, x-store-id, and x-cluster-id.

package client

import (
	"os"
)

// ShopperRequiredHeaders returns the default required headers for every
// Shopper API call. Values come from environment variables with sensible
// defaults so the CLI works out-of-the-box for most users.
//
//   - SHOPPER_STORE_ID   (default "1")
//   - SHOPPER_CLUSTER_ID (default "1")
func ShopperRequiredHeaders() map[string]string {
	storeID := os.Getenv("SHOPPER_STORE_ID")
	if storeID == "" {
		storeID = "1"
	}
	clusterID := os.Getenv("SHOPPER_CLUSTER_ID")
	if clusterID == "" {
		clusterID = "1"
	}
	return map[string]string{
		"app-os-x-version": "web:1002",
		"x-store-id":       storeID,
		"x-cluster-id":     clusterID,
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
