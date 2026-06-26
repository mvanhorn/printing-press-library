// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: access review. Hand-filled scaffold.

// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// accessReviewRow is one resource a member can reach.
type accessReviewRow struct {
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	ResourceID   string `json:"resourceId"`
	Permission   string `json:"permission"`
	Via          string `json:"via"` // "direct" or "team:<name>"
}

func newNovelAccessReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review [member-email]",
		Short: "Show everything one member can reach across workbooks, connections, and workspaces via direct and team grants.",
		Example: strings.Trim(`
  sigma-computing-pp-cli access review analyst@acme.com
  sigma-computing-pp-cli access review analyst@acme.com --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				return cmd.Help()
			}
			email := strings.TrimSpace(args[0])

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			ref, found, err := resolveMemberByEmail(db.DB(), email)
			if err != nil {
				return fmt.Errorf("resolving member %q: %w", email, err)
			}
			if !found {
				return fmt.Errorf("no member found for email %q in the local store; run 'sigma-computing-pp-cli sync' or check the address", email)
			}

			rows, err := reviewAccess(db.DB(), ref.ID)
			if err != nil {
				return fmt.Errorf("reviewing access for %q: %w", email, err)
			}

			if wantJSON(flags, cmd) {
				if rows == nil {
					rows = []accessReviewRow{}
				}
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"RESOURCE TYPE", "RESOURCE NAME", "PERMISSION", "VIA"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{r.ResourceType, r.ResourceName, r.Permission, r.Via})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	return cmd
}

// reviewAccess returns every resource a member can reach: direct grants
// (grants.member_id == memberID) plus grants held by any team the member
// belongs to (grants.team_id IN member's teams). Resources are resolved to a
// name/type via the synced resource tables.
func reviewAccess(db *sql.DB, memberID string) ([]accessReviewRow, error) {
	var out []accessReviewRow

	// A grant edge drained from a result set before any resolveResource call.
	// resolveResource issues follow-up QueryRows on the same *sql.DB; running
	// it inside an open rows loop deadlocks SQLite's single-connection pool, so
	// every result set is fully drained into these structs first.
	type grantEdge struct {
		inodeID, inodeType, perm, via string
	}
	var edges []grantEdge

	drain := func(rows *sql.Rows, via string) error {
		defer rows.Close()
		for rows.Next() {
			var inodeID, inodeType, perm string
			if err := rows.Scan(&inodeID, &inodeType, &perm); err != nil {
				return err
			}
			edges = append(edges, grantEdge{inodeID, inodeType, perm, via})
		}
		return rows.Err()
	}

	// Direct grants.
	direct, err := db.Query(
		`SELECT COALESCE(inode_id,''), COALESCE(inode_type,''), COALESCE(permission,'') FROM grants WHERE member_id = ?`,
		memberID,
	)
	if err != nil {
		return nil, err
	}
	if err := drain(direct, "direct"); err != nil {
		return nil, err
	}

	// Team grants.
	teamIDs, err := memberTeamIDs(db, memberID)
	if err != nil {
		return nil, err
	}
	for _, teamID := range teamIDs {
		teamName := teamNameByID(db, teamID)
		if teamName == "" {
			teamName = teamID
		}
		tg, err := db.Query(
			`SELECT COALESCE(inode_id,''), COALESCE(inode_type,''), COALESCE(permission,'') FROM grants WHERE team_id = ?`,
			teamID,
		)
		if err != nil {
			return nil, err
		}
		if err := drain(tg, "team:"+teamName); err != nil {
			return nil, err
		}
	}

	// All result sets closed; now resolve resource names safely.
	for _, e := range edges {
		rt, name := resolveResource(db, e.inodeID, e.inodeType)
		out = append(out, accessReviewRow{
			ResourceType: rt,
			ResourceName: name,
			ResourceID:   e.inodeID,
			Permission:   e.perm,
			Via:          e.via,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ResourceType != out[j].ResourceType {
			return out[i].ResourceType < out[j].ResourceType
		}
		if out[i].ResourceName != out[j].ResourceName {
			return out[i].ResourceName < out[j].ResourceName
		}
		return out[i].Via < out[j].Via
	})
	return out, nil
}

// resolveResource resolves an inode id to a (type, name). It honors the grant's
// declared inode_type when present, otherwise probes the resource tables.
func resolveResource(db *sql.DB, inodeID, inodeType string) (string, string) {
	tableForType := map[string]string{
		"workbook":   "workbooks",
		"connection": "connections",
		"workspace":  "workspaces",
	}
	lookup := func(table string) (string, bool) {
		var name string
		row := db.QueryRow(`SELECT COALESCE(name,'') FROM `+table+` WHERE id = ? LIMIT 1`, inodeID)
		if err := row.Scan(&name); err != nil {
			return "", false
		}
		return name, true
	}

	if inodeType != "" {
		if table, ok := tableForType[strings.ToLower(inodeType)]; ok {
			if name, ok := lookup(table); ok {
				return strings.ToLower(inodeType), name
			}
		}
	}
	for typ, table := range tableForType {
		if name, ok := lookup(table); ok {
			return typ, name
		}
	}
	if inodeType == "" {
		inodeType = "unknown"
	}
	return strings.ToLower(inodeType), inodeID
}
