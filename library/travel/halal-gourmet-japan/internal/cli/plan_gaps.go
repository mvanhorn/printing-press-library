// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelPlanGapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "gaps"}
	options := &hgjPlanOptions{}
	cmd.Flags().StringSliceVar(&options.restaurants, "restaurants", nil, "Selected restaurant source IDs, at most 20, comma-separated or repeated")
	cmd.Flags().StringSliceVar(&options.prayer, "prayer", nil, "Selected prayer-place source IDs, at most 20, comma-separated or repeated")
	cmd.Flags().StringVar(&options.dbPath, "db", "", "SQLite detail cache path; default is the CLI data directory")
	cmd.Flags().IntVar(&options.limit, "limit", 20, "Maximum planning result rows returned, from 1 to 50")
	return configureHGJPlanCmd(cmd, flags, "gaps", options)
}
