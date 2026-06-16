// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/store"
	"github.com/spf13/cobra"
)

// orphan is one ACTIVE ad unit covered by no placement.
type orphan struct {
	AdUnitID string `json:"ad_unit_id"`
	Name     string `json:"name"`
	Reason   string `json:"reason"`
}

// orphanAdUnit is the minimal ad-unit projection findOrphans needs.
type orphanAdUnit struct {
	ID       string
	Name     string
	Status   string
	ParentID string
}

// orphanReport is the command's machine output.
type orphanReport struct {
	Orphans           []orphan `json:"orphans"`
	ScannedAdUnits    int      `json:"scanned_ad_units"`
	ScannedPlacements int      `json:"scanned_placements"`
}

// findOrphans returns every ACTIVE ad unit whose id is referenced by no
// placement. placementAdUnitIDs is the set of ad unit ids (already extracted
// from each placement's targetedAdUnits) that at least one placement covers.
// Only ACTIVE units can be orphans: INACTIVE/ARCHIVED units are expected to be
// uncovered. Results are sorted by id for stable output.
func findOrphans(units []orphanAdUnit, placementAdUnitIDs map[string]bool) []orphan {
	out := make([]orphan, 0)
	for _, u := range units {
		if u.Status != "ACTIVE" {
			continue
		}
		if placementAdUnitIDs[u.ID] {
			continue
		}
		name := u.Name
		out = append(out, orphan{
			AdUnitID: u.ID,
			Name:     name,
			Reason:   "active ad unit not referenced by any placement",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AdUnitID < out[j].AdUnitID })
	return out
}

// subtreeIDs returns the set of ad unit ids in the subtree rooted at rootID
// (inclusive), walking the parent linkage. When rootID is empty every id is
// returned. Used to scope orphan scanning to --root.
func subtreeIDs(units []orphanAdUnit, rootID string) map[string]bool {
	all := make(map[string]bool, len(units))
	for _, u := range units {
		all[u.ID] = true
	}
	if rootID == "" {
		return all
	}
	childrenOf := make(map[string][]string)
	for _, u := range units {
		childrenOf[u.ParentID] = append(childrenOf[u.ParentID], u.ID)
	}
	keep := make(map[string]bool)
	stack := []string{rootID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if keep[id] || !all[id] {
			continue
		}
		keep[id] = true
		stack = append(stack, childrenOf[id]...)
	}
	return keep
}

// pp:data-source local -- tree-walks the mirrored ad-unit and placement tables
// in the local store; run `sync` first to populate or refresh the mirror.
func newNovelInventoryOrphansCmd(flags *rootFlags) *cobra.Command {
	var flagRoot string
	var flagNetwork string
	var flagDB string

	cmd := &cobra.Command{
		Use:         "orphans",
		Short:       "Tree-walk the ad-unit hierarchy to flag active units with no placement coverage.",
		Example:     "  google-ad-manager-pp-cli inventory orphans --root 21700000 --network 123456 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would load ad-units and placements (live via --network if no local mirror), then flag uncovered active units")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

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
				st = nil
			}

			units, err := loadOrphanAdUnits(ctx, flags, st, networkCode, maxPages)
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
			covered, placementCount, err := loadPlacementAdUnitIDs(ctx, flags, st, networkCode, maxPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			scanUnits := units
			if flagRoot != "" {
				keep := subtreeIDs(units, adUnitNameToID(flagRoot))
				scanUnits = make([]orphanAdUnit, 0, len(units))
				for _, u := range units {
					if keep[u.ID] {
						scanUnits = append(scanUnits, u)
					}
				}
			}

			report := orphanReport{
				Orphans:           findOrphans(scanUnits, covered),
				ScannedAdUnits:    len(scanUnits),
				ScannedPlacements: placementCount,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(report.Orphans) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no orphans (scanned %d ad units, %d placements)\n", report.ScannedAdUnits, report.ScannedPlacements)
				return nil
			}
			for _, o := range report.Orphans {
				name := o.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  — %s\n", o.AdUnitID, name, o.Reason)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d orphan(s) of %d ad units scanned (%d placements)\n", len(report.Orphans), report.ScannedAdUnits, report.ScannedPlacements)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagRoot, "root", "", "Limit the scan to the subtree rooted at this ad unit id (or resource name)")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "GAM network code (defaults to GOOGLE_AD_MANAGER_NETWORK_CODE); used for the sync hint")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: platform data dir)")
	return cmd
}

// loadOrphanAdUnits projects ad units (mirror first, live fallback) into the
// orphan scanner shape.
func loadOrphanAdUnits(ctx context.Context, flags *rootFlags, st *store.Store, networkCode string, maxPages int) ([]orphanAdUnit, error) {
	blobs, _, err := gamLoadResource(ctx, flags, st, networkCode, "ad-units", "adUnits", maxPages)
	if err != nil {
		return nil, err
	}
	out := make([]orphanAdUnit, 0, len(blobs))
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
		out = append(out, orphanAdUnit{
			ID:       adUnitNameToID(au.Name),
			Name:     au.DisplayName,
			Status:   au.Status,
			ParentID: adUnitNameToID(au.ParentAdUnit),
		})
	}
	return out, nil
}

// loadPlacementAdUnitIDs returns the set of ad unit ids referenced by any
// placement (via its targetedAdUnits resource-name list) plus the placement
// count scanned. The GAM Placement field is "targetedAdUnits" (array of ad
// unit resource names), not "targetedAdUnitIds".
func loadPlacementAdUnitIDs(ctx context.Context, flags *rootFlags, st *store.Store, networkCode string, maxPages int) (map[string]bool, int, error) {
	blobs, _, err := gamLoadResource(ctx, flags, st, networkCode, "placements", "placements", maxPages)
	if err != nil {
		return nil, 0, err
	}
	covered := make(map[string]bool)
	for _, data := range blobs {
		var p struct {
			TargetedAdUnits []string `json:"targetedAdUnits"`
		}
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		for _, ref := range p.TargetedAdUnits {
			covered[adUnitNameToID(ref)] = true
		}
	}
	return covered, len(blobs), nil
}
