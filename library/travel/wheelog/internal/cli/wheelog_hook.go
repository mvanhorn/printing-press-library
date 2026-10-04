// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerClientHook(func(c *client.Client) error {
		c.HTTPClient.Transport = client.WheelogTransport{Base: c.HTTPClient.Transport}
		return nil
	})
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		if preview := rootCmd.PersistentFlags().Lookup("dry-run"); preview != nil {
			preview.Usage = "Summarize the intended action without executing the command (does not show request fields)."
		}
		if rootCmd.PersistentFlags().Lookup("data-source") == nil {
			rootCmd.PersistentFlags().StringVar(&flags.dataSource, "data-source", "auto", "Read source: auto prefers live with labeled saved fallback; local uses saved evidence; live requires source requests.")
		}
		// Public WheeLog RPCs are reads despite using POST. Generic API import
		// cannot create/upsert facilities on this anonymous source contract.
		if unsupported, _, err := rootCmd.Find([]string{"import"}); err == nil && unsupported.Name() == "import" {
			rootCmd.RemoveCommand(unsupported)
		}
		rootCmd.AddCommand(newWheelogCategoriesCmd(flags))
		// Explicit AddCommand edges keep the Press structural tree aligned with
		// the runtime tree. Its current static matcher does not follow hook helpers.
		if existing, _, err := rootCmd.Find([]string{"shortlist"}); err == nil && existing.Name() == "shortlist" {
			rootCmd.RemoveCommand(existing)
		}
		rootCmd.AddCommand(newNovelShortlistCmd(flags))
		shortlist, _, err := rootCmd.Find([]string{"shortlist"})
		if err == nil && shortlist.Name() == "shortlist" {
			shortlist.AddCommand(newWheelogSaveCmd(flags))
			shortlist.AddCommand(newWheelogRemoveCmd(flags))
		}
	})
}
