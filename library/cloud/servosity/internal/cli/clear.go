// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/cliutil"
)

// newClearCmd is the daily Tier-One workflow: "ignore all current issues at
// COMPANY (or PARTNER) until HUMAN-TIME". Replaces the cc-skills clear-company
// SKILL's curl ladder.
//
// PRODUCTION SAFETY: --dry-run is default; --confirm required to mutate.
func newClearCmd(flags *rootFlags) *cobra.Command {
	var until string
	var confirm bool
	var asPartner bool

	cmd := &cobra.Command{
		Use:   "clear [names...]",
		Short: "Resolve company-or-partner names and ignore all their active issues until a human time",
		Long: `For each comma-separated name:
  1. Search /companies/?search=NAME first
  2. If no company matches, fall back to /resellers/?search=NAME (treat as partner)
  3. Compute ignored_seconds from --until ("6am tomorrow", "1d", "30m", RFC3339)
  4. List active issues for each match (per company OR per company-under-partner)
  5. Print the plan; with --confirm and no --dry-run, call PUT /issues/{id}/ignore/

Defaults to --dry-run for production safety. The cc-skills clear-company
SKILL is the inspiration — same workflow, one command, agent-friendly.`,
		Example: `  # Plan: clear ACME until 6am tomorrow (read-only by default)
  servosity-pp-cli clear "ACME Corp" --until "6am tomorrow"

  # Multi-name plus partner search
  servosity-pp-cli clear "ACME Corp, BDH Technology" --until "6am tomorrow"

  # Actually execute against PROD
  servosity-pp-cli clear "ACME Corp" --until "6am tomorrow" --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify-friendly: short-circuit on --help-only and --dry-run probes
			// before any required-arg validation, so verify can probe this command.
			if dryRunOK(flags) && len(args) == 0 {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run","will_mutate":false},"plan":{"companies":[],"total_issues":0},"outcomes":[]}` + "\n"))
				}
				return nil
			}
			if len(args) == 0 && until == "" {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			rawNames := strings.Join(args, " ")
			names := splitNames(rawNames)
			if len(names) == 0 {
				return usageErr(fmt.Errorf("no names provided"))
			}

			if until == "" {
				return usageErr(fmt.Errorf("--until is required (e.g. \"6am tomorrow\", \"1d\")"))
			}
			t, err := parseHumanTime(until, time.Now())
			if err != nil {
				return usageErr(err)
			}
			ignoreSeconds := int(time.Until(t).Seconds())
			if ignoreSeconds <= 0 {
				return usageErr(fmt.Errorf("--until %q resolves to a past time", until))
			}

			plan, err := planClear(ctx, c, names, asPartner)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			willMutate := confirm && !dryRunOK(flags) && plan.totalIssues() > 0
			if willMutate && cliutil.IsVerifyEnv() {
				willMutate = false
			}
			outcomes := []map[string]any{}
			if willMutate {
				outcomes = executeClear(ctx, c, plan, ignoreSeconds)
			}
			out := map[string]any{
				"meta": map[string]any{
					"source":         "live",
					"names":          names,
					"until_human":    until,
					"until_resolved": t.Format(time.RFC3339),
					"ignore_seconds": ignoreSeconds,
					"will_mutate":    willMutate,
					"dry_run":        !willMutate,
					"verify_env":     cliutil.IsVerifyEnv(),
				},
				"plan":     plan.summary(),
				"outcomes": outcomes,
			}
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "Human time to ignore until (e.g. \"6am tomorrow\", \"1d\", RFC3339) [REQUIRED]")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required to actually call /issues/{id}/ignore/ (PLAN mode is default)")
	cmd.Flags().BoolVar(&asPartner, "partner-only", false, "Skip the company-search step and treat all names as resellers")
	return cmd
}

func splitNames(raw string) []string {
	out := []string{}
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type clearPlan struct {
	Companies       []companyTriagePlan
	UnresolvedNames []string
}

type companyTriagePlan struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"` // "company" | "partner"
	PartnerName string   `json:"partner_name,omitempty"`
	CompanyID   string   `json:"company_id"`
	IssueIDs    []string `json:"issue_ids"`
}

func (p clearPlan) totalIssues() int {
	n := 0
	for _, c := range p.Companies {
		n += len(c.IssueIDs)
	}
	return n
}

func (p clearPlan) summary() map[string]any {
	rows := make([]map[string]any, 0, len(p.Companies))
	for _, c := range p.Companies {
		rows = append(rows, map[string]any{
			"name":         c.Name,
			"source":       c.Source,
			"partner_name": c.PartnerName,
			"company_id":   c.CompanyID,
			"issue_ids":    c.IssueIDs,
			"issue_count":  len(c.IssueIDs),
		})
	}
	unresolved := p.UnresolvedNames
	if unresolved == nil {
		unresolved = []string{}
	}
	return map[string]any{
		"companies":        rows,
		"total_issues":     p.totalIssues(),
		"unresolved_names": unresolved,
	}
}

type clearHTTP interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
	Put(path string, body any) (json.RawMessage, int, error)
}

func planClear(_ context.Context, c clearHTTP, names []string, asPartner bool) (clearPlan, error) {
	plan := clearPlan{}
	for _, n := range names {
		startCount := len(plan.Companies)
		if !asPartner {
			data, _ := c.Get("/companies/", map[string]string{"search": n})
			if data != nil {
				var env struct {
					Results []map[string]any `json:"results"`
				}
				if err := json.Unmarshal(data, &env); err == nil && len(env.Results) > 0 {
					for _, co := range env.Results {
						cid := anyToString(firstAny(co, "id", "pk"))
						if cid == "" {
							continue
						}
						issues := listActiveIssuesByCompany(c, cid)
						plan.Companies = append(plan.Companies, companyTriagePlan{
							Name:   anyToString(firstAny(co, "name", "title")),
							Source: "company", CompanyID: cid, IssueIDs: issues,
						})
					}
					if len(plan.Companies) > startCount {
						continue
					}
				}
			}
		}
		// Try partner / reseller
		data, _ := c.Get("/resellers/", map[string]string{"search": n})
		var renv struct {
			Results []map[string]any `json:"results"`
		}
		if data != nil {
			if err := json.Unmarshal(data, &renv); err == nil && len(renv.Results) > 0 {
				for _, r := range renv.Results {
					rid := anyToString(firstAny(r, "id", "pk"))
					if rid == "" {
						continue
					}
					rname := anyToString(firstAny(r, "name", "title"))
					// list companies under this reseller, then issues per company
					cdata, _ := c.Get("/companies/", map[string]string{"reseller": rid, "page_size": "200"})
					var cenv struct {
						Results []map[string]any `json:"results"`
					}
					if cdata != nil && json.Unmarshal(cdata, &cenv) == nil {
						for _, co := range cenv.Results {
							cid := anyToString(firstAny(co, "id", "pk"))
							if cid == "" {
								continue
							}
							issues := listActiveIssuesByCompany(c, cid)
							plan.Companies = append(plan.Companies, companyTriagePlan{
								Name:   anyToString(firstAny(co, "name", "title")),
								Source: "partner", PartnerName: rname,
								CompanyID: cid, IssueIDs: issues,
							})
						}
					}
				}
			}
		}
		// Track names that resolved to zero companies/partners — the classic
		// silent fan-out drop the user can't see from `plan.companies: []`.
		if len(plan.Companies) == startCount {
			plan.UnresolvedNames = append(plan.UnresolvedNames, n)
		}
	}
	return plan, nil
}

func listActiveIssuesByCompany(c clearHTTP, cid string) []string {
	data, err := c.Get("/issues/", map[string]string{"company": cid, "state": "ACTIVE"})
	if err != nil {
		return nil
	}
	var env struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	out := make([]string, 0, len(env.Results))
	for _, it := range env.Results {
		if id := anyToString(firstAny(it, "id", "uuid", "pk")); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func executeClear(_ context.Context, c clearHTTP, plan clearPlan, ignoreSeconds int) []map[string]any {
	out := []map[string]any{}
	body := map[string]any{}
	if ignoreSeconds > 0 {
		body["ignored_seconds"] = ignoreSeconds
	}
	for _, co := range plan.Companies {
		for _, id := range co.IssueIDs {
			_, status, err := c.Put("/issues/"+id+"/ignore/", body)
			row := map[string]any{
				"action":     "ignore",
				"company":    co.Name,
				"company_id": co.CompanyID,
				"issue_id":   id,
				"status":     status,
			}
			if err != nil {
				row["error"] = err.Error()
			}
			out = append(out, row)
		}
	}
	return out
}
