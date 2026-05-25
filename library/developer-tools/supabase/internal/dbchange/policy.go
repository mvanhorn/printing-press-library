// PATCH(amend-2026-05-25: policy approver — self-authorize an apply when every
// statement is inside a declared safe envelope). Replaces the human
// `--approved <plan-hash>` paste for the additive+reversible class of changes.
// Not in the Management API.
//
// The policy is the safety classifier the manifest was built to feed. It
// evaluates the typed Manifest against an allowed-operation envelope and
// self-authorizes `db apply` ONLY when every statement is in an allowed class,
// reversible, lint-clean, and touches zero existing data rows. It replaces the
// human approver; it does NOT replace the plan_hash drift gate, the savepoint
// transaction, or the receipt — those safety primitives still run.
package dbchange

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Policy declares the operation envelope an apply may self-authorize within.
// The zero value authorizes nothing; use SafeDefaultPolicy() for the
// Shane-approved additive+reversible envelope, or LoadPolicy() for a file.
type Policy struct {
	// Name is a human label surfaced in the authorization receipt.
	Name string `json:"name"`
	// AllowedClasses is the set of OpClass values a statement may belong to and
	// still be auto-eligible. A statement whose class is absent fails the gate.
	AllowedClasses []OpClass `json:"allowed_classes"`
	// RequireReversible, when true, fails any statement whose Reversibility is
	// RevNone (no reliable inverse).
	RequireReversible bool `json:"require_reversible"`
	// RequireLintClean, when true, fails the plan if it has any ERROR-level lint
	// finding. The caller supplies the error count (lint lives in package cli).
	RequireLintClean bool `json:"require_lint_clean"`
	// MaxEstRowsAffected caps the manifest-level row impact. 0 means "no existing
	// data rows may be touched" — the safe-default. A negative manifest rollup
	// (unknown/unbounded) always fails regardless of this cap.
	MaxEstRowsAffected int `json:"max_est_rows_affected"`
	// AllowDestructive, when false, fails any statement flagged Destructive even
	// if its class is otherwise allowed. The safe-default leaves this false;
	// destructive changes route through the expand/contract decomposer instead.
	AllowDestructive bool `json:"allow_destructive"`
}

// SafeDefaultPolicy returns the Shane-approved AUTO-APPLY envelope: additive +
// reversible only. CREATE table/index/policy/function, ADD COLUMN
// nullable-or-with-constant-default, ADD INDEX CONCURRENTLY, GRANT, RLS policy
// add/rewrite — every statement reversible, lint clean, est_rows == 0, nothing
// destructive.
func SafeDefaultPolicy() *Policy {
	return &Policy{
		Name:               "safe-default",
		AllowedClasses:     []OpClass{OpDDL, OpGrant, OpRLS},
		RequireReversible:  true,
		RequireLintClean:   true,
		MaxEstRowsAffected: 0,
		AllowDestructive:   false,
	}
}

// LoadPolicy reads a policy JSON file from disk. The file shape mirrors the
// Policy struct. A missing or malformed file is a hard error — the apply must
// not fall back to a permissive default.
func LoadPolicy(path string) (*Policy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading policy file %q: %w", path, err)
	}
	var p Policy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parsing policy file %q: %w", path, err)
	}
	if p.Name == "" {
		p.Name = "custom:" + path
	}
	if len(p.AllowedClasses) == 0 {
		return nil, fmt.Errorf("policy %q allows no operation classes — it would authorize nothing", path)
	}
	return &p, nil
}

// PolicyVerdict is the machine-readable result of evaluating a plan against a
// policy. Authorized is true only when Reasons is empty.
type PolicyVerdict struct {
	Policy     string          `json:"policy"`
	Authorized bool            `json:"authorized"`
	PlanHash   string          `json:"plan_hash"`
	// Reasons holds one structured refusal per failing statement (plus
	// manifest-level reasons with StatementIndex -1). Empty when authorized.
	Reasons []PolicyReason `json:"reasons,omitempty"`
}

// PolicyReason is a single structured refusal: which statement, which rule, and
// a human message. StatementIndex is -1 for manifest-level reasons.
type PolicyReason struct {
	StatementIndex int     `json:"statement_index"`
	Rule           string  `json:"rule"`
	OpClass        OpClass `json:"op_class,omitempty"`
	Object         string  `json:"object,omitempty"`
	Message        string  `json:"message"`
}

// Evaluate runs the policy over the manifest and returns a structured verdict.
// errorLintCount is the number of ERROR-level lint findings the caller computed
// (lint lives in package cli, which imports this package; passing the count in
// avoids an import cycle). When RequireLintClean is false the count is ignored.
//
// Evaluate NEVER mutates and NEVER authorizes on ambiguity: any condition it
// cannot positively clear becomes a refusal reason. A plan with zero statements
// is refused (nothing to authorize).
func (p *Policy) Evaluate(m *Manifest, errorLintCount int) PolicyVerdict {
	v := PolicyVerdict{Policy: p.Name, PlanHash: m.PlanHash}

	if m.StatementN == 0 {
		v.Reasons = append(v.Reasons, PolicyReason{
			StatementIndex: -1,
			Rule:           "empty_plan",
			Message:        "plan has no statements; nothing to authorize",
		})
		return v
	}

	allowed := map[OpClass]bool{}
	for _, c := range p.AllowedClasses {
		allowed[c] = true
	}

	for _, s := range m.Statements {
		obj := ""
		if len(s.Objects) > 0 {
			obj = s.Objects[0]
		}
		if !allowed[s.OpClass] {
			v.Reasons = append(v.Reasons, PolicyReason{
				StatementIndex: s.Index, Rule: "class_not_allowed", OpClass: s.OpClass, Object: obj,
				Message: fmt.Sprintf("operation class %q is not in the %q envelope", s.OpClass, p.Name),
			})
		}
		if !p.AllowDestructive && s.Destructive {
			v.Reasons = append(v.Reasons, PolicyReason{
				StatementIndex: s.Index, Rule: "destructive_not_allowed", OpClass: s.OpClass, Object: obj,
				Message: "statement is destructive; the safe envelope refuses it (route through expand/contract decomposition)",
			})
		}
		if p.RequireReversible && s.Reversibility == RevNone {
			v.Reasons = append(v.Reasons, PolicyReason{
				StatementIndex: s.Index, Rule: "not_reversible", OpClass: s.OpClass, Object: obj,
				Message: "statement has no reliable inverse (reversibility=none); the safe envelope requires reversibility",
			})
		}
		// Per-statement row gate: only enforced under the strict envelope
		// (cap 0 = "no existing data rows may be touched"). A policy that
		// declares a higher or unbounded cap accepts row-touching statements;
		// they are then governed only by the manifest-level cap below.
		if p.MaxEstRowsAffected == 0 && s.EstRowsAffected != 0 {
			v.Reasons = append(v.Reasons, PolicyReason{
				StatementIndex: s.Index, Rule: "touches_data_rows", OpClass: s.OpClass, Object: obj,
				Message: fmt.Sprintf("statement touches existing data rows (est_rows_affected=%d); the safe envelope requires 0", s.EstRowsAffected),
			})
		}
	}

	// Manifest-level row cap. A negative policy cap means "unknown/unbounded row
	// impact is accepted" — only an explicit opt-in. Otherwise an unknown plan
	// (-1) or a concrete count above a non-negative cap fails.
	if p.MaxEstRowsAffected >= 0 && (m.MaxEstRowsAffected < 0 || m.MaxEstRowsAffected > p.MaxEstRowsAffected) {
		v.Reasons = append(v.Reasons, PolicyReason{
			StatementIndex: -1, Rule: "est_rows_exceeds_cap",
			Message: fmt.Sprintf("plan max est_rows_affected=%d exceeds policy cap %d", m.MaxEstRowsAffected, p.MaxEstRowsAffected),
		})
	}

	if p.RequireLintClean && errorLintCount > 0 {
		v.Reasons = append(v.Reasons, PolicyReason{
			StatementIndex: -1, Rule: "lint_not_clean",
			Message: fmt.Sprintf("plan has %d ERROR-level lint finding(s); the safe envelope requires a lint-clean plan", errorLintCount),
		})
	}

	// Stable ordering: statement-level reasons by index, then manifest-level
	// (-1) reasons last, so JSON output and human display are deterministic.
	sort.SliceStable(v.Reasons, func(i, j int) bool {
		a, b := v.Reasons[i].StatementIndex, v.Reasons[j].StatementIndex
		if a < 0 {
			a = 1 << 30
		}
		if b < 0 {
			b = 1 << 30
		}
		return a < b
	})

	v.Authorized = len(v.Reasons) == 0
	return v
}
