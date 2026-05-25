// PATCH: novel top-level `advisors security|performance` — ergonomic wrapper over the splinter advisor endpoints with cross-project --refs fleet mode.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newAdvisorsTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advisors",
		Short: "Pull splinter security/performance advisor findings (single project or --refs fleet)",
		Long: `Turn manual dashboard advisor-reading into one command. Wraps the Management
API advisor endpoints:

  advisors security <ref>      Security linter findings (RLS gaps, exposed grants, etc.)
  advisors performance <ref>   Performance linter findings (missing indexes, init-plan re-eval, etc.)

Pass --refs a,b,c instead of a positional <ref> to sweep a whole fleet of
projects and aggregate the findings into one result keyed by project.`,
	}
	cmd.AddCommand(newAdvisorsKindCmd(flags, "security", "/v1/projects/{ref}/advisors/security"))
	cmd.AddCommand(newAdvisorsKindCmd(flags, "performance", "/v1/projects/{ref}/advisors/performance"))
	return cmd
}

func newAdvisorsKindCmd(flags *rootFlags, kind, pathTmpl string) *cobra.Command {
	var refs string

	cmd := &cobra.Command{
		Use:         kind + " [ref]",
		Short:       fmt.Sprintf("Pull %s advisor findings for a project (or --refs fleet)", kind),
		Example:     fmt.Sprintf("  supabase-pp-cli advisors %s abcdefgh --json\n  supabase-pp-cli advisors %s --refs proj1,proj2,proj3 --json", kind, kind),
		Annotations: map[string]string{"pp:method": "GET", "pp:path": pathTmpl, "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			refList, err := resolveRefs(args, refs)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Single-ref: return the advisor payload directly. Advisor
			// endpoints are GET on the Management API.
			if len(refList) == 1 {
				path := replacePathParam(pathTmpl, "ref", refList[0])
				gdata, gerr := c.Get(path, nil)
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				out := cmd.OutOrStdout()
				if wantsHumanTable(out, flags) {
					var items []map[string]any
					if json.Unmarshal(gdata, &items) == nil && len(items) > 0 {
						if printAutoTable(out, items) == nil {
							return nil
						}
					}
				}
				return printOutputWithFlags(out, gdata, flags)
			}

			// Fleet mode: aggregate per-ref.
			agg := fleetAdvisors(c, flags, refList, pathTmpl)
			return printJSONFiltered(cmd.OutOrStdout(), agg, flags)
		},
	}
	cmd.Flags().StringVar(&refs, "refs", "", "Comma-separated project refs to sweep as a fleet (instead of a single positional ref)")
	return cmd
}

// extractProjectRefs pulls the "id"/"ref" field from a /v1/projects list
// response. Used by doctor to surface reachable project refs.
func extractProjectRefs(data json.RawMessage) []string {
	var items []map[string]any
	if json.Unmarshal(data, &items) != nil {
		return nil
	}
	var out []string
	for _, it := range items {
		for _, k := range []string{"id", "ref"} {
			if v, ok := it[k].(string); ok && v != "" {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// resolveRefs reconciles a positional ref arg with --refs. Exactly one source
// must be provided; --refs may carry one or many.
func resolveRefs(args []string, refs string) ([]string, error) {
	hasPositional := len(args) > 0 && args[0] != ""
	hasRefs := strings.TrimSpace(refs) != ""
	switch {
	case hasPositional && hasRefs:
		return nil, usageErr(fmt.Errorf("provide either a positional <ref> or --refs, not both"))
	case hasPositional:
		return []string{args[0]}, nil
	case hasRefs:
		var out []string
		for _, r := range strings.Split(refs, ",") {
			if t := strings.TrimSpace(r); t != "" {
				out = append(out, t)
			}
		}
		if len(out) == 0 {
			return nil, usageErr(fmt.Errorf("--refs was empty after parsing"))
		}
		return out, nil
	default:
		return nil, usageErr(fmt.Errorf("no project ref(s): pass a positional <ref> or --refs a,b,c"))
	}
}

// fleetAdvisors sweeps the advisor endpoint across refs, capturing per-ref
// findings (or per-ref errors) into one aggregate result.
func fleetAdvisors(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, flags *rootFlags, refs []string, pathTmpl string) map[string]any {
	type refResult struct {
		Ref     string          `json:"project_ref"`
		Error   string          `json:"error,omitempty"`
		Count   int             `json:"finding_count"`
		Finding json.RawMessage `json:"findings,omitempty"`
	}
	var results []refResult
	total := 0
	for _, ref := range refs {
		path := replacePathParam(pathTmpl, "ref", ref)
		data, err := c.Get(path, nil)
		rr := refResult{Ref: ref}
		if err != nil {
			rr.Error = err.Error()
		} else {
			var items []json.RawMessage
			_ = json.Unmarshal(data, &items)
			rr.Count = len(items)
			rr.Finding = data
			total += rr.Count
		}
		results = append(results, rr)
	}
	return map[string]any{
		"fleet":         true,
		"project_count": len(refs),
		"total_findings": total,
		"projects":      results,
	}
}
