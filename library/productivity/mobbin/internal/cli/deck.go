// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newDeckCmd(flags *rootFlags) *cobra.Command {
	var platform, exportZip, industry string
	var limit int
	cmd := &cobra.Command{
		Use:         "deck <theme-or-pattern>",
		Short:       "Build a zipped design reference deck for a pattern across an industry.",
		Example:     "  mobbin-pp-cli deck paywall --platform web --industry fintech --limit 20 --export-zip ./deck.zip",
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			hits, err := searchScreensAPI(cmd.Context(), c, platform, args[0], industry, limit)
			if err != nil {
				return err
			}
			hits, errs := cacheHits(cmd.Context(), hits)
			for _, e := range errs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: image download failed: %v\n", e)
			}
			if exportZip != "" {
				if err := writeDeckZip(exportZip, hits); err != nil {
					return err
				}
			}
			return flags.printJSON(cmd, map[string]any{"export_zip": exportZip, "count": len(hits), "items": hits})
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "web", "Platform to search: web, ios, android")
	cmd.Flags().StringVar(&industry, "industry", "", "App category to narrow by, e.g. fintech")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum screens to include")
	cmd.Flags().StringVar(&exportZip, "export-zip", "", "Write images and manifest.csv to this zip path")
	return cmd
}
