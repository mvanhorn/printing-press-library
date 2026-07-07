// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// extractIDField pulls a string or numeric ID field from a JSON response.
// Looks at the top level first, then peeks one level into a "data" envelope
// for JSend-style responses ({success, data: {resourceId: 12, ...}}).
// Returns "" on any parse failure or missing field — callers fall back to
// the next candidate.
func extractIDField(raw []byte, names ...string) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if id := lookupID(m, names); id != "" {
		return id
	}
	// PATCH(jsend-envelope-unwrap-expose): peek into nested-data envelope.
	for _, outer := range []string{"data", "Data", "result", "Result"} {
		if inner, ok := m[outer].(map[string]any); ok {
			if id := lookupID(inner, names); id != "" {
				return id
			}
		}
	}
	return ""
}

func lookupID(m map[string]any, names []string) string {
	for _, n := range names {
		switch v := m[n].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	return ""
}

// exposeStep describes one mutation we'd run against the live API. Each step
// is materialised when the user drops --dry-run; with --dry-run the full plan
// is returned without any side effects.
type exposeStep struct {
	Order  int    `json:"order"`
	Action string `json:"action"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   any    `json:"body,omitempty"`
}

func newExposeCmd(flags *rootFlags) *cobra.Command {
	var (
		target, siteID, roleID, orgID, niceID, fullDomain string
		ssl, http                                          bool
	)
	cmd := &cobra.Command{
		Use:   "expose [niceId]",
		Short: "One-shot: create a resource + target + role binding for a homelab service.",
		Long: `expose plans (and optionally executes) the create-resource + add-target +
add-role chain that the dashboard otherwise spreads across 3-5 tabs.

Required: --target host:port (where the upstream lives) and either --site or
an existing org context. Always start with --dry-run to inspect the plan.`,
		Example: "  pangolin-pp-cli expose grafana --target 192.168.1.50:3000 --site site_42 --role admins --dry-run",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			nice := args[0]
			if niceID != "" {
				nice = niceID
			}
			if target == "" {
				return usageErr(fmt.Errorf("--target host:port is required"))
			}
			if siteID == "" && orgID == "" {
				return usageErr(fmt.Errorf("either --site or --org is required"))
			}
			// PATCH(expose-org-required-with-site): --org is required when --site is
			// given because the API path is /org/{orgId}/site/{siteId}/resource.
			// Without --org the path would contain the literal placeholder "<orgId>".
			if siteID != "" && orgID == "" {
				return usageErr(fmt.Errorf("--org is required when --site is provided (API path requires orgId)"))
			}

			plan := []exposeStep{}
			order := 1

			resourceBody := map[string]any{
				"niceId": nice,
				"ssl":    ssl,
				"http":   http,
			}
			if fullDomain != "" {
				resourceBody["fullDomain"] = fullDomain
			}

			if siteID != "" {
				plan = append(plan, exposeStep{
					Order: order, Action: "create_resource_under_site",
					Method: "PUT",
					Path:   fmt.Sprintf("/org/%s/site/%s/resource", orgValueOr(orgID, "<orgId>"), siteID),
					Body:   resourceBody,
				})
				order++
			} else {
				plan = append(plan, exposeStep{
					Order: order, Action: "create_resource_under_org",
					Method: "PUT",
					Path:   fmt.Sprintf("/org/%s/site-resource", orgID),
					Body:   resourceBody,
				})
				order++
			}

			plan = append(plan, exposeStep{
				Order: order, Action: "attach_target",
				Method: "PUT",
				Path:   "/resource/{resourceId}/target",
				Body:   map[string]any{"target": target},
			})
			order++

			if roleID != "" {
				plan = append(plan, exposeStep{
					Order: order, Action: "bind_role",
					Method: "POST",
					Path:   "/resource/{resourceId}/roles",
					Body:   map[string]any{"roleId": roleID},
				})
				order++
			}

			if flags.dryRun || dryRunOK(flags) {
				out := map[string]any{
					"dry_run": true,
					"niceId":  nice,
					"target":  target,
					"plan":    plan,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Live mode: walk the plan against the API. Resource ID returned
			// from step 1 is plugged into the {resourceId} placeholder in the
			// remaining steps. We bail at the first hard failure rather than
			// continuing past a missing resourceId.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := []map[string]any{}
			var resourceID string
			for _, step := range plan {
				path := step.Path
				if resourceID != "" {
					path = replacePathParam(path, "resourceId", resourceID)
				}
				// PATCH(expose-resourceid-guard): if path still contains the literal
				// {resourceId} placeholder, step 1 did not return a parseable ID —
				// fail early with a clear message rather than sending a malformed URL.
				if strings.Contains(path, "{resourceId}") {
					return fmt.Errorf("expose: step %d (%s) requires {resourceId} but resource ID was not captured from step 1; use --dry-run to inspect the plan", step.Order, step.Action)
				}
				// PATCH(expose-honor-step-method): use step.Method, not always POST.
				var resp []byte
				var perr error
				switch step.Method {
				case "PUT":
					resp, _, perr = c.Put(cmd.Context(), path, step.Body)
				case "POST", "":
					resp, _, perr = c.Post(cmd.Context(), path, step.Body)
				default:
					perr = fmt.Errorf("unsupported step method %q", step.Method)
				}
				if perr != nil {
					results = append(results, map[string]any{
						"order": step.Order, "action": step.Action, "ok": false, "error": perr.Error(),
					})
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"steps": results}, flags)
					return fmt.Errorf("expose failed at step %d (%s): %w", step.Order, step.Action, perr)
				}
				results = append(results, map[string]any{
					"order": step.Order, "action": step.Action, "ok": true, "response": string(resp),
				})
				if step.Action == "create_resource_under_site" || step.Action == "create_resource_under_org" {
					if id := extractIDField(resp, "resourceId", "id"); id != "" {
						resourceID = id
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"steps": results, "resource_id": resourceID}, flags)
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "Upstream target host:port (e.g. 192.168.1.50:3000)")
	cmd.Flags().StringVar(&siteID, "site", "", "Existing site ID to attach the resource to")
	cmd.Flags().StringVar(&roleID, "role", "", "Existing role ID to bind to the new resource (optional)")
	cmd.Flags().StringVar(&orgID, "org", "", "Org ID context (required)")
	cmd.Flags().StringVar(&niceID, "nice-id", "", "Override the niceId positional (subdomain shorthand)")
	cmd.Flags().StringVar(&fullDomain, "domain", "", "Full domain (e.g. grafana.example.com)")
	cmd.Flags().BoolVar(&ssl, "ssl", true, "Enable SSL on the new resource")
	cmd.Flags().BoolVar(&http, "http", true, "Enable HTTP on the new resource")
	return cmd
}

func orgValueOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
