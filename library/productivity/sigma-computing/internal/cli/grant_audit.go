// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: grant audit. Hand-filled scaffold.

// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// grantAuditRow is one effective member's access to a resource.
type grantAuditRow struct {
	Email      string `json:"email"`
	GrantedVia string `json:"grantedVia"` // "direct" or "team:<name>"
	Permission string `json:"permission"`
	OrgWide    bool   `json:"orgWide"`
}

var grantAuditResourceTypes = map[string]struct{}{
	"workbook":   {},
	"connection": {},
	"workspace":  {},
	"dataset":    {},
}

func newNovelGrantAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit [resource-type] [resource-id]",
		Short: "Expand all grants on a resource to the effective member list (direct + team-derived).",
		Example: strings.Trim(`
  sigma-computing-pp-cli grant audit workbook 2Xpsl5dB1qDproductMetrics
  sigma-computing-pp-cli grant audit connection 9aF3connId --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			resourceType := strings.ToLower(strings.TrimSpace(args[0]))
			resourceID := strings.TrimSpace(args[1])
			if _, ok := grantAuditResourceTypes[resourceType]; !ok {
				return fmt.Errorf("invalid [resource-type] %q: must be one of workbook, connection, workspace, dataset", args[0])
			}

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := auditGrants(db.DB(), resourceID)
			if err != nil {
				return fmt.Errorf("auditing grants for %s %s: %w", resourceType, resourceID, err)
			}

			if wantJSON(flags, cmd) {
				if rows == nil {
					rows = []grantAuditRow{}
				}
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"EMAIL", "GRANTED VIA", "PERMISSION", "ORG WIDE"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.Email, r.GrantedVia, r.Permission, fmt.Sprintf("%t", r.OrgWide)})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	return cmd
}

// auditGrants reads grants on inode_id == resourceID and expands every team
// grant into its individual members via the local teams_members join table.
// Direct member grants resolve member_id -> email. Each returned row records
// how the member got access (direct | team:<name>).
func auditGrants(db *sql.DB, resourceID string) ([]grantAuditRow, error) {
	rows, err := db.Query(
		`SELECT COALESCE(member_id,''), COALESCE(team_id,''), COALESCE(permission,''), data FROM grants WHERE inode_id = ?`,
		resourceID,
	)
	if err != nil {
		return nil, err
	}
	// Phase 1: drain the result set into plain structs BEFORE resolving any
	// emails/teams. memberEmailByID / teamMembersFromStore issue follow-up
	// queries on the same *sql.DB; calling them inside this open rows loop
	// deadlocks SQLite's single-connection pool.
	type rawGrant struct {
		memberID, teamID, permission string
		orgWide                      bool
	}
	var raws []rawGrant
	for rows.Next() {
		var memberID, teamID, permission string
		var raw []byte
		if err := rows.Scan(&memberID, &teamID, &permission, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		raws = append(raws, rawGrant{memberID, teamID, permission, grantIsOrgWide(raw, permission)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Phase 2: result set closed; safe to issue follow-up queries.
	var out []grantAuditRow
	for _, g := range raws {
		switch {
		case g.memberID != "":
			email := memberEmailByID(db, g.memberID)
			if email == "" {
				email = g.memberID
			}
			out = append(out, grantAuditRow{
				Email:      email,
				GrantedVia: "direct",
				Permission: g.permission,
				OrgWide:    g.orgWide,
			})
		case g.teamID != "":
			teamName := teamNameByID(db, g.teamID)
			if teamName == "" {
				teamName = g.teamID
			}
			members, err := teamMembersFromStore(db, g.teamID)
			if err != nil {
				return nil, err
			}
			for _, m := range members {
				email := m.Email
				if email == "" {
					email = m.ID
				}
				out = append(out, grantAuditRow{
					Email:      email,
					GrantedVia: "team:" + teamName,
					Permission: g.permission,
					OrgWide:    g.orgWide,
				})
			}
		default:
			// Org-wide / public grant with no member or team binding.
			out = append(out, grantAuditRow{
				Email:      "*",
				GrantedVia: "org-wide",
				Permission: g.permission,
				OrgWide:    true,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Email != out[j].Email {
			return out[i].Email < out[j].Email
		}
		return out[i].GrantedVia < out[j].GrantedVia
	})
	return out, nil
}

// grantIsOrgWide flags a grant whose scope is org-wide/public, either via an
// explicit public/org flag in the data blob or a permission keyword.
func grantIsOrgWide(raw []byte, permission string) bool {
	p := strings.ToLower(permission)
	if strings.Contains(p, "public") || strings.Contains(p, "org") {
		return true
	}
	if len(raw) == 0 {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	for _, k := range []string{"public", "isPublic", "orgWide", "isOrgWide", "everyone"} {
		if v, ok := obj[k]; ok {
			if b, ok := v.(bool); ok && b {
				return true
			}
		}
	}
	if s := firstString(obj, "scope", "grantScope", "visibility"); s != "" {
		ls := strings.ToLower(s)
		if strings.Contains(ls, "public") || strings.Contains(ls, "org") || strings.Contains(ls, "everyone") {
			return true
		}
	}
	return false
}
