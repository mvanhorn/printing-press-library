// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// restoreOrder is the apply order: parents (orgs, idp, users) before
// dependents (sites, resources, targets) before bindings (role assignments).
var restoreOrder = []string{
	"orgs", "idp", "users",
	"sites", "resources", "site_resources",
	"target", "client", "role", "certificate", "domains",
	"org_users", "org_roles", "org_idp", "org_domains",
}

// restorePathFor maps a backup resource_type to the HTTP method + path used
// to (re-)create that record against a live Pangolin host. Returns "" + ""
// when the resource_type is not restorable through the integration API.
//
// NOTE: Pangolin create endpoints are PUT-shaped (PUT /org, PUT /org/{orgId}/site,
// etc.) and most require an org context. restore today is a best-effort skeleton
// for top-level creates that don't need parent context. See `.printing-press-patches.json`
// "restore-best-effort" for the full limitations note.
func restorePathFor(rt string) (string, string) {
	m := map[string]struct{ method, path string }{
		"orgs":      {"PUT", "/org"},
		"idp":       {"PUT", "/idp/oidc"},
		"sites":     {"PUT", "/org/{orgId}/site"},
		"resources": {"PUT", "/org/{orgId}/site/{siteId}/resource"},
		"target":    {"PUT", "/resource/{resourceId}/target"},
		"client":    {"PUT", "/org/{orgId}/client"},
		"role":      {"PUT", "/org/{orgId}/role"},
	}
	if e, ok := m[rt]; ok {
		return e.method, e.path
	}
	return "", ""
}

func newRestoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore [backup.json]",
		Short: "Re-apply top-level org and IdP records from a backup (other types require parent context and are skipped).",
		Long: `restore reads a backup file produced by 'backup' and POSTs records back to
the live Pangolin host. Currently only top-level resource types with static
API paths execute (orgs, idp). Types that require a parent ID in the path
(sites, resources, targets, and bindings) are skipped with a warning because
the restore command does not yet resolve those placeholders automatically.

Always run with --dry-run first to preview which types will execute and which
will be skipped. The command does NOT delete existing records — it only creates
the entries listed in the file. A real restore against a non-empty host will
surface duplicate-ID errors per record and continue with the rest.`,
		Example: "  pangolin-pp-cli restore pangolin-backup.json --dry-run",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			path := args[0]
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			var snap backupFile
			if err := json.Unmarshal(raw, &snap); err != nil {
				return fmt.Errorf("parsing backup: %w", err)
			}
			if snap.Schema != "pangolin-pp-cli/backup" {
				return fmt.Errorf("file %s is not a pangolin-pp-cli backup (schema=%q)", path, snap.Schema)
			}

			plan := []map[string]any{}
			for _, rt := range restoreOrder {
				items, ok := snap.ResourceSets[rt]
				if !ok || len(items) == 0 {
					continue
				}
				method, postPath := restorePathFor(rt)
				if postPath == "" {
					continue
				}
				plan = append(plan, map[string]any{
					"resource_type": rt,
					"method":        method,
					"post_path":     postPath,
					"records":       len(items),
					"will_execute":  !strings.Contains(postPath, "{"),
				})
			}

			if flags.dryRun {
				out := map[string]any{
					"dry_run":      true,
					"backup_file":  path,
					"generated_at": snap.GeneratedAt,
					"plan":         plan,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			applied := 0
			errored := 0
			skipped := 0
			for _, step := range plan {
				rt := step["resource_type"].(string)
				method := step["method"].(string)
				postPath := step["post_path"].(string)
				// PATCH(restore-honor-step-method): paths with unresolved {param}
				// placeholders need parent context the backup file does not provide;
				// skip with a warning rather than POST to a literal "{orgId}" path.
				if strings.Contains(postPath, "{") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: %s requires parent context (%s); skipping %d records\n",
						rt, postPath, len(snap.ResourceSets[rt]))
					skipped += len(snap.ResourceSets[rt])
					continue
				}
				items := snap.ResourceSets[rt]
				for _, item := range items {
					var body any
					if uerr := json.Unmarshal(item, &body); uerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: skipping corrupt record in %s: %v\n", rt, uerr)
						errored++
						continue
					}
					var perr error
					switch method {
					case "PUT":
						_, _, perr = c.Put(cmd.Context(), postPath, body)
					case "POST", "":
						_, _, perr = c.Post(cmd.Context(), postPath, body)
					default:
						perr = fmt.Errorf("unsupported method %q", method)
					}
					if perr != nil {
						errored++
						fmt.Fprintf(cmd.ErrOrStderr(), "warn: %s %s failed: %v\n", method, postPath, perr)
						continue
					}
					applied++
				}
			}

			result := map[string]any{
				"backup_file": path,
				"applied":     applied,
				"errored":     errored,
				"skipped":     skipped,
			}
			if errored > 0 {
				_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
				return fmt.Errorf("restore completed with %d errors", errored)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
