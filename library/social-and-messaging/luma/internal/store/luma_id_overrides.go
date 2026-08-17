// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored extension (NOT generated). Durable across regen-merge.
//
// Luma uses `api_id` as its universal identifier convention (evt-..., cat-...,
// cal-..., discplace-...). The generator's generic ID-fallback list does not
// include `api_id`, so without this override every synced row fails ID
// extraction and the local mirror stays empty ("all_items_failed_id_extraction").
// Populating the store-package override map from init() is the separate-file
// extension pattern the Printing Press recommends over editing generated code.

package store

func init() {
	for _, resource := range []string{"events", "categories", "calendars", "discover"} {
		if resourceIDFieldOverrides[resource] == "" {
			resourceIDFieldOverrides[resource] = "api_id"
		}
	}
}
