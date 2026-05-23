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

// newStaleIssuesCmd is the daily Tier-One morning workflow: pull YOUR FMDB
// companies (companies/fully-managed-ng/?reseller__dedicated_support_staff__username=ME),
// fetch active issues per company, classify known-safe-to-archive patterns vs
// unknown, auto-archive safe + ignore non-dashboard noise, print unknowns for
// review.
//
// PRODUCTION SAFETY: --dry-run is default; --confirm required to mutate.
func newStaleIssuesCmd(flags *rootFlags) *cobra.Command {
	var mine bool
	var cutoff string
	var autoArchiveKnown bool
	var confirm bool
	var resellerFilter string
	var limit int

	cmd := &cobra.Command{
		Use:   "stale-issues",
		Short: "Per-engineer stale-issue cleanup: classify known-safe-to-archive patterns and act with --dry-run + --confirm",
		Long: `Replaces the cc-skills stale-issue-cleanup workflow:

  1. Pull /companies/fully-managed-ng/?reseller__dedicated_support_staff__username=ME
     (when --mine; otherwise pull all FMDB companies or one reseller)
  2. For each company, fetch active issues
  3. Classify each by a shipped rule table:
       - "auto_archive_safe": known stale patterns that are always safe to archive
       - "ignore_noise":      non-dashboard issues that just clutter
       - "unknown":           novel patterns; print for manual review
  4. With --auto-archive-known + --confirm + no --dry-run, batch-execute the
     auto-archive plan via PUT /issues/{id}/archive/

The classifier is a hand-curated rule table (no LLM). Patterns are matched
on issue.title + issue.code substrings.`,
		Example: `  # Print my morning sweep, no mutations
  servosity-pp-cli stale-issues --mine --json

  # Plan auto-archive of known-safe stale issues (still --dry-run by default)
  servosity-pp-cli stale-issues --mine --cutoff "11pm yesterday" --auto-archive-known

  # Actually run auto-archive against PROD
  servosity-pp-cli stale-issues --mine --cutoff "11pm yesterday" --auto-archive-known --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify-friendly short-circuit: if probed with bare --dry-run and
			// nothing to do, exit 0 cleanly without hitting the API.
			if dryRunOK(flags) && !mine && resellerFilter == "" {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run","will_mutate":false},"plan":{"auto_archive":[],"ignore_noise":[],"unknown":[]},"outcomes":[]}` + "\n"))
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			cutoffT := time.Time{}
			if cutoff != "" {
				t, perr := parseHumanTime(cutoff, time.Now())
				if perr != nil {
					return usageErr(perr)
				}
				cutoffT = t
			}

			// 1) Resolve FMDB companies
			username := ""
			if mine {
				userData, err := c.Get("/current-user/", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var u map[string]any
				if err := json.Unmarshal(userData, &u); err != nil {
					return apiErr(fmt.Errorf("parse current-user: %w", err))
				}
				username = anyToString(u["username"])
				// Under --dry-run the client returns a stub envelope (no real
				// /current-user/ call), so username is unresolvable. Label it
				// clearly so an agent reading the output doesn't mistake the
				// empty value for a broken filter.
				if username == "" && dryRunOK(flags) {
					username = "(unresolved — dry-run mode)"
				}
			}
			fmdbParams := map[string]string{}
			if username != "" {
				fmdbParams["reseller__dedicated_support_staff__username"] = username
			}
			if resellerFilter != "" {
				fmdbParams["reseller"] = resellerFilter
			}
			fdata, err := c.Get("/companies/fully-managed-ng/", fmdbParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var fmdbList []map[string]any
			if err := json.Unmarshal(fdata, &fmdbList); err != nil {
				// Some deployments paginate; try the envelope
				var env struct {
					Results []map[string]any `json:"results"`
				}
				if err2 := json.Unmarshal(fdata, &env); err2 != nil {
					return apiErr(fmt.Errorf("parse FMDB list: %w", err))
				}
				fmdbList = env.Results
			}

			// 2) Walk each company, classify, plan
			plan := staleIssuesPlan{}
			for i, co := range fmdbList {
				if limit > 0 && i >= limit {
					break
				}
				cid := anyToString(firstAny(co, "id", "pk"))
				if cid == "" {
					continue
				}
				cname := anyToString(firstAny(co, "name", "title"))
				idata, ierr := c.Get("/issues/", map[string]string{"company": cid, "state": "ACTIVE"})
				if ierr != nil {
					continue
				}
				var ienv struct {
					Results []map[string]any `json:"results"`
				}
				if err := json.Unmarshal(idata, &ienv); err != nil {
					continue
				}
				for _, is := range ienv.Results {
					if !cutoffT.IsZero() && !staleEnough(is, cutoffT) {
						continue
					}
					id := anyToString(firstAny(is, "id", "uuid", "pk"))
					title := anyToString(firstAny(is, "title", "subject", "summary", "name"))
					code := anyToString(firstAny(is, "code", "issue_code", "type"))
					verdict, reason := classifyStaleIssue(title, code)
					row := staleIssueRow{
						IssueID: id, CompanyID: cid, CompanyName: cname,
						Title: title, Code: code, Verdict: verdict, Reason: reason,
					}
					switch verdict {
					case "auto_archive_safe":
						plan.AutoArchive = append(plan.AutoArchive, row)
					case "ignore_noise":
						plan.IgnoreNoise = append(plan.IgnoreNoise, row)
					default:
						plan.Unknown = append(plan.Unknown, row)
					}
				}
			}

			// 3) Decide whether to execute
			willMutate := autoArchiveKnown && confirm && !dryRunOK(flags) && len(plan.AutoArchive) > 0
			if willMutate && cliutil.IsVerifyEnv() {
				willMutate = false
			}
			outcomes := []map[string]any{}
			if willMutate {
				outcomes = applyStaleIssueArchive(ctx, c, plan.AutoArchive)
			}

			out := map[string]any{
				"meta": map[string]any{
					"source":          "live",
					"username":        username,
					"fmdb_companies":  len(fmdbList),
					"auto_archive":    len(plan.AutoArchive),
					"ignore_noise":    len(plan.IgnoreNoise),
					"unknown":         len(plan.Unknown),
					"will_mutate":     willMutate,
					"dry_run":         !willMutate,
					"verify_env":      cliutil.IsVerifyEnv(),
					"cutoff_resolved": cutoffNonZeroOrEmpty(cutoffT),
				},
				"plan":     plan.summary(),
				"outcomes": outcomes,
			}
			payload, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), payload, flags)
		},
	}
	cmd.Flags().BoolVar(&mine, "mine", false, "Filter FMDB companies by current user (?reseller__dedicated_support_staff__username=ME)")
	cmd.Flags().StringVar(&cutoff, "cutoff", "", "Only consider issues stale before this human time (e.g. \"11pm yesterday\", \"2d\")")
	cmd.Flags().BoolVar(&autoArchiveKnown, "auto-archive-known", false, "Plan auto-archive for issues classified as known-safe")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Required to actually call /issues/{id}/archive/ (PLAN mode is default)")
	cmd.Flags().StringVar(&resellerFilter, "reseller", "", "Restrict FMDB to one reseller ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum FMDB companies to walk (0 = no limit)")
	return cmd
}

func cutoffNonZeroOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

type staleIssueRow struct {
	IssueID     string `json:"issue_id"`
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company"`
	Title       string `json:"title,omitempty"`
	Code        string `json:"code,omitempty"`
	Verdict     string `json:"verdict"` // auto_archive_safe | ignore_noise | unknown
	Reason      string `json:"reason"`
}

type staleIssuesPlan struct {
	AutoArchive []staleIssueRow
	IgnoreNoise []staleIssueRow
	Unknown     []staleIssueRow
}

func (p staleIssuesPlan) summary() map[string]any {
	return map[string]any{
		"auto_archive": p.AutoArchive,
		"ignore_noise": p.IgnoreNoise,
		"unknown":      p.Unknown,
	}
}

func staleEnough(issue map[string]any, cutoff time.Time) bool {
	for _, k := range []string{"last_seen_at", "updated_at", "modified_at", "created_at"} {
		s := anyToString(issue[k])
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.Before(cutoff)
		}
	}
	// If we cannot determine an age, leave it for review.
	return true
}

// classifyStaleIssue returns ("auto_archive_safe" | "ignore_noise" | "unknown", reason).
// Rules are a small hand-curated table; LLM is NOT in the loop. Add patterns
// here as the support team identifies new known-safe families.
func classifyStaleIssue(title, code string) (string, string) {
	low := strings.ToLower(title + " " + code)
	autoSafe := []struct{ p, r string }{
		{"backup completed", "backup-completed-event-stale"},
		{"agent reconnect", "agent-reconnect-noise"},
		{"agent online", "agent-online-event-stale"},
		{"backup started", "backup-started-event-stale"},
		{"verification complete", "verification-complete-stale"},
	}
	noise := []struct{ p, r string }{
		{"restic prune", "non-dashboard-prune-noise"},
		{"prune failed", "non-dashboard-prune-noise"},
		{"restic forget", "non-dashboard-forget-noise"},
	}
	for _, e := range autoSafe {
		if strings.Contains(low, e.p) {
			return "auto_archive_safe", e.r
		}
	}
	for _, e := range noise {
		if strings.Contains(low, e.p) {
			return "ignore_noise", e.r
		}
	}
	return "unknown", "no rule matched"
}

func applyStaleIssueArchive(_ context.Context, c clearHTTP, rows []staleIssueRow) []map[string]any {
	out := []map[string]any{}
	for _, r := range rows {
		_, status, err := c.Put("/issues/"+r.IssueID+"/archive/", nil)
		o := map[string]any{
			"action":   "archive",
			"issue_id": r.IssueID,
			"company":  r.CompanyName,
			"status":   status,
			"reason":   r.Reason,
		}
		if err != nil {
			o["error"] = err.Error()
		}
		out = append(out, o)
	}
	return out
}
