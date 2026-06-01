// Copyright 2026 never-mind-3 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type iamAuditRow struct {
	Service         string `json:"service"`
	URI             string `json:"uri,omitempty"`
	OffendingMember string `json:"offending_binding"`
	Role            string `json:"role"`
	IsPublic        bool   `json:"is_public"`
}

type iamAuditView struct {
	Project    string        `json:"project"`
	Services   []iamAuditRow `json:"services"`
	TotalRisky int           `json:"total_risky"`
	TotalSafe  int           `json:"total_safe"`
}

func newNovelIamAuditCmd(flags *rootFlags) *cobra.Command {
	var flagProject string
	var flagRegion string
	var flagShowPublic bool

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "List all services in a project that are publicly accessible via allUsers or allAuthenticatedUsers IAM bindings.",
		Long:  "Walks all Cloud Run services in a project, fetches each service's IAM policy, and flags any with allUsers or allAuthenticatedUsers bindings. Use --show-public to limit output to risky services only.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli iam audit --project my-project
  google-cloud-run-pp-cli iam audit --project my-project --show-public --agent
  google-cloud-run-pp-cli iam audit --project my-project --show-public --json --select services.service,services.offending_binding`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagProject == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit IAM for project:", flagProject)
				return nil
			}
			if flagProject == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			location := flagRegion
			if location == "" {
				location = "-"
			}
			parent := fmt.Sprintf("projects/%s/locations/%s", flagProject, location)
			svcData, err := c.Get(ctx, "/v2/"+parent+"/services", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var svcResp struct {
				Services []struct {
					Name string `json:"name"`
					URI  string `json:"uri"`
				} `json:"services"`
			}
			if err := json.Unmarshal(svcData, &svcResp); err != nil {
				return fmt.Errorf("parsing services: %w", err)
			}

			var rows []iamAuditRow
			risky, safe := 0, 0

			for _, svc := range svcResp.Services {
				iamData, iamErr := c.Get(ctx, "/v2/"+svc.Name+":getIamPolicy", nil)
				if iamErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not get IAM for %s: %v\n", shortName(svc.Name), iamErr)
					continue
				}
				var policy struct {
					Bindings []struct {
						Role    string   `json:"role"`
						Members []string `json:"members"`
					} `json:"bindings"`
				}
				if err := json.Unmarshal(iamData, &policy); err != nil {
					continue
				}
				isRisky := false
				for _, b := range policy.Bindings {
					for _, m := range b.Members {
						if m == "allUsers" || m == "allAuthenticatedUsers" {
							rows = append(rows, iamAuditRow{
								Service:         shortName(svc.Name),
								URI:             svc.URI,
								OffendingMember: m,
								Role:            b.Role,
								IsPublic:        true,
							})
							isRisky = true
						}
					}
				}
				if isRisky {
					risky++
				} else {
					safe++
					if !flagShowPublic {
						rows = append(rows, iamAuditRow{
							Service:  shortName(svc.Name),
							URI:      svc.URI,
							IsPublic: false,
						})
					}
				}
			}

			view := iamAuditView{
				Project:    flagProject,
				Services:   rows,
				TotalRisky: risky,
				TotalSafe:  safe,
			}
			if flagShowPublic {
				// Filter to only risky rows
				var riskyRows []iamAuditRow
				for _, r := range rows {
					if r.IsPublic {
						riskyRows = append(riskyRows, r)
					}
				}
				view.Services = riskyRows
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Services) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no publicly accessible services found in project %s\n", flagProject)
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "SERVICE\tPUBLIC\tOFFENDING_MEMBER\tROLE\tURI")
			for _, r := range view.Services {
				pub := "no"
				if r.IsPublic {
					pub = "YES"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Service, pub, r.OffendingMember, r.Role, r.URI)
			}
			tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d risky, %d safe (total %d services)\n", risky, safe, risky+safe)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagProject, "project", "", "GCP project ID to audit (required)")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Cloud Run region (omit to audit all regions)")
	cmd.Flags().BoolVar(&flagShowPublic, "show-public", false, "Show only publicly accessible services")
	return cmd
}
