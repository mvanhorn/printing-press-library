package cli

import (
	"encoding/json"
	"io"
)

// Planner reports are already curated and carry evidence, eligibility, and
// provenance fields at every depth. The generic identity-only compact filter
// must not drop values such as checks[].passed or groups[].observed_mean.
// Explicit --select and the command's render limit remain available to narrow
// output deliberately, without changing the shared root flags.
func printPlannerEvidenceOutput(w io.Writer, data json.RawMessage, flags *rootFlags, meta map[string]any, fields ...map[string]bool) error {
	outputFlags := *flags
	outputFlags.compact = false
	return printOutputWithFlagsMeta(w, data, &outputFlags, meta, fields...)
}
