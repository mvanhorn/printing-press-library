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

type revisionDiffField struct {
	Field   string `json:"field"`
	From    string `json:"from"`
	To      string `json:"to"`
	Changed bool   `json:"changed"`
}

type revisionsDiffView struct {
	FromRevision string              `json:"from_revision"`
	ToRevision   string              `json:"to_revision"`
	Fields       []revisionDiffField `json:"fields"`
	ChangedCount int                 `json:"changed_count"`
}

func newNovelRevisionsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagService string
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show a field-by-field diff between two revisions: image, CPU/memory, scaling config, and env var key names.",
		Long:  "Shows a field-by-field diff of two revisions: image, resource limits, scaling config, service account, VPC connector, and env var key names. Env var values are omitted to avoid secret exposure.",
		Example: strings.Trim(`
  google-cloud-run-pp-cli revisions diff --service projects/my-proj/locations/us-central1/services/my-svc --from my-svc-00041-abc --to my-svc-00042-def
  google-cloud-run-pp-cli revisions diff --service projects/my-proj/locations/us-central1/services/my-svc --from my-svc-00041-abc --to my-svc-00042-def --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagService == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would diff revisions %s..%s on service %s\n", flagFrom, flagTo, flagService)
				return nil
			}
			if flagService == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--service is required"))
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are required"))
			}

			// Build full resource names from service path + revision short names
			serviceBase := flagService // e.g. projects/p/locations/r/services/s
			fromName := flagFrom
			toName := flagTo
			// If short names (no /), construct full path.
			// Validate short names to prevent path injection.
			validRevName := func(n string) bool {
				for _, r := range n {
					if !(r == '-' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
						return false
					}
				}
				return len(n) > 0 && len(n) <= 63
			}
			if !strings.Contains(flagFrom, "/") {
				if !validRevName(flagFrom) {
					return usageErr(fmt.Errorf("invalid revision name %q: must be alphanumeric + hyphens only", flagFrom))
				}
				fromName = serviceBase + "/revisions/" + flagFrom
			}
			if !strings.Contains(flagTo, "/") {
				if !validRevName(flagTo) {
					return usageErr(fmt.Errorf("invalid revision name %q: must be alphanumeric + hyphens only", flagTo))
				}
				toName = serviceBase + "/revisions/" + flagTo
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			fromData, err := c.Get(ctx, "/v2/"+fromName, nil)
			if err != nil {
				return fmt.Errorf("fetching revision %s: %w", flagFrom, classifyAPIError(err, flags))
			}
			toData, err := c.Get(ctx, "/v2/"+toName, nil)
			if err != nil {
				return fmt.Errorf("fetching revision %s: %w", flagTo, classifyAPIError(err, flags))
			}

			type revisionSpec struct {
				Containers []struct {
					Image     string           `json:"image"`
					Env       []map[string]any `json:"env"`
					Resources struct {
						Limits map[string]string `json:"limits"`
					} `json:"resources"`
				} `json:"containers"`
				Scaling struct {
					MinInstanceCount int `json:"minInstanceCount"`
					MaxInstanceCount int `json:"maxInstanceCount"`
				} `json:"scaling"`
				ServiceAccount string `json:"serviceAccount"`
				VpcAccess      struct {
					Connector string `json:"connector"`
				} `json:"vpcAccess"`
				MaxInstanceRequestConcurrency int `json:"maxInstanceRequestConcurrency"`
			}

			var fromRev, toRev revisionSpec
			if err := json.Unmarshal(fromData, &fromRev); err != nil {
				return fmt.Errorf("parsing from revision: %w", err)
			}
			if err := json.Unmarshal(toData, &toRev); err != nil {
				return fmt.Errorf("parsing to revision: %w", err)
			}

			diffStr := func(a, b string) revisionDiffField {
				return revisionDiffField{From: a, To: b, Changed: a != b}
			}
			orNA := func(s string) string {
				if s == "" {
					return "(none)"
				}
				return s
			}

			fromImage, toImage := "", ""
			if len(fromRev.Containers) > 0 {
				fromImage = fromRev.Containers[0].Image
			}
			if len(toRev.Containers) > 0 {
				toImage = toRev.Containers[0].Image
			}

			safeLimit := func(c revisionSpec, key string) string {
				if len(c.Containers) == 0 {
					return "(none)"
				}
				return orNA(c.Containers[0].Resources.Limits[key])
			}
			fromCPU := safeLimit(fromRev, "cpu")
			toCPU := safeLimit(toRev, "cpu")
			fromMem := safeLimit(fromRev, "memory")
			toMem := safeLimit(toRev, "memory")

			// Collect env var key names (not values)
			envKeys := func(c revisionSpec) []string {
				if len(c.Containers) == 0 {
					return nil
				}
				var keys []string
				for _, e := range c.Containers[0].Env {
					if name, ok := e["name"].(string); ok {
						keys = append(keys, name)
					}
				}
				return keys
			}
			fromEnvKeys := strings.Join(envKeys(fromRev), ",")
			toEnvKeys := strings.Join(envKeys(toRev), ",")

			fields := []revisionDiffField{
				{Field: "image", From: orNA(fromImage), To: orNA(toImage), Changed: fromImage != toImage},
				{Field: "cpu_limit", From: fromCPU, To: toCPU, Changed: fromCPU != toCPU},
				{Field: "memory_limit", From: fromMem, To: toMem, Changed: fromMem != toMem},
			}
			f := func(name, a, b string) revisionDiffField {
				d := diffStr(a, b)
				d.Field = name
				return d
			}
			fields = append(fields,
				f("min_instances", fmt.Sprint(fromRev.Scaling.MinInstanceCount), fmt.Sprint(toRev.Scaling.MinInstanceCount)),
				f("max_instances", fmt.Sprint(fromRev.Scaling.MaxInstanceCount), fmt.Sprint(toRev.Scaling.MaxInstanceCount)),
				f("concurrency", fmt.Sprint(fromRev.MaxInstanceRequestConcurrency), fmt.Sprint(toRev.MaxInstanceRequestConcurrency)),
				f("service_account", orNA(fromRev.ServiceAccount), orNA(toRev.ServiceAccount)),
				f("vpc_connector", orNA(fromRev.VpcAccess.Connector), orNA(toRev.VpcAccess.Connector)),
				f("env_keys (names only)", orNA(fromEnvKeys), orNA(toEnvKeys)),
			)

			changed := 0
			for _, f := range fields {
				if f.Changed {
					changed++
				}
			}

			view := revisionsDiffView{
				FromRevision: shortName(fromName),
				ToRevision:   shortName(toName),
				Fields:       fields,
				ChangedCount: changed,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if changed == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no differences between revisions")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "FIELD\tFROM (%s)\tTO (%s)\n", shortName(fromName), shortName(toName))
			for _, fld := range fields {
				marker := " "
				if fld.Changed {
					marker = "~"
				}
				fmt.Fprintf(tw, "%s %s\t%s\t%s\n", marker, fld.Field, fld.From, fld.To)
			}
			tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d field(s) changed\n", changed)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagService, "service", "", "Full resource name of the service (projects/{project}/locations/{region}/services/{service})")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Source revision (short name or full resource name)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Target revision (short name or full resource name)")
	return cmd
}
