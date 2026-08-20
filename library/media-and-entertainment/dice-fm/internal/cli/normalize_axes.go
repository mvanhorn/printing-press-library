// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// Axis keys for the ticket-type attributes overlay. Shared so a rename is caught
// by the compiler instead of silently diverging across call sites.
const (
	axisAccessClass     = "access_class"
	axisSalesStage      = "sales_stage"
	axisEntryWindowType = "entry_window_type"
	axisEntryWindowTime = "entry_window_time"
	axisGroupSize       = "group_size"
	axisCompFlag        = "comp_flag"
)

// Axis keys for the venue attributes overlay. These name the two columns of the
// venue_attributes table so config-driven rules can target them by key.
const (
	axisComplex = "complex"
	axisRoom    = "room"
)

// Crosswalk/attribute method labels. These are categorical strings stamped onto
// crosswalk and typed-attribute rows to record how a row was classified.
const (
	// methodRule labels rows classified by a config-driven rule. The Go-side
	// name is methodRule (the accurate name for the rule-engine mechanism), but
	// the STORED value is deliberately kept as "regex" for backward
	// compatibility: that string is already persisted in every existing local
	// store and is asserted by the analytics test fixtures, so changing it now
	// would be a data migration, not a rename. The code never compares the raw
	// literal — every read/write routes through this const — so the eventual
	// value change to "rule" is a one-line edit here plus a store migration
	// (rewrite existing method='regex' rows) whenever a schema-version bump
	// lands. Until then, anyone reading method values in SQL should treat
	// "regex" as "config-driven rule".
	methodRule      = "regex"
	methodUnmatched = "unmatched"
	methodCanonical = "canonical"
	methodManual    = "manual"
)
