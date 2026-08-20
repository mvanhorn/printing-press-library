// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRedirectsGenerateCmd(flags *rootFlags) *cobra.Command {
	var oldSitemap string

	cmd := &cobra.Command{
		Use:   "redirects-generate",
		Short: "Auto-generate redirect map by matching old sitemap URLs to Framer page slugs",
		Long: strings.Trim(`
Auto-generate a redirect map by crawling an old site's sitemap and
fuzzy-matching URLs to Framer page slugs.

Redirect map generation is planned for a future version.`, "\n"),
		Example: strings.Trim(`
  # Generate redirects from old sitemap
  framer-pp-cli redirects-generate --old-sitemap https://old-site.com/sitemap.xml

  # Dry-run with JSON output
  framer-pp-cli redirects-generate --old-sitemap https://old-site.com/sitemap.xml --dry-run --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Redirect map generation is planned for a future version.")
			return nil
		},
	}

	cmd.Flags().StringVar(&oldSitemap, "old-sitemap", "", "URL of the old site's sitemap.xml")

	return cmd
}
