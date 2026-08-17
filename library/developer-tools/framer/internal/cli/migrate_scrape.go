// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newMigrateScrapeCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var outputFile string

	cmd := &cobra.Command{
		Use:   "migrate-scrape <url>",
		Short: "Scrape an existing website and generate a Framer migration manifest",
		Long: strings.Trim(`
Scrape an existing website and generate a complete Framer migration plan
with pages, content, and assets.

Site migration scaffolding is planned for a future version.`, "\n"),
		Example: strings.Trim(`
  # Scrape a site with default depth
  framer-pp-cli migrate-scrape https://old-site.com

  # Scrape with custom depth and output file
  framer-pp-cli migrate-scrape https://old-site.com --depth 3 --output manifest.json

  # Dry-run mode
  framer-pp-cli migrate-scrape https://old-site.com --dry-run`, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Site migration scaffolding is planned for a future version.")
			return nil
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum crawl depth")
	cmd.Flags().StringVar(&outputFile, "output", "manifest.json", "Output manifest file path")

	return cmd
}
