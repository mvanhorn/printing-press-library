// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source local
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/store"
	"github.com/spf13/cobra"
)

type issueLifecycle struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Tier           string `json:"tier"`
	Status         string `json:"status"`
	FirstSeen      string `json:"first_seen"`
	LastSeen       string `json:"last_seen"`
	Occurrences    int    `json:"occurrences"`
	Recommendation string `json:"recommendation,omitempty"`
}

func newNovelIssuesCmd(flags *rootFlags) *cobra.Command {
	var target, status string
	cmd := &cobra.Command{Use: "issues", Short: "Track when readiness findings first appeared, last appeared, were fixed, or regressed.", Example: "  is-agentic-pp-cli issues --target https://is-agentic.com --json", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "derive issue lifecycle from local history")
		}
		if target == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--target is required"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, path, err := openAgenticStore(ctx, flags)
		if err != nil {
			return err
		}
		if s == nil {
			return missingStore(cmd, flags, path)
		}
		defer s.Close()
		snapshots, err := s.ListAgenticSnapshots(ctx, target, 1000)
		if err != nil {
			return err
		}
		if len(snapshots) == 0 {
			return emitSnapshots(cmd, flags, make([]store.AgenticSnapshot, 0))
		}
		byID := map[string]*issueLifecycle{}
		regressed := map[string]bool{}
		previous := map[string]bool{}
		for idx := len(snapshots) - 1; idx >= 0; idx-- {
			snap := snapshots[idx]
			report, e := parseReportSnapshot(snap)
			if e != nil {
				return e
			}
			current := make(map[string]bool, len(report.Issues))
			for _, issue := range report.Issues {
				current[issue.Id] = true
				item := byID[issue.Id]
				if item == nil {
					item = &issueLifecycle{ID: issue.Id, Name: issue.Name, Tier: issue.Tier, FirstSeen: snap.FetchedAt}
					byID[issue.Id] = item
				}
				item.LastSeen = snap.FetchedAt
				item.Occurrences++
				item.Recommendation = issue.Recommendation
				if previous[issue.Id] == false && item.Occurrences > 1 {
					regressed[issue.Id] = true
				}
			}
			previous = current
		}
		latest, _ := parseReportSnapshot(snapshots[0])
		current := make(map[string]bool, len(latest.Issues))
		for _, issue := range latest.Issues {
			current[issue.Id] = true
		}
		for id, item := range byID {
			if !current[id] {
				item.Status = "fixed"
			} else if regressed[id] {
				item.Status = "regressed"
			} else {
				item.Status = "open"
			}
		}
		items := make([]issueLifecycle, 0, len(byID))
		for _, item := range byID {
			if status == "" || item.Status == status {
				items = append(items, *item)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		return printJSONFiltered(cmd.OutOrStdout(), items, flags)
	}}
	cmd.Flags().StringVar(&target, "target", "", "public URL whose issue history should be summarized")
	cmd.Flags().StringVar(&status, "status", "", "filter by open, fixed, or regressed")
	return cmd
}
