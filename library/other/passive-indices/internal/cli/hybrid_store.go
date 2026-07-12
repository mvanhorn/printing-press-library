// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/store"
)

// resourceType* name the generic resources table's resource_type values
// used by hand-written commands to cache live-fetched data for offline
// reads and cross-source joins (constituents-diff, index-fund joins, etc.).
const (
	resourceTypeIndexQuote           = "index_quote"
	resourceTypeIndexConstituentSnap = "index_constituent_snapshot"
	resourceTypeIndexHistory         = "index_history"
	resourceTypeIndexTRI             = "index_tri"
	resourceTypeIndexValuation       = "index_valuation"
	resourceTypeFund                 = "fund"
	resourceTypeFundNFO              = "fund_nfo"
)

// openStoreForCache opens the local store at the resolved default path so
// hand-written commands can cache live-fetched data. Returns an error the
// caller should treat as non-fatal (cache-miss is fine; the live result is
// still returned to stdout).
func openStoreForCache(flags *rootFlags) (*store.Store, string, error) {
	dbPath := defaultDBPath("passive-indices-pp-cli")
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, dbPath, err
	}
	return db, dbPath, nil
}
