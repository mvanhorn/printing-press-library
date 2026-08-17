// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newProgramsHacktivityCmd(flags *rootFlags) *cobra.Command {
	var flagPage string
	var flagResultsPerPage int
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "hacktivity <slug>",
		Short:       "List disclosed activity for one program",
		Example:     "  yeswehack-pp-cli programs hacktivity example-program --json",
		Annotations: map[string]string{"pp:endpoint": "programs.hacktivity", "pp:method": "GET", "pp:path": "/programs/{slug}/hacktivity", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, prov, err := readProgramHacktivity(cmd, c, flags, args[0], flagPage, flagResultsPerPage, flagAll)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printHacktivityListOutput(cmd, flags, data, prov)
		},
	}
	cmd.Flags().StringVar(&flagPage, "page", "", "Page number")
	cmd.Flags().IntVar(&flagResultsPerPage, "results-per-page", 0, "Page size")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")
	return cmd
}
