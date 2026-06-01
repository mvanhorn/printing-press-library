// Copyright 2026 never-mind-3 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type trafficRow struct {
	Revision  string `json:"revision"`
	Percent   int    `json:"percent"`
	Image     string `json:"image"`
	IsLatest  bool   `json:"is_latest"`
	CreatedAt string `json:"created_at"`
	Tag       string `json:"tag,omitempty"`
}

type servicesTrafficView struct {
	Service string       `json:"service"`
	Traffic []trafficRow `json:"traffic"`
}

func newNovelServicesTrafficCmd(flags *rootFlags) *cobra.Command {
	var flagService string

	cmd := &cobra.Command{
		Use:   "traffic",
		Short: "Show the current traffic split across revisions with image tag and deploy timestamp in one table.",
		Long:  "Shows traffic split table with revision name, image tag, creation timestamp, and latest-tag status; requires a Get Revision call per entry to surface image info not present on the Service object.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli services traffic --service projects/my-proj/locations/us-central1/services/my-svc
  google-cloud-run-pp-cli services traffic --service projects/my-proj/locations/us-central1/services/my-svc --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagService == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch traffic split for service:", flagService)
				return nil
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

			svcData, err := c.Get(ctx, "/v2/"+flagService, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var svc struct {
				LatestCreatedRevision string `json:"latestCreatedRevision"`
				LatestReadyRevision   string `json:"latestReadyRevision"`
				TrafficStatuses       []struct {
					Type     string `json:"type"`
					Revision string `json:"revision"`
					Percent  int    `json:"percent"`
					Tag      string `json:"tag,omitempty"`
				} `json:"trafficStatuses"`
			}
			if err := json.Unmarshal(svcData, &svc); err != nil {
				return fmt.Errorf("parsing service: %w", err)
			}

			// Collect unique revision names to fetch
			revNames := make(map[string]bool)
			for _, t := range svc.TrafficStatuses {
				if t.Revision != "" {
					revNames[t.Revision] = true
				}
			}

			type revInfo struct {
				Image      string
				CreateTime time.Time
			}
			revInfoMap := make(map[string]revInfo)
			for revName := range revNames {
				revData, revErr := c.Get(ctx, "/v2/"+revName, nil)
				if revErr != nil {
					continue
				}
				var rev struct {
					Containers []struct {
						Image string `json:"image"`
					} `json:"containers"`
					CreateTime time.Time `json:"createTime"`
				}
				if json.Unmarshal(revData, &rev) == nil && len(rev.Containers) > 0 {
					revInfoMap[revName] = revInfo{
						Image:      rev.Containers[0].Image,
						CreateTime: rev.CreateTime,
					}
				}
			}

			var rows []trafficRow
			for _, t := range svc.TrafficStatuses {
				if t.Percent == 0 {
					continue
				}
				info := revInfoMap[t.Revision]
				rows = append(rows, trafficRow{
					Revision:  shortName(t.Revision),
					Percent:   t.Percent,
					Image:     info.Image,
					IsLatest:  t.Revision == svc.LatestReadyRevision,
					CreatedAt: info.CreateTime.Format(time.RFC3339),
					Tag:       t.Tag,
				})
			}

			view := servicesTrafficView{Service: shortName(flagService), Traffic: rows}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "REVISION\tPCT\tIMAGE\tLATEST\tCREATED")
			for _, r := range rows {
				latest := ""
				if r.IsLatest {
					latest = "yes"
				}
				fmt.Fprintf(tw, "%s\t%d%%\t%s\t%s\t%s\n", r.Revision, r.Percent, r.Image, latest, r.CreatedAt)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagService, "service", "", "Full resource name of the service (projects/{project}/locations/{region}/services/{service})")
	return cmd
}
