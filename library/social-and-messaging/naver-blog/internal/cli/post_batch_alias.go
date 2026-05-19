// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Gate-1 alias: `post-batch --urls <file>` is shorthand for
// `batch --from-file <file>`. MCP-hidden to avoid tool duplication.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newPostBatchAliasCmd(flags *rootFlags) *cobra.Command {
	var (
		flagURLs   string
		flagConc   int
		flagPacing time.Duration
		flagEnrich bool
	)

	cmd := &cobra.Command{
		Use:   "post-batch",
		Short: "Alias for 'batch': backfill engagement for a file of post URLs.",
		Long: `Shorthand for the canonical 'batch' command with --urls in place of --from-file. Same file shapes (CSV / JSON / newline), same per-URL fetch pipeline, same input-order output. Use 'batch' directly if you want to script against the canonical form.

Hidden from the MCP tool surface.`,
		Example: `  naver-blog-pp-cli post-batch --urls urls.csv
  naver-blog-pp-cli post-batch --urls urls.txt --concurrency 3 --pacing 2s`,
		Annotations: map[string]string{
			"pp:endpoint":   "posts.batch",
			"pp:method":     "GET",
			"mcp:hidden":    "true",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample urls file.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagURLs) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "urls is required",
						"usage": fmt.Sprintf("%s --urls <path>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --urls not set"))
			}
			if flagConc <= 0 {
				flagConc = 5
			}
			inputs, err := loadBatchURLs(flagURLs)
			if err != nil {
				return usageErr(err)
			}
			if len(inputs) == 0 {
				return usageErr(fmt.Errorf("no URLs parsed from %q", flagURLs))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows := runBatchFetch(ctx, c, inputs, flagEnrich, flagConc, flagPacing)
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagURLs, "urls", "", "Path to the URL list file (CSV / JSON / newline-delimited). Required.")
	cmd.Flags().IntVar(&flagConc, "concurrency", 5, "Same as 'batch --concurrency'")
	cmd.Flags().DurationVar(&flagPacing, "pacing", time.Second, "Same as 'batch --pacing'")
	cmd.Flags().BoolVar(&flagEnrich, "enrich-engagement", true, "Same as 'batch --enrich-engagement'")
	return cmd
}
