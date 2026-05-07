// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newSegmentsOverlapCmd computes pairwise + multi-way Venn region counts of
// segment memberships. Each segment's membership is fetched live from the API
// (segments/{id}/membership); regions are computed in-memory.
//
// Why live: there is no segment_members table in the local store today —
// segments resource caches segment metadata, not the per-segment customer
// roster. Falling back to live calls keeps the command honest until that
// table lands.
func newSegmentsOverlapCmd(flags *rootFlags) *cobra.Command {
	var showIDs bool
	var perSegmentLimit int
	cmd := &cobra.Command{
		Use:   "overlap <environment-id> <segment-id> <segment-id> [<segment-id>...]",
		Short: "Compute pairwise and multi-way Venn-region counts of segment memberships",
		Long: `Compute the Venn-region counts (only-A, only-B, A∩B, etc.) for two or more
segments. Membership is read live from the API; with --show-ids, every
recipient ID per region is also listed.

Customer.io's UI offers no multi-segment intersection; this command answers
"are my churned-risk and high-value segments overlapping more than I think?"
without manual exports.`,
		Example: strings.Trim(`
  customer-io-pp-cli segments overlap 123457 1 2
  customer-io-pp-cli segments overlap 123457 1 2 3 --json
  customer-io-pp-cli segments overlap 123457 1 2 --show-ids --select regions
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			envID := strings.TrimSpace(args[0])
			segIDs := args[1:]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			memberships := make(map[string]map[string]struct{}, len(segIDs))
			ordered := make([]string, 0, len(segIDs))
			for _, segID := range segIDs {
				if _, dup := memberships[segID]; dup {
					continue
				}
				ordered = append(ordered, segID)
				params := map[string]string{}
				if perSegmentLimit > 0 {
					params["limit"] = fmt.Sprintf("%d", perSegmentLimit)
				}
				path := "/v1/environments/" + envID + "/segments/" + segID + "/membership"
				data, getErr := c.Get(path, params)
				if getErr != nil {
					return classifyAPIError(fmt.Errorf("segment %s membership: %w", segID, getErr), flags)
				}
				ids := extractMembershipIDs(data)
				set := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					set[id] = struct{}{}
				}
				memberships[segID] = set
			}

			regions := computeOverlapRegions(ordered, memberships)
			out := buildOverlapOutput(ordered, memberships, regions, showIDs)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Segments compared: %s\n\n", strings.Join(ordered, ", "))
			fmt.Fprintln(cmd.OutOrStdout(), "Sizes:")
			for _, segID := range ordered {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d members\n", segID, len(memberships[segID]))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Regions (count):")
			for _, region := range regions {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", region.Label, len(region.IDs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showIDs, "show-ids", false, "Include the recipient IDs in each Venn region (default: counts only)")
	cmd.Flags().IntVar(&perSegmentLimit, "per-segment-limit", 0, "Cap each segment's membership fetch (0 = no cap; useful for huge segments)")
	return cmd
}

// extractMembershipIDs handles Customer.io's segment-membership response
// shape: an "identifiers" array of objects with "id" or "email".
func extractMembershipIDs(data json.RawMessage) []string {
	var raw struct {
		Identifiers []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"identifiers"`
	}
	if err := json.Unmarshal(data, &raw); err == nil && len(raw.Identifiers) > 0 {
		ids := make([]string, 0, len(raw.Identifiers))
		for _, ident := range raw.Identifiers {
			if ident.ID != "" {
				ids = append(ids, ident.ID)
			} else if ident.Email != "" {
				ids = append(ids, ident.Email)
			}
		}
		return ids
	}
	// Fallback: a flat array of strings.
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	return nil
}

type overlapRegion struct {
	Label string   `json:"label"`
	In    []string `json:"in"`
	Out   []string `json:"out"`
	IDs   []string `json:"ids,omitempty"`
}

// computeOverlapRegions enumerates every non-empty subset of segments and
// returns the recipient IDs that are in exactly that subset.
func computeOverlapRegions(segments []string, memberships map[string]map[string]struct{}) []overlapRegion {
	allIDs := make(map[string]struct{})
	for _, segID := range segments {
		for id := range memberships[segID] {
			allIDs[id] = struct{}{}
		}
	}

	regionsByMask := make(map[uint64]*overlapRegion)
	for id := range allIDs {
		var mask uint64
		for i, segID := range segments {
			if _, ok := memberships[segID][id]; ok {
				mask |= 1 << i
			}
		}
		if mask == 0 {
			continue
		}
		r, ok := regionsByMask[mask]
		if !ok {
			in := []string{}
			out := []string{}
			labelParts := []string{}
			for i, segID := range segments {
				if mask&(1<<i) != 0 {
					in = append(in, segID)
					labelParts = append(labelParts, segID)
				} else {
					out = append(out, segID)
				}
			}
			label := strings.Join(labelParts, " ∩ ")
			if len(in) == 1 && len(out) > 0 {
				label = "only " + in[0]
			}
			r = &overlapRegion{Label: label, In: in, Out: out}
			regionsByMask[mask] = r
		}
		r.IDs = append(r.IDs, id)
	}

	regions := make([]overlapRegion, 0, len(regionsByMask))
	for _, r := range regionsByMask {
		sort.Strings(r.IDs)
		regions = append(regions, *r)
	}
	sort.Slice(regions, func(i, j int) bool {
		// Most-overlapping regions first (more "in" segments), then by size.
		if len(regions[i].In) != len(regions[j].In) {
			return len(regions[i].In) > len(regions[j].In)
		}
		return len(regions[i].IDs) > len(regions[j].IDs)
	})
	return regions
}

func buildOverlapOutput(segments []string, memberships map[string]map[string]struct{}, regions []overlapRegion, showIDs bool) map[string]any {
	sizes := make(map[string]int, len(segments))
	for _, segID := range segments {
		sizes[segID] = len(memberships[segID])
	}
	cleanRegions := make([]map[string]any, 0, len(regions))
	for _, r := range regions {
		entry := map[string]any{
			"label": r.Label,
			"in":    r.In,
			"out":   r.Out,
			"count": len(r.IDs),
		}
		if showIDs {
			entry["ids"] = r.IDs
		}
		cleanRegions = append(cleanRegions, entry)
	}
	return map[string]any{
		"segments": segments,
		"sizes":    sizes,
		"regions":  cleanRegions,
	}
}
