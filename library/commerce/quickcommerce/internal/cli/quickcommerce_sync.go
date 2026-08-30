// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// applyQuickCommerceSyncDefaults supplies the documented safe sample location
// for the default manual sync. Users can override every value with --param.
func applyQuickCommerceSyncDefaults(resource string, params map[string]string) {
	if resource != "products" {
		return
	}
	if params["q"] == "" {
		params["q"] = "milk"
	}
	if params["lat"] == "" {
		params["lat"] = "12.9021"
	}
	if params["lon"] == "" {
		params["lon"] = "77.6639"
	}
	if params["platform"] == "" {
		params["platform"] = "BlinkIt"
	}
}
