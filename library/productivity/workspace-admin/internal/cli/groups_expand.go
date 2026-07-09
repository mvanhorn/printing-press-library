// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: groups expand.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/workspace-admin/internal/cliutil"
)

// effectiveMember is one resolved direct user of a group after nested groups
// are flattened. ViaGroup names the immediate parent group the user was found
// under ("" when a direct member of the queried group); Depth is how many
// group hops from the queried group.
type effectiveMember struct {
	Email    string `json:"email"`
	ViaGroup string `json:"viaGroup"`
	Depth    int    `json:"depth"`
}

type groupsExpandView struct {
	Group         string            `json:"group"`
	Members       []effectiveMember `json:"members"`
	NestedGroups  []string          `json:"nestedGroups,omitempty"`
	CyclesSkipped []string          `json:"cyclesSkipped,omitempty"`
	MaxDepthHit   bool              `json:"maxDepthHit,omitempty"`
	ScannedGroups int               `json:"scannedGroups"`
	Note          string            `json:"note,omitempty"`
}

type directoryMember struct {
	Email string `json:"email"`
	Type  string `json:"type"`
	ID    string `json:"id"`
}

func newNovelGroupsExpandCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagMaxDepth int

	cmd := &cobra.Command{
		Use:   "expand <groupKey>",
		Short: "Recursively flatten nested groups into their effective direct-user membership, cycle-safe.",
		Long: "Resolve a group's effective direct-user membership by recursively expanding nested groups.\n" +
			"The Directory members.list endpoint returns nested groups un-expanded; this walks them\n" +
			"cycle-safe and reports each user with the immediate parent group it was found under.\n\n" +
			"Use this command to flatten a nested group into its effective direct-user membership.\n" +
			"Do NOT use it to list one user's group memberships; use 'audit user360' instead.",
		Example:     "  workspace-admin-pp-cli groups expand all-staff@example.com --agent --select members.email,members.viaGroup",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would recursively expand the group's nested membership")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a group email or ID is required"))
			}
			rootGroup := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			maxGroups := 500
			if cliutil.IsDogfoodEnv() && maxGroups > 25 {
				maxGroups = 25
			}

			fetch := func(groupKey string) ([]directoryMember, error) {
				return listGroupMembers(ctx, c, groupKey)
			}
			view, err := expandGroup(rootGroup, fetch, flagMaxDepth, flagLimit, maxGroups)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum effective users to return (0 = no limit)")
	cmd.Flags().IntVar(&flagMaxDepth, "max-depth", 0, "Maximum nesting depth to expand (0 = unbounded, cycle-guarded)")
	return cmd
}

// expandGroup performs the cycle-safe breadth-first expansion of a group into
// its effective direct users, using fetch to resolve each group's direct
// members. Pure over fetch so it is unit-testable without a live client.
// First parent wins for a user found via multiple paths; visited groups are
// recorded in CyclesSkipped rather than re-expanded.
func expandGroup(rootGroup string, fetch func(groupKey string) ([]directoryMember, error), maxDepth, limit, maxGroups int) (groupsExpandView, error) {
	view := groupsExpandView{Group: rootGroup}
	visited := map[string]bool{}  // groups already expanded (cycle guard)
	seenUser := map[string]bool{} // first parent wins for a given user
	nestedSet := map[string]bool{}
	type frame struct {
		group string
		depth int
	}
	queue := []frame{{group: rootGroup, depth: 0}}
	scanCapHit := false

	for len(queue) > 0 {
		if limit > 0 && len(view.Members) >= limit {
			break
		}
		if view.ScannedGroups >= maxGroups {
			scanCapHit = true
			break
		}
		f := queue[0]
		queue = queue[1:]
		if visited[f.group] {
			view.CyclesSkipped = append(view.CyclesSkipped, f.group)
			continue
		}
		if maxDepth > 0 && f.depth > maxDepth {
			view.MaxDepthHit = true
			continue
		}
		visited[f.group] = true
		view.ScannedGroups++

		members, merr := fetch(f.group)
		if merr != nil {
			return view, merr
		}
		reachedLimit := false
		for _, m := range members {
			switch m.Type {
			case "GROUP":
				child := m.Email
				if child == "" {
					child = m.ID
				}
				if child == "" {
					continue
				}
				if visited[child] {
					// Back-edge to an already-expanded group: a cycle. Record
					// it and do not re-expand.
					view.CyclesSkipped = append(view.CyclesSkipped, child)
					continue
				}
				if nestedSet[child] {
					// Already queued via another parent (shared subgroup, not a
					// cycle): don't enqueue a duplicate. Bounds queue growth.
					continue
				}
				nestedSet[child] = true
				view.NestedGroups = append(view.NestedGroups, child)
				queue = append(queue, frame{group: child, depth: f.depth + 1})
			case "USER":
				if m.Email == "" || seenUser[m.Email] {
					continue
				}
				if limit > 0 && len(view.Members) >= limit {
					reachedLimit = true
					break
				}
				seenUser[m.Email] = true
				// ViaGroup names the immediate group the user was found in;
				// blank for direct members of the queried root group.
				via := ""
				if f.group != rootGroup {
					via = f.group
				}
				view.Members = append(view.Members, effectiveMember{Email: m.Email, ViaGroup: via, Depth: f.depth})
			}
		}
		if reachedLimit {
			break
		}
	}

	sort.Slice(view.Members, func(i, j int) bool {
		if view.Members[i].Depth != view.Members[j].Depth {
			return view.Members[i].Depth < view.Members[j].Depth
		}
		return view.Members[i].Email < view.Members[j].Email
	})
	sort.Strings(view.NestedGroups)

	if scanCapHit {
		view.MaxDepthHit = true
		view.Note = fmt.Sprintf("scan capped at %d groups; narrow the group or run against a smaller subtree to see more", maxGroups)
	} else if len(view.Members) == 0 {
		view.Note = "no effective users found; the group may be empty or the key may be wrong"
	}
	return view, nil
}

// listGroupMembers fetches one group's direct members (users and nested
// groups) across all pages from the Directory members.list endpoint.
func listGroupMembers(ctx context.Context, c *client.Client, groupKey string) ([]directoryMember, error) {
	var out []directoryMember
	pageToken := ""
	for {
		params := map[string]string{"maxResults": "200"}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.Get(ctx, wsDirectoryBase+"/groups/"+groupKey+"/members", params)
		if err != nil {
			return nil, err
		}
		var env struct {
			NextPageToken string            `json:"nextPageToken"`
			Members       []directoryMember `json:"members"`
		}
		if json.Unmarshal(data, &env) != nil {
			break
		}
		out = append(out, env.Members...)
		if env.NextPageToken == "" || len(env.Members) == 0 {
			break
		}
		pageToken = env.NextPageToken
	}
	return out, nil
}
