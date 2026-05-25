// PATCH: novel safe-mutation lifecycle — typed change manifest powering db plan/apply/receipt/revert. Not in the Management API.
//
// Package dbchange parses a SQL migration into a typed, inspectable change
// manifest. The manifest is the legibility primitive that lets a human (or a
// safety classifier) reason about a mutation by operation class and
// reversibility BEFORE it touches production, and lets `db apply` refuse to run
// anything whose recomputed hash drifted from the approved one.
package dbchange

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// OpClass is the safety-relevant classification of a single SQL statement.
// A safety classifier gates by this field: additive ddl/rls/grant can be
// allowed while data-write/destructive are gated.
type OpClass string

const (
	OpDDL         OpClass = "ddl"         // CREATE/ALTER/DROP of schema objects (non-destructive subset)
	OpGrant       OpClass = "grant"       // GRANT/REVOKE privilege changes
	OpRLS         OpClass = "rls"         // CREATE/ALTER/DROP POLICY, ENABLE/DISABLE ROW LEVEL SECURITY
	OpDataWrite   OpClass = "data-write"  // INSERT/UPDATE (non-destructive row writes)
	OpDestructive OpClass = "destructive" // DROP TABLE, TRUNCATE, DELETE, DROP COLUMN, DROP SCHEMA
	OpOther       OpClass = "other"       // SET, COMMENT, anything unclassified
)

// Reversibility is the manifest's verdict on whether a statement can be undone.
type Reversibility string

const (
	// RevAuto: the mechanical inverse is derivable from the statement text
	// alone (CREATE -> DROP, ADD COLUMN -> DROP COLUMN, REVOKE -> GRANT).
	RevAuto Reversibility = "auto"
	// RevSnapshot: reversible only by first snapshotting the prior object
	// definition (CREATE OR REPLACE FUNCTION, ALTER POLICY, DROP of an object
	// whose definition must be captured to recreate).
	RevSnapshot Reversibility = "snapshot"
	// RevNone: no reliable inverse (DELETE/TRUNCATE/UPDATE of data rows).
	RevNone Reversibility = "none"
)

// Statement is one parsed SQL statement plus its safety classification.
type Statement struct {
	Index         int           `json:"index"`
	SQL           string        `json:"sql"`
	OpClass       OpClass       `json:"op_class"`
	Objects       []string      `json:"objects"`
	EstRowsNote   string        `json:"estimated_rows_note,omitempty"`
	// EstRowsAffected is the structured, machine-evaluable row-impact verdict the
	// policy approver gates on. PATCH(amend-2026-05-25: structured est-rows for
	// the policy gate) — the prior manifest only carried EstRowsNote (a human
	// string), which a policy cannot reason about. Convention:
	//   0  = no existing data rows are read or rewritten (pure additive DDL,
	//        grant, RLS policy add/rewrite, ADD COLUMN, CREATE INDEX). Backfilling
	//        a NOT NULL default does NOT count as 0 — see ADD COLUMN classify.
	//   -1 = unknown or unbounded row impact (INSERT/UPDATE/DELETE/TRUNCATE, or an
	//        ALTER that rewrites the table). The policy treats -1 as a hard fail
	//        for the safe-default envelope; it never auto-applies an unknown.
	// A statement is auto-eligible only when EstRowsAffected == 0.
	EstRowsAffected int           `json:"est_rows_affected"`
	Reversibility Reversibility `json:"reversibility"`
	Inverse       string        `json:"inverse_sql,omitempty"` // mechanical inverse when RevAuto
	Destructive   bool          `json:"destructive"`
}

// Manifest is the typed plan emitted by `db plan` and consumed by `db apply`.
type Manifest struct {
	PlanHash      string      `json:"plan_hash"`
	StatementN    int         `json:"statement_count"`
	Statements    []Statement `json:"statements"`
	HasDestructive bool       `json:"has_destructive"`
	HasIrreversible bool      `json:"has_irreversible"`
	// MaxEstRowsAffected is the manifest-level rollup the policy approver reads:
	// the maximum per-statement EstRowsAffected, with -1 (unknown) dominating any
	// concrete count. 0 means every statement touches zero existing data rows.
	// PATCH(amend-2026-05-25: manifest-level est-rows rollup for the policy gate).
	MaxEstRowsAffected int     `json:"max_est_rows_affected"`
	SourceLabel   string      `json:"source,omitempty"` // file path or "--sql"
}

var (
	reStmtSplit  = regexp.MustCompile(`;\s*(?:\n|$)`)
	reLineComment = regexp.MustCompile(`(?m)--.*$`)
	reBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reWS         = regexp.MustCompile(`\s+`)
)

// Parse turns a SQL blob into a typed Manifest with a deterministic plan_hash.
// The hash is computed over the normalized (comment-stripped, whitespace-
// collapsed, lowercased-keywords-preserved) statement list so cosmetic edits
// that don't change semantics keep the same hash, while any real SQL change
// drifts it — which `db apply --approved` rejects.
func Parse(sql, sourceLabel string) *Manifest {
	clean := stripComments(sql)
	raw := splitStatements(clean)

	m := &Manifest{SourceLabel: sourceLabel}
	var normParts []string
	for i, s := range raw {
		st := classify(i, s)
		m.Statements = append(m.Statements, st)
		if st.Destructive {
			m.HasDestructive = true
		}
		if st.Reversibility == RevNone {
			m.HasIrreversible = true
		}
		// PATCH(amend-2026-05-25: roll up est-rows so the policy gate sees the
		// worst case). -1 (unknown/unbounded) dominates any concrete count, so
		// once any statement is -1 the rollup is -1 for the whole manifest.
		if st.EstRowsAffected < 0 || m.MaxEstRowsAffected < 0 {
			m.MaxEstRowsAffected = -1
		} else if st.EstRowsAffected > m.MaxEstRowsAffected {
			m.MaxEstRowsAffected = st.EstRowsAffected
		}
		normParts = append(normParts, normalize(s))
	}
	m.StatementN = len(m.Statements)
	m.PlanHash = hashStatements(normParts)
	return m
}

// stripComments removes -- line comments and /* */ block comments.
func stripComments(sql string) string {
	sql = reBlockComment.ReplaceAllString(sql, "")
	sql = reLineComment.ReplaceAllString(sql, "")
	return sql
}

// splitStatements splits on semicolons that terminate a statement. This is a
// pragmatic splitter, not a full SQL parser; it handles the migration shapes
// the lifecycle targets. Dollar-quoted bodies are preserved by tracking the
// active dollar-quote tag and refusing to split until the matching closing
// tag is seen. PATCH(amend-2026-05-25: track arbitrary dollar-quote tags) —
// the prior splitter only matched bare `$$`, so a `DO $ttl$ ... $ttl$` block
// (migration 008's TTL cleanup) was split on the `;` inside its body,
// corrupting the manifest. Postgres dollar-quote tags are `$[tag]$` where the
// optional tag is an identifier; an opening tag is closed only by an identical
// tag, and tags do not nest. See spec build-correctness note #1.
func splitStatements(sql string) []string {
	var out []string
	var buf strings.Builder
	runes := []rune(sql)
	dollarTag := "" // non-empty while inside a dollar-quoted body; holds the opening tag, e.g. "$$" or "$ttl$".
	for i := 0; i < len(runes); i++ {
		if runes[i] == '$' {
			if tag, n := scanDollarTag(runes, i); tag != "" {
				if dollarTag == "" {
					// Opening a dollar-quoted body.
					dollarTag = tag
				} else if dollarTag == tag {
					// Closing the active body (tags must match exactly; they don't nest).
					dollarTag = ""
				}
				// Whether opening, closing, or an inner non-matching tag, copy
				// the whole tag verbatim and advance past it.
				for j := i; j < i+n; j++ {
					buf.WriteRune(runes[j])
				}
				i += n - 1
				continue
			}
		}
		if runes[i] == ';' && dollarTag == "" {
			s := strings.TrimSpace(buf.String())
			if s != "" {
				out = append(out, s)
			}
			buf.Reset()
			continue
		}
		buf.WriteRune(runes[i])
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}

// scanDollarTag reports whether the runes starting at i form a Postgres
// dollar-quote tag (`$$` or `$identifier$`). It returns the tag text and its
// length in runes, or ("", 0) when i is not the start of a valid tag (e.g. a
// `$1` positional parameter or a stray `$`). The tag's inner text must be a
// valid identifier: it starts with a letter or underscore and continues with
// letters, digits, or underscores. PATCH(amend-2026-05-25: dollar-tag scanner).
func scanDollarTag(runes []rune, i int) (string, int) {
	if runes[i] != '$' {
		return "", 0
	}
	// Bare `$$`.
	if i+1 < len(runes) && runes[i+1] == '$' {
		return "$$", 2
	}
	// `$identifier$`.
	j := i + 1
	for j < len(runes) {
		r := runes[j]
		if r == '$' {
			// A `$identifier$` requires at least one inner rune.
			if j == i+1 {
				return "", 0
			}
			return string(runes[i : j+1]), j + 1 - i
		}
		isIdentStart := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isIdentCont := isIdentStart || (r >= '0' && r <= '9')
		if j == i+1 {
			if !isIdentStart {
				return "", 0 // e.g. `$1` positional param — not a dollar-quote tag.
			}
		} else if !isIdentCont {
			return "", 0
		}
		j++
	}
	return "", 0
}

func normalize(s string) string {
	return reWS.ReplaceAllString(strings.TrimSpace(s), " ")
}

func hashStatements(parts []string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, ";\n")))
	return "sha256:" + hex.EncodeToString(h[:])
}

// upperHead returns the first n leading tokens of a statement, uppercased, for
// keyword matching.
func upperHead(s string) string {
	return strings.ToUpper(normalize(s))
}

var reIdentAfter = func(kw string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b` + kw + `\b\s+(?:if\s+(?:not\s+)?exists\s+)?(?:only\s+)?["]?([a-zA-Z0-9_."]+)`)
}

// classify assigns the operation class, objects, reversibility, and mechanical
// inverse for a single statement.
func classify(idx int, sql string) Statement {
	st := Statement{Index: idx, SQL: strings.TrimSpace(sql)}
	head := upperHead(sql)

	switch {
	// Destructive first — these dominate the safety verdict.
	case strings.HasPrefix(head, "DROP TABLE"):
		st.OpClass, st.Destructive, st.Reversibility = OpDestructive, true, RevSnapshot
		st.Objects = extractObjects(sql, "TABLE")
	case strings.HasPrefix(head, "DROP SCHEMA"):
		st.OpClass, st.Destructive, st.Reversibility = OpDestructive, true, RevSnapshot
		st.Objects = extractObjects(sql, "SCHEMA")
	case strings.HasPrefix(head, "TRUNCATE"):
		st.OpClass, st.Destructive, st.Reversibility = OpDestructive, true, RevNone
		st.Objects = extractObjects(sql, "TRUNCATE")
		st.EstRowsNote = "all rows in target table(s)"
		st.EstRowsAffected = -1 // PATCH(amend-2026-05-25): unbounded row impact
	case strings.HasPrefix(head, "DELETE FROM"), strings.HasPrefix(head, "DELETE "):
		st.OpClass, st.Destructive, st.Reversibility = OpDestructive, true, RevNone
		st.Objects = extractObjects(sql, "FROM")
		st.EstRowsNote = rowEstimate(head)
		st.EstRowsAffected = -1 // PATCH(amend-2026-05-25): bounded or not, row count is unknown at plan time
	case strings.Contains(head, "DROP COLUMN"):
		st.OpClass, st.Destructive, st.Reversibility = OpDestructive, true, RevSnapshot
		st.Objects = extractObjects(sql, "TABLE")
	// RLS
	case strings.Contains(head, "POLICY"):
		st.OpClass = OpRLS
		// The safety-relevant object on a policy is the table it guards
		// (the ON clause), not the policy's own name.
		st.Objects = extractObjects(sql, "ON")
		if strings.HasPrefix(head, "CREATE POLICY") {
			st.Reversibility, st.Inverse = RevAuto, inverseCreatePolicy(sql)
		} else if strings.HasPrefix(head, "DROP POLICY") {
			st.Reversibility = RevSnapshot
		} else {
			st.Reversibility = RevSnapshot // ALTER POLICY needs prior body
		}
	case strings.Contains(head, "ROW LEVEL SECURITY"):
		st.OpClass = OpRLS
		st.Objects = extractObjects(sql, "TABLE")
		if strings.Contains(head, "ENABLE ROW LEVEL SECURITY") {
			st.Reversibility, st.Inverse = RevAuto, swapEnableDisableRLS(sql, true)
		} else {
			st.Reversibility, st.Inverse = RevAuto, swapEnableDisableRLS(sql, false)
		}
	// Grants
	case strings.HasPrefix(head, "GRANT "):
		st.OpClass, st.Reversibility = OpGrant, RevAuto
		st.Objects = grantTargets(sql)
		st.Inverse = grantToRevoke(sql, true)
	case strings.HasPrefix(head, "REVOKE "):
		st.OpClass, st.Reversibility = OpGrant, RevAuto
		st.Objects = grantTargets(sql)
		st.Inverse = grantToRevoke(sql, false)
	// DDL additive
	case strings.HasPrefix(head, "CREATE TABLE"):
		st.OpClass, st.Reversibility = OpDDL, RevAuto
		st.Objects = extractObjects(sql, "TABLE")
		st.Inverse = "DROP TABLE IF EXISTS " + firstObject(st.Objects) + ";"
	case strings.Contains(head, "ADD COLUMN"):
		st.OpClass, st.Reversibility = OpDDL, RevAuto
		st.Objects = extractObjects(sql, "TABLE")
		if col := addColumnName(sql); col != "" {
			st.Inverse = "ALTER TABLE " + firstObject(st.Objects) + " DROP COLUMN IF EXISTS " + col + ";"
		} else {
			st.Reversibility = RevSnapshot
		}
		// PATCH(amend-2026-05-25: est-rows for ADD COLUMN). A nullable column or
		// one with a constant default is metadata-only on modern Postgres (no
		// table rewrite, est_rows 0 — stays in the safe envelope). A column with
		// a VOLATILE default expression forces a full-table rewrite, so its row
		// impact is unbounded and it must NOT auto-apply.
		if addColumnHasVolatileDefault(sql) {
			st.EstRowsAffected = -1
			st.EstRowsNote = "volatile-default ADD COLUMN rewrites every existing row"
		}
	case strings.HasPrefix(head, "CREATE OR REPLACE FUNCTION"), strings.HasPrefix(head, "CREATE FUNCTION"):
		st.OpClass = OpDDL
		st.Objects = extractObjects(sql, "FUNCTION")
		if strings.HasPrefix(head, "CREATE OR REPLACE") {
			st.Reversibility = RevSnapshot // need prior body to restore
		} else {
			st.Reversibility = RevAuto
			st.Inverse = "DROP FUNCTION IF EXISTS " + firstObject(st.Objects) + ";"
		}
	case strings.HasPrefix(head, "CREATE INDEX"), strings.HasPrefix(head, "CREATE UNIQUE INDEX"):
		st.OpClass, st.Reversibility = OpDDL, RevAuto
		st.Objects = extractObjects(sql, "INDEX")
		st.Inverse = "DROP INDEX IF EXISTS " + firstObject(st.Objects) + ";"
	case strings.HasPrefix(head, "ALTER TABLE"):
		st.OpClass, st.Reversibility = OpDDL, RevSnapshot
		st.Objects = extractObjects(sql, "TABLE")
		// PATCH(amend-2026-05-25): a generic ALTER TABLE that fell through the
		// additive ADD COLUMN / ADD INDEX cases (e.g. ALTER COLUMN ... TYPE,
		// SET NOT NULL, DROP CONSTRAINT) can rewrite or revalidate the whole
		// table, so its row impact is unknown. The expand/contract decomposer
		// handles the type-change subset; everything else stays out of the safe
		// envelope.
		st.EstRowsAffected = -1
	// Data writes
	case strings.HasPrefix(head, "INSERT INTO"):
		st.OpClass, st.Reversibility = OpDataWrite, RevNone
		st.Objects = extractObjects(sql, "INTO")
		st.EstRowsNote = "inserted rows (not auto-revertible without PK capture)"
		st.EstRowsAffected = -1 // PATCH(amend-2026-05-25): row write, outside safe envelope
	case strings.HasPrefix(head, "UPDATE "):
		st.OpClass, st.Reversibility = OpDataWrite, RevNone
		st.Objects = extractObjects(sql, "UPDATE")
		st.EstRowsNote = rowEstimate(head)
		st.EstRowsAffected = -1 // PATCH(amend-2026-05-25): row rewrite, outside safe envelope
	default:
		st.OpClass, st.Reversibility = OpOther, RevSnapshot
		// PATCH(amend-2026-05-25): an unclassified statement has unknown row
		// impact, so it can never satisfy the est_rows==0 safe-envelope gate.
		st.EstRowsAffected = -1
	}
	if len(st.Objects) == 0 {
		st.Objects = []string{}
	}
	return st
}

func rowEstimate(head string) string {
	if !strings.Contains(head, " WHERE ") {
		return "ALL rows (no WHERE clause)"
	}
	return "rows matching WHERE clause"
}

func firstObject(objs []string) string {
	if len(objs) > 0 {
		return objs[0]
	}
	return ""
}

func extractObjects(sql, keyword string) []string {
	re := reIdentAfter(keyword)
	m := re.FindStringSubmatch(sql)
	if len(m) >= 2 {
		return []string{strings.Trim(m[1], `"`)}
	}
	return nil
}

// grantTargets pulls the ON <object> target from a GRANT/REVOKE.
func grantTargets(sql string) []string {
	re := regexp.MustCompile(`(?i)\bON\b\s+(?:TABLE\s+|FUNCTION\s+|SCHEMA\s+)?([a-zA-Z0-9_."]+)`)
	m := re.FindStringSubmatch(sql)
	if len(m) >= 2 {
		return []string{strings.Trim(m[1], `"`)}
	}
	return nil
}

// grantToRevoke produces the mechanical inverse of a GRANT (a REVOKE) or a
// REVOKE (a GRANT). It swaps the leading verb and the TO/FROM keyword.
func grantToRevoke(sql string, isGrant bool) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	if isGrant {
		out := regexp.MustCompile(`(?i)^GRANT\b`).ReplaceAllString(trimmed, "REVOKE")
		out = regexp.MustCompile(`(?i)\bTO\b`).ReplaceAllString(out, "FROM")
		return out + ";"
	}
	out := regexp.MustCompile(`(?i)^REVOKE\b`).ReplaceAllString(trimmed, "GRANT")
	out = regexp.MustCompile(`(?i)\bFROM\b`).ReplaceAllString(out, "TO")
	return out + ";"
}

func swapEnableDisableRLS(sql string, wasEnable bool) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	if wasEnable {
		return regexp.MustCompile(`(?i)\bENABLE ROW LEVEL SECURITY\b`).ReplaceAllString(trimmed, "DISABLE ROW LEVEL SECURITY") + ";"
	}
	return regexp.MustCompile(`(?i)\bDISABLE ROW LEVEL SECURITY\b`).ReplaceAllString(trimmed, "ENABLE ROW LEVEL SECURITY") + ";"
}

func inverseCreatePolicy(sql string) string {
	name := policyName(sql)
	tbl := firstObject(extractObjects(sql, "ON"))
	if name == "" || tbl == "" {
		return ""
	}
	return "DROP POLICY IF EXISTS " + name + " ON " + tbl + ";"
}

func policyName(sql string) string {
	re := regexp.MustCompile(`(?i)CREATE\s+POLICY\s+"?([a-zA-Z0-9_ ]+?)"?\s+ON\b`)
	m := re.FindStringSubmatch(sql)
	if len(m) >= 2 {
		return `"` + strings.TrimSpace(m[1]) + `"`
	}
	return ""
}

func addColumnName(sql string) string {
	re := regexp.MustCompile(`(?i)ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?"?([a-zA-Z0-9_]+)"?`)
	m := re.FindStringSubmatch(sql)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

var reColumnDefault = regexp.MustCompile(`(?i)\bDEFAULT\s+(.+?)(?:\s+(?:NOT\s+NULL|NULL|CHECK|REFERENCES|UNIQUE|PRIMARY\s+KEY|COLLATE)\b|,|$)`)

// addColumnHasVolatileDefault reports whether an ADD COLUMN carries a DEFAULT
// whose expression is volatile (a function call), which forces Postgres to
// rewrite every existing row. A bare literal / NULL / quoted-string default is
// constant: Postgres stores it as a catalog attmissingval and the add is
// metadata-only (est_rows 0, safe-envelope eligible). PATCH(amend-2026-05-25:
// volatile-default detection for the est-rows gate). The check is deliberately
// conservative — any `(` in the default expression (a function call such as
// now(), gen_random_uuid(), nextval(...)) is treated as volatile so the policy
// never auto-applies a full-table rewrite it mistook for metadata-only.
func addColumnHasVolatileDefault(sql string) bool {
	m := reColumnDefault.FindStringSubmatch(sql)
	if len(m) < 2 {
		return false // no DEFAULT clause -> nullable add, metadata-only
	}
	expr := strings.TrimSpace(m[1])
	// A function-call default contains a "(" (now(), gen_random_uuid(),
	// nextval('seq')). Bare literals (42, 'active', true, NULL) do not.
	return strings.Contains(expr, "(")
}
