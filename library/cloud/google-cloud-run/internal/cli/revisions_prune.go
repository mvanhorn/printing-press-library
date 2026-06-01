// Copyright 2026 never-mind-3 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type revisionPruneRow struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Action    string `json:"action"`
}

type revisionsPruneView struct {
	Pruned       []revisionPruneRow `json:"pruned"`
	Kept         []revisionPruneRow `json:"kept"`
	DryRun       bool               `json:"dry_run"`
	TotalKept    int                `json:"total_kept"`
	TotalDeleted int                `json:"total_deleted"`
}

func newNovelRevisionsPruneCmd(flags *rootFlags) *cobra.Command {
	var flagService string
	var flagKeepLast int

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old revisions that are not serving any traffic, keeping only the N most recent.",
		Long:  "Lists all revisions for a service, keeps the N most recent non-traffic-serving ones, and deletes the rest. Revisions actively serving traffic are never deleted. Use --dry-run to preview changes without deleting.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli revisions prune --service projects/my-proj/locations/us-central1/services/my-svc --keep-last 3 --dry-run
  google-cloud-run-pp-cli revisions prune --service projects/my-proj/locations/us-central1/services/my-svc --keep-last 5 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagService == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if flagService == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--service is required (format: projects/{project}/locations/{region}/services/{service})"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			// Fetch the service to get traffic-serving revisions
			svcData, err := c.Get(ctx, "/v2/"+flagService, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var svc struct {
				TrafficStatuses []struct {
					Revision string `json:"revision"`
				} `json:"trafficStatuses"`
			}
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("parsing service: %w", err)
			}
			serving := make(map[string]bool)
			for _, t := range svc.TrafficStatuses {
				if t.Revision != "" {
					serving[t.Revision] = true
					serving[shortName(t.Revision)] = true
				}
			}

			// List all revisions for this service
			revData, err := c.Get(ctx, "/v2/"+flagService+"/revisions", nil)
			if err != nil {
				return fmt.Errorf("listing revisions: %w", classifyAPIError(err, flags))
			}
			var revResp struct {
				Revisions []struct {
					Name       string    `json:"name"`
					CreateTime time.Time `json:"createTime"`
				} `json:"revisions"`
			}
			if err := json.Unmarshal(revData, &revResp); err != nil {
				return fmt.Errorf("parsing revisions: %w", err)
			}

			// Sort newest first
			sort.Slice(revResp.Revisions, func(i, j int) bool {
				return revResp.Revisions[i].CreateTime.After(revResp.Revisions[j].CreateTime)
			})

			keepN := flagKeepLast
			if keepN <= 0 {
				keepN = 3
			}

			var toKeep, toDelete []revisionPruneRow
			nonServingCount := 0
			for _, r := range revResp.Revisions {
				if serving[r.Name] || serving[shortName(r.Name)] {
					continue // never touch traffic-serving revisions
				}
				row := revisionPruneRow{
					Name:      r.Name,
					CreatedAt: r.CreateTime.Format(time.RFC3339),
				}
				if nonServingCount < keepN {
					row.Action = "keep"
					toKeep = append(toKeep, row)
				} else {
					row.Action = "delete"
					toDelete = append(toDelete, row)
				}
				nonServingCount++
			}

			view := revisionsPruneView{
				Pruned:    toDelete,
				Kept:      toKeep,
				DryRun:    flags.dryRun,
				TotalKept: len(toKeep),
			}

			if len(toDelete) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no revisions to prune")
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			if flags.dryRun {
				view.DryRun = true
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ACTION\tNAME\tCREATED")
				for _, r := range toKeep {
					fmt.Fprintf(tw, "keep  \t%s\t%s\n", shortName(r.Name), r.CreatedAt)
				}
				for _, r := range toDelete {
					fmt.Fprintf(tw, "DELETE\t%s\t%s\n", shortName(r.Name), r.CreatedAt)
				}
				tw.Flush()
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d revision(s) would be deleted. Re-run without --dry-run to apply.\n", len(toDelete))
				return nil
			}

			deleted := 0
			for _, r := range toDelete {
				// Validate that the revision name is scoped to the specified service
				// before constructing the DELETE path.
				if !strings.HasPrefix(r.Name, flagService+"/revisions/") {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping unexpected revision name %q (not under service %s)\n", r.Name, flagService)
					continue
				}
				_, _, delErr := c.Delete(ctx, "/v2/"+r.Name)
				if delErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to delete %s: %v\n", shortName(r.Name), delErr)
					continue
				}
				deleted++
			}
			view.TotalDeleted = deleted
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %d revision(s), kept %d\n", deleted, len(toKeep))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagService, "service", "", "Full resource name of the service (projects/{project}/locations/{region}/services/{service})")
	cmd.Flags().IntVar(&flagKeepLast, "keep-last", 3, "Number of most recent non-traffic-serving revisions to keep")
	return cmd
}
