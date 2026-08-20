// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/store"
	"github.com/spf13/cobra"
)

// adUnitNode is one node in the ad-unit hierarchy. The JSON shape doubles as
// the --json/--agent output: a node plus its nested children.
type adUnitNode struct {
	AdUnitID string        `json:"ad_unit_id"`
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	ParentID string        `json:"-"`
	Children []*adUnitNode `json:"children"`
}

// adUnitNameToID extracts the trailing id segment from a GAM AdUnit resource
// name ("networks/{network_code}/adUnits/{ad_unit_id}" -> "{ad_unit_id}").
// A bare id (no slash) is returned unchanged so callers can pass either form.
func adUnitNameToID(resourceName string) string {
	if resourceName == "" {
		return ""
	}
	if i := strings.LastIndex(resourceName, "/"); i >= 0 {
		return resourceName[i+1:]
	}
	return resourceName
}

// buildAdUnitTree assembles a forest from a flat slice of ad units using each
// node's ParentID linkage, then returns the roots. When root is non-empty it
// returns just the subtree(s) whose node id (or full resource name) matches
// root. Nodes whose parent is not present in the input become roots so no unit
// is dropped. Children are sorted by id for deterministic output. The input
// nodes are mutated (Children populated); callers pass freshly built nodes.
func buildAdUnitTree(units []adUnitNode, root string) []*adUnitNode {
	byID := make(map[string]*adUnitNode, len(units))
	order := make([]string, 0, len(units))
	for i := range units {
		n := &units[i]
		if n.Children == nil {
			n.Children = make([]*adUnitNode, 0)
		}
		byID[n.AdUnitID] = n
		order = append(order, n.AdUnitID)
	}

	roots := make([]*adUnitNode, 0)
	for _, id := range order {
		n := byID[id]
		parent, ok := byID[n.ParentID]
		if ok && parent != n {
			parent.Children = append(parent.Children, n)
		} else {
			roots = append(roots, n)
		}
	}

	sortAdUnitChildren(roots)

	if root == "" {
		return roots
	}

	wantID := adUnitNameToID(root)
	if n, ok := byID[wantID]; ok {
		return []*adUnitNode{n}
	}
	// No matching subtree: return an empty forest rather than the whole tree.
	return make([]*adUnitNode, 0)
}

// sortAdUnitChildren recursively orders every child slice by ad unit id so the
// rendered tree is stable across runs (map iteration order is not).
func sortAdUnitChildren(nodes []*adUnitNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].AdUnitID < nodes[j].AdUnitID })
	for _, n := range nodes {
		sortAdUnitChildren(n.Children)
	}
}

// filterAdUnitTreeStatus prunes the forest to nodes whose status equals want
// (case-insensitive). A node is kept if it matches OR any descendant matches,
// so ancestors of a matching node remain to preserve the path. Returns a new
// forest; input nodes are not mutated beyond their existing Children.
func filterAdUnitTreeStatus(nodes []*adUnitNode, want string) []*adUnitNode {
	if want == "" {
		return nodes
	}
	want = strings.ToUpper(want)
	out := make([]*adUnitNode, 0, len(nodes))
	for _, n := range nodes {
		keptChildren := filterAdUnitTreeStatus(n.Children, want)
		if strings.ToUpper(n.Status) == want || len(keptChildren) > 0 {
			clone := *n
			clone.Children = keptChildren
			out = append(out, &clone)
		}
	}
	return out
}

// renderAdUnitTree writes an indented text tree for human output.
func renderAdUnitTree(sb *strings.Builder, nodes []*adUnitNode, depth int) {
	for _, n := range nodes {
		indent := strings.Repeat("  ", depth)
		name := n.Name
		if name == "" {
			name = "(unnamed)"
		}
		status := n.Status
		if status == "" {
			status = "-"
		}
		fmt.Fprintf(sb, "%s%s  %s  [%s]\n", indent, n.AdUnitID, name, status)
		renderAdUnitTree(sb, n.Children, depth+1)
	}
}

// pp:data-source local -- renders the ad-unit tree from the locally mirrored
// store; run `sync` first to populate or refresh the mirror.
func newNovelAdunitsTreeCmd(flags *rootFlags) *cobra.Command {
	var flagRoot string
	var flagStatus string
	var flagNetwork string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Render the hierarchical ad-unit tree (or any subtree), with status and depth, in one call.",
		Long: strings.Trim(`
Render the Google Ad Manager ad-unit hierarchy as a tree.

Reads the local mirror; if the mirror is empty, fetches ad units live via
--network (or $GOOGLE_AD_MANAGER_NETWORK_CODE) and caches them for next time.
--root scopes to a subtree by ad unit id or resource name; --status keeps only
units with a given status (ancestors of matches are retained).`, "\n"),
		Example:     "  google-ad-manager-pp-cli adunits tree --root 21700000 --status ACTIVE --network 123456",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would load ad-units (live via --network if no local mirror) and build the hierarchy tree")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Non-fatal: empty network code means mirror-only.
			networkCode, _ := resolveNetworkCode(flagNetwork)
			maxPages := 25
			if cliutil.IsDogfoodEnv() {
				maxPages = 2
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("google-ad-manager-pp-cli")
			}
			st, stErr := store.OpenWithContext(ctx, dbPath)
			if stErr == nil {
				defer st.Close()
			} else {
				st = nil // live-only, no cache
			}

			units, err := loadAdUnitNodes(ctx, flags, st, networkCode, maxPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(units) == 0 && networkCode == "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror and no network code set.\npass --network <code> or set GOOGLE_AD_MANAGER_NETWORK_CODE to fetch live.\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			forest := buildAdUnitTree(units, flagRoot)
			forest = filterAdUnitTreeStatus(forest, flagStatus)

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), forest, flags)
			}
			var sb strings.Builder
			if len(forest) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no ad units matched")
				return nil
			}
			renderAdUnitTree(&sb, forest, 0)
			fmt.Fprint(cmd.OutOrStdout(), sb.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&flagRoot, "root", "", "Ad unit id (or resource name) to use as the subtree root; default is the whole forest")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Keep only ad units with this status (ACTIVE, INACTIVE, ARCHIVED); ancestors of matches are retained")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (defaults to GOOGLE_AD_MANAGER_NETWORK_CODE); used for the sync hint")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: platform data dir)")
	return cmd
}

// loadAdUnitNodes returns every ad unit (mirror first, live fallback via
// gamLoadResource) projected into the flat node slice buildAdUnitTree consumes.
// The id is the trailing segment of the resource name; displayName is the human
// label; parentAdUnit is the parent's resource name (empty for the root unit
// Google creates).
func loadAdUnitNodes(ctx context.Context, flags *rootFlags, st *store.Store, networkCode string, maxPages int) ([]adUnitNode, error) {
	blobs, _, err := gamLoadResource(ctx, flags, st, networkCode, "ad-units", "adUnits", maxPages)
	if err != nil {
		return nil, err
	}
	out := make([]adUnitNode, 0, len(blobs))
	for _, data := range blobs {
		var au struct {
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			Status       string `json:"status"`
			ParentAdUnit string `json:"parentAdUnit"`
		}
		if err := json.Unmarshal(data, &au); err != nil {
			continue
		}
		out = append(out, adUnitNode{
			AdUnitID: adUnitNameToID(au.Name),
			Name:     au.DisplayName,
			Status:   au.Status,
			ParentID: adUnitNameToID(au.ParentAdUnit),
		})
	}
	return out, nil
}
