// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source auto
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/spf13/cobra"
)

func newRestaurantsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "get <id>"}
	var dbPath string
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite cache path; default is the CLI data directory")
	return configureHGJGetCmd(cmd, flags, hgj.Restaurant, &dbPath)
}
