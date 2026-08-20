// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type segmentAuditEntry struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	MemberCount int      `json:"member_count"`
	Type        string   `json:"type"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
	Issues      []string `json:"issues"`
}

type segmentAuditReport struct {
	ListID   string              `json:"list_id"`
	Total    int                 `json:"total_segments"`
	Healthy  int                 `json:"healthy"`
	Flagged  int                 `json:"flagged"`
	Segments []segmentAuditEntry `json:"segments"`
}

func newSegmentsAuditCmd(flags *rootFlags) *cobra.Command {
	var listID string
	var staleDays int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Find empty and stale segments in an audience. Flags zero-member segments and segments not updated in N days.",
		Long: `Reads /lists/{list-id}/segments and flags each segment with one or more of:
  - empty: member_count == 0
  - stale: updated_at older than --stale-days (default 90)

Outputs a list of flagged segments with the reason for each, plus healthy/flagged
counts. Use to clean up segment debt that accumulates as audiences age.`,
		Example: `  mailchimp-pp-cli segments audit --list b7661f2918
  mailchimp-pp-cli segments audit --list b7661f2918 --stale-days 60 --json
  mailchimp-pp-cli segments audit --list b7661f2918 --json --select segments[*].issues`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if listID == "" {
				return fmt.Errorf("--list is required (audience/list id)")
			}
			if staleDays <= 0 {
				staleDays = 90
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"would_audit": map[string]any{
						"GET":               fmt.Sprintf("/lists/%s/segments?count=1000", listID),
						"flag_zero_members": true,
						"flag_stale_days":   staleDays,
					},
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(fmt.Sprintf("/lists/%s/segments", listID), map[string]string{
				"count": "1000",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp map[string]any
			_ = json.Unmarshal(data, &resp)
			segments, _ := resp["segments"].([]any)

			cutoff := time.Now().AddDate(0, 0, -staleDays)
			report := segmentAuditReport{ListID: listID, Total: len(segments)}
			for _, s := range segments {
				m, _ := s.(map[string]any)
				if m == nil {
					continue
				}
				e := segmentAuditEntry{}
				if v, ok := m["id"].(float64); ok {
					e.ID = int(v)
				}
				if v, ok := m["name"].(string); ok {
					e.Name = v
				}
				if v, ok := m["member_count"].(float64); ok {
					e.MemberCount = int(v)
				}
				if v, ok := m["type"].(string); ok {
					e.Type = v
				}
				if v, ok := m["updated_at"].(string); ok {
					e.UpdatedAt = v
				}

				if e.MemberCount == 0 {
					e.Issues = append(e.Issues, "empty")
				}
				if e.UpdatedAt != "" {
					if t, err := time.Parse(time.RFC3339, e.UpdatedAt); err == nil && t.Before(cutoff) {
						e.Issues = append(e.Issues, fmt.Sprintf("stale (last updated %s)", t.Format("2006-01-02")))
					}
				}
				if len(e.Issues) > 0 {
					report.Flagged++
				} else {
					report.Healthy++
				}
				report.Segments = append(report.Segments, e)
			}
			// Sort flagged-first then by member count ascending so cleanup candidates lead.
			// stable sort: flagged at top, healthy below; both sorted by member count asc.
			sortSegments(report.Segments)

			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&listID, "list", "", "Audience (list) ID — find via 'audiences get-contacts'")
	cmd.Flags().IntVar(&staleDays, "stale-days", 90, "Flag segments not updated in this many days")
	return cmd
}

func sortSegments(segs []segmentAuditEntry) {
	// Flagged segments first (they need attention), then healthy. Within each group, ascending by member count.
	flagged := make([]segmentAuditEntry, 0, len(segs))
	healthy := make([]segmentAuditEntry, 0, len(segs))
	for _, s := range segs {
		if len(s.Issues) > 0 {
			flagged = append(flagged, s)
		} else {
			healthy = append(healthy, s)
		}
	}
	sortByMembers(flagged)
	sortByMembers(healthy)
	copy(segs, append(flagged, healthy...))
}

func sortByMembers(segs []segmentAuditEntry) {
	for i := 1; i < len(segs); i++ {
		for j := i; j > 0 && segs[j-1].MemberCount > segs[j].MemberCount; j-- {
			segs[j-1], segs[j] = segs[j], segs[j-1]
		}
	}
}
