// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written dbt Cloud config accessors — safe to keep across reprints.

package config

import "os"

// AccountID returns the dbt Cloud account ID.
// Preference order: explicit argument (callers pass "" when not set) → DBT_CLOUD_ACCOUNT_ID env var.
// Returns "" when neither is set so callers can emit a helpful error.
func AccountID(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return os.Getenv("DBT_CLOUD_ACCOUNT_ID")
}

// init applies DBT_CLOUD_HOST as a fallback for DBT_CLOUD_BASE_URL.
// dbt Cloud documentation uses DBT_CLOUD_HOST; the generated config loader
// already supports DBT_CLOUD_BASE_URL. This init ensures both names work so
// users who follow dbt Cloud's own docs don't have to rename their env var.
func init() {
	if os.Getenv("DBT_CLOUD_BASE_URL") == "" {
		if h := os.Getenv("DBT_CLOUD_HOST"); h != "" {
			os.Setenv("DBT_CLOUD_BASE_URL", h)
		}
	}
}
