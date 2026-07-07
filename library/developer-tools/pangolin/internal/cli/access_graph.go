// Copyright 2026 cfinney. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for pangolin-pp-cli.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/internal/store"
)

type accessEdge struct {
	UserID     string `json:"userId,omitempty"`
	UserEmail  string `json:"userEmail,omitempty"`
	RoleID     string `json:"roleId,omitempty"`
	RoleName   string `json:"roleName,omitempty"`
	ResourceID string `json:"resourceId,omitempty"`
	Resource   string `json:"resource,omitempty"`
	OrgID      string `json:"orgId,omitempty"`
}

func newAccessGraphCmd(flags *rootFlags) *cobra.Command {
	var userID, resourceID, orgID string
	cmd := &cobra.Command{
		Use:   "access-graph",
		Short: "Join users x roles x resources x orgs into a single 'who can reach what' view.",
		Long: `access-graph joins the local store's users, roles, and resources tables to
answer 'who can reach what'. Filter by --user, --resource, or --org to narrow.

Pangolin's API exposes each piece separately; no single endpoint returns the
joined view. Run 'sync --full' first.`,
		Example: "  pangolin-pp-cli access-graph --user user_42 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("pangolin-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Build the joined view: each role has resources attached; each
			// role is bound to users. The store mirrors these via the
			// /role/{roleId}/add/{userId} surface and /resource/{resourceId}/roles.
			// We approximate the join from the raw resources table.
			edges := []accessEdge{}

			// PATCH(access-graph-filter-warnings): make --user and --org filters
			// honest. Pangolin's user_role mapping table is only populated when
			// the API key has List Users + List Roles + List Allowed Role
			// Resources. When the prerequisite data isn't synced, --user would
			// silently return [] — warn instead of fabricating an answer.
			userMapRows := 0
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM resources WHERE resource_type IN ('user_role', 'role_user')`).Scan(&userMapRows)
			if userID != "" && userMapRows == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(),
					"warn: --user filter cannot be applied — no user-role mappings synced. "+
						"Sync users + roles + role-resource bindings first.")
			}

			// PATCH(access-graph-user-role-join): build the set of role IDs the
			// requested user holds, so --user can filter edges to only their roles.
			userRoles := map[string]bool{}
			if userID != "" && userMapRows > 0 {
				urrows, _ := db.DB().QueryContext(cmd.Context(),
					`SELECT COALESCE(json_extract(data, '$.roleId'), '')
					 FROM resources WHERE resource_type IN ('user_role', 'role_user')
					 AND (json_extract(data, '$.userId') = ? OR json_extract(data, '$.userEmail') = ?)`,
					userID, userID)
				if urrows != nil {
					for urrows.Next() {
						var roleID sql.NullString
						if urrows.Scan(&roleID) == nil && roleID.String != "" {
							userRoles[roleID.String] = true
						}
					}
					_ = urrows.Err()
					urrows.Close()
				}
			}

			// Pangolin's role->resource binding lives in resource_role rows
			// (each carries roleId + resourceId after sync via /resource/{id}/roles).
			// When users aren't synced (org-scoped read-only key without List Users),
			// we still emit role->resource edges so the user sees "which role unlocks what".

			// Apply --org filter by restricting the resource lookup to that org's
			// resources. Without this, --org was a no-op.
			resQuery := `SELECT id, COALESCE(json_extract(data, '$.name'), id) FROM resources WHERE resource_type IN ('resource', 'resources')`
			resQueryArgs := []any{}
			if orgID != "" {
				resQuery += ` AND (json_extract(data, '$.orgId') = ? OR json_extract(data, '$.orgName') = ?)`
				resQueryArgs = append(resQueryArgs, orgID, orgID)
			}
			resNames := map[string]string{}
			rnrows, _ := db.DB().QueryContext(cmd.Context(), resQuery, resQueryArgs...)
			if rnrows != nil {
				for rnrows.Next() {
					var id, name sql.NullString
					if rnrows.Scan(&id, &name) == nil {
						resNames[id.String] = name.String
					}
				}
				rnrows.Close()
			}

			roleNames := map[string]string{}
			rsrows, _ := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(json_extract(data, '$.name'), id) FROM resources WHERE resource_type IN ('role', 'roles', 'org_roles')`)
			if rsrows != nil {
				for rsrows.Next() {
					var id, name sql.NullString
					if rsrows.Scan(&id, &name) == nil {
						roleNames[id.String] = name.String
					}
				}
				rsrows.Close()
			}

			rrrows, _ := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(json_extract(data, '$.roleId'), ''),
				        COALESCE(json_extract(data, '$.__resourceId'), json_extract(data, '$.resourceId'), '')
				 FROM resources WHERE resource_type = 'resource_role'`)
			if rrrows != nil {
				for rrrows.Next() {
					var rid, resID sql.NullString
					if rrrows.Scan(&rid, &resID) != nil || rid.String == "" {
						continue
					}
					if resourceID != "" && resID.String != resourceID {
						continue
					}
					// --org filter: skip resource_role rows whose resource isn't in
					// our (already org-filtered) resNames map.
					if orgID != "" {
						if _, ok := resNames[resID.String]; !ok {
							continue
						}
					}
					if userID != "" {
						if userMapRows == 0 {
							// No mapping data synced — already warned; skip all edges.
							continue
						}
						// Filter to only roles the requested user holds.
						if !userRoles[rid.String] {
							continue
						}
					}
					edges = append(edges, accessEdge{
						RoleID:     rid.String,
						RoleName:   roleNames[rid.String],
						ResourceID: resID.String,
						Resource:   resNames[resID.String],
					})
				}
				_ = rrrows.Err()
				rrrows.Close()
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), edges, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Access graph: %d edges\n", len(edges))
			for _, e := range edges {
				fmt.Fprintf(cmd.OutOrStdout(), "  role:%s -> resource:%s\n",
					e.RoleName, e.Resource)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&userID, "user", "", "Filter by userId")
	cmd.Flags().StringVar(&resourceID, "resource", "", "Filter by resourceId")
	cmd.Flags().StringVar(&orgID, "org", "", "Filter by orgId")
	return cmd
}
