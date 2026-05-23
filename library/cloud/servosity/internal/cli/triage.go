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

// newTriageCmd is terminal-speed batch issue triage. List + filter + batch
// mutations (ignore / archive / reactivate / comment) in one shot.
//
// PRODUCTION SAFETY:
//   - --dry-run is the DEFAULT. The user must drop --dry-run AND pass --confirm
//     to actually call the mutation endpoints.
//   - cliutil.IsVerifyEnv() is checked even with --confirm; verify subprocesses
//     never mutate.
//   - Admin broadcast endpoints are NOT reachable through this command.
func newTriageCmd(flags *rootFlags) *cobra.Command {
	var audience, companyFilter, resellerFilter string
	var ignoreList, archiveList, reactivateList string
	var commentList, commentText string
	var ignoreUntil string
	var ignoreSeconds int
	var confirm bool
	var limit int

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Terminal-speed batch issue triage (plan-by-default; --confirm to actually mutate)",
		Long: `List active issues with optional filters and batch-apply mutations:

  --ignore <ids>              POST /issues/{id}/ignore/   (with optional --ignore-until / --ignore-seconds)
  --archive <ids>             PUT  /issues/{id}/archive/
  --reactivate <ids>          PUT  /issues/{id}/reactivate/
  --comment <ids> --text "X"  PUT  /issues/{id}/comments/

Multiple ids are comma-separated. By default this command runs in PLAN
mode — it lists the issues that match your filters and prints what it
WOULD do, but does not call any mutation endpoint. To actually mutate,
pass --confirm. The global --dry-run flag is an extra safety net: it
keeps PLAN mode even if --confirm is passed.

Note: Servosity's /issues/?company=X requires state=ACTIVE — the CLI
passes it automatically. Ignored issues live at /issues/ignored/.`,
		Example: `  # See what's open for one company (read-only)
  servosity-pp-cli triage --company 4421 --json

  # Plan a batch ignore (no mutation; PLAN mode is default)
  servosity-pp-cli triage --company 4421 --ignore 18,22,29 --ignore-until "6am tomorrow"

  # Actually run the batch ignore against PROD
  servosity-pp-cli triage --company 4421 --ignore 18,22,29 --ignore-until "6am tomorrow" --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify-friendly short-circuit: if probed bare with --dry-run and
			// no filters, exit 0 cleanly without hitting the API.
			if dryRunOK(flags) && companyFilter == "" && resellerFilter == "" &&
				ignoreList == "" && archiveList == "" && reactivateList == "" && commentList == "" {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run","will_mutate":false},"issues":[],"outcomes":[]}` + "\n"))
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// 1) List active issues for context
			params := map[string]string{"state": "ACTIVE"}
			if audience != "" {
				params["audience"] = audience
			}
			if companyFilter != "" {
				params["company"] = companyFilter
			}
			if resellerFilter != "" {
				params["reseller"] = resellerFilter
			}
			data, err := c.Get("/issues/", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var listEnv struct {
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal(data, &listEnv); err != nil {
				return apiErr(fmt.Errorf("parse /issues/: %w", err))
			}
			issues := listEnv.Results
			if limit > 0 && len(issues) > limit {
				issues = issues[:limit]
			}

			// 2) Compute the planned mutation set
			plan := triagePlan{
				Ignore:     splitIDs(ignoreList),
				Archive:    splitIDs(archiveList),
				Reactivate: splitIDs(reactivateList),
				Comment:    splitIDs(commentList),
				CommentTxt: commentText,
			}
			if ignoreUntil != "" {
				t, perr := parseHumanTime(ignoreUntil, time.Now())
				if perr != nil {
					return usageErr(perr)
				}
				secs := int(time.Until(t).Seconds())
				if secs <= 0 {
					return usageErr(fmt.Errorf("--ignore-until %q resolves to a past time", ignoreUntil))
				}
				plan.IgnoreSeconds = secs
				plan.IgnoreUntilHuman = ignoreUntil
			} else if ignoreSeconds > 0 {
				plan.IgnoreSeconds = ignoreSeconds
			}

			// 3) Decide whether to execute
			//    PLAN mode by default. --confirm is required to mutate.
			//    Global --dry-run is an extra safety net that overrides --confirm.
			willMutate := confirm && !dryRunOK(flags) && plan.NonEmpty()
			if willMutate && cliutil.IsVerifyEnv() {
				willMutate = false
			}

			outcomes := []map[string]any{}
			if willMutate {
				outcomes = applyTriage(ctx, c, plan)
			}

			out := map[string]any{
				"meta": map[string]any{
					"source":         "live",
					"matched_issues": len(issues),
					"will_mutate":    willMutate,
					"dry_run":        !willMutate,
					"plan":           plan.summary(),
					"verify_env":     cliutil.IsVerifyEnv(),
				},
				"issues":   issues,
				"outcomes": outcomes,
			}
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().StringVar(&audience, "audience", "", "Filter by issue audience")
	cmd.Flags().StringVar(&companyFilter, "company", "", "Filter to one company ID")
	cmd.Flags().StringVar(&resellerFilter, "reseller", "", "Filter to one reseller ID")
	cmd.Flags().StringVar(&ignoreList, "ignore", "", "Comma-separated issue IDs to ignore (POST /issues/{id}/ignore/)")
	cmd.Flags().StringVar(&archiveList, "archive", "", "Comma-separated issue IDs to archive")
	cmd.Flags().StringVar(&reactivateList, "reactivate", "", "Comma-separated issue IDs to reactivate")
	cmd.Flags().StringVar(&commentList, "comment", "", "Comma-separated issue IDs to comment on")
	cmd.Flags().StringVar(&commentText, "text", "", "Comment body (required with --comment)")
	cmd.Flags().StringVar(&ignoreUntil, "ignore-until", "", "Human time for ignore expiry (e.g. \"6am tomorrow\", \"1d\", RFC3339)")
	cmd.Flags().IntVar(&ignoreSeconds, "ignore-seconds", 0, "Explicit ignored_seconds (use --ignore-until instead unless you have a precomputed value)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required to actually call the mutation endpoints (PLAN mode is default)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum issues to list before applying mutations")
	return cmd
}

type triagePlan struct {
	Ignore           []string
	Archive          []string
	Reactivate       []string
	Comment          []string
	CommentTxt       string
	IgnoreSeconds    int
	IgnoreUntilHuman string
}

func (p triagePlan) NonEmpty() bool {
	return len(p.Ignore)+len(p.Archive)+len(p.Reactivate)+len(p.Comment) > 0
}

func (p triagePlan) summary() map[string]any {
	return map[string]any{
		"ignore":             p.Ignore,
		"archive":            p.Archive,
		"reactivate":         p.Reactivate,
		"comment":            p.Comment,
		"comment_text":       p.CommentTxt,
		"ignore_seconds":     p.IgnoreSeconds,
		"ignore_until_human": p.IgnoreUntilHuman,
	}
}

func splitIDs(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

type triageHTTP interface {
	Put(path string, body any) (json.RawMessage, int, error)
}

func applyTriage(_ context.Context, c triageHTTP, plan triagePlan) []map[string]any {
	out := []map[string]any{}
	mark := func(action, id string, status int, err error) {
		row := map[string]any{"action": action, "id": id, "status": status}
		if err != nil {
			row["error"] = err.Error()
		}
		out = append(out, row)
	}
	for _, id := range plan.Ignore {
		body := map[string]any{}
		if plan.IgnoreSeconds > 0 {
			body["ignored_seconds"] = plan.IgnoreSeconds
		}
		_, status, err := c.Put("/issues/"+id+"/ignore/", body)
		mark("ignore", id, status, err)
	}
	for _, id := range plan.Archive {
		_, status, err := c.Put("/issues/"+id+"/archive/", nil)
		mark("archive", id, status, err)
	}
	for _, id := range plan.Reactivate {
		_, status, err := c.Put("/issues/"+id+"/reactivate/", nil)
		mark("reactivate", id, status, err)
	}
	for _, id := range plan.Comment {
		body := map[string]any{"text": plan.CommentTxt}
		_, status, err := c.Put("/issues/"+id+"/comments/", body)
		mark("comment", id, status, err)
	}
	return out
}
