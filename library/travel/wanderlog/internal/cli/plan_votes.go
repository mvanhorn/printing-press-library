// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type planVoteRow struct {
	Day            int      `json:"day,omitempty"`
	Date           string   `json:"date,omitempty"`
	BlockID        int      `json:"block_id,omitempty"`
	Name           string   `json:"name,omitempty"`
	UpvotedByCount int      `json:"upvoted_by_count"`
	UpvotedBy      []string `json:"upvoted_by,omitempty"`
}

func newNovelPlanVotesCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{
		Use:     "votes",
		Short:   "List place and hotel block upvote counts for a Wanderlog plan",
		Example: "  wanderlog-pp-cli plan votes --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := planLiveClient(flags)
			if err != nil {
				return err
			}
			key, err := resolveEditablePlanKey(opts)
			if err != nil {
				return usageErr(err)
			}
			trip, _, err := fetchPlan(ctx, c, key, opts.clientSchemaVersion)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"command":    "plan votes",
				"target_key": key,
				"items":      collectPlanVotes(trip),
			}, flags)
		},
	}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

// PATCH(amend-2026-08-23: read-only place/hotel upvote projection; no comments API)
func collectPlanVotes(trip map[string]any) []planVoteRow {
	reports := sectionReports(trip)
	secs := sections(trip)
	out := []planVoteRow{}
	for i, raw := range secs {
		sec, _ := raw.(map[string]any)
		if sec == nil {
			continue
		}
		rep := planSectionReport{}
		if i < len(reports) {
			rep = reports[i]
		}
		day := rep.Day
		date := firstNonEmpty(rep.Date, stringField(sec, "date"))
		if !datedDayPlanSection(rep, sec) {
			day = 0
		}
		blocks, _ := sec["blocks"].([]any)
		for bi, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			if block == nil {
				continue
			}
			if stringField(block, "type") != "place" && mapField(block, "hotel") == nil {
				continue
			}
			ob := outlineBlock(trip, block, bi, date)
			ids := upvotedByIDs(block)
			out = append(out, planVoteRow{
				Day:            day,
				Date:           date,
				BlockID:        ob.BlockID,
				Name:           ob.Name,
				UpvotedByCount: len(ids),
				UpvotedBy:      ids,
			})
		}
	}
	return out
}

func upvotedByIDs(block map[string]any) []string {
	if block == nil {
		return nil
	}
	switch v := block["upvotedBy"].(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, id := range v {
			if id = strings.TrimSpace(id); id != "" {
				out = append(out, id)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if id := upvoteID(item); id != "" {
				out = append(out, id)
			}
		}
		return out
	default:
		return nil
	}
}

func upvoteID(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case map[string]any:
		id := firstNonEmpty(stringField(t, "id"), stringField(t, "userId"), stringField(t, "user_id"), stringField(t, "uid"))
		if id != "" {
			return id
		}
		if n := intAny(t["id"]); n != 0 {
			return strconv.Itoa(n)
		}
		if n := intAny(t["userId"]); n != 0 {
			return strconv.Itoa(n)
		}
		if n := intAny(t["user_id"]); n != 0 {
			return strconv.Itoa(n)
		}
		return ""
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}
