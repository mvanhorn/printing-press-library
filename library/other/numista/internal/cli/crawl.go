// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// PATCH: hand-written crawl parent for quota-aware multi-page workflows promised by README Highlights.
func newCrawlCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Quota-aware multi-page crawls. Persists results to the local store.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCrawlIssuerCmd(flags))
	return cmd
}
