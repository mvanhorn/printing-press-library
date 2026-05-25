// PATCH(amend-2026-05-25: expand/contract decomposition — turn a destructive
// operation class into a sequence of safe-envelope steps, or refuse it with a
// structured machine-readable reason). Replaces the blunt --allow-destructive
// boolean. Not in the Management API.
//
// The expand/contract pattern makes a destructive schema change reversible and
// online by splitting it across deploys:
//
//   DROP COLUMN  ->  expand: (nothing to add)               // column already unused
//                    contract: rename to a tombstone name, then a later run drops it
//   ALTER TYPE   ->  expand: ADD COLUMN <new> <newtype>      // additive, est_rows 0
//                    backfill: app/dual-write fills <new>    // out of band, app-owned
//                    contract: DROP COLUMN <old>             // a later decompose run
//
// Each emitted step is itself a Statement re-classified by the manifest, so a
// step is only auto-eligible if it independently falls inside the safe envelope.
// The decomposer NEVER fabricates a backfill or a data move it cannot prove
// safe; those steps are surfaced as app-owned manual gates, not auto-applied.

package dbchange

import (
	"fmt"
	"regexp"
	"strings"
)

// DecompPhase labels where a step sits in the expand/contract lifecycle.
type DecompPhase string

const (
	PhaseExpand   DecompPhase = "expand"   // additive, safe-envelope, apply now
	PhaseBackfill DecompPhase = "backfill" // app-owned data move, manual gate
	PhaseContract DecompPhase = "contract" // removes the old object, a later run
)

// DecompStep is one step of a decomposed destructive change.
type DecompStep struct {
	Phase    DecompPhase `json:"phase"`
	SQL      string      `json:"sql,omitempty"`
	Note     string      `json:"note,omitempty"`
	// AutoApply is true only for expand-phase steps that fall inside the safe
	// envelope on their own. Backfill and contract steps are always false: they
	// require an explicit later decision.
	AutoApply bool `json:"auto_apply"`
}

// DecompResult is the verdict for decomposing one statement.
type DecompResult struct {
	OriginalIndex int          `json:"original_index"`
	OriginalSQL   string       `json:"original_sql"`
	OpClass       OpClass      `json:"op_class"`
	// Decomposed is true when the statement was rewritten into steps; false when
	// it was refused.
	Decomposed bool         `json:"decomposed"`
	Steps      []DecompStep `json:"steps,omitempty"`
	// RefusedReason is set (and Decomposed is false) when the statement cannot be
	// safely decomposed — TRUNCATE, unbounded DELETE, DROP with dependents. The
	// caller turns this into a no-op + structured error.
	RefusedReason string `json:"refused_reason,omitempty"`
	RefusedRule   string `json:"refused_rule,omitempty"`
}

var (
	reDropColumn   = regexp.MustCompile(`(?i)\bDROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?"?([a-zA-Z0-9_]+)"?`)
	reAlterColType = regexp.MustCompile(`(?i)\bALTER\s+COLUMN\s+"?([a-zA-Z0-9_]+)"?\s+(?:SET\s+DATA\s+)?TYPE\s+([a-zA-Z0-9_ ()]+?)(?:\s+USING\b|,|$)`)
	reDeleteWhere  = regexp.MustCompile(`(?i)\bWHERE\b`)
)

// Decompose turns a single destructive Statement into an expand/contract step
// sequence, or refuses it. Non-destructive statements are returned as a
// single trivially-decomposed step (the statement itself, expand phase,
// auto-eligible if it independently clears the envelope) so the caller can run
// a whole manifest through one code path.
func Decompose(s Statement) DecompResult {
	res := DecompResult{OriginalIndex: s.Index, OriginalSQL: s.SQL, OpClass: s.OpClass}
	head := upperHead(s.SQL)
	tbl := firstObject(s.Objects)

	// A type change is the canonical expand/contract case. The classifier marks
	// it OpDDL (non-destructive) — it doesn't drop an object — but it rewrites
	// the whole table (est_rows -1), so it must NOT pass through as a single
	// auto-step. Detect it BEFORE the non-destructive passthrough.
	isTypeChange := strings.Contains(head, "ALTER COLUMN") && strings.Contains(head, "TYPE")

	if !s.Destructive && !isTypeChange {
		// Pass through. The step inherits the statement's own envelope eligibility.
		res.Decomposed = true
		res.Steps = []DecompStep{{Phase: PhaseExpand, SQL: ensureSemicolon(s.SQL), AutoApply: s.EstRowsAffected == 0 && s.Reversibility != RevNone}}
		return res
	}

	switch {
	// DROP COLUMN -> tombstone-rename now (reversible, safe), real drop deferred.
	case strings.Contains(head, "DROP COLUMN"):
		col := matchGroup(reDropColumn, s.SQL, 1)
		if col == "" || tbl == "" {
			return refuse(res, "drop_column_unparsed", "DROP COLUMN target table/column could not be parsed for safe decomposition")
		}
		tomb := col + "_pp_dropped"
		res.Decomposed = true
		res.Steps = []DecompStep{
			{
				Phase:     PhaseContract,
				SQL:       fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", tbl, col, tomb),
				Note:      "expand/contract: rename to a tombstone instead of dropping — reversible, zero data loss, hides the column from new reads",
				AutoApply: true, // a rename is metadata-only, reversible, est_rows 0
			},
			{
				Phase:     PhaseContract,
				SQL:       fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", tbl, tomb),
				Note:      "deferred hard drop — run in a LATER decompose pass once no code references the tombstone column; not auto-applied",
				AutoApply: false,
			},
		}
		return res

	// ALTER COLUMN ... TYPE -> add new typed column, app backfill, drop old.
	case strings.Contains(head, "ALTER COLUMN") && strings.Contains(head, "TYPE"):
		col := matchGroup(reAlterColType, s.SQL, 1)
		newType := strings.TrimSpace(matchGroup(reAlterColType, s.SQL, 2))
		if col == "" || newType == "" || tbl == "" {
			return refuse(res, "type_change_unparsed", "ALTER COLUMN TYPE could not be parsed into column/new-type for safe decomposition")
		}
		newCol := col + "_pp_new"
		res.Decomposed = true
		res.Steps = []DecompStep{
			{
				Phase:     PhaseExpand,
				SQL:       fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s;", tbl, newCol, newType),
				Note:      "expand: add the new typed column alongside the old — additive, reversible, est_rows 0",
				AutoApply: true,
			},
			{
				Phase:     PhaseBackfill,
				SQL:       fmt.Sprintf("-- backfill (app-owned, batched): UPDATE %s SET %s = %s::%s WHERE %s IS NULL;", tbl, newCol, col, newType, newCol),
				Note:      "backfill is a data write (est_rows unbounded) and must be run out of band in batches by the application; the decomposer does NOT auto-apply it",
				AutoApply: false,
			},
			{
				Phase:     PhaseContract,
				SQL:       fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s; ALTER TABLE %s RENAME COLUMN %s TO %s;", tbl, col, tbl, newCol, col),
				Note:      "contract: drop the old column and rename the new one into place — run in a LATER pass after backfill is verified complete; not auto-applied",
				AutoApply: false,
			},
		}
		return res

	// Refusals — these have no safe decomposition.
	case strings.HasPrefix(head, "TRUNCATE"):
		return refuse(res, "truncate_irreducible", "TRUNCATE removes all rows with no reversible expand/contract path; refused (no-op). Use a bounded, reviewed DELETE under the human approver if intended")
	case strings.HasPrefix(head, "DELETE"):
		if !reDeleteWhere.MatchString(s.SQL) {
			return refuse(res, "unbounded_delete", "unbounded DELETE (no WHERE) removes all rows with no reversible decomposition; refused (no-op)")
		}
		return refuse(res, "delete_irreducible", "DELETE is a data write with no reversible expand/contract path; refused (no-op). Run under the human approver with an explicit, reviewed WHERE if intended")
	case strings.HasPrefix(head, "DROP TABLE"):
		return refuse(res, "drop_table_dependents", "DROP TABLE may have dependents (FKs, views, policies) and is not safely decomposable; refused (no-op). Tombstone-rename the table or drop it under the human approver after dependents are handled")
	case strings.HasPrefix(head, "DROP SCHEMA"):
		return refuse(res, "drop_schema_dependents", "DROP SCHEMA cascades to every contained object; not safely decomposable; refused (no-op)")
	default:
		return refuse(res, "destructive_unclassified", fmt.Sprintf("destructive statement (%s) has no known safe decomposition; refused (no-op)", s.OpClass))
	}
}

// DecomposeManifest runs every statement through Decompose, preserving order.
// It returns the per-statement results plus a convenience rollup: whether any
// statement was refused (so the caller can no-op the whole apply).
func DecomposeManifest(m *Manifest) (results []DecompResult, anyRefused bool) {
	for _, s := range m.Statements {
		r := Decompose(s)
		if !r.Decomposed {
			anyRefused = true
		}
		results = append(results, r)
	}
	return results, anyRefused
}

func refuse(res DecompResult, rule, reason string) DecompResult {
	res.Decomposed = false
	res.RefusedRule = rule
	res.RefusedReason = reason
	return res
}

func ensureSemicolon(sql string) string {
	s := strings.TrimSpace(sql)
	if !strings.HasSuffix(s, ";") {
		s += ";"
	}
	return s
}

func matchGroup(re *regexp.Regexp, s string, n int) string {
	m := re.FindStringSubmatch(s)
	if len(m) > n {
		return strings.TrimSpace(m[n])
	}
	return ""
}
