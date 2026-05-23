// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/cliutil"
)

// newTriageCmd builds the `triage` command: list open issues with filters,
// then batch-mutate them (ignore / archive / reactivate / comment) in one
// invocation. The web portal forces one-at-a-time; this command does many
// in a single call with --dry-run and typed exit codes.
func newTriageCmd(flags *rootFlags) *cobra.Command {
	var flagAudience string
	var flagCompany int
	var flagIgnore string
	var flagArchive string
	var flagReactivate string
	var flagComment string

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "List or batch-mutate open issues (ignore/archive/reactivate/comment)",
		Example: `  # List open support issues for company 4421
  servosity-msp-cli triage --audience support --company 4421

  # Ignore three issues and add a comment to each
  servosity-msp-cli triage --ignore 18,22,31 --comment "scheduled outage"`,
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			hasMutation := flagIgnore != "" || flagArchive != "" || flagReactivate != ""

			// Parse target IDs up front so input validation errors land on
			// exit code 2 before any network call.
			ignoreIDs, err := parseTriageIDs(flagIgnore, "ignore")
			if err != nil {
				return &cliError{code: 2, err: err}
			}
			archiveIDs, err := parseTriageIDs(flagArchive, "archive")
			if err != nil {
				return &cliError{code: 2, err: err}
			}
			reactivateIDs, err := parseTriageIDs(flagReactivate, "reactivate")
			if err != nil {
				return &cliError{code: 2, err: err}
			}

			filters := map[string]any{}
			if flagAudience != "" {
				filters["audience"] = flagAudience
			}
			if flagCompany != 0 {
				filters["company"] = flagCompany
			}

			// LIST mode: no mutation flags set.
			if !hasMutation {
				if dryRunOK(flags) {
					if flags.asJSON {
						envelope := map[string]any{
							"operation": "triage",
							"mode":      "list",
							"filters":   filters,
							"dry_run":   true,
						}
						b, _ := json.Marshal(envelope)
						return printOutput(cmd.OutOrStdout(), json.RawMessage(b), true)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would list open issues with filters: %+v\n", filters)
					return nil
				}

				c, err := flags.newClient()
				if err != nil {
					return err
				}
				// Partner tokens cannot hit /issues/ globally — that
				// endpoint is admin-only and returns 403. Resolve the
				// caller's reseller ID and hit /resellers/{id}/issues/.
				resellerID, err := cliutil.ResolveResellerID(cmd.Context(), c)
				if err != nil {
					return fmt.Errorf("resolving reseller ID: %w", err)
				}
				params := map[string]string{}
				if flagAudience != "" {
					params["audience"] = flagAudience
				}
				if flagCompany != 0 {
					params["company"] = strconv.Itoa(flagCompany)
				}
				listPath := fmt.Sprintf("/resellers/%d/issues/", resellerID)
				data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "issues", listPath, params, nil, false, "cursor", "", "")
				if err != nil {
					return classifyAPIError(err, flags)
				}

				var items []json.RawMessage
				_ = json.Unmarshal(data, &items)
				listCount := len(items)

				if flags.asJSON {
					var parsed any
					_ = json.Unmarshal(data, &parsed)
					envelope := map[string]any{
						"operation":  "triage",
						"mode":       "list",
						"filters":    filters,
						"list_count": listCount,
						"issues":     parsed,
					}
					b, mErr := json.Marshal(envelope)
					if mErr != nil {
						return mErr
					}
					wrapped, wErr := wrapWithProvenance(json.RawMessage(b), prov)
					if wErr != nil {
						return wErr
					}
					return printOutput(cmd.OutOrStdout(), wrapped, true)
				}

				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					printProvenance(cmd, listCount, prov)
					var rows []map[string]any
					if json.Unmarshal(data, &rows) == nil && len(rows) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), rows); err == nil {
							return nil
						}
					}
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			// MUTATE mode.
			type result struct {
				Action  string `json:"action"`
				IssueID int    `json:"issue_id"`
				OK      bool   `json:"ok"`
				Comment string `json:"comment,omitempty"`
				Error   string `json:"error,omitempty"`
			}

			results := []result{}
			summary := map[string]int{
				"ignored": 0, "archived": 0, "reactivated": 0, "commented": 0, "failed": 0,
			}

			// Dry-run: do not call the network. Report what would happen.
			if dryRunOK(flags) {
				for _, id := range ignoreIDs {
					results = append(results, result{Action: "ignore", IssueID: id, OK: true})
				}
				for _, id := range archiveIDs {
					results = append(results, result{Action: "archive", IssueID: id, OK: true})
				}
				for _, id := range reactivateIDs {
					results = append(results, result{Action: "reactivate", IssueID: id, OK: true})
				}
				if flagComment != "" {
					targets := uniqueInts(ignoreIDs, archiveIDs, reactivateIDs)
					for _, id := range targets {
						results = append(results, result{Action: "comment", IssueID: id, OK: true, Comment: flagComment})
					}
				}

				if flags.asJSON {
					envelope := map[string]any{
						"operation": "triage",
						"mode":      "mutate",
						"filters":   filters,
						"mutations": results,
						"summary": map[string]int{
							"ignored":     len(ignoreIDs),
							"archived":    len(archiveIDs),
							"reactivated": len(reactivateIDs),
							"commented":   len(uniqueInts(ignoreIDs, archiveIDs, reactivateIDs)) * boolToInt(flagComment != ""),
							"failed":      0,
						},
						"dry_run": true,
					}
					b, _ := json.Marshal(envelope)
					return printOutput(cmd.OutOrStdout(), json.RawMessage(b), true)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would ignore=%d archive=%d reactivate=%d comment=%q\n",
					len(ignoreIDs), len(archiveIDs), len(reactivateIDs), flagComment)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			mutationTotal := 0
			mutationFailures := 0

			runMutation := func(action, pathTemplate string, id int) {
				mutationTotal++
				path := replacePathParam(pathTemplate, "id", strconv.Itoa(id))
				_, _, err := c.PutWithParams(path, map[string]string{}, map[string]any{})
				r := result{Action: action, IssueID: id, OK: err == nil}
				if err != nil {
					r.Error = err.Error()
					mutationFailures++
					summary["failed"]++
				} else {
					switch action {
					case "ignore":
						summary["ignored"]++
					case "archive":
						summary["archived"]++
					case "reactivate":
						summary["reactivated"]++
					}
				}
				results = append(results, r)
			}

			for _, id := range ignoreIDs {
				runMutation("ignore", "/issues/{id}/ignore/", id)
			}
			for _, id := range archiveIDs {
				runMutation("archive", "/issues/{id}/archive/", id)
			}
			for _, id := range reactivateIDs {
				runMutation("reactivate", "/issues/{id}/reactivate/", id)
			}

			if flagComment != "" {
				targets := uniqueInts(ignoreIDs, archiveIDs, reactivateIDs)
				for _, id := range targets {
					mutationTotal++
					path := replacePathParam("/issues/{id}/comments/", "id", strconv.Itoa(id))
					body := map[string]any{"body": flagComment}
					_, _, err := c.PutWithParams(path, map[string]string{}, body)
					r := result{Action: "comment", IssueID: id, OK: err == nil, Comment: flagComment}
					if err != nil {
						r.Error = err.Error()
						mutationFailures++
						summary["failed"]++
					} else {
						summary["commented"]++
					}
					results = append(results, r)
				}
			}

			// Render output.
			if flags.asJSON {
				envelope := map[string]any{
					"operation": "triage",
					"mode":      "mutate",
					"filters":   filters,
					"mutations": results,
					"summary":   summary,
				}
				b, mErr := json.Marshal(envelope)
				if mErr != nil {
					return mErr
				}
				if perr := printOutput(cmd.OutOrStdout(), json.RawMessage(b), true); perr != nil {
					return perr
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Ignored: %d issues. Archived: %d. Reactivated: %d. Commented: %d issues",
					summary["ignored"], summary["archived"], summary["reactivated"], summary["commented"])
				if flagComment != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " with: %q", flagComment)
				}
				fmt.Fprintln(cmd.OutOrStdout(), ".")
				if summary["failed"] > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Failed: %d.\n", summary["failed"])
				}
			}

			// Exit-code policy: 0 on full success, 4 partial, 5 all-failed.
			if mutationTotal > 0 && mutationFailures == mutationTotal {
				return &cliError{code: 5, err: fmt.Errorf("all %d triage mutations failed", mutationTotal)}
			}
			if mutationFailures > 0 {
				return &cliError{code: 4, err: fmt.Errorf("%d of %d triage mutations failed", mutationFailures, mutationTotal)}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAudience, "audience", "", "Filter list by audience (e.g. support, MSP)")
	cmd.Flags().IntVar(&flagCompany, "company", 0, "Filter list by company ID")
	cmd.Flags().StringVar(&flagIgnore, "ignore", "", "Comma-separated issue IDs to mark ignored")
	cmd.Flags().StringVar(&flagArchive, "archive", "", "Comma-separated issue IDs to archive")
	cmd.Flags().StringVar(&flagReactivate, "reactivate", "", "Comma-separated issue IDs to reactivate")
	cmd.Flags().StringVar(&flagComment, "comment", "", "Comment to add to every issue named in --ignore/--archive/--reactivate")

	return cmd
}

// parseTriageIDs splits a comma-separated list of integer issue IDs. Empty
// input returns a nil slice (no IDs). A non-integer token returns an error
// that the caller wraps as exit-code 2 (input validation).
func parseTriageIDs(raw, flagName string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("--%s: %q is not an integer issue ID", flagName, s)
		}
		out = append(out, n)
	}
	return out, nil
}

// uniqueInts returns a stable de-duplicated union of the given slices,
// preserving first-seen order. Used to compute the comment target set when
// the same ID appears in multiple mutation flags.
func uniqueInts(groups ...[]int) []int {
	seen := map[int]struct{}{}
	out := []int{}
	for _, g := range groups {
		for _, n := range g {
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
