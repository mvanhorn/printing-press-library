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
	// methodRule labels rows classified by a config-driven rule. The value is
	// historically "regex"; the rule-based mechanism replaced the compiled regex
	// but the stored label is kept to avoid churning existing fixtures/data.
	// Change this single const to rename the label everywhere.
	methodRule      = "regex"
	methodUnmatched = "unmatched"
	methodCanonical = "canonical"
	methodManual    = "manual"
)
