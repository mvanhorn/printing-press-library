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

// cloudRunService is a minimal representation of a Cloud Run service for multi-project listing.
type cloudRunService struct {
	Name        string            `json:"name"`
	URI         string            `json:"uri,omitempty"`
	Region      string            `json:"region,omitempty"`
	Project     string            `json:"project,omitempty"`
	Revision    string            `json:"latestReadyRevision,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type servicesListAllView struct {
	Services       []cloudRunServiceRow `json:"services"`
	Total          int                  `json:"total"`
	FailedProjects []failedProjectEntry `json:"failed_projects,omitempty"`
}

// failedProjectEntry records a project that could not be fetched during a
// multi-project list. Agents can inspect this field to distinguish "no
// services exist" from "some or all projects failed to respond."
type failedProjectEntry struct {
	Project string `json:"project"`
	Reason  string `json:"reason"`
}

type cloudRunServiceRow struct {
	Name     string `json:"name"`
	Project  string `json:"project"`
	Region   string `json:"region"`
	URI      string `json:"uri"`
	Revision string `json:"serving_revision"`
}

func newNovelServicesListAllCmd(flags *rootFlags) *cobra.Command {
	var flagProjects string
	var flagRegion string

	cmd := &cobra.Command{
		Use:   "list-all",
		Short: "List Cloud Run services across multiple GCP projects in a single unified table.",
		Long:  "Lists Cloud Run services across one or more GCP projects, showing project, region, service name, URI, and serving revision in a single merged table. Useful for fleet-wide visibility when managing services spread across multiple projects.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli services list-all --projects my-proj-1,my-proj-2
  google-cloud-run-pp-cli services list-all --projects my-proj-1 --region us-central1 --json
  google-cloud-run-pp-cli services list-all --projects prod,staging --agent --select services.name,services.uri`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flagProjects != "" {
					fmt.Fprintln(cmd.OutOrStdout(), "would list services across projects:", flagProjects)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "would list services (--projects required)")
				}
				return nil
			}
			if flagProjects == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--projects is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			projects := strings.Split(flagProjects, ",")
			var rows []cloudRunServiceRow
			var failedProjects []failedProjectEntry

			for _, proj := range projects {
				proj = strings.TrimSpace(proj)
				if proj == "" {
					continue
				}

				// Build the parent path. If region is specified, list that region only.
				// Otherwise use "-" as a wildcard location.
				location := flagRegion
				if location == "" {
					location = "-"
				}
				parent := fmt.Sprintf("projects/%s/locations/%s", proj, location)
				path := fmt.Sprintf("/v2/%s/services", parent)

				data, err := c.Get(context.Background(), path, map[string]string{})
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to list services in project %q: %v\n", proj, err)
					failedProjects = append(failedProjects, failedProjectEntry{
						Project: proj,
						Reason:  err.Error(),
					})
					continue
				}

				var resp struct {
					Services []json.RawMessage `json:"services"`
				}
				if err := json.Unmarshal(data, &resp); err != nil {
					// Try as array directly
					var arr []json.RawMessage
					if err2 := json.Unmarshal(data, &arr); err2 == nil {
						resp.Services = arr
					}
				}

				for _, raw := range resp.Services {
					var svc struct {
						Name                string `json:"name"`
						URI                 string `json:"uri"`
						LatestReadyRevision string `json:"latestReadyRevision"`
					}
					if err := json.Unmarshal(raw, &svc); err != nil {
						continue
					}
					// Extract project and region from resource name
					// name = projects/{project}/locations/{region}/services/{service}
					row := cloudRunServiceRow{
						Name:     shortName(svc.Name),
						Project:  proj,
						Region:   extractSegment(svc.Name, "locations"),
						URI:      svc.URI,
						Revision: shortName(svc.LatestReadyRevision),
					}
					rows = append(rows, row)
				}
			}

			view := servicesListAllView{Services: rows, Total: len(rows), FailedProjects: failedProjects}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "PROJECT\tREGION\tSERVICE\tURI\tREVISION")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Project, r.Region, r.Name, r.URI, r.Revision)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagProjects, "projects", "", "Comma-separated GCP project IDs to list services from (required)")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Cloud Run region (e.g. us-central1); omit to list all regions")
	return cmd
}

// shortName returns the last segment of a resource name (e.g. "services/my-svc" -> "my-svc").
func shortName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// extractSegment returns the value after the given key segment in a resource path.
// e.g. extractSegment("projects/p/locations/us-central1/services/s", "locations") = "us-central1"
func extractSegment(name, key string) string {
	parts := strings.Split(name, "/")
	for i, p := range parts {
		if p == key && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
