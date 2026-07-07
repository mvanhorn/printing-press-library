// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored extension (NOT generated). Durable across regen-merge.
//
// Mirror of internal/store/luma_id_overrides.go for the cli-package extractID
// path (single-object upserts and sync-failure detection). Luma identifies every
// resource by `api_id`, which the generic fallback list omits.

package cli

func init() {
	for _, resource := range []string{"events", "categories", "calendars", "discover"} {
		if resourceIDFieldOverrides[resource] == "" {
			resourceIDFieldOverrides[resource] = "api_id"
		}
	}
}
