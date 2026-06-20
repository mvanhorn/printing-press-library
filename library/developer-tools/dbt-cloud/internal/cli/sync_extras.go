// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written sync extras — wires DBT_CLOUD_ACCOUNT_ID into sync resource paths.
// Safe to keep across reprints.

package cli

import "github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/config"

// init registers dbt Cloud API sync paths when DBT_CLOUD_ACCOUNT_ID is set.
// The generated syncResourcePath() returns "" for all resources because
// every dbt Cloud endpoint requires {account_id} in the path; this init
// resolves those placeholders from the env var so `sync --resources runs`
// and `sync --resources jobs` work out of the box.
func init() {
	accountID := config.AccountID("")
	if accountID == "" {
		return
	}
	registerSyncPaths(accountID)
}

// syncExtraPaths is patched into the sync path lookup by registerSyncPaths.
// Exported so sync_extras_test.go can verify the registration.
var syncExtraPaths = map[string]string{}

// syncExtraParams holds default query params injected into a resource's sync
// requests, keyed by resource name. The generated sync loop applies these
// (via applySyncExtraParams) after spec-derived pagination/since params but
// before user --param/--resource-param overrides, so users can still override
// them. Used to force the dbt Cloud `runs` list endpoint to return
// newest-first; its API default is created_at ascending, which means a
// page-capped sync would otherwise store only the OLDEST runs and leave
// `runs stats` / `failures` empty for recent windows.
var syncExtraParams = map[string]map[string]string{}

// registerSyncPaths populates the extra paths map and monkey-patches
// syncResourcePath to check it before returning an error.
// We register paths here rather than touching the generated sync.go.
func registerSyncPaths(accountID string) {
	syncExtraPaths["runs"] = "/api/v2/accounts/" + accountID + "/runs/"
	syncExtraPaths["jobs"] = "/api/v2/accounts/" + accountID + "/jobs/"
	syncExtraPaths["cloud-jobs"] = "/api/v2/accounts/" + accountID + "/jobs/"

	// Force newest-first ordering for runs. The dbt Cloud Admin API v2
	// /runs/ endpoint defaults to created_at ASCENDING, so the first pages
	// returned are the OLDEST runs. With page caps (dogfood, --max-pages,
	// or simply not paginating to the very end) the local store would only
	// hold ancient runs and recent-window queries return empty. order_by
	// accepts a field name with a leading "-" for descending.
	syncExtraParams["runs"] = map[string]string{"order_by": "-created_at"}
}

// applySyncExtraParams merges the hand-registered default query params for a
// resource into the request params map. Called by the generated sync loop
// (sync.go) before user-supplied --param/--resource-param overrides so users
// retain the final say. No-op for resources without registered defaults.
func applySyncExtraParams(resource string, params map[string]string) {
	for k, v := range syncExtraParams[resource] {
		params[k] = v
	}
}
