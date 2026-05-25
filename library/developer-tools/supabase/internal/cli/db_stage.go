// PATCH(amend-2026-05-25: branch-first staging gate — a plan is only
// prod-eligible after it applies cleanly to a Supabase preview branch with the
// security + performance advisors green). Reuses the branch config + database
// query + advisor endpoints. Not a single Management API endpoint.
package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

// stageOnBranch enforces the branch-first staging gate. It resolves the preview
// branch's own project ref, applies the same transaction to the branch, then
// runs the security and performance advisors against the branch and refuses
// (non-nil error) unless both come back with zero findings. A clean stage is
// the precondition for the prod apply that follows in db_apply.go.
//
// This deliberately reuses the existing surfaces: branch config (GET
// /v1/branches/{ref}) to resolve the branch project ref, the same
// database/query endpoint db apply uses, and the advisor endpoints the
// top-level `advisors` command wraps. It does NOT push via the migration
// pipeline — staging the exact transaction that prod will run is the stronger
// guarantee than replaying a migration file.
func stageOnBranch(cmd *cobra.Command, flags *rootFlags, branch string, m *dbchange.Manifest, _ string, _ string) error {
	errOut := cmd.ErrOrStderr()
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	// 1. Resolve the branch's own project ref from its config.
	branchRef, rerr := resolveBranchProjectRef(c, branch)
	if rerr != nil {
		return apiErr(fmt.Errorf("branch-first staging: cannot resolve preview branch %q: %w", branch, rerr))
	}
	fmt.Fprintf(errOut, "staging on preview branch %s (project_ref %s)...\n", branch, branchRef)

	// 2. Apply the exact transaction to the branch.
	txSQL := assembleTransaction(m)
	qPath := replacePathParam("/v1/projects/{ref}/database/query", "ref", branchRef)
	if _, _, perr := c.Post(qPath, map[string]any{"query": txSQL}); perr != nil {
		return apiErr(fmt.Errorf("branch-first staging: plan failed to apply to preview branch %s: %w (NOT applied to prod)", branchRef, perr))
	}

	// 3. Advisors must be green on the branch.
	for _, kind := range []string{"security", "performance"} {
		n, aerr := advisorFindingCount(c, branchRef, kind)
		if aerr != nil {
			return apiErr(fmt.Errorf("branch-first staging: %s advisor check failed on branch %s: %w (NOT applied to prod)", kind, branchRef, aerr))
		}
		if n > 0 {
			return apiErr(fmt.Errorf("branch-first staging: %s advisor reports %d finding(s) on preview branch %s; not prod-eligible (NOT applied to prod). Inspect with: supabase-pp-cli advisors %s %s", kind, n, branchRef, kind, branchRef))
		}
		fmt.Fprintf(errOut, "  advisors %s: green\n", kind)
	}
	fmt.Fprintf(errOut, "staging passed — proceeding to prod apply\n")
	return nil
}

// resolveBranchProjectRef reads the preview branch config and returns the
// branch's own database project ref. Supabase branch config returns the ref
// under one of a few field names depending on API version; try them in order.
func resolveBranchProjectRef(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, branch string) (string, error) {
	path := replacePathParam("/v1/branches/{branch_id_or_ref}", "branch_id_or_ref", branch)
	data, err := c.Get(path, nil)
	if err != nil {
		return "", err
	}
	var cfg map[string]any
	if uerr := json.Unmarshal(data, &cfg); uerr != nil {
		return "", fmt.Errorf("parsing branch config: %w", uerr)
	}
	for _, k := range []string{"project_ref", "ref", "id"} {
		if v, ok := cfg[k].(string); ok && v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("branch config has no project_ref/ref/id field")
}

// advisorFindingCount runs one advisor kind against a ref and returns the
// number of findings. The advisor payload is a JSON array (or {data:[...]}); an
// empty array is green.
func advisorFindingCount(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, ref, kind string) (int, error) {
	path := replacePathParam("/v1/projects/{ref}/advisors/"+kind, "ref", ref)
	data, err := c.Get(path, nil)
	if err != nil {
		return 0, err
	}
	// Direct array shape.
	var items []json.RawMessage
	if json.Unmarshal(data, &items) == nil {
		return len(items), nil
	}
	// Wrapped {data: [...]} or {lints: [...]} shape.
	var wrapped struct {
		Data  []json.RawMessage `json:"data"`
		Lints []json.RawMessage `json:"lints"`
	}
	if json.Unmarshal(data, &wrapped) == nil {
		if len(wrapped.Lints) > 0 {
			return len(wrapped.Lints), nil
		}
		return len(wrapped.Data), nil
	}
	return 0, nil
}
