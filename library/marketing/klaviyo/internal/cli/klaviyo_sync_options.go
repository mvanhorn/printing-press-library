// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

type syncRequiredFilter struct {
	Field, ParentResource string
	Values                []string
}

type syncResourceDescriptor struct {
	Path                                                 string
	MaxPageSize                                          int
	IncrementalParam, IncrementalFilter, IncrementalSort string
	RequiredFilters                                      []syncRequiredFilter
}

func syncDescriptorForResource(resource string) (syncResourceDescriptor, bool) {
	path, err := syncResourcePath(resource)
	desc := syncResourceDescriptor{Path: path, MaxPageSize: determinePaginationDefaults(resource).limit}
	switch resource {
	case "events":
		desc.IncrementalFilter = "greater-or-equal(datetime,%s)"
	case "profiles", "lists", "segments":
		desc.IncrementalFilter = "greater-than(updated,%s)"
	case "campaigns", "forms", "images":
		desc.IncrementalFilter = "greater-or-equal(updated_at,%s)"
	case "flows", "campaign-messages":
		desc.IncrementalFilter = "greater-or-equal(updated,%s)"
		desc.IncrementalSort = "updated"
	case "templates", "template-universal-content", "web-feeds":
		desc.IncrementalFilter = "greater-or-equal(updated,%s)"
	case "reviews":
		desc.IncrementalFilter = "greater-or-equal(created,%s)"
	}
	if desc.IncrementalFilter != "" {
		desc.IncrementalParam = "filter"
	}
	switch resource {
	case "coupon-codes":
		desc.RequiredFilters = []syncRequiredFilter{{Field: "coupon.id", ParentResource: "coupons"}}
	}
	return desc, err == nil
}

func syncSinceFilter(desc syncResourceDescriptor, sinceTS, filter string) string {
	if sinceTS == "" || desc.IncrementalParam == "" {
		return filter
	}
	incremental := fmt.Sprintf(desc.IncrementalFilter, sinceTS)
	if filter != "" {
		return "and(" + filter + "," + incremental + ")"
	}
	return incremental
}

// The enclosing verified store proves ownership of checkpoints predating scope keys.
func adoptKlaviyoSyncCheckpoint(db *store.Store, resource, scope string, write bool) (bool, error) {
	if !write {
		var found bool
		err := db.DB().QueryRow(`SELECT EXISTS(SELECT 1 FROM sync_state WHERE resource_type = ? AND COALESCE(scope_key, '') = '' AND COALESCE(last_cursor, '') != '')`, resource).Scan(&found)
		return found, err
	}
	result, err := db.DB().Exec(`UPDATE sync_state SET scope_key = ? WHERE resource_type = ? AND COALESCE(scope_key, '') = '' AND COALESCE(last_cursor, '') != ''`, scope, resource)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
